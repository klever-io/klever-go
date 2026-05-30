// Package httpserver builds the hardened *http.Server shared by both REST start
// paths (seednode and node). Gin's Engine.Run uses http.ListenAndServe with no
// ReadHeaderTimeout, leaving it open to slow-header connection exhaustion
// (GHSA-w4c6-7r69-w7j9); this helper hardens both listeners identically.
package httpserver

import (
	"net/http"
	"time"
)

const (
	// ReadHeaderTimeout is the slow-header (slowloris) mitigation: it bounds the
	// time to send the complete header. Header-only, so it is safe for the
	// long-lived websocket streams these APIs serve (cleared before hijack).
	ReadHeaderTimeout = 10 * time.Second

	// IdleTimeout bounds how long an idle keep-alive connection stays open.
	IdleTimeout = 120 * time.Second

	// MaxHeaderBytes caps request header size (Go's default, set explicitly).
	MaxHeaderBytes = 1 << 20 // 1 MiB

	// MaxBodyBytes caps the request body. A single tx is bounded by the ~960 KiB
	// P2P wire limit (~1.9 MiB once JSON-encoded), so 4 MiB covers the largest
	// legitimate request with margin; bulk /transaction/broadcast is additionally
	// bounded by an explicit tx count. Over-cap bodies are refused (400 on bind,
	// 413 raw). Bounds body size, not read time — see the body read-deadline follow-up.
	MaxBodyBytes = 4 << 20 // 4 MiB
)

// NewHardenedServer returns an *http.Server for addr serving handler, hardened
// against slow-header exhaustion and oversized bodies. ReadTimeout/WriteTimeout
// are left unset on purpose: a whole-connection deadline would sever the
// long-lived websocket streams these APIs serve (/log, /subscribe).
func NewHardenedServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           limitRequestBody(handler),
		ReadHeaderTimeout: ReadHeaderTimeout,
		IdleTimeout:       IdleTimeout,
		MaxHeaderBytes:    MaxHeaderBytes,
	}
}

// limitRequestBody caps every request body at MaxBodyBytes.
func limitRequestBody(next http.Handler) http.Handler {
	return limitRequestBodyN(next, MaxBodyBytes)
}

// limitRequestBodyN caps each request body at limit bytes. Applied ahead of gin so
// w is the *http.response MaxBytesReader needs to close an over-cap connection.
// Websocket upgrades hijack the connection and never read r.Body, so are unaffected.
func limitRequestBodyN(next http.Handler, limit int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}
