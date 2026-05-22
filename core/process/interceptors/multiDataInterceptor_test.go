package interceptors_test

import (
	"bytes"
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
	"github.com/klever-io/klever-go/data/batch"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fromConnectedPeerID = core.PeerID("from connected peer ID")

func createMockArgMultiDataInterceptor() interceptors.ArgMultiDataInterceptor {
	return interceptors.ArgMultiDataInterceptor{
		Topic:            "test topic",
		Marshalizer:      &mock.MarshalizerMock{},
		DataFactory:      &mock.InterceptedDataFactoryStub{},
		Processor:        &mock.InterceptorProcessorStub{},
		Throttler:        createMockThrottler(),
		AntifloodHandler: &mock.P2PAntifloodHandlerStub{},
		WhiteListRequest: &mock.WhiteListHandlerStub{},
		CurrentPeerID:    "pid",
	}
}

func TestNewMultiDataInterceptor_EmptyTopicShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgMultiDataInterceptor()
	arg.Topic = ""
	mdi, err := interceptors.NewMultiDataInterceptor(arg)

	assert.Nil(t, mdi)
	assert.Equal(t, process.ErrEmptyTopic, err)
}

func TestNewMultiDataInterceptor_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgMultiDataInterceptor()
	arg.Marshalizer = nil
	mdi, err := interceptors.NewMultiDataInterceptor(arg)

	assert.Nil(t, mdi)
	assert.Equal(t, process.ErrNilMarshalizer, err)
}

func TestNewMultiDataInterceptor_NilInterceptedDataFactoryShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgMultiDataInterceptor()
	arg.DataFactory = nil
	mdi, err := interceptors.NewMultiDataInterceptor(arg)

	assert.Nil(t, mdi)
	assert.Equal(t, process.ErrNilInterceptedDataFactory, err)
}

func TestNewMultiDataInterceptor_NilInterceptedDataProcessorShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgMultiDataInterceptor()
	arg.Processor = nil
	mdi, err := interceptors.NewMultiDataInterceptor(arg)

	assert.Nil(t, mdi)
	assert.Equal(t, process.ErrNilInterceptedDataProcessor, err)
}

func TestNewMultiDataInterceptor_NilInterceptorThrottlerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgMultiDataInterceptor()
	arg.Throttler = nil
	mdi, err := interceptors.NewMultiDataInterceptor(arg)

	assert.Nil(t, mdi)
	assert.Equal(t, process.ErrNilInterceptorThrottler, err)
}

func TestNewMultiDataInterceptor_NilAntifloodHandlerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgMultiDataInterceptor()
	arg.AntifloodHandler = nil
	mdi, err := interceptors.NewMultiDataInterceptor(arg)

	assert.Nil(t, mdi)
	assert.Equal(t, process.ErrNilAntifloodHandler, err)
}

func TestNewMultiDataInterceptor_NilWhiteListHandlerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgMultiDataInterceptor()
	arg.WhiteListRequest = nil
	mdi, err := interceptors.NewMultiDataInterceptor(arg)

	assert.Nil(t, mdi)
	assert.Equal(t, process.ErrNilWhiteListHandler, err)
}

func TestNewMultiDataInterceptor_EmptyPeerIDShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgMultiDataInterceptor()
	arg.CurrentPeerID = ""
	mdi, err := interceptors.NewMultiDataInterceptor(arg)

	assert.Nil(t, mdi)
	assert.Equal(t, process.ErrEmptyPeerID, err)
}

func TestNewMultiDataInterceptor(t *testing.T) {
	t.Parallel()

	arg := createMockArgMultiDataInterceptor()
	mdi, err := interceptors.NewMultiDataInterceptor(arg)

	require.False(t, check.IfNil(mdi))
	require.Nil(t, err)
	assert.Equal(t, arg.Topic, mdi.Topic())
}

//------- ProcessReceivedMessage

func TestMultiDataInterceptor_ProcessReceivedMessageNilMessageShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgMultiDataInterceptor()
	mdi, _ := interceptors.NewMultiDataInterceptor(arg)

	err := mdi.ProcessReceivedMessage(nil, fromConnectedPeerID)

	assert.Equal(t, common.ErrNilMessage, err)
}

