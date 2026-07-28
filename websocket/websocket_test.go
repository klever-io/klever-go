package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/api"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFacade struct {
	getTransactionFn  func(hash string, withResults bool) (*api.Transaction, error)
	getBlockByHashFn  func(hash string, withTxs bool) (*api.Block, error)
	getBlockByNonceFn func(nonce uint64, withTxs bool) (*api.Block, error)
}

func (m *mockFacade) GetTransaction(hash string, withResults bool) (*api.Transaction, error) {
	if m.getTransactionFn != nil {
		return m.getTransactionFn(hash, withResults)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFacade) GetBlockByHash(hash string, withTxs bool) (*api.Block, error) {
	if m.getBlockByHashFn != nil {
		return m.getBlockByHashFn(hash, withTxs)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFacade) GetBlockByNonce(nonce uint64, withTxs bool) (*api.Block, error) {
	if m.getBlockByNonceFn != nil {
		return m.getBlockByNonceFn(nonce, withTxs)
	}
	return nil, errors.New("not implemented")
}

func newTestHub(facade WSFacade) *SocketHub {
	return NewHub("", "", facade)
}

// assertReturnsQuickly runs fn on its own goroutine and fails the test if fn hasn't
// returned within timeout, instead of letting a regression that blocks fn hang the whole
// test (or the suite) rather than failing it.
func assertReturnsQuickly(t *testing.T, timeout time.Duration, failMsg string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal(failMsg)
	}
}

func newTestClient(hub *SocketHub) *client {
	return &client{
		hub:   hub,
		out:   make(chan interface{}, 10),
		alive: true,
		sem:   make(chan struct{}, maxWorkers),
	}
}

func sendRequest(hub *SocketHub, c *client, req WSRequest) WSResponse {
	hub.HandleClientRequest(c, req)

	select {
	case msg := <-c.out:
		resp, ok := msg.(WSResponse)
		if !ok {
			return WSResponse{}
		}
		return resp
	case <-time.After(2 * time.Second):
		panic("timeout waiting for response in sendRequest")
	}
}

func killClient(c *client) {
	c.aliveLock.Lock()
	c.alive = false
	c.aliveLock.Unlock()
}

type serverEnv struct {
	hub       *SocketHub
	queue     chan indexer.Event
	cancel    context.CancelFunc
	done      <-chan struct{}
	restoreFn func()
}

func startServerEnv(t *testing.T, facade WSFacade) *serverEnv {
	t.Helper()
	hub := newTestHub(facade)
	origQueue := indexer.EventQueue
	testQueue := make(chan indexer.Event, 10)
	indexer.EventQueue = testQueue
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		hub.StartServer(ctx)
		close(done)
	}()
	env := &serverEnv{
		hub: hub, queue: testQueue, cancel: cancel, done: done,
		restoreFn: func() { indexer.EventQueue = origQueue },
	}
	t.Cleanup(func() {
		cancel()
		<-done
		indexer.EventQueue = origQueue
	})
	return env
}

func (e *serverEnv) teardown(clients ...*client) {
	for _, c := range clients {
		killClient(c)
	}
	e.cancel()
	<-e.done
	e.restoreFn()
}

func awaitSend(t *testing.T, c *client) *Send {
	t.Helper()
	select {
	case msg := <-c.out:
		s, ok := msg.(*Send)
		require.True(t, ok)
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
		return nil
	}
}

func TestHandleGetTransaction_Success(t *testing.T) {
	expectedTx := &api.Transaction{Hash: "abc123"}
	facade := &mockFacade{
		getTransactionFn: func(hash string, withResults bool) (*api.Transaction, error) {
			assert.Equal(t, "abc123", hash)
			assert.True(t, withResults)
			return expectedTx, nil
		},
	}
	hub := newTestHub(facade)
	c := newTestClient(hub)

	params, _ := json.Marshal(GetTransactionParams{Hash: "abc123", WithResults: true})
	resp := sendRequest(hub, c, WSRequest{ID: "req-1", Method: MethodGetTransaction, Params: params})

	assert.Equal(t, "req-1", resp.ID)
	assert.Empty(t, resp.Error)
	assert.Equal(t, expectedTx, resp.Data)
}

func TestHandleGetTransaction_NilFacade(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	params, _ := json.Marshal(GetTransactionParams{Hash: "abc123"})
	resp := sendRequest(hub, c, WSRequest{ID: "req-2", Method: MethodGetTransaction, Params: params})

	assert.Equal(t, "req-2", resp.ID)
	assert.Contains(t, resp.Error, "facade unavailable")
}

func TestHandleGetTransaction_EmptyHash(t *testing.T) {
	hub := newTestHub(&mockFacade{})
	c := newTestClient(hub)

	params, _ := json.Marshal(GetTransactionParams{})
	resp := sendRequest(hub, c, WSRequest{ID: "req-3", Method: MethodGetTransaction, Params: params})

	assert.Equal(t, "req-3", resp.ID)
	assert.Contains(t, resp.Error, "missing required param: hash")
}

