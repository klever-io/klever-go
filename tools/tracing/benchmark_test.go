package tracing

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestMain provides global setup and cleanup for all benchmark tests
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()

	// Global cleanup after all tests
	cleanupAllTestFiles()

	os.Exit(code)
}

// cleanupAllTestFiles performs global cleanup after all tests
func cleanupAllTestFiles() {
	// Clean up any trace JSON files in the current directory
	entries, err := os.ReadDir(".")
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		// Only remove JSON trace files, not test files!
		if !entry.IsDir() && strings.HasSuffix(name, ".json") &&
			(strings.HasPrefix(name, "traces_") ||
				strings.Contains(name, "_trace_")) {
			os.Remove(name)
		}
	}

	// Also clean up /tmp directories created by tests
	os.RemoveAll("/tmp/test_traces")
	os.RemoveAll("/tmp/test-traces-2024")
	os.RemoveAll("/tmp/test-traces")
}

// setupBenchmarkTracer sets up a tracer optimized for benchmarking
// It increases memory limits to avoid span dropping during benchmarks
func setupBenchmarkTracer(enabled bool) *ZipkinTracer {
	// Force reset the sync.Once by recreating it
	configOnce = sync.Once{}
	// Set a config with very high limits for benchmarking
	globalConfig = &Config{
		Enabled:          enabled,
		ServerURL:        "",
		BatchSize:        100,
		PushInterval:     5 * time.Second,
		ServiceName:      "BenchmarkService",
		SaveOnExit:       false,
		SavePath:         "./test_traces",
		MaxSpans:         10000000, // 10 million - effectively unlimited for benchmarks
		CleanupThreshold: 0.99,     // Only cleanup at 99% to minimize interference
	}
	// Reset the configured tracer
	configuredTracer = nil
	ResetZipkinTracer()
	return GetZipkinTracer()
}

// BenchmarkStartStop measures the performance of starting and stopping a single span
func BenchmarkStartStop(b *testing.B) {
	// Reset and initialize tracer for benchmarking
	tracer := setupBenchmarkTracer(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracer.Start("test.operation")
		tracer.Stop("test.operation")
	}
}

// BenchmarkStartStopEnabled measures performance when tracing is enabled
func BenchmarkStartStopEnabled(b *testing.B) {
	// Reset and initialize tracer with tracing enabled
	tracer := setupBenchmarkTracer(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracer.Start("test.operation")
		tracer.Stop("test.operation")
	}
}

// BenchmarkStartStopWithTags measures performance with tags
func BenchmarkStartStopWithTags(b *testing.B) {
	tracer := setupBenchmarkTracer(true)

	tags := map[string]string{
		"service": "test",
		"version": "1.0.0",
		"env":     "benchmark",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracer.StartWithTags("test.operation", tags)
		tracer.Stop("test.operation")
	}
}

// BenchmarkNestedSpans measures performance with nested spans
func BenchmarkNestedSpans(b *testing.B) {
	tracer := setupBenchmarkTracer(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracer.Start("parent.operation")
		tracer.Start("child.operation.1")
		tracer.Stop("child.operation.1")
		tracer.Start("child.operation.2")
		tracer.Stop("child.operation.2")
		tracer.Stop("parent.operation")
	}
}

// BenchmarkAPIStartSpan measures the performance of the API StartSpan function
func BenchmarkAPIStartSpan(b *testing.B) {
	testResetConfig(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stop := StartSpan("test.operation")
		stop()
	}
}

// BenchmarkAPIStartSpanDisabled measures overhead when tracing is disabled
func BenchmarkAPIStartSpanDisabled(b *testing.B) {
	testResetConfig(false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stop := StartSpan("test.operation")
		stop()
	}
}

// BenchmarkAPIStartSpanWithTags measures API performance with tags
func BenchmarkAPIStartSpanWithTags(b *testing.B) {
	testResetConfig(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stop := StartSpan("test.operation", "key1", "value1", "key2", "value2")
		stop()
	}
}

// BenchmarkAddTag measures the performance of adding tags
func BenchmarkAddTag(b *testing.B) {
	testResetConfig(true)
	tracer := GetZipkinTracer()
	tracer.Start("test.operation")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracer.AddTag("key", "value")
	}
	b.StopTimer()

	tracer.Stop("test.operation")
}

// BenchmarkAddAnnotation measures the performance of adding annotations
func BenchmarkAddAnnotation(b *testing.B) {
	testResetConfig(true)
	tracer := GetZipkinTracer()
	tracer.Start("test.operation")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracer.AddAnnotation("test annotation")
	}
	b.StopTimer()

	tracer.Stop("test.operation")
}

