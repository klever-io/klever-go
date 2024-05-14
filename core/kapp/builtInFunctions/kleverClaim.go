package builtInFunctions

import (
	"fmt"
	"sync"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

type kleverClaim struct {
	baseAlwaysActiveHandler
	accountsCacher state.AccountsCacher
	kappController kapp.KAppController
	funcGasCost    uint64
	marshaller     vmcommon.Marshalizer
	keyPrefix      []byte
	mutExecution   sync.RWMutex
	forkController core.ForkController
}

// NewkleverClaimFunc returns the create asset built-in function component
func NewKleverClaimFunc(
	funcGasCost uint64,
	marshaller vmcommon.Marshalizer,
	accountsCacher state.AccountsCacher,
	forkController core.ForkController,
	kappController kapp.KAppController,
) (*kleverClaim, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(forkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	e := &kleverClaim{
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
func (e *kleverClaim) SetNewGasConfig(gasCost *vmcommon.GasCost) {
	if gasCost == nil {
		return
	}

	e.mutExecution.Lock()
	e.funcGasCost = gasCost.BuiltInCost.Claim
	e.mutExecution.Unlock()
}

// ProcessBuiltinFunction resolves KDA transfer function calls
func (e *kleverClaim) ProcessBuiltinFunction(vmInput *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	e.mutExecution.RLock()
	defer e.mutExecution.RUnlock()

	contract, err := e.getClaimContract(vmInput)
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
	var resultCode transaction.Transaction_TXResultCode
	switch contract.GetClaimType() {
	case transaction.ClaimContract_StakingClaim:
		resultCode, err = e.kappController.GetAccountsKApp().ClaimStaking(vmInput.CallerAddr, contract)
	case transaction.ClaimContract_AllowanceClaim:
		resultCode, err = e.kappController.GetAccountsKApp().ClaimAllowance(vmInput.CallerAddr, contract)
	case transaction.ClaimContract_MarketClaim:
		resultCode, err = e.kappController.GetMarketKApp().Claim(vmInput.CallerAddr, contract)
	default:
		resultCode = transaction.Transaction_ParameterInvalid
		err = common.ErrClaimTypeInvalid
	}
	if err != nil {
		log.Trace("KleverClaim error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	if resultCode != transaction.Transaction_Ok {
		err = fmt.Errorf("KleverClaim error: %s", resultCode.String())
		log.Trace("KleverClaim error", "resultCode", resultCode, "err", err.Error())
		return nil, err
	}

	vmOutput.GasRemaining, err = vmcommon.SafeSubUint64(vmInput.GasProvided, e.funcGasCost)
	if err != nil {
		return nil, err
	}

	return vmOutput, nil
}

// getClaimContract convert the arguments to an ClaimContract
func (e *kleverClaim) getClaimContract(vmInput *vmcommon.ContractCallInput) (*transaction.ClaimContract, error) {
	if len(vmInput.Arguments) < core.MinLenArgumentsClaim {
		return nil, ErrInvalidArguments
	}

	contract := &transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_EnumClaimType(vmInput.NextArg().Int32()),
		ID:        vmInput.NextArg(),
	}

	return contract, nil
}

// IsInterfaceNil returns true if underlying object in nil
func (e *kleverClaim) IsInterfaceNil() bool {
	return e == nil
}
