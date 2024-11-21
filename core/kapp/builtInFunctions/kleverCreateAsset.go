package builtInFunctions

import (
	"fmt"
	"sync"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

type kleverCreateAsset struct {
	baseAlwaysActiveHandler
	accountsCacher state.AccountsCacher
	kappController kapp.KAppController
	funcGasCost    uint64
	marshaller     vmcommon.Marshalizer
	keyPrefix      []byte
	mutExecution   sync.RWMutex
	forkController core.ForkController
}

// NewKleverCreateAssetFunc returns the create asset built-in function component
func NewKleverCreateAssetFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverCreateAsset, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverCreateAsset{
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
func (e *kleverCreateAsset) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.CreateAsset
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverCreateAsset) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, err := e.getCreateAssetContract(vmInput)
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
	resultCode, err := e.kappController.GetKDAKApp().Create(vmInput.CallerAddr, contract)
	if err != nil {
		log.Trace("CreateAsset error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("CreateAsset error: %s", resultCode.String())
		log.Trace("CreateAsset error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	return vmOutput, nil
}

// getCreateAssetContract convert the arguments to a CreateAssetContract
func (e *kleverCreateAsset) getCreateAssetContract(vmInput *vmcommon.ContractCallInput) (*transaction.CreateAssetContract, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsCreateAsset {
		return nil, ErrInvalidArguments
	}

	contract := &transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_EnumAssetType(vmInput.NextArg().Int32()),
		Name:          vmInput.NextArg(),
		Ticker:        vmInput.NextArg(),
		Precision:     vmInput.NextArg().Uint32(),
		OwnerAddress:  vmInput.NextArg(),
		Logo:          vmInput.NextArg().String(),
		InitialSupply: vmInput.NextArg().Int64(),
		MaxSupply:     vmInput.NextArg().Int64(),
		Properties: &transaction.PropertiesInfo{
			CanFreeze:      vmInput.NextArg().Bool(),
			CanWipe:        vmInput.NextArg().Bool(),
			CanPause:       vmInput.NextArg().Bool(),
			CanMint:        vmInput.NextArg().Bool(),
			CanBurn:        vmInput.NextArg().Bool(),
			CanChangeOwner: vmInput.NextArg().Bool(),
			CanAddRoles:    vmInput.NextArg().Bool(),
			LimitTransfer:  vmInput.NextArg().Bool(),
		},
		Attributes: &transaction.AttributesInfo{
			IsPaused:                   vmInput.NextArg().Bool(),
			IsNFTMintStopped:           vmInput.NextArg().Bool(),
			IsRoyaltiesChangeStopped:   vmInput.NextArg().Bool(),
			IsNFTMetadataChangeStopped: vmInput.NextArg().Bool(),
		},
	}

	// use ticker if name is empty
	if len(contract.Name) == 0 {
		contract.Name = contract.Ticker
	}

	// initial admin address is the owner address
	contract.AdminAddress = contract.OwnerAddress

	uris, err := DecodeURIs(vmInput.NextArg())
	if err != nil {
		return nil, ErrInvalidArguments
	}

	contract.URIs = uris

	royalties, err := DecodeRoyaltiesData(vmInput.NextArg())
	if err != nil {
		return nil, ErrInvalidArguments
	}
	contract.Royalties = royalties

	return contract, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverCreateAsset) IsInterfaceNil() bool {
	return e == nil
}
