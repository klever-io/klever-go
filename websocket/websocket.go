package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/klever-io/klever-go/cmd/operator/utils"

	logger "github.com/klever-io/klever-go-logger"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/indexer/data"
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
)

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
	unregister              chan *client
}

func NewHub(postConnectionURL, postConnectionAPIKey string, facade WSFacade, limits ...Limits) *SocketHub {
	var l Limits
	if len(limits) > 0 {
		l = limits[0]
	}

	return &SocketHub{
		unregister:              make(chan *client),
		addressSubscription:     make(map[string]map[*client]userOptions),
		clientAddresses:         make(map[*client]int),
		blockSubscription:       make(map[*client]struct{}),
		transactionSubscription: make(map[*client]struct{}),
		postConnectionURL:       postConnectionURL,
		postConnectionAPIKey:    postConnectionAPIKey,
		facade:                  facade,
		limits:                  l.resolve(),
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

func (h *SocketHub) asyncPost(parsed *Send) {
	go func() {
		if err := h.postWSConnection(parsed); err != nil {
			log.Warn("ws.EventReceive.postWSConnection", "failed to post", err.Error())
		}
	}()
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
	for {
		select {
		case <-ctx.Done():
			log.Info("delete all client and close gracefully")
			h.deleteAll()
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

func (h *SocketHub) postWSConnection(message *Send) error {
	if h.postConnectionURL == "" && h.postConnectionAPIKey == "" {
		return nil
	}

	b, err := json.Marshal(message)
	if err != nil {
		return err
	}

	return utils.PostURL(h.postConnectionURL, string(b), []string{"x-api-key", h.postConnectionAPIKey}, nil)
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
	if len(addresses) > h.limits.maxAddressesPerSubscribe {
		return fmt.Errorf("too many addresses in a single subscribe: %d (max %d)", len(addresses), h.limits.maxAddressesPerSubscribe)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Reject before mutating if the per-connection cap would be exceeded; only addresses
	// new to this client count (GHSA-4fwh-wrm6-97xm, Impact C).
	if h.clientAddresses[c]+h.countNewAddresses(addresses, c) > h.limits.maxAddressesPerClient {
		return fmt.Errorf("address subscription limit reached for this connection (max %d)", h.limits.maxAddressesPerClient)
	}

	acceptAccounts, acceptTransactions := h.applyEventTypes(eventType, c)
	h.addAddressSubscriptions(addresses, c, acceptAccounts, acceptTransactions)
	return nil
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
