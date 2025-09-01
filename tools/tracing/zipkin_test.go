package tracing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewZipkinTracer(t *testing.T) {
	tracer := NewZipkinTracer()
	if tracer == nil {
		t.Fatal("NewZipkinTracer returned nil")
	}
	if tracer.spans == nil {
		t.Fatal("spans map not initialized")
	}
	if tracer.batchSize != 100 {
		t.Errorf("expected default batchSize to be 100, got %d", tracer.batchSize)
	}
	if tracer.pushInterval != 5*time.Second {
		t.Errorf("expected default pushInterval to be 5s, got %v", tracer.pushInterval)
	}
}

func TestNewZipkinTracerWithServer(t *testing.T) {
	serverURL := "http://localhost:9411"
	batchSize := 50
	pushInterval := 10 * time.Second

	tracer := NewZipkinTracerWithServer(serverURL, batchSize, pushInterval)
	if tracer == nil {
		t.Fatal("NewZipkinTracerWithServer returned nil")
	}
	if tracer.serverURL != serverURL {
		t.Errorf("expected serverURL %s, got %s", serverURL, tracer.serverURL)
	}
	if tracer.batchSize != batchSize {
		t.Errorf("expected batchSize %d, got %d", batchSize, tracer.batchSize)
	}
	if tracer.pushInterval != pushInterval {
		t.Errorf("expected pushInterval %v, got %v", pushInterval, tracer.pushInterval)
	}

	// Cleanup
	tracer.DisableServerPush()
}

func TestGetZipkinTracer(t *testing.T) {
	// Reset the singleton first
	ResetZipkinTracer()

	tracer1 := GetZipkinTracer()
	tracer2 := GetZipkinTracer()

	if tracer1 != tracer2 {
		t.Error("GetZipkinTracer should return the same instance")
	}
}

func TestResetZipkinTracer(t *testing.T) {
	tracer1 := GetZipkinTracer()
	ResetZipkinTracer()
	tracer2 := GetZipkinTracer()

	if tracer1 == tracer2 {
		t.Error("ResetZipkinTracer should create a new instance")
	}
}

func TestStartStop(t *testing.T) {
	tracer := NewZipkinTracer()

	// Test root span
	tracer.Start("operation1")
	if tracer.activeSpan == nil {
		t.Fatal("activeSpan should not be nil after Start")
	}
	if tracer.activeSpan.Name != "operation1" {
		t.Errorf("expected span name 'operation1', got %s", tracer.activeSpan.Name)
	}
	if tracer.activeSpan.ParentID != "" {
		t.Error("root span should have empty ParentID")
	}

	rootSpanID := tracer.activeSpan.ID
	rootTraceID := tracer.activeSpan.TraceID

	// Test child span
	tracer.Start("operation2")
	if tracer.activeSpan.ParentID != rootSpanID {
		t.Errorf("child span should have ParentID %s, got %s", rootSpanID, tracer.activeSpan.ParentID)
	}
	if tracer.activeSpan.TraceID != rootTraceID {
		t.Errorf("child span should have same TraceID as root: %s, got %s", rootTraceID, tracer.activeSpan.TraceID)
	}

	// Add a small delay to ensure duration is measurable
	time.Sleep(time.Millisecond)

	// Stop child span
	tracer.Stop("operation2")
	if tracer.activeSpan == nil || tracer.activeSpan.Name != "operation1" {
		t.Error("activeSpan should return to parent after Stop")
	}

	// Stop root span
	tracer.Stop("operation1")
	if tracer.activeSpan != nil {
		t.Error("activeSpan should be nil after stopping root span")
	}

	// Verify both spans have duration
	for _, span := range tracer.spans {
		if span.Duration <= 0 {
			t.Errorf("span %s should have positive duration, got %d", span.Name, span.Duration)
		}
	}
}

