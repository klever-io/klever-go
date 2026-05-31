// Package httpserver builds the hardened *http.Server shared by both REST start paths
// (seednode and node). Gin's Engine.Run uses http.ListenAndServe with no
// ReadHeaderTimeout, leaving it open to slow-header exhaustion (GHSA-w4c6-7r69-w7j9).
package httpserver

import (
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	// ReadHeaderTimeout bounds header read time — the slow-header (slowloris) fix.
	ReadHeaderTimeout = 10 * time.Second

	// BodyReadTimeout bounds body read time — the slow-body fix.
	BodyReadTimeout = 30 * time.Second

	// IdleTimeout bounds idle keep-alive connections.
	IdleTimeout = 120 * time.Second

	// MaxHeaderBytes caps total request header size (Go's 1 MiB default is a no-op).
	MaxHeaderBytes = 32 << 10 // 32 KiB

	// MaxBodyBytes caps the request body; over-cap bodies are refused (400/413).
	MaxBodyBytes = 2 << 20 // 2 MiB
)

// NewHardenedServer returns an *http.Server hardened against slow-header/slow-body
// exhaustion. ReadTimeout/WriteTimeout are left unset: a whole-connection deadline
// would sever the long-lived websocket streams these APIs serve (/log, /subscribe).
func NewHardenedServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           guardRequestBody(handler),
		ReadHeaderTimeout: ReadHeaderTimeout,
		IdleTimeout:       IdleTimeout,
		MaxHeaderBytes:    MaxHeaderBytes,
	}
}

// guardRequestBody caps body size (MaxBodyBytes) and body read time (BodyReadTimeout).
// Applied ahead of gin so w is the *http.response both mechanisms need.
func guardRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capRequestBody(w, r, MaxBodyBytes)
		applyBodyReadDeadline(w, r, BodyReadTimeout)
		next.ServeHTTP(w, r)
	})
}

// capRequestBody limits r.Body to limit bytes.
func capRequestBody(w http.ResponseWriter, r *http.Request, limit int64) {
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
	}
}

// applyBodyReadDeadline bounds body read time at d, clearing the deadline once the body
// is read so it never cancels a slow post-read handler. Bodiless requests are skipped —
// that covers the websocket handshakes (/log, /subscribe), which are bodiless GETs, and
// leaves no Upgrade-header check for a body-bearing request to spoof.
func applyBodyReadDeadline(w http.ResponseWriter, r *http.Request, d time.Duration) {
	if r.Body == nil || !requestHasBody(r) {
		return
	}
	rc := http.NewResponseController(w)
	if rc.SetReadDeadline(time.Now().Add(d)) != nil {
		return
	}
	r.Body = &deadlineClearingBody{ReadCloser: r.Body, rc: rc}
}

func requestHasBody(r *http.Request) bool {
	return r.ContentLength != 0 // 0 = no body; -1 (chunked) and positive = body
}

// deadlineClearingBody clears the read deadline once the body read completes (EOF or
// error) or the body is closed, so the deadline never cancels a later slow handler.
type deadlineClearingBody struct {
	io.ReadCloser
	rc        *http.ResponseController
	clearOnce sync.Once
}

func (b *deadlineClearingBody) clear() {
	b.clearOnce.Do(func() { _ = b.rc.SetReadDeadline(time.Time{}) })
}

func (b *deadlineClearingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		b.clear()
	}
	return n, err
}

func (b *deadlineClearingBody) Close() error {
	b.clear()
	return b.ReadCloser.Close()
}
