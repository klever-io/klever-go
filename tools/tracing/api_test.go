package tracing

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAPIStartSpan(t *testing.T) {
	// Reset and configure for testing
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	// Test when enabled
	stopFn := StartSpan("test-operation")
	assert.NotNil(t, stopFn)

	// Verify span was created
	tracer := GetConfiguredTracer()
	assert.NotNil(t, tracer)
	assert.NotNil(t, tracer.activeSpan)
	assert.Equal(t, "test-operation", tracer.activeSpan.Name)

	// Stop the span
	stopFn()

	// Test when disabled
	testResetConfig(false)
	stopFn2 := StartSpan("disabled-operation")
	assert.NotNil(t, stopFn2)
	stopFn2() // Should be no-op

	// Test with tags using variadic args
	testResetConfig(true)
	stopFn3 := StartSpan("tagged-op", "key1", "value1", "key2", "value2")
	tracer = GetConfiguredTracer()
	assert.Equal(t, "value1", tracer.activeSpan.Tags["key1"])
	assert.Equal(t, "value2", tracer.activeSpan.Tags["key2"])
	stopFn3()
}

func TestAPIStartSpanWithTags(t *testing.T) {
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	tags := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	stopFn := StartSpanWithTags("tagged-operation", tags)
	assert.NotNil(t, stopFn)

	// Verify tags were added
	tracer := GetConfiguredTracer()
	assert.NotNil(t, tracer.activeSpan)
	assert.Equal(t, "value1", tracer.activeSpan.Tags["key1"])
	assert.Equal(t, "value2", tracer.activeSpan.Tags["key2"])

	stopFn()
}

func TestAPIStartSpanf(t *testing.T) {
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	operationType := "query"
	id := 123

	stopFn := StartSpanf("operation-%s-%d", operationType, id)
	assert.NotNil(t, stopFn)

	// Verify formatted name
	tracer := GetConfiguredTracer()
	assert.NotNil(t, tracer.activeSpan)
	assert.Equal(t, "operation-query-123", tracer.activeSpan.Name)

	stopFn()
}

func TestAPIStartSpanfWithTags(t *testing.T) {
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	tags := map[string]string{
		"component": "database",
	}

	// The tags map should be the last argument
	stopFn := StartSpanfWithTags("db-%s-%d", "select", 456, tags)
	assert.NotNil(t, stopFn)

	// Verify formatted name and tags
	tracer := GetConfiguredTracer()
	assert.NotNil(t, tracer.activeSpan)
	assert.Equal(t, "db-select-456", tracer.activeSpan.Name)
	assert.Equal(t, "database", tracer.activeSpan.Tags["component"])

	stopFn()
}

func TestAPITraceIf(t *testing.T) {
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	// Test with tracing function
	var traceCalled bool
	TraceIf(func(tracer *ZipkinTracer) {
		traceCalled = true
		tracer.Start("conditional-op")
		tracer.Stop("conditional-op")
	})

	assert.True(t, traceCalled, "TraceIf should call function when enabled")

	// Test when disabled
	testResetConfig(false)
	traceCalled = false
	TraceIf(func(tracer *ZipkinTracer) {
		traceCalled = true
	})

	assert.False(t, traceCalled, "TraceIf should not call function when disabled")
}

func TestAPIAddTag(t *testing.T) {
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	stopFn := StartSpan("test-tags")

	// Add tags using the API
	AddTag("tag1", "value1")
	AddTag("tag2", "value2")

	// Verify tags were added
	tracer := GetConfiguredTracer()
	assert.Equal(t, "value1", tracer.activeSpan.Tags["tag1"])
	assert.Equal(t, "value2", tracer.activeSpan.Tags["tag2"])

	stopFn()

	// Test adding tag when no active span
	AddTag("orphan", "tag")
	// Should not crash
}

func TestAPIAddTagf(t *testing.T) {
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	stopFn := StartSpan("test-tagf")

	// Add formatted tag
	AddTagf("formatted", "value-%d", 42)

	// Verify tag was added
	tracer := GetConfiguredTracer()
	assert.Equal(t, "value-42", tracer.activeSpan.Tags["formatted"])

	stopFn()
}

func TestAPIAddAnnotation(t *testing.T) {
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	stopFn := StartSpan("test-annotation")

	// Add annotation
	AddAnnotation("checkpoint reached")

	// Verify annotation was added
	tracer := GetConfiguredTracer()
	assert.Len(t, tracer.activeSpan.Annotations, 1)
	assert.Equal(t, "checkpoint reached", tracer.activeSpan.Annotations[0].Value)

	stopFn()

	// Test adding annotation when no active span
	AddAnnotation("orphan annotation")
	// Should not crash
}

func TestAPIAddAnnotationf(t *testing.T) {
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	stopFn := StartSpan("test-annotationf")

	// Add formatted annotation
	AddAnnotationf("processed %d items", 100)

	// Verify annotation was added
	tracer := GetConfiguredTracer()
	assert.Len(t, tracer.activeSpan.Annotations, 1)
	assert.Equal(t, "processed 100 items", tracer.activeSpan.Annotations[0].Value)

	stopFn()
}

func TestAPIStopSpan(t *testing.T) {
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	StartSpan("op1")
	StartSpan("op2")
	StartSpan("op3")

	// Stop specific span
	StopSpan("op1")

	tracer := GetConfiguredTracer()

	// Find op1 and verify it has duration
	var op1Span *ZipkinSpan
	for _, span := range tracer.spans {
		if span.Name == "op1" {
			op1Span = span
			break
		}
	}

	assert.NotNil(t, op1Span)
	assert.Greater(t, op1Span.Duration, int64(0))

	// Active span should still be op3
	assert.Equal(t, "op3", tracer.activeSpan.Name)

	StopSpan("op3")
	StopSpan("op2")
}

