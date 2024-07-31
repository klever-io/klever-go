package builtInFunctions

import (
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

type kleverAssetTrigger struct {
	baseAlwaysActiveHandler
	accountsCacher state.AccountsCacher
	kappController kapp.KAppController
	funcGasCost    uint64
	marshaller     vmcommon.Marshalizer
	keyPrefix      []byte
	mutExecution   sync.RWMutex
	forkController core.ForkController
}

// NewKleverAssetTriggerFunc returns the create asset built-in function component
func NewKleverAssetTriggerFunc(
	funcGasCost uint64,
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

	gasRemaining := computeGasRemaining(vmInput.GasProvided, e.funcGasCost)
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

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverAssetTrigger) IsInterfaceNil() bool {
	return e == nil
}
