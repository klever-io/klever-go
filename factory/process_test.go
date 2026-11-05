package factory_test

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/economics"
	"github.com/klever-io/klever-go/core/process/transactionLog"
	txSimData "github.com/klever-io/klever-go/core/process/txsimulator/data"
	"github.com/klever-io/klever-go/core/statistics"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/eventNotifier/notifier"
	"github.com/klever-io/klever-go/factory"
	"github.com/klever-io/klever-go/genesis/parsing"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/network/p2p/libp2p"
	"github.com/klever-io/klever-go/network/p2p/libp2p/p2pkey"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/vmcommon"
	libp2pCrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
)

func createP2pPrivateKey() libp2pCrypto.PrivKey {
	sk, err := p2pkey.Generate()
	if err != nil {
		panic(err)
	}

	return sk
}

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
		createP2pPrivateKey(),
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
	args, _, _, _ := createMockArgumentsWithLogs()
	return args
}

func createMockArgumentsWithLogs() (*factory.ProcessComponentsFactoryArgs, process.TransactionLogProcessor, *factory.DataComponents, *mock.AccountsCacherStub) {
	return createMockArgumentsWithGasSchedule("../config/node/gasScheduleV1.yaml")
}

func createMockArgumentsWithGasSchedule(gasScheduleFile string) (*factory.ProcessComponentsFactoryArgs, process.TransactionLogProcessor, *factory.DataComponents, *mock.AccountsCacherStub) {
	return createMockArgumentsWithCoreData(gasScheduleFile, getCoreComponents(), &mock.EpochNotifierStub{})
}

func createMockArgumentsWithCoreData(
	gasScheduleFile string,
	coreData *factory.CoreComponents,
	epochNotifier process.EpochNotifier,
) (*factory.ProcessComponentsFactoryArgs, process.TransactionLogProcessor, *factory.DataComponents, *mock.AccountsCacherStub) {
	cfg := getConfig()
	cfg.GasScheduleConfig.GasScheduleByEpochs[0].FileName = gasScheduleFile

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
	dc := getDataComponents()

	return factory.NewProcessComponentsFactoryArgs(
		&coreArgs,
		accountsParser,
		economicsData,
		&sharding.NodesSetup{ChainID: "12345", MinTransactionVersion: 1},
		&mock.NodesCoordinatorMock{},
		dc,
		coreData,
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
		nil,
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
		core.Normal,
	), txLogsProcessor, dc, accCacher
}

func TestProcessComponentsFactory_ShouldWork(t *testing.T) {
	processComponentsFactoryArgs := createMockArguments()
	processComponentsFactory, err := factory.ProcessComponentsFactory(processComponentsFactoryArgs)
	assert.Nil(t, err)
	assert.NotNil(t, processComponentsFactory)
}

// A simulation must not reach the block's log cache: CommitBlock persists whatever is
// in it under the transaction hash, which a simulated copy of a real tx shares.
func TestProcessComponentsFactory_SimulationDoesNotWriteBlockLogs(t *testing.T) {
	args, txLogsProcessor, dataComponents, _ := createMockArgumentsWithLogs()
	pc, err := factory.ProcessComponentsFactory(args)
	require.Nil(t, err)
	require.Nil(t, dataComponents.Blkc.SetCurrentBlockHeader(&block.Block{Header: &block.BlockHeader{Nonce: 1}}))

	// a failing init still makes the SC processor save the VM's error log
	code, err := os.ReadFile("../kvm/test/contracts/init-wrong/output/init-wrong.wasm")
	require.Nil(t, err)
	deploy, err := anypb.New(&transaction.SmartContract{Type: transaction.SmartContract_SCDeploy})
	require.Nil(t, err)
	tx := &transaction.Transaction{RawData: &transaction.Transaction_Raw{
		Sender:   make([]byte, 32),
		Data:     [][]byte{[]byte(hex.EncodeToString(code) + "@0500@0000")},
		Contract: []*transaction.TXContract{{Type: transaction.TXContract_SmartContractType, Parameter: deploy}},
	}}

	res, err := pc.TXSimulatorProcessor.ProcessTx(tx)
	require.Nil(t, err)
	require.NotNil(t, res.VMOutput, "the simulation must have reached the VM")
	require.Empty(t, txLogsProcessor.GetAllCurrentLogs())
}