func TestAPIStopSpanf(t *testing.T) {
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	id := 789
	StartSpanf("operation-%d", id)

	time.Sleep(time.Millisecond)

	// Stop with formatted name
	StopSpanf("operation-%d", id)

	tracer := GetConfiguredTracer()

	// Find the span and verify it has duration
	var targetSpan *ZipkinSpan
	for _, span := range tracer.spans {
		if span.Name == "operation-789" {
			targetSpan = span
			break
		}
	}

	assert.NotNil(t, targetSpan)
	assert.Greater(t, targetSpan.Duration, int64(0))
}

func TestAPIWhenDisabled(t *testing.T) {
	// Test API functions when tracing is disabled
	testResetConfig(false) // This disables tracing
	defer testResetConfig(false)

	// All API functions should return safely when tracing is disabled
	stopFn := StartSpan("test")
	assert.NotNil(t, stopFn)
	stopFn() // Should be no-op

	stopFn2 := StartSpanWithTags("test", map[string]string{"key": "value"})
	assert.NotNil(t, stopFn2)
	stopFn2() // Should be no-op

	stopFn3 := StartSpanf("test-%d", 1)
	assert.NotNil(t, stopFn3)
	stopFn3() // Should be no-op

	// These should all be no-ops when disabled
	AddTag("key", "value")
	AddTagf("key", "value-%d", 1)
	AddAnnotation("annotation")
	AddAnnotationf("annotation-%d", 1)
	StopSpan("test")
	StopSpanf("test-%d", 1)

	// TraceIf should not execute when disabled
	executed := false
	TraceIf(func(tracer *ZipkinTracer) {
		executed = true
	})
	assert.False(t, executed, "TraceIf should not execute when tracing is disabled")
}

func TestAPIWithoutFormatArgs(t *testing.T) {
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	// Test StartSpanfWithTags without format args
	stopFn := StartSpanfWithTags("simple-name")
	assert.NotNil(t, stopFn)

	tracer := GetConfiguredTracer()
	assert.Equal(t, "simple-name", tracer.activeSpan.Name)
	stopFn()

	// Test with only tags, no format args
	tags := map[string]string{"key": "value"}
	stopFn2 := StartSpanfWithTags("name-with-tags", tags)
	assert.NotNil(t, stopFn2)
	assert.Equal(t, "name-with-tags", tracer.activeSpan.Name)
	assert.Equal(t, "value", tracer.activeSpan.Tags["key"])
	stopFn2()
}

func TestAPIConcurrency(t *testing.T) {
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	var wg sync.WaitGroup

	// Start multiple goroutines using the API
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			stopFn := StartSpanf("concurrent-%d", id)
			AddTagf("goroutine", "%d", id)
			AddAnnotationf("started by goroutine %d", id)
			time.Sleep(time.Millisecond)
			stopFn()
		}(i)
	}

	// Wait for all to complete
	wg.Wait()

	// Verify spans were created
	tracer := GetConfiguredTracer()
	spanCount := 0
	for _, span := range tracer.spans {
		if span.Duration > 0 {
			spanCount++
		}
	}

	// Due to concurrent nature, we expect at least some spans
	assert.GreaterOrEqual(t, spanCount, 5)
}

func TestAPIEdgeCases(t *testing.T) {
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	// Test empty span name
	stopFn := StartSpan("")
	assert.NotNil(t, stopFn)
	stopFn()

	// Test nil tags
	stopFn2 := StartSpanWithTags("test", nil)
	assert.NotNil(t, stopFn2)
	stopFn2()

	// Test stopping non-existent span
	StopSpan("non-existent")
	// Should not crash

	// Test formatted with empty format
	stopFn3 := StartSpanf("")
	assert.NotNil(t, stopFn3)
	stopFn3()

	// Test odd number of variadic tags (should ignore last one)
	stopFn4 := StartSpan("test", "key1", "value1", "key2")
	tracer := GetConfiguredTracer()
	assert.Equal(t, "value1", tracer.activeSpan.Tags["key1"])
	assert.NotContains(t, tracer.activeSpan.Tags, "key2")
	stopFn4()
}

func TestAPIIntegration(t *testing.T) {
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	// Simulate a real-world usage pattern
	mainStop := StartSpan("http.request", "method", "GET", "path", "/api/users")

	// Database query
	dbStop := StartSpanWithTags("db.query", map[string]string{
		"db.type":      "postgres",
		"db.statement": "SELECT * FROM users",
	})
	time.Sleep(5 * time.Millisecond)
	AddAnnotation("query executed")
	dbStop()

	// Cache check
	cacheStop := StartSpanf("cache.%s", "get")
	AddTag("cache.hit", "false")
	time.Sleep(2 * time.Millisecond)
	cacheStop()

	// Process results using TraceIf
	TraceIf(func(tracer *ZipkinTracer) {
		tracer.Start("process.results")
		for i := 0; i < 3; i++ {
			tracer.AddAnnotation(fmt.Sprintf("processing item %d", i))
			time.Sleep(time.Millisecond)
		}
		tracer.Stop("process.results")
	})

	AddTag("status", "200")
	mainStop()

	// Verify the trace structure
	tracer := GetConfiguredTracer()
	assert.GreaterOrEqual(t, len(tracer.spans), 4)

	// All spans should be completed
	for _, span := range tracer.spans {
		if span.Name != "" {
			assert.Greater(t, span.Duration, int64(0),
				fmt.Sprintf("Span %s should have duration", span.Name))
		}
	}
}
