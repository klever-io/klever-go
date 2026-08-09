package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

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

	postQueueDropLogIntervalSeconds = 10
)

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
	acceptLogs        bool
}

type SocketHub struct {
	mu                   sync.RWMutex
	postConnectionURL    string
	postConnectionAPIKey string
	// mirrorConfigured mirrors postQueue != nil (the mirror's actual enable condition — see
	// NewHub, URL is the single source of truth) as a plain bool so per-event dispatch
	// doesn't need to reach through the channel. Computed once at construction; neither
	// underlying field is ever mutated afterward.
	mirrorConfigured        bool
	facade                  WSFacade
	blockSubscription       map[*client]struct{}
	transactionSubscription map[*client]struct{}
	addressSubscription     map[string]map[*client]userOptions
	clientAddresses         map[*client]int
	// logsSubscriberCount is the number of (address, client) entries currently accepting
	// LOGS, maintained incrementally alongside addressSubscription (guarded by the same
	// mu). HasLogsSubscriberOrMirror reads this instead of scanning addressSubscription,
	// because it runs synchronously on the block-commit goroutine for every block with
	// logs — an O(n) scan there would let a client cheaply inflate commit-goroutine cost
	// simply by holding many address subscriptions, without ever accepting LOGS.
	logsSubscriberCount int
	limits              resolvedLimits
	unregister          chan *client
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
		unregister:              make(chan *client),
		addressSubscription:     make(map[string]map[*client]userOptions),
		clientAddresses:         make(map[*client]int),
		blockSubscription:       make(map[*client]struct{}),
		transactionSubscription: make(map[*client]struct{}),
		postConnectionURL:       postConnectionURL,
		postConnectionAPIKey:    postConnectionAPIKey,
		mirrorConfigured:        postQueue != nil,
		facade:                  facade,
		limits:                  resolved,
		postQueue:               postQueue,
		queueDropWarn:           dropWarner{windowSecs: postQueueDropLogIntervalSeconds},
		postFailWarn:            dropWarner{windowSecs: postQueueDropLogIntervalSeconds},
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

// dispatchToAddress marshals message for evType/address/hash and sends it to every client
// currently watching address whose userOptions filterFn accepts, skipping the
// marshal/mirror-post cost entirely when nobody would receive it (no matching subscriber
// and no mirror configured). One lock/lookup snapshots the matching clients; shared by
// every address-scoped event handler (ACCOUNTS, LOGS, tx sender/receipts) so the
// "is anybody listening" gate isn't special-cased to one event type.
func (h *SocketHub) dispatchToAddress(evType indexer.EventType, address, hash string, message interface{}, filterFn func(userOptions) bool) {
	h.mu.RLock()
	clients := h.addressSubscription[address]
	snapshot := make([]*client, 0, len(clients))
	for c, opts := range clients {
		if filterFn(opts) {
			snapshot = append(snapshot, c)
		}
	}
	h.mu.RUnlock()

	if len(snapshot) == 0 && !h.mirrorConfigured {
		return
	}

	parsed := h.marshalAndPost(evType, address, hash, message)
	if parsed == nil {
		return
	}
	for _, c := range snapshot {
		if c.IsAlive() {
			c.send(parsed)
		}
	}
}

// HasLogsSubscriberOrMirror reports whether dispatching a LOGS event would actually be
// delivered anywhere — a mirror endpoint is configured, or at least one client currently
// watches an address with LOGS accepted. Wired into indexer.LogsSubscriberChecker (see
// network/api/api.go) so the block-commit goroutine can skip the log-conversion cost
// entirely for a block that nobody would receive it for.
func (h *SocketHub) HasLogsSubscriberOrMirror() bool {
	if h.mirrorConfigured {
		return true
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.logsSubscriberCount > 0
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
			return
		case client := <-h.unregister:
			h.handleClientDelete(client)
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
			case indexer.LOGS:
				h.handleLogsEvent(event)
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
		h.dispatchToAddress(event.EvType, account, "", info, acceptAccount)
	}
}

func (h *SocketHub) handleLogsEvent(event indexer.Event) {
	logs, ok := event.Message.([]*data.Logs)
	if !ok {
		log.Error("ws.EventReceive", "err", "cannot convert message to []*data.Logs")
		return
	}
	acceptLogs := func(opts userOptions) bool { return opts.acceptLogs }
	for _, entry := range logs {
		if entry == nil || entry.Address == "" {
			continue
		}
		// entry.ID is the hex-encoded hash of the transaction that produced this log
		// (already computed for the Elasticsearch-facing shape; reused here as the
		// envelope hash so a subscriber can correlate a log back to its transaction).
		h.dispatchToAddress(event.EvType, entry.Address, entry.ID, entry, acceptLogs)
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
	h.dispatchToAddress(evType, tx.Sender, "", tx, acceptTx)
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
		h.dispatchToAddress(evType, address, "", tx, acceptTx)
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

// RemoveClient remove client from hub
func (h *SocketHub) RemoveClient(c *client) {
	h.unregister <- c
}

func (h *SocketHub) HandleClientInsertion(eventType []indexer.EventType, addresses []string, c *client) error {
	// Addresses are only meaningful for the address-scoped types (ACCOUNTS,
	// USER_TRANSACTIONS). A blocks/transactions-only subscribe must not count or
	// store them, or it would burn the per-connection address budget on entries
	// that never match anything (GHSA-4fwh-wrm6-97xm, Impact C).
	wantsAddresses := containsAddressScoped(eventType)

	if wantsAddresses && len(addresses) > h.limits.maxAddressesPerSubscribe {
		return fmt.Errorf("too many addresses in a single subscribe: %d (max %d)", len(addresses), h.limits.maxAddressesPerSubscribe)
	}

	// Bound each address by byte size before it is retained as a subscription key. The count
	// caps alone leave a memory-amplification path: a long-lived connection could otherwise
	// keep sending unique oversized strings that can never match a real address yet are
	// retained per connection (GHSA-4fwh-wrm6-97xm).
	if wantsAddresses {
		for _, address := range addresses {
			if len(address) > maxEncodedAddressLength {
				return fmt.Errorf("subscription address exceeds the maximum length of %d bytes", maxEncodedAddressLength)
			}
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Reject before mutating if the per-connection cap would be exceeded; only addresses
	// new to this client count.
	if wantsAddresses && h.clientAddresses[c]+h.countNewAddresses(addresses, c) > h.limits.maxAddressesPerClient {
		return fmt.Errorf("address subscription limit reached for this connection (max %d)", h.limits.maxAddressesPerClient)
	}

	opts := h.applyEventTypes(eventType, c)
	if wantsAddresses {
		h.addAddressSubscriptions(addresses, c, opts)
	}
	return nil
}

// containsAddressScoped reports whether eventType includes a type for which the
// request's addresses are meaningful (ACCOUNTS, USER_TRANSACTIONS or LOGS).
func containsAddressScoped(eventType []indexer.EventType) bool {
	for _, t := range eventType {
		if t == indexer.ACCOUNTS || t == indexer.USER_TRANSACTIONS || t == indexer.LOGS {
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
// the per-address accept flags as a userOptions value. The caller must hold h.mu.
func (h *SocketHub) applyEventTypes(eventType []indexer.EventType, c *client) userOptions {
	var opts userOptions
	for _, t := range eventType {
		switch t {
		case indexer.BLOCKS:
			h.blockSubscription[c] = struct{}{}
		case indexer.TRANSACTIONS:
			h.transactionSubscription[c] = struct{}{}
		case indexer.USER_TRANSACTIONS:
			opts.acceptTransaction = true
		case indexer.ACCOUNTS:
			opts.acceptAccount = true
		case indexer.LOGS:
			opts.acceptLogs = true
		}
	}
	return opts
}

// addAddressSubscriptions adds c to each address, merging opts' set flags into any
// existing subscription instead of replacing it, and bumping the per-client count for
// newly added (address, client) pairs. The caller must hold h.mu.
func (h *SocketHub) addAddressSubscriptions(addresses []string, c *client, opts userOptions) {
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
		if opts.acceptAccount {
			existing.acceptAccount = true
		}
		if opts.acceptTransaction {
			existing.acceptTransaction = true
		}
		if opts.acceptLogs && !existing.acceptLogs {
			h.logsSubscriberCount++
			existing.acceptLogs = true
		}
		value[c] = existing
	}
}

func closeAndClear(subscription map[*client]struct{}) {
	for c := range subscription {
		c.close()
		delete(subscription, c)
	}
}

func (h *SocketHub) deleteAll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for addr, clients := range h.addressSubscription {
		for cl := range clients {
			cl.close()
			delete(clients, cl)
		}
		delete(h.addressSubscription, addr)
	}
	h.clientAddresses = make(map[*client]int)
	h.logsSubscriberCount = 0

	closeAndClear(h.blockSubscription)
	closeAndClear(h.transactionSubscription)
}

func (h *SocketHub) handleClientDelete(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.blockSubscription, c)
	delete(h.transactionSubscription, c)
	c.close()
	// Remove the client from every watched address and reclaim the outer key when its
	// inner map empties. Without this, a disconnect leaks one map entry per address
	// permanently (GHSA-4fwh-wrm6-97xm, Impact C).
	for addr, clients := range h.addressSubscription {
		if opts, has := clients[c]; has && opts.acceptLogs {
			h.logsSubscriberCount--
		}
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
		log.Warn("ws.handleGetTransaction", "hash", params.Hash, "err", err.Error())
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
			log.Warn("ws.handleGetBlock", "nonce", *params.Nonce, "err", err.Error())
			c.send(WSResponse{ID: req.ID, Error: errBlockNotFound})
			return
		}
		c.send(WSResponse{ID: req.ID, Data: blk})
		return
	}

	blk, err := h.facade.GetBlockByHash(params.Hash, params.WithTxs)
	if err != nil {
		log.Warn("ws.handleGetBlock", "hash", params.Hash, "err", err.Error())
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

	var remove userOptions
	for _, t := range eventTypes {
		switch t {
		case indexer.BLOCKS:
			delete(h.blockSubscription, c)
		case indexer.TRANSACTIONS:
			delete(h.transactionSubscription, c)
		case indexer.ACCOUNTS:
			remove.acceptAccount = true
		case indexer.USER_TRANSACTIONS:
			remove.acceptTransaction = true
		case indexer.LOGS:
			remove.acceptLogs = true
		}
	}

	for _, addr := range addresses {
		h.removeClientFromAddress(addr, c, remove)
	}
}

// removeClientFromAddress clears each set flag in remove from c's subscription at addr.
// The caller must hold h.mu.
func (h *SocketHub) removeClientFromAddress(addr string, c *client, remove userOptions) {
	clients, ok := h.addressSubscription[addr]
	if !ok {
		return
	}
	existing, ok := clients[c]
	if !ok {
		return
	}
	if remove.acceptAccount {
		existing.acceptAccount = false
	}
	if remove.acceptTransaction {
		existing.acceptTransaction = false
	}
	if remove.acceptLogs && existing.acceptLogs {
		h.logsSubscriberCount--
		existing.acceptLogs = false
	}
	if !existing.acceptAccount && !existing.acceptTransaction && !existing.acceptLogs {
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