// A simulation must run on the read-only cacher: through the live one a simulated
// deploy, upgrade or delete reaches the accounts the next block commits.
func TestProcessComponentsFactory_SimulationDoesNotTouchLiveAccounts(t *testing.T) {
	args, _, dataComponents, liveAccounts := createMockArgumentsWithLogs()
	pc, err := factory.ProcessComponentsFactory(args)
	require.Nil(t, err)
	require.Nil(t, dataComponents.Blkc.SetCurrentBlockHeader(&block.Block{Header: &block.BlockHeader{Nonce: 1}}))

	var touched []string
	refuse := func(name string) error {
		touched = append(touched, name)
		return assert.AnError
	}
	liveAccounts.LoadUserCalled = func([]byte) (state.UserAccountHandler, error) { return nil, refuse("LoadUser") }
	liveAccounts.GetExistingUserCalled = func([]byte) (state.UserAccountHandler, error) { return nil, refuse("GetExistingUser") }
	liveAccounts.UpdateUserCalled = func(state.AccountHandler) error { return refuse("UpdateUser") }
	liveAccounts.SaveUserCalled = func(state.AccountHandler) error { return refuse("SaveUser") }
	liveAccounts.RemoveCodeCalled = func([]byte) error { return refuse("RemoveCode") }
	liveAccounts.GetCodeCalled = func([]byte) []byte { _ = refuse("GetCode"); return nil }

	// a successful deploy, so the SC processor loads and writes the new contract account
	code, err := os.ReadFile("../kvm/test/contracts/init-correct/output/init-correct.wasm")
	require.Nil(t, err)
	deploy, err := anypb.New(&transaction.SmartContract{Type: transaction.SmartContract_SCDeploy})
	require.Nil(t, err)
	tx := &transaction.Transaction{RawData: &transaction.Transaction_Raw{
		Sender:   make([]byte, 32),
		Data:     [][]byte{[]byte(hex.EncodeToString(code) + "@0500@0000")},
		Contract: []*transaction.TXContract{{Type: transaction.TXContract_SmartContractType, Parameter: deploy}},
	}}

	res, err := pc.TXSimulatorProcessor.ProcessTx(tx)
	require.Empty(t, touched, "the simulation used the live accounts cacher")
	require.Nil(t, err)
	require.Equal(t, transaction.Transaction_SUCCESS, res.Result, res.FailReason)
}

// A simulation must not take the live VM container's change lock: block processing takes
// it for writing on every block (SetVMExecutionMode), so a simulation holding it for
// reading delays the block by up to one SC execution.
func TestProcessComponentsFactory_SimulationDoesNotTakeLiveVMChangeLock(t *testing.T) {
	coreData := getCoreComponents()
	args, _, dataComponents, _ := createMockArgumentsWithCoreData("../config/node/gasScheduleV1.yaml", coreData, &mock.EpochNotifierStub{})
	pc, err := factory.ProcessComponentsFactory(args)
	require.Nil(t, err)
	require.Nil(t, dataComponents.Blkc.SetCurrentBlockHeader(&block.Block{Header: &block.BlockHeader{Nonce: 1}}))

	code, err := os.ReadFile("../kvm/test/contracts/init-correct/output/init-correct.wasm")
	require.Nil(t, err)
	deploy, err := anypb.New(&transaction.SmartContract{Type: transaction.SmartContract_SCDeploy})
	require.Nil(t, err)
	tx := &transaction.Transaction{RawData: &transaction.Transaction_Raw{
		Sender:   make([]byte, 32),
		Data:     [][]byte{[]byte(hex.EncodeToString(code) + "@0500@0000")},
		Contract: []*transaction.TXContract{{Type: transaction.TXContract_SmartContractType, Parameter: deploy}},
	}}

	// hold the live lock for writing, as SetVMExecutionMode does at the start of a block
	liveLocker := coreData.LiveWasmVMChangeLocker
	liveLocker.Lock()
	locked := true
	defer func() {
		if locked {
			liveLocker.Unlock()
		}
	}()

	// the held lock must be the one block processing takes: the live SC processor's
	// SetVMExecutionMode waits on it, and goes through once it is released
	modeSet := make(chan struct{})
	go func() {
		pc.LiveSCProcessor().SetVMExecutionMode(vmcommon.ExecutionModeLeader)
		close(modeSet)
	}()
	select {
	case <-modeSet:
		t.Fatal("the live SC processor does not take the live VM change lock")
	case <-time.After(200 * time.Millisecond):
	}

	type outcome struct {
		res *txSimData.SimulationResults
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, errSim := pc.TXSimulatorProcessor.ProcessTx(tx)
		done <- outcome{res: res, err: errSim}
	}()

	select {
	case out := <-done:
		require.Nil(t, out.err)
		require.NotNil(t, out.res)
		require.NotNil(t, out.res.VMOutput, "the simulation must have reached the VM")
	case <-time.After(10 * time.Second):
		t.Fatal("the simulation waited on the live VM container's change lock")
	}

	liveLocker.Unlock()
	locked = false
	select {
	case <-modeSet:
	case <-time.After(10 * time.Second):
		t.Fatal("SetVMExecutionMode did not go through after the live lock was released")
	}
}

