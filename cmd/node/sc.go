package main

import (
	"fmt"
	"sync"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/factory/chain"
	"github.com/klever-io/klever-go/core/process/smartContract"
	"github.com/klever-io/klever-go/core/process/smartContract/builtInFunctions"
	"github.com/klever-io/klever-go/core/process/smartContract/hooks"
	"github.com/klever-io/klever-go/core/process/smartContract/hooks/counters"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/eventNotifier/notifier"
	"github.com/klever-io/klever-go/factory"
	"github.com/klever-io/klever-go/sharding"
	storageFactory "github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/vmcommon/parsers"
)

type scQueryServiceArgs struct {
	epochNotifier       process.EpochNotifier
	forkController      core.ForkController
	generalConfig       *config.Config
	coreComponents      *factory.CoreComponents
	stateComponents     *factory.StateComponents
	dataComponents      *factory.DataComponents
	processComponents   *factory.Process
	gasScheduleNotifier core.GasScheduleNotifier
	economicsData       process.EconomicsDataHandler
	allowVMQueriesChan  chan struct{}
	workingDir          string
	rater               sharding.ChanceComputer
}

type scQueryElementArgs struct {
	epochNotifier       process.EpochNotifier
	forkController      core.ForkController
	generalConfig       *config.Config
	coreComponents      *factory.CoreComponents
	stateComponents     *factory.StateComponents
	dataComponents      *factory.DataComponents
	processComponents   *factory.Process
	gasScheduleNotifier core.GasScheduleNotifier
	economicsData       process.EconomicsDataHandler
	rater               sharding.ChanceComputer
	allowVMQueriesChan  chan struct{}
	workingDir          string
	index               int
}

func createScQueryService(
	args *scQueryServiceArgs,
) (process.SCQueryService, error) {
	numConcurrentVms := args.generalConfig.VirtualMachine.Querying.NumConcurrentVMs
	if numConcurrentVms < 1 {
		return nil, fmt.Errorf("VirtualMachine.Querying.NumConcurrentVms should be a positive number more than 1")
	}

	argsQueryElem := &scQueryElementArgs{
		epochNotifier:       args.epochNotifier,
		forkController:      args.forkController,
		generalConfig:       args.generalConfig,
		coreComponents:      args.coreComponents,
		stateComponents:     args.stateComponents,
		dataComponents:      args.dataComponents,
		processComponents:   args.processComponents,
		gasScheduleNotifier: args.gasScheduleNotifier,
		economicsData:       args.economicsData,
		rater:               args.rater,
		allowVMQueriesChan:  args.allowVMQueriesChan,
		workingDir:          args.workingDir,
		index:               0,
	}

	var err error
	var scQueryService process.SCQueryService

	list := make([]process.SCQueryService, 0, numConcurrentVms)
	for i := 0; i < numConcurrentVms; i++ {
		argsQueryElem.index = i
		scQueryService, err = createScQueryElement(argsQueryElem)
		if err != nil {
			return nil, err
		}

		list = append(list, scQueryService)
	}

	sqQueryDispatcher, err := smartContract.NewScQueryServiceDispatcher(list)
	if err != nil {
		return nil, err
	}

	return sqQueryDispatcher, nil
}

