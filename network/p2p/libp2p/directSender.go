package libp2p

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	ggio "github.com/gogo/protobuf/io"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/p2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubPb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/whyrusleeping/timecache"
)

var _ p2p.DirectSender = (*directSender)(nil)

const timeSeenMessages = time.Second * 120
const maxMutexes = 10000
const seqnoLength = 8

// directSendWriteTimeout bounds a single direct-message write so a stalled peer cannot pin the
// writer. 10s covers legitimate ~1 MiB responses. See GHSA-hf2g-6j7h-98wg.
var directSendWriteTimeout = time.Second * 10

// directRecvIdleTimeout bounds how long the inbound direct-stream reader waits for the next full
// frame, so a slow-loris peer cannot pin a reader goroutine indefinitely. Refreshed on every
// frame; idle streams are reset. A sender reusing a stream reset this way sees the write fail and
// retries once on a fresh stream (see Send), so the reset is recovered rather than transparent.
// Var (not const) so tests can shorten it. KLC-2433 F3.
var directRecvIdleTimeout = time.Minute * 2

// maxInboundDirectStreamsPerPeer caps concurrent inbound direct-send streams accepted from a
// single peer. Copied into the directSender at construction (like the other four knobs) so the
// handler goroutine never reads the package var, which would race the restore closures the test
// setters hand back. Well-behaved peers reuse one outbound stream (see getOrCreateStream), so steady
// state is 1; headroom covers reset/reopen churn. Excess streams are reset at accept, bounding
// reader goroutines per peer even under NullResourceManager (seed nodes). KLC-2433 F4.
var maxInboundDirectStreamsPerPeer = 4

// maxInboundDirectStreamsTotal caps inbound direct-send streams across ALL peers, so an attacker
// rotating peer identities (Sybil) cannot multiply reader goroutines past the per-peer cap on
// nodes without libp2p resource limits (seeds). 512 bounds worst-case transient memory to
// ~0.5 GiB (one 1 MiB frame in flight per stream) while staying far above the one reused stream
// per legitimately direct-sending peer. Var (not const) so tests can shorten it. KLC-2433.
var maxInboundDirectStreamsTotal = 512

// maxConsecutiveInvalidFrames bounds how many frames a peer may send that fail processing before
// its stream is reset. Without it a peer can hold a slot indefinitely by emitting frames that are
// well-formed enough to read but always rejected, since only an accepted frame refreshes the idle
// deadline. Var (not const) so tests can shorten it.
var maxConsecutiveInvalidFrames = 8

type directSender struct {
	counter             uint64
	ctx                 context.Context
	hostP2P             host.Host
	messageHandler      func(msg *pubsub.Message, fromConnectedPeer core.PeerID) error
	mutSeenMessages     sync.Mutex
	seenMessages        *timecache.TimeCache
	mutexForPeer        *MutexHolder
	recvIdleTimeout     time.Duration
	sendWriteTimeout    time.Duration
	recvStreamsPerPeer  int
	recvStreamsTotalCap int
	maxInvalidFrames    int
	mutInboundStreams   sync.Mutex
	inboundStreams      map[peer.ID]int
	totalInboundStreams int
}

type directSenderOption func(*directSender)

func withDirectSendConfig(dsCfg config.DirectSendConfig) directSenderOption {
	return func(ds *directSender) {
		if dsCfg.MaxInboundStreamsPerPeer > 0 {
			ds.recvStreamsPerPeer = dsCfg.MaxInboundStreamsPerPeer
		}
		if dsCfg.MaxInboundStreamsTotal > 0 {
			ds.recvStreamsTotalCap = dsCfg.MaxInboundStreamsTotal
		}
	}
}

