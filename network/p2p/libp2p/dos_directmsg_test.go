package libp2p_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ggio "github.com/gogo/protobuf/io"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/network/p2p/data"
	"github.com/klever-io/klever-go/network/p2p/libp2p"
	"github.com/klever-io/klever-go/network/p2p/mock"
	corelibp2p "github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubPb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
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

// TestDirectSender_SinglePeerManyStreams_ExcessStreamsReset guards KLC-2433 F4/F6: a single peer
// opening many concurrent DirectSendID streams must not multiply reader goroutines without bound
// (on seed nodes the NullResourceManager enforces no libp2p-level limit). The receiver keeps at
// most maxInboundDirectStreamsPerPeer inbound direct-send streams per peer and resets the excess.
func TestDirectSender_SinglePeerManyStreams_ExcessStreamsReset(t *testing.T) {
	const attackStreams = 12

	netw := mocknet.New()

	receiver, err := libp2p.NewMockMessenger(createMockNetworkArgs(), netw)
	require.Nil(t, err)
	defer func() { _ = receiver.Close() }()

	attacker, err := netw.GenPeer()
	require.Nil(t, err)

	require.Nil(t, netw.LinkAll())

	var receiverHost host.Host
	for _, h := range netw.Hosts() {
		if h.ID() == peer.ID(receiver.ID()) {
			receiverHost = h
		}
	}
	require.NotNil(t, receiverHost)

	_, err = netw.ConnectPeers(attacker.ID(), receiverHost.ID())
	require.Nil(t, err)

	// Open the streams and push one valid frame down each so protocol negotiation completes and
	// the receiver's stream handler runs. Errors are tolerated: once the cap is enforced, excess
	// streams are reset and their open/write fails — that is the enforcement working.
	topic := "test"
	opened := make([]network.Stream, 0, attackStreams)
	for i := 0; i < attackStreams; i++ {
		ctxStream, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		s, errS := attacker.NewStream(ctxStream, receiverHost.ID(), libp2p.DirectSendID)
		cancel()
		if errS != nil {
			continue
		}
		w := ggio.NewDelimitedWriter(s)
		_ = w.WriteMsg(&pubsubPb.Message{
			From:  []byte(attacker.ID()),
			Data:  []byte("flood"),
			Seqno: []byte{0, 0, 0, 0, 0, 0, 0, byte(i)},
			Topic: &topic,
		})
		opened = append(opened, s)
	}
	require.Greater(t, len(opened), libp2p.MaxInboundDirectStreamsPerPeer,
		"the attacker opened no more streams than the per-peer cap, so the cap was never exercised")

	streamEnded := make(chan error, len(opened))
	for _, s := range opened {
		go func(st network.Stream) {
			_, errRead := st.Read(make([]byte, 1))
			streamEnded <- errRead
		}(s)
	}

	excess := len(opened) - libp2p.MaxInboundDirectStreamsPerPeer
	for i := 0; i < excess; i++ {
		select {
		case errEnded := <-streamEnded:
			require.ErrorIs(t, errEnded, network.ErrReset,
				"an excess inbound direct-send stream ended for a reason other than being reset")
		case <-time.After(20 * time.Second):
			t.Fatalf("only %d of the %d excess streams were reset: the per-peer cap is not enforced",
				i, excess)
		}
	}

	countInbound := func() int {
		n := 0
		for _, c := range receiverHost.Network().ConnsToPeer(attacker.ID()) {
			for _, st := range c.GetStreams() {
				if st.Protocol() == libp2p.DirectSendID && st.Stat().Direction == network.DirInbound {
					n++
				}
			}
		}
		return n
	}

	// Eventually the receiver must hold exactly the per-peer cap: the excess is reset AND the
	// first cap streams survive (guards against an over-aggressive reset breaking direct send).
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) && countInbound() > libp2p.MaxInboundDirectStreamsPerPeer {
		time.Sleep(time.Millisecond)
	}
	require.Equal(t, libp2p.MaxInboundDirectStreamsPerPeer, countInbound(),
		"a single peer must hold exactly the per-peer cap of inbound direct-send streams: "+
			"more means the cap is not enforced, fewer means legitimate streams were reset")
}

