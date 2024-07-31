package builtInFunctions

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/mitchellh/mapstructure"
)

var _ vmcommon.BuiltInFunctionFactory = (*builtInFuncCreator)(nil)

var trueHandler = func() bool { return true }
var falseHandler = func() bool { return false }

// ArgsCreateBuiltInFunctionContainer defines the input arguments to create built in functions container
type ArgsCreateBuiltInFunctionContainer struct {
	AccountsCacher                   state.AccountsCacher
	KAppController                   kapp.KAppController
	GasMap                           map[string]map[string]uint64
	MapDNSAddresses                  map[string]struct{}
	EnableUserNameChange             bool
	Marshalizer                      vmcommon.Marshalizer
	ForkController                   core.ForkController
	MaxNumOfAddressesForTransferRole uint32
	ConfigAddress                    []byte
}

type builtInFuncCreator struct {
	accountsCacher                   state.AccountsCacher
	kappController                   kapp.KAppController
	mapDNSAddresses                  map[string]struct{}
	enableUserNameChange             bool
	marshaller                       vmcommon.Marshalizer
	builtInFunctions                 vmcommon.BuiltInFunctionContainer
	gasConfig                        *vmcommon.GasCost
	forkController                   core.ForkController
	maxNumOfAddressesForTransferRole uint32
	configAddress                    []byte
}

