package websocket

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/klever-io/klever-go/data/api"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// KLC-2596 regression tests for the panic barriers on the client goroutines.
//
// Read these as survival tests first: every goroutine covered here is spawned outside the
// gin handler, so gin.Recovery() cannot see it. If a barrier is removed, the injected panic
// is unrecovered and takes down the whole test binary — the package fails loudly rather
// than reporting a clean assertion failure. That crash *is* the regression signal; the
// assertions below then pin the recovered behaviour on top of it.

// dialLiveClient starts a real server that hands each accepted connection to NewClient —
// the same wiring processSubscription uses — and returns both ends: the dialed client
// connection and the server-side client whose goroutines are under test.
func dialLiveClient(t *testing.T, hub *SocketHub) (*ws.Conn, *client) {
	t.Helper()

	upgrader := ws.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	served := make(chan *client, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// t.Errorf (unlike Fatal/FailNow) is safe to call from any goroutine.
			t.Errorf("server failed to upgrade the websocket connection: %v", err)
			return
		}
		served <- NewClient(conn, hub)
	}))
	t.Cleanup(srv.Close)

	conn, _, err := ws.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	select {
	case c := <-served:
		return conn, c
	case <-time.After(3 * time.Second):
		t.Fatal("server never handed the connection to NewClient")
		return nil, nil
	}
}

func readWSResponse(t *testing.T, conn *ws.Conn) WSResponse {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))

	var resp WSResponse
	require.NoError(t, conn.ReadJSON(&resp))
	return resp
}

// TestPanicBarrier_HandleClientRequest_KeepsNodeAlive covers the widest instance of the
// class: the per-request worker runs an attacker-supplied req through the same facade
// methods REST serves behind gin.Recovery(). The blast radius of a panic there must be one
// request — not the connection, and certainly not the process.
func TestPanicBarrier_HandleClientRequest_KeepsNodeAlive(t *testing.T) {
	hub := newTestHub(&mockFacade{
		getTransactionFn: func(string, bool) (*api.Transaction, error) {
			panic("boom: simulated facade panic on an attacker-supplied request")
		},
		getBlockByNonceFn: func(uint64, bool) (*api.Block, error) {
			return &api.Block{}, nil
		},
	})
	conn, _ := dialLiveClient(t, hub)

	require.NoError(t, conn.WriteJSON(WSRequest{
		ID:     "panics",
		Method: MethodGetTransaction,
		Params: json.RawMessage(`{"hash":"abc"}`),
	}))

	resp := readWSResponse(t, conn)
	assert.Equal(t, "panics", resp.ID, "the recovered request must still be answered, and correlated")
	assert.Equal(t, errInternal, resp.Error, "a recovered panic must answer with the opaque internal error")
	assert.NotContains(t, resp.Error, "boom", "the panic value must never reach an unauthenticated client")

	// The connection must still be serving afterwards: one poisoned request may not cost the
	// client its subscription.
	require.NoError(t, conn.WriteJSON(WSRequest{
		ID:     "survives",
		Method: MethodGetBlock,
		Params: json.RawMessage(`{"nonce":7}`),
	}))

	resp = readWSResponse(t, conn)
	assert.Equal(t, "survives", resp.ID)
	assert.Empty(t, resp.Error, "the connection must keep serving after a recovered panic")
}

// TestPanicBarrier_HandleClientRequest_ReleasesWorkerSlot pins the resource half of the
// contract: recovering must not cost the connection its worker slot. Surviving one panic is
// not enough — if a recovered request kept its slot, maxWorkers panics would wedge the
// connection for good while leaving the process alive, which is exactly the failure the
// survival assertions above cannot see.
//
// This is a property of the barrier, not of the two defers' relative order: a receive on a
// live channel cannot panic, and Go runs the remaining defers even when a deferred call
// panics, so the slot comes back under either ordering. What would break it is a barrier
// that swallowed the unwind before the release ran at all.
func TestPanicBarrier_HandleClientRequest_ReleasesWorkerSlot(t *testing.T) {
	hub := newTestHub(&mockFacade{
		getTransactionFn: func(string, bool) (*api.Transaction, error) {
			panic("boom: simulated facade panic on an attacker-supplied request")
		},
	})
	conn, c := dialLiveClient(t, hub)

	// One more than the pool holds: if a single slot leaked, this loop could not finish.
	for i := 0; i < maxWorkers+1; i++ {
		require.NoError(t, conn.WriteJSON(WSRequest{
			ID:     "panics",
			Method: MethodGetTransaction,
			Params: json.RawMessage(`{"hash":"abc"}`),
		}))
		resp := readWSResponse(t, conn)
		require.Equal(t, errInternal, resp.Error)
	}

	require.Eventually(t, func() bool { return len(c.sem) == 0 }, 3*time.Second, 20*time.Millisecond,
		"every recovered request must hand its worker slot back; a leak here wedges the connection")
}