// TestDirectSender_ManyPeersManyStreams_GlobalCapBoundsTotal guards the Sybil residual of
// KLC-2433 F4: the per-peer cap alone lets an attacker rotating peer identities take 4 streams
// per identity, so a cross-peer cap must bound the total. Three peers try 4 streams each (12,
// all within the per-peer cap); with the global cap shrunk to 6, exactly 6 may survive.
func TestDirectSender_ManyPeersManyStreams_GlobalCapBoundsTotal(t *testing.T) {
	const globalCap = 6
	const attackerPeers = 3

	restore := libp2p.SetMaxInboundDirectStreamsTotal(globalCap)
	defer restore()

	netw := mocknet.New()

	receiver, err := libp2p.NewMockMessenger(createMockNetworkArgs(), netw)
	require.Nil(t, err)
	defer func() { _ = receiver.Close() }()

	attackers := make([]host.Host, 0, attackerPeers)
	for i := 0; i < attackerPeers; i++ {
		a, errG := netw.GenPeer()
		require.Nil(t, errG)
		attackers = append(attackers, a)
	}

	require.Nil(t, netw.LinkAll())

	var receiverHost host.Host
	for _, h := range netw.Hosts() {
		if h.ID() == peer.ID(receiver.ID()) {
			receiverHost = h
		}
	}
	require.NotNil(t, receiverHost)

	topic := "test"
	for ai, a := range attackers {
		_, errC := netw.ConnectPeers(a.ID(), receiverHost.ID())
		require.Nil(t, errC)

		for i := 0; i < libp2p.MaxInboundDirectStreamsPerPeer; i++ {
			ctxStream, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			s, errS := a.NewStream(ctxStream, receiverHost.ID(), libp2p.DirectSendID)
			cancel()
			if errS != nil {
				continue
			}
			w := ggio.NewDelimitedWriter(s)
			_ = w.WriteMsg(&pubsubPb.Message{
				From:  []byte(a.ID()),
				Data:  []byte("flood"),
				Seqno: []byte{0, 0, 0, 0, 0, 0, byte(ai), byte(i)},
				Topic: &topic,
			})
		}
	}

	countTotal := func() int {
		n := 0
		for _, a := range attackers {
			for _, c := range receiverHost.Network().ConnsToPeer(a.ID()) {
				for _, st := range c.GetStreams() {
					if st.Protocol() == libp2p.DirectSendID && st.Stat().Direction == network.DirInbound {
						n++
					}
				}
			}
		}
		return n
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && countTotal() != globalCap {
		time.Sleep(20 * time.Millisecond)
	}
	require.Equal(t, globalCap, countTotal(),
		"total inbound direct-send streams must converge to the global cap: more means the cap "+
			"is not enforced, fewer means legitimate streams were reset")
}

// TestDirectSender_InboundStreamIdleDeadline_ResetsStalledStream guards KLC-2433 F3 (slow-loris):
// an inbound DirectSendID stream that stops delivering frames must be reset once the idle read
// deadline elapses, while a stream actively delivering frames must stay open — the deadline
// refreshes on every frame. Uses real TCP hosts: mocknet streams ignore SetReadDeadline.
func TestDirectSender_InboundStreamIdleDeadline_ResetsStalledStream(t *testing.T) {
	const idleTimeout = 800 * time.Millisecond
	restore := libp2p.SetDirectRecvIdleTimeout(idleTimeout)
	defer restore()

	args := createMockNetworkArgs()
	receiver, err := libp2p.NewNetworkMessenger(args)
	require.Nil(t, err)
	defer func() { _ = receiver.Close() }()

	topic := "test"
	err = receiver.RegisterMessageProcessor(topic, &mock.MessageProcessorStub{
		ProcessMessageCalled: func(_ p2p.MessageP2P, _ core.PeerID) error {
			return nil
		},
	})
	require.Nil(t, err)

	attacker, err := corelibp2p.New(corelibp2p.NoListenAddrs)
	require.Nil(t, err)
	defer func() { _ = attacker.Close() }()

	addrInfo, err := peer.AddrInfoFromString(receiver.Addresses()[0])
	require.Nil(t, err)
	ctxDial, cancelDial := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelDial()
	require.Nil(t, attacker.Connect(ctxDial, *addrInfo))

	s, err := attacker.NewStream(ctxDial, addrInfo.ID, libp2p.DirectSendID)
	require.Nil(t, err)

	// The receiver never writes back on direct-send streams, so this read unblocks with an error
	// exactly when the receiver resets the stream.
	readErr := make(chan error, 1)
	go func() {
		_, errRead := s.Read(make([]byte, 1))
		readErr <- errRead
	}()

	payload, err := args.Marshalizer.Marshal(&data.TopicMessage{
		Version:   libp2p.CurrentTopicMessageVersion,
		Payload:   []byte("frame"),
		Timestamp: time.Now().Unix(),
	})
	require.Nil(t, err)

	// Frames spaced well under the idle window, sent for longer than the window in total: the
	// stream must survive, proving the deadline refreshes per accepted frame instead of being set
	// once. These pass frame-level validation (distinct seqno, matching From, non-nil topic) and
	// reach a registered processor, which is what makes them count as use of the stream.
	// 5 frames at idleTimeout/4 spacing: 1.25× the window in total (proves per-frame refresh)
	// while each gap keeps a 4× margin against CI scheduling jitter.
	w := ggio.NewDelimitedWriter(s)
	for i := 0; i < 5; i++ {
		msg := &pubsubPb.Message{
			From:  []byte(attacker.ID()),
			Data:  payload,
			Seqno: []byte{0, 0, 0, 0, 0, 0, 0, byte(i)},
			Topic: &topic,
		}
		require.Nil(t, w.WriteMsg(msg), "write %d failed — stream reset while actively sending", i)
		select {
		case errRead := <-readErr:
			t.Fatalf("stream reset while frames were flowing within the idle window: %v", errRead)
		case <-time.After(idleTimeout / 4):
		}
	}

	// Slow-loris: start a frame (varint length prefix promising 128 bytes) and stall mid-frame.
	_, _ = s.Write([]byte{0x80, 0x01})

	// The stalled stream must be reset within the idle window (generous slack for CI).
	select {
	case <-readErr:
		// the stalled stream was reset — the F3 bound is enforced
	case <-time.After(10 * idleTimeout):
		t.Fatal("stalled inbound direct stream was not reset after the idle read deadline")
	}
}

// TestDirectSender_InboundStreamResetAfterRepeatedInvalidFrames pins the usefulness side of the
// idle bound: a stream that keeps the connection busy with frames that never pass validation must
// still be reset. Without it the idle deadline degrades into a liveness check, and a peer can hold
// a capped slot indefinitely by emitting garbage faster than the timeout.
func TestDirectSender_InboundStreamResetAfterRepeatedInvalidFrames(t *testing.T) {
	const invalidFrameLimit = 4
	restoreLimit := libp2p.SetMaxConsecutiveInvalidFrames(invalidFrameLimit)
	defer restoreLimit()

	// Long enough that the idle deadline cannot be what resets the stream.
	restoreIdle := libp2p.SetDirectRecvIdleTimeout(30 * time.Second)
	defer restoreIdle()

	receiver, err := libp2p.NewNetworkMessenger(createMockNetworkArgs())
	require.Nil(t, err)
	defer func() { _ = receiver.Close() }()

	attacker, err := corelibp2p.New(corelibp2p.NoListenAddrs)
	require.Nil(t, err)
	defer func() { _ = attacker.Close() }()

	addrInfo, err := peer.AddrInfoFromString(receiver.Addresses()[0])
	require.Nil(t, err)
	ctxDial, cancelDial := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelDial()
	require.Nil(t, attacker.Connect(ctxDial, *addrInfo))

	s, err := attacker.NewStream(ctxDial, addrInfo.ID, libp2p.DirectSendID)
	require.Nil(t, err)

	readErr := make(chan error, 1)
	go func() {
		_, errRead := s.Read(make([]byte, 1))
		readErr <- errRead
	}()

	// Every frame carries a From that does not match the sending peer, so each one fails
	// frame-level validation while remaining perfectly readable.
	topic := "test"
	w := ggio.NewDelimitedWriter(s)
	for i := 0; i < invalidFrameLimit+2; i++ {
		msg := &pubsubPb.Message{
			From:  []byte("not-the-sending-peer"),
			Data:  []byte("frame"),
			Seqno: []byte{0, 0, 0, 0, 0, 0, 0, byte(i)},
			Topic: &topic,
		}
		if errWrite := w.WriteMsg(msg); errWrite != nil {
			// The stream was reset mid-run, which is the behaviour under test.
			break
		}
	}

	select {
	case <-readErr:
		// reset after repeated invalid frames
	case <-time.After(10 * time.Second):
		t.Fatal("inbound direct stream survived repeated invalid frames")
	}
}

// TestDirectSender_ResetsStreamAfterRepeatedUnservedTopicFrames pins the other half of the
// usefulness bound: a frame that clears every check the reader can make on its own is still not use
// of the stream if no processor is registered for its topic. Frame-level validation alone would let
// a peer emitting syntactically perfect frames on a topic nobody serves refresh its idle deadline
// forever and hold a capped slot without the node ever doing work for it.
func TestDirectSender_ResetsStreamAfterRepeatedUnservedTopicFrames(t *testing.T) {
	const invalidFrameLimit = 4
	restoreLimit := libp2p.SetMaxConsecutiveInvalidFrames(invalidFrameLimit)
	defer restoreLimit()

	// Long enough that the idle deadline cannot be what resets the stream.
	restoreIdle := libp2p.SetDirectRecvIdleTimeout(30 * time.Second)
	defer restoreIdle()

	args := createMockNetworkArgs()
	receiver, err := libp2p.NewNetworkMessenger(args)
	require.Nil(t, err)
	defer func() { _ = receiver.Close() }()

	attacker, err := corelibp2p.New(corelibp2p.NoListenAddrs)
	require.Nil(t, err)
	defer func() { _ = attacker.Close() }()

	addrInfo, err := peer.AddrInfoFromString(receiver.Addresses()[0])
	require.Nil(t, err)
	ctxDial, cancelDial := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelDial()
	require.Nil(t, attacker.Connect(ctxDial, *addrInfo))

	s, err := attacker.NewStream(ctxDial, addrInfo.ID, libp2p.DirectSendID)
	require.Nil(t, err)

	readErr := make(chan error, 1)
	go func() {
		_, errRead := s.Read(make([]byte, 1))
		readErr <- errRead
	}()

	// Every frame is well-formed the whole way down: it passes frame-level validation, unmarshals
	// into a current-version topic message and carries a fresh timestamp, so nothing before the
	// handler rejects it and the peer is never blacklisted. The only thing it lacks is a processor
	// registered for its topic, which is exactly what the handler turns down.
	payload, err := args.Marshalizer.Marshal(&data.TopicMessage{
		Version:   libp2p.CurrentTopicMessageVersion,
		Payload:   []byte("frame"),
		Timestamp: time.Now().Unix(),
	})
	require.Nil(t, err)

	topic := "topic-with-no-registered-processor"
	w := ggio.NewDelimitedWriter(s)
	for i := 0; i < invalidFrameLimit+2; i++ {
		msg := &pubsubPb.Message{
			From:  []byte(attacker.ID()),
			Data:  payload,
			Seqno: []byte{0, 0, 0, 0, 0, 0, 0, byte(i)},
			Topic: &topic,
		}
		if errWrite := w.WriteMsg(msg); errWrite != nil {
			// The stream was reset mid-run, which is the behaviour under test.
			break
		}
	}

	select {
	case <-readErr:
		// reset once the unserved frames reached the limit
	case <-time.After(10 * time.Second):
		t.Fatal("inbound direct stream survived repeated well-formed frames on a topic no processor serves")
	}
}

// TestDirectSender_SendRecoversFromRemotelyResetStream pins the send side of the idle bound.
// libp2p's swarm only drops a stream from conn.GetStreams() on a local Close or Reset, so after the
// receiver's idle timeout resets an inbound stream the sender still finds the dead outbound half and
// reuses it. Without a retry the first send after any quiet period is silently lost, which matters
// because direct send carries resolver replies and the requester would eat a full request timeout.
func TestDirectSender_SendRecoversFromRemotelyResetStream(t *testing.T) {
	const idleTimeout = 700 * time.Millisecond
	restore := libp2p.SetDirectRecvIdleTimeout(idleTimeout)
	defer restore()

	receiver, err := libp2p.NewNetworkMessenger(createMockNetworkArgs())
	require.Nil(t, err)
	defer func() { _ = receiver.Close() }()

	sender, err := libp2p.NewNetworkMessenger(createMockNetworkArgs())
	require.Nil(t, err)
	defer func() { _ = sender.Close() }()

	received := make(chan struct{}, 8)
	err = receiver.RegisterMessageProcessor("test", &mock.MessageProcessorStub{
		ProcessMessageCalled: func(_ p2p.MessageP2P, _ core.PeerID) error {
			received <- struct{}{}
			return nil
		},
	})
	require.Nil(t, err)

	require.Nil(t, sender.ConnectToPeer(receiver.Addresses()[0]))

	connectDeadline := time.Now().Add(5 * time.Second)
	for !sender.IsConnected(receiver.ID()) {
		require.True(t, time.Now().Before(connectDeadline), "sender never connected")
		time.Sleep(10 * time.Millisecond)
	}

	// First send establishes the outbound stream and must arrive.
	require.Nil(t, sender.SendToConnectedPeer("test", []byte("first"), receiver.ID()))
	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("first direct message was not delivered")
	}

	// Go quiet past the receiver's idle window so it resets the inbound half. The sender's
	// outbound half stays in conn.GetStreams() and will be reused by getOrCreateStream.
	time.Sleep(idleTimeout * 3)

	// This is the send that was previously lost.
	require.Nil(t, sender.SendToConnectedPeer("test", []byte("second"), receiver.ID()),
		"send after the receiver's idle reset returned an error instead of retrying on a fresh stream")

	select {
	case <-received:
	case <-time.After(5 * time.Second):
		t.Fatal("direct message after the receiver's idle reset was lost")
	}
}

