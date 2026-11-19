package websocket_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/indexer"
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

func TestSubscribe_RejectsTooManyAddresses(t *testing.T) {
	hub := socket.NewHub("", "", nil, socket.Limits{MaxAddressesPerSubscribe: 2})
	addr, cleanup := startTestServerOpts(t, hub, wsocket.SubscribeOptions{})
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteJSON(map[string]interface{}{
		"subscribed_types": []string{"accounts"},
		"addresses":        []string{"a", "b", "c"}, // > per-subscribe cap of 2
	}))

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var resp map[string]string
	require.NoError(t, conn.ReadJSON(&resp))
	assert.Contains(t, resp["error"], "too many addresses")
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

// requireClosed asserts the server closed conn. A read deadline expiring is an error
// too, so "the read failed" alone would also hold for a connection that was wrongly kept
// open. Only a non-timeout failure proves the peer actually went away.
func requireClosed(t *testing.T, conn *websocket.Conn, msg string) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err := conn.ReadMessage()
	require.Error(t, err, msg)

	var netErr net.Error
	require.False(t, errors.As(err, &netErr) && netErr.Timeout(), msg)
}

// TestSubscribe_HubShutdownClosesTheConnection covers processSubscription's insertion
// failure path: once StartServer has torn the hub down, a fresh subscribe must be closed
// rather than registered into a hub that has already shut down (the closed flag deleteAll
// sets).
func TestSubscribe_HubShutdownClosesTheConnection(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	hubCtx, stopHub := context.WithCancel(context.Background())
	go hub.StartServer(hubCtx)

	addr, cleanup := startTestServerOpts(t, hub, wsocket.SubscribeOptions{})
	defer cleanup()

	subscribe := func(t *testing.T) *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
		require.NoError(t, err)
		require.NoError(t, conn.WriteJSON(map[string]interface{}{
			"subscribed_types": []string{"blocks"},
			"addresses":        []string{"klv1a"},
		}))
		return conn
	}

	// deleteAll sets the closed flag under the same lock, before it closes any
	// connection, so this socket dying is a happens-after edge on the flag: whether the
	// client was inserted first (deleteAll closes it) or not (the insertion is refused
	// and processSubscription closes it), the next subscribe is guaranteed to see it.
	live := subscribe(t)
	defer live.Close()

	stopHub()
	requireClosed(t, live, "hub teardown must close the live connection")

	next := subscribe(t)
	defer next.Close()

	requireClosed(t, next, "a subscribe against a shut-down hub must be closed, not registered")
}

// TestSubscribe_OversizedAddressReportsTheReasonThenCloses covers the cap rejection. It is
// reachable on any fresh connection — the route's own pre-check bounds the address count,
// not each address's byte length — so the peer must be told why rather than having the
// socket dropped on it. The route rejects before NewClient, while it still owns the raw
// connection and a write is safe.
func TestSubscribe_OversizedAddressReportsTheReasonThenCloses(t *testing.T) {
	hub := socket.NewHub("", "", nil)
	addr, cleanup := startTestServerOpts(t, hub, wsocket.SubscribeOptions{})
	defer cleanup()

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteJSON(map[string]interface{}{
		"subscribed_types": []string{"accounts"},
		"addresses":        []string{strings.Repeat("a", 63)}, // one past the 62-byte address cap
	}))

	var rejection map[string]string
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	require.NoError(t, conn.ReadJSON(&rejection), "the peer must be told why it was rejected")
	require.Contains(t, rejection["error"], "maximum length",
		"the reason must name the limit that was exceeded")

	requireClosed(t, conn, "a subscribe with an oversized address must be closed, not registered")
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

func TestSubscribe_AddressBudgetReportsTheReasonThenCloses(t *testing.T) {
	hub := socket.NewHub("", "", nil, socket.Limits{MaxAddressesPerSubscribe: 2, MaxAddressesPerClient: 2, MaxAddressesPerIP: 2, MaxTotalAddresses: 2})
	addr, cleanup := startTestServerOpts(t, hub, wsocket.SubscribeOptions{})
	defer cleanup()

	holder, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	defer holder.Close()
	require.NoError(t, holder.WriteJSON(map[string]interface{}{"subscribed_types": []string{"accounts"}, "addresses": []string{"a", "b"}}))

	// The holder's insertion is asynchronous; retry until the budget it fills is seen.
	var rejected *websocket.Conn
	require.Eventually(t, func() bool {
		conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
		if err != nil {
			return false
		}
		var rejection map[string]string
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		if conn.WriteJSON(map[string]interface{}{"subscribed_types": []string{"accounts"}, "addresses": []string{"c"}}) != nil ||
			conn.ReadJSON(&rejection) != nil || !strings.Contains(rejection["error"], "budget") {
			_ = conn.Close()
			return false
		}
		rejected = conn
		return true
	}, 5*time.Second, 50*time.Millisecond, "the peer must be told the budget is full")
	defer rejected.Close()
	// Asserted on the test goroutine: Eventually runs the condition on its own.
	requireClosed(t, rejected, "a subscribe past the address budget must be closed, not registered")
}

// logBuffer collects log output; the log subsystem is global, so writes are concurrent.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestSubscribe_PrecheckRefusalIsLogged(t *testing.T) {
	hub := socket.NewHub("", "", nil, socket.Limits{MaxAddressesPerSubscribe: 2, MaxAddressesPerClient: 2, MaxAddressesPerIP: 2, MaxTotalAddresses: 2})
	addr, cleanup := startTestServerOpts(t, hub, wsocket.SubscribeOptions{})
	defer cleanup()

	holder, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	defer holder.Close()
	require.NoError(t, holder.WriteJSON(map[string]interface{}{"subscribed_types": []string{"accounts"}, "addresses": []string{"a", "b"}}))
	// Once the holder is inserted, the next subscribe is refused at the precheck.
	local := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1}
	require.Eventually(t, func() bool {
		return hub.PrecheckAddressBudget([]indexer.EventType{indexer.ACCOUNTS}, []string{"c"}, local) != nil
	}, 5*time.Second, 20*time.Millisecond)

	var observed logBuffer
	require.NoError(t, logger.AddLogObserver(&observed, &logger.PlainFormatter{}))
	t.Cleanup(func() { assert.NoError(t, logger.RemoveLogObserver(&observed)) })

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/subscribe", nil)
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, conn.WriteJSON(map[string]interface{}{"subscribed_types": []string{"accounts"}, "addresses": []string{"c"}}))
	var rejection map[string]string
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	require.NoError(t, conn.ReadJSON(&rejection))
	require.Contains(t, rejection["error"], "budget")

	// Logged before the rejection frame is written, so it is already in the buffer.
	out := observed.String()
	assert.Contains(t, out, "ws.Subscribe")
	assert.Contains(t, out, "per-ip")
}

func TestRemoteIP_KeysIPv6By64(t *testing.T) {
	a := &http.Request{RemoteAddr: "[2001:db8:1:201::1]:1111"}
	b := &http.Request{RemoteAddr: "[2001:db8:1:202::1]:2222"}
	assert.Equal(t, "2001:db8:1:201::", wsocket.RemoteIP(a))
	assert.NotEqual(t, wsocket.RemoteIP(a), wsocket.RemoteIP(b), "the connection limiter keys each /64 apart")
	assert.Equal(t, "2001:db8:1:201::", wsocket.RemoteIP(&http.Request{RemoteAddr: "2001:db8:1:201::1"}), "a bare host still buckets")
	assert.Equal(t, "192.0.2.7", wsocket.RemoteIP(&http.Request{RemoteAddr: "192.0.2.7:1"}))
}
