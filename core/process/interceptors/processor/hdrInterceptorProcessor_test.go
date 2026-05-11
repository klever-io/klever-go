package processor_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/interceptors/processor"
	processmock "github.com/klever-io/klever-go/core/process/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/stretchr/testify/require"
)

// hdrValidatorStub is the minimal stub satisfying both process.InterceptedData
// (the static type Save accepts) and process.HdrValidatorHandler (the dynamic
// type Save asserts to).
type hdrValidatorStub struct {
	hash   []byte
	header data.HeaderHandler
}

var _ process.InterceptedData = (*hdrValidatorStub)(nil)
var _ process.HdrValidatorHandler = (*hdrValidatorStub)(nil)

func (s *hdrValidatorStub) Hash() []byte                      { return s.hash }
func (s *hdrValidatorStub) HeaderHandler() data.HeaderHandler { return s.header }
func (s *hdrValidatorStub) CheckValidity() error              { return nil }
func (s *hdrValidatorStub) Type() string                      { return "hdr-validator-stub" }
func (s *hdrValidatorStub) Identifiers() [][]byte             { return nil }
func (s *hdrValidatorStub) String() string                    { return "hdr-validator-stub" }
func (s *hdrValidatorStub) IsInterfaceNil() bool              { return s == nil }

func newHdrInterceptorProcessorForTest(t *testing.T) *processor.HdrInterceptorProcessor {
	t.Helper()
	hip, err := processor.NewHdrInterceptorProcessor(&processor.ArgHdrInterceptorProcessor{
		Headers:        &mock.HeadersCacherStub{},
		BlockBlackList: &processmock.TimeCacheStub{},
	})
	require.NoError(t, err)
	return hip
}

//------- regression: missing recover() on goroutine spawned by Save (Finding 4.1)

// A panicking registered handler must not abort the process. Pre-fix, the
// goroutine spawned by Save at hdrInterceptorProcessor.go:71 had no
// defer-recover; any panic propagated out of the goroutine and the Go runtime
// killed the process. Post-fix, the panic is caught inside notify and logged.
func TestHdrInterceptorProcessor_Save_HandlerPanic_DoesNotCrashProcess(t *testing.T) {
	t.Parallel()

	hip := newHdrInterceptorProcessorForTest(t)

	invoked := make(chan struct{})
	hip.RegisterHandler(func(_ string, _ []byte, _ interface{}) {
		// Signal entry BEFORE panicking so the test can wait deterministically
		// (the deferred recover in notify will still fire on the panic below).
		close(invoked)
		panic("regression: handler that panics must not crash the process")
	})

	stub := &hdrValidatorStub{hash: []byte("hash"), header: &mock.HeaderHandlerStub{}}

	require.NoError(t, hip.Save(stub, "peer", "topic"))

	select {
	case <-invoked:
		// success — handler ran, panicked, and the deferred recover() in
		// notify caught it. Pre-fix the test process would already be dead.
	case <-time.After(2 * time.Second):
		t.Fatal("regression: panicking handler was never invoked within 2s; " +
			"the spawned notify goroutine may be missing or wired incorrectly")
	}
}

//------- regression: per-handler failure isolation (F1 refactor)

// A panic in handler[i] must NOT skip handler[i+1..N-1]. The initial F1 fix
// wrapped the entire notify() for-loop in a single recover, which meant the
// first panicking handler unwound out of notify and every later handler in
// the snapshot was silently never invoked — a behavior change disguised as a
// safety fix. The follow-up refactor moved the recover boundary to
// invokeHandlerSafely (one per handler), so a panic is contained to a single
// invocation and the loop continues.
//
// This test locks down that invariant: if anyone collapses the per-handler
// recover back to a notify-wide one, this test will fail because handler[1]
// never runs.
func TestHdrInterceptorProcessor_Save_HandlerPanic_DoesNotSkipSubsequentHandlers(t *testing.T) {
	t.Parallel()

	hip := newHdrInterceptorProcessorForTest(t)

	handler0Ran := make(chan struct{})
	handler1Ran := make(chan struct{})

	// Registration order = invocation order (notify iterates the snapshot in
	// append order), so handler[0] panics first; handler[1] is the one that
	// must still run despite handler[0]'s panic.
	hip.RegisterHandler(func(_ string, _ []byte, _ interface{}) {
		close(handler0Ran)
		panic("regression: panic in handler[0] must not skip handler[1]")
	})
	hip.RegisterHandler(func(_ string, _ []byte, _ interface{}) {
		close(handler1Ran)
	})

	stub := &hdrValidatorStub{hash: []byte("hash"), header: &mock.HeaderHandlerStub{}}
	require.NoError(t, hip.Save(stub, "peer", "topic"))

	// Sanity: handler[0] must actually have been invoked, otherwise the test
	// isn't exercising the panic path at all (e.g., the notify goroutine
	// failed to spawn).
	select {
	case <-handler0Ran:
	case <-time.After(2 * time.Second):
		t.Fatal("setup: handler[0] was never invoked; the notify goroutine may be missing or wired incorrectly")
	}

	// The actual regression assertion: handler[1] MUST run despite handler[0]
	// panicking. This is the failure-isolation property the per-handler
	// recover in invokeHandlerSafely guarantees.
	select {
	case <-handler1Ran:
		// success — per-handler recover contained the panic; loop continued
	case <-time.After(2 * time.Second):
		t.Fatal("regression: panic in handler[0] caused handler[1] to be skipped — " +
			"the recover boundary is at the wrong level (notify-wide instead of per-handler)")
	}
}

