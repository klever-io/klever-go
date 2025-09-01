package tracing

import (
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

func TestLoadFromEnvEdgeCases(t *testing.T) {
	// Save original env
	originalEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, e := range originalEnv {
			pair := strings.SplitN(e, "=", 2)
			if len(pair) == 2 {
				os.Setenv(pair[0], pair[1])
			}
		}
	}()

	os.Clearenv()

	// Test with invalid boolean
	os.Setenv("KLEVER_TRACING_ENABLED", "not-a-bool")
	config := DefaultConfig()
	config.LoadFromEnv()
	assert.False(t, config.Enabled) // Should default to false on error

	// Test with invalid batch size
	os.Setenv("KLEVER_TRACING_BATCH_SIZE", "not-a-number")
	config = DefaultConfig()
	config.LoadFromEnv()
	assert.Equal(t, 100, config.BatchSize) // Should keep default

	// Test with invalid push interval
	os.Setenv("KLEVER_TRACING_PUSH_INTERVAL", "invalid")
	config = DefaultConfig()
	config.LoadFromEnv()
	assert.Equal(t, 5*time.Second, config.PushInterval) // Should keep default

	// Test with invalid max spans
	os.Setenv("KLEVER_TRACING_MAX_SPANS", "-100")
	config = DefaultConfig()
	config.LoadFromEnv()
	assert.Equal(t, 10000, config.MaxSpans) // Should keep default

	// Test with invalid cleanup threshold
	os.Setenv("KLEVER_TRACING_CLEANUP_THRESHOLD", "not-a-float")
	config = DefaultConfig()
	config.LoadFromEnv()
	assert.Equal(t, 0.9, config.CleanupThreshold) // Should keep default
}

func TestMustInitializeWhenDisabled(t *testing.T) {
	// Save original env
	originalEnabled := os.Getenv("KLEVER_TRACING_ENABLED")
	defer func() {
		if originalEnabled != "" {
			os.Setenv("KLEVER_TRACING_ENABLED", originalEnabled)
		} else {
			os.Unsetenv("KLEVER_TRACING_ENABLED")
		}
		testResetConfig(false)
	}()

	// Disable tracing via environment
	os.Setenv("KLEVER_TRACING_ENABLED", "false")

	// Reset test state properly
	testResetConfig(false)
	ResetZipkinTracer()

	// Should not panic even when disabled
	MustInitialize()

	// Verify tracing is disabled
	assert.False(t, IsEnabled(), "Tracing should be disabled")

	// Tracer should be initialized but without server push
	tracer := GetConfiguredTracer()
	assert.NotNil(t, tracer, "Should have a tracer instance even when disabled")
	assert.False(t, tracer.HasServerPush(), "Should not have server push when disabled")
}

func TestMustInitializeWhenEnabled(t *testing.T) {
	// Save original env
	originalEnabled := os.Getenv("KLEVER_TRACING_ENABLED")
	defer func() {
		if originalEnabled != "" {
			os.Setenv("KLEVER_TRACING_ENABLED", originalEnabled)
		} else {
			os.Unsetenv("KLEVER_TRACING_ENABLED")
		}
		testResetConfig(false)
	}()

	// Enable tracing via environment
	os.Setenv("KLEVER_TRACING_ENABLED", "true")

	// Reset test state to force config reload from environment
	testResetConfigForEnv()

	// Initialize tracing
	MustInitialize()

	// Verify tracing is enabled
	assert.True(t, IsEnabled(), "Tracing should be enabled")

	// Tracer should be initialized
	tracer := GetConfiguredTracer()
	assert.NotNil(t, tracer, "Should have a tracer instance")
}

func TestInitializeFromConfigWithServerURL(t *testing.T) {
	// Create test server to handle Zipkin API requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/spans" && r.Method == "POST" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Reset state
	testResetConfig(false)
	defer testResetConfig(false)

	// Set config with test server URL
	globalConfig = &Config{
		Enabled:      true,
		ServerURL:    server.URL,
		BatchSize:    50,
		PushInterval: 2 * time.Second,
		ServiceName:  "test-service",
	}

	tracer := InitializeFromConfig()
	assert.NotNil(t, tracer)
	assert.Equal(t, server.URL, tracer.serverURL)
	assert.Equal(t, 50, tracer.batchSize)
	assert.Equal(t, 2*time.Second, tracer.pushInterval)

	// Clean up
	tracer.DisableServerPush()
}

func TestGetConfiguredTracerConcurrency(t *testing.T) {
	// Reset state
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	var wg sync.WaitGroup
	tracers := make([]*ZipkinTracer, 10)

	// Multiple goroutines calling GetConfiguredTracer
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tracers[idx] = GetConfiguredTracer()
		}(i)
	}

	wg.Wait()

	// All should get the same instance
	first := tracers[0]
	for i := 1; i < 10; i++ {
		assert.Equal(t, first, tracers[i], "All goroutines should get same tracer instance")
	}
}

func TestShutdownWithSaveOnExit(t *testing.T) {
	// Reset state
	testResetConfig(false)
	defer testResetConfig(false)

	// Create temp directory for traces
	tempDir := t.TempDir()

	// Configure with SaveOnExit
	globalConfig = &Config{
		Enabled:    true,
		SaveOnExit: true,
		SavePath:   tempDir,
	}

	// Initialize and create some spans
	tracer := InitializeFromConfig()
	tracer.Start("test-span-1")
	time.Sleep(time.Millisecond)
	tracer.Stop("test-span-1")

	tracer.Start("test-span-2")
	time.Sleep(time.Millisecond)
	tracer.Stop("test-span-2")

	// Shutdown should save traces
	err := Shutdown()
	assert.NoError(t, err)

	// Check that a trace file was created
	files, err := os.ReadDir(tempDir)
	assert.NoError(t, err)
	assert.Greater(t, len(files), 0, "Trace file should be created")

	// Verify file contains traces
	if len(files) > 0 {
		assert.Contains(t, files[0].Name(), "trace_")
		assert.Contains(t, files[0].Name(), ".json")
	}
}

