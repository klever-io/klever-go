package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/klever-io/klever-go/cmd/operator/utils"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/statusHandler"
	atomicPkg "github.com/klever-io/klever-go/tools/atomic"
	"github.com/klever-io/klever-go/tools/check"
)

var log = logger.GetOrCreate("websocket")

const (
	errInvalidParams    = "invalid params: "
	errInvalidSubType   = "invalid subscription type: "
	errFacadeUnavail    = "query not supported: facade unavailable"
	errUnknownMethod    = "unknown method: "
	errMissingHash      = "missing required param: hash"
	errMissingNonceHash = "must provide nonce or hash"
	errTxNotFound       = "transaction not found"
	errBlockNotFound    = "block not found"
	// errInternal is the opaque answer for a panicked request. The value stays in the log (KLC-2596).
	errInternal = "internal error"

	// Client goroutines with their own stack budget. See panicWarner.
	opLoopIn              = "ws.loopIn"
	opLoopOut             = "ws.loopOut"
	opHandleClientRequest = "ws.HandleClientRequest"

	postQueueDropLogIntervalSeconds = 10

	// peerFailLogOp labels the shutdown summaries for the peer-driven failure budgets.
	peerFailLogOp = "ws.peerFailures"
)

var (
	// ErrClientClosed is returned by HandleClientInsertion when the client's connection is
	// already being torn down. Classify it with errors.Is, not by string equality.
	ErrClientClosed = errors.New("client connection is closed")
	// ErrHubClosed is returned by HandleClientInsertion once the hub has shut down.
	ErrHubClosed = errors.New("websocket hub is shutting down")
)

// PeerDrivenLogWindow is the window behind every peer-driven warning: one line per window,
// carrying the count of what was folded into it. Shared with the /log routes, which have the
// same class of event and no hub of their own.
const PeerDrivenLogWindow = postQueueDropLogIntervalSeconds * time.Second

// dropWarner rate-limits a recurring warning behind a fixed window: every occurrence
// calls fire(), which folds the count since the last log into one summary line at most
// once per window instead of logging every single occurrence. Deliberately a per-instance
// value (unlike indexer.trySendEvent's package-global rate-limit vars, which use the same
// pattern but are intentionally left alone — see PR discussion) so two independent
// warning sources on the same hub (queue-full drops vs. post failures) can't share, or
// reset, each other's windows.
type dropWarner struct {
	count      atomicPkg.Counter
	lastLogged int64 // unix seconds; accessed only via sync/atomic
	windowSecs int64
}

// DropWarner is dropWarner for callers outside this package — the /log routes on the node and
// the seednode, and the log sender — which have the same class of peer-driven event and no hub
// of their own. It wraps rather than exports dropWarner so the hub's own call sites stay
// exactly as #68 left them.
type DropWarner struct {
	w dropWarner
}

// NewDropWarner builds a warner for callers outside this package. The zero value of the
// underlying warner is not a rate limiter — a zero window lets every occurrence through — so
// the window is taken explicitly and a non-positive one is clamped to a second rather than
// silently disabling it.
func NewDropWarner(window time.Duration) *DropWarner {
	secs := int64(window / time.Second)
	if secs < 1 {
		secs = 1
	}

	return &DropWarner{w: dropWarner{windowSecs: secs}}
}

// Fire records one occurrence and reports (count, true) with the number folded into the window
// at most once per window; otherwise (0, false). See dropWarner.fire.
func (d *DropWarner) Fire() (int64, bool) {
	return d.w.fire()
}

// Flush reports any occurrences still pending in the current window. See dropWarner.flush.
func (d *DropWarner) Flush() (int64, bool) {
	return d.w.flush()
}

// fire records one occurrence and reports (count, true) with the number of occurrences
// folded into the window (including this one) at most once per windowSecs; otherwise
// reports (0, false).
func (w *dropWarner) fire() (int64, bool) {
	w.count.Increment()
	now := time.Now().Unix()
	last := atomic.LoadInt64(&w.lastLogged)
	if now-last < w.windowSecs {
		return 0, false
	}
	if !atomic.CompareAndSwapInt64(&w.lastLogged, last, now) {
		return 0, false
	}
	return w.count.Reset(), true
}

// flush reports any occurrences still pending in the current window (count > 0) — used
// on worker shutdown so a burst that never reaches another occurrence to trigger the next
// window doesn't sit unreported, or get misattributed to some unrelated later window.
func (w *dropWarner) flush() (int64, bool) {
	count := w.count.Reset()
	return count, count > 0
}

type userOptions struct {
	acceptAccount     bool
	acceptTransaction bool
}

