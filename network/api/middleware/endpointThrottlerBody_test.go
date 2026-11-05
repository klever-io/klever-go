package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/throttler"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const throttledPath = "/throttled"

type realThrottlerFacade struct {
	throttler core.Throttler
}

func (f *realThrottlerFacade) GetThrottlerForEndpoint(_ string) (core.Throttler, bool) {
	return f.throttler, true
}

func startThrottledServer(t *testing.T, maxNumGoRoutines int32, handler gin.HandlerFunc) *gin.Engine {
	th, err := throttler.NewNumGoRoutinesThrottler(maxNumGoRoutines)
	require.NoError(t, err)

	ws := gin.New()
	ws.Use(middleware.WithFacade(&realThrottlerFacade{throttler: th}))
	ws.POST(throttledPath, middleware.CreateEndpointThrottler(throttledPath), handler)

	return ws
}

func readBodyHandler(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	c.String(http.StatusOK, string(body))
}

// signalingReader closes started on its first Read, then reads from the wrapped reader
type signalingReader struct {
	once    sync.Once
	started chan struct{}
	reader  io.Reader
}

func (r *signalingReader) Read(p []byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	return r.reader.Read(p)
}

func TestCreateEndpointThrottler_SlowBodyHoldsNoSlot(t *testing.T) {
	t.Parallel()

	ws := startThrottledServer(t, 1, readBodyHandler)

	// the first client sends its body slowly: the middleware is still reading it
	pipeReader, pipeWriter := io.Pipe()
	slowBody := &signalingReader{started: make(chan struct{}), reader: pipeReader}
	slowReq := httptest.NewRequest(http.MethodPost, throttledPath, slowBody)
	slowResp := httptest.NewRecorder()
	slowDone := make(chan struct{})
	go func() {
		ws.ServeHTTP(slowResp, slowReq)
		close(slowDone)
	}()
	<-slowBody.started

	// another client still gets the only slot meanwhile
	req := httptest.NewRequest(http.MethodPost, throttledPath, strings.NewReader("fast"))
	resp := httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "fast", resp.Body.String())

	// the slow client finishes and its handler sees the whole body
	_, err := pipeWriter.Write([]byte("slow"))
	require.NoError(t, err)
	require.NoError(t, pipeWriter.Close())
	<-slowDone
	assert.Equal(t, http.StatusOK, slowResp.Code)
	assert.Equal(t, "slow", slowResp.Body.String())
}

func TestCreateEndpointThrottler_OverCapBodyIsRefusedWithoutASlot(t *testing.T) {
	t.Parallel()

	handlerCalled := int32(0)
	ws := startThrottledServer(t, 1, func(c *gin.Context) {
		atomic.AddInt32(&handlerCalled, 1)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, throttledPath, strings.NewReader("0123456789"))
	resp := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(resp, req.Body, 4)
	ws.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.Code)
	assert.Equal(t, int32(0), atomic.LoadInt32(&handlerCalled))

	// the refused request did not keep the slot
	req = httptest.NewRequest(http.MethodPost, throttledPath, strings.NewReader("ok"))
	resp = httptest.NewRecorder()
	ws.ServeHTTP(resp, req)
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestCreateEndpointThrottler_BurstNeverExceedsTheLimit(t *testing.T) {
	t.Parallel()

	const maxNumGoRoutines = 3
	const numRequests = 50

	inFlight := int32(0)
	maxInFlight := int32(0)
	release := make(chan struct{})
	ws := startThrottledServer(t, maxNumGoRoutines, func(c *gin.Context) {
		current := atomic.AddInt32(&inFlight, 1)
		for {
			seen := atomic.LoadInt32(&maxInFlight)
			if current <= seen || atomic.CompareAndSwapInt32(&maxInFlight, seen, current) {
				break
			}
		}
		<-release
		atomic.AddInt32(&inFlight, -1)
		c.Status(http.StatusOK)
	})

	codes := make(chan int, numRequests)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			req := httptest.NewRequest(http.MethodPost, throttledPath, strings.NewReader("{}"))
			resp := httptest.NewRecorder()
			ws.ServeHTTP(resp, req)
			codes <- resp.Code
		}()
	}
	close(start)

	// every request over the limit is refused while the admitted ones are held
	deadline := time.Now().Add(10 * time.Second)
	for len(codes) < numRequests-maxNumGoRoutines && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	close(release)
	wg.Wait()
	close(codes)

	counts := make(map[int]int)
	for code := range codes {
		counts[code]++
	}
	assert.LessOrEqual(t, atomic.LoadInt32(&maxInFlight), int32(maxNumGoRoutines))
	assert.Equal(t, maxNumGoRoutines, counts[http.StatusOK])
	assert.Equal(t, numRequests-maxNumGoRoutines, counts[http.StatusTooManyRequests])
}
