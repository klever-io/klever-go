package websocket_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/klever-io/klever-go/indexer"
	wsdata "github.com/klever-io/klever-go/indexer/data"
	wsocket "github.com/klever-io/klever-go/network/api/websocket"
	socket "github.com/klever-io/klever-go/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startTestServer(t *testing.T, hub *socket.SocketHub) (string, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ws := gin.New()
	ws.Use(cors.Default())
	wsocket.SubscribeTopics(ws, hub)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := &http.Server{
		Handler:           ws,
		ReadHeaderTimeout: 1 * time.Second,
	}

	go func() { _ = srv.Serve(listener) }()

	addr := listener.Addr().String()
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}

	return addr, cleanup
}

func TestSubscribeTopics(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	addr, cleanup := startTestServer(t, hub)
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	msg := map[string]interface{}{
		"addresses":        []string{"klv1test"},
		"subscribed_types": []string{"blocks"},
	}
	err = conn.WriteJSON(msg)
	assert.NoError(t, err)
}

func TestSubscribeTopics_InvalidType(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	addr, cleanup := startTestServer(t, hub)
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	msg := map[string]interface{}{
		"addresses":        []string{},
		"subscribed_types": []string{"invalid_type"},
	}
	err = conn.WriteJSON(msg)
	require.NoError(t, err)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var resp map[string]string
	err = conn.ReadJSON(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "invalid subscription type")
}

func TestSubscribeTopics_ValidTypes(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	addr, cleanup := startTestServer(t, hub)
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	msg := map[string]interface{}{
		"addresses":        []string{"klv1abc"},
		"subscribed_types": []string{"blocks", "transactions", "accounts", "user_transactions", "logs"},
	}
	err = conn.WriteJSON(msg)
	assert.NoError(t, err)
}

func TestSubscribeTopics_DuplicateTypes(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	addr, cleanup := startTestServer(t, hub)
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	msg := map[string]interface{}{
		"addresses":        []string{"klv1abc"},
		"subscribed_types": []string{"blocks", "blocks"},
	}
	err = conn.WriteJSON(msg)
	assert.NoError(t, err)
}

func TestSubscribeTopics_EmptyTypes(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	addr, cleanup := startTestServer(t, hub)
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	msg := map[string]interface{}{
		"addresses":        []string{},
		"subscribed_types": []string{},
	}
	err = conn.WriteJSON(msg)
	require.NoError(t, err)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var resp map[string]string
	err = conn.ReadJSON(&resp)
	require.NoError(t, err)
	assert.Equal(t, "subscribed_types must not be empty", resp["error"])
}

func TestSubscribeTopics_InvalidJSON(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	addr, cleanup := startTestServer(t, hub)
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	err = conn.WriteMessage(websocket.TextMessage, []byte(`not valid json`))
	require.NoError(t, err)

	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, _, err = conn.ReadMessage()
	assert.Error(t, err)
}

func TestSubscribeTopics_BlockEventDelivery(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.StartServer(ctx)

	addr, cleanup := startTestServer(t, hub)
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	msg := map[string]interface{}{
		"addresses":        []string{},
		"subscribed_types": []string{"blocks"},
	}
	err = conn.WriteJSON(msg)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	blockJSON := []byte(`{"nonce":1,"hash":"abc123"}`)
	indexer.EventQueue <- indexer.Event{
		EvType:  indexer.BLOCKS,
		Message: blockJSON,
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var received socket.Send
	err = conn.ReadJSON(&received)
	require.NoError(t, err, "expected to receive block event but got nothing")
	assert.Equal(t, indexer.BLOCKS, received.Type)
}

func TestSubscribeTopics_LogsEventDelivery(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.StartServer(ctx)

	addr, cleanup := startTestServer(t, hub)
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	// Minimal initial handshake — the actual `logs` subscription is added dynamically
	// below via the `subscribe` method, which (unlike the initial handshake) sends back
	// a "subscribed" ack. Waiting on that ack lets the LOGS event be pushed only once the
	// subscription is deterministically registered, instead of guessing with a sleep.
	msg := map[string]interface{}{
		"addresses":        []string{},
		"subscribed_types": []string{"blocks"},
	}
	err = conn.WriteJSON(msg)
	require.NoError(t, err)

	subReq := socket.WSRequest{
		ID:     "sub-logs",
		Method: socket.MethodSubscribe,
		Params: json.RawMessage(`{"types":["logs"],"addresses":["klv1contract"]}`),
	}
	err = conn.WriteJSON(subReq)
	require.NoError(t, err)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var subResp socket.WSResponse
	err = conn.ReadJSON(&subResp)
	require.NoError(t, err, "expected a response to the dynamic subscribe request")
	require.Equal(t, "sub-logs", subResp.ID)
	require.Empty(t, subResp.Error)

	indexer.EventQueue <- indexer.Event{
		EvType: indexer.LOGS,
		Message: []*wsdata.Logs{
			{Address: "klv1contract", Events: []*wsdata.Event{{Identifier: "transfer"}}},
		},
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var received socket.Send
	err = conn.ReadJSON(&received)
	require.NoError(t, err, "expected to receive logs event but got nothing")
	assert.Equal(t, indexer.LOGS, received.Type)
	assert.Equal(t, "klv1contract", received.Address)
}

func TestSubscribeTopics_ThenSendRequest(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	addr, cleanup := startTestServer(t, hub)
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	msg := map[string]interface{}{
		"addresses":        []string{},
		"subscribed_types": []string{"blocks"},
	}
	err = conn.WriteJSON(msg)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	req := socket.WSRequest{
		ID:     "test-1",
		Method: "get_transaction",
		Params: json.RawMessage(`{"hash":"abc"}`),
	}
	err = conn.WriteJSON(req)
	require.NoError(t, err)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var resp socket.WSResponse
	err = conn.ReadJSON(&resp)
	require.NoError(t, err)
	assert.Equal(t, "test-1", resp.ID)
	assert.Contains(t, resp.Error, "facade unavailable")
}
