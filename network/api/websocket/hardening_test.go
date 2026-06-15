package websocket_test

import (
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	wsocket "github.com/klever-io/klever-go/network/api/websocket"
	socket "github.com/klever-io/klever-go/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startTestServerOpts(t *testing.T, hub *socket.SocketHub, opts wsocket.SubscribeOptions) (string, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ws := gin.New()
	wsocket.SubscribeTopics(ws, hub, opts)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := &http.Server{Handler: ws, ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(listener) }()

	return listener.Addr().String(), func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}

// TestSubscribe_ReadLimit_RejectsOversizedFrame confirms GAP#2 is closed: a single
// frame larger than MaxMessageSize is rejected with WebSocket close 1009 instead of
// being buffered whole.
func TestSubscribe_ReadLimit_RejectsOversizedFrame(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	addr, cleanup := startTestServerOpts(t, hub, wsocket.SubscribeOptions{})
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	defer conn.Close()

	// A valid-JSON subscribe frame whose address string pushes the message past the
	// read limit, so the decoder keeps reading until SetReadLimit fires (a non-JSON
	// blob would fail to parse on the first byte, before the limit triggers).
	oversized := []byte(`{"subscribed_types":["accounts"],"addresses":["` +
		strings.Repeat("A", int(hub.MaxMessageSize())) + `"]}`)

	// Write from a goroutine and ignore its error: the server stops reading and hard-
	// closes at the limit, which can reset the client mid-write. The security property
	// is that the frame is rejected, not buffered whole.
	go func() { _ = conn.WriteMessage(websocket.TextMessage, oversized) }()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err = conn.ReadMessage()
	require.Error(t, err, "oversized frame must terminate the connection, not be accepted")
	if ce, ok := err.(*websocket.CloseError); ok {
		assert.Equal(t, websocket.CloseMessageTooBig, ce.Code, "clean close must be 1009 (message too big)")
	}
}

func TestSubscribe_GlobalConnectionCap(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	addr, cleanup := startTestServerOpts(t, hub, wsocket.SubscribeOptions{MaxConnections: 2})
	defer cleanup()

	var conns []*websocket.Conn
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()

	for i := 0; i < 2; i++ {
		c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
		require.NoErrorf(t, err, "connection %d within the cap should be accepted", i)
		conns = append(conns, c)
	}

	_, resp, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.Error(t, err, "connection beyond the global cap must be rejected")
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestSubscribe_PerIPConnectionCap(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	addr, cleanup := startTestServerOpts(t, hub, wsocket.SubscribeOptions{MaxConnectionsPerIP: 2})
	defer cleanup()

	var conns []*websocket.Conn
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()

	for i := 0; i < 2; i++ {
		c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
		require.NoError(t, err)
		conns = append(conns, c)
	}

	_, resp, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.Error(t, err, "connection beyond the per-IP cap must be rejected")
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestSubscribe_ConnectionCap_ReleasedOnClose(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	addr, cleanup := startTestServerOpts(t, hub, wsocket.SubscribeOptions{MaxConnections: 1})
	defer cleanup()

	c1, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)

	// Cap reached.
	_, resp, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.Error(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	// Close the first connection; the slot must free up.
	_ = c1.Close()
	require.Eventually(t, func() bool {
		c, _, derr := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
		if derr != nil {
			return false
		}
		_ = c.Close()
		return true
	}, 3*time.Second, 50*time.Millisecond, "slot must be released after the connection closes")
}

// TestSubscribe_Secured confirms the AuthHandlers wiring runs before the WebSocket
// upgrade, so `secured: true` is honoured instead of silently ignored.
func TestSubscribe_Secured(t *testing.T) {
	auth := func(c *gin.Context) {
		user, pass, ok := c.Request.BasicAuth()
		if !ok || user != "u" || pass != "p" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}

	hub := socket.NewHub("", "", nil)
	addr, cleanup := startTestServerOpts(t, hub, wsocket.SubscribeOptions{AuthHandlers: []gin.HandlerFunc{auth}})
	defer cleanup()

	_, resp, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.Error(t, err, "handshake without auth must be rejected")
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	hdr := http.Header{}
	hdr.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("u:p")))
	conn, resp2, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", hdr)
	require.NoError(t, err, "handshake with valid auth must be accepted")
	require.Equal(t, http.StatusSwitchingProtocols, resp2.StatusCode)
	_ = conn.Close()
}