func TestMultiDataInterceptor_ProcessReceivedMessageUnmarshalFailsShouldErr(t *testing.T) {
	t.Parallel()

	errExpeced := errors.New("expected error")
	originatorPid := core.PeerID("originator")
	originatorBlackListed := false
	fromConnectedPeerBlackListed := false
	arg := createMockArgMultiDataInterceptor()
	throttler := createMockThrottler()
	arg.Throttler = throttler
	arg.Marshalizer = &mock.MarshalizerStub{
		UnmarshalCalled: func(obj interface{}, buff []byte) error {
			return errExpeced
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

	mdi, _ := interceptors.NewMultiDataInterceptor(arg)

	msg := &mock.P2PMessageMock{
		DataField: []byte("data to be processed"),
		PeerField: originatorPid,
	}
	err := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)

	assert.Equal(t, errExpeced, err)
	assert.True(t, originatorBlackListed)
	assert.True(t, fromConnectedPeerBlackListed)
	// Regression GHSA-74m6-4hjp-7226 / KLC-2348: every synchronous error path
	// after preProcessMessage must release the throttler slot exactly once.
	assert.Equal(t, int32(1), throttler.StartProcessingCount())
	assert.Equal(t, int32(1), throttler.EndProcessingCount())
}

func TestMultiDataInterceptor_ProcessReceivedMessageUnmarshalReturnsEmptySliceShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgMultiDataInterceptor()
	throttler := createMockThrottler()
	arg.Throttler = throttler
	arg.Marshalizer = &mock.MarshalizerStub{
		UnmarshalCalled: func(obj interface{}, buff []byte) error {
			return nil
		},
	}
	mdi, _ := interceptors.NewMultiDataInterceptor(arg)

	msg := &mock.P2PMessageMock{
		DataField: []byte("data to be processed"),
	}
	err := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)

	assert.Equal(t, process.ErrNoDataInMessage, err)
	// Regression GHSA-74m6-4hjp-7226 / KLC-2348.
	assert.Equal(t, int32(1), throttler.StartProcessingCount())
	assert.Equal(t, int32(1), throttler.EndProcessingCount())
}

func TestMultiDataInterceptor_ProcessReceivedCreateFailsShouldErr(t *testing.T) {
	t.Parallel()

	buffData := [][]byte{[]byte("buff1"), []byte("buff2")}

	checkCalledNum := int32(0)
	processCalledNum := int32(0)
	throttler := createMockThrottler()
	errExpected := errors.New("expected err")
	originatorPid := core.PeerID("originator")
	originatorBlackListed := false
	fromConnectedPeerBlackListed := false
	arg := createMockArgMultiDataInterceptor()
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(buff []byte) (data process.InterceptedData, e error) {
			return nil, errExpected
		},
	}
	arg.Throttler = throttler
	arg.Processor = createMockInterceptorStub(&checkCalledNum, &processCalledNum)
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
	mdi, _ := interceptors.NewMultiDataInterceptor(arg)

	dataField, _ := arg.Marshalizer.Marshal(&batch.Batch{Data: buffData})
	msg := &mock.P2PMessageMock{
		DataField: dataField,
		PeerField: originatorPid,
	}
	err := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)

	time.Sleep(time.Second)

	assert.Equal(t, errExpected, err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&checkCalledNum))
	assert.Equal(t, int32(0), atomic.LoadInt32(&processCalledNum))
	assert.Equal(t, int32(1), throttler.StartProcessingCount())
	assert.Equal(t, int32(1), throttler.EndProcessingCount())
	assert.True(t, originatorBlackListed)
	assert.True(t, fromConnectedPeerBlackListed)
}