// NewBuiltInFunctionsCreator creates a component which will instantiate the built in functions contracts
func NewBuiltInFunctionsCreator(args ArgsCreateBuiltInFunctionContainer) (*builtInFuncCreator, error) {
	if check.IfNil(args.Marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(args.AccountsCacher) {
		return nil, ErrNilAccountsAdapter
	}
	if check.IfNil(args.KAppController) {
		return nil, common.ErrNilKAppController
	}
	if args.MapDNSAddresses == nil {
		return nil, ErrNilDnsAddresses
	}
	if check.IfNil(args.ForkController) {
		return nil, ErrNilEnableEpochsHandler
	}

	b := &builtInFuncCreator{
		mapDNSAddresses:                  args.MapDNSAddresses,
		enableUserNameChange:             args.EnableUserNameChange,
		marshaller:                       args.Marshalizer,
		accountsCacher:                   args.AccountsCacher,
		kappController:                   args.KAppController,
		forkController:                   args.ForkController,
		maxNumOfAddressesForTransferRole: args.MaxNumOfAddressesForTransferRole,
		configAddress:                    args.ConfigAddress,
	}

	var err error
	b.gasConfig, err = createGasConfig(args.GasMap)
	if err != nil {
		return nil, err
	}
	b.builtInFunctions = NewBuiltInFunctionContainer()

	return b, nil
}

// GasScheduleChange is called when gas schedule is changed, thus all contracts must be updated
func (b *builtInFuncCreator) GasScheduleChange(gasSchedule map[string]map[string]uint64) {
	newGasConfig, err := createGasConfig(gasSchedule)
	if err != nil {
		return
	}

	b.gasConfig = newGasConfig
	for key := range b.builtInFunctions.Keys() {
		builtInFunc, errGet := b.builtInFunctions.Get(key)
		if errGet != nil {
			return
		}

		builtInFunc.SetNewGasConfig(b.gasConfig)
	}
}

// BuiltInFunctionContainer will return the built in function container
func (b *builtInFuncCreator) BuiltInFunctionContainer() vmcommon.BuiltInFunctionContainer {
	return b.builtInFunctions
}

// CreateBuiltInFunctionContainer will create the list of built-in functions
func (b *builtInFuncCreator) CreateBuiltInFunctionContainer() error {
	b.builtInFunctions = NewBuiltInFunctionContainer()
	var newFunc vmcommon.BuiltinFunction
	newFunc, err := NewKDATransferFunc(
		b.gasConfig.BuiltInCost.Transfer,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionTransfer, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverAssetTriggerFunc(
		b.gasConfig.BuiltInCost.AssetTrigger,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionAssetTrigger, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverBuyFunc(
		b.gasConfig.BuiltInCost.Buy,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionBuy, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverCancelMarketOrderFunc(
		b.gasConfig.BuiltInCost.CancelMarketOrder,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionCancelMarketOrder, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverClaimFunc(
		b.gasConfig.BuiltInCost.Claim,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionClaim, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverConfigITOFunc(
		b.gasConfig.BuiltInCost.ConfigITO,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionConfigITO, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverConfigMarketplaceFunc(
		b.gasConfig.BuiltInCost.ConfigMarketplace,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionConfigMarketplace, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverCreateAssetFunc(
		b.gasConfig.BuiltInCost.CreateAsset,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionCreateAsset, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverCreateMarketplaceFunc(
		b.gasConfig.BuiltInCost.CreateMarketplace,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionCreateMarketplace, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverCreateValidatorFunc(
		b.gasConfig.BuiltInCost.CreateValidator,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionCreateValidator, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverDelegateFunc(
		b.gasConfig.BuiltInCost.Delegate,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDelegate, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverDepositFunc(
		b.gasConfig.BuiltInCost.Deposit,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionDeposit, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverFreezeFunc(
		b.gasConfig.BuiltInCost.Freeze,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionFreeze, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverITOTriggerFunc(
		b.gasConfig.BuiltInCost.ITOTrigger,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionITOTrigger, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverProposalFunc(
		b.gasConfig.BuiltInCost.Proposal,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionProposal, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverSellFunc(
		b.gasConfig.BuiltInCost.Sell,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionSell, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverSetAccountNameFunc(
		b.gasConfig.BuiltInCost.SetAccountName,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionSetAccountName, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverUndelegateFunc(
		b.gasConfig.BuiltInCost.Undelegate,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionUndelegate, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverUnfreezeFunc(
		b.gasConfig.BuiltInCost.Unfreeze,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionUnfreeze, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverUnjailFunc(
		b.gasConfig.BuiltInCost.Unjail,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionUnjail, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverUpdateAccountPermissionFunc(
		b.gasConfig.BuiltInCost.UpdateAccountPermission,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionUpdateAccountPermission, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverChangeOwnerAddressFunc(
		b.gasConfig.BuiltInCost.ChangeOwnerAddress,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionChangeOwnerAddress, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverValidatorConfigFunc(
		b.gasConfig.BuiltInCost.ValidatorConfig,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionValidatorConfig, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverVoteFunc(
		b.gasConfig.BuiltInCost.Vote,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionVote, newFunc)
	if err != nil {
		return err
	}

	newFunc, err = NewKleverWithdrawFunc(
		b.gasConfig.BuiltInCost.Withdraw,
		b.marshaller,
		b.accountsCacher,
		b.forkController,
		b.kappController)
	if err != nil {
		return err
	}
	err = b.builtInFunctions.Add(core.BuiltInFunctionWithdraw, newFunc)
	if err != nil {
		return err
	}

	return nil
}

func createGasConfig(gasMap map[string]map[string]uint64) (*vmcommon.GasCost, error) {
	baseOps := &vmcommon.BaseOperationCost{}
	err := mapstructure.Decode(gasMap[core.BaseOperationCostString], baseOps)
	if err != nil {
		return nil, err
	}

	err = check.ForZeroUintFields(*baseOps)
	if err != nil {
		return nil, err
	}

	builtInOps := &vmcommon.BuiltInCost{}
	err = mapstructure.Decode(gasMap[core.BuiltInCostString], builtInOps)
	if err != nil {
		return nil, err
	}

	err = check.ForZeroUintFields(*builtInOps)
	if err != nil {
		return nil, err
	}

	gasCost := vmcommon.GasCost{
		BaseOperationCost: *baseOps,
		BuiltInCost:       *builtInOps,
	}

	return &gasCost, nil
}

// SetPayableHandler sets the payableCheck interface to the needed functions
func (b *builtInFuncCreator) SetPayableHandler(payableHandler vmcommon.PayableHandler) error {
	payableChecker, err := NewPayableCheckFunc(
		payableHandler,
		b.forkController,
	)
	if err != nil {
		return err
	}

	listOfTransferFunc := []string{
		core.BuiltInFunctionTransfer,
	}

	for _, transferFunc := range listOfTransferFunc {
		builtInFunc, err := b.builtInFunctions.Get(transferFunc)
		if err != nil {
			return err
		}

		kdaTransferFunc, ok := builtInFunc.(vmcommon.AcceptPayableChecker)
		if !ok {
			return ErrWrongTypeAssertion
		}

		err = kdaTransferFunc.SetPayableChecker(payableChecker)
		if err != nil {
			return err
		}
	}

	return nil
}

// IsInterfaceNil returns true if underlying object is nil
func (b *builtInFuncCreator) IsInterfaceNil() bool {
	return b == nil
}
