package interceptors_test

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/interceptors"
	"github.com/klever-io/klever-go/core/throttler"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createMockArgSingleDataInterceptor() interceptors.ArgSingleDataInterceptor {
	return interceptors.ArgSingleDataInterceptor{
		Topic:            "test topic",
		DataFactory:      &mock.InterceptedDataFactoryStub{},
		Processor:        &mock.InterceptorProcessorStub{},
		Throttler:        createMockThrottler(),
		AntifloodHandler: &mock.P2PAntifloodHandlerStub{},
		WhiteListRequest: &mock.WhiteListHandlerStub{},
		CurrentPeerID:    "pid",
	}
}

func createMockInterceptorStub(checkCalledNum *int32, processCalledNum *int32) process.InterceptorProcessor {
	return &mock.InterceptorProcessorStub{
		ValidateCalled: func(data process.InterceptedData) error {
			if checkCalledNum != nil {
				atomic.AddInt32(checkCalledNum, 1)
			}

			return nil
		},
		SaveCalled: func(data process.InterceptedData) error {
			if processCalledNum != nil {
				atomic.AddInt32(processCalledNum, 1)
			}

			return nil
		},
	}
}

func createMockThrottler() *mock.InterceptorThrottlerStub {
	return &mock.InterceptorThrottlerStub{
		CanProcessCalled: func() bool {
			return true
		},
	}
}

func TestNewSingleDataInterceptor_EmptyTopicShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgSingleDataInterceptor()
	arg.Topic = ""
	sdi, err := interceptors.NewSingleDataInterceptor(arg)

	assert.Nil(t, sdi)
	assert.Equal(t, process.ErrEmptyTopic, err)
}

func TestNewSingleDataInterceptor_NilInterceptedDataFactoryShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgSingleDataInterceptor()
	arg.DataFactory = nil
	sdi, err := interceptors.NewSingleDataInterceptor(arg)

	assert.Nil(t, sdi)
	assert.Equal(t, process.ErrNilInterceptedDataFactory, err)
}

func TestNewSingleDataInterceptor_NilInterceptedDataProcessorShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgSingleDataInterceptor()
	arg.Processor = nil
	sdi, err := interceptors.NewSingleDataInterceptor(arg)

	assert.Nil(t, sdi)
	assert.Equal(t, process.ErrNilInterceptedDataProcessor, err)
}

func TestNewSingleDataInterceptor_NilInterceptorThrottlerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgSingleDataInterceptor()
	arg.Throttler = nil
	sdi, err := interceptors.NewSingleDataInterceptor(arg)

	assert.Nil(t, sdi)
	assert.Equal(t, process.ErrNilInterceptorThrottler, err)
}

func TestNewSingleDataInterceptor_NilP2PAntifloodHandlerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgSingleDataInterceptor()
	arg.AntifloodHandler = nil
	sdi, err := interceptors.NewSingleDataInterceptor(arg)

	assert.Nil(t, sdi)
	assert.Equal(t, process.ErrNilAntifloodHandler, err)
}

func TestNewSingleDataInterceptor_NilWhiteListHandlerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgSingleDataInterceptor()
	arg.WhiteListRequest = nil
	sdi, err := interceptors.NewSingleDataInterceptor(arg)

	assert.Nil(t, sdi)
	assert.Equal(t, process.ErrNilWhiteListHandler, err)
}

func TestNewSingleDataInterceptor_EmptyPeerIDShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgSingleDataInterceptor()
	arg.CurrentPeerID = ""
	sdi, err := interceptors.NewSingleDataInterceptor(arg)

	assert.Nil(t, sdi)
	assert.Equal(t, process.ErrEmptyPeerID, err)
}

func TestNewSingleDataInterceptor(t *testing.T) {
	t.Parallel()

	arg := createMockArgSingleDataInterceptor()
	sdi, err := interceptors.NewSingleDataInterceptor(arg)

	require.False(t, check.IfNil(sdi))
	require.Nil(t, err)
	assert.Equal(t, arg.Topic, sdi.Topic())
}

//------- ProcessReceivedMessage