func TestMultiDataInterceptor_ProcessReceivedPartiallyCorrectDataShouldErr(t *testing.T) {
	t.Parallel()

	correctData := []byte("buff1")
	incorrectData := []byte("buff2")
	buffData := [][]byte{incorrectData, correctData}

	checkCalledNum := int32(0)
	processCalledNum := int32(0)
	throttler := createMockThrottler()
	errExpected := errors.New("expected err")
	interceptedData := &mock.InterceptedDataStub{
		CheckValidityCalled: func() error {
			return nil
		},
	}
	arg := createMockArgMultiDataInterceptor()
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(buff []byte) (data process.InterceptedData, e error) {
			if bytes.Equal(buff, incorrectData) {
				return nil, errExpected
			}

			return interceptedData, nil
		},
	}
	arg.Processor = createMockInterceptorStub(&checkCalledNum, &processCalledNum)
	arg.Throttler = throttler
	mdi, _ := interceptors.NewMultiDataInterceptor(arg)

	dataField, _ := arg.Marshalizer.Marshal(&batch.Batch{Data: buffData})
	msg := &mock.P2PMessageMock{
		DataField: dataField,
	}
	err := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)

	time.Sleep(time.Second)

	assert.Equal(t, errExpected, err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&checkCalledNum))
	assert.Equal(t, int32(0), atomic.LoadInt32(&processCalledNum))
	assert.Equal(t, int32(1), throttler.StartProcessingCount())
	assert.Equal(t, int32(1), throttler.EndProcessingCount())
}

func TestMultiDataInterceptor_ProcessReceivedMessageNotValidShouldErrAndNotProcess(t *testing.T) {
	t.Parallel()

	errExpected := errors.New("expected err")
	testProcessReceiveMessageMultiData(t, errExpected, 0)
}

func TestMultiDataInterceptor_ProcessReceivedMessageIsAddressedToOtherShardShouldRetNilAndNotProcess(t *testing.T) {
	t.Parallel()

	testProcessReceiveMessageMultiData(t, process.ErrInterceptedDataNotForCurrentShard, 0)
}

func TestMultiDataInterceptor_ProcessReceivedMessageOkMessageShouldRetNil(t *testing.T) {
	t.Parallel()

	testProcessReceiveMessageMultiData(t, nil, 2)
}

func testProcessReceiveMessageMultiData(t *testing.T, expectedErr error, calledNum int) {
	buffData := [][]byte{[]byte("buff1"), []byte("buff2")}

	marshalizer := &mock.MarshalizerMock{}
	checkCalledNum := int32(0)
	processCalledNum := int32(0)
	throttler := createMockThrottler()
	interceptedData := &mock.InterceptedDataStub{
		CheckValidityCalled: func() error {
			return expectedErr
		},
	}
	arg := createMockArgMultiDataInterceptor()
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(buff []byte) (data process.InterceptedData, e error) {
			return interceptedData, nil
		},
	}
	arg.Processor = createMockInterceptorStub(&checkCalledNum, &processCalledNum)
	arg.Throttler = throttler
	mdi, _ := interceptors.NewMultiDataInterceptor(arg)

	dataField, _ := marshalizer.Marshal(&batch.Batch{Data: buffData})
	msg := &mock.P2PMessageMock{
		DataField: dataField,
	}
	err := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)

	time.Sleep(time.Second)

	assert.Equal(t, expectedErr, err)
	assert.Equal(t, int32(calledNum), atomic.LoadInt32(&checkCalledNum))
	assert.Equal(t, int32(calledNum), atomic.LoadInt32(&processCalledNum))
	assert.Equal(t, int32(1), throttler.StartProcessingCount())
	assert.Equal(t, int32(1), throttler.EndProcessingCount())
}

func TestMultiDataInterceptor_ProcessReceivedMessageWhitelistedShouldRetNil(t *testing.T) {
	t.Parallel()

	buffData := [][]byte{[]byte("buff1"), []byte("buff2")}

	checkCalledNum := int32(0)
	processCalledNum := int32(0)
	throttler := createMockThrottler()
	interceptedData := &mock.InterceptedDataStub{
		CheckValidityCalled: func() error {
			return nil
		},
	}
	arg := createMockArgMultiDataInterceptor()
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
	mdi, _ := interceptors.NewMultiDataInterceptor(arg)

	dataField, _ := arg.Marshalizer.Marshal(&batch.Batch{Data: buffData})
	msg := &mock.P2PMessageMock{
		DataField: dataField,
	}
	err := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)

	time.Sleep(time.Second)

	assert.Nil(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&checkCalledNum))
	assert.Equal(t, int32(2), atomic.LoadInt32(&processCalledNum))
	assert.Equal(t, int32(1), throttler.StartProcessingCount())
	assert.Equal(t, int32(1), throttler.EndProcessingCount())
}

