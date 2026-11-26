package websocket

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/klever-io/klever-go/data/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func subscribeFrame(t *testing.T, id, method, address string) WSRequest {
	t.Helper()
	params, err := json.Marshal(SubscribeParams{Types: []string{"accounts"}, Addresses: []string{address}})
	require.NoError(t, err)
	return WSRequest{ID: id, Method: method, Params: params}
}

// readResponses reads n replies and indexes them by request ID (handler replies are async).
func readResponses(t *testing.T, conn *ws.Conn, n int) map[string]WSResponse {
	t.Helper()
	got := make(map[string]WSResponse, n)
	for i := 0; i < n; i++ {
		resp := readWSResponse(t, conn)
		got[resp.ID] = resp
	}
	require.Len(t, got, n)
	return got
}

func isSubscribed(hub *SocketHub, c *client, address string) bool {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	_, ok := hub.addressSubscription[address][c]
	return ok
}

// Rate 1/s keeps refill negligible while the burst is sent, so the counts are exact.
func TestLoopIn_SubscribeRateLimit(t *testing.T) {
	const burst = 3
	facade := &mockFacade{
		getTransactionFn:  func(hash string, _ bool) (*api.Transaction, error) { return &api.Transaction{Hash: hash}, nil },
		getBlockByNonceFn: func(uint64, bool) (*api.Block, error) { return &api.Block{}, nil },
	}
	hub := NewHub("", "", facade, Limits{SubscribeRatePerSec: 1, SubscribeBurst: burst})
	conn, c := dialLiveClient(t, hub)
	t.Cleanup(c.Close)

	const frames = 8
	for i := 0; i < frames; i++ {
		require.NoError(t, conn.WriteJSON(subscribeFrame(t, fmt.Sprintf("s%d", i), MethodSubscribe, fmt.Sprintf("a%d", i))))
	}
	// Unsubscribe shares the bucket: refused, so a0 stays subscribed.
	require.NoError(t, conn.WriteJSON(subscribeFrame(t, "u0", MethodUnsubscribe, "a0")))
	// Queries are not rate-limited.
	require.NoError(t, conn.WriteJSON(WSRequest{ID: "tx", Method: MethodGetTransaction, Params: json.RawMessage(`{"hash":"h"}`)}))
	require.NoError(t, conn.WriteJSON(WSRequest{ID: "blk", Method: MethodGetBlock, Params: json.RawMessage(`{"nonce":7}`)}))

	got := readResponses(t, conn, frames+3)
	for i := 0; i < frames; i++ {
		id, addr := fmt.Sprintf("s%d", i), fmt.Sprintf("a%d", i)
		if i < burst {
			assert.Empty(t, got[id].Error, id)
			assert.True(t, isSubscribed(hub, c, addr), addr)
			continue
		}
		assert.Equal(t, ErrSubscribeRateLimited.Error(), got[id].Error, id)
		assert.False(t, isSubscribed(hub, c, addr), "a rate-limited subscribe must not reach the hub: %s", addr)
	}
	assert.Equal(t, ErrSubscribeRateLimited.Error(), got["u0"].Error)
	assert.True(t, isSubscribed(hub, c, "a0"))
	assert.Equal(t, burst, snapshotBudget(hub, c).perClient)
	assert.Empty(t, got["tx"].Error)
	assert.Empty(t, got["blk"].Error)

	// After a refill a subscribe goes through again.
	deadline := time.Now().Add(3 * time.Second)
	for attempt := 0; ; attempt++ {
		id := fmt.Sprintf("r%d", attempt)
		require.NoError(t, conn.WriteJSON(subscribeFrame(t, id, MethodSubscribe, "refill")))
		resp := readWSResponse(t, conn)
		require.Equal(t, id, resp.ID)
		if resp.Error == "" {
			break
		}
		require.Equal(t, ErrSubscribeRateLimited.Error(), resp.Error)
		require.True(t, time.Now().Before(deadline), "bucket never refilled")
		time.Sleep(100 * time.Millisecond)
	}
	assert.True(t, isSubscribed(hub, c, "refill"))
}

func TestLimits_Resolve_SubscribeRateDefaultsAndClamps(t *testing.T) {
	r := Limits{}.resolve()
	assert.Equal(t, 10, r.subscribeRatePerSec)
	assert.Equal(t, 20, r.subscribeBurst)
	assert.Equal(t, defaultSubscribeRatePerSec, r.subscribeRatePerSec)
	assert.Equal(t, defaultSubscribeBurst, r.subscribeBurst)

	r = Limits{SubscribeRatePerSec: 3, SubscribeBurst: 5}.resolve()
	assert.Equal(t, 3, r.subscribeRatePerSec)
	assert.Equal(t, 5, r.subscribeBurst)

	// Burst is raised to at least the rate, whether set or defaulted.
	r = Limits{SubscribeRatePerSec: 50, SubscribeBurst: 4}.resolve()
	assert.Equal(t, 50, r.subscribeBurst)
	r = Limits{SubscribeRatePerSec: 50}.resolve()
	assert.Equal(t, 50, r.subscribeBurst)

	r = Limits{SubscribeRatePerSec: -1, SubscribeBurst: -1}.resolve()
	assert.Equal(t, defaultSubscribeRatePerSec, r.subscribeRatePerSec)
	assert.Equal(t, defaultSubscribeBurst, r.subscribeBurst)
}

func TestNewClient_SubscribeLimiterFromHubLimits(t *testing.T) {
	_, c := dialLiveClient(t, NewHub("", "", nil, Limits{SubscribeRatePerSec: 4, SubscribeBurst: 6}))
	t.Cleanup(c.Close)
	require.NotNil(t, c.subscribeLimiter)
	assert.Equal(t, 4.0, float64(c.subscribeLimiter.Limit()))
	assert.Equal(t, 6, c.subscribeLimiter.Burst())
}