func TestHandleGetTransaction_FacadeError(t *testing.T) {
	facade := &mockFacade{
		getTransactionFn: func(hash string, withResults bool) (*api.Transaction, error) {
			return nil, errors.New("db connection refused")
		},
	}
	hub := newTestHub(facade)
	c := newTestClient(hub)

	params, _ := json.Marshal(GetTransactionParams{Hash: "abc123"})
	resp := sendRequest(hub, c, WSRequest{ID: "req-4", Method: MethodGetTransaction, Params: params})

	assert.Equal(t, "req-4", resp.ID)
	assert.Equal(t, errTxNotFound, resp.Error)
}

func TestHandleGetTransaction_InvalidJSON(t *testing.T) {
	hub := newTestHub(&mockFacade{})
	c := newTestClient(hub)
	resp := sendRequest(hub, c, WSRequest{ID: "bad", Method: MethodGetTransaction, Params: json.RawMessage(`invalid`)})
	assert.Equal(t, "bad", resp.ID)
	assert.Contains(t, resp.Error, "invalid params")
}

func TestHandleGetBlock_ByNonce(t *testing.T) {
	expectedBlock := &api.Block{Hash: "block123"}
	nonce := uint64(42)
	facade := &mockFacade{
		getBlockByNonceFn: func(n uint64, withTxs bool) (*api.Block, error) {
			assert.Equal(t, uint64(42), n)
			assert.True(t, withTxs)
			return expectedBlock, nil
		},
	}
	hub := newTestHub(facade)
	c := newTestClient(hub)

	params, _ := json.Marshal(GetBlockParams{Nonce: &nonce, WithTxs: true})
	resp := sendRequest(hub, c, WSRequest{ID: "req-5", Method: MethodGetBlock, Params: params})

	assert.Equal(t, "req-5", resp.ID)
	assert.Empty(t, resp.Error)
	assert.Equal(t, expectedBlock, resp.Data)
}

func TestHandleGetBlock_ByHash(t *testing.T) {
	expectedBlock := &api.Block{Hash: "block456"}
	facade := &mockFacade{
		getBlockByHashFn: func(hash string, withTxs bool) (*api.Block, error) {
			assert.Equal(t, "block456", hash)
			return expectedBlock, nil
		},
	}
	hub := newTestHub(facade)
	c := newTestClient(hub)

	params, _ := json.Marshal(GetBlockParams{Hash: "block456"})
	resp := sendRequest(hub, c, WSRequest{ID: "req-6", Method: MethodGetBlock, Params: params})

	assert.Equal(t, "req-6", resp.ID)
	assert.Empty(t, resp.Error)
	assert.Equal(t, expectedBlock, resp.Data)
}

func TestHandleGetBlock_NoParams(t *testing.T) {
	hub := newTestHub(&mockFacade{})
	c := newTestClient(hub)

	params, _ := json.Marshal(GetBlockParams{})
	resp := sendRequest(hub, c, WSRequest{ID: "req-7", Method: MethodGetBlock, Params: params})

	assert.Equal(t, "req-7", resp.ID)
	assert.Contains(t, resp.Error, "must provide nonce or hash")
}

func TestHandleGetBlock_NilFacade(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	nonce := uint64(1)
	params, _ := json.Marshal(GetBlockParams{Nonce: &nonce})
	resp := sendRequest(hub, c, WSRequest{ID: "req-8", Method: MethodGetBlock, Params: params})

	assert.Equal(t, "req-8", resp.ID)
	assert.Contains(t, resp.Error, "facade unavailable")
}

func TestHandleGetBlock_InvalidJSON(t *testing.T) {
	hub := newTestHub(&mockFacade{})
	c := newTestClient(hub)
	resp := sendRequest(hub, c, WSRequest{ID: "bad2", Method: MethodGetBlock, Params: json.RawMessage(`not json`)})
	assert.Equal(t, "bad2", resp.ID)
	assert.Contains(t, resp.Error, "invalid params")
}

func TestHandleGetBlock_ByNonce_FacadeError(t *testing.T) {
	nonce := uint64(99)
	facade := &mockFacade{
		getBlockByNonceFn: func(n uint64, withTxs bool) (*api.Block, error) {
			return nil, errors.New("storage error")
		},
	}
	hub := newTestHub(facade)
	c := newTestClient(hub)

	params, _ := json.Marshal(GetBlockParams{Nonce: &nonce})
	resp := sendRequest(hub, c, WSRequest{ID: "be", Method: MethodGetBlock, Params: params})

	assert.Equal(t, "be", resp.ID)
	assert.Equal(t, errBlockNotFound, resp.Error)
}