func TestMultiDataInterceptor_InvalidTxVersionShouldBackList(t *testing.T) {
	t.Parallel()

	processReceivedMessageMultiDataInvalidVersion(t, process.ErrInvalidTransactionVersion)
}

func TestMultiDataInterceptor_InvalidTxChainIDShouldBackList(t *testing.T) {
	t.Parallel()

	processReceivedMessageMultiDataInvalidVersion(t, process.ErrInvalidChainID)
}

func TestMultiDataInterceptor_WrappedInvalidTxVersionShouldBackList(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("intercepted-tx: %w", process.ErrInvalidTransactionVersion)
	processReceivedMessageMultiDataInvalidVersion(t, wrapped)
}

func TestMultiDataInterceptor_WrappedInvalidChainIDShouldBackList(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("intercepted-tx: %w", process.ErrInvalidChainID)
	processReceivedMessageMultiDataInvalidVersion(t, wrapped)
}

func processReceivedMessageMultiDataInvalidVersion(t *testing.T, expectedErr error) {
	buffData := [][]byte{[]byte("buff1"), []byte("buff2")}
	marshalizer := &mock.MarshalizerMock{}
	checkCalledNum := int32(0)
	processCalledNum := int32(0)
	interceptedData := &mock.InterceptedDataStub{
		CheckValidityCalled: func() error {
			return expectedErr
		},
	}

	isOriginatorBlackListed := false
	isFromConnectedPeerBlackListed := false
	originator := core.PeerID("originator")
	arg := createMockArgMultiDataInterceptor()
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(buff []byte) (data process.InterceptedData, e error) {
			return interceptedData, nil
		},
	}
	arg.Processor = createMockInterceptorStub(&checkCalledNum, &processCalledNum)
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
	mdi, _ := interceptors.NewMultiDataInterceptor(arg)

	dataField, _ := marshalizer.Marshal(&batch.Batch{Data: buffData})
	msg := &mock.P2PMessageMock{
		DataField: dataField,
		PeerField: originator,
	}

	err := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
	assert.Equal(t, expectedErr, err)
	assert.True(t, isFromConnectedPeerBlackListed)
	assert.True(t, isOriginatorBlackListed)
}

//------- debug

func TestMultiDataInterceptor_SetInterceptedDebugHandlerNilShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgMultiDataInterceptor()
	mdi, _ := interceptors.NewMultiDataInterceptor(arg)

	err := mdi.SetInterceptedDebugHandler(nil)

	assert.Equal(t, process.ErrNilDebugger, err)
}

func TestMultiDataInterceptor_SetInterceptedDebugHandlerShouldWork(t *testing.T) {
	t.Parallel()

	arg := createMockArgMultiDataInterceptor()
	mdi, _ := interceptors.NewMultiDataInterceptor(arg)

	debugger := &mock.InterceptedDebugHandlerStub{}
	err := mdi.SetInterceptedDebugHandler(debugger)

	assert.Nil(t, err)
	assert.True(t, debugger == mdi.InterceptedDebugHandler()) //pointer testing
}

func TestMultiDataInterceptor_ProcessReceivedMessageIsOriginatorNotOkButWhiteListed(t *testing.T) {
	t.Parallel()

	buffData := [][]byte{[]byte("buff1"), []byte("buff2")}

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
	errOriginator := process.ErrOnlyValidatorsCanUseThisTopic
	arg := createMockArgMultiDataInterceptor()
	arg.DataFactory = &mock.InterceptedDataFactoryStub{
		CreateCalled: func(buff []byte) (data process.InterceptedData, e error) {
			return interceptedData, nil
		},
	}
	arg.Processor = createMockInterceptorStub(&checkCalledNum, &processCalledNum)
	arg.Throttler = throttler
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		IsOriginatorElectedForTopicCalled: func(pid core.PeerID, topic string) error {
			return errOriginator
		},
	}
	arg.WhiteListRequest = whiteListHandler
	mdi, _ := interceptors.NewMultiDataInterceptor(arg)

	dataField, _ := arg.Marshalizer.Marshal(&batch.Batch{Data: buffData})
	msg := &mock.P2PMessageMock{
		DataField: dataField,
	}
	err := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)

	time.Sleep(time.Second)

	assert.Nil(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&checkCalledNum))
	assert.Equal(t, int32(2), atomic.LoadInt32(&processCalledNum))
	assert.Equal(t, int32(1), throttler.StartProcessingCount())
	assert.Equal(t, int32(1), throttler.EndProcessingCount())

	whiteListHandler.IsWhiteListedCalled = func(interceptedData process.InterceptedData) bool {
		return false
	}
	err = mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
	time.Sleep(time.Second)

	assert.Equal(t, err, errOriginator)
	assert.Equal(t, int32(2), atomic.LoadInt32(&checkCalledNum))
	assert.Equal(t, int32(2), atomic.LoadInt32(&processCalledNum))
	assert.Equal(t, int32(2), throttler.StartProcessingCount())
	assert.Equal(t, int32(2), throttler.EndProcessingCount())
}

