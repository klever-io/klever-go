package factory_test

import (
	"encoding/hex"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/process/economics"
	"github.com/klever-io/klever-go/core/process/transactionLog"
	"github.com/klever-io/klever-go/core/statistics"
	"github.com/klever-io/klever-go/factory"
	"github.com/klever-io/klever-go/genesis/parsing"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/p2p/libp2p"
	"github.com/klever-io/klever-go/sharding"
	"github.com/stretchr/testify/assert"
)

func createMockHexPubkeyConverter() *mock.PubkeyConverterStub {
	return &mock.PubkeyConverterStub{
		DecodeCalled: func(humanReadable string) ([]byte, error) {
			return hex.DecodeString(humanReadable)
		},
	}
}

func getNetworkComponents() *factory.NetworkComponents {
	p2pConfig := config.P2PConfig{
		Node: config.NodeConfig{
			Port: "0",
			Seed: "seed",
		},
		KadDhtPeerDiscovery: config.KadDhtPeerDiscoveryConfig{
			Enabled:                          false,
			RefreshIntervalInSec:             10,
			ProtocolID:                       "klv/kad/1.0.0",
			InitialPeerList:                  []string{"peer0", "peer1"},
			BucketSize:                       10,
			RoutingTableRefreshIntervalInSec: 5,
		},
		Sharding: config.ShardingConfig{
			TargetPeerCount:         10,
			MaxIntraShardValidators: 10,
			MaxCrossShardValidators: 10,
			MaxIntraShardObservers:  10,
			MaxCrossShardObservers:  10,
			Type:                    "NilListSharder",
		},
	}
	ncf, _ := factory.NewNetworkComponentsFactory(
		config.Config{
			P2P: p2pConfig,
			Debug: config.DebugConfig{
				Antiflood: config.AntifloodDebugConfig{
					Enabled:                    true,
					CacheSize:                  100,
					IntervalAutoPrintInSeconds: 1,
				},
			},
		},
		&mock.AppStatusHandlerMock{},
		&mock.MarshalizerMock{},
		&libp2p.LocalSyncTimer{},
	)

	ncf.SetListenAddress(libp2p.ListenLocalhostAddrWithIp4AndTcp)

	nc, _ := ncf.Create()

	return nc
}

func getDataComponents() *factory.DataComponents {
	args := getDataArgs()
	dcf, _ := factory.NewDataComponentsFactory(args)
	dc, _ := dcf.Create()
	return dc
}

func getConfig() config.Config {
	cfg := mock.GetGeneralConfig()
	cfg.Versions = config.VersionsConfig{
		DefaultVersion:   "default",
		VersionsByEpochs: []config.VersionByEpochs{{StartEpoch: 0, Version: "*"}},
		Cache: config.CacheConfig{
			Type:     "LRU",
			Capacity: 1000,
			Shards:   1,
		},
	}

	cfg.Ratings.General.MaxRating = 100
	cfg.Antiflood = config.AntifloodConfig{NumConcurrentResolverJobs: 10}
	cfg.VMOutputCacher = config.CacheConfig{
		Type:     "LRU",
		Capacity: 1000,
		Shards:   1,
	}
	cfg.GasScheduleConfig = config.GasScheduleConfig{
		GasScheduleByEpochs: []config.GasScheduleByEpochs{
			{
				StartEpoch: 0,
				FileName:   "../config/node/gasScheduleV1.yaml",
			},
		},
	}
	cfg.VirtualMachine.Execution.WasmVMVersions = []config.WasmVMVersionByEpoch{{StartEpoch: 0, Version: "v1.0"}}

	return cfg
}

func getStateComponents(forkController core.ForkController) *factory.StateComponents {
	args := getStateArgs()

	scf, _ := factory.NewStateComponentsFactory(args)
	sc, _ := scf.Create(forkController)
	return sc
}

func createMockArguments() *factory.ProcessComponentsFactoryArgs {
	cfg := getConfig()

	coreArgs := factory.CoreComponentsFactoryArgs{
		Config:                cfg,
		ChainID:               []byte("12345"),
		MinTransactionVersion: 1,
	}

	accountsParser, err := parsing.NewAccountsParser(
		"../genesis/parsing/testdata/genesis_ok.json",
		createMockHexPubkeyConverter(),
		&mock.KeyGeneratorStub{},
	)
	assert.Nil(nil, err)

	epochNotifier := &mock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	proposalController, _ := kapps.NewProposalController(forkController)

	argsNewEconomicsData := economics.ArgsNewEconomicsData{
		EpochNotifier: &mock.EpochNotifierStub{},
	}
	economicsData, _ := economics.NewEconomicsData(argsNewEconomicsData)

	_ = economicsData.SetProposalController(proposalController)

	txLogsProcessor, _ := transactionLog.NewTxLogProcessor(transactionLog.ArgTxLogProcessor{
		Marshalizer:          &mock.MarshalizerMock{},
		SaveInStorageEnabled: false,
	})

	accCacher := &mock.AccountsCacherStub{}

	slotInterval := uint64(4)
	tpsBenchmark, _ := statistics.NewTPSBenchmark(slotInterval)

	args := getCryptoArgs()
	ccf, _ := factory.NewCryptoComponentsFactory(args, false)
	cc, _ := ccf.Create()

	return factory.NewProcessComponentsFactoryArgs(
		&coreArgs,
		accountsParser,
		economicsData,
		&sharding.NodesSetup{ChainID: "12345", MinTransactionVersion: 1},
		&mock.NodesCoordinatorMock{},
		getDataComponents(),
		getCoreComponents(),
		cc,
		getStateComponents(forkController),
		getNetworkComponents(),
		getTriesComponents(),
		&mock.RequestedItemsHandlerStub{},
		&mock.WhiteListHandlerStub{},
		&mock.WhiteListHandlerStub{},
		&mock.EpochStartNotifierStub{},
		cfg,
		&mock.RaterMock{},
		&mock.RatingsInfoMock{},
		10,
		mock.NewPubkeyConverterMock(96),
		"1",
		&mock.Uint64ByteSliceConverterMock{},
		"./",
		&consensusMock.IndexerMock{},
		tpsBenchmark,
		epochNotifier,
		"",
		&consensusMock.SlotManagerMock{},
		nil,
		&mock.FallBackHeaderValidatorStub{},
		false,
		accCacher,
		forkController,
		txLogsProcessor,
	)

}

func TestProcessComponentsFactory_ShouldWork(t *testing.T) {
	processComponentsFactoryArgs := createMockArguments()
	processComponentsFactory, err := factory.ProcessComponentsFactory(processComponentsFactoryArgs)
	assert.Nil(t, err)
	assert.NotNil(t, processComponentsFactory)
}