// BenchmarkMemoryLimit measures performance impact of memory limiting
func BenchmarkMemoryLimit(b *testing.B) {
	// Use a small limit to trigger cleanup frequently
	configOnce = sync.Once{}
	globalConfig = &Config{
		Enabled:          true,
		ServerURL:        "",
		BatchSize:        100,
		PushInterval:     5 * time.Second,
		ServiceName:      "BenchmarkService",
		SaveOnExit:       false,
		SavePath:         "./test_traces",
		MaxSpans:         100, // Small limit to trigger cleanup
		CleanupThreshold: 0.9,
	}
	configuredTracer = nil
	ResetZipkinTracer()

	tracer := GetZipkinTracer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		name := fmt.Sprintf("operation.%d", i)
		tracer.Start(name)
		tracer.Stop(name)
	}
}

// BenchmarkConcurrentSpans measures performance under concurrent load
func BenchmarkConcurrentSpans(b *testing.B) {
	setupBenchmarkTracer(true)

	b.RunParallel(func(pb *testing.PB) {
		tracer := GetZipkinTracer()
		i := 0
		for pb.Next() {
			name := fmt.Sprintf("concurrent.operation.%d", i)
			tracer.Start(name)
			tracer.Stop(name)
			i++
		}
	})
}

// BenchmarkGetMeasurement measures performance of retrieving measurements
func BenchmarkGetMeasurement(b *testing.B) {
	tracer := setupBenchmarkTracer(true)

	// Create some spans
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("operation.%d", i)
		tracer.Start(name)
		tracer.Stop(name)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tracer.GetMeasurement("operation.50")
	}
}

// BenchmarkSaveSpansLog measures performance of saving spans to file
func BenchmarkSaveSpansLog(b *testing.B) {
	testResetConfig(true)
	tracer := GetZipkinTracer()

	// Create some spans
	for i := 0; i < 1000; i++ {
		name := fmt.Sprintf("operation.%d", i)
		tracer.Start(name)
		tracer.Stop(name)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tracer.SaveSpansLog(fmt.Sprintf("benchmark_%d", i))
	}
	b.StopTimer()

	// Cleanup
	// Note: In a real scenario, you might want to clean up the created files
}

// BenchmarkCircuitBreaker measures circuit breaker overhead
func BenchmarkCircuitBreaker(b *testing.B) {
	testResetConfig(true)
	tracer := GetZipkinTracer()

	// Enable server push to test circuit breaker
	tracer.EnableServerPush("http://localhost:9411", 100, 5*time.Second)

	// Simulate circuit breaker open state
	tracer.cbOpen = true
	tracer.cbFailures = 5

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tracer.isCircuitOpen()
	}

	b.StopTimer()
	tracer.DisableServerPush()
}

// BenchmarkTagExtraction measures performance of tag extraction
func BenchmarkTagExtraction(b *testing.B) {
	testResetConfig(true)

	identifiers := []string{
		"consensus.slot.42",
		"consensus.subslot.StartSlot",
		"consensus.block.createHeader",
		"network.send.message",
		"storage.write.block",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetDefaultTags(identifiers[i%len(identifiers)])
	}
}

// BenchmarkStartSpanf measures formatted span name performance
func BenchmarkStartSpanf(b *testing.B) {
	testResetConfig(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stop := StartSpanf("operation.%d.%s", i, "benchmark")
		stop()
	}
}

// BenchmarkAPIWithDisabledTracing measures the overhead of disabled tracing
func BenchmarkAPIWithDisabledTracing(b *testing.B) {
	testResetConfig(false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// These should all be no-ops when tracing is disabled
		stop := StartSpan("operation")
		AddTag("key", "value")
		AddAnnotation("annotation")
		stop()
	}
}

// === Comparison Benchmarks: Disabled vs No Tracing Code ===

// BenchmarkDisabledOverhead_Simple measures overhead of disabled tracing vs no code
func BenchmarkDisabledOverhead_Simple(b *testing.B) {
	testResetConfig(false) // Tracing disabled

	b.Run("WithTracingCode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			stop := StartSpan("operation")
			stop()
		}
	})

	b.Run("NoTracingCode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Baseline: no tracing code at all
			_ = "operation" // Just to prevent optimization
		}
	})
}