func TestSingleDataInterceptor_ProcessReceivedMessageNilMessageShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgSingleDataInterceptor()
	sdi, _ := interceptors.NewSingleDataInterceptor(arg)

	err := sdi.ProcessReceivedMessage(nil, fromConnectedPeerID)

	assert.Equal(t, common.ErrNilMessage, err)
}

func TestSingleDataInterceptor_ProcessReceivedMessageFactoryCreationErrorShouldErr(t *testing.T) {
	t.Parallel()

	errExpected := errors.New("expected error")
	originatorPid := core.PeerID("originator")
	originatorBlackListed := false
	fromConnectedPeerBlackListed := false
	arg := createMockArgSingleDataInterceptor()
	throttler := createMockThrottler()
	arg.Throttler = throttler
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(buff []byte) (data process.InterceptedData, e error) {
			return nil, errExpected
		},
	}
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		BlacklistPeerCalled: func(peer core.PeerID, reason string, duration time.Duration) {
			if peer == originatorPid {
				originatorBlackListed = true
			}
			if peer == fromConnectedPeerID {
				fromConnectedPeerBlackListed = true
			}
		},
	}
	sdi, _ := interceptors.NewSingleDataInterceptor(arg)

	msg := &mock.P2PMessageMock{
		DataField: []byte("data to be processed"),
		PeerField: originatorPid,
	}
	err := sdi.ProcessReceivedMessage(msg, fromConnectedPeerID)

	assert.Equal(t, errExpected, err)
	assert.True(t, originatorBlackListed)
	assert.True(t, fromConnectedPeerBlackListed)
	// Regression GHSA-74m6-4hjp-7226 / KLC-2348: every synchronous error path
	// after preProcessMessage must release the throttler slot exactly once.
	assert.Equal(t, int32(1), throttler.StartProcessingCount())
	assert.Equal(t, int32(1), throttler.EndProcessingCount())
}

func TestSingleDataInterceptor_ProcessReceivedMessageIsNotValidShouldNotCallProcess(t *testing.T) {
	t.Parallel()

	errExpected := errors.New("expected error")
	testProcessReceiveMessage(t, errExpected, 0)
}

func TestSingleDataInterceptor_ProcessReceivedMessageShouldWork(t *testing.T) {
	t.Parallel()

	testProcessReceiveMessage(t, nil, 1)
}

func testProcessReceiveMessage(t *testing.T, validityErr error, calledNum int) {
	checkCalledNum := int32(0)
	processCalledNum := int32(0)
	throttler := createMockThrottler()
	interceptedData := &mock.InterceptedDataStub{
		CheckValidityCalled: func() error {
			return validityErr
		},
	}

	arg := createMockArgSingleDataInterceptor()
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(buff []byte) (data process.InterceptedData, e error) {
			return interceptedData, nil
		},
	}
	arg.Processor = createMockInterceptorStub(&checkCalledNum, &processCalledNum)
	arg.Throttler = throttler
	sdi, _ := interceptors.NewSingleDataInterceptor(arg)

	msg := &mock.P2PMessageMock{
		DataField: []byte("data to be processed"),
	}
	err := sdi.ProcessReceivedMessage(msg, fromConnectedPeerID)

	time.Sleep(time.Second)

	assert.Equal(t, validityErr, err)
	assert.Equal(t, int32(calledNum), atomic.LoadInt32(&checkCalledNum))
	assert.Equal(t, int32(calledNum), atomic.LoadInt32(&processCalledNum))
	assert.Equal(t, int32(1), throttler.EndProcessingCount())
	assert.Equal(t, int32(1), throttler.EndProcessingCount())
}