type SocketHub struct {
	mu                      sync.RWMutex
	postConnectionURL       string
	postConnectionAPIKey    string
	facade                  WSFacade
	blockSubscription       map[*client]struct{}
	transactionSubscription map[*client]struct{}
	addressSubscription     map[string]map[*client]userOptions
	clientAddresses         map[*client]int
	limits                  resolvedLimits
	// clients is every client the hub has accepted, so deleteAll can close all of them.
	// The subscription maps alone are not enough: an address-scoped subscribe with an
	// empty address list (or an unsubscribe that empties the last one) leaves a live
	// client in none of them, and shutdown would then never close its socket.
	clients map[*client]struct{}
	// closed is set by deleteAll (under mu) once StartServer's shutdown has torn the hub
	// down. A client created but not yet inserted is in no map and in no clients entry, so
	// deleteAll cannot reach it; without this flag its insertion would register into a dead
	// hub, and its map entries, goroutines and connection-limiter slot would outlive
	// shutdown for as long as the peer holds the socket.
	closed bool
	// postQueue feeds the bounded postWSConnection worker pool; nil when the mirror is
	// disabled (no URL configured — see NewHub), so asyncPost is a no-op and never
	// allocates a goroutine or channel slot for a feature nobody turned on.
	postQueue chan *Send
	// postWorkersWG tracks live post workers so StartServer's shutdown can wait for them
	// to actually exit rather than returning while one is still mid-request.
	postWorkersWG sync.WaitGroup
	// queueDropWarn/postFailWarn are per-hub (not package-global) rate-limited warning
	// state for, respectively, the mirror queue being full and a post to the mirror
	// failing — kept separate so a burst of one kind can't reset or share the other's
	// window.
	queueDropWarn dropWarner
	postFailWarn  dropWarner
	rejectWarn    dropWarner
	// One budget per peer-driven failure source; see logPeerDrivenFailure for why they
	// are not shared.
	upgradeFailWarn   dropWarner
	handshakeFailWarn dropWarner
	readFailWarn      dropWarner
	sendDropWarn      dropWarner
	writeFailWarn     dropWarner
	queryFailWarn     dropWarner
	// One stack budget per barrier, so a flood on one path cannot spend another's.
	// These bound the stack only; the panic line is always logged.
	loopInPanicWarn  dropWarner
	loopOutPanicWarn dropWarner
	requestPanicWarn dropWarner
	otherPanicWarn   dropWarner
	// appStatusHandler exports the mirror's cumulative drop/failure counts (see
	// MetricWSMirrorQueueDroppedTotal/MetricWSMirrorPostFailuresTotal) alongside the
	// rate-limited WARN logs above — the log is a periodic sample, this is the exact
	// total. Defaults to a no-op handler (see NewHub) so it's always safe to call without
	// a nil check; SetAppStatusHandler swaps in a real one.
	appStatusHandler core.AppStatusHandler
}

// SetAppStatusHandler wires ash in to receive the mirror's drop/failure counters. Safe to
// call with a nil ash (no-op, keeps the current handler) so a caller can pass through
// whatever it has without its own nil check.
func (h *SocketHub) SetAppStatusHandler(ash core.AppStatusHandler) {
	if check.IfNil(ash) {
		return
	}
	h.appStatusHandler = ash
}

func NewHub(postConnectionURL, postConnectionAPIKey string, facade WSFacade, limits ...Limits) *SocketHub {
	var l Limits
	if len(limits) > 0 {
		l = limits[0]
	}
	resolved := l.resolve()

	// The mirror requires a URL: an API key with no URL can never succeed (every request
	// fails inside the HTTP client with "no Host in request URL"), so treating URL as the
	// single source of truth for whether the mirror is enabled means postWSConnection's
	// own now-removed URL/key check could never disagree with it.
	var postQueue chan *Send
	if postConnectionURL != "" {
		postQueue = make(chan *Send, resolved.postQueueSize)
	} else if postConnectionAPIKey != "" {
		log.Warn("ws.NewHub", "msg", "postConnectionAPIKey set without postConnectionURL; mirror stays disabled")
	}

	return &SocketHub{
		clients:                 make(map[*client]struct{}),
		addressSubscription:     make(map[string]map[*client]userOptions),
		clientAddresses:         make(map[*client]int),
		blockSubscription:       make(map[*client]struct{}),
		transactionSubscription: make(map[*client]struct{}),
		postConnectionURL:       postConnectionURL,
		postConnectionAPIKey:    postConnectionAPIKey,
		facade:                  facade,
		limits:                  resolved,
		postQueue:               postQueue,
		queueDropWarn:           dropWarner{windowSecs: postQueueDropLogIntervalSeconds},
		postFailWarn:            dropWarner{windowSecs: postQueueDropLogIntervalSeconds},
		rejectWarn:              dropWarner{windowSecs: postQueueDropLogIntervalSeconds},
		upgradeFailWarn:         dropWarner{windowSecs: postQueueDropLogIntervalSeconds},
		handshakeFailWarn:       dropWarner{windowSecs: postQueueDropLogIntervalSeconds},
		readFailWarn:            dropWarner{windowSecs: postQueueDropLogIntervalSeconds},
		sendDropWarn:            dropWarner{windowSecs: postQueueDropLogIntervalSeconds},
		writeFailWarn:           dropWarner{windowSecs: postQueueDropLogIntervalSeconds},
		queryFailWarn:           dropWarner{windowSecs: postQueueDropLogIntervalSeconds},
		loopInPanicWarn:         dropWarner{windowSecs: postQueueDropLogIntervalSeconds},
		loopOutPanicWarn:        dropWarner{windowSecs: postQueueDropLogIntervalSeconds},
		requestPanicWarn:        dropWarner{windowSecs: postQueueDropLogIntervalSeconds},
		otherPanicWarn:          dropWarner{windowSecs: postQueueDropLogIntervalSeconds},
		appStatusHandler:        statusHandler.NewNilStatusHandler(),
	}
}