func TestNestedSpans(t *testing.T) {
	tracer := NewZipkinTracer()

	tracer.Start("root")
	rootID := tracer.activeSpan.ID

	tracer.Start("child1")
	child1ID := tracer.activeSpan.ID

	tracer.Start("grandchild")
	grandchildID := tracer.activeSpan.ID

	// Verify parent relationships
	if tracer.spans[child1ID].ParentID != rootID {
		t.Error("child1 should have root as parent")
	}
	if tracer.spans[grandchildID].ParentID != child1ID {
		t.Error("grandchild should have child1 as parent")
	}

	// Stop in reverse order
	tracer.Stop("grandchild")
	if tracer.activeSpan.ID != child1ID {
		t.Error("activeSpan should be child1 after stopping grandchild")
	}

	tracer.Stop("child1")
	if tracer.activeSpan.ID != rootID {
		t.Error("activeSpan should be root after stopping child1")
	}

	tracer.Stop("root")
	if tracer.activeSpan != nil {
		t.Error("activeSpan should be nil after stopping root")
	}
}

func TestGetMeasurement(t *testing.T) {
	tracer := NewZipkinTracer()

	tracer.Start("operation1")
	time.Sleep(10 * time.Millisecond)
	tracer.Stop("operation1")

	duration := tracer.GetMeasurement("operation1")
	if duration <= 0 {
		t.Error("GetMeasurement should return positive duration for completed span")
	}
	if duration < 10*time.Millisecond {
		t.Errorf("duration should be at least 10ms, got %v", duration)
	}

	// Test non-existent span
	nonExistent := tracer.GetMeasurement("nonexistent")
	if nonExistent != 0 {
		t.Error("GetMeasurement should return 0 for non-existent span")
	}

	// Test incomplete span
	tracer.Start("incomplete")
	incomplete := tracer.GetMeasurement("incomplete")
	if incomplete != 0 {
		t.Error("GetMeasurement should return 0 for incomplete span")
	}
}

