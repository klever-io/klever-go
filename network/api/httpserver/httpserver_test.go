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
	handler := limitRequestBodyN(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}), limit)

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