// MaxMessageSize returns the inbound WebSocket frame read limit (bytes).
func (h *SocketHub) MaxMessageSize() int64 {
	return h.limits.maxMessageSize
}

// MaxAddressesPerSubscribe returns the per-call address cap for a subscribe request.
func (h *SocketHub) MaxAddressesPerSubscribe() int {
	return h.limits.maxAddressesPerSubscribe
}

// asyncPost hands parsed to the bounded post-mirror worker pool. It is a pure no-op when
// the mirror isn't configured, and drops (with a rate-limited warning) rather than block
// or grow without bound when postWorkerCount workers are all busy with a slow endpoint.
func (h *SocketHub) asyncPost(parsed *Send) {
	if h.postQueue == nil {
		return
	}
	select {
	case h.postQueue <- parsed:
	default:
		h.appStatusHandler.Increment(core.MetricWSMirrorQueueDroppedTotal)
		if count, ok := h.queueDropWarn.fire(); ok {
			log.Warn("ws.EventReceive.postWSConnection", "msg", "mirror queue full, dropping events", "droppedCount", count)
		}
	}
}

// startPostWorkers runs postWorkerCount goroutines draining postQueue until ctx is done.
// No-op when the mirror is disabled (postQueue == nil). Each worker is tracked in
// postWorkersWG so StartServer's shutdown can wait for them to actually exit — ctx is
// threaded into the HTTP call itself (via postWSConnection) so an in-flight request is
// aborted promptly on cancellation instead of running out its full timeout.
func (h *SocketHub) startPostWorkers(ctx context.Context) {
	if h.postQueue == nil {
		return
	}
	for i := 0; i < h.limits.postWorkers; i++ {
		h.postWorkersWG.Add(1)
		go func() {
			defer h.postWorkersWG.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case parsed := <-h.postQueue:
					// select has no case priority: ctx.Done() and postQueue can both be
					// ready at once, so a cancelled shutdown can still dequeue one more
					// item. Check ctx.Err() before posting instead of starting (and
					// immediately failing) a doomed request.
					if ctx.Err() != nil {
						log.Warn("ws.EventReceive.postWSConnection", "msg", "shutdown: abandoning mirror queue", "pending", len(h.postQueue)+1)
						return
					}
					if err := h.postWSConnection(ctx, parsed); err != nil {
						h.appStatusHandler.Increment(core.MetricWSMirrorPostFailuresTotal)
						if count, ok := h.postFailWarn.fire(); ok {
							log.Warn("ws.EventReceive.postWSConnection", "msg", "failed to post to mirror", "failedCount", count, "lastError", err.Error())
						}
					}
				}
			}
		}()
	}
}

func (h *SocketHub) notifyAddressSubscribers(address string, parsed *Send, filterFn func(userOptions) bool) {
	h.mu.RLock()
	original, ok := h.addressSubscription[address]
	if !ok {
		h.mu.RUnlock()
		return
	}
	snapshot := make(map[*client]userOptions, len(original))
	for c, opts := range original {
		snapshot[c] = opts
	}
	h.mu.RUnlock()

	for c, opts := range snapshot {
		if c.IsAlive() && filterFn(opts) {
			c.send(parsed)
		}
	}
}

func (h *SocketHub) broadcastToSubscription(parsed *Send, subscription map[*client]struct{}) {
	h.mu.RLock()
	snapshot := make([]*client, 0, len(subscription))
	for c := range subscription {
		snapshot = append(snapshot, c)
	}
	h.mu.RUnlock()

	for _, c := range snapshot {
		if c.IsAlive() {
			c.send(parsed)
		}
	}
}

func (h *SocketHub) marshalAndPost(evType indexer.EventType, address, hash string, message interface{}) *Send {
	parsed, err := marshalMessage(evType, address, hash, message)
	if err != nil {
		log.Error("ws.EventReceive", "cannot marshal message", err.Error())
		return nil
	}
	h.asyncPost(parsed)
	return parsed
}