func TestHandleGetBlock_ByHash_FacadeError(t *testing.T) {
	facade := &mockFacade{
		getBlockByHashFn: func(hash string, withTxs bool) (*api.Block, error) {
			return nil, errors.New("storage error")
		},
	}
	hub := newTestHub(facade)
	c := newTestClient(hub)

	params, _ := json.Marshal(GetBlockParams{Hash: "abc"})
	resp := sendRequest(hub, c, WSRequest{ID: "bhe", Method: MethodGetBlock, Params: params})

	assert.Equal(t, "bhe", resp.ID)
	assert.Equal(t, errBlockNotFound, resp.Error)
}

func TestHandleDynamicSubscribe_Success(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	params, _ := json.Marshal(SubscribeParams{Types: []string{"blocks", "transactions"}, Addresses: []string{"klv1abc"}})
	resp := sendRequest(hub, c, WSRequest{ID: "req-9", Method: MethodSubscribe, Params: params})

	assert.Equal(t, "req-9", resp.ID)
	assert.Empty(t, resp.Error)
	assert.Equal(t, "subscribed", resp.Data)

	hub.mu.Lock()
	_, ok := hub.blockSubscription[c]
	hub.mu.Unlock()
	require.True(t, ok)
}

func TestHandleDynamicSubscribe_InvalidType(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	params, _ := json.Marshal(SubscribeParams{Types: []string{"invalid_type"}})
	resp := sendRequest(hub, c, WSRequest{ID: "req-10", Method: MethodSubscribe, Params: params})

	assert.Equal(t, "req-10", resp.ID)
	assert.Contains(t, resp.Error, "invalid subscription type")
}

func TestHandleDynamicSubscribe_InvalidJSON(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)
	resp := sendRequest(hub, c, WSRequest{ID: "sj", Method: MethodSubscribe, Params: json.RawMessage(`bad`)})
	assert.Contains(t, resp.Error, "invalid params")
}

func TestHandleDynamicUnsubscribe_Success(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	hub.mu.Lock()
	hub.blockSubscription[c] = struct{}{}
	hub.mu.Unlock()

	params, _ := json.Marshal(UnsubscribeParams{Types: []string{"blocks"}})
	resp := sendRequest(hub, c, WSRequest{ID: "req-11", Method: MethodUnsubscribe, Params: params})

	assert.Equal(t, "req-11", resp.ID)
	assert.Empty(t, resp.Error)
	assert.Equal(t, "unsubscribed", resp.Data)

	hub.mu.Lock()
	_, ok := hub.blockSubscription[c]
	hub.mu.Unlock()
	require.False(t, ok)
}

func TestHandleDynamicUnsubscribe_InvalidJSON(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)
	resp := sendRequest(hub, c, WSRequest{ID: "uj", Method: MethodUnsubscribe, Params: json.RawMessage(`bad`)})
	assert.Contains(t, resp.Error, "invalid params")
}

func TestHandleDynamicUnsubscribe_InvalidType(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	params, _ := json.Marshal(UnsubscribeParams{Types: []string{"bogus"}})
	resp := sendRequest(hub, c, WSRequest{ID: "ut", Method: MethodUnsubscribe, Params: params})

	assert.Contains(t, resp.Error, "invalid subscription type")
}

func TestHandleClientRequest_UnknownMethod(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)
	resp := sendRequest(hub, c, WSRequest{ID: "req-12", Method: "nonexistent", Params: json.RawMessage(`{}`)})
	assert.Equal(t, "req-12", resp.ID)
	assert.Contains(t, resp.Error, "unknown method")
}

func TestDynamicSubscribe_MergesFlags(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	params1, _ := json.Marshal(SubscribeParams{Types: []string{"accounts"}, Addresses: []string{"klv1abc"}})
	sendRequest(hub, c, WSRequest{ID: "s1", Method: MethodSubscribe, Params: params1})

	params2, _ := json.Marshal(SubscribeParams{Types: []string{"user_transactions"}, Addresses: []string{"klv1abc"}})
	sendRequest(hub, c, WSRequest{ID: "s2", Method: MethodSubscribe, Params: params2})

	hub.mu.Lock()
	opts := hub.addressSubscription["klv1abc"][c]
	hub.mu.Unlock()

	assert.True(t, opts.acceptAccount)
	assert.True(t, opts.acceptTransaction)
}

func TestDynamicUnsubscribe_PartialFlags(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	hub.mu.Lock()
	hub.addressSubscription["klv1abc"] = map[*client]userOptions{
		c: {acceptAccount: true, acceptTransaction: true},
	}
	hub.mu.Unlock()

	params, _ := json.Marshal(UnsubscribeParams{Types: []string{"accounts"}, Addresses: []string{"klv1abc"}})
	sendRequest(hub, c, WSRequest{ID: "u1", Method: MethodUnsubscribe, Params: params})

	hub.mu.Lock()
	opts, ok := hub.addressSubscription["klv1abc"][c]
	hub.mu.Unlock()

	require.True(t, ok)
	assert.False(t, opts.acceptAccount)
	assert.True(t, opts.acceptTransaction)
}

