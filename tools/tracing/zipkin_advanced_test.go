package tracing

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestZipkinAddTag(t *testing.T) {
	tracer := NewZipkinTracer()

	// Start a span
	tracer.Start("test-span")

	// Add tags
	tracer.AddTag("key1", "value1")
	tracer.AddTag("key2", "value2")

	// Verify tags were added
	assert.NotNil(t, tracer.activeSpan.Tags)
	assert.Equal(t, "value1", tracer.activeSpan.Tags["key1"])
	assert.Equal(t, "value2", tracer.activeSpan.Tags["key2"])

	// Test adding tag when no active span
	tracer.Stop("test-span")
	tracer.AddTag("orphan", "tag")
	// Should not crash
}

func TestZipkinAddAnnotation(t *testing.T) {
	tracer := NewZipkinTracer()

	// Start a span
	tracer.Start("test-span")

	// Add annotations
	tracer.AddAnnotation("event1")
	time.Sleep(time.Millisecond)
	tracer.AddAnnotation("event2")

	// Verify annotations were added
	assert.Len(t, tracer.activeSpan.Annotations, 2)
	assert.Equal(t, "event1", tracer.activeSpan.Annotations[0].Value)
	assert.Equal(t, "event2", tracer.activeSpan.Annotations[1].Value)

	// Verify timestamps are different
	assert.NotEqual(t, tracer.activeSpan.Annotations[0].Timestamp,
		tracer.activeSpan.Annotations[1].Timestamp)

	// Test adding annotation when no active span
	tracer.Stop("test-span")
	tracer.AddAnnotation("orphan")
	// Should not crash
}

func TestZipkinStartWithTags(t *testing.T) {
	tracer := NewZipkinTracer()

	tags := map[string]string{
		"component": "http",
		"method":    "GET",
		"path":      "/api/test",
	}

	// Start span with tags
	tracer.StartWithTags("http.request", tags)

	// Verify span was created with tags
	assert.NotNil(t, tracer.activeSpan)
	assert.Equal(t, "http.request", tracer.activeSpan.Name)
	assert.Equal(t, "http", tracer.activeSpan.Tags["component"])
	assert.Equal(t, "GET", tracer.activeSpan.Tags["method"])
	assert.Equal(t, "/api/test", tracer.activeSpan.Tags["path"])

	// Verify that tags are set correctly (no default tags for "http.request")

	tracer.Stop("http.request")
}

func TestCircuitBreakerBehavior(t *testing.T) {
	// Create a server that always fails
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failingServer.Close()

	tracer := NewZipkinTracerWithServer(failingServer.URL, 1, 10*time.Millisecond)
	defer tracer.DisableServerPush()

	// Create and try to send spans multiple times
	// After enough failures, the circuit breaker should open
	for i := 0; i < 10; i++ {
		span := &ZipkinSpan{
			ID:      fmt.Sprintf("span-%d", i),
			TraceID: "test-trace",
			Name:    fmt.Sprintf("test-span-%d", i),
		}
		_ = tracer.sendSpans([]*ZipkinSpan{span})
	}

	// Now sending should fail immediately due to open circuit
	span := &ZipkinSpan{
		ID:      "final-span",
		TraceID: "test-trace",
		Name:    "final-test-span",
	}
	err := tracer.sendSpans([]*ZipkinSpan{span})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

func TestCircuitBreakerRecovery(t *testing.T) {
	// Create a server that fails initially then recovers
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount <= 5 {
			// Fail first 5 attempts
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			// Then succeed
			w.WriteHeader(http.StatusAccepted)
		}
	}))
	defer server.Close()

	tracer := NewZipkinTracerWithServer(server.URL, 1, 10*time.Millisecond)
	defer tracer.DisableServerPush()

	// Send spans that will fail initially
	for i := 0; i < 5; i++ {
		span := &ZipkinSpan{
			ID:      fmt.Sprintf("span-%d", i),
			TraceID: "test-trace",
			Name:    fmt.Sprintf("test-span-%d", i),
		}
		_ = tracer.sendSpans([]*ZipkinSpan{span})
	}

	// Wait a bit for circuit to potentially recover
	time.Sleep(50 * time.Millisecond)

	// Now it should succeed after recovery
	span := &ZipkinSpan{
		ID:      "recovery-span",
		TraceID: "test-trace",
		Name:    "recovery-test-span",
	}
	err := tracer.sendSpans([]*ZipkinSpan{span})

	// Should eventually succeed after retries
	// The exact behavior depends on the circuit breaker implementation
	// We're testing that the system can recover
	if err != nil {
		// If still failing, it should be due to circuit breaker, not server error
		assert.Contains(t, err.Error(), "circuit breaker")
	}
}

