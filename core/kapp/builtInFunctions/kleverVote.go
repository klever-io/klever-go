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

type kleverVote struct {
	baseAlwaysActiveHandler
	accountsCacher state.AccountsCacher
	kappController kapp.KAppController
	funcGasCost    uint64
	marshaller     vmcommon.Marshalizer
	keyPrefix      []byte
	payableHandler vmcommon.PayableChecker
	mutExecution   sync.RWMutex
	forkController core.ForkController
}

// NewKleverVoteFunc returns the create asset built-in function component
func NewKleverVoteFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverVote, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverVote{
		funcGasCost:    funcGasCost,
		marshaller:     marshaller,
		keyPrefix:      []byte(""),
		payableHandler: &disabledPayableHandler{},
		accountsCacher: accountsCacher,
		forkController: forkController,
		kappController: kappController,
	}

	return e, nil
}

// SetNewGasConfig is called whenever gas cost is changed
func (e *kleverVote) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.Vote
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverVote) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, err := e.getVoteContract(vmInput)
	if err != nil {
		return nil, err
	}

	err = checkBasicKDAArguments(vmInput)
	if err != nil {
		return nil, err
	}

	gasRemaining := computeGasRemaining(vmInput.GasProvided, e.funcGasCost)

	vmOutput := &vmcommon.VMOutput{GasRemaining: gasRemaining, ReturnCode: vmcommon.Ok}
	err = e.payableHandler.CheckPayable(vmInput, vmInput.RecipientAddr, core.MinLenArgumentsVote)
	if err != nil {
		return nil, err
	}

	//Using Kapps
	resultCode, err := e.kappController.GetProposalKApp().Vote(vmInput.CallerAddr, contract)
	if err != nil {
		log.Trace("ProposalVote error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("ProposalVote error: %s", resultCode.String())
		log.Trace("ProposalVote error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	vmOutput.GasRemaining, err = vmcommon.SafeSubUint64(vmInput.GasProvided, e.funcGasCost)
	if err != nil {
		return nil, err
	}

	return vmOutput, nil
}

// getVoteContract convert the arguments to an VoteContract
func (e *kleverVote) getVoteContract(vmInput *vmcommon.ContractCallInput) (*transaction.VoteContract, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsVote {
		return nil, ErrInvalidArguments
	}

	contract := &transaction.VoteContract{
		ProposalID: vmInput.NextArg().Uint64(),
		Type:       transaction.VoteContract_EnumVoteType(vmInput.NextArg().Int32()),
		Amount:     vmInput.NextArg().Int64(),
	}

	return contract, nil
}

// SetPayableChecker will set the payableCheck handler to the function
func (e *kleverVote) SetPayableChecker(payableHandler vmcommon.PayableChecker) error {
	if check.IfNil(payableHandler) {
		return ErrNilPayableHandler
	}

	e.payableHandler = payableHandler
	return nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverVote) IsInterfaceNil() bool {
	return e == nil
}
