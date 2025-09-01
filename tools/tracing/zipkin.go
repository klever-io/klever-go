package tracing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ZipkinTracer provides distributed tracing functionality using the Zipkin format.
// It supports both local span collection and remote server push with circuit breaking.
type ZipkinTracer struct {
	mu         sync.RWMutex
	spans      map[string]*ZipkinSpan
	activeSpan *ZipkinSpan
	localIPv4  string

	// Server push configuration
	serverURL    string
	batchSize    int
	pushInterval time.Duration
	client       *http.Client

	// Batching
	batchMu  sync.Mutex
	batch    []*ZipkinSpan
	stopPush chan struct{}

	// Circuit breaker for network failures
	cbMu            sync.RWMutex
	cbFailures      int
	cbLastFailTime  time.Time
	cbOpen          bool
	cbRetryAttempts int       // Current retry attempt count for exponential backoff
	cbNextRetry     time.Time // Next time to attempt retry
	pushDone        chan struct{}

	// Metrics
	totalDropped int64 // Total number of spans dropped due to memory limits
}

var (
	instance *ZipkinTracer
	once     sync.Once
)

// File permissions
const (
	dirPermission  = 0755 // Permission for created directories
	filePermission = 0600 // Permission for trace files
)

// ID generation constants
const (
	maxFilenameLength = 64   // Maximum length for sanitized filenames
	timestampShift    = 16   // Bit shift for timestamp in ID generation
	counterMask       = 0xFF // Mask for counter bits
	randomMask        = 0xFF // Mask for random bits
)

// NewZipkinTracer creates a new instance of ZipkinTracer.
// The tracer starts with local storage only (no server push).
func NewZipkinTracer() *ZipkinTracer {
	return &ZipkinTracer{
		spans:        make(map[string]*ZipkinSpan),
		localIPv4:    getLocalIP(),
		batchSize:    defaultBatchSize,
		pushInterval: defaultPushInterval,
		client: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

// NewZipkinTracerWithServer creates a new ZipkinTracer configured to send spans to a server.
// It automatically starts the background push goroutine.
func NewZipkinTracerWithServer(serverURL string, batchSize int, pushInterval time.Duration) *ZipkinTracer {
	zt := &ZipkinTracer{
		spans:        make(map[string]*ZipkinSpan),
		localIPv4:    getLocalIP(),
		serverURL:    serverURL,
		batchSize:    batchSize,
		pushInterval: pushInterval,
		client: &http.Client{
			Timeout: httpTimeout,
		},
		batch:    make([]*ZipkinSpan, 0, batchSize),
		stopPush: make(chan struct{}),
		pushDone: make(chan struct{}),
	}

	if serverURL != "" {
		go zt.pushLoop()
	}

	return zt
}

// GetZipkinTracer returns the singleton instance of ZipkinTracer.
// Creates a new instance on first call.
func GetZipkinTracer() *ZipkinTracer {
	once.Do(func() {
		// Check if we should auto-configure from environment
		config := GetConfig()
		if config.Enabled && config.ServerURL != "" {
			instance = NewZipkinTracerWithServer(
				config.ServerURL,
				config.BatchSize,
				config.PushInterval,
			)
		} else {
			instance = NewZipkinTracer()
		}
	})
	return instance
}

// ResetZipkinTracer resets the singleton instance.
// This is mainly useful for testing.
func ResetZipkinTracer() {
	// Best-effort shutdown of previous pusher (if any) to avoid leaks.
	prev := instance
	if prev != nil && prev.serverURL != "" && prev.stopPush != nil {
		prev.DisableServerPush()
	}
	instance = NewZipkinTracer()
	once = sync.Once{}
}

// enforceMemoryLimit removes oldest spans if the limit is exceeded
func (z *ZipkinTracer) enforceMemoryLimit() {
	config := GetConfig()
	if config == nil || config.MaxSpans <= 0 {
		return // No limit configured
	}

	// Check if we're over the limit
	if len(z.spans) < config.MaxSpans {
		return
	}

	// Find and remove oldest spans (keeping CleanupThreshold percentage to avoid frequent cleanup)
	cleanupThreshold := config.CleanupThreshold
	if cleanupThreshold <= 0 || cleanupThreshold > 1.0 {
		cleanupThreshold = defaultCleanupRatio // Default to 90% if invalid
	}
	targetSize := int(float64(config.MaxSpans) * cleanupThreshold)
	toRemove := len(z.spans) - targetSize

	if toRemove <= 0 {
		return
	}

	// Collect span IDs with timestamps
	type spanInfo struct {
		id        string
		timestamp int64
	}
	spans := make([]spanInfo, 0, len(z.spans))
	for id, span := range z.spans {
		if span != nil && span != z.activeSpan { // Don't remove active span
			spans = append(spans, spanInfo{id: id, timestamp: span.Timestamp})
		}
	}

	// Sort by timestamp (oldest first)
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].timestamp < spans[j].timestamp
	})

	// Remove oldest spans
	droppedCount := 0
	for i := 0; i < toRemove && i < len(spans); i++ {
		delete(z.spans, spans[i].id)
		droppedCount++
	}

	// Update total dropped metric
	if droppedCount > 0 {
		z.totalDropped += int64(droppedCount)

		// Log when spans are dropped due to memory limit
		log.Warn("tracing: Dropped spans due to memory limit",
			"dropped", droppedCount,
			"total_dropped", z.totalDropped,
			"max_spans", config.MaxSpans,
			"remaining", len(z.spans))
	}
}

