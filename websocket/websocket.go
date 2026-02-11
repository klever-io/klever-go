package websocket

import "C"
import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/klever-io/klever-go/cmd/operator/utils"

	logger "github.com/klever-io/klever-go-logger"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/indexer/data"
)

var log = logger.GetOrCreate("websocket")

type userOptions struct {
	acceptAccount     bool
	acceptTransaction bool
}

type SocketHub struct {
	mu                      sync.Mutex
	postConnectionURL       string
	postConnectionAPIKey    string
	facade                  WSFacade
	blockSubscription       map[*client]struct{}
	transactionSubscription map[*client]struct{}
	addressSubscription     map[string]map[*client]userOptions
	unregister              chan *client
}

func NewHub(postConnectionURL, postConnectionAPIKey string, facade WSFacade) *SocketHub {
	return &SocketHub{
		unregister:              make(chan *client),
		addressSubscription:     make(map[string]map[*client]userOptions),
		blockSubscription:       make(map[*client]struct{}),
		transactionSubscription: make(map[*client]struct{}),
		postConnectionURL:       postConnectionURL,
		postConnectionAPIKey:    postConnectionAPIKey,
		facade:                  facade,
	}
}

// StartServer start the hub to receive clients and send messages
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
			switch event.EvType {
			case indexer.ACCOUNTS:
				accounts := event.Message.(map[string]*data.AccountInfo)
				for account, info := range accounts {
					parsed, err := marshalMessage(event.EvType, account, "", info)
					if err != nil {
						log.Error("ws.EventReceive", "cannot marshal message", err.Error())
						continue
					}

					go func() {
						if err := h.postWSConnection(parsed); err != nil {
							log.Warn("ws.EventReceive.postWSConnection", "failed to post", err.Error())
						}
					}()

					clients, ok := h.addressSubscription[account]
					if !ok {
						continue
					}

					for c, opts := range clients {
						if c.IsAlive() && opts.acceptAccount {
							c.out <- parsed
						}
					}
				}
			case indexer.USER_TRANSACTION:
				transactions := event.Message.([]*data.Transaction)
				for _, tx := range transactions {
					var wg sync.WaitGroup
					wg.Add(3)
					go func() {
						defer wg.Done()
						parsed, err := marshalMessage(event.EvType, tx.Sender, "", tx)
						if err != nil {
							log.Error("ws.EventReceive", "cannot marshal message", err.Error())
							return
						}

						go func() {
							if err := h.postWSConnection(parsed); err != nil {
								log.Warn("ws.EventReceive.postWSConnection", "failed to post", err.Error())
							}
						}()

						clients, ok := h.addressSubscription[tx.Sender]
						if !ok {
							return
						}

						for c, opts := range clients {
							if c.IsAlive() && opts.acceptTransaction {
								c.out <- parsed
							}
						}
					}()
					go func(tx *data.Transaction) {
						defer wg.Done()
						for _, receipts := range tx.Receipts {
							to := receipts["to"]
							if to == nil {
								continue
							}

							address := to.(string)

							parsed, err := marshalMessage(event.EvType, address, "", tx)
							if err != nil {
								log.Error("ws.EventReceive", "cannot marshal message", err.Error())
								return
							}

							go func() {
								if err := h.postWSConnection(parsed); err != nil {
									log.Warn("ws.EventReceive.postWSConnection", "failed to post", err.Error())
								}
							}()

							clients, ok := h.addressSubscription[address]
							if !ok {
								continue
							}

							for c, opts := range clients {
								if c.IsAlive() && opts.acceptTransaction {
									c.out <- parsed
								}
							}
						}
					}(tx)
					go func() {
						defer wg.Done()
						parsed, err := marshalMessage(event.EvType, "", tx.Hash, tx)
						if err != nil {
							log.Error("ws.EventReceive", "cannot marshal message", err.Error())
							return
						}

						go func() {
							if err := h.postWSConnection(parsed); err != nil {
								log.Warn("ws.EventReceive.postWSConnection", "failed to post", err.Error())
							}
						}()
					}()

					wg.Wait()
				}
			case indexer.TRANSACTION:
				parsed, err := marshalMessage(event.EvType, "", "", event.Message)
				if err != nil {
					log.Error("ws.EventReceive", "cannot marshal message", err.Error())
					continue
				}

				go func() {
					if err := h.postWSConnection(parsed); err != nil {
						log.Warn("ws.EventReceive.postWSConnection", "failed to post", err.Error())
					}
				}()

				clients := h.transactionSubscription
				for c := range clients {
					if c.IsAlive() {
						c.out <- parsed
					}
				}
			default:
				parsed, err := marshalMessage(event.EvType, "", "", event.Message)
				if err != nil {
					log.Error("ws.EventReceive", "cannot marshal message", err.Error())
					continue
				}

				go func() {
					if err := h.postWSConnection(parsed); err != nil {
						log.Warn("ws.EventReceive.postWSConnection", "failed to post", err.Error())
					}
				}()

				clients := h.blockSubscription
				for c := range clients {
					if c.IsAlive() {
						c.out <- parsed
					}
				}
			}
		}
	}
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
	Data    []byte            `json:"data"`
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

