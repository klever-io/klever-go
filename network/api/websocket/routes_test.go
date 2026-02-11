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
		"addresses":       []string{"klv1test"},
		"subcribed_types": []string{"blocks"},
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
		"addresses":       []string{},
		"subcribed_types": []string{"invalid_type"},
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
		"addresses":       []string{"klv1abc"},
		"subcribed_types": []string{"blocks", "transactions", "accounts", "user_transaction"},
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
		"addresses":       []string{"klv1abc"},
		"subcribed_types": []string{"blocks", "blocks"},
	}
	err = conn.WriteJSON(msg)
	assert.NoError(t, err)
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

func TestSubscribeTopics_ThenSendRequest(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	addr, cleanup := startTestServer(t, hub)
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	msg := map[string]interface{}{
		"addresses":       []string{},
		"subcribed_types": []string{"blocks"},
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