//------- regression: lock held across user callback in notify (Finding 4.3)

// A registered handler that itself calls RegisterHandler must not deadlock.
// Pre-fix, notify held mutHandlers.RLock across the handler call; the inner
// RegisterHandler tried to acquire mutHandlers.Lock() on the same goroutine,
// blocking forever because the RLock could not be released until the handler
// returned. Post-fix, notify snapshots the handler list and releases the
// RLock before invoking handlers.
//
// In addition to the deadlock property, this test asserts the snapshot
// semantics that the fix relies on:
//  1. A handler registered DURING notify() must NOT fire on the same Save
//     (the snapshot was taken before the for-loop).
//  2. A subsequent Save MUST invoke the recursively-registered handler
//     (RegisterHandler must have actually appended; it must not silently
//     no-op when called from inside notify).
func TestHdrInterceptorProcessor_Save_HandlerCallingRegisterHandler_NoDeadlock(t *testing.T) {
	t.Parallel()

	hip := newHdrInterceptorProcessorForTest(t)

	completed := make(chan struct{})
	innerInvoked := make(chan struct{})
	var (
		innerInvokeCount  atomic.Int32
		completedOnce     sync.Once
		registerInnerOnce sync.Once
	)

	hip.RegisterHandler(func(_ string, _ []byte, _ interface{}) {
		// Idempotent: the outer handler runs on EVERY Save, including the
		// follow-up Save below. We only want to register the inner handler
		// and close `completed` on the first invocation.
		registerInnerOnce.Do(func() {
			hip.RegisterHandler(func(_ string, _ []byte, _ interface{}) {
				if innerInvokeCount.Add(1) == 1 {
					close(innerInvoked)
				}
			})
		})
		completedOnce.Do(func() { close(completed) })
	})

	stub := &hdrValidatorStub{hash: []byte("hash"), header: &mock.HeaderHandlerStub{}}

	require.NoError(t, hip.Save(stub, "peer", "topic"))

	select {
	case <-completed:
		// success — recursive RegisterHandler returned, no deadlock
	case <-time.After(2 * time.Second):
		t.Fatal("regression: recursive RegisterHandler from inside notify deadlocked " +
			"(RLock held across user callback)")
	}

	// Snapshot semantics #1: the inner handler must not have fired on this Save.
	// Allow a brief grace window for any straggling goroutine; if the snapshot
	// is correct, no goroutine will ever invoke it for this Save.
	select {
	case <-innerInvoked:
		t.Fatal("regression: handler registered during notify() fired on the SAME Save — " +
			"snapshot semantics broken (handlers slice was iterated post-RegisterHandler)")
	case <-time.After(100 * time.Millisecond):
		// expected — the snapshot was taken before the loop, so the inner
		// handler is not in the snapshot for this Save
	}

	// Snapshot semantics #2: the inner handler MUST be invoked on a follow-up
	// Save (proves RegisterHandler actually appended and didn't silently
	// no-op when called from inside notify).
	require.NoError(t, hip.Save(stub, "peer", "topic"))
	select {
	case <-innerInvoked:
		// success — the recursively-registered handler is now in the snapshot
	case <-time.After(2 * time.Second):
		t.Fatal("regression: recursively-registered handler never fired on a follow-up Save — " +
			"RegisterHandler may be silently no-op'ing when called from inside notify")
	}
}