func TestDirectSender_SendDoesNotRetryAfterWriteTimeout(t *testing.T) {
	const writeTimeout = 300 * time.Millisecond
	restore := libp2p.SetDirectSendWriteTimeout(writeTimeout)
	defer restore()

	id, sk := createLibP2PCredentialsDirectSender()
	remotePeer := peer.ID("remote peer")

	stalling := mock.NewStreamMock()
	require.Nil(t, stalling.SetProtocol(libp2p.DirectSendID))
	stalling.SetWriteBlocked(true)

	cs := createConnStub(stalling, id, sk, remotePeer)
	netw := &mock.NetworkStub{
		ConnsToPeerCalled: func(_ peer.ID) []network.Conn {
			return []network.Conn{cs}
		},
	}

	var openedStreams int32
	hs := &mock.ConnectableHostStub{
		SetStreamHandlerCalled: func(protocol.ID, network.StreamHandler) {},
		NetworkCalled: func() network.Network {
			return netw
		},
		NewStreamCalled: func(_ context.Context, _ peer.ID, _ ...protocol.ID) (network.Stream, error) {
			atomic.AddInt32(&openedStreams, 1)

			fresh := mock.NewStreamMock()
			_ = fresh.SetProtocol(libp2p.DirectSendID)
			fresh.SetWriteBlocked(true)

			return fresh, nil
		},
	}

	ds, err := libp2p.NewDirectSender(context.Background(), hs, blankMessageHandler)
	require.Nil(t, err)

	start := time.Now()
	err = ds.Send("topic", []byte("data"), core.PeerID(remotePeer))
	elapsed := time.Since(start)

	require.NotNil(t, err)
	require.Equal(t, int32(0), atomic.LoadInt32(&openedStreams),
		"a write that hit the send timeout was retried on a fresh stream, so an unresponsive peer can "+
			"hold the per-peer send mutex for twice the write timeout")
	require.True(t, elapsed < 2*writeTimeout,
		fmt.Sprintf("Send took %s, more than the single write timeout of %s it is bounded by", elapsed, writeTimeout))
}