func TestSaveSpansLog(t *testing.T) {
	tracer := NewZipkinTracer()

	tracer.Start("test-operation")
	time.Sleep(5 * time.Millisecond)
	tracer.Stop("test-operation")

	filename, err := tracer.SaveSpansLog("test_trace")
	if err != nil {
		t.Fatalf("SaveSpansLog failed: %v", err)
	}
	defer os.Remove(filename)

	// Verify file was created
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Fatal("trace file was not created")
	}

	// Verify file contains valid JSON
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("failed to read trace file: %v", err)
	}

	var spans []*ZipkinSpan
	if err := json.Unmarshal(data, &spans); err != nil {
		t.Fatalf("failed to unmarshal trace file: %v", err)
	}

	if len(spans) != 1 {
		t.Errorf("expected 1 span, got %d", len(spans))
	}

	if spans[0].Name != "test-operation" {
		t.Errorf("expected span name 'test-operation', got %s", spans[0].Name)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "default"},
		{"valid-name_123", "valid-name_123"},
		{"invalid/name\\with:special*chars", "invalid_name_with_special_chars"},
		{strings.Repeat("a", 100), strings.Repeat("a", 64)},
		{"Test@#$%Name", "Test____Name"},
	}

	for _, tt := range tests {
		result := sanitizeFilename(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == "" {
		t.Error("generateID should not return empty string")
	}
	if id1 == id2 {
		t.Error("generateID should generate unique IDs")
	}
	if len(id1) != 16 {
		t.Errorf("expected ID length 16, got %d", len(id1))
	}
}

func TestServerPush(t *testing.T) {
	receivedSpans := make([]*ZipkinSpan, 0)
	var mu sync.Mutex

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/spans" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var spans []*ZipkinSpan
		if err := json.NewDecoder(r.Body).Decode(&spans); err != nil {
			t.Errorf("failed to decode spans: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		mu.Lock()
		receivedSpans = append(receivedSpans, spans...)
		mu.Unlock()

		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	// Create tracer with server
	tracer := NewZipkinTracerWithServer(server.URL, 2, 50*time.Millisecond)
	defer tracer.DisableServerPush()

	// Create some spans
	tracer.Start("op1")
	tracer.Stop("op1")

	tracer.Start("op2")
	tracer.Stop("op2")

	// Wait for batch to be sent (batch size is 2)
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(receivedSpans) != 2 {
		t.Errorf("expected 2 spans to be sent, got %d", len(receivedSpans))
	}
}

func TestFlush(t *testing.T) {
	receivedCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var spans []*ZipkinSpan
		json.NewDecoder(r.Body).Decode(&spans)
		mu.Lock()
		receivedCount += len(spans)
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	tracer := NewZipkinTracerWithServer(server.URL, 100, 10*time.Second)
	defer tracer.DisableServerPush()

	// Create a span
	tracer.Start("op1")
	tracer.Stop("op1")

	// Flush immediately
	tracer.Flush()

	// Give time for the async send
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if receivedCount != 1 {
		t.Errorf("expected 1 span after flush, got %d", receivedCount)
	}
}

func TestEnableDisableServerPush(t *testing.T) {
	tracer := NewZipkinTracer()

	// Initially no server push
	if tracer.serverURL != "" {
		t.Error("serverURL should be empty initially")
	}

	// Enable server push
	tracer.EnableServerPush("http://localhost:9411", 50, 1*time.Second)
	if tracer.serverURL != "http://localhost:9411" {
		t.Error("serverURL should be set after EnableServerPush")
	}

	// Disable server push
	tracer.DisableServerPush()
	if tracer.serverURL != "" {
		t.Error("serverURL should be empty after DisableServerPush")
	}
}

func TestRetryLogic(t *testing.T) {
	attemptCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attemptCount++
		current := attemptCount
		mu.Unlock()

		if current < 3 {
			// Fail first 2 attempts
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	tracer := NewZipkinTracerWithServer(server.URL, 1, 10*time.Millisecond)
	defer tracer.DisableServerPush()

	tracer.Start("op1")
	tracer.Stop("op1")

	// Wait for retries (10ms + 20ms + 40ms = 70ms, wait 100ms to be safe)
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if attemptCount < 3 {
		t.Errorf("expected at least 3 attempts due to retry logic, got %d", attemptCount)
	}
}

func TestConcurrentAccess(t *testing.T) {
	tracer := NewZipkinTracer()
	var wg sync.WaitGroup

	// Start multiple goroutines creating spans
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Add a small random delay to reduce contention at Start
			time.Sleep(time.Duration(id) * 100 * time.Microsecond)
			spanName := fmt.Sprintf("span-%d", id)
			tracer.Start(spanName)
			time.Sleep(time.Millisecond)
			tracer.Stop(spanName)
		}(i)
	}

	wg.Wait()

	// Count completed spans
	completedCount := 0
	totalSpans := 0
	incompleteSpans := []string{}
	for _, span := range tracer.spans {
		totalSpans++
		if span.Duration > 0 {
			completedCount++
		} else {
			incompleteSpans = append(incompleteSpans, span.Name)
		}
	}

	fmt.Printf("Completed spans: %d, Incomplete spans: %d (%v)\n", completedCount, len(incompleteSpans), incompleteSpans)
	assert.Equal(t, 10, totalSpans, "should have 10 total spans")
}

func TestStopNonActiveSpan(t *testing.T) {
	tracer := NewZipkinTracer()

	// Start multiple spans
	tracer.Start("span1")
	tracer.Start("span2")
	tracer.Start("span3")

	// Stop a non-active span (span1)
	tracer.Stop("span1")

	// Verify span1 has duration
	var span1 *ZipkinSpan
	for _, s := range tracer.spans {
		if s.Name == "span1" {
			span1 = s
			break
		}
	}

	if span1 == nil {
		t.Fatal("span1 not found")
	}
	if span1.Duration <= 0 {
		t.Error("span1 should have positive duration even when stopped as non-active")
	}

	// Active span should still be span3
	if tracer.activeSpan == nil || tracer.activeSpan.Name != "span3" {
		t.Error("activeSpan should still be span3")
	}
}
