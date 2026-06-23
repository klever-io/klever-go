package storageResolvers

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/batch"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/network/p2p/libp2p"
	p2pMock "github.com/klever-io/klever-go/network/p2p/mock"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

// selfSendTimeout bounds each self-send so a deadlock regression fails fast instead of hanging.
const selfSendTimeout = 2 * time.Second

// newLoopbackMessenger builds a real in-memory networkMessenger to exercise the genuine
// SendToConnectedPeer(self) -> sendDirectToSelf -> directMessageHandler loopback.
func newLoopbackMessenger(t *testing.T) p2p.Messenger {
	t.Helper()

	mes, err := libp2p.NewMockMessenger(
		libp2p.ArgsNetworkMessenger{
			Marshalizer:   &commonMock.ProtoMarshalizerMock{},
			ListenAddress: libp2p.ListenLocalhostAddrWithIp4AndTcp,
			P2pConfig: config.P2PConfig{
				Node:                config.NodeConfig{Port: "0"},
				KadDhtPeerDiscovery: config.KadDhtPeerDiscoveryConfig{Enabled: false},
				Sharding:            config.ShardingConfig{Type: p2p.NilListSharder},
			},
			SyncTimer: &libp2p.LocalSyncTimer{},
		},
		mocknet.New(),
	)
	require.Nil(t, err)

	return mes
}

// runWithin runs fn and fails the test if it does not return before selfSendTimeout.
func runWithin(t *testing.T, what string, fn func() error) error {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		// Turn a panic in fn into a clean test failure instead of crashing the test binary.
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("%s panicked: %v", what, r)
			}
		}()
		done <- fn()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(selfSendTimeout):
		// On timeout the worker goroutine is intentionally leaked: this is a fail-fast guard and
		// the process exits via Fatalf, so the leak does not outlive the test.
		t.Fatalf("%s deadlocked: self-send did not return within %s — import-db replay would hang", what, selfSendTimeout)
		return nil
	}
}

// TestSliceResolver_RequestDataFromHash_SelfSendIsSynchronous guards the import-db replay path:
// storage resolvers answer local requests via sendToSelf -> SendToConnectedPeer(self). After the
// GHSA-hf2g-6j7h-98wg fix this must deliver to the response-topic processor synchronously (already
// processed by the time RequestDataFromHash returns), with no error and no deadlock.
func TestSliceResolver_RequestDataFromHash_SelfSendIsSynchronous(t *testing.T) {
	t.Parallel()

	const responseTopic = "txBlockBodies_0_RESPONSE"
	hash := []byte("mb-hash")
	value := []byte("marshalled-miniblock-bytes")

	mes := newLoopbackMessenger(t)
	defer func() { _ = mes.Close() }()

	marshalizer := &commonMock.MarshalizerMock{}

	var processed int32
	var received atomic.Value // []byte
	err := mes.RegisterMessageProcessor(responseTopic, &p2pMock.MessageProcessorStub{
		ProcessMessageCalled: func(msg p2p.MessageP2P, _ core.PeerID) error {
			received.Store(append([]byte(nil), msg.Data()...))
			atomic.AddInt32(&processed, 1)
			return nil
		},
	})
	require.Nil(t, err)

	storer := commonMock.NewStorerMock("Storage", 0)
	require.Nil(t, storer.Put(hash, value))

	arg := ArgSliceResolver{
		Messenger:                mes,
		ResponseTopicName:        responseTopic,
		Storage:                  storer,
		DataPacker:               &commonMock.DataPackerStub{},
		Marshalizer:              marshalizer,
		ManualEpochStartNotifier: &commonMock.ManualEpochStartNotifierStub{},
		ChanGracefullyClose:      make(chan endProcess.ArgEndProcess, 1),
	}
	res, err := NewSliceResolver(arg)
	require.Nil(t, err)

	err = runWithin(t, "RequestDataFromHash", func() error {
		return res.RequestDataFromHash(hash, 0)
	})
	require.Nil(t, err)

	// Synchronous: processing must be done by the time RequestDataFromHash returns.
	require.Equal(t, int32(1), atomic.LoadInt32(&processed),
		"self-sent response must be processed synchronously before RequestDataFromHash returns")

	// Payload must match the resolver's marshalled batch.
	expected, err := marshalizer.Marshal(&batch.Batch{Data: [][]byte{value}})
	require.Nil(t, err)
	got, _ := received.Load().([]byte)
	require.True(t, bytes.Equal(expected, got),
		"delivered payload must match the resolver's marshalled batch")
}

// TestSliceResolver_RequestDataFromHashArray_SelfSendDeliversEveryChunk guards the multi-message
// self-send loop (one sendToSelf per packed chunk): every chunk must arrive synchronously, in order.
func TestSliceResolver_RequestDataFromHashArray_SelfSendDeliversEveryChunk(t *testing.T) {
	t.Parallel()

	const responseTopic = "miniBlocks_0_RESPONSE"
	hashes := [][]byte{[]byte("h1"), []byte("h2"), []byte("h3")}
	values := [][]byte{[]byte("v1"), []byte("v2"), []byte("v3")}

	mes := newLoopbackMessenger(t)
	defer func() { _ = mes.Close() }()

	var mut sync.Mutex
	deliveries := make([][]byte, 0, len(hashes))
	err := mes.RegisterMessageProcessor(responseTopic, &p2pMock.MessageProcessorStub{
		ProcessMessageCalled: func(msg p2p.MessageP2P, _ core.PeerID) error {
			mut.Lock()
			deliveries = append(deliveries, append([]byte(nil), msg.Data()...))
			mut.Unlock()
			return nil
		},
	})
	require.Nil(t, err)

	storer := commonMock.NewStorerMock("Storage", 0)
	for i := range hashes {
		require.Nil(t, storer.Put(hashes[i], values[i]))
	}

	// Identity packer: one chunk per stored value, preserving order.
	packer := &commonMock.DataPackerStub{
		PackDataInChunksCalled: func(data [][]byte, _ int) ([][]byte, error) {
			return data, nil
		},
	}

	arg := ArgSliceResolver{
		Messenger:                mes,
		ResponseTopicName:        responseTopic,
		Storage:                  storer,
		DataPacker:               packer,
		Marshalizer:              &commonMock.MarshalizerMock{},
		ManualEpochStartNotifier: &commonMock.ManualEpochStartNotifierStub{},
		ChanGracefullyClose:      make(chan endProcess.ArgEndProcess, 1),
	}
	res, err := NewSliceResolver(arg)
	require.Nil(t, err)

	err = runWithin(t, "RequestDataFromHashArray", func() error {
		return res.RequestDataFromHashArray(hashes, 0)
	})
	require.Nil(t, err)

	mut.Lock()
	defer mut.Unlock()
	require.Equal(t, values, deliveries,
		"every self-sent chunk must be delivered synchronously and in order")
}
