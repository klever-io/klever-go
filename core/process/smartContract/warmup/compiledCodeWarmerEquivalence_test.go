//go:build !race

// Excluded from -race builds: -race enables checkptr, which aborts every real wasm call that
// reaches a VM hook (kvm/wasmer2 getVMHooksFromContextRawPtr, flagged by go vet), on HEAD as
// well. The warmer's own concurrency is covered by the -race run of the other tests here.

package warmup

// KLC-3001 consensus equivalence: a call that restores a module produced by the warmer
// must behave exactly like the same call on the cold path (compile from bytecode) and like
// a call restoring the module an earlier cold call cached, on real contracts, under several
// gas schedules and fork-flag combinations, and at the out-of-gas boundary.
//
// The modules are not byte-identical: the GasLimit option is serialized into Cache()
// output and the warmer compiles with a fixed limit. What must hold is that the restoring
// call's own limit governs metering, so every observable result is the same.

import (
	"sync"
	"testing"

	commonMock "github.com/klever-io/klever-go/common/mock"
	blockchainConfig "github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/crypto/hashing/blake2b"
	notifierMock "github.com/klever-io/klever-go/eventNotifier/mock"
	kvmConfig "github.com/klever-io/klever-go/kvm/config"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	gasSchedules "github.com/klever-io/klever-go/kvm/scenarioexec/gasSchedules"
	"github.com/klever-io/klever-go/kvm/testcommon"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/require"
)

const equivalenceGasBudget = uint64(500_000_000)

// moduleStore stands in for the compiled-code pool behind a BlockchainHookStub.
type moduleStore struct {
	mut     sync.Mutex
	modules map[string][]byte
	saves   int
}

func newModuleStore() *moduleStore {
	return &moduleStore{modules: make(map[string][]byte)}
}

func (s *moduleStore) attach(hook *contextmock.BlockchainHookStub) {
	hook.SaveCompiledCodeCalled = func(codeHash []byte, code []byte) {
		s.mut.Lock()
		defer s.mut.Unlock()
		s.saves++
		s.modules[string(codeHash)] = append([]byte(nil), code...)
	}
	hook.GetCompiledCodeCalled = func(codeHash []byte) (bool, []byte) {
		s.mut.Lock()
		defer s.mut.Unlock()
		code, ok := s.modules[string(codeHash)]
		return ok, code
	}
}

func (s *moduleStore) snapshot() (int, int) {
	s.mut.Lock()
	defer s.mut.Unlock()
	return len(s.modules), s.saves
}

func (s *moduleStore) clone() *moduleStore {
	s.mut.Lock()
	defer s.mut.Unlock()
	c := newModuleStore()
	for k, v := range s.modules {
		c.modules[k] = v
	}
	return c
}

type equivalenceCall struct {
	name      string
	contract  string
	function  string
	arguments [][]byte
}

type callResult struct {
	output *vmcommon.VMOutput
	err    error
}

func runEquivalenceCall(
	t *testing.T,
	code []byte,
	call equivalenceCall,
	epochs blockchainConfig.EnableEpochs,
	schedule kvmConfig.GasScheduleMap,
	store *moduleStore,
	gasProvided uint64,
) callResult {
	t.Helper()

	hook := testcommon.BlockchainHookStubForContracts([]*testcommon.InstanceTestSmartContract{
		testcommon.CreateInstanceContract(testcommon.ParentAddress).
			WithCode(code).
			WithBalance(1000),
	})
	store.attach(hook)

	host := testcommon.NewTestHostBuilder(t).
		WithBlockchainHook(hook).
		WithEnableEpochs(epochs).
		WithGasSchedule(schedule).
		Build()
	defer host.Reset()

	input := testcommon.CreateTestContractCallInputBuilder().
		WithRecipientAddr(testcommon.ParentAddress).
		WithGasProvided(gasProvided).
		WithFunction(call.function).
		WithArguments(call.arguments...).
		Build()

	output, err := host.RunSmartContractCall(input)
	return callResult{output: output, err: err}
}