// startInternal is the internal implementation for creating a new span.
// It must be called with z.mu already locked.
func (z *ZipkinTracer) startInternal(identifier string, additionalTags map[string]string) {
	spanID := generateID()
	traceID := spanID // for root span
	parentID := ""

	if z.activeSpan != nil {
		// Use existing trace ID and set parent
		traceID = z.activeSpan.TraceID
		parentID = z.activeSpan.ID
	}

	// Use configured service name with thread-safe access
	serviceName := GetServiceName()

	// Extract tags using the extensible tag system
	tags := GetDefaultTags(identifier)

	// Merge additional tags if provided
	if additionalTags != nil {
		if tags == nil {
			tags = make(map[string]string)
		}
		for k, v := range additionalTags {
			tags[k] = v
		}
	}

	span := &ZipkinSpan{
		ID:        spanID,
		TraceID:   traceID,
		ParentID:  parentID,
		Name:      identifier,
		Timestamp: time.Now().UnixNano() / 1000, // microseconds
		Kind:      KindServer,
		Tags:      tags,
		LocalEndpoint: &Endpoint{
			ServiceName: serviceName,
			IPv4:        z.localIPv4,
		},
	}

	z.spans[spanID] = span
	z.activeSpan = span

	// Enforce memory limit after adding new span
	z.enforceMemoryLimit()
}

// Start begins a new span with the given identifier.
// If there's an active span, the new span becomes its child.
// The identifier is used as the span name and tags are extracted from it.
func (z *ZipkinTracer) Start(identifier string) {
	z.mu.Lock()
	defer z.mu.Unlock()

	z.startInternal(identifier, nil)
}

// Stop completes the span with the given identifier.
// It calculates the duration and returns to the parent span if any.
// If the identifier doesn't match the active span, it searches for it in the span list.
func (z *ZipkinTracer) Stop(identifier string) {
	z.mu.Lock()
	defer z.mu.Unlock()

	completedSpan := z.completeSpan(identifier)
	if completedSpan == nil {
		return
	}

	z.updateActiveSpan(completedSpan)
	z.sendToBatchIfEnabled(completedSpan)
}

// completeSpan finds and completes a span with the given identifier
func (z *ZipkinTracer) completeSpan(identifier string) *ZipkinSpan {
	// Check if it's the active span
	if z.activeSpan != nil && z.activeSpan.Name == identifier {
		z.activeSpan.Duration = (time.Now().UnixNano() / 1000) - z.activeSpan.Timestamp
		return z.activeSpan
	}

	// Find the most recent incomplete span with this identifier
	return z.findAndCompleteSpan(identifier)
}

// findAndCompleteSpan searches for and completes an incomplete span
func (z *ZipkinTracer) findAndCompleteSpan(identifier string) *ZipkinSpan {
	var candidate *ZipkinSpan

	for _, span := range z.spans {
		if span.Name != identifier || span.Duration != 0 {
			continue
		}
		if candidate == nil || span.Timestamp > candidate.Timestamp {
			candidate = span
		}
	}

	if candidate != nil {
		candidate.Duration = (time.Now().UnixNano() / 1000) - candidate.Timestamp
	}

	return candidate
}

