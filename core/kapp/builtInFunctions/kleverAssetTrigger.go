package builtInFunctions

import (
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kvm/math"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

type kleverAssetTrigger struct {
	baseAlwaysActiveHandler
	accountsCacher state.AccountsCacher
	kappController kapp.KAppController
	funcGasCost    uint64
	storePerByte   uint64
	marshaller     vmcommon.Marshalizer
	keyPrefix      []byte
	mutExecution   sync.RWMutex
	forkController core.ForkController
}

// NewKleverAssetTriggerFunc returns the create asset built-in function component
func NewKleverAssetTriggerFunc(
	funcGasCost uint64,
	storePerByte uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverAssetTrigger, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverAssetTrigger{
		funcGasCost:    funcGasCost,
		storePerByte:   storePerByte,
		marshaller:     marshaller,
		keyPrefix:      []byte(""),
		accountsCacher: accountsCacher,
		forkController: forkController,
		kappController: kappController,
	}

	return e, nil
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *kleverAssetTrigger) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.AssetTrigger
	e.storePerByte = gasCost.BaseOperationCost.StorePerByte
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverAssetTrigger) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, txData, err := e.getAssetTriggerContract(vmInput)
	if err != nil {
		return nil, err
	}

	err = checkBasicKDAArguments(vmInput)
	if err != nil {
		return nil, err
	}

	gasCost := e.funcGasCost
	if e.forkController.FixAuditChangesV7() {
		// state written from a contract is kept by every node, so it pays per stored byte
		gasCost = math.AddUint64(gasCost, math.MulUint64(e.storePerByte, e.storedSize(contract, txData)))
	}

	gasRemaining, err := computeBuiltinGasRemaining(vmInput.GasProvided, gasCost, e.forkController)
	if err != nil {
		return nil, err
	}
	vmOutput := &vmcommon.VMOutput{GasRemaining: gasRemaining, ReturnCode: vmcommon.Ok}

	//Using Kapps
	_, err = e.kappController.GetKDAKApp().Trigger(vmInput.CallerAddr, contract, txData)
	if err != nil {
		return nil, err
	}

	return vmOutput, nil
}