// NewDirectSender returns a new instance of direct sender object
func NewDirectSender(
	ctx context.Context,
	h host.Host,
	messageHandler func(msg *pubsub.Message, fromConnectedPeer core.PeerID) error,
	opts ...directSenderOption,
) (*directSender, error) {

	if h == nil {
		return nil, p2p.ErrNilHost
	}
	if ctx == nil {
		return nil, p2p.ErrNilContext
	}
	if messageHandler == nil {
		return nil, p2p.ErrNilDirectSendMessageHandler
	}

	mutexForPeer, err := NewMutexHolder(maxMutexes)
	if err != nil {
		return nil, err
	}

	ds := &directSender{
		counter:             uint64(time.Now().UnixNano()), // #nosec G115
		ctx:                 ctx,
		hostP2P:             h,
		seenMessages:        timecache.NewTimeCache(timeSeenMessages),
		messageHandler:      messageHandler,
		mutexForPeer:        mutexForPeer,
		recvIdleTimeout:     directRecvIdleTimeout,
		sendWriteTimeout:    directSendWriteTimeout,
		recvStreamsPerPeer:  maxInboundDirectStreamsPerPeer,
		recvStreamsTotalCap: maxInboundDirectStreamsTotal,
		maxInvalidFrames:    maxConsecutiveInvalidFrames,
		inboundStreams:      make(map[peer.ID]int),
	}

	for _, opt := range opts {
		opt(ds)
	}

	//wire-up a handler for direct messages
	h.SetStreamHandler(DirectSendID, ds.directStreamHandler)

	return ds, nil
}

func (ds *directSender) directStreamHandler(s network.Stream) {
	remotePeer := s.Conn().RemotePeer()
	if !ds.tryAcquireInboundStream(remotePeer) {
		log.Trace("inbound direct stream limit reached, resetting stream",
			"peer", remotePeer)
		_ = s.Reset()
		return
	}

	reader := ggio.NewDelimitedReader(s, 1<<20)

	go ds.readInboundStream(s, reader, remotePeer)
}

func (ds *directSender) readInboundStream(s network.Stream, reader ggio.ReadCloser, remotePeer peer.ID) {
	defer ds.releaseInboundStream(remotePeer)

	if !ds.refreshReadDeadline(s, remotePeer) {
		return
	}

	consecutiveFailures := 0

	for {
		msg := &pubsubPb.Message{}

		err := reader.ReadMsg(msg)
		if err != nil {
			//stream has encountered an error, close this go routine

			ds.closeAfterReadError(s, err)
			return
		}

		if ds.processInboundFrame(msg, s) != nil {
			consecutiveFailures++
			if ds.resetOnRepeatedInvalidFrames(s, remotePeer, consecutiveFailures) {
				return
			}

			continue
		}

		consecutiveFailures = 0

		if !ds.refreshReadDeadline(s, remotePeer) {
			return
		}
	}
}

func (ds *directSender) processInboundFrame(msg *pubsubPb.Message, s network.Stream) error {
	err := ds.validateDirectMessage(msg, s.Conn().RemotePeer())
	if err != nil {
		log.Trace("p2p direct message rejected", "error", err.Error())
		return err
	}

	pbMessage := &pubsub.Message{Message: msg}
	err = ds.messageHandler(pbMessage, core.PeerID(s.Conn().RemotePeer()))
	if err != nil {
		log.Trace("p2p direct message handler failed", "error", err.Error())
		return err
	}

	return nil
}

// Idle deadline, refreshed only after a frame is accepted: a peer that stalls, or that
// only ever emits frames we reject, gets reset. Refreshing before every read would make
// this a liveness check rather than a usefulness one, letting a garbage-frame stream hold
// its slot indefinitely. Fail closed: without an enforceable deadline the reader cannot
// be bounded.
func (ds *directSender) refreshReadDeadline(s network.Stream, remotePeer peer.ID) bool {
	errDeadline := s.SetReadDeadline(time.Now().Add(ds.recvIdleTimeout))
	if errDeadline == nil {
		return true
	}

	log.Trace("cannot set read deadline on inbound direct stream, resetting stream",
		"peer", remotePeer, "error", errDeadline.Error())
	_ = s.Reset()
	return false
}

func (ds *directSender) closeAfterReadError(s network.Stream, err error) {
	if err != io.EOF {
		_ = s.Reset()
		log.Trace("error reading rpc",
			"from", s.Conn().RemotePeer(),
			"error", err.Error(),
		)
		return
	}

	// Just be nice. They probably won't read this
	// but it doesn't hurt to send it.
	_ = s.Close()
}