func (h *SocketHub) StartServer(ctx context.Context) {
	// deleteAll sets closed on shutdown and nothing else clears it, so a hub whose context
	// was cancelled would reject every insertion forever. Clear it here rather than leaving
	// the type single-use — the drop-warner flush below already reasons about hub reuse.
	//
	// This makes a hub restartable after a previous StartServer has RETURNED, not while one
	// is still running. Overlapping calls are still wrong and always were: cancel A, start B
	// before A reaches its teardown, and A's deleteAll then closes B's clients and leaves
	// closed set under a running B — and A's postWorkersWG.Wait() can block on B's workers.
	// Serialising starts is the caller's contract; no production caller overlaps them
	// (network/api/api.go builds a fresh hub per registration).
	h.mu.Lock()
	h.closed = false
	h.mu.Unlock()

	h.startPostWorkers(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Info("delete all client and close gracefully")
			h.deleteAll()
			h.postWorkersWG.Wait()
			// A burst that never reaches another occurrence to trigger fire()'s next
			// window would otherwise sit unreported in the counter — possibly forever, if
			// the hub shuts down before the next log fires — or get misattributed to some
			// unrelated later window on hub reuse. Flush both here, once, after every
			// worker has actually exited.
			if count, ok := h.queueDropWarn.flush(); ok {
				log.Warn("ws.EventReceive.postWSConnection", "msg", "mirror queue full, dropping events (final)", "droppedCount", count)
			}
			if count, ok := h.postFailWarn.flush(); ok {
				log.Warn("ws.EventReceive.postWSConnection", "msg", "failed to post to mirror (final)", "failedCount", count)
			}
			// Same reasoning for the peer-driven budgets: without this, the tail of a burst
			// that stops before the next window opens is never reported at all.
			//
			// Best effort, not exhaustive: deleteAll closes clients but does not join their
			// reader, writer and request goroutines, so an occurrence that lands after this
			// point — a query still inside the facade, say — is counted into a window that
			// nothing will report. Joining them would need a lifecycle these types do not
			// have today.
			for _, final := range []struct {
				warner *dropWarner
				msg    string
			}{
				{&h.upgradeFailWarn, "websocket upgrades failed (final)"},
				{&h.handshakeFailWarn, "subscribe handshakes failed (final)"},
				{&h.readFailWarn, "connection reads failed without a close frame (final)"},
				{&h.sendDropWarn, "responses dropped on full client buffers (final)"},
				{&h.writeFailWarn, "connection writes failed (final)"},
				{&h.queryFailWarn, "client queries failed (final)"},
				{&h.rejectWarn, "subscription inserts rejected at a cap (final)"},
				// Omitted stacks only; the panic line was already logged.
				{&h.loopInPanicWarn, "reader-loop panic stacks omitted (final)"},
				{&h.loopOutPanicWarn, "writer-loop panic stacks omitted (final)"},
				{&h.requestPanicWarn, "client-request panic stacks omitted (final)"},
				{&h.otherPanicWarn, "other client panic stacks omitted (final)"},
			} {
				if count, ok := final.warner.flush(); ok {
					log.Warn(peerFailLogOp, "msg", final.msg, "count", count)
				}
			}
			return
		case event := <-indexer.EventQueue:
			h.mu.RLock()
			blockCount, txCount := len(h.blockSubscription), len(h.transactionSubscription)
			h.mu.RUnlock()
			log.Debug("ws.EventReceived", "type", string(event.EvType), "blockSubs", blockCount, "txSubs", txCount)
			switch event.EvType {
			case indexer.ACCOUNTS:
				h.handleAccountsEvent(event)
			case indexer.USER_TRANSACTIONS:
				h.handleUserTransactionEvent(event)
			case indexer.TRANSACTIONS:
				h.handleBroadcastEvent(event, h.transactionSubscription)
			default:
				h.handleBroadcastEvent(event, h.blockSubscription)
			}
		}
	}
}

func (h *SocketHub) handleAccountsEvent(event indexer.Event) {
	accounts := event.Message.(map[string]*data.AccountInfo)
	acceptAccount := func(opts userOptions) bool { return opts.acceptAccount }
	for account, info := range accounts {
		parsed := h.marshalAndPost(event.EvType, account, "", info)
		if parsed == nil {
			continue
		}
		h.notifyAddressSubscribers(account, parsed, acceptAccount)
	}
}

func (h *SocketHub) handleUserTransactionEvent(event indexer.Event) {
	transactions := event.Message.([]*data.Transaction)
	acceptTx := func(opts userOptions) bool { return opts.acceptTransaction }
	for _, tx := range transactions {
		var wg sync.WaitGroup
		wg.Add(3)
		go h.notifyTxSender(&wg, event.EvType, tx, acceptTx)
		go h.notifyTxReceipts(&wg, event.EvType, tx, acceptTx)
		go h.postTxHash(&wg, event.EvType, tx)
		wg.Wait()
	}
}

func (h *SocketHub) notifyTxSender(wg *sync.WaitGroup, evType indexer.EventType, tx *data.Transaction, acceptTx func(userOptions) bool) {
	defer wg.Done()
	parsed := h.marshalAndPost(evType, tx.Sender, "", tx)
	if parsed == nil {
		return
	}
	h.notifyAddressSubscribers(tx.Sender, parsed, acceptTx)
}

func (h *SocketHub) notifyTxReceipts(wg *sync.WaitGroup, evType indexer.EventType, tx *data.Transaction, acceptTx func(userOptions) bool) {
	defer wg.Done()
	for _, receipts := range tx.Receipts {
		to := receipts["to"]
		if to == nil {
			continue
		}
		address, ok := to.(string)
		if !ok {
			continue
		}
		parsed := h.marshalAndPost(evType, address, "", tx)
		if parsed == nil {
			continue
		}
		h.notifyAddressSubscribers(address, parsed, acceptTx)
	}
}

func (h *SocketHub) postTxHash(wg *sync.WaitGroup, evType indexer.EventType, tx *data.Transaction) {
	defer wg.Done()
	h.marshalAndPost(evType, "", tx.Hash, tx)
}

func (h *SocketHub) handleBroadcastEvent(event indexer.Event, subscription map[*client]struct{}) {
	parsed := h.marshalAndPost(event.EvType, "", "", event.Message)
	if parsed == nil {
		return
	}
	h.broadcastToSubscription(parsed, subscription)
}