// warmedStore runs the warmer, configured with the same gas schedule and fork flags as the
// host, against the contract and returns the compiled-code store it filled.
func warmedStore(t *testing.T, code []byte, epochs blockchainConfig.EnableEpochs, schedule kvmConfig.GasScheduleMap) *moduleStore {
	t.Helper()

	store := newModuleStore()
	hook := &contextmock.BlockchainHookStub{}
	store.attach(hook)

	accounts := newFakeAccounts()
	accounts.setContract(testcommon.ParentAddress, code)

	forkController, err := fork.NewForkController(epochs, &commonMock.EpochNotifierStub{})
	require.NoError(t, err)

	w, err := newCompiledCodeWarmer(ArgsCompiledCodeWarmer{
		TxPool:        newFakeTxPool(t),
		Accounts:      accounts,
		CompiledCode:  hook,
		GasSchedule:   notifierMock.NewGasScheduleNotifierMock(schedule),
		ForkFlags:     forkController,
		Hasher:        &blake2b.Blake2b{}, // the hasher the test host computes code hashes with
		StatusHandler: statusHandler.NewNilStatusHandler(),
		// no Executor: the warmer creates its own, exactly as in production
	}, 4, 4)
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	w.warm(testcommon.ParentAddress)
	require.Equal(t, uint64(1), w.Counters().Compiled, "counters: %+v", w.Counters())

	numModules, _ := store.snapshot()
	require.Equal(t, 1, numModules)
	return store
}

// minimumSuccessfulBudget finds, on the cold path, the smallest gas budget the call
// succeeds with. It is at least the gas the call consumes, and can be more: some checks
// require gas to be left over before charging it.
func minimumSuccessfulBudget(
	t *testing.T,
	code []byte,
	call equivalenceCall,
	epochs blockchainConfig.EnableEpochs,
	schedule kvmConfig.GasScheduleMap,
	gasUsed uint64,
) uint64 {
	t.Helper()

	succeeds := func(budget uint64) bool {
		result := runEquivalenceCall(t, code, call, epochs, schedule, newModuleStore(), budget)
		require.NoError(t, result.err)
		return result.output.ReturnCode == vmcommon.Ok
	}

	low := gasUsed - 1 // the call cannot succeed with less than it consumes
	require.False(t, succeeds(low))
	high := gasUsed
	for step := uint64(1); !succeeds(high); step *= 2 {
		low = high
		high = gasUsed + step
		require.LessOrEqual(t, high, equivalenceGasBudget)
	}
	for high-low > 1 {
		mid := low + (high-low)/2
		if succeeds(mid) {
			high = mid
		} else {
			low = mid
		}
	}

	return high
}

func requireSameResult(t *testing.T, expected callResult, actual callResult, path string) {
	t.Helper()

	require.Equal(t, expected.err, actual.err, "%s: error", path)
	if expected.output == nil || actual.output == nil {
		require.Equal(t, expected.output, actual.output, "%s: output", path)
		return
	}
	require.Equal(t, expected.output.ReturnCode, actual.output.ReturnCode, "%s: ReturnCode", path)
	require.Equal(t, expected.output.ReturnMessage, actual.output.ReturnMessage, "%s: ReturnMessage", path)
	require.Equal(t, expected.output.GasRemaining, actual.output.GasRemaining, "%s: GasRemaining", path)
	require.Equal(t, expected.output.ReturnData, actual.output.ReturnData, "%s: ReturnData", path)
	require.Equal(t, expected.output, actual.output, "%s: full VMOutput", path)
}

