package websocket

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/klever-io/klever-go/data/api"
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

func drainResponse(c *client) WSResponse {
	msg := <-c.out
	resp, ok := msg.(WSResponse)
	if !ok {
		return WSResponse{}
	}
	return resp
}

func newTestClient(hub *SocketHub) *client {
	return &client{
		hub:   hub,
		out:   make(chan interface{}, 10),
		alive: true,
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
	req := WSRequest{ID: "req-1", Method: MethodGetTransaction, Params: params}

	hub.HandleClientRequest(c, req)
	resp := drainResponse(c)

	assert.Equal(t, "req-1", resp.ID)
	assert.Empty(t, resp.Error)
	assert.Equal(t, expectedTx, resp.Data)
}

func TestHandleGetTransaction_NilFacade(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	params, _ := json.Marshal(GetTransactionParams{Hash: "abc123"})
	req := WSRequest{ID: "req-2", Method: MethodGetTransaction, Params: params}

	hub.HandleClientRequest(c, req)
	resp := drainResponse(c)

	assert.Equal(t, "req-2", resp.ID)
	assert.Contains(t, resp.Error, "facade unavailable")
}

func TestHandleGetTransaction_EmptyHash(t *testing.T) {
	hub := newTestHub(&mockFacade{})
	c := newTestClient(hub)

	params, _ := json.Marshal(GetTransactionParams{})
	req := WSRequest{ID: "req-3", Method: MethodGetTransaction, Params: params}

	hub.HandleClientRequest(c, req)
	resp := drainResponse(c)

	assert.Equal(t, "req-3", resp.ID)
	assert.Contains(t, resp.Error, "missing required param: hash")
}

func TestHandleGetTransaction_FacadeError(t *testing.T) {
	facade := &mockFacade{
		getTransactionFn: func(hash string, withResults bool) (*api.Transaction, error) {
			return nil, errors.New("not found")
		},
	}

	hub := newTestHub(facade)
	c := newTestClient(hub)

	params, _ := json.Marshal(GetTransactionParams{Hash: "abc123"})
	req := WSRequest{ID: "req-4", Method: MethodGetTransaction, Params: params}

	hub.HandleClientRequest(c, req)
	resp := drainResponse(c)

	assert.Equal(t, "req-4", resp.ID)
	assert.Equal(t, "not found", resp.Error)
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
	req := WSRequest{ID: "req-5", Method: MethodGetBlock, Params: params}

	hub.HandleClientRequest(c, req)
	resp := drainResponse(c)

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
	req := WSRequest{ID: "req-6", Method: MethodGetBlock, Params: params}

	hub.HandleClientRequest(c, req)
	resp := drainResponse(c)

	assert.Equal(t, "req-6", resp.ID)
	assert.Empty(t, resp.Error)
	assert.Equal(t, expectedBlock, resp.Data)
}

func TestHandleGetBlock_NoParams(t *testing.T) {
	hub := newTestHub(&mockFacade{})
	c := newTestClient(hub)

	params, _ := json.Marshal(GetBlockParams{})
	req := WSRequest{ID: "req-7", Method: MethodGetBlock, Params: params}

	hub.HandleClientRequest(c, req)
	resp := drainResponse(c)

	assert.Equal(t, "req-7", resp.ID)
	assert.Contains(t, resp.Error, "must provide nonce or hash")
}

func TestHandleGetBlock_NilFacade(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	nonce := uint64(1)
	params, _ := json.Marshal(GetBlockParams{Nonce: &nonce})
	req := WSRequest{ID: "req-8", Method: MethodGetBlock, Params: params}

	hub.HandleClientRequest(c, req)
	resp := drainResponse(c)

	assert.Equal(t, "req-8", resp.ID)
	assert.Contains(t, resp.Error, "facade unavailable")
}

func TestHandleDynamicSubscribe_Success(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	params, _ := json.Marshal(SubscribeParams{
		Types:     []string{"blocks", "transactions"},
		Addresses: []string{"klv1abc"},
	})
	req := WSRequest{ID: "req-9", Method: MethodSubscribe, Params: params}

	hub.HandleClientRequest(c, req)
	resp := drainResponse(c)

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

	params, _ := json.Marshal(SubscribeParams{
		Types: []string{"invalid_type"},
	})
	req := WSRequest{ID: "req-10", Method: MethodSubscribe, Params: params}

	hub.HandleClientRequest(c, req)
	resp := drainResponse(c)

	assert.Equal(t, "req-10", resp.ID)
	assert.Contains(t, resp.Error, "invalid subscription type")
}

func TestHandleDynamicUnsubscribe_Success(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	hub.mu.Lock()
	hub.blockSubscription[c] = struct{}{}
	hub.mu.Unlock()

	params, _ := json.Marshal(UnsubscribeParams{
		Types: []string{"blocks"},
	})
	req := WSRequest{ID: "req-11", Method: MethodUnsubscribe, Params: params}

	hub.HandleClientRequest(c, req)
	resp := drainResponse(c)

	assert.Equal(t, "req-11", resp.ID)
	assert.Empty(t, resp.Error)
	assert.Equal(t, "unsubscribed", resp.Data)

	hub.mu.Lock()
	_, ok := hub.blockSubscription[c]
	hub.mu.Unlock()
	require.False(t, ok)
}

func TestHandleClientRequest_UnknownMethod(t *testing.T) {
	hub := newTestHub(nil)
	c := newTestClient(hub)

	req := WSRequest{ID: "req-12", Method: "nonexistent", Params: json.RawMessage(`{}`)}

	hub.HandleClientRequest(c, req)
	resp := drainResponse(c)

	assert.Equal(t, "req-12", resp.ID)
	assert.Contains(t, resp.Error, "unknown method")
}