// postWSConnection is only ever invoked from startPostWorkers's dequeue loop, which only
// runs at all when postQueue != nil — and NewHub only allocates postQueue when
// postConnectionURL is set — so postConnectionURL is guaranteed non-empty here; no local
// empty-config guard needed (the direct-call tests exercise that invariant instead, see
// TestNewHub_PostQueueAllocatedOnlyWhenConfigured).
func (h *SocketHub) postWSConnection(ctx context.Context, message *Send) error {
	b, err := json.Marshal(message)
	if err != nil {
		return err
	}

	return utils.PostURLWithContext(ctx, h.postConnectionURL, string(b), []string{"x-api-key", h.postConnectionAPIKey}, nil)
}

type Send struct {
	Type    indexer.EventType `json:"type"`
	Address string            `json:"address"`
	Hash    string            `json:"hash"`
	Data    json.RawMessage   `json:"data"`
}

func marshalMessage(evType indexer.EventType, address string, hash string, message interface{}) (*Send, error) {
	var messageBytes []byte
	if evType == indexer.BLOCKS {
		m, ok := message.([]byte)
		if !ok {
			err := errors.New("failed to convert block")
			log.Error("ws.Send", "err", err.Error())
			return nil, err
		}
		messageBytes = m
	} else {
		m, err := json.Marshal(&message)
		if err != nil {
			log.Error("ws.Send", "err", err.Error())
			return nil, err
		}
		messageBytes = m
	}

	return &Send{
		Type:    evType,
		Address: address,
		Hash:    hash,
		Data:    messageBytes,
	}, nil
}

// logPeerDrivenFailure folds a failure a peer can provoke at will into at most one line per
// window: one line per occurrence would hand a peer control of the node's log volume.
//
// Each source owns its budget rather than sharing one, because only the caller that opens a
// window has its op and err logged — every other occurrence survives as a bare count under
// that caller's message. Sharing would let a flood of the cheapest failure (a bare GET to
// /subscribe) hide the identity of a rarer one (an established connection's read fault) for
// as long as the flood lasts, not merely for one window.
//
// Warn rather than Error because these are usually the peer's doing, though not always: an
// upgrade can fail on this side, and a transport error does not say which end broke.
func (h *SocketHub) logPeerDrivenFailure(warner *dropWarner, op string, err error) {
	if count, ok := warner.fire(); ok {
		log.Warn(op, "err", loggableError(err), "similarSinceLastLog", count)
	}
}

// LogUpgradeFailure reports a websocket upgrade that never completed — a request with no
// upgrade headers reaches it, so anyone who can open a TCP connection can drive it.
func (h *SocketHub) LogUpgradeFailure(op string, err error) {
	h.logPeerDrivenFailure(&h.upgradeFailWarn, op, err)
}

// LogHandshakeFailure reports a subscribe handshake that never produced a usable request:
// malformed JSON, a wrong-typed field, or a close before it arrived.
func (h *SocketHub) LogHandshakeFailure(op string, err error) {
	h.logPeerDrivenFailure(&h.handshakeFailWarn, op, err)
}

// logReadFailure reports a read on an established connection that failed with no close
// frame at all. Gorilla's IsUnexpectedCloseError answers false for every one of these, so
// before they were routed here they were dropped silently rather than logged.
func (h *SocketHub) logReadFailure(op string, err error) {
	h.logPeerDrivenFailure(&h.readFailWarn, op, err)
}

// logSendDrop reports a response dropped because the client's output buffer was full. A
// peer that keeps sending requests while never draining its socket fills the buffer and
// then earns a line per dropped response, so this needs the same bound as the rest.
func (h *SocketHub) logSendDrop(op string, msg string) {
	if count, ok := h.sendDropWarn.fire(); ok {
		log.Warn(op, "msg", msg, "similarSinceLastLog", count)
	}
}

// logWriteFailure reports a failed write or ping on an established connection. Each one
// tears the connection down, so it is bounded per connection — but a peer reconnecting in
// a loop repeats it, and that is what the shared window bounds.
func (h *SocketHub) logWriteFailure(op string, err error) {
	h.logPeerDrivenFailure(&h.writeFailWarn, op, err)
}

// logQueryFailure reports a failed get_transaction/get_block lookup. Asking for something
// that is not there is ordinary client behaviour and a peer can repeat it indefinitely on
// one connection, so the volume is bounded and the peer-supplied hash is not logged raw:
// it is capped only by the inbound message limit and could otherwise carry newlines into
// the log. See loggableHash.
func (h *SocketHub) logQueryFailure(op string, key string, value interface{}, err error) {
	if count, ok := h.queryFailWarn.fire(); ok {
		log.Warn(op, key, value, "err", loggableError(err), "similarSinceLastLog", count)
	}
}

// loggableError renders an error the peer had a hand in producing. Sanitising only the
// fields we choose is not enough, because the error text carries peer input back out on its
// own: a storage miss quotes the whole key it was handed, so bounding the hash field alone
// still let a 100KB hash reach the log through err. A websocket close reason arrives the
// same way — gorilla puts it verbatim in CloseError.Error(), and the logger's formatter
// passes CR/LF through, so a reason of "\nWARN ..." forges a log line of its own.
func loggableError(err error) string {
	const maxLoggableErrorLength = 256

	if err == nil {
		return ""
	}

	return loggableText(err.Error(), maxLoggableErrorLength)
}

