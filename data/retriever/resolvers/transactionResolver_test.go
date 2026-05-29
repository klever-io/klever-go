package resolvers_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/batch"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/retriever/resolvers"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var connectedPeerId = core.PeerID("connected peer id")

func createMockArgTxResolver() resolvers.ArgTxResolver {
	return resolvers.ArgTxResolver{
		SenderResolver:   &mock.TopicResolverSenderStub{},
		TxPool:           mock.NewShardedDataStub(),
		TxStorage:        &mock.StorerStub{},
		Marshalizer:      &mock.MarshalizerMock{},
		DataPacker:       &mock.DataPackerStub{},
		AntifloodHandler: &mock.P2PAntifloodHandlerStub{},
		Throttler:        &mock.ThrottlerStub{},
	}
}

//------- NewTxResolver

func TestNewTxResolver_NilResolverShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTxResolver()
	arg.SenderResolver = nil
	txRes, err := resolvers.NewTxResolver(arg)

	assert.Equal(t, common.ErrNilResolverSender, err)
	assert.Nil(t, txRes)
}

func TestNewTxResolver_NilTxPoolShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTxResolver()
	arg.TxPool = nil
	txRes, err := resolvers.NewTxResolver(arg)

	assert.Equal(t, common.ErrNilTxDataPool, err)
	assert.Nil(t, txRes)
}

func TestNewTxResolver_NilTxStorageShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTxResolver()
	arg.TxStorage = nil
	txRes, err := resolvers.NewTxResolver(arg)

	assert.Equal(t, common.ErrNilTxStorage, err)
	assert.Nil(t, txRes)
}

func TestNewTxResolver_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTxResolver()
	arg.Marshalizer = nil
	txRes, err := resolvers.NewTxResolver(arg)

	assert.Equal(t, common.ErrNilMarshalizer, err)
	assert.Nil(t, txRes)
}

func TestNewTxResolver_NilDataPackerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTxResolver()
	arg.DataPacker = nil
	txRes, err := resolvers.NewTxResolver(arg)

	assert.Equal(t, common.ErrNilDataPacker, err)
	assert.Nil(t, txRes)
}

func TestNewTxResolver_NilAntifloodHandlerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTxResolver()
	arg.AntifloodHandler = nil
	txRes, err := resolvers.NewTxResolver(arg)

	assert.Equal(t, common.ErrNilAntifloodHandler, err)
	assert.Nil(t, txRes)
}

func TestNewTxResolver_NilThrottlerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTxResolver()
	arg.Throttler = nil
	txRes, err := resolvers.NewTxResolver(arg)

	assert.Equal(t, common.ErrNilThrottler, err)
	assert.Nil(t, txRes)
}

func TestNewTxResolver_OkValsShouldWork(t *testing.T) {
	t.Parallel()

	arg := createMockArgTxResolver()
	txRes, err := resolvers.NewTxResolver(arg)

	assert.Nil(t, err)
	assert.False(t, check.IfNil(txRes))
}

//------- ProcessReceivedMessage