// requireVMSwapOnEpochChange checks the premise of the epoch tests: the test config's WASM VM
// version differs from the host's, so every epoch confirmation replaces each container's VM.
func requireVMSwapOnEpochChange(t *testing.T) {
	t.Helper()

	versions := getConfig().VirtualMachine.Execution.WasmVMVersions
	require.Len(t, versions, 1)
	msg := "the test config's VM version must differ from the host's, or CheckEpoch never swaps the VM"
	require.NotEqual(t, "*", versions[0].Version, msg)
	require.NotEqual(t, vmhost.VMVersion, versions[0].Version, msg)
}

// requireBlockedUntil runs call in a goroutine, checks it does not return while the lock is
// held, then releases the lock with unlock and checks call returns.
func requireBlockedUntil(t *testing.T, call func(), unlock func(), blockedMsg string) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		call()
		close(done)
	}()

	select {
	case <-done:
		unlock()
		t.Fatal(blockedMsg)
	case <-time.After(200 * time.Millisecond):
	}

	unlock()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the call did not return after the lock was released")
	}
}

// Every epoch confirmation replaces each container's WASM VM (the configured version is not
// the host's), so each VM factory must lock the change lock its processor runs under: the
// swap then waits for runs in flight on that container instead of closing their VM.
func TestProcessComponentsFactory_LiveVMSwapTakesLiveVMChangeLock(t *testing.T) {
	requireVMSwapOnEpochChange(t)

	coreData := getCoreComponents()
	epochNotifier := notifier.NewGenericEpochNotifier()
	args, _, _, _ := createMockArgumentsWithCoreData("../config/node/gasScheduleV1.yaml", coreData, epochNotifier)
	_, err := factory.ProcessComponentsFactory(args)
	require.Nil(t, err)

	// a live run in flight holds the live lock for reading
	coreData.LiveWasmVMChangeLocker.RLock()
	requireBlockedUntil(t,
		func() { epochNotifier.CheckEpoch(1) },
		coreData.LiveWasmVMChangeLocker.RUnlock,
		"the live VM was swapped without taking the live VM change lock",
	)
}

