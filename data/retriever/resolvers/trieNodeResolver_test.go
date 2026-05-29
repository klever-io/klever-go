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
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fromConnectedPeer = core.PeerID("from connected peer")

func createMockArgTrieNodeResolver() resolvers.ArgTrieNodeResolver {
	return resolvers.ArgTrieNodeResolver{
		SenderResolver:   &mock.TopicResolverSenderStub{},
		TrieDataGetter:   &mock.TrieStub{},
		Marshalizer:      &mock.MarshalizerMock{},
		AntifloodHandler: &mock.P2PAntifloodHandlerStub{},
		Throttler:        &mock.ThrottlerStub{},
	}
}

func TestNewTrieNodeResolver_NilResolverShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTrieNodeResolver()
	arg.SenderResolver = nil
	tnRes, err := resolvers.NewTrieNodeResolver(arg)

	assert.Equal(t, common.ErrNilResolverSender, err)
	assert.Nil(t, tnRes)
}

func TestNewTrieNodeResolver_NilTrieShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTrieNodeResolver()
	arg.TrieDataGetter = nil
	tnRes, err := resolvers.NewTrieNodeResolver(arg)

	assert.Equal(t, common.ErrNilTrieDataGetter, err)
	assert.Nil(t, tnRes)
}

func TestNewTrieNodeResolver_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTrieNodeResolver()
	arg.Marshalizer = nil
	tnRes, err := resolvers.NewTrieNodeResolver(arg)

	assert.Equal(t, common.ErrNilMarshalizer, err)
	assert.Nil(t, tnRes)
}

func TestNewTrieNodeResolver_NilAntiflooderShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTrieNodeResolver()
	arg.AntifloodHandler = nil
	tnRes, err := resolvers.NewTrieNodeResolver(arg)

	assert.Equal(t, common.ErrNilAntifloodHandler, err)
	assert.Nil(t, tnRes)
}

func TestNewTrieNodeResolver_NilThrottlerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTrieNodeResolver()
	arg.Throttler = nil
	tnRes, err := resolvers.NewTrieNodeResolver(arg)

	assert.Equal(t, common.ErrNilThrottler, err)
	assert.Nil(t, tnRes)
}

func TestNewTrieNodeResolver_OkValsShouldWork(t *testing.T) {
	t.Parallel()

	arg := createMockArgTrieNodeResolver()
	tnRes, err := resolvers.NewTrieNodeResolver(arg)

	assert.Nil(t, err)
	assert.False(t, check.IfNil(tnRes))
}

//------- ProcessReceivedMessage