func TestSend_DoesNotPanicOnClosedClient(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)
	killClient(c)

	assert.NotPanics(t, func() {
		c.send(WSResponse{ID: "x", Error: "test"})
	})

	select {
	case <-c.out:
		t.Fatal("expected no message on closed client")
	default:
	}
}

func TestSend_BufferFull(t *testing.T) {
	hub := newTestHub(nil)
	c := &client{hub: hub, out: make(chan interface{}, 1), alive: true}

	c.send("first")
	c.send("second")

	msg := <-c.out
	assert.Equal(t, "first", msg)

	select {
	case <-c.out:
		t.Fatal("expected no second message in buffer")
	default:
	}
}

func TestNewHub(t *testing.T) {
	hub := NewHub("http://example.com", "api-key", nil)
	assert.NotNil(t, hub)
	assert.Equal(t, "http://example.com", hub.postConnectionURL)
	assert.Equal(t, "api-key", hub.postConnectionAPIKey)
	assert.Nil(t, hub.facade)
	assert.NotNil(t, hub.blockSubscription)
	assert.NotNil(t, hub.transactionSubscription)
	assert.NotNil(t, hub.addressSubscription)
}

func TestNewHub_WithFacade(t *testing.T) {
	f := &mockFacade{}
	hub := NewHub("", "", f)
	assert.Equal(t, f, hub.facade)
}

func TestMarshalMessage_Blocks(t *testing.T) {
	blockData := []byte(`{"nonce":1}`)
	msg, err := marshalMessage("blocks", "", "hash1", blockData)
	require.NoError(t, err)
	assert.Equal(t, indexer.BLOCKS, msg.Type)
	assert.Equal(t, json.RawMessage(blockData), msg.Data)
	assert.Equal(t, "hash1", msg.Hash)
}

func TestMarshalMessage_BlocksInvalidType(t *testing.T) {
	msg, err := marshalMessage("blocks", "", "", "not bytes")
	assert.Error(t, err)
	assert.Nil(t, msg)
}

func TestMarshalMessage_NonBlocks(t *testing.T) {
	payload := map[string]string{"key": "value"}
	msg, err := marshalMessage("transactions", "addr1", "hash1", payload)
	require.NoError(t, err)
	assert.Equal(t, indexer.EventType("transactions"), msg.Type)
	assert.Equal(t, "addr1", msg.Address)
	assert.Equal(t, "hash1", msg.Hash)
	assert.NotEmpty(t, msg.Data)
}

func TestHandleClientInsertion_BlocksAndTransactions(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	hub.HandleClientInsertion([]indexer.EventType{indexer.BLOCKS, indexer.TRANSACTIONS}, nil, c)

	hub.mu.Lock()
	_, hasBlock := hub.blockSubscription[c]
	_, hasTx := hub.transactionSubscription[c]
	hub.mu.Unlock()

	assert.True(t, hasBlock)
	assert.True(t, hasTx)
}

func TestHandleClientInsertion_AddressTypes(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	hub.HandleClientInsertion([]indexer.EventType{indexer.ACCOUNTS, indexer.USER_TRANSACTIONS}, []string{"klv1test"}, c)

	hub.mu.Lock()
	opts := hub.addressSubscription["klv1test"][c]
	hub.mu.Unlock()

	assert.True(t, opts.acceptAccount)
	assert.True(t, opts.acceptTransaction)
}

func TestHandleClientRemoval_BlocksAndTransactions(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	hub.mu.Lock()
	hub.blockSubscription[c] = struct{}{}
	hub.transactionSubscription[c] = struct{}{}
	hub.mu.Unlock()

	hub.HandleClientRemoval([]indexer.EventType{indexer.BLOCKS, indexer.TRANSACTIONS}, nil, c)

	hub.mu.Lock()
	_, hasBlock := hub.blockSubscription[c]
	_, hasTx := hub.transactionSubscription[c]
	hub.mu.Unlock()

	assert.False(t, hasBlock)
	assert.False(t, hasTx)
}

func TestHandleClientRemoval_FullUnsubscribeRemovesEntry(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	hub.mu.Lock()
	hub.addressSubscription["klv1x"] = map[*client]userOptions{c: {acceptAccount: true, acceptTransaction: false}}
	hub.mu.Unlock()

	hub.HandleClientRemoval([]indexer.EventType{indexer.ACCOUNTS}, []string{"klv1x"}, c)

	hub.mu.Lock()
	_, exists := hub.addressSubscription["klv1x"]
	hub.mu.Unlock()

	assert.False(t, exists)
}

func TestHandleClientRemoval_UnknownAddressNoOp(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	assert.NotPanics(t, func() {
		hub.HandleClientRemoval([]indexer.EventType{indexer.ACCOUNTS}, []string{"klv1nonexistent"}, c)
	})
}