// updateActiveSpan updates the active span after completing a span
func (z *ZipkinTracer) updateActiveSpan(completedSpan *ZipkinSpan) {
	// Only update if we completed the active span
	if z.activeSpan != completedSpan {
		return
	}

	// Set active span to parent if it exists and is incomplete
	if completedSpan.ParentID != "" {
		if parentSpan, exists := z.spans[completedSpan.ParentID]; exists && parentSpan.Duration == 0 {
			z.activeSpan = parentSpan
			return
		}
	}

	z.activeSpan = nil
}

// sendToBatchIfEnabled adds the completed span to batch if server push is enabled
func (z *ZipkinTracer) sendToBatchIfEnabled(completedSpan *ZipkinSpan) {
	if completedSpan != nil && z.serverURL != "" {
		z.addToBatch(completedSpan)
	}
}

// GetMeasurement returns the duration of a completed span with the given identifier.
// Returns 0 if the span is not found or not yet completed.
func (z *ZipkinTracer) GetMeasurement(identifier string) time.Duration {
	z.mu.RLock()
	defer z.mu.RUnlock()

	for span := range z.spans {
		if z.spans[span].Name == identifier && z.spans[span].Duration > 0 {
			return time.Duration(z.spans[span].Duration) * time.Microsecond
		}
	}
	return 0
}

// GetMetrics returns current tracer metrics
func (z *ZipkinTracer) GetMetrics() map[string]interface{} {
	z.mu.RLock()
	activeSpans := len(z.spans)
	totalDropped := z.totalDropped
	serverConfigured := z.serverURL != ""
	z.mu.RUnlock()

	z.cbMu.RLock()
	circuitOpen := z.cbOpen
	circuitFailures := z.cbFailures
	retryAttempts := z.cbRetryAttempts
	var nextRetryIn time.Duration
	if z.cbOpen && !z.cbNextRetry.IsZero() {
		nextRetryIn = max(time.Until(z.cbNextRetry), 0)
	}
	z.cbMu.RUnlock()

	metrics := map[string]interface{}{
		"active_spans":      activeSpans,
		"total_dropped":     totalDropped,
		"circuit_open":      circuitOpen,
		"circuit_failures":  circuitFailures,
		"server_configured": serverConfigured,
	}

	if circuitOpen {
		metrics["retry_attempts"] = retryAttempts
		metrics["next_retry_in_seconds"] = nextRetryIn.Seconds()
	}

	return metrics
}

// SaveSpansLog saves all collected spans to a JSON file.
// The name parameter is sanitized and used as part of the filename.
// Returns the full path of the saved file or an error if the operation fails.
func (z *ZipkinTracer) SaveSpansLog(name string) (string, error) {
	// Snapshot spans under read lock, then release before I/O.
	z.mu.RLock()
	spanSlice := make([]*ZipkinSpan, 0, len(z.spans))
	for _, span := range z.spans {
		if span != nil {
			spanSlice = append(spanSlice, span)
		}
	}
	z.mu.RUnlock()

	var filename string

	// Check if name contains a directory path
	dir := filepath.Dir(name)
	base := filepath.Base(name)

	// If name is just a filename (no directory), use current directory
	if dir == "." || dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = cwd
	}

	// Only sanitize the filename part, not the directory
	safe := sanitizeFilename(base)

	// Generate filename with timestamp
	filename = filepath.Join(dir, fmt.Sprintf("traces_%s_%s.json", safe, time.Now().Format("20060102_150405")))

	// Ensure directory exists
	if err := os.MkdirAll(dir, dirPermission); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Sort spans by start time for easier analysis
	sort.Slice(spanSlice, func(i, j int) bool { return spanSlice[i].Timestamp < spanSlice[j].Timestamp })

	jsonData, err := json.MarshalIndent(spanSlice, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal spans: %w", err)
	}
	return filename, os.WriteFile(filename, jsonData, filePermission)
}

// sanitizeFilename maps to [A-Za-z0-9_-] and trims length to maxFilenameLength.
func sanitizeFilename(s string) string {
	if s == "" {
		return "default"
	}
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, s)
	if len(s) > maxFilenameLength {
		s = s[:maxFilenameLength]
	}
	return s
}