// panicReadConn panics from Read once armed, after the websocket handshake has finished.
type panicReadConn struct {
	net.Conn
	prefix      []byte
	panicOnRead atomic.Bool
}

func (p *panicReadConn) Read(b []byte) (int, error) {
	if p.panicOnRead.Load() {
		panic("boom: simulated panic on the read path")
	}
	if len(p.prefix) > 0 {
		n := copy(b, p.prefix)
		p.prefix = p.prefix[n:]
		return n, nil
	}
	return p.Conn.Read(b)
}

// panicHijackWriter hands Upgrade a connection whose later reads can be made to panic.
type panicHijackWriter struct {
	http.ResponseWriter
	conn *panicReadConn
}

func (w *panicHijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	raw, brw, err := w.ResponseWriter.(http.Hijacker).Hijack()
	if err != nil {
		return nil, nil, err
	}
	var prefix []byte
	if n := brw.Reader.Buffered(); n > 0 {
		prefix = make([]byte, n)
		if _, err := io.ReadFull(brw.Reader, prefix); err != nil {
			_ = raw.Close()
			return nil, nil, err
		}
	}
	w.conn = &panicReadConn{Conn: raw, prefix: prefix}
	return w.conn, bufio.NewReadWriter(bufio.NewReader(w.conn), bufio.NewWriter(w.conn)), nil
}

// dialPanicReadServer upgrades one connection and returns the server side plus the
// conn that loopIn will read. The panic stays disarmed until the test arms it.
func dialPanicReadServer(t *testing.T) (*ws.Conn, *panicReadConn) {
	t.Helper()

	upgrader := ws.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	served := make(chan *ws.Conn, 1)
	var raw *panicReadConn
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hw := &panicHijackWriter{ResponseWriter: w}
		conn, err := upgrader.Upgrade(hw, r, nil)
		if err != nil {
			t.Errorf("server failed to upgrade the websocket connection: %v", err)
			return
		}
		raw = hw.conn
		served <- conn
	}))
	t.Cleanup(srv.Close)

	peer, _, err := ws.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = peer.Close() })

	select {
	case conn := <-served:
		t.Cleanup(func() { _ = conn.Close() })
		require.NotNil(t, raw)
		return conn, raw
	case <-time.After(3 * time.Second):
		t.Fatal("server never finished the websocket upgrade")
		return nil, nil
	}
}

func assertClientDeregistered(t *testing.T, hub *SocketHub, c *client) {
	t.Helper()
	_, inClients := hub.clients[c]
	_, inBlocks := hub.blockSubscription[c]
	_, inTxs := hub.transactionSubscription[c]
	_, inAddrs := hub.clientAddresses[c]
	assert.False(t, inClients, "closed client must leave hub.clients")
	assert.False(t, inBlocks, "closed client must leave blockSubscription")
	assert.False(t, inTxs, "closed client must leave transactionSubscription")
	assert.False(t, inAddrs, "closed client must leave clientAddresses")
	for addr, subs := range hub.addressSubscription {
		_, ok := subs[c]
		assert.Falsef(t, ok, "closed client still subscribed to %s", addr)
	}
}