// TestDirectSender_HonestPeerServedAfterAttackerSaturatesCap is the assertion the cap tests were
// missing: they only ever counted attacker-held streams, so they passed identically whether an
// honest peer was served or reset at accept. The node-wide cap is first-come with no eviction, so
// without a reservation a source that can mint identities takes all of it and every later peer is
// refused indefinitely — and direct send carries resolver replies, so that means the node stops
// receiving chain data it asked for.
func TestDirectSender_HonestPeerServedAfterAttackerSaturatesCap(t *testing.T) {
	t.Skip("no fairness policy yet: the node-wide cap is first-come with no eviction, so peers " +
		"rotating identities take every slot and a later honest peer is refused. Enable this once " +
		"an eviction or reservation policy lands — reserving capacity for peers that hold no " +
		"stream is not sufficient, since each new identity opens its first stream as a new peer.")

	const totalCap = 8
	restoreTotal := libp2p.SetMaxInboundDirectStreamsTotal(totalCap)
	defer restoreTotal()

	restoreIdle := libp2p.SetDirectRecvIdleTimeout(30 * time.Second)
	defer restoreIdle()

	netw := mocknet.New()

	receiver, err := libp2p.NewMockMessenger(createMockNetworkArgs(), netw)
	require.Nil(t, err)
	defer func() { _ = receiver.Close() }()

	delivered := make(chan struct{}, 4)
	err = receiver.RegisterMessageProcessor("test", &mock.MessageProcessorStub{
		ProcessMessageCalled: func(_ p2p.MessageP2P, _ core.PeerID) error {
			delivered <- struct{}{}
			return nil
		},
	})
	require.Nil(t, err)

	honest, err := libp2p.NewMockMessenger(createMockNetworkArgs(), netw)
	require.Nil(t, err)
	defer func() { _ = honest.Close() }()

	// Attackers rotate identities to defeat the per-peer cap, as a single source with spare
	// addresses would. Created before LinkAll so every host is wired into the mock network.
	numAttackers := totalCap
	attackers := make([]host.Host, 0, numAttackers)
	for i := 0; i < numAttackers; i++ {
		a, errHost := netw.GenPeer()
		require.Nil(t, errHost)
		attackers = append(attackers, a)
	}

	require.Nil(t, netw.LinkAll())

	var receiverHost host.Host
	for _, h := range netw.Hosts() {
		if h.ID() == peer.ID(receiver.ID()) {
			receiverHost = h
		}
	}
	require.NotNil(t, receiverHost)

	// Saturate: each attacker pushes one valid frame per stream so protocol negotiation completes
	// and the receiver's handler actually accounts for the stream. Errors are expected once the
	// cap bites — that is the enforcement working.
	topic := "test"
	for _, a := range attackers {
		_, errConn := netw.ConnectPeers(a.ID(), receiverHost.ID())
		require.Nil(t, errConn)

		for j := 0; j < libp2p.MaxInboundDirectStreamsPerPeer; j++ {
			ctxStream, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			st, errS := a.NewStream(ctxStream, receiverHost.ID(), libp2p.DirectSendID)
			cancel()
			if errS != nil {
				continue
			}
			w := ggio.NewDelimitedWriter(st)
			_ = w.WriteMsg(&pubsubPb.Message{
				From:  []byte(a.ID()),
				Data:  []byte("flood"),
				Seqno: []byte{0, 0, 0, 0, 0, 0, 0, byte(j)},
				Topic: &topic,
			})
		}
	}

	// Let the receiver accept and account for the attacker streams.
	time.Sleep(500 * time.Millisecond)

	// An honest peer arriving after saturation must still be able to open a stream and be heard.
	require.Nil(t, honest.ConnectToPeer(receiver.Addresses()[0]))
	connectDeadline := time.Now().Add(10 * time.Second)
	for !honest.IsConnected(receiver.ID()) {
		require.True(t, time.Now().Before(connectDeadline), "honest peer never connected")
		time.Sleep(10 * time.Millisecond)
	}

	err = honest.SendToConnectedPeer("test", []byte("honest"), receiver.ID())
	require.Nil(t, err, "honest peer could not send once attackers saturated the inbound cap")

	select {
	case <-delivered:
		// served despite the saturated cap
	case <-time.After(10 * time.Second):
		t.Fatal("honest peer was starved: attackers holding the inbound cap blocked its message")
	}
}