func TestHandleClientRemoval_ClientNotInAddressMap(t *testing.T) {
	hub := newTestHub(nil)
	c1 := newTestClient(hub)
	c2 := newTestClient(hub)

	hub.mu.Lock()
	hub.addressSubscription["klv1a"] = map[*client]userOptions{c1: {acceptAccount: true}}
	hub.mu.Unlock()

	hub.HandleClientRemoval([]indexer.EventType{indexer.ACCOUNTS}, []string{"klv1a"}, c2)

	hub.mu.Lock()
	opts, ok := hub.addressSubscription["klv1a"][c1]
	hub.mu.Unlock()
	assert.True(t, ok)
	assert.True(t, opts.acceptAccount)
}

func TestHandleClientDelete(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)
	killClient(c)

	hub.mu.Lock()
	hub.blockSubscription[c] = struct{}{}
	hub.transactionSubscription[c] = struct{}{}
	hub.addressSubscription["klv1a"] = map[*client]userOptions{c: {acceptAccount: true}}
	hub.mu.Unlock()

	hub.handleClientDelete(c)

	hub.mu.Lock()
	_, hasBlock := hub.blockSubscription[c]
	_, hasTx := hub.transactionSubscription[c]
	_, hasAddr := hub.addressSubscription["klv1a"][c]
	hub.mu.Unlock()

	assert.False(t, hasBlock)
	assert.False(t, hasTx)
	assert.False(t, hasAddr)
}

func TestRemoveClient(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	go func() {
		removed := <-hub.unregister
		assert.Equal(t, c, removed)
	}()

	hub.RemoveClient(c)
}

func TestIsAlive(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	assert.True(t, c.IsAlive())
	killClient(c)
	assert.False(t, c.IsAlive())
}

func TestDeleteAll(t *testing.T) {
	hub := newTestHub(nil)
	c1 := newTestClient(hub)
	c2 := newTestClient(hub)
	c3 := newTestClient(hub)
	killClient(c1)
	killClient(c2)
	killClient(c3)

	hub.mu.Lock()
	hub.blockSubscription[c1] = struct{}{}
	hub.transactionSubscription[c2] = struct{}{}
	hub.addressSubscription["klv1a"] = map[*client]userOptions{c3: {acceptAccount: true}}
	hub.mu.Unlock()

	hub.deleteAll()

	hub.mu.Lock()
	assert.Empty(t, hub.blockSubscription)
	assert.Empty(t, hub.transactionSubscription)
	_, hasC3 := hub.addressSubscription["klv1a"][c3]
	hub.mu.Unlock()
	assert.False(t, hasC3)
}

func TestPostWSConnection_WithServer(t *testing.T) {
	receivedCh := make(chan []byte, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		receivedCh <- buf[:n]
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	hub := NewHub(ts.URL, "test-key", nil)
	err := hub.postWSConnection(t.Context(), &Send{Type: indexer.BLOCKS, Data: []byte(`{}`)})
	assert.NoError(t, err)

	select {
	case received := <-receivedCh:
		assert.NotEmpty(t, received)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for POST request")
	}
}

func TestAsyncPost_NoopWithoutConfig(t *testing.T) {
	hub := newTestHub(nil)
	assert.Nil(t, hub.postQueue)

	// A select on a nil channel always falls straight to its default case (it never
	// blocks), so the timeout alone wouldn't catch a regression that deleted asyncPost's
	// `postQueue == nil` early return — the select would just keep taking default. Assert
	// the drop-warn counter too: taking that path (instead of the early return) fires it.
	assertReturnsQuickly(t, 2*time.Second, "asyncPost blocked with no mirror configured", func() {
		hub.asyncPost(&Send{Type: indexer.BLOCKS, Data: []byte(`{}`)})
	})
	assert.Zero(t, hub.queueDropWarn.count.Get(),
		"asyncPost must take the postQueue == nil early return, not fall through to the drop-warn path")
}

func TestNewHub_PostQueueAllocatedOnlyWhenConfigured(t *testing.T) {
	assert.Nil(t, NewHub("", "", nil).postQueue, "no URL or API key: mirror disabled")
	assert.NotNil(t, NewHub("http://example.com", "", nil).postQueue, "URL set: mirror enabled")
	// An API key with no URL can never succeed (every request would fail inside the HTTP
	// client with "no Host in request URL"), so the mirror requires a URL — API key alone
	// leaves postQueue nil rather than starting postWorkerCount workers doomed to fail.
	assert.Nil(t, NewHub("", "api-key", nil).postQueue, "API key without URL: mirror stays disabled")
}

func TestSocketHub_SetAppStatusHandler_NilIsNoop(t *testing.T) {
	hub := newTestHub(nil)
	original := hub.appStatusHandler
	require.NotNil(t, original, "NewHub must default to a non-nil handler")

	hub.SetAppStatusHandler(nil)
	assert.Same(t, original, hub.appStatusHandler, "a nil handler must not replace the default")
}

func TestAsyncPost_QueueFull_IncrementsAppStatusMetric(t *testing.T) {
	hub := NewHub("http://example.invalid", "key", nil)

	var incremented []string
	hub.SetAppStatusHandler(&mock.AppStatusHandlerStub{
		IncrementHandler: func(key string) {
			incremented = append(incremented, key)
		},
	})

	for i := 0; i < hub.limits.postQueueSize; i++ {
		hub.asyncPost(&Send{Type: indexer.BLOCKS, Hash: fmt.Sprintf("seed-%d", i)})
	}
	require.Empty(t, incremented, "must not increment before the queue is actually full")

	hub.asyncPost(&Send{Type: indexer.BLOCKS, Hash: "overflow"})
	assert.Equal(t, []string{core.MetricWSMirrorQueueDroppedTotal}, incremented)
}

func TestStartPostWorkers_PostFailure_IncrementsAppStatusMetric(t *testing.T) {
	hub := NewHub("http://example.invalid", "key", nil)

	var mu sync.Mutex
	var incremented []string
	hub.SetAppStatusHandler(&mock.AppStatusHandlerStub{
		IncrementHandler: func(key string) {
			mu.Lock()
			defer mu.Unlock()
			incremented = append(incremented, key)
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		hub.postWorkersWG.Wait()
	}()
	hub.startPostWorkers(ctx)

	// example.invalid never resolves (RFC 6761), so every post to it fails.
	hub.asyncPost(&Send{Type: indexer.BLOCKS, Hash: "will-fail"})

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(incremented) > 0
	}, 2*time.Second, 10*time.Millisecond, "expected a post-failure metric increment")

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, core.MetricWSMirrorPostFailuresTotal, incremented[0])
}