func TestMultiDataInterceptor_RegisterHandler(t *testing.T) {
	t.Parallel()

	arg := createMockArgMultiDataInterceptor()
	wasCalled := false
	arg.Processor = &mock.InterceptorProcessorStub{
		RegisterHandlerCalled: func(handler func(topic string, hash []byte, data interface{})) {
			wasCalled = true
		},
	}

	mdi, _ := interceptors.NewMultiDataInterceptor(arg)
	mdi.RegisterHandler(nil)

	assert.True(t, wasCalled)
}

//------- IsInterfaceNil

func TestMultiDataInterceptor_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	var mdi *interceptors.MultiDataInterceptor

	assert.True(t, check.IfNil(mdi))
}

//------- regression: GHSA-74m6-4hjp-7226 / KLC-2348 (Finding 2.1)

func malformedCompressedBatchPayload(t *testing.T, m *mock.MarshalizerMock) []byte {
	t.Helper()

	payload, err := m.Marshal(&batch.Batch{
		IsCompressed: true,
		Stream:       []byte("not-a-gzip-stream"),
		DataSize:     1,
	})
	require.NoError(t, err)
	return payload
}

// A malformed compressed P2P batch must not leak a throttler slot:
// every path after preProcessMessage's StartProcessing must release the slot exactly once.
func TestMultiDataInterceptor_ProcessReceivedMessage_DecompressionErrorShouldReleaseThrottlerSlot(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	payload := malformedCompressedBatchPayload(t, marshalizer)

	countingThrottler := &mock.InterceptorThrottlerStub{
		CanProcessCalled: func() bool { return true },
	}

	arg := createMockArgMultiDataInterceptor()
	arg.Marshalizer = marshalizer
	arg.Throttler = countingThrottler

	mdi, err := interceptors.NewMultiDataInterceptor(arg)
	require.NoError(t, err)

	msg := &mock.P2PMessageMock{
		DataField:  payload,
		PeerField:  core.PeerID("origin-peer"),
		SeqNoField: []byte("seq-1"),
	}

	processErr := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
	require.Error(t, processErr, "expected a decompression error to reach the vulnerable branch")

	assert.Equal(t, int32(1), countingThrottler.StartProcessingCount(),
		"preProcessMessage should call StartProcessing exactly once")
	assert.Equal(t, int32(1), countingThrottler.EndProcessingCount(),
		"regression GHSA-74m6-4hjp-7226 / KLC-2348: decompression-error path must release the throttler slot")
}

// Repeated malformed compressed batches must not exhaust a real, bounded throttler.
// On the unfixed code, the third malformed batch returns common.ErrSystemBusy because the
// previous two leaked their slots in the gzip-error branch.
func TestMultiDataInterceptor_ProcessReceivedMessage_RepeatedDecompressionErrorsMustNotExhaustThrottler(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	payload := malformedCompressedBatchPayload(t, marshalizer)

	const throttlerCapacity int32 = 2
	realThrottler, err := throttler.NewNumGoRoutinesThrottler(throttlerCapacity)
	require.NoError(t, err)

	arg := createMockArgMultiDataInterceptor()
	arg.Marshalizer = marshalizer
	arg.Throttler = realThrottler

	mdi, err := interceptors.NewMultiDataInterceptor(arg)
	require.NoError(t, err)

	const attempts = 5
	for i := 0; i < attempts; i++ {
		msg := &mock.P2PMessageMock{
			DataField:  payload,
			PeerField:  core.PeerID("origin-peer"),
			SeqNoField: fmt.Appendf(nil, "seq-%d", i+1),
		}
		processErr := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
		require.Error(t, processErr, "expected gzip error on iteration %d", i)
		require.Falsef(t, errors.Is(processErr, common.ErrSystemBusy),
			"regression GHSA-74m6-4hjp-7226 / KLC-2348: throttler exhausted on iteration %d: %v", i, processErr)
	}
}

