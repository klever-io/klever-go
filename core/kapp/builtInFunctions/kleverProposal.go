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

type kleverProposal struct {
	baseAlwaysActiveHandler
	accountsCacher state.AccountsCacher
	kappController kapp.KAppController
	funcGasCost    uint64
	marshaller     vmcommon.Marshalizer
	keyPrefix      []byte
	mutExecution   sync.RWMutex
	forkController core.ForkController
}

// NewKleverProposalFunc returns the create asset built-in function component
func NewKleverProposalFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverProposal, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverProposal{
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
func (e *kleverProposal) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.Proposal
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverProposal) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, err := e.getProposalContract(vmInput)
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
	resultCode, err := e.kappController.GetProposalKApp().Create(vmInput.CallerAddr, contract)
	if err != nil {
		log.Trace("Proposal error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("KleverProposal error: %s", resultCode.String())
		log.Trace("Proposal error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	return vmOutput, nil
}

// getProposalContract convert the arguments to an ProposalContract
func (e *kleverProposal) getProposalContract(vmInput *vmcommon.ContractCallInput) (*transaction.ProposalContract, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsProposal {
		return nil, ErrInvalidArguments
	}

	contract := &transaction.ProposalContract{
		EpochsDuration: vmInput.NextArg().Uint32(),
		Description:    vmInput.NextArg(),
		Parameters:     make(map[int32][]byte),
	}

	parameters, err := DecodeParameters(vmInput.NextArg())
	if err != nil {
		return nil, err
	}

	contract.Parameters = parameters

	return contract, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverProposal) IsInterfaceNil() bool {
	return e == nil
}
