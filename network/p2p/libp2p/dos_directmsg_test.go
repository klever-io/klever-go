package libp2p_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/network/p2p/libp2p"
	"github.com/klever-io/klever-go/network/p2p/mock"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

// TestNetworkMessenger_DirectMessageHandler_ProcessesSynchronously guards GHSA-hf2g-6j7h-98wg:
// direct-message processing must be synchronous, so in-flight processing is bounded by the number
// of streams, not the number of messages. N senders (=> N streams) flood one receiver whose
// processor parks until released; the test asserts peak in-flight stays at N (pre-fix it would
// approach the message count, as each message spawned its own goroutine).
func TestNetworkMessenger_DirectMessageHandler_ProcessesSynchronously(t *testing.T) {
	const numSenders = 4
	const perSender = 25 // 100 messages total; pre-fix this would approach 100 in-flight

	netw := mocknet.New()

	receiver, err := libp2p.NewMockMessenger(createMockNetworkArgs(), netw)
	require.Nil(t, err)
	defer func() { _ = receiver.Close() }()

	senders := make([]p2p.Messenger, numSenders)
	for i := range senders {
		s, errS := libp2p.NewMockMessenger(createMockNetworkArgs(), netw)
		require.Nil(t, errS)
		senders[i] = s
		defer func(m p2p.Messenger) { _ = m.Close() }(s)
	}

	require.Nil(t, netw.LinkAll())

	var current, maxObserved, exceeded int32
	release := make(chan struct{})

	err = receiver.RegisterMessageProcessor("test", &mock.MessageProcessorStub{
		ProcessMessageCalled: func(_ p2p.MessageP2P, _ core.PeerID) error {
			c := atomic.AddInt32(&current, 1)
			for {
				m := atomic.LoadInt32(&maxObserved)
				if c <= m || atomic.CompareAndSwapInt32(&maxObserved, m, c) {
					break
				}
			}
			if c > int32(numSenders) {
				atomic.StoreInt32(&exceeded, 1)
			}
			<-release // park, keeping every concurrently-processing message in-flight
			atomic.AddInt32(&current, -1)
			return nil
		},
	})
	require.Nil(t, err)

	for _, s := range senders {
		require.Nil(t, s.ConnectToPeer(receiver.Addresses()[0]))
	}

	// Wait until every sender is connected before flooding (polling beats a fixed sleep, which
	// flakes under load). A direct send needs a live connection.
	connectDeadline := time.Now().Add(5 * time.Second)
	for _, s := range senders {
		for !s.IsConnected(receiver.ID()) {
			require.True(t, time.Now().Before(connectDeadline),
				"senders did not connect to the receiver within the deadline")
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Each sender floods on its own goroutine. With a parked processor, only the first message per
	// stream is consumed; the rest block on backpressure rather than soaking into goroutines.
	var wg sync.WaitGroup
	for _, s := range senders {
		wg.Add(1)
		go func(m p2p.Messenger) {
			defer wg.Done()
			for j := 0; j < perSender; j++ {
				_ = m.SendToConnectedPeer("test", []byte(fmt.Sprintf("m-%d", j)), receiver.ID())
			}
		}(s)
	}

	// Wait until one message per stream is parked, asserting in-flight never exceeds stream count.
	reached := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		require.Zero(t, atomic.LoadInt32(&exceeded),
			"in-flight exceeded live-stream count — per-message goroutine spawn has regressed")
		if atomic.LoadInt32(&current) == int32(numSenders) {
			reached = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.True(t, reached,
		"in-flight never reached the stream count (%d); sent %d total", numSenders, numSenders*perSender)

	// Hold to confirm in-flight does NOT climb toward the message count.
	time.Sleep(300 * time.Millisecond)
	require.LessOrEqual(t, atomic.LoadInt32(&maxObserved), int32(numSenders),
		"peak in-flight must stay at stream count (%d), not message count (%d)",
		numSenders, numSenders*perSender)

	close(release) // unblock parked processors; backpressured sends now drain
	wg.Wait()
}
