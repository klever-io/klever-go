package builtInFunctions

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

type kleverUpdateAccountPermission struct {
	baseAlwaysActiveHandler
	accountsCacher state.AccountsCacher
	kappController kapp.KAppController
	funcGasCost    uint64
	marshaller     vmcommon.Marshalizer
	keyPrefix      []byte
	mutExecution   sync.RWMutex
	forkController core.ForkController
}

// NewKleverUpdateAccountPermissionFunc returns the create asset built-in function component
func NewKleverUpdateAccountPermissionFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverUpdateAccountPermission, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverUpdateAccountPermission{
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
func (e *kleverUpdateAccountPermission) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.UpdateAccountPermission
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverUpdateAccountPermission) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	address := vmInput.NextArg()

	contract, err := e.getUpdateAccountPermissionContract(vmInput)
	if err != nil {
		return nil, err
	}

	err = checkBasicKDAArguments(vmInput)
	if err != nil {
		return nil, err
	}

	gasRemaining := computeGasRemaining(vmInput.GasProvided, e.funcGasCost)
	vmOutput := &vmcommon.VMOutput{GasRemaining: gasRemaining, ReturnCode: vmcommon.Ok}

	acc, err := e.accountsCacher.LoadUser(address)
	if err != nil {
		return nil, err
	}

	has := false
	permissions := acc.GetPermissions()

LoopPermissions:
	for _, permission := range permissions {
		for _, signer := range permission.Signers {
			if !bytes.Equal(signer.Address, vmInput.RecipientAddr) {
				continue
			}

			if permission.Type != state.Permission_Owner {
				value := int32(transaction.TXContract_UpdateAccountPermissionContractType)
				base := value / 8
				index := value % 8

				if int32(len(permission.Operations)) <= base || permission.Operations[base]&(1<<index) == 0 {
					continue
				}
			}

			if permission.Threshold >= signer.Weight {
				has = true
				break LoopPermissions
			}
		}
	}

	if !has {
		return nil, errors.New("invalid permission operation")
	}

	//Using Kapps
	resultCode, err := e.kappController.GetAccountsKApp().UpdatePermission(address, contract)
	if err != nil {
		log.Trace("UpdatePermission error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("UpdatePermission error: %s", resultCode.String())
		log.Trace("UpdatePermission error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	return vmOutput, nil
}

// getUpdateAccountPermissionContract convert the arguments to an UpdateAccountPermissionContract
func (e *kleverUpdateAccountPermission) getUpdateAccountPermissionContract(vmInput *vmcommon.ContractCallInput) (*transaction.UpdateAccountPermissionContract, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsUpdateAccountPermission {
		return nil, ErrInvalidArguments
	}

	permissionsBytes := vmInput.NextArg()

	permissions, err := DecodeAccountPermissionData(permissionsBytes)
	if err != nil {
		return nil, err
	}

	contract := &transaction.UpdateAccountPermissionContract{
		Permissions: permissions,
	}

	return contract, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverUpdateAccountPermission) IsInterfaceNil() bool {
	return e == nil
}