func TestShutdownWithServerPush(t *testing.T) {
	// Create test server to handle Zipkin API requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/spans" && r.Method == "POST" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Save original env
	originalEnabled := os.Getenv("KLEVER_TRACING_ENABLED")
	originalURL := os.Getenv("KLEVER_TRACING_SERVER_URL")
	defer func() {
		if originalEnabled != "" {
			os.Setenv("KLEVER_TRACING_ENABLED", originalEnabled)
		} else {
			os.Unsetenv("KLEVER_TRACING_ENABLED")
		}
		if originalURL != "" {
			os.Setenv("KLEVER_TRACING_SERVER_URL", originalURL)
		} else {
			os.Unsetenv("KLEVER_TRACING_SERVER_URL")
		}
		testResetConfig(false)
	}()

	// Set up environment with test server URL
	os.Setenv("KLEVER_TRACING_ENABLED", "true")
	os.Setenv("KLEVER_TRACING_SERVER_URL", server.URL)

	// Reset and initialize from environment
	testResetConfigForEnv()
	MustInitialize()

	// Create some spans using public API
	stopFn := StartSpan("test-span")
	stopFn()

	// Get the tracer to verify it has server push
	tracer := GetConfiguredTracer()
	assert.True(t, tracer.HasServerPush(), "Should have server push enabled")

	// Shutdown should flush and disable server push
	err := Shutdown()
	assert.NoError(t, err)

	// After shutdown, getting a new tracer should not have the old state
	// Note: We can't check if configuredTracer is nil as it's private
	// But we can verify behavior - after shutdown, a new initialization would be needed
}

func TestUpdateServiceNameConcurrency(t *testing.T) {
	// Reset state
	testResetConfig(true)
	defer testResetConfig(false)

	// Initialize tracer from config
	InitializeFromConfig()

	var wg sync.WaitGroup

	// Multiple goroutines updating service name
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			UpdateServiceName(fmt.Sprintf("service-%d", id))
		}(i)
	}

	wg.Wait()

	// Service name should be set to one of the values
	config := GetConfig()
	assert.Contains(t, config.ServiceName, "service-")
}

func TestGetServiceNameFallback(t *testing.T) {
	// Reset state
	testResetConfig(false)
	defer testResetConfig(false)

	// Clear service name
	globalConfig = &Config{
		ServiceName: "",
	}

	// Should return default
	name := GetServiceName()
	assert.Equal(t, "KleverGo", name)

	// With nil config
	globalConfig = nil
	name = GetServiceName()
	assert.Equal(t, "KleverGo", name)
}

func TestGenerateUniqueServiceNameConcurrency(t *testing.T) {
	var wg sync.WaitGroup
	names := make([]string, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			names[idx] = generateUniqueServiceName()
		}(i)
	}

	wg.Wait()

	// All should have the KleverGo prefix
	for _, name := range names {
		assert.Contains(t, name, "KleverGo")
	}
}

func TestServiceNameUniquenessStress(t *testing.T) {
	// The function generates a unique name per process, not per call
	// So we expect the same name for all calls in the same process
	const iterations = 100
	var firstName string

	for i := 0; i < iterations; i++ {
		name := generateUniqueServiceName()

		// Should contain required parts
		assert.Contains(t, name, "KleverGo")

		// All calls should return the same name in the same process
		if i == 0 {
			firstName = name
		} else {
			assert.Equal(t, firstName, name, "Service name should be consistent within a process")
		}
	}
}

func TestConfigStringFormattingEdgeCases(t *testing.T) {
	// Test with various config values
	config := &Config{
		Enabled:          true,
		ServerURL:        "https://very-long-server-url-that-might-be-truncated.example.com:9411/api/v2/spans",
		BatchSize:        999999,
		PushInterval:     123456789 * time.Nanosecond,
		ServiceName:      "very-long-service-name-with-special-chars-!@#$%^&*()",
		SaveOnExit:       true,
		SavePath:         "/very/long/path/to/save/directory/that/might/not/exist",
		MaxSpans:         999999999,
		CleanupThreshold: 0.123456789,
	}

	str := config.String()

	// Should contain all fields
	assert.Contains(t, str, "Enabled:true")
	assert.Contains(t, str, "ServerURL:https://")
	assert.Contains(t, str, "BatchSize:999999")
	assert.Contains(t, str, "ServiceName:very-long-service-name")
	assert.Contains(t, str, "SaveOnExit:true")
	assert.Contains(t, str, "MaxSpans:999999999")

	// Should be formatted correctly
	assert.True(t, strings.HasPrefix(str, "TracingConfig{"))
	assert.True(t, strings.HasSuffix(str, "}"))
}

func TestTestResetConfigIdempotent(t *testing.T) {
	// Multiple calls should not panic
	testResetConfig(true)
	testResetConfig(true)
	testResetConfig(false)
	testResetConfig(false)

	// Config should be in expected state
	config := GetConfig()
	assert.False(t, config.Enabled)
}