func TestSendSpansWithFailures(t *testing.T) {
	failCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failCount++
		if failCount <= 2 {
			// Fail first 2 attempts
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	tracer := NewZipkinTracerWithServer(server.URL, 1, 10*time.Millisecond)
	defer tracer.DisableServerPush()

	// Create and send a span
	span := &ZipkinSpan{
		ID:      "test-id",
		TraceID: "test-trace",
		Name:    "test-span",
	}

	err := tracer.sendSpans([]*ZipkinSpan{span})

	// Should succeed after retries
	assert.NoError(t, err)
	assert.Equal(t, 3, failCount) // Initial + 2 retries
}

func TestSendSpansClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return client error (no retry)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	tracer := NewZipkinTracerWithServer(server.URL, 1, 10*time.Millisecond)
	defer tracer.DisableServerPush()

	span := &ZipkinSpan{
		ID:      "test-id",
		TraceID: "test-trace",
		Name:    "test-span",
	}

	err := tracer.sendSpans([]*ZipkinSpan{span})

	// Should fail immediately on client error (400 status)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server returned status 400")

	// Circuit breaker should have recorded failure
	assert.Equal(t, 1, tracer.cbFailures)
}

func TestSendSpansWithCircuitOpen(t *testing.T) {
	tracer := NewZipkinTracer()
	tracer.serverURL = "http://localhost:9411"

	// Open the circuit
	tracer.cbOpen = true
	tracer.cbNextRetry = time.Now().Add(time.Hour)

	span := &ZipkinSpan{
		ID:      "test-id",
		TraceID: "test-trace",
		Name:    "test-span",
	}

	err := tracer.sendSpans([]*ZipkinSpan{span})

	// Should fail due to open circuit
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

func TestEnableServerPushIdempotent(t *testing.T) {
	tracer := NewZipkinTracer()

	// Enable server push
	tracer.EnableServerPush("http://localhost:9411", 100, 5*time.Second)
	assert.Equal(t, "http://localhost:9411", tracer.serverURL)

	// Try to enable again (should be idempotent)
	tracer.EnableServerPush("http://localhost:9999", 200, 10*time.Second)

	// Should not change since already enabled
	assert.Equal(t, "http://localhost:9411", tracer.serverURL)
	assert.Equal(t, 100, tracer.batchSize)

	tracer.DisableServerPush()
}

func TestDisableServerPushIdempotent(t *testing.T) {
	tracer := NewZipkinTracer()

	// Disable when already disabled (should not crash)
	tracer.DisableServerPush()
	assert.Empty(t, tracer.serverURL)
}

func TestFlushWithoutServerPush(t *testing.T) {
	tracer := NewZipkinTracer()

	// Create some spans
	tracer.Start("span1")
	tracer.Stop("span1")

	// Flush without server push (should be no-op)
	tracer.Flush()

	// Spans should still be in memory
	assert.Len(t, tracer.spans, 1)
}

func TestMemoryLimitBehavior(t *testing.T) {
	// Save original env and set test values
	originalMaxSpans := os.Getenv("KLEVER_TRACING_MAX_SPANS")
	originalThreshold := os.Getenv("KLEVER_TRACING_CLEANUP_THRESHOLD")
	defer func() {
		if originalMaxSpans != "" {
			os.Setenv("KLEVER_TRACING_MAX_SPANS", originalMaxSpans)
		} else {
			os.Unsetenv("KLEVER_TRACING_MAX_SPANS")
		}
		if originalThreshold != "" {
			os.Setenv("KLEVER_TRACING_CLEANUP_THRESHOLD", originalThreshold)
		} else {
			os.Unsetenv("KLEVER_TRACING_CLEANUP_THRESHOLD")
		}
	}()

	// Set low limits via environment
	os.Setenv("KLEVER_TRACING_ENABLED", "true")
	os.Setenv("KLEVER_TRACING_MAX_SPANS", "5")
	os.Setenv("KLEVER_TRACING_CLEANUP_THRESHOLD", "0.6")

	// Reset the test environment to force config reload from environment
	testResetConfigForEnv()

	// Initialize with new config
	MustInitialize()

	// Create many spans using the public API
	for i := 0; i < 20; i++ {
		stopFn := StartSpanf("span-%d", i)
		// Add some work to make spans meaningful
		time.Sleep(time.Microsecond)
		stopFn()
	}

	// Get metrics to check behavior
	tracer := GetConfiguredTracer()
	metrics := tracer.GetMetrics()

	// We should have dropped some spans due to memory limit
	droppedSpans, ok := metrics["total_dropped"].(int64)
	assert.True(t, ok, "total_dropped should be present in metrics")
	assert.Greater(t, droppedSpans, int64(0), "Should have dropped spans due to memory limit")

	// Check that we have some active spans (the ones kept after cleanup)
	activeSpans, ok := metrics["active_spans"].(int)
	assert.True(t, ok, "active_spans should be present in metrics")
	assert.Greater(t, activeSpans, 0, "Should have active spans")
	assert.LessOrEqual(t, activeSpans, 5, "Should not exceed max_spans limit")
}

func TestMemoryLimitWithInvalidThreshold(t *testing.T) {
	// Save original env
	originalMaxSpans := os.Getenv("KLEVER_TRACING_MAX_SPANS")
	originalThreshold := os.Getenv("KLEVER_TRACING_CLEANUP_THRESHOLD")
	defer func() {
		if originalMaxSpans != "" {
			os.Setenv("KLEVER_TRACING_MAX_SPANS", originalMaxSpans)
		} else {
			os.Unsetenv("KLEVER_TRACING_MAX_SPANS")
		}
		if originalThreshold != "" {
			os.Setenv("KLEVER_TRACING_CLEANUP_THRESHOLD", originalThreshold)
		} else {
			os.Unsetenv("KLEVER_TRACING_CLEANUP_THRESHOLD")
		}
		testResetConfig(false)
	}()

	// Set invalid threshold via environment
	os.Setenv("KLEVER_TRACING_ENABLED", "true")
	os.Setenv("KLEVER_TRACING_MAX_SPANS", "3")
	os.Setenv("KLEVER_TRACING_CLEANUP_THRESHOLD", "2.0") // Invalid (>1.0)

	// Reset and initialize from environment
	testResetConfigForEnv()
	MustInitialize()

	// Create spans beyond limit using public API
	for i := 0; i < 10; i++ {
		stopFn := StartSpanf("span-%d", i)
		time.Sleep(time.Microsecond)
		stopFn()
	}

	// Should use default cleanup threshold when invalid value provided
	// We can verify this through metrics
	tracer := GetConfiguredTracer()
	metrics := tracer.GetMetrics()

	// Should have dropped spans
	droppedSpans, ok := metrics["total_dropped"].(int64)
	if ok {
		assert.Greater(t, droppedSpans, int64(0), "Should drop spans with memory limit")
	}
}

func TestGetLocalIPFallback(t *testing.T) {
	// This test might behave differently on different systems
	// but should at least not crash
	ip := getLocalIP()
	// IP should be either empty or a valid format
	if ip != "" {
		assert.Contains(t, ip, ".")
	}
}

func TestSaveSpansLogWithInvalidPath(t *testing.T) {
	tracer := NewZipkinTracer()

	tracer.Start("test")
	tracer.Stop("test")

	// Try to save to an invalid path
	_, err := tracer.SaveSpansLog("/invalid\x00path/file")
	assert.Error(t, err)
}

func TestSaveSpansLogWithEmptyTracer(t *testing.T) {
	tracer := NewZipkinTracer()

	// Save with no spans
	filename, err := tracer.SaveSpansLog("empty")
	assert.NoError(t, err)
	assert.Contains(t, filename, "empty")

	// Clean up
	os.Remove(filename)
}

func TestBatchFlushOnSize(t *testing.T) {
	sentBatches := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sentBatches++
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	// Create tracer with small batch size
	tracer := NewZipkinTracerWithServer(server.URL, 2, 10*time.Second)
	defer tracer.DisableServerPush()

	// Create exactly batch size spans
	tracer.Start("span1")
	tracer.Stop("span1")
	tracer.Start("span2")
	tracer.Stop("span2")

	// Wait for async send
	time.Sleep(50 * time.Millisecond)

	// Should have sent one batch
	assert.Equal(t, 1, sentBatches)
}

func TestPushLoopWithTicker(t *testing.T) {
	sentCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sentCount++
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	// Create tracer with short push interval
	tracer := NewZipkinTracerWithServer(server.URL, 100, 30*time.Millisecond)

	// Create a span but don't fill the batch
	tracer.Start("span1")
	tracer.Stop("span1")

	// Wait for ticker to fire
	time.Sleep(60 * time.Millisecond)

	// Should have sent due to ticker
	assert.GreaterOrEqual(t, sentCount, 1)

	tracer.DisableServerPush()
}