func TestSingleDataInterceptor_ProcessReceivedMessageWhitelistedShouldWork(t *testing.T) {
	t.Parallel()

	checkCalledNum := int32(0)
	processCalledNum := int32(0)
	throttler := createMockThrottler()
	interceptedData := &mock.InterceptedDataStub{
		CheckValidityCalled: func() error {
			return nil
		},
	}

	arg := createMockArgSingleDataInterceptor()
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(buff []byte) (data process.InterceptedData, e error) {
			return interceptedData, nil
		},
	}
	arg.Processor = createMockInterceptorStub(&checkCalledNum, &processCalledNum)
	arg.Throttler = throttler
	arg.WhiteListRequest = &mock.WhiteListHandlerStub{
		IsWhiteListedCalled: func(interceptedData process.InterceptedData) bool {
			return true
		},
	}
	sdi, _ := interceptors.NewSingleDataInterceptor(arg)

	msg := &mock.P2PMessageMock{
		DataField: []byte("data to be processed"),
	}
	err := sdi.ProcessReceivedMessage(msg, fromConnectedPeerID)

	time.Sleep(time.Second)

	assert.Nil(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&checkCalledNum))
	assert.Equal(t, int32(1), atomic.LoadInt32(&processCalledNum))
	assert.Equal(t, int32(1), throttler.EndProcessingCount())
	assert.Equal(t, int32(1), throttler.EndProcessingCount())
}

func TestSingleDataInterceptor_InvalidTxVersionShouldBlackList(t *testing.T) {
	t.Parallel()

	processReceivedMessageSingleDataInvalidVersion(t, process.ErrInvalidTransactionVersion)
}

func TestSingleDataInterceptor_InvalidTxChainIDShouldBlackList(t *testing.T) {
	t.Parallel()

	processReceivedMessageSingleDataInvalidVersion(t, process.ErrInvalidChainID)
}

func TestSingleDataInterceptor_WrappedInvalidTxVersionShouldBlackList(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("intercepted-tx: %w", process.ErrInvalidTransactionVersion)
	processReceivedMessageSingleDataInvalidVersion(t, wrapped)
}

func TestSingleDataInterceptor_WrappedInvalidChainIDShouldBlackList(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("intercepted-tx: %w", process.ErrInvalidChainID)
	processReceivedMessageSingleDataInvalidVersion(t, wrapped)
}

func processReceivedMessageSingleDataInvalidVersion(t *testing.T, expectedErr error) {
	checkCalledNum := int32(0)
	processCalledNum := int32(0)
	throttler := createMockThrottler()
	interceptedData := &mock.InterceptedDataStub{
		CheckValidityCalled: func() error {
			return expectedErr
		},
	}

	isOriginatorBlackListed := false
	isFromConnectedPeerBlackListed := false
	originator := core.PeerID("originator")
	arg := createMockArgSingleDataInterceptor()
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(buff []byte) (data process.InterceptedData, e error) {
			return interceptedData, nil
		},
	}
	arg.Processor = createMockInterceptorStub(&checkCalledNum, &processCalledNum)
	arg.Throttler = throttler
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		BlacklistPeerCalled: func(peer core.PeerID, reason string, duration time.Duration) {
			switch string(peer) {
			case string(originator):
				isOriginatorBlackListed = true
			case string(fromConnectedPeerID):
				isFromConnectedPeerBlackListed = true
			}
		},
	}
	arg.WhiteListRequest = &mock.WhiteListHandlerStub{
		IsWhiteListedCalled: func(interceptedData process.InterceptedData) bool {
			return true
		},
	}
	sdi, _ := interceptors.NewSingleDataInterceptor(arg)

	msg := &mock.P2PMessageMock{
		DataField: []byte("data to be processed"),
		PeerField: originator,
	}
	err := sdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
	assert.Equal(t, expectedErr, err)
	assert.True(t, isFromConnectedPeerBlackListed)
	assert.True(t, isOriginatorBlackListed)
}