// BenchmarkDisabledOverhead_Complex measures overhead in a more realistic scenario
func BenchmarkDisabledOverhead_Complex(b *testing.B) {
	testResetConfig(false) // Tracing disabled

	b.Run("WithTracingCode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Simulate a function with multiple trace points
			stop1 := StartSpan("main.operation")

			stop2 := StartSpan("sub.operation1")
			AddTag("iteration", fmt.Sprintf("%d", i))
			stop2()

			stop3 := StartSpan("sub.operation2")
			AddAnnotation("processing")
			stop3()

			stop1()
		}
	})

	b.Run("NoTracingCode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Baseline: same operations without tracing
			_ = "main.operation"
			_ = "sub.operation1"
			_ = fmt.Sprintf("%d", i)
			_ = "sub.operation2"
			_ = "processing"
		}
	})
}

// BenchmarkDisabledOverhead_Formatted measures overhead of formatted span names
func BenchmarkDisabledOverhead_Formatted(b *testing.B) {
	testResetConfig(false) // Tracing disabled

	b.Run("WithTracingCode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			stop := StartSpanf("operation.%d.%s", i, "test")
			stop()
		}
	})

	b.Run("NoTracingCode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Baseline: just format the string
			_ = fmt.Sprintf("operation.%d.%s", i, "test")
		}
	})
}

// BenchmarkDisabledOverhead_Nested measures overhead of nested spans
func BenchmarkDisabledOverhead_Nested(b *testing.B) {
	testResetConfig(false) // Tracing disabled

	b.Run("WithTracingCode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			stop1 := StartSpan("parent")
			stop2 := StartSpan("child1")
			stop3 := StartSpan("grandchild")
			stop3()
			stop2()
			stop4 := StartSpan("child2")
			stop4()
			stop1()
		}
	})

	b.Run("NoTracingCode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Baseline: no operations
			_ = "parent"
			_ = "child1"
			_ = "grandchild"
			_ = "child2"
		}
	})
}

// BenchmarkEnabledVsDisabled compares the same operations with tracing on/off
func BenchmarkEnabledVsDisabled(b *testing.B) {
	b.Run("Disabled", func(b *testing.B) {
		testResetConfig(false)
		for i := 0; i < b.N; i++ {
			stop := StartSpan("operation")
			AddTag("key", "value")
			AddAnnotation("test")
			stop()
		}
	})

	b.Run("Enabled", func(b *testing.B) {
		testResetConfig(true)
		for i := 0; i < b.N; i++ {
			stop := StartSpan("operation")
			AddTag("key", "value")
			AddAnnotation("test")
			stop()
		}
	})
}

// BenchmarkRealWorldScenario simulates a realistic consensus operation
func BenchmarkRealWorldScenario(b *testing.B) {
	b.Run("TracingDisabled", func(b *testing.B) {
		testResetConfig(false)
		for i := 0; i < b.N; i++ {
			// Simulate consensus slot processing
			stopSlot := StartSpan("consensus.slot.42")

			stopSubslot := StartSpan("consensus.subslot.StartSlot")
			AddTag("slot.index", "42")
			AddTag("node.role", "proposer")
			stopSubslot()

			stopBlock := StartSpan("consensus.block.create")
			AddTag("block.height", "1000")
			AddAnnotation("Creating block")
			stopBlock()

			stopSig := StartSpan("consensus.signature.broadcast")
			stopSig()

			stopEnd := StartSpan("consensus.endSlot.commit")
			AddAnnotation("Committing block")
			stopEnd()

			stopSlot()
		}
	})

	b.Run("TracingEnabled", func(b *testing.B) {
		testResetConfig(true)
		for i := 0; i < b.N; i++ {
			// Same consensus slot processing
			stopSlot := StartSpan("consensus.slot.42")

			stopSubslot := StartSpan("consensus.subslot.StartSlot")
			AddTag("slot.index", "42")
			AddTag("node.role", "proposer")
			stopSubslot()

			stopBlock := StartSpan("consensus.block.create")
			AddTag("block.height", "1000")
			AddAnnotation("Creating block")
			stopBlock()

			stopSig := StartSpan("consensus.signature.broadcast")
			stopSig()

			stopEnd := StartSpan("consensus.endSlot.commit")
			AddAnnotation("Committing block")
			stopEnd()

			stopSlot()
		}
	})

	b.Run("NoTracingCode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Baseline: just the string operations
			_ = "consensus.slot.42"
			_ = "consensus.subslot.StartSlot"
			_ = "42"
			_ = "proposer"
			_ = "consensus.block.create"
			_ = "1000"
			_ = "Creating block"
			_ = "consensus.signature.broadcast"
			_ = "consensus.endSlot.commit"
			_ = "Committing block"
		}
	})
}