func TestAsyncPost_DeliversToWorker(t *testing.T) {
	type receivedPost struct {
		body []byte
		err  error
	}
	receivedCh := make(chan receivedPost, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		receivedCh <- receivedPost{body: body, err: err}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	hub := NewHub(ts.URL, "test-key", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		hub.postWorkersWG.Wait()
	}()
	hub.startPostWorkers(ctx)

	hub.asyncPost(&Send{Type: indexer.BLOCKS, Data: []byte(`{"nonce":1}`)})

	select {
	case received := <-receivedCh:
		require.NoError(t, received.err)
		assert.JSONEq(t,
			`{"type":"blocks","address":"","hash":"","data":{"nonce":1}}`,
			string(received.body),
		)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for mirrored POST")
	}
}

func TestAsyncPost_DropsWhenQueueFull(t *testing.T) {
	hub := NewHub("http://example.invalid", "key", nil)
	// No workers started: the queue only drains via asyncPost's own capacity, so filling
	// it exercises the drop-on-full path deterministically without a live HTTP server.
	// Each seed message is distinct (by Hash) so the assertions below can tell "the new
	// send was dropped" apart from "an existing send was evicted to make room for it" —
	// both would otherwise leave the queue at the same length.
	for i := 0; i < hub.limits.postQueueSize; i++ {
		hub.asyncPost(&Send{Type: indexer.BLOCKS, Hash: fmt.Sprintf("seed-%d", i)})
	}
	require.Len(t, hub.postQueue, hub.limits.postQueueSize)

	assertReturnsQuickly(t, 2*time.Second, "asyncPost blocked on a full queue instead of dropping", func() {
		hub.asyncPost(&Send{Type: indexer.BLOCKS, Hash: "overflow"})
	})

	require.Len(t, hub.postQueue, hub.limits.postQueueSize, "queue-full send must be dropped, not queued")
	for i := 0; i < hub.limits.postQueueSize; i++ {
		msg := <-hub.postQueue
		assert.NotEqual(t, "overflow", msg.Hash,
			"overflow message must be dropped, not enqueued in place of an existing one")
	}
}

func TestStartPostWorkers_StopOnContextCancel(t *testing.T) {
	hub := NewHub("http://example.invalid", "key", nil)
	ctx, cancel := context.WithCancel(context.Background())
	hub.startPostWorkers(ctx)
	cancel()

	// Deterministically prove every worker actually exited, rather than assuming it did
	// after a fixed sleep: postWorkersWG.Wait() only returns once each worker's Done()
	// fires. A stuck worker hangs this test instead of passing silently.
	assertReturnsQuickly(t, 2*time.Second, "post workers did not exit after context cancellation", func() {
		hub.postWorkersWG.Wait()
	})
}

func TestStartPostWorkers_NoopWithoutConfig(t *testing.T) {
	hub := newTestHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub.startPostWorkers(ctx)

	// Prove no workers were actually started (not just "didn't panic"): if
	// startPostWorkers incorrectly launched goroutines despite the nil queue, they'd be
	// registered in postWorkersWG and this Wait() would hang until the timeout below
	// instead of returning immediately.
	assertReturnsQuickly(t, 2*time.Second, "startPostWorkers appears to have started workers despite no mirror config", func() {
		hub.postWorkersWG.Wait()
	})
}

// TestStartPostWorkers_BoundedInFlightAndCtxCancelAbortsRequest covers the actual
// invariant this worker pool exists for — at most postWorkerCount mirror requests running
// concurrently — plus ctx cancellation aborting an in-flight request rather than waiting
// out httpClient's full timeout. Every other test in this file would stay green even if
// startPostWorkers reverted to `go func(){ h.postWSConnection(...) }()` per request: none
// of them ever gets postWorkerCount+1 requests in flight at once, and
// StopOnContextCancel/DeliversToWorker never have a request actually parked in the
// handler when ctx is cancelled. A blocking httptest handler is needed to prove both.
func TestStartPostWorkers_BoundedInFlightAndCtxCancelAbortsRequest(t *testing.T) {
	const totalSends = defaultPostWorkerCount * 4

	var inFlight int64
	var peakInFlight int64
	var peakMu sync.Mutex
	release := make(chan struct{})
	handlerEntered := make(chan struct{}, totalSends)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&inFlight, 1)
		defer atomic.AddInt64(&inFlight, -1)
		peakMu.Lock()
		if n > peakInFlight {
			peakInFlight = n
		}
		peakMu.Unlock()
		handlerEntered <- struct{}{}
		select {
		case <-release:
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			// Client aborted (ctx cancellation below): nothing left to write to.
		}
	}))
	defer ts.Close()

	hub := NewHub(ts.URL, "key", nil)
	ctx, cancel := context.WithCancel(context.Background())
	hub.startPostWorkers(ctx)

	for i := 0; i < totalSends; i++ {
		hub.asyncPost(&Send{Type: indexer.BLOCKS, Hash: fmt.Sprintf("req-%d", i)})
	}

	// Wait until postWorkerCount handlers are simultaneously parked: proves the bound is
	// actually being exercised, since anything beyond postWorkerCount must wait for a free
	// worker instead of firing its request immediately.
	for i := 0; i < defaultPostWorkerCount; i++ {
		select {
		case <-handlerEntered:
		case <-time.After(2 * time.Second):
			t.Fatalf("expected %d concurrent in-flight requests, only saw %d", defaultPostWorkerCount, i)
		}
	}

	// Give an (incorrect) unbounded pool a chance to also enter the handler before reading
	// the peak, so a regression back to one-goroutine-per-send would actually show up here.
	time.Sleep(50 * time.Millisecond)
	peakMu.Lock()
	got := peakInFlight
	peakMu.Unlock()
	assert.LessOrEqual(t, got, int64(defaultPostWorkerCount), "more than postWorkerCount mirror requests were in flight at once")

	// Cancel while postWorkerCount requests are parked in the handler (blocked on release,
	// which nothing closes) — proves ctx cancellation aborts the in-flight request instead
	// of running out httpClient's 10s timeout.
	start := time.Now()
	cancel()
	assertReturnsQuickly(t, 5*time.Second, "post workers did not exit promptly on context cancellation with requests in flight", func() {
		hub.postWorkersWG.Wait()
	})
	assert.Less(t, time.Since(start), 9*time.Second, "workers should exit well before httpClient's 10s timeout")
	close(release)
}