func (ds *directSender) resetOnRepeatedInvalidFrames(s network.Stream, remotePeer peer.ID, consecutiveFailures int) bool {
	if consecutiveFailures < ds.maxInvalidFrames {
		return false
	}

	log.Debug("resetting inbound direct stream after repeated invalid frames",
		"peer", remotePeer, "failures", consecutiveFailures)
	_ = s.Reset()
	return true
}

func (ds *directSender) tryAcquireInboundStream(p peer.ID) bool {
	ds.mutInboundStreams.Lock()
	defer ds.mutInboundStreams.Unlock()

	if ds.inboundStreams[p] >= ds.recvStreamsPerPeer {
		log.Debug("refusing inbound direct stream: per-peer cap reached",
			"peer", p, "cap", ds.recvStreamsPerPeer)
		return false
	}
	if ds.totalInboundStreams >= ds.recvStreamsTotalCap {
		// Node-wide saturation is the operationally interesting one: direct send carries resolver
		// replies, so a saturated cap means this node stops receiving chain data it asked for.
		// The cap is first-come with no eviction and nothing reserved for peers not already
		// holding a slot, so a source that can mint identities takes all of it and later peers are
		// refused indefinitely. Bounding that needs an eviction policy, which is not settled; this
		// warning is what makes the condition visible in the meantime.
		log.Warn("refusing inbound direct stream: node-wide cap reached",
			"peer", p, "cap", ds.recvStreamsTotalCap, "held", ds.totalInboundStreams)
		return false
	}

	ds.inboundStreams[p]++
	ds.totalInboundStreams++
	return true
}

func (ds *directSender) releaseInboundStream(p peer.ID) {
	ds.mutInboundStreams.Lock()
	defer ds.mutInboundStreams.Unlock()

	ds.totalInboundStreams--
	ds.inboundStreams[p]--
	if ds.inboundStreams[p] <= 0 {
		delete(ds.inboundStreams, p)
	}
}

// validateDirectMessage covers the frame-level checks the reader can make on its own, before the
// message reaches any application handler. These, together with a handler that actually took the
// frame, are what count as use of the stream: a handler error — in practice an unregistered topic —
// feeds the same consecutive-failure counter as an invalid frame, so a peer emitting well-formed
// frames on a topic nobody serves no longer refreshes its deadline forever. What still counts as
// use is a processor rejecting a well-formed frame on a registered topic, since directMessageHandler
// returns nil whatever the processor reports. That is deliberate: those are policy rejections
// (antiflood, not-found) and must not evict an honest peer. Fully bounding slot-holding still needs
// the eviction policy the node-wide cap is missing.
func (ds *directSender) validateDirectMessage(message *pubsubPb.Message, fromConnectedPeer peer.ID) error {
	if message == nil {
		return p2p.ErrNilMessage
	}
	if message.Topic == nil {
		return p2p.ErrNilTopic
	}
	if !bytes.Equal(message.GetFrom(), []byte(fromConnectedPeer)) {
		return fmt.Errorf("%w mismatch between From and fromConnectedPeer values", p2p.ErrInvalidValue)
	}
	if len(message.GetSeqno()) != seqnoLength {
		return fmt.Errorf("%w unexpected seqno length %d", p2p.ErrInvalidValue, len(message.GetSeqno()))
	}
	if ds.checkAndSetSeenMessage(message) {
		return p2p.ErrAlreadySeenMessage
	}

	return nil
}

func (ds *directSender) checkAndSetSeenMessage(msg *pubsubPb.Message) bool {
	msgId := string(msg.GetFrom()) + string(msg.GetSeqno())

	ds.mutSeenMessages.Lock()
	defer ds.mutSeenMessages.Unlock()

	if ds.seenMessages.Has(msgId) {
		return true
	}

	ds.seenMessages.Add(msgId)
	return false
}

// NextSeqno returns the next uint64 found in *counter as byte slice
func (ds *directSender) NextSeqno() []byte {
	seqno := make([]byte, seqnoLength)
	newVal := atomic.AddUint64(&ds.counter, 1)
	binary.BigEndian.PutUint64(seqno, newVal)
	return seqno
}