// loggableText bounds and scrubs one field of text that peer input may have reached. Shared
// by loggableError and loggablePanic, which differ only in where the text comes from.
func loggableText(text string, maxLength int) string {
	if len(text) > maxLength {
		text = text[:maxLength] + fmt.Sprintf("… (%d bytes)", len(text))
	}

	// Anything that could end a log line becomes a space: CR/LF, the other control runes,
	// and the Unicode line/paragraph separators, which IsControl does not cover and some
	// aggregators treat as newlines. This guards line structure only. The logger renders
	// "name = value", so an "=" inside a value can still read as an extra field to a human;
	// mapping it would mangle ordinary errors ("key=value not found") to defend against a
	// far weaker forgery than a line break, so it is left alone.
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return ' '
		}

		return r
	}, text)
}

// loggablePanic scrubs a recovered panic value. It can quote peer input, so it gets the
// same bound and control-rune scrub as loggableError (KLC-2596).
func loggablePanic(r interface{}) string {
	const maxLoggablePanicLength = 256

	return loggableText(fmt.Sprintf("%v", r), maxLoggablePanicLength)
}

// panicWarner is the stack budget for one barrier. An unknown op shares otherPanicWarn.
func (h *SocketHub) panicWarner(op string) *dropWarner {
	switch op {
	case opLoopIn:
		return &h.loopInPanicWarn
	case opLoopOut:
		return &h.loopOutPanicWarn
	case opHandleClientRequest:
		return &h.requestPanicWarn
	default:
		return &h.otherPanicWarn
	}
}

// logRecoveredPanic logs a panic from a client goroutine outside gin.Recovery (KLC-2596).
// The panic value is scrubbed. The stack is kept for the first panic of the window;
// stacksOmitted is count-1 because this line's own stack is attached.
func (h *SocketHub) logRecoveredPanic(op string, r interface{}) {
	if count, ok := h.panicWarner(op).fire(); ok {
		log.Error(op+" panicked",
			"panic", loggablePanic(r),
			"stack", string(debug.Stack()),
			"stacksOmitted", count-1,
		)
		return
	}

	log.Error(op+" panicked",
		"panic", loggablePanic(r),
		"stack", "omitted; attached to the first panic of this window on this path",
	)
}

// loggableHash renders a peer-supplied hash safely: a well-formed one is returned as-is,
// and anything else is reduced to its length. A hash is hex, so a value that is not is
// already not a hash — logging its bytes would only put peer-controlled content, newlines
// included, into the log.
func loggableHash(hash string) string {
	const maxLoggableHashLength = 64

	if len(hash) > maxLoggableHashLength {
		return fmt.Sprintf("<%d bytes>", len(hash))
	}

	for _, r := range hash {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
		if !isHex {
			return fmt.Sprintf("<%d bytes, not hex>", len(hash))
		}
	}

	return hash
}

// LogRejectedInsertion reports a rejected insertion at the level its cause deserves.
// Losing a race with teardown is routine and peer-triggerable, so it stays at Debug. The
// cap rejections are the abuse signal (GHSA-4fwh-wrm6-97xm) and stay at Warn, where a
// default *:INFO node still sees them — but a peer can provoke them too, so they are
// folded into one summary line per window instead of one line per occurrence.
func (h *SocketHub) LogRejectedInsertion(op string, err error) {
	if errors.Is(err, ErrClientClosed) || errors.Is(err, ErrHubClosed) {
		log.Debug(op, "err", err.Error())
		return
	}

	if count, ok := h.rejectWarn.fire(); ok {
		log.Warn(op, "err", loggableError(err), "rejectedSinceLastLog", count)
	}
}

// RemoveClient closes c and removes it from the hub. It runs the teardown inline rather
// than handing it to StartServer, so it stays safe to call once the hub has shut down.
func (h *SocketHub) RemoveClient(c *client) {
	h.handleClientDelete(c)
}

// ValidateSubscription applies the input caps HandleClientInsertion enforces before it
// touches any state. It is exported so a route can reject a bad subscribe with a reason
// frame while it still owns the raw connection — once NewClient has started loopIn/loopOut
// there is no longer a safe way to write one. HandleClientInsertion repeats the checks
// rather than trusting callers: it is also reached from handleDynamicSubscribe.
func (h *SocketHub) ValidateSubscription(eventType []indexer.EventType, addresses []string) error {
	// Addresses are only meaningful for the address-scoped types (ACCOUNTS,
	// USER_TRANSACTIONS). A blocks/transactions-only subscribe must not count or
	// store them, or it would burn the per-connection address budget on entries
	// that never match anything (GHSA-4fwh-wrm6-97xm, Impact C).
	if !containsAddressScoped(eventType) {
		return nil
	}

	if len(addresses) > h.limits.maxAddressesPerSubscribe {
		return fmt.Errorf("too many addresses in a single subscribe: %d (max %d)", len(addresses), h.limits.maxAddressesPerSubscribe)
	}

	// Bound each address by byte size before it is retained as a subscription key. The count
	// caps alone leave a memory-amplification path: a long-lived connection could otherwise
	// keep sending unique oversized strings that can never match a real address yet are
	// retained per connection (GHSA-4fwh-wrm6-97xm).
	for _, address := range addresses {
		if len(address) > maxEncodedAddressLength {
			return fmt.Errorf("subscription address exceeds the maximum length of %d bytes", maxEncodedAddressLength)
		}
	}

	return nil
}