func TestStartServer_ContextCancel(t *testing.T) {
	env := startServerEnv(t, nil)
	c := newTestClient(env.hub)
	killClient(c)

	env.hub.mu.Lock()
	env.hub.blockSubscription[c] = struct{}{}
	env.hub.mu.Unlock()

	env.teardown()

	env.hub.mu.Lock()
	_, has := env.hub.blockSubscription[c]
	env.hub.mu.Unlock()
	assert.False(t, has)
}

func TestStartServer_BlocksEvent(t *testing.T) {
	env := startServerEnv(t, nil)
	c := newTestClient(env.hub)

	env.hub.mu.Lock()
	env.hub.blockSubscription[c] = struct{}{}
	env.hub.mu.Unlock()

	env.queue <- indexer.Event{EvType: indexer.BLOCKS, Message: []byte(`{"nonce":1}`)}

	s := awaitSend(t, c)
	assert.Equal(t, indexer.BLOCKS, s.Type)

	env.teardown(c)
}

func TestStartServer_TransactionEvent(t *testing.T) {
	env := startServerEnv(t, nil)
	c := newTestClient(env.hub)

	env.hub.mu.Lock()
	env.hub.transactionSubscription[c] = struct{}{}
	env.hub.mu.Unlock()

	env.queue <- indexer.Event{EvType: indexer.TRANSACTIONS, Message: map[string]string{"hash": "abc"}}

	s := awaitSend(t, c)
	assert.Equal(t, indexer.TRANSACTIONS, s.Type)

	env.teardown(c)
}