// idCounter for generating unique IDs with timestamp + counter
var idCounter uint64

func generateID() string {
	ms := uint64(time.Now().UnixMilli())            // #nosec G115
	seq := atomic.AddUint64(&idCounter, 1) & 0xFFFF // 16 bits

	id := (ms << 16) | seq

	// Always 16 hex chars (zero-padded)
	return fmt.Sprintf("%016x", id)
}

// getLocalIP attempts to get the local IP address
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// Server push methods

func (z *ZipkinTracer) addToBatch(span *ZipkinSpan) {
	z.batchMu.Lock()
	defer z.batchMu.Unlock()

	z.batch = append(z.batch, span)

	// Flush if batch is full
	if len(z.batch) >= z.batchSize {
		z.flushBatch()
	}
}

func (z *ZipkinTracer) flushBatch() {
	if len(z.batch) == 0 {
		return
	}

	// Copy batch for sending
	toSend := make([]*ZipkinSpan, len(z.batch))
	copy(toSend, z.batch)
	z.batch = z.batch[:0]

	// Send in background
	go func() {
		err := z.sendSpans(toSend)
		if err != nil {
			log.Error("tracing: failed to send spans", "error", err)
		}
	}()
}

// isCircuitOpen checks if the circuit breaker is open
func (z *ZipkinTracer) isCircuitOpen() bool {
	z.cbMu.RLock()
	defer z.cbMu.RUnlock()

	if !z.cbOpen {
		return false
	}

	// Check if enough time has passed for next retry attempt (exponential backoff)
	if time.Now().After(z.cbNextRetry) {
		// Move to half-open state (will be handled in sendSpans)
		return false
	}

	return true
}

// recordSuccess records a successful request
func (z *ZipkinTracer) recordSuccess() {
	z.cbMu.Lock()
	defer z.cbMu.Unlock()

	z.cbFailures = 0
	z.cbOpen = false
	z.cbRetryAttempts = 0
	z.cbNextRetry = time.Time{}
}

// recordFailure records a failed request
func (z *ZipkinTracer) recordFailure() {
	z.cbMu.Lock()
	defer z.cbMu.Unlock()

	z.cbFailures++
	z.cbLastFailTime = time.Now()

	if z.cbFailures >= cbMaxFailures {
		z.cbOpen = true
		// Calculate exponential backoff for next retry
		z.cbRetryAttempts++
		backoff := z.calculateBackoff()
		z.cbNextRetry = time.Now().Add(backoff)

		log.Warn("tracing: Circuit breaker opened",
			"failures", z.cbFailures,
			"retry_in", backoff.String(),
			"attempt", z.cbRetryAttempts)
	}
}

// calculateBackoff calculates the exponential backoff duration
func (z *ZipkinTracer) calculateBackoff() time.Duration {
	backoff := cbInitialBackoff
	for i := 1; i < z.cbRetryAttempts; i++ {
		backoff *= cbBackoffMultiplier
		if backoff > cbMaxBackoff {
			backoff = cbMaxBackoff
			break
		}
	}
	return backoff
}

func (z *ZipkinTracer) sendSpans(spans []*ZipkinSpan) error {
	if z.serverURL == "" {
		return nil
	}

	// Check circuit breaker
	if z.isCircuitOpen() {
		return fmt.Errorf("circuit breaker is open, dropping %d spans", len(spans))
	}

	data, err := json.Marshal(spans)
	if err != nil {
		return fmt.Errorf("failed to marshal spans: %w", err)
	}

	return z.sendWithRetry(data)
}

// sendWithRetry handles the retry logic for sending spans
func (z *ZipkinTracer) sendWithRetry(data []byte) error {
	retryDelay := 10 * time.Millisecond // Fast retry for tests

	for attempt := range maxRetries {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<uint(attempt-1)) * retryDelay) // #nosec G115
		}

		err := z.attemptSend(data)
		if err == nil {
			z.recordSuccess()
			return nil
		}

		// Check if error is retryable
		if !z.isRetryableError(err, attempt) {
			z.recordFailure()
			return err
		}
	}

	z.recordFailure()
	return fmt.Errorf("failed to send spans after all retries")
}