// TestPanicBarrier_LoopIn_ClosesAndDeregisters panics on the read path of a registered
// client. A barrier that recovered but skipped Close or handleClientDelete fails here.
func TestPanicBarrier_LoopIn_ClosesAndDeregisters(t *testing.T) {
	hub := newTestHub(nil)
	serverConn, raw := dialPanicReadServer(t)

	c := newTestClient(hub)
	c.conn = serverConn
	c.ctx, c.cancel = context.WithCancel(context.Background())
	require.NoError(t, hub.HandleClientInsertion([]indexer.EventType{indexer.BLOCKS}, nil, c))

	raw.panicOnRead.Store(true)
	assertReturnsQuickly(t, 2*time.Second, "loopIn did not return; its panic barrier failed to recover", func() {
		c.loopIn()
	})

	assert.False(t, c.IsAlive(), "loopIn's panic barrier must close the client")
	select {
	case <-c.Done():
	default:
		t.Fatal("loopIn's teardown must cancel the client context")
	}
	assertClientDeregistered(t, hub, c)
}

// TestPanicBarrier_LoopIn_RecoversPanicInTeardown panics inside Close, the first teardown
// step. The barrier defer is registered before that teardown, so it runs after it and must
// recover the panic. Swapping the defers lets Close's panic escape and crash the test.
func TestPanicBarrier_LoopIn_RecoversPanicInTeardown(t *testing.T) {
	c := newTestClient(newTestHub(nil))

	assertReturnsQuickly(t, 2*time.Second, "loopIn did not return; a panic in Close escaped the barrier", func() {
		c.loopIn()
	})
}

// panicMarshaler panics as the outbound writer marshals it, standing in for any latent
// panic on the write path.
type panicMarshaler struct{}

func (panicMarshaler) MarshalJSON() ([]byte, error) {
	panic("boom: simulated panic while marshalling an outbound frame")
}

// TestPanicBarrier_LoopOut_KeepsNodeAlive asserts the write loop's barrier tears the
// connection down rather than leaving a client with no writer behind it.
func TestPanicBarrier_LoopOut_KeepsNodeAlive(t *testing.T) {
	_, c := dialLiveClient(t, newTestHub(nil))

	c.send(panicMarshaler{})

	require.Eventually(t, func() bool { return !c.IsAlive() }, 3*time.Second, 20*time.Millisecond,
		"loopOut's panic barrier must close the client instead of leaving a half-live connection")
}

// TestPanicWarner_BudgetsAreNotSharedAcrossBarriers pins the separation the stack budgets
// exist for. A peer that can drive a panic on one path must not spend the budget a rarer
// defect on another path needs: if both shared a window, the rare panic would log with its
// stack omitted and the only evidence it leaves would be gone — attached instead to an
// unrelated panic in a different goroutine.
func TestPanicWarner_BudgetsAreNotSharedAcrossBarriers(t *testing.T) {
	t.Parallel()

	hub := newTestHub(nil)

	// Spend the request path's window, the one a peer can drive.
	_, ok := hub.panicWarner(opHandleClientRequest).fire()
	require.True(t, ok, "the first panic on a path must open that path's window")
	_, ok = hub.panicWarner(opHandleClientRequest).fire()
	require.False(t, ok, "a second panic on the same path within the window is stack-suppressed")

	// The rarer paths must still be able to report a stack.
	for _, op := range []string{opLoopIn, opLoopOut, "ws.somethingNew"} {
		_, ok := hub.panicWarner(op).fire()
		assert.True(t, ok, "%s must keep its own stack budget; a flood elsewhere may not spend it", op)
	}

	assert.Same(t, hub.panicWarner("ws.somethingNew"), hub.panicWarner("ws.anotherNewOne"),
		"an unnamed barrier must fall back to a shared budget, never to no limit")
}

// TestLoggablePanic_ScrubsPeerInputFromTheLogLine pins the sanitising the panic path adds:
// a panic value routinely quotes what the peer supplied, so it reaches the log the same way
// an error does and gets the same treatment.
func TestLoggablePanic_ScrubsPeerInputFromTheLogLine(t *testing.T) {
	t.Parallel()

	t.Run("control runes cannot forge a log line", func(t *testing.T) {
		t.Parallel()
		got := loggablePanic("boom\nERROR forged line")
		assert.NotContains(t, got, "\n", "a newline in a panic value would forge a log line of its own")
		assert.Equal(t, "boom ERROR forged line", got)
	})

	t.Run("an oversized value is bounded", func(t *testing.T) {
		t.Parallel()
		got := loggablePanic(strings.Repeat("A", 4096))
		assert.Less(t, len(got), 4096, "an unbounded panic value is a log-amplification lever")
		assert.Contains(t, got, "4096 bytes", "the bound must still report the original size")
	})
}