func TestTrieNodeResolver_ProcessReceivedAntiflooderCanProcessMessageErrShouldErr(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("expected error")
	arg := createMockArgTrieNodeResolver()
	arg.AntifloodHandler = &mock.P2PAntifloodHandlerStub{
		CanProcessMessageCalled: func(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
			return expectedErr
		},
		CanProcessMessagesOnTopicCalled: func(peer core.PeerID, topic string, numMessages uint32, totalSize uint64, sequence []byte) error {
			return nil
		},
	}
	tnRes, _ := resolvers.NewTrieNodeResolver(arg)

	err := tnRes.ProcessReceivedMessage(&mock.P2PMessageMock{}, fromConnectedPeer)
	assert.True(t, errors.Is(err, expectedErr))
	assert.False(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.False(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

func TestTrieNodeResolver_ProcessReceivedMessageNilMessageShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgTrieNodeResolver()
	tnRes, _ := resolvers.NewTrieNodeResolver(arg)

	err := tnRes.ProcessReceivedMessage(nil, fromConnectedPeer)
	assert.Equal(t, common.ErrNilMessage, err)
	assert.False(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.False(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

func TestTrieNodeResolver_ProcessReceivedMessageWrongTypeShouldErr(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}

	arg := createMockArgTrieNodeResolver()
	tnRes, _ := resolvers.NewTrieNodeResolver(arg)

	data, _ := marshalizer.Marshal(&retriever.RequestData{Type: retriever.RequestDataType_NonceType, Value: []byte("aaa")})
	msg := &mock.P2PMessageMock{DataField: data}

	err := tnRes.ProcessReceivedMessage(msg, fromConnectedPeer)
	assert.Equal(t, common.ErrRequestTypeNotImplemented, err)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

func TestTrieNodeResolver_ProcessReceivedMessageNilValueShouldErr(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}

	arg := createMockArgTrieNodeResolver()
	tnRes, _ := resolvers.NewTrieNodeResolver(arg)

	data, _ := marshalizer.Marshal(&retriever.RequestData{Type: retriever.RequestDataType_HashType, Value: nil})
	msg := &mock.P2PMessageMock{DataField: data}

	err := tnRes.ProcessReceivedMessage(msg, fromConnectedPeer)
	assert.Equal(t, common.ErrNilValue, err)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

func TestTrieNodeResolver_ProcessReceivedMessageShouldGetFromTrieAndSend(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	getSerializedNodesWasCalled := false
	sendWasCalled := false
	returnedEncNodes := [][]byte{[]byte("node1"), []byte("node2")}

	tr := &mock.TrieStub{
		GetSerializedNodesCalled: func(hash []byte, maxSize uint64) ([][]byte, uint64, error) {
			if bytes.Equal([]byte("node1"), hash) {
				getSerializedNodesWasCalled = true
				return returnedEncNodes, 0, nil
			}

			return nil, 0, errors.New("wrong hash")
		},
	}

	arg := createMockArgTrieNodeResolver()
	arg.TrieDataGetter = tr
	arg.SenderResolver = &mock.TopicResolverSenderStub{
		SendCalled: func(buff []byte, peer core.PeerID) error {
			sendWasCalled = true
			return nil
		},
	}
	tnRes, _ := resolvers.NewTrieNodeResolver(arg)

	data, _ := marshalizer.Marshal(&retriever.RequestData{Type: retriever.RequestDataType_HashType, Value: []byte("node1")})
	msg := &mock.P2PMessageMock{DataField: data}

	err := tnRes.ProcessReceivedMessage(msg, fromConnectedPeer)

	assert.Nil(t, err)
	assert.True(t, getSerializedNodesWasCalled)
	assert.True(t, sendWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

func TestTrieNodeResolver_ProcessReceivedMessageShouldGetFromTrieAndMarshalizerFailShouldRetNilAndErr(t *testing.T) {
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

	arg := createMockArgTrieNodeResolver()
	arg.Marshalizer = marshalizerStub
	tnRes, _ := resolvers.NewTrieNodeResolver(arg)

	data, _ := marshalizerMock.Marshal(&retriever.RequestData{Type: retriever.RequestDataType_HashType, Value: []byte("node1")})
	msg := &mock.P2PMessageMock{DataField: data}

	err := tnRes.ProcessReceivedMessage(msg, fromConnectedPeer)
	assert.Equal(t, errExpected, err)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

func TestTrieNodeResolver_ProcessReceivedMessageTrieErrorsShouldErr(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("expected err")
	arg := createMockArgTrieNodeResolver()
	arg.TrieDataGetter = &mock.TrieStub{
		GetSerializedNodesCalled: func(_ []byte, _ uint64) ([][]byte, uint64, error) {
			return nil, 0, expectedErr
		},
	}
	tnRes, _ := resolvers.NewTrieNodeResolver(arg)

	data, _ := arg.Marshalizer.Marshal(&retriever.RequestData{Type: retriever.RequestDataType_HashType, Value: []byte("node1")})
	msg := &mock.P2PMessageMock{DataField: data}

	err := tnRes.ProcessReceivedMessage(msg, fromConnectedPeer)
	assert.Equal(t, expectedErr, err)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

//------- RequestTransactionFromHash

func TestTrieNodeResolver_RequestDataFromHashShouldWork(t *testing.T) {
	t.Parallel()

	requested := &retriever.RequestData{}

	res := &mock.TopicResolverSenderStub{}
	res.SendOnRequestTopicCalled = func(rd *retriever.RequestData, hashes [][]byte) error {
		requested = rd
		return nil
	}

	buffRequested := []byte("node1")

	arg := createMockArgTrieNodeResolver()
	arg.SenderResolver = res
	tnRes, _ := resolvers.NewTrieNodeResolver(arg)

	assert.Nil(t, tnRes.RequestDataFromHash(buffRequested, 0))
	assert.Equal(t, &retriever.RequestData{
		Type:  retriever.RequestDataType_HashType,
		Value: buffRequested,
	}, requested)
}

//------ NumPeersToQuery setter and getter

func TestTrieNodeResolver_SetAndGetNumPeersToQuery(t *testing.T) {
	t.Parallel()

	expectedIntra := 5
	expectedCross := 7

	arg := createMockArgTrieNodeResolver()
	arg.SenderResolver = &mock.TopicResolverSenderStub{
		GetNumPeersToQueryCalled: func() (int, int) {
			return expectedIntra, expectedCross
		},
	}
	tnRes, _ := resolvers.NewTrieNodeResolver(arg)

	tnRes.SetNumPeersToQuery(expectedIntra, expectedCross)
	actualIntra, actualCross := tnRes.NumPeersToQuery()
	assert.Equal(t, expectedIntra, actualIntra)
	assert.Equal(t, expectedCross, actualCross)
}

//------- regression: GHSA-w342-mj6g-v9c4

func TestTrieNodeResolver_ProcessReceivedMessage_RejectsHashArrayItemBomb_Uncompressed(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}

	arg := createMockArgTrieNodeResolver()
	arg.Marshalizer = marshalizer
	tnRes, err := resolvers.NewTrieNodeResolver(arg)
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

	processErr := tnRes.ProcessReceivedMessage(msg, fromConnectedPeer)
	require.ErrorIs(t, processErr, common.ErrTooManyItemsInBatch)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

// Regression GHSA-w342-mj6g-v9c4 defense-in-depth: oversized hashesBuff must
// be rejected before Unmarshal (slice-header amplification window).
func TestTrieNodeResolver_ProcessReceivedMessage_RejectsOversizedRawHashArrayBuff(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}

	arg := createMockArgTrieNodeResolver()
	arg.Marshalizer = marshalizer
	tnRes, err := resolvers.NewTrieNodeResolver(arg)
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

	processErr := tnRes.ProcessReceivedMessage(msg, fromConnectedPeer)
	require.ErrorIs(t, processErr, common.ErrBatchWireTooLarge)
	thr := arg.Throttler.(*mock.ThrottlerStub)
	assert.Equal(t, int32(1), thr.StartProcessingCount())
	assert.Equal(t, int32(1), thr.EndProcessingCount(),
		"the wire-size rejection path must release the throttler slot exactly once")
}

// Regression GHSA-w342-mj6g-v9c4: real amplification pattern. Payload is `0x0a 0x00` × N —
// proto3 field-1 LEN tag + length-0 varint = one empty `repeated bytes` entry per pair.
// Pre-check must reject on byte length before Unmarshal allocates N empty []byte headers.
func TestTrieNodeResolver_ProcessReceivedMessage_RejectsRealAmplificationPattern(t *testing.T) {
	t.Parallel()

	// Proto marshalizer so the pattern decodes (not just a size rejection) if pre-check is bypassed.
	protoMarsh := marshal.NewProtoMarshalizer()

	arg := createMockArgTrieNodeResolver()
	arg.Marshalizer = protoMarsh
	tnRes, err := resolvers.NewTrieNodeResolver(arg)
	require.NoError(t, err)

	const numEmptyEntries = batch.MaxHashArrayBuffSize/2 + 1
	hashesBuff := bytes.Repeat([]byte{0x0a, 0x00}, numEmptyEntries)

	data, err := protoMarsh.Marshal(&retriever.RequestData{
		Type:  retriever.RequestDataType_HashArrayType,
		Value: hashesBuff,
	})
	require.NoError(t, err)

	msg := &mock.P2PMessageMock{DataField: data}

	processErr := tnRes.ProcessReceivedMessage(msg, fromConnectedPeer)
	require.ErrorIs(t, processErr, common.ErrBatchWireTooLarge,
		"wire-size pre-check must fire before proto Unmarshal allocates %d empty entries", numEmptyEntries)
	thr := arg.Throttler.(*mock.ThrottlerStub)
	assert.Equal(t, int32(1), thr.EndProcessingCount(),
		"throttler slot must be released on the pre-check rejection path")
}

func TestTrieNodeResolver_ProcessReceivedMessage_RejectsHashArrayItemBomb_Compressed(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}

	arg := createMockArgTrieNodeResolver()
	arg.Marshalizer = marshalizer
	tnRes, err := resolvers.NewTrieNodeResolver(arg)
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

	processErr := tnRes.ProcessReceivedMessage(msg, fromConnectedPeer)
	require.ErrorIs(t, processErr, common.ErrTooManyItemsInBatch)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}

// Regression: a panic in any dependency must be recovered by
// ProcessReceivedMessage rather than killing the message-handling goroutine.
func TestTrieNodeResolver_ProcessReceivedMessage_RecoversFromPanic(t *testing.T) {
	t.Parallel()

	arg := createMockArgTrieNodeResolver()
	arg.Marshalizer = &mock.MarshalizerStub{
		UnmarshalCalled: func(obj interface{}, buff []byte) error {
			panic("injected panic during Unmarshal")
		},
	}

	tnRes, err := resolvers.NewTrieNodeResolver(arg)
	require.NoError(t, err)

	msg := &mock.P2PMessageMock{DataField: []byte("anything")}

	processErr := tnRes.ProcessReceivedMessage(msg, fromConnectedPeer)
	require.Error(t, processErr)
	require.Truef(t, errors.Is(processErr, common.ErrProcessReceivedMessagePanicked),
		"expected ErrProcessReceivedMessagePanicked, got %v", processErr)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).StartWasCalled)
	assert.True(t, arg.Throttler.(*mock.ThrottlerStub).EndWasCalled)
}