func (h *SocketHub) HandleClientInsertion(eventType []indexer.EventType, addresses []string, c *client) error {
	wantsAddresses := containsAddressScoped(eventType)

	if err := h.ValidateSubscription(eventType, addresses); err != nil {
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return ErrHubClosed
	}

	if !c.IsAlive() {
		return ErrClientClosed
	}

	// Reject before mutating if the per-connection cap would be exceeded; only addresses
	// new to this client count.
	if wantsAddresses && h.clientAddresses[c]+h.countNewAddresses(addresses, c) > h.limits.maxAddressesPerClient {
		return fmt.Errorf("address subscription limit reached for this connection (max %d)", h.limits.maxAddressesPerClient)
	}

	// Track the client itself, not just its subscriptions: an address-scoped subscribe with
	// an empty address list writes to no map, and deleteAll must still close it.
	h.clients[c] = struct{}{}

	acceptAccounts, acceptTransactions := h.applyEventTypes(eventType, c)
	if wantsAddresses {
		h.addAddressSubscriptions(addresses, c, acceptAccounts, acceptTransactions)
	}
	return nil
}

// containsAddressScoped reports whether eventType includes a type for which the
// request's addresses are meaningful (ACCOUNTS or USER_TRANSACTIONS).
func containsAddressScoped(eventType []indexer.EventType) bool {
	for _, t := range eventType {
		if t == indexer.ACCOUNTS || t == indexer.USER_TRANSACTIONS {
			return true
		}
	}
	return false
}

// countNewAddresses returns how many of addresses are not yet watched by c, counting
// duplicates within the call once. The caller must hold h.mu.
func (h *SocketHub) countNewAddresses(addresses []string, c *client) int {
	seen := make(map[string]struct{}, len(addresses))
	count := 0
	for _, address := range addresses {
		if _, dup := seen[address]; dup {
			continue
		}
		seen[address] = struct{}{}
		if inner, ok := h.addressSubscription[address]; ok {
			if _, has := inner[c]; has {
				continue
			}
		}
		count++
	}
	return count
}

// applyEventTypes registers the global block/transaction subscriptions for c and returns
// the per-address accept flags. The caller must hold h.mu.
func (h *SocketHub) applyEventTypes(eventType []indexer.EventType, c *client) (acceptAccounts, acceptTransactions bool) {
	for _, t := range eventType {
		switch t {
		case indexer.BLOCKS:
			h.blockSubscription[c] = struct{}{}
		case indexer.TRANSACTIONS:
			h.transactionSubscription[c] = struct{}{}
		case indexer.USER_TRANSACTIONS:
			acceptTransactions = true
		case indexer.ACCOUNTS:
			acceptAccounts = true
		}
	}
	return acceptAccounts, acceptTransactions
}

// addAddressSubscriptions adds c to each address with the given accept flags, bumping the
// per-client count for newly added (address, client) pairs. The caller must hold h.mu.
func (h *SocketHub) addAddressSubscriptions(addresses []string, c *client, acceptAccounts, acceptTransactions bool) {
	for _, address := range addresses {
		value, ok := h.addressSubscription[address]
		if !ok {
			value = make(map[*client]userOptions)
			h.addressSubscription[address] = value
		}

		if _, has := value[c]; !has {
			h.clientAddresses[c]++
		}

		existing := value[c]
		if acceptAccounts {
			existing.acceptAccount = true
		}
		if acceptTransactions {
			existing.acceptTransaction = true
		}
		value[c] = existing
	}
}

func (h *SocketHub) deleteAll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.closed = true

	for c := range h.clients {
		c.Close()
	}
	clear(h.clients)

	clear(h.addressSubscription)
	clear(h.clientAddresses)
	clear(h.blockSubscription)
	clear(h.transactionSubscription)
}

func (h *SocketHub) handleClientDelete(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.clients, c)
	delete(h.blockSubscription, c)
	delete(h.transactionSubscription, c)
	c.Close()
	// Remove the client from every watched address and reclaim the outer key when its
	// inner map empties. Without this, a disconnect leaks one map entry per address
	// permanently (GHSA-4fwh-wrm6-97xm, Impact C).
	for addr, clients := range h.addressSubscription {
		delete(clients, c)
		if len(clients) == 0 {
			delete(h.addressSubscription, addr)
		}
	}
	delete(h.clientAddresses, c)
}

func (h *SocketHub) HandleClientRequest(c *client, req WSRequest) {
	switch req.Method {
	case MethodGetTransaction:
		h.handleGetTransaction(c, req)
	case MethodGetBlock:
		h.handleGetBlock(c, req)
	case MethodSubscribe:
		h.handleDynamicSubscribe(c, req)
	case MethodUnsubscribe:
		h.handleDynamicUnsubscribe(c, req)
	default:
		c.send(WSResponse{ID: req.ID, Error: errUnknownMethod + req.Method})
	}
}