// Send will send a direct message to the connected peer
func (ds *directSender) Send(topic string, buff []byte, peer core.PeerID) error {
	if len(buff) >= maxSendBuffSize {
		return fmt.Errorf("%w, to be sent: %d, maximum: %d", p2p.ErrMessageTooLarge, len(buff), maxSendBuffSize)
	}

	mut := ds.mutexForPeer.Get(string(peer))
	mut.Lock()
	defer mut.Unlock()

	conn, err := ds.getConnection(peer)
	if err != nil {
		return err
	}

	stream, reused, err := ds.getOrCreateStream(conn)
	if err != nil {
		return err
	}

	msg := ds.createMessage(topic, buff, conn)

	err = ds.writeMessage(stream, msg)
	if err == nil {
		return nil
	}

	// A reused stream may already have been reset by the remote side: libp2p's swarm only drops a
	// stream from conn.GetStreams() on a local Close/Reset, so a remote RST leaves a corpse that
	// getOrCreateStream will happily hand back. Retry once on a freshly opened stream before
	// reporting failure, otherwise the first send after the receiver's idle timeout is always lost.
	if !reused || isTimeoutError(err) {
		return err
	}

	log.Trace("direct send retrying on a fresh stream", "peer", conn.RemotePeer(), "error", err)

	stream, err = ds.newStream(conn)
	if err != nil {
		return err
	}

	return ds.writeMessage(stream, msg)
}

func isTimeoutError(err error) bool {
	var netErr net.Error

	return errors.As(err, &netErr) && netErr.Timeout()
}

func (ds *directSender) writeMessage(stream network.Stream, msg *pubsubPb.Message) error {
	errDeadline := stream.SetWriteDeadline(time.Now().Add(ds.sendWriteTimeout))
	if errDeadline != nil {
		// Fail closed, as the inbound reader does: without a deadline the write is unbounded and a
		// peer that accepts but never reads can pin the writer indefinitely.
		log.Trace("could not set direct send write deadline", "error", errDeadline)
		_ = stream.Reset()
		return errDeadline
	}

	bufw := bufio.NewWriter(stream)
	w := ggio.NewDelimitedWriter(bufw)

	err := w.WriteMsg(msg)
	if err != nil {
		_ = stream.Reset()
		_ = stream.Close()
		return err
	}

	err = bufw.Flush()
	if err != nil {
		_ = stream.Reset()
		_ = stream.Close()
		return err
	}

	// Clear the deadline on the reused stream.
	_ = stream.SetWriteDeadline(time.Time{})

	return nil
}

func (ds *directSender) getConnection(p core.PeerID) (network.Conn, error) {
	conns := ds.hostP2P.Network().ConnsToPeer(peer.ID(p))
	if len(conns) == 0 {
		return nil, p2p.ErrPeerNotDirectlyConnected
	}

	//return the connection that has the highest number of streams
	lStreams := 0
	var conn network.Conn
	for _, c := range conns {
		length := len(c.GetStreams())
		if length >= lStreams {
			lStreams = length
			conn = c
		}
	}

	return conn, nil
}

func (ds *directSender) getOrCreateStream(conn network.Conn) (network.Stream, bool, error) {
	streams := conn.GetStreams()
	var foundStream network.Stream
	for i := 0; i < len(streams); i++ {
		isExpectedStream := streams[i].Protocol() == DirectSendID
		isSendableStream := streams[i].Stat().Direction == network.DirOutbound

		if isExpectedStream && isSendableStream {
			foundStream = streams[i]
			break
		}
	}

	if foundStream != nil {
		return foundStream, true, nil
	}

	newStream, err := ds.newStream(conn)
	if err != nil {
		return nil, false, err
	}

	return newStream, false, nil
}

func (ds *directSender) newStream(conn network.Conn) (network.Stream, error) {
	return ds.hostP2P.NewStream(ds.ctx, conn.RemotePeer(), DirectSendID)
}

func (ds *directSender) createMessage(topic string, buff []byte, conn network.Conn) *pubsubPb.Message {
	seqno := ds.NextSeqno()
	mes := pubsubPb.Message{}
	mes.Data = buff
	mes.Topic = &topic
	mes.From = []byte(conn.LocalPeer())
	mes.Seqno = seqno

	return &mes
}

// IsInterfaceNil returns true if there is no value under the interface
func (ds *directSender) IsInterfaceNil() bool {
	return ds == nil
}