// getAssetTriggerContract convert the arguments to an AssetTriggerContract
func (e *kleverAssetTrigger) getAssetTriggerContract(vmInput *vmcommon.ContractCallInput) (*transaction.AssetTriggerContract, [][]byte, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsAssetTrigger {
		return nil, nil, ErrInvalidArguments
	}

	triggerType := transaction.AssetTriggerContract_EnumTriggerType(vmInput.NextArg().Int32())

	assetID := vmInput.NextArg()

	contract := &transaction.AssetTriggerContract{
		TriggerType: triggerType,
		AssetID:     assetID,
	}

	var txData [][]byte

	switch triggerType {
	case transaction.AssetTriggerContract_Mint:
		nonce := vmInput.NextArg()
		if len(nonce) > 0 {
			contract.AssetID = []byte(fmt.Sprintf("%s/%d", contract.AssetID, nonce.Int64()))
		}

		contract.Amount = vmInput.NextArg().Int64()
		contract.ToAddress = vmInput.NextArg()
		contract.Value = vmInput.NextArg().Int64()
	case transaction.AssetTriggerContract_Wipe:
		nonce := vmInput.NextArg()
		if len(nonce) > 0 {
			contract.AssetID = []byte(fmt.Sprintf("%s/%d", contract.AssetID, nonce.Int64()))
		}
		contract.Amount = vmInput.NextArg().Int64()
		contract.ToAddress = vmInput.NextArg()
	case transaction.AssetTriggerContract_Burn:
		nonce := vmInput.NextArg()
		if len(nonce) > 0 {
			contract.AssetID = []byte(fmt.Sprintf("%s/%d", contract.AssetID, nonce.Int64()))
		}
		contract.Amount = vmInput.NextArg().Int64()
	case transaction.AssetTriggerContract_Pause,
		transaction.AssetTriggerContract_Resume,
		transaction.AssetTriggerContract_StopNFTMint,
		transaction.AssetTriggerContract_StopNFTMetadataChange,
		transaction.AssetTriggerContract_StopRoyaltiesChange:
		//noop - no extra arguments needed
	case transaction.AssetTriggerContract_ChangeOwner,
		transaction.AssetTriggerContract_ChangeAdmin,
		transaction.AssetTriggerContract_RemoveRole,
		transaction.AssetTriggerContract_ChangeRoyaltiesReceiver:
		contract.ToAddress = vmInput.NextArg()
	case transaction.AssetTriggerContract_UpdateMetadata:
		nonce := vmInput.NextArg()
		if len(nonce) > 0 {
			contract.AssetID = []byte(fmt.Sprintf("%s/%d", contract.AssetID, nonce.Int64()))
		}

		contract.ToAddress = vmInput.NextArg()
		contract.MIME = vmInput.NextArg()

		//Add metadata to txData
		currentContractID := e.kappController.GetCurrentKAppContext().ContractID()
		data := make([][]byte, currentContractID+1)
		// Make compatible with TX from contract adding `@` to split keys if any
		attributes := vmInput.NextArg()
		if vmInput.HasNextArg() {
			name := vmInput.NextArg()
			data[currentContractID] = []byte(hex.EncodeToString(name))
			// add separator
			data[currentContractID] = append(data[currentContractID], []byte("@")...)
		}
		data[currentContractID] = append(data[currentContractID],
			[]byte(hex.EncodeToString(attributes))...)

		txData = data
	case transaction.AssetTriggerContract_UpdateLogo:
		contract.Logo = vmInput.NextArg().String()
	case transaction.AssetTriggerContract_UpdateStaking:
		contract.Staking = &transaction.StakingInfo{
			Type:                transaction.StakingInfo_InterestType(vmInput.NextArg().Int32()),
			APR:                 vmInput.NextArg().Uint32(),
			MinEpochsToClaim:    vmInput.NextArg().Uint32(),
			MinEpochsToUnstake:  vmInput.NextArg().Uint32(),
			MinEpochsToWithdraw: vmInput.NextArg().Uint32(),
		}
	case transaction.AssetTriggerContract_AddRole:
		contract.Role = &transaction.RolesInfo{
			Address:             vmInput.NextArg(),
			HasRoleMint:         vmInput.NextArg().Bool(),
			HasRoleSetITOPrices: vmInput.NextArg().Bool(),
			HasRoleDeposit:      vmInput.NextArg().Bool(),
			HasRoleTransfer:     vmInput.NextArg().Bool(),
		}
	case transaction.AssetTriggerContract_UpdateRoyalties:
		royalties, err := DecodeRoyaltiesData(vmInput.NextArg())
		if err != nil {
			return nil, txData, err
		}
		contract.Royalties = royalties
	case transaction.AssetTriggerContract_UpdateKDAFeePool:
		contract.KDAPool = &transaction.KDAPoolInfo{
			Active:       vmInput.NextArg().Bool(),
			AdminAddress: vmInput.NextArg(),
			FRatioKDA:    vmInput.NextArg().Int64(),
			FRatioKLV:    vmInput.NextArg().Int64(),
		}
	case transaction.AssetTriggerContract_UpdateURIs:
		uris, err := DecodeURIs(vmInput.NextArg())
		if err != nil {
			return nil, txData, err
		}

		contract.URIs = uris
	default:
		return nil, txData, ErrInvalidArguments
	}

	return contract, txData, nil
}

// storedSize returns the bytes a trigger adds to state beyond what its flat cost covers: the
// metadata of UpdateMetadata, and the records a mint adds (see mintStoredSize). Other trigger types
// store size-capped data and return 0.
func (e *kleverAssetTrigger) storedSize(contract *transaction.AssetTriggerContract, txData [][]byte) uint64 {
	switch contract.GetTriggerType() {
	case transaction.AssetTriggerContract_UpdateMetadata:
		return metadataSize(contract, txData, e.kappController.GetCurrentKAppContext().ContractID())
	case transaction.AssetTriggerContract_Mint:
		return mintStoredSize(e.kappController, contract.GetAssetID(), contract.GetAmount())
	default:
		return 0
	}
}

// metadataSize returns the bytes UpdateMetadata stores on an NFT: the MIME and the hex-encoded
// name@attributes, each with its field tag and length prefix. The KDA kapp stores only the
// metadata at the current contract's index, so only that entry counts. An SFT keeps only the
// decoded name and attributes, not the MIME, so it pays for the MIME and for about twice its
// stored bytes: a deliberate over-approximation that saves loading the asset type. A rewrite pays
// for the full new value, not only its growth over the stored one, since the builtin does not load
// the stored record.
func metadataSize(contract *transaction.AssetTriggerContract, txData [][]byte, contractID int) uint64 {
	size := uint64(len(contract.GetMIME())) + 2*maxVarintFieldSize
	if contractID >= 0 && contractID < len(txData) {
		size += uint64(len(txData[contractID]))
	}

	return size
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverAssetTrigger) IsInterfaceNil() bool {
	return e == nil
}
