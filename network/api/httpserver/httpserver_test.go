package httpserver

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// TestNewHardenedServer_SetsSlowHeaderDefenses guards GHSA-w4c6-7r69-w7j9: the
// hardening fields are set, and ReadTimeout/WriteTimeout stay zero so the APIs'
// long-lived websocket streams are not severed.
func TestNewHardenedServer_SetsSlowHeaderDefenses(t *testing.T) {
	t.Parallel()

	srv := NewHardenedServer(":8080", http.NewServeMux())

	require.Equal(t, ":8080", srv.Addr)
	require.NotNil(t, srv.Handler)
	require.Equal(t, ReadHeaderTimeout, srv.ReadHeaderTimeout)
	require.Positive(t, srv.ReadHeaderTimeout, "ReadHeaderTimeout must be set to defeat slow-header DoS")
	require.Equal(t, IdleTimeout, srv.IdleTimeout)
	require.Equal(t, MaxHeaderBytes, srv.MaxHeaderBytes)

	// Websocket safety: a whole-connection deadline would kill long-lived streams.
	require.Zero(t, srv.ReadTimeout, "ReadTimeout must stay unset to not sever websocket streams")
	require.Zero(t, srv.WriteTimeout, "WriteTimeout must stay unset to not sever websocket streams")
}

// TestNewHardenedServer_ClosesSlowHeaderConnection proves end-to-end that a connection
// whose header never completes is dropped, not pinned. Short ReadHeaderTimeout for speed.
func TestNewHardenedServer_ClosesSlowHeaderConnection(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.Nil(t, err)

	srv := NewHardenedServer(ln.Addr().String(), http.NewServeMux())
	srv.ReadHeaderTimeout = 200 * time.Millisecond // tighten for a fast test
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	conn, err := net.Dial("tcp", ln.Addr().String())
	require.Nil(t, err)
	defer func() { _ = conn.Close() }()

	// Send headers but never the terminating blank line, so the header never completes.
	_, err = fmt.Fprint(conn, "GET / HTTP/1.1\r\nHost: x\r\n")
	require.Nil(t, err)

	// With ReadHeaderTimeout the server drops the connection ~200ms in; a sub-second
	// return proves the defense (without it, the read blocks until our 3s deadline).
	start := time.Now()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _ = bufio.NewReader(conn).ReadString('\n')
	require.Less(t, time.Since(start), time.Second,
		"server must drop the stalled slow-header connection promptly (ReadHeaderTimeout)")
}

// TestNewHardenedServer_DropsDrippingSlowHeader proves ReadHeaderTimeout is an absolute
// deadline, not a reset-on-read idle timer: a client actively dripping header bytes
// (never completing the header) is still dropped at ~ReadHeaderTimeout.
func TestNewHardenedServer_DropsDrippingSlowHeader(t *testing.T) {
	t.Parallel()

	const headerTimeout = 400 * time.Millisecond

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.Nil(t, err)

	srv := NewHardenedServer(ln.Addr().String(), http.NewServeMux())
	srv.ReadHeaderTimeout = headerTimeout // tighten for a fast test
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	conn, err := net.Dial("tcp", ln.Addr().String())
	require.Nil(t, err)
	defer func() { _ = conn.Close() }()

	_, err = fmt.Fprint(conn, "GET / HTTP/1.1\r\nHost: x\r\n")
	require.Nil(t, err)

	// Drip header lines well under the timeout interval, never ending the header.
	dripStop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(headerTimeout / 8)
		defer ticker.Stop()
		for i := 0; ; i++ {
			select {
			case <-dripStop:
				return
			case <-ticker.C:
				if _, werr := fmt.Fprintf(conn, "X-Pad-%d: y\r\n", i); werr != nil {
					return // server closed the connection
				}
			}
		}
	}()
	defer close(dripStop)

	start := time.Now()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _ = bufio.NewReader(conn).ReadString('\n')
	elapsed := time.Since(start)

	require.Less(t, elapsed, time.Second,
		"absolute ReadHeaderTimeout must drop an actively-dripping slow-header connection")
	require.GreaterOrEqual(t, elapsed, headerTimeout/2,
		"connection should survive until ~the header deadline, not be dropped on the first read")
}

// TestNewHardenedServer_CapsRequestBody guards the body-size cap: a body over the
// limit is refused, one within it is read normally. Small explicit limit for speed.
func TestNewHardenedServer_CapsRequestBody(t *testing.T) {
	t.Parallel()

	const limit = 16

	// Status carries the read outcome, so assertions read only the response code —
	// no state shared across the server/test goroutines.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capRequestBody(w, r, limit)
		if _, err := io.ReadAll(r.Body); err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Over the cap: MaxBytesReader fails the body read, so the handler replies 413.
	resp, err := http.Post(srv.URL, "application/octet-stream", bytes.NewReader(make([]byte, limit+1)))
	require.Nil(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode, "body over the cap must be refused")

	// Within the cap: the body reads cleanly and the handler replies 200.
	resp, err = http.Post(srv.URL, "application/octet-stream", bytes.NewReader(make([]byte, limit)))
	require.Nil(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "body within the cap must be accepted")
}