func (h *SocketHub) handleGetTransaction(c *client, req WSRequest) {
	if h.facade == nil {
		c.send(WSResponse{ID: req.ID, Error: errFacadeUnavail})
		return
	}

	var params GetTransactionParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		c.send(WSResponse{ID: req.ID, Error: errInvalidParams + err.Error()})
		return
	}

	if params.Hash == "" {
		c.send(WSResponse{ID: req.ID, Error: errMissingHash})
		return
	}

	tx, err := h.facade.GetTransaction(params.Hash, params.WithResults)
	if err != nil {
		h.logQueryFailure("ws.handleGetTransaction", "hash", loggableHash(params.Hash), err)
		c.send(WSResponse{ID: req.ID, Error: errTxNotFound})
		return
	}

	c.send(WSResponse{ID: req.ID, Data: tx})
}

func (h *SocketHub) handleGetBlock(c *client, req WSRequest) {
	if h.facade == nil {
		c.send(WSResponse{ID: req.ID, Error: errFacadeUnavail})
		return
	}

	var params GetBlockParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		c.send(WSResponse{ID: req.ID, Error: errInvalidParams + err.Error()})
		return
	}

	if params.Nonce == nil && params.Hash == "" {
		c.send(WSResponse{ID: req.ID, Error: errMissingNonceHash})
		return
	}

	if params.Nonce != nil {
		blk, err := h.facade.GetBlockByNonce(*params.Nonce, params.WithTxs)
		if err != nil {
			h.logQueryFailure("ws.handleGetBlock", "nonce", *params.Nonce, err)
			c.send(WSResponse{ID: req.ID, Error: errBlockNotFound})
			return
		}
		c.send(WSResponse{ID: req.ID, Data: blk})
		return
	}

	blk, err := h.facade.GetBlockByHash(params.Hash, params.WithTxs)
	if err != nil {
		h.logQueryFailure("ws.handleGetBlock", "hash", loggableHash(params.Hash), err)
		c.send(WSResponse{ID: req.ID, Error: errBlockNotFound})
		return
	}
	c.send(WSResponse{ID: req.ID, Data: blk})
}

func parseStrictEventTypes(c *client, reqID string, types []string) ([]indexer.EventType, bool) {
	var eventTypes []indexer.EventType
	for _, t := range types {
		parsed, err := indexer.NewEventTypeStrict(t)
		if err != nil {
			c.send(WSResponse{ID: reqID, Error: errInvalidSubType + t})
			return nil, false
		}
		eventTypes = append(eventTypes, parsed)
	}
	return eventTypes, true
}

func (h *SocketHub) handleDynamicSubscribe(c *client, req WSRequest) {
	var params SubscribeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		c.send(WSResponse{ID: req.ID, Error: errInvalidParams + err.Error()})
		return
	}

	eventTypes, ok := parseStrictEventTypes(c, req.ID, params.Types)
	if !ok {
		return
	}

	if err := h.HandleClientInsertion(eventTypes, params.Addresses, c); err != nil {
		// Answer with the state of the client's own connection rather than the node's
		// shutdown state. Defensive only: reaching here means the client completed its
		// initial insertion, so it is in h.clients and deleteAll always closes it before
		// closed can be observed — c.send is then a no-op and no peer sees either error.
		if errors.Is(err, ErrHubClosed) {
			err = ErrClientClosed
		}
		c.send(WSResponse{ID: req.ID, Error: err.Error()})
		return
	}
	c.send(WSResponse{ID: req.ID, Data: "subscribed"})
}

func (h *SocketHub) handleDynamicUnsubscribe(c *client, req WSRequest) {
	var params UnsubscribeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		c.send(WSResponse{ID: req.ID, Error: errInvalidParams + err.Error()})
		return
	}

	eventTypes, ok := parseStrictEventTypes(c, req.ID, params.Types)
	if !ok {
		return
	}

	h.HandleClientRemoval(eventTypes, params.Addresses, c)
	c.send(WSResponse{ID: req.ID, Data: "unsubscribed"})
}

func (h *SocketHub) HandleClientRemoval(eventTypes []indexer.EventType, addresses []string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var removeAccounts bool
	var removeTransactions bool
	for _, t := range eventTypes {
		switch t {
		case indexer.BLOCKS:
			delete(h.blockSubscription, c)
		case indexer.TRANSACTIONS:
			delete(h.transactionSubscription, c)
		case indexer.ACCOUNTS:
			removeAccounts = true
		case indexer.USER_TRANSACTIONS:
			removeTransactions = true
		}
	}

	for _, addr := range addresses {
		h.removeClientFromAddress(addr, c, removeAccounts, removeTransactions)
	}
}

func (h *SocketHub) removeClientFromAddress(addr string, c *client, removeAccounts, removeTransactions bool) {
	clients, ok := h.addressSubscription[addr]
	if !ok {
		return
	}
	existing, ok := clients[c]
	if !ok {
		return
	}
	if removeAccounts {
		existing.acceptAccount = false
	}
	if removeTransactions {
		existing.acceptTransaction = false
	}
	if !existing.acceptAccount && !existing.acceptTransaction {
		delete(clients, c)
		h.decrClientAddresses(c)
	} else {
		clients[c] = existing
	}
	if len(clients) == 0 {
		delete(h.addressSubscription, addr)
	}
}

// decrClientAddresses lowers a client's tracked address count, removing the entry
// at zero so the map cannot accumulate stale clients.
func (h *SocketHub) decrClientAddresses(c *client) {
	if h.clientAddresses[c] <= 1 {
		delete(h.clientAddresses, c)
		return
	}
	h.clientAddresses[c]--
}
