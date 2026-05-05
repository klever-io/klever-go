package interceptors_test

import (
	"errors"
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

	processReceivedMessageSingleDataInvalidVersion(t, process.ErrInvalidTransactionVersion)
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