// TestDirectSender_StalledStreamReaderIsReclaimed mirrors the reported proof of concept: a peer
// opens a direct-send stream and sends nothing. Before the idle deadline the reader goroutine
// blocked in ReadMsg forever, pinning its stack and the stream's buffers until the attacker chose
// to disconnect. The deadline must reclaim it without any action from the peer, which shows up as
// the stream being reset rather than left open.
func TestDirectSender_StalledStreamReaderIsReclaimed(t *testing.T) {
	const idleTimeout = 300 * time.Millisecond
	restore := libp2p.SetDirectRecvIdleTimeout(idleTimeout)
	defer restore()

	ds, err := libp2p.NewDirectSender(
		context.Background(),
		&mock.ConnectableHostStub{
			SetStreamHandlerCalled: func(protocol.ID, network.StreamHandler) {},
		},
		func(*pubsub.Message, core.PeerID) error { return nil },
	)
	require.Nil(t, err)

	stream := mock.NewStreamMock()
	stream.SetConn(&mock.ConnStub{
		RemotePeerCalled: func() peer.ID { return peer.ID("stalling-peer") },
	})

	ds.DirectStreamHandler(stream)

	// The peer sends nothing at all. The reader must still be reclaimed by the idle deadline, and
	// a reclaimed reader resets the stream rather than leaving it open.
	require.Eventually(t, stream.IsReset, 10*idleTimeout, 20*time.Millisecond,
		"stalled stream was never reset: a peer that opens a stream and sends nothing can pin the "+
			"reader goroutine indefinitely")
}