//------- regression: GHSA-74m6-4hjp-7226 / KLC-2353 (Finding C3 / M4)

// Regression: an uncompressed Batch carrying len(Data) > MaxItemsPerBatch must be
// rejected with ErrTooManyItemsInBatch *before* the make([]InterceptedData, ...) below.
// Without the cap, ~16 B per attacker-controlled entry is allocated before any
// per-item validation runs (CWE-789 / CWE-770).
func TestMultiDataInterceptor_ProcessReceivedMessage_RejectsItemCountBomb_Uncompressed(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	bomb := &batch.Batch{Data: make([][]byte, interceptors.MaxItemsPerBatch+1)}
	payload, err := marshalizer.Marshal(bomb)
	require.NoError(t, err)

	arg := createMockArgMultiDataInterceptor()
	arg.Marshalizer = marshalizer

	mdi, err := interceptors.NewMultiDataInterceptor(arg)
	require.NoError(t, err)

	msg := &mock.P2PMessageMock{
		DataField:  payload,
		PeerField:  core.PeerID("origin-peer"),
		SeqNoField: []byte("seq-1"),
	}

	processErr := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
	require.ErrorIs(t, processErr, process.ErrTooManyItemsInBatch)
}

// Regression: a *compressed* Batch whose inflated payload encodes more than
// MaxItemsPerBatch entries must also be rejected. The 10 MiB gzip-bomb cap
// (KLC-2352) is necessary but not sufficient — a 10 MiB inflated stream of mostly
// empty entries can still encode millions of items.
func TestMultiDataInterceptor_ProcessReceivedMessage_RejectsItemCountBomb_Compressed(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	bomb := &batch.Batch{
		Algo: batch.CType_GZip,
		Data: make([][]byte, interceptors.MaxItemsPerBatch+1),
	}
	require.NoError(t, bomb.Compress(marshalizer))

	payload, err := marshalizer.Marshal(bomb)
	require.NoError(t, err)

	arg := createMockArgMultiDataInterceptor()
	arg.Marshalizer = marshalizer

	mdi, err := interceptors.NewMultiDataInterceptor(arg)
	require.NoError(t, err)

	msg := &mock.P2PMessageMock{
		DataField:  payload,
		PeerField:  core.PeerID("origin-peer"),
		SeqNoField: []byte("seq-1"),
	}

	processErr := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
	require.ErrorIs(t, processErr, process.ErrTooManyItemsInBatch)
}

// The uncompressed cap rejection must not leak a throttler slot — the same defense
// added in KLC-2348 must cover this new return path too.
func TestMultiDataInterceptor_ProcessReceivedMessage_ItemCountBomb_ReleasesThrottlerSlot(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	bomb := &batch.Batch{Data: make([][]byte, interceptors.MaxItemsPerBatch+1)}
	payload, err := marshalizer.Marshal(bomb)
	require.NoError(t, err)

	countingThrottler := &mock.InterceptorThrottlerStub{
		CanProcessCalled: func() bool { return true },
	}

	arg := createMockArgMultiDataInterceptor()
	arg.Marshalizer = marshalizer
	arg.Throttler = countingThrottler

	mdi, err := interceptors.NewMultiDataInterceptor(arg)
	require.NoError(t, err)

	msg := &mock.P2PMessageMock{
		DataField:  payload,
		PeerField:  core.PeerID("origin-peer"),
		SeqNoField: []byte("seq-1"),
	}

	processErr := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
	require.ErrorIs(t, processErr, process.ErrTooManyItemsInBatch)
	assert.Equal(t, int32(1), countingThrottler.StartProcessingCount())
	assert.Equal(t, int32(1), countingThrottler.EndProcessingCount(),
		"the items-per-batch rejection path must release the throttler slot")
}