func createScQueryElement(
	args *scQueryElementArgs,
) (process.SCQueryService, error) {
	var vmFactory process.VirtualMachinesContainerFactory
	var err error

	wasmVMChangeLocker := &sync.RWMutex{}
	argsGasScheduleNotifier := notifier.ArgsNewGasScheduleNotifier{
		GasScheduleConfig:  args.generalConfig.GasScheduleConfig,
		EpochNotifier:      args.epochNotifier,
		WasmVMChangeLocker: wasmVMChangeLocker,
	}
	gasScheduleNotifier, err := notifier.NewGasScheduleNotifier(argsGasScheduleNotifier)
	if err != nil {
		return nil, err
	}

	accCacher, err := state.NewReadOnlyAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: args.stateComponents.AccountsAdapter,
			Kapps:    args.stateComponents.KAppsAdapter,
			Peers:    args.stateComponents.PeersAdapter,
		},
	)
	if err != nil {
		return nil, err
	}

	argsBuiltIn := builtInFunctions.ArgsCreateBuiltInFunctionContainer{
		GasSchedule:     gasScheduleNotifier,
		MapDNSAddresses: make(map[string]struct{}),
		Marshalizer:     args.coreComponents.InternalMarshalizer,
		AccountsCacher:  accCacher,
		KAppController:  args.stateComponents.KAppController,
		EpochNotifier:   args.epochNotifier,
		ForkController:  args.forkController,
	}

	builtInFuncFactory, err := builtInFunctions.CreateBuiltInFunctionsFactory(argsBuiltIn)
	if err != nil {
		return nil, err
	}

	cacherCfg := storageFactory.GetCacherFromConfig(args.generalConfig.SmartContractDataPool)
	smartContractsCache, err := storageUnit.NewCache(cacherCfg)
	if err != nil {
		return nil, err
	}

	scStorage := args.generalConfig.Storages.SmartContractsStorageForSCQuery
	scStorage.DB.FilePath += fmt.Sprintf("%d", args.index)
	argsHook := hooks.ArgBlockChainHook{
		AccountsCacher:     accCacher,
		KAppController:     args.stateComponents.KAppController,
		PubkeyConv:         args.stateComponents.AddressPubkeyConverter,
		StorageService:     args.dataComponents.Store,
		BlockChain:         args.dataComponents.Blkc,
		Marshalizer:        args.coreComponents.InternalMarshalizer,
		Uint64Converter:    args.coreComponents.Uint64ByteSliceConverter,
		BuiltInFunctions:   builtInFuncFactory.BuiltInFunctionContainer(),
		DataPool:           args.dataComponents.Datapool,
		ConfigSCStorage:    scStorage,
		CompiledSCPool:     smartContractsCache,
		WorkingDir:         args.workingDir,
		EpochNotifier:      args.epochNotifier,
		EnableEpochs:       args.generalConfig.EnableEpochs,
		ForkController:     args.forkController,
		NilCompiledSCStore: true,
		GasSchedule:        args.gasScheduleNotifier,
		Counter:            counters.NewDisabledCounter(),
	}

	maxGasForVmQueries := args.generalConfig.VirtualMachine.GasConfig.MetaMaxGasPerVmQuery

	blockChainHookImpl, errBlockChainHook := hooks.NewBlockChainHookImpl(argsHook)
	if errBlockChainHook != nil {
		return nil, errBlockChainHook
	}

	kdaTransferParser, errParser := parsers.NewKDATransferParser(args.coreComponents.InternalMarshalizer)
	if errParser != nil {
		return nil, errParser
	}

	argsNewVMFactory := chain.ArgVMContainerFactory{
		BlockChainHook:     blockChainHookImpl,
		BuiltInFunctions:   argsHook.BuiltInFunctions,
		Config:             args.generalConfig.VirtualMachine.Execution,
		EpochNotifier:      args.epochNotifier,
		ForkController:     args.forkController,
		WasmVMChangeLocker: args.coreComponents.WasmVMChangeLocker,
		KDATransferParser:  kdaTransferParser,
		Hasher:             args.coreComponents.Hasher,
		GasSchedule:        args.gasScheduleNotifier,
	}

	vmFactory, err = chain.NewVMContainerFactory(argsNewVMFactory)
	if err != nil {
		return nil, err
	}

	vmContainer, err := vmFactory.Create()
	if err != nil {
		return nil, err
	}

	argsNewSCQueryService := smartContract.ArgsNewSCQueryService{
		VmContainer:              vmContainer,
		EconomicsFee:             args.economicsData,
		BlockChainHook:           vmFactory.BlockChainHookImpl(),
		BlockChain:               args.dataComponents.Blkc,
		WasmVMChangeLocker:       args.coreComponents.WasmVMChangeLocker,
		AllowExternalQueriesChan: args.allowVMQueriesChan,
		MaxGasLimitPerQuery:      maxGasForVmQueries,
	}

	return smartContract.NewSCQueryService(argsNewSCQueryService)
}