func TestCompiledCodeWarmer_WarmedModuleBehavesLikeColdPath(t *testing.T) {
	v1Schedule, err := gasSchedules.LoadGasScheduleConfig(gasSchedules.GetV1())
	require.NoError(t, err)

	schedules := []struct {
		name     string
		schedule func() kvmConfig.GasScheduleMap
	}{
		{"test schedule", kvmConfig.MakeGasMapForTests},
		{"v1 schedule", func() kvmConfig.GasScheduleMap { return v1Schedule }},
	}
	forks := []struct {
		name   string
		epochs blockchainConfig.EnableEpochs
	}{
		{"before V4", blockchainConfig.EnableEpochs{FixAuditChangesV4: 1000, FixAuditChangesV5: 1001}},
		{"V4 only", blockchainConfig.EnableEpochs{FixAuditChangesV5: 1000}},
		{"V4 and V5", blockchainConfig.EnableEpochs{}},
	}
	calls := []equivalenceCall{
		{name: "gas-probe accumulate", contract: "gas-probe", function: "accumulate", arguments: [][]byte{{50}, {32}}},
		{name: "gas-probe readModifyWrite", contract: "gas-probe", function: "readModifyWrite", arguments: [][]byte{{10}, {64}}},
		{name: "answer", contract: "answer", function: "answer"},
		{name: "counter increment", contract: "counter", function: "increment"},
		{name: "missing function", contract: "counter", function: "klc3001NotExported"},
	}

	for _, sched := range schedules {
		for _, fk := range forks {
			for _, call := range calls {
				t.Run(sched.name+"/"+fk.name+"/"+call.name, func(t *testing.T) {
					code := testcommon.GetTestSCCode(call.contract, "../../../../kvm/")
					schedule := sched.schedule()

					warmStore := warmedStore(t, code, fk.epochs, schedule)

					// Seed: a cold call with a generous budget. It also caches its own module,
					// the established path the warmed module must match.
					coldCachedStore := newModuleStore()
					cold := runEquivalenceCall(t, code, call, fk.epochs, schedule, coldCachedStore, equivalenceGasBudget)
					require.NoError(t, cold.err)
					numCold, savesCold := coldCachedStore.snapshot()
					require.Equal(t, 1, numCold, "the cold call must have cached its module")
					require.Equal(t, 1, savesCold)

					budgets := []uint64{equivalenceGasBudget}
					minimum := uint64(0)
					if call.function != "klc3001NotExported" {
						require.Equal(t, vmcommon.Ok, cold.output.ReturnCode, cold.output.ReturnMessage)
						gasUsed := equivalenceGasBudget - cold.output.GasRemaining
						minimum = minimumSuccessfulBudget(t, code, call, fk.epochs, schedule, gasUsed)
						budgets = append(budgets, gasUsed, minimum, minimum-1)
						t.Logf("gas used %d, minimum successful budget %d", gasUsed, minimum)
					}

					for _, budget := range budgets {
						coldRun := runEquivalenceCall(t, code, call, fk.epochs, schedule, newModuleStore(), budget)

						warmRunStore := warmStore.clone()
						warmRun := runEquivalenceCall(t, code, call, fk.epochs, schedule, warmRunStore, budget)
						_, warmSaves := warmRunStore.snapshot()
						require.Equal(t, 0, warmSaves, "budget %d: the call must restore the warmed module, not compile", budget)

						cachedRunStore := coldCachedStore.clone()
						cachedRun := runEquivalenceCall(t, code, call, fk.epochs, schedule, cachedRunStore, budget)
						_, cachedSaves := cachedRunStore.snapshot()
						require.Equal(t, 0, cachedSaves, "budget %d: the call must restore the cold-cached module", budget)

						requireSameResult(t, coldRun, warmRun, "warmed vs cold")
						requireSameResult(t, cachedRun, warmRun, "warmed vs cold-cached")
					}

					if minimum > 0 {
						// the boundary is meaningful on the warmed path too
						atLimit := runEquivalenceCall(t, code, call, fk.epochs, schedule, warmStore.clone(), minimum)
						require.Equal(t, vmcommon.Ok, atLimit.output.ReturnCode, "the minimum budget must succeed")
						belowLimit := runEquivalenceCall(t, code, call, fk.epochs, schedule, warmStore.clone(), minimum-1)
						require.Equal(t, vmcommon.VMOutOfGas, belowLimit.output.ReturnCode, "one unit less must run out of gas")
					}
				})
			}
		}
	}
}
