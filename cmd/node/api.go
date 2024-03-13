package main

import (
	"github.com/klever-io/klever-go/common/facade"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	txcostestimator "github.com/klever-io/klever-go/core/process/transaction/txCostEstimator"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/factory"
	"github.com/klever-io/klever-go/network/api/apiResolver"
	"github.com/klever-io/klever-go/node/stakeValuesProcessor"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

func createAPIResolver(
	generalConfig *config.Config,
	accnts state.AccountsAdapter,
	validatorAccounts state.AccountsAdapter,
	kapps state.AccountsAdapter,
	pubkeyConv core.PubkeyConverter,
	storageService retriever.StorageService,
	dataPool retriever.PoolsHolder,
	blockChain data.ChainHandler,
	marshalizer marshal.Marshalizer,
	hasher hashing.Hasher,
	uint64Converter typeConverters.Uint64ByteSliceConverter,
	statusMetrics core.StatusMetricsHandler,
	economics process.EconomicsDataHandler,
	nodesSetup sharding.GenesisNodesSetupHandler,
	epochNotifier process.EpochNotifier,
	apiWorkingDir string,
	coreComponents *factory.CoreComponents,
	stateComponents *factory.StateComponents,
	dataComponents *factory.DataComponents,
	processComponents *factory.Process,
	gasScheduleNotifier core.GasScheduleNotifier,
	cryptoComponents *factory.CryptoComponents,
	forkController core.ForkController,
	rater sharding.ChanceComputer,
	allowVMQueriesChan chan struct{},
) (facade.APIResolver, error) {

	args := &stakeValuesProcessor.ArgsTotalStakedValueHandler{
		InternalMarshalizer: marshalizer,
		Accounts:            accnts,
	}
	totalStakedValueHandler, err := stakeValuesProcessor.CreateTotalStakedValueHandler(args)
	if err != nil {
		return nil, err
	}

	var argsSCQuery = &scQueryServiceArgs{
		epochNotifier:       epochNotifier,
		forkController:      forkController,
		generalConfig:       generalConfig,
		coreComponents:      coreComponents,
		stateComponents:     stateComponents,
		dataComponents:      dataComponents,
		processComponents:   processComponents,
		gasScheduleNotifier: gasScheduleNotifier,
		economicsData:       economics,
		allowVMQueriesChan:  allowVMQueriesChan,
		workingDir:          apiWorkingDir,
		rater:               rater,
	}
	scQueryService, err := createScQueryService(argsSCQuery)
	if err != nil {
		return nil, err
	}

	readOnlyAccountsCacher, err := state.NewReadOnlyAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: stateComponents.AccountsAdapter,
			Kapps:    stateComponents.KAppsAdapter,
			Peers:    stateComponents.PeersAdapter,
		},
	)
	if err != nil {
		return nil, err
	}

	txCostHandler, err := txcostestimator.NewTransactionCostEstimator(
		readOnlyAccountsCacher,
		economics,
		processComponents.TXSimulatorProcessor,
		processComponents.ForkController,
	)
	if err != nil {
		return nil, err
	}

	return apiResolver.NewNodeAPIResolver(statusMetrics, totalStakedValueHandler, scQueryService, txCostHandler)
}