func TestTxResolver_ProcessReceivedMessageCanProcessMessageErrorsShouldErr(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("expected error")
	arg := createMockArgTxResolver()
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		CanProcessMessageCalled: func(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
			return expectedErr
		},
		CanProcessMessagesOnTopicCalled: func(peer core.PeerID, topic string, numMessages uint32, totalSize uint64, sequence []byte) error {
			return nil
		},
	}
	txRes, _ := resolvers.NewTxResolver(arg)

	err := txRes.ProcessReceivedMessage(&mock.P2PMessageMock{}, connectedPeerId)

	assert.True(t, errors.Is(err, expectedErr))
	assert.False(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.False(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

func TestTxResolver_ProcessReceivedMessageNilMessageShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTxResolver()
	txRes, _ := resolvers.NewTxResolver(arg)

	err := txRes.ProcessReceivedMessage(nil, connectedPeerId)

	assert.Equal(t, common.ErrNilMessage, err)
	assert.False(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.False(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

func TestTxResolver_ProcessReceivedMessageWrongTypeShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTxResolver()
	txRes, _ := resolvers.NewTxResolver(arg)

	data, _ := arg.Marshalizer.Marshal(&retriever.RequestData{Type: retriever.RequestDataType_NonceType, Value: []byte("aaa")})

	msg := &mock.P2PMessageMock{DataField: data}

	err := txRes.ProcessReceivedMessage(msg, connectedPeerId)

	assert.True(t, errors.Is(err, common.ErrRequestTypeNotImplemented))
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

func TestTxResolver_ProcessReceivedMessageNilValueShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTxResolver()
	txRes, _ := resolvers.NewTxResolver(arg)

	data, _ := arg.Marshalizer.Marshal(&retriever.RequestData{Type: retriever.RequestDataType_HashType, Value: nil})

	msg := &mock.P2PMessageMock{DataField: data}

	err := txRes.ProcessReceivedMessage(msg, connectedPeerId)

	assert.Equal(t, common.ErrNilValue, err)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

func TestTxResolver_ProcessReceivedMessageFoundInTxPoolShouldSearchAndSend(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	searchWasCalled := false
	sendWasCalled := false
	txReturned := &transaction.Transaction{}
	txPool := mock.NewShardedDataStub()
	txPool.SearchFirstDataCalled = func(key []byte) (value interface{}, ok bool) {
		if bytes.Equal([]byte("aaa"), key) {
			searchWasCalled = true
			return txReturned, true
		}

		return nil, false
	}

	arg := createMockArgTxResolver()
	arg.SenderResolver = &mock.TopicResolverSenderStub{
		SendCalled: func(buff []byte, peer core.PeerID) error {
			sendWasCalled = true
			return nil
		},
	}
	arg.TxPool = txPool
	txRes, _ := resolvers.NewTxResolver(arg)

	data, _ := marshalizer.Marshal(&retriever.RequestData{Type: retriever.RequestDataType_HashType, Value: []byte("aaa")})

	msg := &mock.P2PMessageMock{DataField: data}

	err := txRes.ProcessReceivedMessage(msg, connectedPeerId)

	assert.Nil(t, err)
	assert.True(t, searchWasCalled)
	assert.True(t, sendWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

func TestTxResolver_ProcessReceivedMessageFoundInTxPoolMarshalizerFailShouldRetNilAndErr(t *testing.T) {
	t.Parallel()

	errExpected := errors.New("MarshalizerMock generic error")

	marshalizerMock := &mock.MarshalizerMock{}
	marshalizerStub := &mock.MarshalizerStub{
		MarshalCalled: func(obj interface{}) (i []byte, e error) {
			return nil, errExpected
		},
		UnmarshalCalled: func(obj interface{}, buff []byte) error {
			return marshalizerMock.Unmarshal(obj, buff)
		},
	}
	txReturned := &transaction.Transaction{}
	txPool := mock.NewShardedDataStub()
	txPool.SearchFirstDataCalled = func(key []byte) (value interface{}, ok bool) {
		if bytes.Equal([]byte("aaa"), key) {
			return txReturned, true
		}

		return nil, false
	}

	arg := createMockArgTxResolver()
	arg.TxPool = txPool
	arg.Marshalizer = marshalizerStub
	txRes, _ := resolvers.NewTxResolver(arg)

	data, _ := marshalizerMock.Marshal(&retriever.RequestData{Type: retriever.RequestDataType_HashType, Value: []byte("aaa")})

	msg := &mock.P2PMessageMock{DataField: data}

	err := txRes.ProcessReceivedMessage(msg, connectedPeerId)

	assert.True(t, errors.Is(err, errExpected))
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

func TestTxResolver_ProcessReceivedMessageFoundInTxStorageShouldRetValAndSend(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}

	txPool := mock.NewShardedDataStub()
	txPool.SearchFirstDataCalled = func(key []byte) (value interface{}, ok bool) {
		//not found in txPool
		return nil, false
	}
	searchWasCalled := false
	sendWasCalled := false
	txReturned := &transaction.Transaction{}
	txReturnedAsBuffer, _ := marshalizer.Marshal(txReturned)
	txStorage := &mock.StorerStub{}
	txStorage.SearchFirstCalled = func(key []byte) (i []byte, e error) {
		if bytes.Equal([]byte("aaa"), key) {
			searchWasCalled = true
			return txReturnedAsBuffer, nil
		}

		return nil, nil
	}

	arg := createMockArgTxResolver()
	arg.SenderResolver = &mock.TopicResolverSenderStub{
		SendCalled: func(buff []byte, peer core.PeerID) error {
			sendWasCalled = true
			return nil
		},
	}
	arg.TxPool = txPool
	arg.TxStorage = txStorage
	txRes, _ := resolvers.NewTxResolver(arg)

	data, _ := marshalizer.Marshal(&retriever.RequestData{Type: retriever.RequestDataType_HashType, Value: []byte("aaa")})

	msg := &mock.P2PMessageMock{DataField: data}

	err := txRes.ProcessReceivedMessage(msg, connectedPeerId)

	assert.Nil(t, err)
	assert.True(t, searchWasCalled)
	assert.True(t, sendWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

func TestTxResolver_ProcessReceivedMessageFoundInTxStorageCheckRetError(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}

	txPool := mock.NewShardedDataStub()
	txPool.SearchFirstDataCalled = func(key []byte) (value interface{}, ok bool) {
		//not found in txPool
		return nil, false
	}

	errExpected := errors.New("expected error")

	txStorage := &mock.StorerStub{}
	txStorage.SearchFirstCalled = func(key []byte) (i []byte, e error) {
		if bytes.Equal([]byte("aaa"), key) {
			return nil, errExpected
		}

		return nil, nil
	}

	arg := createMockArgTxResolver()
	arg.TxPool = txPool
	arg.TxStorage = txStorage
	txRes, _ := resolvers.NewTxResolver(arg)

	data, _ := marshalizer.Marshal(&retriever.RequestData{Type: retriever.RequestDataType_HashType, Value: []byte("aaa")})

	msg := &mock.P2PMessageMock{DataField: data}

	err := txRes.ProcessReceivedMessage(msg, connectedPeerId)

	assert.True(t, errors.Is(err, errExpected))
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

func TestTxResolver_ProcessReceivedMessageRequestedTwoSmallTransactionsShouldCallSliceSplitter(t *testing.T) {
	t.Parallel()

	txHash1 := []byte("txHash1")
	txHash2 := []byte("txHash2")

	tx1 := &transaction.Transaction{}
	tx2 := &transaction.Transaction{}

	marshalizer := &mock.MarshalizerMock{}
	txPool := mock.NewShardedDataStub()
	txPool.SearchFirstDataCalled = func(key []byte) (value interface{}, ok bool) {
		if bytes.Equal(txHash1, key) {
			return tx1, true
		}
		if bytes.Equal(txHash2, key) {
			return tx2, true
		}

		return nil, false
	}

	splitSliceWasCalled := false
	sendWasCalled := false
	arg := createMockArgTxResolver()
	arg.SenderResolver = &mock.TopicResolverSenderStub{
		SendCalled: func(buff []byte, peer core.PeerID) error {
			sendWasCalled = true
			return nil
		},
	}
	arg.TxPool = txPool
	arg.DataPacker = &mock.DataPackerStub{
		PackDataInChunksCalled: func(data [][]byte, limit int) ([][]byte, error) {
			if len(data) != 2 {
				return nil, errors.New("should have been 2 data pieces")
			}

			splitSliceWasCalled = true
			return make([][]byte, 1), nil
		},
	}
	txRes, _ := resolvers.NewTxResolver(arg)

	buff, _ := marshalizer.Marshal(&batch.Batch{Data: [][]byte{txHash1, txHash2}})
	data, _ := marshalizer.Marshal(&retriever.RequestData{Type: retriever.RequestDataType_HashArrayType, Value: buff})

	msg := &mock.P2PMessageMock{DataField: data}

	err := txRes.ProcessReceivedMessage(msg, connectedPeerId)

	assert.Nil(t, err)
	assert.True(t, splitSliceWasCalled)
	assert.True(t, sendWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

func TestTxResolver_ProcessReceivedMessageRequestedTwoSmallTransactionsFoundOnlyOneShouldWork(t *testing.T) {
	t.Parallel()

	txHash1 := []byte("txHash1")
	txHash2 := []byte("txHash2")

	tx1 := &transaction.Transaction{}

	marshalizer := &mock.MarshalizerMock{}
	txPool := mock.NewShardedDataStub()
	txPool.SearchFirstDataCalled = func(key []byte) (value interface{}, ok bool) {
		if bytes.Equal(txHash1, key) {
			return tx1, true
		}

		return nil, false
	}

	splitSliceWasCalled := false
	sendWasCalled := false
	arg := createMockArgTxResolver()
	arg.SenderResolver = &mock.TopicResolverSenderStub{
		SendCalled: func(buff []byte, peer core.PeerID) error {
			sendWasCalled = true
			return nil
		},
	}
	arg.TxStorage = &mock.StorerStub{
		SearchFirstCalled: func(key []byte) (i []byte, err error) {
			return nil, errors.New("not found")
		},
	}
	arg.TxPool = txPool
	arg.DataPacker = &mock.DataPackerStub{
		PackDataInChunksCalled: func(data [][]byte, limit int) ([][]byte, error) {
			if len(data) != 1 {
				return nil, errors.New("should have been 1 data piece")
			}

			splitSliceWasCalled = true
			return make([][]byte, 1), nil
		},
	}
	txRes, _ := resolvers.NewTxResolver(arg)

	buff, _ := marshalizer.Marshal(&batch.Batch{Data: [][]byte{txHash1, txHash2}})
	data, _ := marshalizer.Marshal(&retriever.RequestData{Type: retriever.RequestDataType_HashArrayType, Value: buff})

	msg := &mock.P2PMessageMock{DataField: data}

	err := txRes.ProcessReceivedMessage(msg, connectedPeerId)

	assert.NotNil(t, err)
	assert.True(t, splitSliceWasCalled)
	assert.True(t, sendWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

//------- RequestTransactionFromHash

func TestTxResolver_RequestDataFromHashShouldWork(t *testing.T) {
	t.Parallel()

	requested := &retriever.RequestData{}

	res := &mock.TopicResolverSenderStub{}
	res.SendOnRequestTopicCalled = func(rd *retriever.RequestData, hashes [][]byte) error {
		requested = rd
		return nil
	}

	buffRequested := []byte("aaaa")

	arg := createMockArgTxResolver()
	arg.SenderResolver = res
	txRes, _ := resolvers.NewTxResolver(arg)

	assert.Nil(t, txRes.RequestDataFromHash(buffRequested, 0))
	assert.Equal(t, &retriever.RequestData{
		Type:  retriever.RequestDataType_HashType,
		Value: buffRequested,
	}, requested)
}

//------- RequestDataFromHashArray

func TestTxResolver_RequestDataFromHashArrayShouldWork(t *testing.T) {
	t.Parallel()

	requested := &retriever.RequestData{}

	res := &mock.TopicResolverSenderStub{}
	res.SendOnRequestTopicCalled = func(rd *retriever.RequestData, hashes [][]byte) error {
		requested = rd
		return nil
	}

	buffRequested := [][]byte{[]byte("aaaa"), []byte("bbbb")}

	marshalizer := &marshal.ProtoMarshalizer{}
	arg := createMockArgTxResolver()
	arg.Marshalizer = marshalizer
	arg.SenderResolver = res
	txRes, _ := resolvers.NewTxResolver(arg)

	buff, _ := marshalizer.Marshal(&batch.Batch{Data: buffRequested})

	assert.Nil(t, txRes.RequestDataFromHashArray(buffRequested, 0))
	assert.Equal(t, &retriever.RequestData{
		Type:  retriever.RequestDataType_HashArrayType,
		Value: buff,
	}, requested)
}

//------ NumPeersToQuery setter and getter

func TestTxResolver_SetAndGetNumPeersToQuery(t *testing.T) {
	t.Parallel()

	expectedIntra := 5
	expectedCross := 7

	arg := createMockArgTxResolver()
	arg.SenderResolver = &mock.TopicResolverSenderStub{
		GetNumPeersToQueryCalled: func() (int, int) {
			return expectedIntra, expectedCross
		},
	}
	txRes, _ := resolvers.NewTxResolver(arg)

	txRes.SetNumPeersToQuery(expectedIntra, expectedCross)
	actualIntra, actualCross := txRes.NumPeersToQuery()
	assert.Equal(t, expectedIntra, actualIntra)
	assert.Equal(t, expectedCross, actualCross)
}

//------- regression: GHSA-w342-mj6g-v9c4

func TestTxResolver_ProcessReceivedMessage_RejectsHashArrayItemBomb_Uncompressed(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}

	arg := createMockArgTxResolver()
	arg.Marshalizer = marshalizer
	txRes, err := resolvers.NewTxResolver(arg)
	require.NoError(t, err)

	bomb := &batch.Batch{Data: make([][]byte, batch.MaxItemsPerBatch+1)}
	buff, err := marshalizer.Marshal(bomb)
	require.NoError(t, err)

	data, err := marshalizer.Marshal(&retriever.RequestData{
		Type:  retriever.RequestDataType_HashArrayType,
		Value: buff,
	})
	require.NoError(t, err)

	msg := &mock.P2PMessageMock{DataField: data}

	processErr := txRes.ProcessReceivedMessage(msg, connectedPeerId)
	require.ErrorIs(t, processErr, common.ErrTooManyItemsInBatch)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

// Regression GHSA-w342-mj6g-v9c4 defense-in-depth: oversized hashesBuff must
// be rejected before Unmarshal (slice-header amplification window).
func TestTxResolver_ProcessReceivedMessage_RejectsOversizedRawHashArrayBuff(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}

	arg := createMockArgTxResolver()
	arg.Marshalizer = marshalizer
	txRes, err := resolvers.NewTxResolver(arg)
	require.NoError(t, err)

	// Junk bytes (not a marshaled Batch): on vulnerable code Unmarshal would
	// either error or decode to empty; only the fix returns ErrTooManyItemsInBatch.
	hashesBuff := make([]byte, batch.MaxHashArrayBuffSize+1)

	data, err := marshalizer.Marshal(&retriever.RequestData{
		Type:  retriever.RequestDataType_HashArrayType,
		Value: hashesBuff,
	})
	require.NoError(t, err)

	msg := &mock.P2PMessageMock{DataField: data}

	processErr := txRes.ProcessReceivedMessage(msg, connectedPeerId)
	require.ErrorIs(t, processErr, common.ErrBatchWireTooLarge)
	thr := arg.Throttler.(*mock.ThrottlerStub)
	assert.Equal(t, int32(1), thr.StartProcessingCount())
	assert.Equal(t, int32(1), thr.EndProcessingCount(),
		"the wire-size rejection path must release the throttler slot exactly once")
}

// Regression GHSA-w342-mj6g-v9c4: real amplification pattern. Payload is `0x0a 0x00` × N —
// proto3 field-1 LEN tag + length-0 varint = one empty `repeated bytes` entry per pair.
// Pre-check must reject on byte length before Unmarshal allocates N empty []byte headers.
func TestTxResolver_ProcessReceivedMessage_RejectsRealAmplificationPattern(t *testing.T) {
	t.Parallel()

	// Proto marshalizer so the pattern decodes (not just a size rejection) if pre-check is bypassed.
	protoMarsh := marshal.NewProtoMarshalizer()

	arg := createMockArgTxResolver()
	arg.Marshalizer = protoMarsh
	txRes, err := resolvers.NewTxResolver(arg)
	require.NoError(t, err)

	const numEmptyEntries = batch.MaxHashArrayBuffSize/2 + 1 // 2 B per entry → just over cap
	hashesBuff := bytes.Repeat([]byte{0x0a, 0x00}, numEmptyEntries)

	data, err := protoMarsh.Marshal(&retriever.RequestData{
		Type:  retriever.RequestDataType_HashArrayType,
		Value: hashesBuff,
	})
	require.NoError(t, err)

	msg := &mock.P2PMessageMock{DataField: data}

	processErr := txRes.ProcessReceivedMessage(msg, connectedPeerId)
	require.ErrorIs(t, processErr, common.ErrBatchWireTooLarge,
		"wire-size pre-check must fire before proto Unmarshal allocates %d empty entries", numEmptyEntries)
	thr := arg.Throttler.(*mock.ThrottlerStub)
	assert.Equal(t, int32(1), thr.EndProcessingCount(),
		"throttler slot must be released on the pre-check rejection path")
}

func TestTxResolver_ProcessReceivedMessage_RejectsHashArrayItemBomb_Compressed(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}

	arg := createMockArgTxResolver()
	arg.Marshalizer = marshalizer
	txRes, err := resolvers.NewTxResolver(arg)
	require.NoError(t, err)

	bomb := &batch.Batch{
		Algo: batch.CType_GZip,
		Data: make([][]byte, batch.MaxItemsPerBatch+1),
	}
	require.NoError(t, bomb.Compress(marshalizer))

	buff, err := marshalizer.Marshal(bomb)
	require.NoError(t, err)

	data, err := marshalizer.Marshal(&retriever.RequestData{
		Type:  retriever.RequestDataType_HashArrayType,
		Value: buff,
	})
	require.NoError(t, err)

	msg := &mock.P2PMessageMock{DataField: data}

	processErr := txRes.ProcessReceivedMessage(msg, connectedPeerId)
	require.ErrorIs(t, processErr, common.ErrTooManyItemsInBatch)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

// Regression: a panic in any dependency must be recovered by
// ProcessReceivedMessage rather than killing the message-handling goroutine.
func TestTxResolver_ProcessReceivedMessage_RecoversFromPanic(t *testing.T) {
	t.Parallel()

	arg := createMockArgTxResolver()
	arg.Marshalizer = &mock.MarshalizerStub{
		UnmarshalCalled: func(obj interface{}, buff []byte) error {
			panic("injected panic during Unmarshal")
		},
	}

	txRes, err := resolvers.NewTxResolver(arg)
	require.NoError(t, err)

	msg := &mock.P2PMessageMock{DataField: []byte("anything")}

	processErr := txRes.ProcessReceivedMessage(msg, connectedPeerId)
	require.Error(t, processErr)
	require.Truef(t, errors.Is(processErr, common.ErrProcessReceivedMessagePanicked),
		"expected ErrProcessReceivedMessagePanicked, got %v", processErr)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}