func TestSingleDataInterceptor_ProcessReceivedMessageWithOriginator(t *testing.T) {
	t.Parallel()

	checkCalledNum := int32(0)
	processCalledNum := int32(0)
	throttler := createMockThrottler()
	interceptedData := &mock.InterceptedDataStub{
		CheckValidityCalled: func() error {
			return nil
		},
	}

	whiteListHandler := &mock.WhiteListHandlerStub{
		IsWhiteListedCalled: func(interceptedData process.InterceptedData) bool {
			return true
		},
	}
	arg := createMockArgSingleDataInterceptor()
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(buff []byte) (data process.InterceptedData, e error) {
			return interceptedData, nil
		},
	}
	arg.Processor = createMockInterceptorStub(&checkCalledNum, &processCalledNum)
	arg.Throttler = throttler
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		IsOriginatorElectedForTopicCalled: func(pid core.PeerID, topic string) error {
			return process.ErrOnlyValidatorsCanUseThisTopic
		},
	}
	arg.WhiteListRequest = whiteListHandler
	sdi, _ := interceptors.NewSingleDataInterceptor(arg)

	msg := &mock.P2PMessageMock{
		DataField: []byte("data to be processed"),
	}
	err := sdi.ProcessReceivedMessage(msg, fromConnectedPeerID)

	time.Sleep(time.Second)

	assert.Nil(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&checkCalledNum))
	assert.Equal(t, int32(1), atomic.LoadInt32(&processCalledNum))
	assert.Equal(t, int32(1), throttler.EndProcessingCount())
	assert.Equal(t, int32(1), throttler.EndProcessingCount())

	whiteListHandler.IsWhiteListedCalled = func(interceptedData process.InterceptedData) bool {
		return false
	}

	err = sdi.ProcessReceivedMessage(msg, fromConnectedPeerID)

	time.Sleep(time.Second)

	assert.Equal(t, err, process.ErrOnlyValidatorsCanUseThisTopic)
	assert.Equal(t, int32(1), atomic.LoadInt32(&checkCalledNum))
	assert.Equal(t, int32(1), atomic.LoadInt32(&processCalledNum))
	assert.Equal(t, int32(2), throttler.EndProcessingCount())
	assert.Equal(t, int32(2), throttler.EndProcessingCount())
}

//------- debug

func TestSingleDataInterceptor_SetInterceptedDebugHandlerNilShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgSingleDataInterceptor()
	sdi, _ := interceptors.NewSingleDataInterceptor(arg)

	err := sdi.SetInterceptedDebugHandler(nil)

	assert.Equal(t, process.ErrNilDebugger, err)
}

func TestSingleDataInterceptor_SetInterceptedDebugHandlerShouldWork(t *testing.T) {
	t.Parallel()

	arg := createMockArgSingleDataInterceptor()
	sdi, _ := interceptors.NewSingleDataInterceptor(arg)

	debugger := &mock.InterceptedDebugHandlerStub{}
	err := sdi.SetInterceptedDebugHandler(debugger)

	assert.Nil(t, err)
	assert.True(t, debugger == sdi.InterceptedDebugHandler()) //pointer testing
}

//------- IsInterfaceNil

func TestSingleDataInterceptor_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	var sdi *interceptors.SingleDataInterceptor

	assert.True(t, check.IfNil(sdi))
}

//------- regression: GHSA-74m6-4hjp-7226 / KLC-2348 (defensive hardening of SingleDataInterceptor)

// Mirror of the MultiDataInterceptor regression test: repeated synchronous
// error returns (here via factory.Create errors) on a real bounded throttler
// must not leak slots. On code where a synchronous error branch fails to
// release the slot, the third attempt would return common.ErrSystemBusy.
func TestSingleDataInterceptor_ProcessReceivedMessage_RepeatedFactoryErrorsMustNotExhaustThrottler(t *testing.T) {
	t.Parallel()

	errExpected := errors.New("expected error")

	const throttlerCapacity int32 = 2
	realThrottler, err := throttler.NewNumGoRoutinesThrottler(throttlerCapacity)
	require.NoError(t, err)

	arg := createMockArgSingleDataInterceptor()
	arg.Throttler = realThrottler
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(buff []byte) (process.InterceptedData, error) {
			return nil, errExpected
		},
	}

	sdi, err := interceptors.NewSingleDataInterceptor(arg)
	require.NoError(t, err)

	const attempts = 5
	for i := 0; i < attempts; i++ {
		msg := &mock.P2PMessageMock{
			DataField: []byte("data to be processed"),
			PeerField: core.PeerID("origin-peer"),
		}
		processErr := sdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
		require.Equal(t, errExpected, processErr, "attempt %d", i)
		require.Falsef(t, errors.Is(processErr, common.ErrSystemBusy),
			"regression GHSA-74m6-4hjp-7226 / KLC-2348: throttler exhausted on iteration %d: %v", i, processErr)
	}
}

