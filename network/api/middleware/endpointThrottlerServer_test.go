package middleware_test

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/core/throttler"
	"github.com/klever-io/klever-go/network/api/httpserver"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startHardenedServer serves a throttled POST and GET route behind the global throttler,
// each with a single slot, through the same hardened http.Server wrapper the node uses,
// so net/http's body handling runs for real
func startHardenedServer(t *testing.T) *httptest.Server {
	endpointThrottler, err := throttler.NewNumGoRoutinesThrottler(1)
	require.NoError(t, err)
	globalThrottler, err := middleware.NewGlobalThrottler(1)
	require.NoError(t, err)

	ws := gin.New()
	ws.Use(globalThrottler.MiddlewareHandlerFunc())
	ws.Use(middleware.WithFacade(&realThrottlerFacade{throttler: endpointThrottler}))
	ws.POST(throttledPath, middleware.CreateEndpointThrottler(throttledPath), readBodyHandler)
	ws.GET(throttledPath, middleware.CreateEndpointThrottler(throttledPath), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	hardened := httpserver.NewHardenedServer("", ws)
	srv := httptest.NewUnstartedServer(hardened.Handler)
	srv.Config.ReadHeaderTimeout = hardened.ReadHeaderTimeout
	srv.Start()
	t.Cleanup(srv.Close)

	return srv
}

func dialServer(t *testing.T, srv *httptest.Server) net.Conn {
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return conn
}

// readStatus reads the status line of the response on conn, failing after timeout
func readStatus(t *testing.T, conn net.Conn, timeout time.Duration) int {
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(timeout)))
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err, "no response within %s", timeout)
	_ = resp.Body.Close()

	return resp.StatusCode
}

func TestCreateEndpointThrottler_StalledOverCapBodyIsAnsweredAndFreesTheSlot(t *testing.T) {
	t.Parallel()

	srv := startHardenedServer(t)

	// a chunked body one byte over the cap, then the client stalls without ending the body
	conn := dialServer(t, srv)
	overCap := httpserver.MaxBodyBytes + 1
	header := fmt.Sprintf("POST %s HTTP/1.1\r\nHost: test\r\nTransfer-Encoding: chunked\r\n\r\n%x\r\n", throttledPath, overCap)
	go func() {
		_, _ = conn.Write([]byte(header))
		_, _ = conn.Write([]byte(strings.Repeat("a", int(overCap))))
		_, _ = conn.Write([]byte("\r\n"))
	}()

	assert.Equal(t, http.StatusRequestEntityTooLarge, readStatus(t, conn, 5*time.Second))

	// the stalled client holds no slot: the only one serves the next request
	next := dialServer(t, srv)
	_, err := next.Write([]byte(fmt.Sprintf("POST %s HTTP/1.1\r\nHost: test\r\nContent-Length: 2\r\n\r\nok", throttledPath)))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, readStatus(t, next, 5*time.Second))
}

func TestCreateEndpointThrottler_GetWithUnsentBodyHoldsNoSlot(t *testing.T) {
	t.Parallel()

	srv := startHardenedServer(t)

	// declares a body it never sends; the GET handler does not read bodies
	stalled := dialServer(t, srv)
	_, err := stalled.Write([]byte(fmt.Sprintf("GET %s HTTP/1.1\r\nHost: test\r\nContent-Length: 10\r\n\r\n", throttledPath)))
	require.NoError(t, err)

	// net/http discards the unread body before it replies, after the handler chain has
	// returned, so the stalled client holds neither the global nor the endpoint slot
	time.Sleep(100 * time.Millisecond)
	next := dialServer(t, srv)
	_, err = next.Write([]byte(fmt.Sprintf("GET %s HTTP/1.1\r\nHost: test\r\n\r\n", throttledPath)))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, readStatus(t, next, 5*time.Second))
}
