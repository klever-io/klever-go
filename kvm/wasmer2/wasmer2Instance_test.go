package wasmer2

import (
	"encoding/hex"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newLiveInstance compiles a minimal WASM module into a real native instance, so
// Clean actually runs cWasmerInstanceDestroy. Tests that only assert the cleaned
// guards (returning zero/error without touching cgo) do not need this; the ones
// exercising real destruction, idempotency and the concurrent Clean/accessor race do.
func newLiveInstance(t *testing.T) *Wasmer2Instance {
	t.Helper()

	exec, err := CreateExecutor()
	require.NoError(t, err)

	code, err := os.ReadFile("../test/contracts/answer/output/answer.wasm")
	require.NoError(t, err)

	inst, err := exec.NewInstanceWithOptions(code, executor.CompilationOptions{
		GasLimit:           100_000_000,
		RuntimeBreakpoints: true,
		// MaxDeclaredTableSize's Go zero value is 0, not "unlimited" - unlike
		// the other CompilationOptions fields, an omitted zero value here
		// would reject any table declaration at all, not merely fail to cap
		// one. answer.wasm happens to declare no table today, so this would
		// stay silently correct either way, but that is not something a
		// future test fixture should have to rely on.
		MaxDeclaredTableSize: math.MaxUint64,
	})
	require.NoError(t, err)

	instance, ok := inst.(*Wasmer2Instance)
	require.True(t, ok)
	require.False(t, instance.IsAlreadyCleaned())

	return instance
}

// assertCleanedInstanceIsInert verifies that every native accessor short-circuits on
// its cleaned guard, returning the documented zero/error value instead of dereferencing
// the (nil) cgo pointer. It exercises the guard branches only; the concurrency and
// destruction behaviour that actually protect against the use-after-free are covered by
// the live-instance tests below.
func assertCleanedInstanceIsInert(t *testing.T, instance *Wasmer2Instance) {
	t.Helper()

	// Setters must be no-ops (and must not panic on the nil native pointer).
	instance.SetGasLimit(1)
	instance.SetPointsUsed(1)
	instance.SetBreakpointValue(1)

	assert.Zero(t, instance.GetPointsUsed())
	assert.Zero(t, instance.GetBreakpointValue())
	assert.Zero(t, instance.MemLength())
	assert.Zero(t, instance.MaxDeclaredTableSize())
	assert.Nil(t, instance.MemDump())

	cacheBytes, err := instance.Cache()
	assert.Nil(t, cacheBytes)
	assert.ErrorIs(t, err, ErrInstanceCleaned)

	memLoad, err := instance.MemLoad(executor.MemPtr(0), executor.MemLength(0))
	assert.Nil(t, memLoad)
	assert.ErrorIs(t, err, ErrInstanceCleaned)

	assert.ErrorIs(t, instance.MemStore(executor.MemPtr(0), []byte{0x01}), ErrInstanceCleaned)
	assert.ErrorIs(t, instance.MemGrow(1), ErrInstanceCleaned)

	// Function introspection must not hand a NULL native pointer to the FFI either.
	assert.False(t, instance.HasFunction("anything"))
	assert.Nil(t, instance.GetFunctionNames())
	assert.ErrorIs(t, instance.ValidateFunctionArities(), ErrInstanceCleaned)

	// Reset and Clean must both report "nothing to do" without touching cgo.
	assert.False(t, instance.Reset())
	assert.False(t, instance.Clean())
}

func TestWasmer2Instance_AccessorsAreInertAfterClean(t *testing.T) {
	t.Parallel()

	t.Run("cleaned flag set", func(t *testing.T) {
		t.Parallel()

		// Mirrors the post-Clean state: flag set and native pointers cleared.
		instance := &Wasmer2Instance{
			alreadyClean: true,
			id:           "cleaned-instance",
		}

		assert.True(t, instance.IsAlreadyCleaned())
		assertCleanedInstanceIsInert(t, instance)
	})

	t.Run("nil native pointer without flag", func(t *testing.T) {
		t.Parallel()

		// Exercises the `cgoInstance == nil` half of every guard independently of the
		// cleaned flag, e.g. an instance cleared by a concurrent Clean.
		instance := &Wasmer2Instance{
			alreadyClean: false,
			cgoInstance:  nil,
			id:           "nil-native-instance",
		}

		assert.False(t, instance.IsAlreadyCleaned())
		assertCleanedInstanceIsInert(t, instance)
	})
}

func TestWasmer2Instance_CleanDestroysOnceThenIsIdempotent(t *testing.T) {
	t.Parallel()

	instance := newLiveInstance(t)

	// The first Clean performs the real cWasmerInstanceDestroy.
	assert.True(t, instance.Clean())
	assert.True(t, instance.IsAlreadyCleaned())

	// Every further Clean must short-circuit on the guard: no second destroy
	// (the double-free) and a false return so the tracker does not decrement twice.
	assert.False(t, instance.Clean())
	assert.False(t, instance.Clean())
}

func TestWasmer2Instance_IDSurvivesClean(t *testing.T) {
	t.Parallel()

	// The id captured at construction is the instance-tracker map key and must
	// survive Clean clearing the native pointer. Assert it across a real Clean,
	// not by writing the fields by hand.
	instance := newLiveInstance(t)

	idBeforeClean := instance.ID()
	require.NotEmpty(t, idBeforeClean)

	require.True(t, instance.Clean())
	assert.Equal(t, idBeforeClean, instance.ID())
}

func TestWasmer2Instance_ConcurrentCleanAndAccessors(t *testing.T) {
	t.Parallel()

	// Concurrent accessors racing Clean must never dereference the freed native
	// pointer. The mutex serializes them and each accessor re-checks the cleaned
	// state under the lock, so a caller sees either a live result or the documented
	// zero value — never a half-destroyed instance. Without the lock this races the
	// native pointer and crashes (SIGSEGV / use-after-free). Additionally, exactly one
	// of several concurrent Clean() calls may win the destruction, which is what
	// instanceTracker.updateNumRunningInstances(-1) relies on.
	instance := newLiveInstance(t)

	const (
		readers    = 16
		iterations = 200
		cleaners   = 4
	)

	var wg sync.WaitGroup
	start := make(chan struct{})

	for range readers {
		wg.Go(func() {
			<-start
			for range iterations {
				// Stop once the instance is observed cleaned to keep the post-clean
				// guard-log volume bounded; the calls that race the actual destruction
				// (between this check and Clean winning) are still exercised.
				if instance.IsAlreadyCleaned() {
					return
				}
				instance.GetPointsUsed()
				instance.GetBreakpointValue()
				instance.SetPointsUsed(1)
				instance.SetBreakpointValue(1)
				instance.MemLength()
				instance.HasFunction("answer")
				instance.ValidateFunctionArities()
				instance.Reset()
			}
		})
	}

	var cleanWins int32
	for range cleaners {
		wg.Go(func() {
			<-start
			if instance.Clean() {
				atomic.AddInt32(&cleanWins, 1)
			}
		})
	}

	close(start)
	wg.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&cleanWins), "exactly one Clean must perform the destruction")
	assert.True(t, instance.IsAlreadyCleaned())
}