//------- regression: data race on bdi.debugHandler from worker goroutine (Finding 4.2)

// SingleDataInterceptor.ProcessReceivedMessage previously held mutDebugHandler.RLock
// only for the synchronous frame; the worker goroutine spawned at the end read
// bdi.debugHandler after the lock had already been released. Concurrent
// SetInterceptedDebugHandler calls must not race with that worker-side read.
// Run with `go test -race` to enforce.
//
// The validate/save/whitelist stubs all return nil/true so ProcessReceivedMessage
// reaches the success path that spawns the worker goroutine and ultimately
// dispatches LogProcessedHashes through bdi.debugHandler — that is precisely
// the read site the rotation goroutine races with. Changing any of those mocks
// to err-return would silently un-cover the race.
func TestSingleDataInterceptor_ProcessReceivedMessage_DebugHandlerRotation_NoRace(t *testing.T) {
	t.Parallel()

	interceptedData := &mock.InterceptedDataStub{
		CheckValidityCalled: func() error { return nil },
		IdentifiersCalled:   func() [][]byte { return [][]byte{[]byte("id-0")} },
	}

	arg := createMockArgSingleDataInterceptor()
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(_ []byte) (process.InterceptedData, error) { return interceptedData, nil },
	}
	arg.Processor = &mock.InterceptorProcessorStub{
		ValidateCalled: func(_ process.InterceptedData) error { return nil },
		SaveCalled:     func(_ process.InterceptedData) error { return nil },
	}
	arg.WhiteListRequest = &mock.WhiteListHandlerStub{
		IsWhiteListedCalled: func(_ process.InterceptedData) bool { return true },
	}

	sdi, err := interceptors.NewSingleDataInterceptor(arg)
	require.NoError(t, err)

	// workersWG tracks the spawned async LogProcessedHashes calls so we can
	// deterministically wait for every worker goroutine's debugHandler read
	// to complete before exiting (replaces the previous timing-based sleep).
	var workersWG sync.WaitGroup

	logProcessed := func(_ string, _ [][]byte, _ error) {
		workersWG.Done()
	}
	dh1 := &mock.InterceptedDebugHandlerStub{
		LogProcessedHashesCalled: logProcessed,
		LogReceivedHashesCalled:  func(_ string, _ [][]byte) {},
	}
	dh2 := &mock.InterceptedDebugHandlerStub{
		LogProcessedHashesCalled: logProcessed,
		LogReceivedHashesCalled:  func(_ string, _ [][]byte) {},
	}

	// One-time sanity check that the rotation API itself works before we
	// drown its return value in the race loop. Without this, a regression
	// that made SetInterceptedDebugHandler always-error would silently turn
	// every iteration below into a no-op rotation and leave the race
	// detector with nothing to observe (false negative).
	require.NoError(t, sdi.SetInterceptedDebugHandler(dh1),
		"SetInterceptedDebugHandler must accept a valid handler — sanity check")

	const iterations = 1000
	done := make(chan struct{}, 2)

	go func() {
		for i := 0; i < iterations; i++ {
			workersWG.Add(1) // optimistic Add; refunded below on err
			msg := &mock.P2PMessageMock{
				DataField:  []byte("any-non-empty-payload"),
				PeerField:  core.PeerID("origin-peer"),
				SeqNoField: []byte("seq"),
			}
			if procErr := sdi.ProcessReceivedMessage(msg, core.PeerID("from-peer")); procErr != nil {
				// No worker goroutine was spawned for this iteration, so the
				// LogProcessedHashes Done() will never fire — refund the Add.
				workersWG.Done()
			}
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < iterations; i++ {
			if i%2 == 0 {
				_ = sdi.SetInterceptedDebugHandler(dh1)
			} else {
				_ = sdi.SetInterceptedDebugHandler(dh2)
			}
		}
		done <- struct{}{}
	}()

	<-done
	<-done
	// Block until every spawned worker has executed its LogProcessedHashes
	// callback (i.e. its debugHandler read has happened) so the race detector
	// has had a chance to observe each read.
	workersWG.Wait()
}