func (h *SocketHub) HandleClientInsertion(eventType []indexer.EventType, addresses []string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var acceptTransactions bool
	var acceptAccounts bool
	for _, types := range eventType {
		switch types {
		case indexer.BLOCKS:
			h.blockSubscription[c] = struct{}{}
		case indexer.TRANSACTION:
			h.transactionSubscription[c] = struct{}{}
		case indexer.USER_TRANSACTION:
			acceptTransactions = true
		case indexer.ACCOUNTS:
			acceptAccounts = true
		}
	}

	for _, address := range addresses {
		if _, ok := h.addressSubscription[address]; !ok {
			h.addressSubscription[address] = make(map[*client]userOptions)
		}

		value := h.addressSubscription[address]
		value[c] = userOptions{
			acceptAccount:     acceptAccounts,
			acceptTransaction: acceptTransactions,
		}

		h.addressSubscription[address] = value
	}
}

func (h *SocketHub) deleteAll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, clients := range h.addressSubscription {
		for cl := range clients {
			cl.close()
			delete(clients, cl)
		}
	}

	for client := range h.blockSubscription {
		if client.alive {
			client.close()
		}

		delete(h.blockSubscription, client)
	}

	for client := range h.transactionSubscription {
		if client.alive {
			client.close()
		}

		delete(h.transactionSubscription, client)
	}
}

func (h *SocketHub) handleClientDelete(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.blockSubscription, c)
	delete(h.transactionSubscription, c)
	c.close()
	for _, clients := range h.addressSubscription {
		for cl := range clients {
			if cl == c {
				delete(clients, c)
			}
		}
	}
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
		c.out <- WSResponse{ID: req.ID, Error: "unknown method: " + req.Method}
	}
}

func (h *SocketHub) handleGetTransaction(c *client, req WSRequest) {
	if h.facade == nil {
		c.out <- WSResponse{ID: req.ID, Error: "query not supported: facade unavailable"}
		return
	}

	var params GetTransactionParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		c.out <- WSResponse{ID: req.ID, Error: "invalid params: " + err.Error()}
		return
	}

	if params.Hash == "" {
		c.out <- WSResponse{ID: req.ID, Error: "missing required param: hash"}
		return
	}

	tx, err := h.facade.GetTransaction(params.Hash, params.WithResults)
	if err != nil {
		c.out <- WSResponse{ID: req.ID, Error: err.Error()}
		return
	}

	c.out <- WSResponse{ID: req.ID, Data: tx}
}

func (h *SocketHub) handleGetBlock(c *client, req WSRequest) {
	if h.facade == nil {
		c.out <- WSResponse{ID: req.ID, Error: "query not supported: facade unavailable"}
		return
	}

	var params GetBlockParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		c.out <- WSResponse{ID: req.ID, Error: "invalid params: " + err.Error()}
		return
	}

	if params.Nonce == nil && params.Hash == "" {
		c.out <- WSResponse{ID: req.ID, Error: "must provide nonce or hash"}
		return
	}

	if params.Nonce != nil {
		blk, err := h.facade.GetBlockByNonce(*params.Nonce, params.WithTxs)
		if err != nil {
			c.out <- WSResponse{ID: req.ID, Error: err.Error()}
			return
		}
		c.out <- WSResponse{ID: req.ID, Data: blk}
		return
	}

	blk, err := h.facade.GetBlockByHash(params.Hash, params.WithTxs)
	if err != nil {
		c.out <- WSResponse{ID: req.ID, Error: err.Error()}
		return
	}
	c.out <- WSResponse{ID: req.ID, Data: blk}
}

func (h *SocketHub) handleDynamicSubscribe(c *client, req WSRequest) {
	var params SubscribeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		c.out <- WSResponse{ID: req.ID, Error: "invalid params: " + err.Error()}
		return
	}

	var eventTypes []indexer.EventType
	for _, t := range params.Types {
		parsed, err := indexer.NewEventTypeStrict(t)
		if err != nil {
			c.out <- WSResponse{ID: req.ID, Error: "invalid subscription type: " + t}
			return
		}
		eventTypes = append(eventTypes, parsed)
	}

	h.HandleClientInsertion(eventTypes, params.Addresses, c)
	c.out <- WSResponse{ID: req.ID, Data: "subscribed"}
}

func (h *SocketHub) handleDynamicUnsubscribe(c *client, req WSRequest) {
	var params UnsubscribeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		c.out <- WSResponse{ID: req.ID, Error: "invalid params: " + err.Error()}
		return
	}

	var eventTypes []indexer.EventType
	for _, t := range params.Types {
		parsed, err := indexer.NewEventTypeStrict(t)
		if err != nil {
			c.out <- WSResponse{ID: req.ID, Error: "invalid subscription type: " + t}
			return
		}
		eventTypes = append(eventTypes, parsed)
	}

	h.HandleClientRemoval(eventTypes, params.Addresses, c)
	c.out <- WSResponse{ID: req.ID, Data: "unsubscribed"}
}

func (h *SocketHub) HandleClientRemoval(eventTypes []indexer.EventType, addresses []string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, t := range eventTypes {
		switch t {
		case indexer.BLOCKS:
			delete(h.blockSubscription, c)
		case indexer.TRANSACTION:
			delete(h.transactionSubscription, c)
		}
	}

	for _, addr := range addresses {
		clients, ok := h.addressSubscription[addr]
		if !ok {
			continue
		}
		delete(clients, c)
		if len(clients) == 0 {
			delete(h.addressSubscription, addr)
		}
	}
}