// The simulator's VM factory and SC processor must share the simulator's change lock: the
// factory's VM swap waits for a simulation in flight, and a simulation waits for a swap.
func TestProcessComponentsFactory_SimulatorVMFactoryAndProcessorShareALock(t *testing.T) {
	requireVMSwapOnEpochChange(t)

	coreData := getCoreComponents()
	epochNotifier := notifier.NewGenericEpochNotifier()
	args, _, dataComponents, _ := createMockArgumentsWithCoreData("../config/node/gasScheduleV1.yaml", coreData, epochNotifier)
	pc, err := factory.ProcessComponentsFactory(args)
	require.Nil(t, err)
	require.Nil(t, dataComponents.Blkc.SetCurrentBlockHeader(&block.Block{Header: &block.BlockHeader{Nonce: 1}}))

	simulatorLocker := coreData.SimulatorWasmVMChangeLocker

	// a simulation in flight holds the simulator lock for reading: the swap waits for it
	simulatorLocker.RLock()
	requireBlockedUntil(t,
		func() { epochNotifier.CheckEpoch(1) },
		simulatorLocker.RUnlock,
		"the simulator's VM was swapped without taking the simulator's VM change lock",
	)

	code, err := os.ReadFile("../kvm/test/contracts/init-correct/output/init-correct.wasm")
	require.Nil(t, err)
	deploy, err := anypb.New(&transaction.SmartContract{Type: transaction.SmartContract_SCDeploy})
	require.Nil(t, err)
	tx := &transaction.Transaction{RawData: &transaction.Transaction_Raw{
		Sender:   make([]byte, 32),
		Data:     [][]byte{[]byte(hex.EncodeToString(code) + "@0500@0000")},
		Contract: []*transaction.TXContract{{Type: transaction.TXContract_SmartContractType, Parameter: deploy}},
	}}

	// a swap holds the simulator lock for writing: a simulation waits for it
	var res *txSimData.SimulationResults
	var errSim error
	simulatorLocker.Lock()
	requireBlockedUntil(t,
		func() { res, errSim = pc.TXSimulatorProcessor.ProcessTx(tx) },
		simulatorLocker.Unlock,
		"the simulation ran without taking the simulator's VM change lock",
	)
	require.Nil(t, errSim)
	require.NotNil(t, res)
	require.NotNil(t, res.VMOutput, "the simulation must have reached the VM")
}

// gasScheduleWithTrieReadLimit copies the node's gas schedule with MaxNumberOfTrieReadsPerTx replaced.
func gasScheduleWithTrieReadLimit(t *testing.T, limit string) string {
	t.Helper()

	schedule, err := os.ReadFile("../config/node/gasScheduleV1.yaml")
	require.Nil(t, err)
	re := regexp.MustCompile(`(?m)^(\s*MaxNumberOfTrieReadsPerTx:)\s*\d+`)
	require.True(t, re.Match(schedule), "the gas schedule has no MaxNumberOfTrieReadsPerTx")

	path := filepath.Join(t.TempDir(), "gasSchedule.yaml")
	require.Nil(t, os.WriteFile(path, re.ReplaceAll(schedule, []byte("${1} "+limit)), 0o600))
	return path
}

// The simulator's blockchain hook must meter trie reads with a real usage counter: with the
// disabled one a simulation succeeds where the same tx fails once it is in a block.
func TestProcessComponentsFactory_SimulationEnforcesTrieReadLimit(t *testing.T) {
	// erc20 init stores twice (total supply and sender balance), reading each key first
	simulateERC20Deploy := func(t *testing.T, limit string) *txSimData.SimulationResults {
		args, _, dataComponents, _ := createMockArgumentsWithGasSchedule(gasScheduleWithTrieReadLimit(t, limit))
		pc, err := factory.ProcessComponentsFactory(args)
		require.Nil(t, err)
		require.Nil(t, dataComponents.Blkc.SetCurrentBlockHeader(&block.Block{Header: &block.BlockHeader{Nonce: 1}}))

		code, err := os.ReadFile("../kvm/test/contracts/erc20/output/erc20.wasm")
		require.Nil(t, err)
		deploy, err := anypb.New(&transaction.SmartContract{Type: transaction.SmartContract_SCDeploy})
		require.Nil(t, err)
		tx := &transaction.Transaction{RawData: &transaction.Transaction_Raw{
			Sender:   make([]byte, 32),
			Data:     [][]byte{[]byte(hex.EncodeToString(code) + "@0500@0000@64")},
			Contract: []*transaction.TXContract{{Type: transaction.TXContract_SmartContractType, Parameter: deploy}},
		}}

		res, err := pc.TXSimulatorProcessor.ProcessTx(tx)
		require.Nil(t, err)
		return res
	}

	t.Run("under the limit", func(t *testing.T) {
		res := simulateERC20Deploy(t, "1500")
		require.Equal(t, transaction.Transaction_SUCCESS, res.Result, res.FailReason)
	})

	t.Run("over the limit", func(t *testing.T) {
		res := simulateERC20Deploy(t, "1")
		require.NotEqual(t, transaction.Transaction_SUCCESS, res.Result)
		require.NotNil(t, res.VMOutput)
		require.Contains(t, res.FailReason+" "+res.VMOutput.ReturnMessage, "too many reads from trie")
	})
}