// tableAtCapWasmHex declares a funcref table with min 0 and max 1,000. It
// gives MaxDeclaredTableSize a non-zero value to report, which a module
// without a table section could not: zero is also what the cleaned-instance
// guard returns, so a fixture without a table would pass this test whether or
// not the cgo bridge worked.
const tableAtCapWasmHex = "0061736d0100000001040160000003020100040601700100e8070503010001071102066d656d6f7279020004696e697400000a040102000b"

// TestWasmer2Instance_MaxDeclaredTableSizeReadsThroughCgo covers the live path
// of MaxDeclaredTableSize, and with it cWasmerInstanceMaxDeclaredTableSize in
// bridge2.go. The existing cleaned-instance assertion only reaches the guard
// that returns zero before touching cgo, so without this the bridge function
// is never actually called from a test in its own package.
func TestWasmer2Instance_MaxDeclaredTableSizeReadsThroughCgo(t *testing.T) {
	exec, err := CreateExecutor()
	require.NoError(t, err)

	code, err := hex.DecodeString(tableAtCapWasmHex)
	require.NoError(t, err)

	instance, err := exec.NewInstanceWithOptions(code, executor.CompilationOptions{
		GasLimit:             100_000_000,
		RuntimeBreakpoints:   true,
		MaxDeclaredTableSize: math.MaxUint64,
	})
	require.NoError(t, err)
	defer instance.Clean()

	require.Equal(t, uint32(1000), instance.MaxDeclaredTableSize(),
		"the declared maximum must come back through the cgo bridge unchanged")
}
