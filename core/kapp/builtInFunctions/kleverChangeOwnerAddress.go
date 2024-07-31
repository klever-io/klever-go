package builtInFunctions

import (
	"bytes"
	"sync"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

type kleverChangeOwnerAddress struct {
	baseAlwaysActiveHandler
	accountsCacher state.AccountsCacher
	kappController kapp.KAppController
	funcGasCost    uint64
	marshaller     vmcommon.Marshalizer
	keyPrefix      []byte
	mutExecution   sync.RWMutex
	forkController core.ForkController
}

func NewKleverChangeOwnerAddressFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverChangeOwnerAddress, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverChangeOwnerAddress{
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
func (e *kleverChangeOwnerAddress) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.ChangeOwnerAddress
	e.mutExecution.Unlock()
}

func (e *kleverChangeOwnerAddress) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	if len(vmInput.Arguments) != core.MinLenArgumentsChangeOwnerAddress {
		return nil, ErrInvalidArguments
	}

	callerAddress := vmInput.CallerAddr
	newOwnerAddress := vmInput.Arguments[0]

	gasRemaining := computeGasRemaining(vmInput.GasProvided, e.funcGasCost)
	vmOutput := &vmcommon.VMOutput{GasRemaining: gasRemaining, ReturnCode: vmcommon.Ok}

	acc, err := e.accountsCacher.GetExistingUser(vmInput.RecipientAddr)
	if err != nil {
		return nil, err
	}

	if len(newOwnerAddress) != vmhost.AddressLen {
		return nil, ErrInvalidAddressLength
	}

	if !bytes.Equal(callerAddress, acc.GetOwnerAddress()) {
		return nil, ErrOperationNotPermitted
	}

	acc.SetOwnerAddress(newOwnerAddress)

	return vmOutput, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverChangeOwnerAddress) IsInterfaceNil() bool {
	return e == nil
}
