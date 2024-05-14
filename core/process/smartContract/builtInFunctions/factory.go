package builtInFunctions

import (
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	vmcommonBuiltInFunctions "github.com/klever-io/klever-go/core/kapp/builtInFunctions"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
)

var log = logger.GetOrCreate("process/smartcontract/builtInFunctions")

// ArgsCreateBuiltInFunctionContainer defines the argument structure to create new built in function container
type ArgsCreateBuiltInFunctionContainer struct {
	AccountsCacher            state.AccountsCacher
	KAppController            kapp.KAppController
	GasSchedule               core.GasScheduleNotifier
	MapDNSAddresses           map[string]struct{}
	EnableUserNameChange      bool
	Marshalizer               marshal.Marshalizer
	EpochNotifier             vmcommon.EpochNotifier
	ForkController            core.ForkController
	MaxNumNodesInTransferRole uint32
}

// CreateBuiltInFunctionsFactory creates a container that will hold all the available built in functions
func CreateBuiltInFunctionsFactory(args ArgsCreateBuiltInFunctionContainer) (vmcommon.BuiltInFunctionFactory, error) {
	if check.IfNil(args.GasSchedule) {
		return nil, process.ErrNilGasSchedule
	}
	if check.IfNil(args.Marshalizer) {
		return nil, process.ErrNilMarshalizer
	}
	if check.IfNil(args.AccountsCacher) {
		return nil, process.ErrNilAccountsAdapter
	}
	if check.IfNil(args.KAppController) {
		return nil, common.ErrNilKAppController
	}
	if args.MapDNSAddresses == nil {
		return nil, process.ErrNilDnsAddresses
	}
	if check.IfNil(args.EpochNotifier) {
		return nil, process.ErrNilEpochNotifier
	}
	if check.IfNil(args.ForkController) {
		return nil, process.ErrNilEnableEpochsHandler
	}

	modifiedArgs := vmcommonBuiltInFunctions.ArgsCreateBuiltInFunctionContainer{
		GasMap:                           args.GasSchedule.LatestGasSchedule(),
		MapDNSAddresses:                  args.MapDNSAddresses,
		EnableUserNameChange:             args.EnableUserNameChange,
		Marshalizer:                      args.Marshalizer,
		AccountsCacher:                   args.AccountsCacher,
		KAppController:                   args.KAppController,
		ForkController:                   args.ForkController,
		ConfigAddress:                    nil,
		MaxNumOfAddressesForTransferRole: args.MaxNumNodesInTransferRole,
	}

	bContainerFactory, err := vmcommonBuiltInFunctions.NewBuiltInFunctionsCreator(modifiedArgs)
	if err != nil {
		return nil, err
	}

	err = bContainerFactory.CreateBuiltInFunctionContainer()
	if err != nil {
		return nil, err
	}

	args.GasSchedule.RegisterNotifyHandler(bContainerFactory)

	return bContainerFactory, nil
}