// TestApplyBodyReadDeadline_CutsSlowBody: a client that completes the header, promises
// a large body, then stalls is cut at ~BodyReadTimeout instead of pinning the connection.
func TestApplyBodyReadDeadline_CutsSlowBody(t *testing.T) {
	t.Parallel()

	const deadline = 300 * time.Millisecond

	readDone := make(chan error, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applyBodyReadDeadline(w, r, deadline)
		_, err := io.ReadAll(r.Body)
		readDone <- err
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	require.Nil(t, err)
	defer func() { _ = conn.Close() }()

	// Promise 1 MiB but send only a few bytes, then stall — the body read blocks.
	_, err = fmt.Fprint(conn, "POST / HTTP/1.1\r\nHost: x\r\nContent-Length: 1048576\r\n\r\npartial")
	require.Nil(t, err)

	start := time.Now()
	select {
	case err := <-readDone:
		require.Error(t, err, "a stalled body read must be cut by the deadline")
		require.Less(t, time.Since(start), 2*time.Second, "body must be cut promptly (~BodyReadTimeout)")
	case <-time.After(2 * time.Second):
		t.Fatal("stalled body read was not cut by the deadline")
	}
}

// TestApplyBodyReadDeadline_DoesNotCancelSlowHandler: a handler that reads the body then
// works past the deadline still returns 200 — the deadline bounds the body read, not
// post-read CPU work (e.g. a VM query) or the response write. (clear() has no observable
// effect to assert in standard net/http, so this checks the property that matters.)
func TestApplyBodyReadDeadline_DoesNotCancelSlowHandler(t *testing.T) {
	t.Parallel()

	const deadline = 200 * time.Millisecond

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applyBodyReadDeadline(w, r, deadline)
		if _, err := io.ReadAll(r.Body); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		time.Sleep(3 * deadline) // work well past the deadline
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Post(srv.URL, "text/plain", bytes.NewReader([]byte("hello")))
	require.Nil(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"the body deadline must not cancel a slow post-read handler")
}

// TestRequestHasBody covers the predicate that gates the body deadline: bodiless requests
// (including the bodiless-GET websocket handshakes) are skipped.
func TestRequestHasBody(t *testing.T) {
	t.Parallel()

	require.False(t, requestHasBody(&http.Request{ContentLength: 0}), "no body (incl. websocket handshake)")
	require.True(t, requestHasBody(&http.Request{ContentLength: 5}), "known body")
	require.True(t, requestHasBody(&http.Request{ContentLength: -1}), "chunked body")
}

// TestApplyBodyReadDeadline_SkipsBodilessUpgrade: a bodiless GET with Upgrade: websocket
// is not wrapped (no deadline armed), so the /log and /subscribe streams are never severed.
func TestApplyBodyReadDeadline_SkipsBodilessUpgrade(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		before := r.Body
		applyBodyReadDeadline(w, r, BodyReadTimeout)
		// r.Body must be unchanged — a wrapped body would mean a deadline was armed.
		if r.Body != before {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil) // bodiless GET, like a handshake
	require.Nil(t, err)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	resp, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "bodiless websocket handshake must not be wrapped")
}

// TestNewHardenedServer_WebSocketStreamWorks: a real websocket upgrade survives the
// hardened server — frames round-trip across idle gaps without the stream being severed.
func TestNewHardenedServer_WebSocketStreamWorks(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		for { // echo until the client closes
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(mt, msg); err != nil {
				return
			}
		}
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.Nil(t, err)
	srv := NewHardenedServer(ln.Addr().String(), mux)
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	dialer := websocket.Dialer{}
	wsURL := "ws://" + ln.Addr().String() + "/ws"
	conn, resp, err := dialer.Dial(wsURL, nil)
	require.Nil(t, err, "websocket upgrade must succeed through the hardened server")
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	defer func() { _ = conn.Close() }()

	// Exchange frames with an idle gap between them to show the stream stays open and is
	// not cut by ReadHeaderTimeout/IdleTimeout/body deadline once upgraded.
	for i, gap := range []time.Duration{0, 250 * time.Millisecond, 250 * time.Millisecond} {
		time.Sleep(gap)
		want := fmt.Sprintf("frame-%d", i)
		require.Nil(t, conn.WriteMessage(websocket.TextMessage, []byte(want)))
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, got, err := conn.ReadMessage()
		require.Nil(t, err, "frame %d must round-trip", i)
		require.Equal(t, want, string(got))
	}
}