func TestStartServer_AccountsEvent(t *testing.T) {
	env := startServerEnv(t, nil)
	c := newTestClient(env.hub)

	env.hub.mu.Lock()
	env.hub.addressSubscription["klv1test"] = map[*client]userOptions{c: {acceptAccount: true}}
	env.hub.mu.Unlock()

	env.queue <- indexer.Event{
		EvType:  indexer.ACCOUNTS,
		Message: map[string]*data.AccountInfo{"klv1test": {Address: "klv1test", Balance: 100}},
	}

	s := awaitSend(t, c)
	assert.Equal(t, indexer.ACCOUNTS, s.Type)
	assert.Equal(t, "klv1test", s.Address)

	env.teardown(c)
}

func TestStartServer_AccountsEvent_NoSubscribers(t *testing.T) {
	env := startServerEnv(t, nil)

	env.queue <- indexer.Event{
		EvType:  indexer.ACCOUNTS,
		Message: map[string]*data.AccountInfo{"klv1nobody": {Address: "klv1nobody"}},
	}

	time.Sleep(100 * time.Millisecond)
	env.teardown()
}

func TestStartServer_UserTransactionEvent(t *testing.T) {
	env := startServerEnv(t, nil)
	c := newTestClient(env.hub)

	env.hub.mu.Lock()
	env.hub.addressSubscription["klv1sender"] = map[*client]userOptions{c: {acceptTransaction: true}}
	env.hub.mu.Unlock()

	env.queue <- indexer.Event{
		EvType: indexer.USER_TRANSACTIONS,
		Message: []*data.Transaction{{
			Hash: "tx1", Sender: "klv1sender",
			Receipts: []map[string]interface{}{{"to": "klv1receiver"}},
		}},
	}

	s := awaitSend(t, c)
	assert.Equal(t, indexer.USER_TRANSACTIONS, s.Type)

	env.teardown(c)
}

func TestStartServer_UserTransaction_NoReceiptTo(t *testing.T) {
	env := startServerEnv(t, nil)
	c := newTestClient(env.hub)

	env.hub.mu.Lock()
	env.hub.addressSubscription["klv1sender"] = map[*client]userOptions{c: {acceptTransaction: true}}
	env.hub.mu.Unlock()

	env.queue <- indexer.Event{
		EvType: indexer.USER_TRANSACTIONS,
		Message: []*data.Transaction{{
			Hash: "tx-no-to", Sender: "klv1sender",
			Receipts: []map[string]interface{}{{"amount": "100"}},
		}},
	}

	s := awaitSend(t, c)
	assert.Equal(t, indexer.USER_TRANSACTIONS, s.Type)

	env.teardown(c)
}

func TestStartServer_UnregisterClient(t *testing.T) {
	env := startServerEnv(t, nil)
	c := newTestClient(env.hub)
	killClient(c)

	env.hub.mu.Lock()
	env.hub.blockSubscription[c] = struct{}{}
	env.hub.mu.Unlock()

	env.hub.unregister <- c
	time.Sleep(100 * time.Millisecond)

	env.hub.mu.Lock()
	_, has := env.hub.blockSubscription[c]
	env.hub.mu.Unlock()
	assert.False(t, has)

	env.teardown()
}

func setupTestWSServer(t *testing.T, hub *SocketHub) (*ws.Conn, func()) {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := ws.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		NewClient(conn, hub)
	}))

	url := "ws" + strings.TrimPrefix(s.URL, "http")
	conn, _, err := ws.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	return conn, func() {
		conn.Close()
		s.Close()
		time.Sleep(50 * time.Millisecond)
	}
}

func TestNewClient_LoopIn_TextMessage(t *testing.T) {
	facade := &mockFacade{
		getTransactionFn: func(hash string, withResults bool) (*api.Transaction, error) {
			return &api.Transaction{Hash: hash}, nil
		},
	}
	hub := newTestHub(facade)
	conn, cleanup := setupTestWSServer(t, hub)
	defer cleanup()

	err := conn.WriteJSON(WSRequest{ID: "live-1", Method: MethodGetTransaction, Params: json.RawMessage(`{"hash":"live_hash"}`)})
	require.NoError(t, err)

	var resp WSResponse
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	err = conn.ReadJSON(&resp)
	require.NoError(t, err)

	assert.Equal(t, "live-1", resp.ID)
	assert.Empty(t, resp.Error)
}

func TestNewClient_LoopIn_InvalidJSON(t *testing.T) {
	hub := newTestHub(nil)
	conn, cleanup := setupTestWSServer(t, hub)
	defer cleanup()

	err := conn.WriteMessage(ws.TextMessage, []byte(`not valid json`))
	require.NoError(t, err)

	var resp WSResponse
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	err = conn.ReadJSON(&resp)
	require.NoError(t, err)

	assert.Contains(t, resp.Error, "invalid json")
}

func TestNewClient_LoopIn_ConnectionClose(t *testing.T) {
	hub := newTestHub(nil)
	conn, cleanup := setupTestWSServer(t, hub)

	err := conn.WriteMessage(ws.CloseMessage, ws.FormatCloseMessage(ws.CloseNormalClosure, "bye"))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	cleanup()
}