// Same KLC-2348 invariant on the post-Decompress return path: a compressed batch
// whose inflated Data exceeds MaxItemsPerBatch hits a different return statement
// (multiDataInterceptor.go after b.Decompress) and must still release the throttler
// slot. Pinning this with its own test prevents a future refactor of
// ProcessReceivedMessage from regressing the slot accounting on the compressed path.
func TestMultiDataInterceptor_ProcessReceivedMessage_CompressedItemCountBomb_ReleasesThrottlerSlot(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	bomb := &batch.Batch{
		Algo: batch.CType_GZip,
		Data: make([][]byte, interceptors.MaxItemsPerBatch+1),
	}
	require.NoError(t, bomb.Compress(marshalizer))

	payload, err := marshalizer.Marshal(bomb)
	require.NoError(t, err)

	countingThrottler := &mock.InterceptorThrottlerStub{
		CanProcessCalled: func() bool { return true },
	}

	arg := createMockArgMultiDataInterceptor()
	arg.Marshalizer = marshalizer
	arg.Throttler = countingThrottler

	mdi, err := interceptors.NewMultiDataInterceptor(arg)
	require.NoError(t, err)

	msg := &mock.P2PMessageMock{
		DataField:  payload,
		PeerField:  core.PeerID("origin-peer"),
		SeqNoField: []byte("seq-1"),
	}

	processErr := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID)
	require.ErrorIs(t, processErr, process.ErrTooManyItemsInBatch)
	assert.Equal(t, int32(1), countingThrottler.StartProcessingCount())
	assert.Equal(t, int32(1), countingThrottler.EndProcessingCount(),
		"the post-Decompress items-per-batch rejection path must release the throttler slot")
}

//------- regression: data race on bdi.debugHandler from worker goroutine (Finding 4.2)

// MultiDataInterceptor.ProcessReceivedMessage spawns a worker goroutine that
// reads bdi.debugHandler via processInterceptedData → processDebugInterceptedData
// after the synchronous frame returns. Concurrent SetInterceptedDebugHandler
// calls must not race with that read. Run with `go test -race` to enforce.
//
// The validate/save/whitelist stubs all return nil/true so ProcessReceivedMessage
// reaches the success path that spawns the worker goroutine and ultimately
// dispatches LogProcessedHashes through bdi.debugHandler — that is precisely
// the read site the rotation goroutine races with. Changing any of those mocks
// to err-return would silently un-cover the race.
func TestMultiDataInterceptor_ProcessReceivedMessage_DebugHandlerRotation_NoRace(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}

	interceptedData := &mock.InterceptedDataStub{
		CheckValidityCalled: func() error { return nil },
		IdentifiersCalled:   func() [][]byte { return [][]byte{[]byte("id-0")} },
	}

	arg := createMockArgMultiDataInterceptor()
	arg.Marshalizer = marshalizer
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

	mdi, err := interceptors.NewMultiDataInterceptor(arg)
	require.NoError(t, err)

	payload, err := marshalizer.Marshal(&batch.Batch{Data: [][]byte{[]byte("x")}})
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
	require.NoError(t, mdi.SetInterceptedDebugHandler(dh1),
		"SetInterceptedDebugHandler must accept a valid handler — sanity check")

	const iterations = 1000
	// One item per batch (Data: [][]byte{[]byte("x")}), so success ⇒ exactly
	// one worker goroutine ⇒ exactly one LogProcessedHashes per iteration.
	done := make(chan struct{}, 2)

	go func() {
		for i := 0; i < iterations; i++ {
			workersWG.Add(1) // optimistic Add; refunded below on err
			msg := &mock.P2PMessageMock{
				DataField:  payload,
				PeerField:  core.PeerID("origin-peer"),
				SeqNoField: []byte("seq"),
			}
			if procErr := mdi.ProcessReceivedMessage(msg, fromConnectedPeerID); procErr != nil {
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
				_ = mdi.SetInterceptedDebugHandler(dh1)
			} else {
				_ = mdi.SetInterceptedDebugHandler(dh2)
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