// attemptSend makes a single attempt to send spans
func (z *ZipkinTracer) attemptSend(data []byte) error {
	req, err := http.NewRequest("POST", z.serverURL+"/api/v2/spans", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := z.client.Do(req)
	if err != nil {
		return err
	}

	// Always drain and close body
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode >= 400 {
		return &httpError{statusCode: resp.StatusCode}
	}

	return nil
}

// httpError represents an HTTP error response
type httpError struct {
	statusCode int
}

func (e *httpError) Error() string {
	return fmt.Sprintf("server returned status %d", e.statusCode)
}

// isRetryableError determines if an error should trigger a retry
func (z *ZipkinTracer) isRetryableError(err error, attempt int) bool {
	// Last attempt - don't retry
	if attempt == maxRetries-1 {
		return false
	}

	// Check if it's an HTTP error
	if httpErr, ok := err.(*httpError); ok {
		// Don't retry client errors (4xx)
		return httpErr.statusCode >= 500
	}

	// Network errors are retryable
	return true
}

func (z *ZipkinTracer) pushLoop() {
	ticker := time.NewTicker(z.pushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			z.batchMu.Lock()
			z.flushBatch()
			z.batchMu.Unlock()

		case <-z.stopPush:
			// Final flush (synchronous)
			z.batchMu.Lock()
			toSend := make([]*ZipkinSpan, len(z.batch))
			copy(toSend, z.batch)
			z.batch = z.batch[:0]
			z.batchMu.Unlock()
			_ = z.sendSpans(toSend)
			close(z.pushDone)
			return
		}
	}
}

// EnableServerPush configures the tracer to send spans to a Zipkin server.
// Spans are batched and sent periodically or when the batch size is reached.
// This starts a background goroutine for periodic pushing.
func (z *ZipkinTracer) EnableServerPush(serverURL string, batchSize int, pushInterval time.Duration) {
	z.mu.Lock()
	defer z.mu.Unlock()

	if z.serverURL != "" {
		// Already enabled
		return
	}

	z.serverURL = serverURL
	z.batchSize = batchSize
	z.pushInterval = pushInterval
	z.batch = make([]*ZipkinSpan, 0, batchSize)
	z.stopPush = make(chan struct{})
	z.pushDone = make(chan struct{})

	go z.pushLoop()
}

// DisableServerPush stops sending spans to the server.
// It flushes any pending spans and stops the background push goroutine.
func (z *ZipkinTracer) DisableServerPush() {
	z.mu.Lock()
	defer z.mu.Unlock()

	if z.serverURL == "" {
		return
	}

	close(z.stopPush)
	<-z.pushDone
	z.serverURL = ""
}

// Flush immediately sends all pending spans to the server.
// This is a no-op if server push is not enabled.
func (z *ZipkinTracer) Flush() {
	if z.serverURL == "" {
		return
	}

	z.batchMu.Lock()
	defer z.batchMu.Unlock()
	z.flushBatch()
}

// HasServerPush returns true if server push is enabled
func (z *ZipkinTracer) HasServerPush() bool {
	z.mu.RLock()
	defer z.mu.RUnlock()
	return z.serverURL != ""
}

// AddTag adds a key-value tag to the currently active span.
// Tags provide additional metadata about the span.
func (z *ZipkinTracer) AddTag(key, value string) {
	z.mu.Lock()
	defer z.mu.Unlock()

	if z.activeSpan != nil {
		if z.activeSpan.Tags == nil {
			z.activeSpan.Tags = make(map[string]string)
		}
		z.activeSpan.Tags[key] = value
	}
}

// AddAnnotation adds a timestamped annotation to the currently active span.
// Annotations mark specific events within a span's lifetime.
func (z *ZipkinTracer) AddAnnotation(value string) {
	z.mu.Lock()
	defer z.mu.Unlock()

	if z.activeSpan != nil {
		annotation := Annotation{
			Timestamp: time.Now().UnixNano() / 1000,
			Value:     value,
		}
		z.activeSpan.Annotations = append(z.activeSpan.Annotations, annotation)
	}
}

// StartWithTags begins a new span with the given identifier and tags.
// This is equivalent to calling Start followed by multiple AddTag calls, but atomic.
func (z *ZipkinTracer) StartWithTags(identifier string, tags map[string]string) {
	z.mu.Lock()
	defer z.mu.Unlock()

	z.startInternal(identifier, tags)
}
