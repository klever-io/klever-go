package tracing

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, false, config.Enabled, "Default Enabled should be false")
	assert.Equal(t, "", config.ServerURL, "Default ServerURL should be empty")
	assert.Equal(t, 100, config.BatchSize, "Default BatchSize should be 100")
	assert.Equal(t, 5*time.Second, config.PushInterval, "Default PushInterval should be 5s")
	assert.Equal(t, "KleverGo", config.ServiceName, "Default ServiceName should be 'KleverGo'")
	assert.Equal(t, false, config.SaveOnExit, "Default config should have SaveOnExit=false")
	assert.Equal(t, "./traces", config.SavePath, "Default SavePath should be './traces'")
	assert.Equal(t, 10000, config.MaxSpans, "Default MaxSpans should be 10000")
}

func TestLoadFromEnv(t *testing.T) {
	// Save original env vars
	originalVars := map[string]string{
		"KLEVER_TRACING_ENABLED":       os.Getenv("KLEVER_TRACING_ENABLED"),
		"KLEVER_TRACING_SERVER_URL":    os.Getenv("KLEVER_TRACING_SERVER_URL"),
		"KLEVER_TRACING_BATCH_SIZE":    os.Getenv("KLEVER_TRACING_BATCH_SIZE"),
		"KLEVER_TRACING_PUSH_INTERVAL": os.Getenv("KLEVER_TRACING_PUSH_INTERVAL"),
		"KLEVER_TRACING_SERVICE_NAME":  os.Getenv("KLEVER_TRACING_SERVICE_NAME"),
		"KLEVER_TRACING_SAVE_ON_EXIT":  os.Getenv("KLEVER_TRACING_SAVE_ON_EXIT"),
		"KLEVER_TRACING_SAVE_PATH":     os.Getenv("KLEVER_TRACING_SAVE_PATH"),
		"KLEVER_TRACING_MAX_SPANS":     os.Getenv("KLEVER_TRACING_MAX_SPANS"),
	}

	// Restore env vars after test
	defer func() {
		for key, val := range originalVars {
			if val == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, val)
			}
		}
	}()

	// Set test environment variables
	os.Setenv("KLEVER_TRACING_ENABLED", "true")
	os.Setenv("KLEVER_TRACING_SERVER_URL", "http://test:9411")
	os.Setenv("KLEVER_TRACING_BATCH_SIZE", "50")
	os.Setenv("KLEVER_TRACING_PUSH_INTERVAL", "10s")
	os.Setenv("KLEVER_TRACING_SERVICE_NAME", "TestService")
	os.Setenv("KLEVER_TRACING_SAVE_ON_EXIT", "1")
	os.Setenv("KLEVER_TRACING_SAVE_PATH", "/tmp/traces")
	os.Setenv("KLEVER_TRACING_MAX_SPANS", "5000")

	config := DefaultConfig()
	config.LoadFromEnv()

	assert.Equal(t, true, config.Enabled, "Config should be enabled")
	assert.Equal(t, "http://test:9411", config.ServerURL, "ServerURL should be 'http://test:9411'")
	assert.Equal(t, 50, config.BatchSize, "BatchSize should be 50")
	assert.Equal(t, 10*time.Second, config.PushInterval, "PushInterval should be 10s")
	assert.Equal(t, "TestService", config.ServiceName, "ServiceName should be 'TestService'")
	assert.Equal(t, true, config.SaveOnExit, "SaveOnExit should be true")
	assert.Equal(t, "/tmp/traces", config.SavePath, "SavePath should be '/tmp/traces'")
	assert.Equal(t, 5000, config.MaxSpans, "MaxSpans should be 5000")
}

func TestConfigString(t *testing.T) {
	config := &Config{
		Enabled:      true,
		ServerURL:    "http://localhost:9411",
		BatchSize:    100,
		PushInterval: 5 * time.Second,
		ServiceName:  "TestService",
		SaveOnExit:   true,
		SavePath:     "./traces",
		MaxSpans:     10000,
	}

	str := config.String()
	if str == "" {
		t.Error("Config string should not be empty")
	}

	// Check that key values are in the string
	expectedSubstrings := []string{
		"Enabled:true",
		"ServerURL:http://localhost:9411",
		"BatchSize:100",
		"ServiceName:TestService",
	}

	for _, substr := range expectedSubstrings {
		if !contains(str, substr) {
			t.Errorf("Config string should contain '%s', got: %s", substr, str)
		}
	}
}

func TestInitializeFromConfig(t *testing.T) {
	// Reset global state before test
	testResetConfig(false)
	defer testResetConfig(false) // Clean up after test
	// Reset global state
	globalConfig = nil
	configOnce = sync.Once{}
	configuredTracer = nil
	ResetZipkinTracer()

	// Save original env
	originalEnabled := os.Getenv("KLEVER_TRACING_ENABLED")
	defer func() {
		if originalEnabled == "" {
			os.Unsetenv("KLEVER_TRACING_ENABLED")
		} else {
			os.Setenv("KLEVER_TRACING_ENABLED", originalEnabled)
		}
		// Reset globals
		globalConfig = nil
		configOnce = sync.Once{}
		configuredTracer = nil
		ResetZipkinTracer()
	}()

	// Set up test environment
	os.Setenv("KLEVER_TRACING_ENABLED", "true")

	tracer := InitializeFromConfig()
	if tracer == nil {
		t.Fatal("InitializeFromConfig should return a tracer")
	}

	// Second call should return the same instance
	tracer2 := InitializeFromConfig()
	if tracer != tracer2 {
		t.Error("InitializeFromConfig should return the same instance")
	}

	// GetConfiguredTracer should also return the same instance
	tracer3 := GetConfiguredTracer()
	if tracer != tracer3 {
		t.Error("GetConfiguredTracer should return the same instance")
	}
}

func TestIsEnabled(t *testing.T) {
	// Reset global state before test
	testResetConfig(false)
	defer testResetConfig(false) // Clean up after test
	// Save original env
	originalEnabled := os.Getenv("KLEVER_TRACING_ENABLED")
	defer func() {
		if originalEnabled == "" {
			os.Unsetenv("KLEVER_TRACING_ENABLED")
		} else {
			os.Setenv("KLEVER_TRACING_ENABLED", originalEnabled)
		}
		// Reset globals
		globalConfig = nil
		configOnce = sync.Once{}
	}()

	// Reset global state
	globalConfig = nil
	configOnce = sync.Once{}

	// Test with disabled
	os.Setenv("KLEVER_TRACING_ENABLED", "false")

	if IsEnabled() {
		t.Error("IsEnabled should return false when KLEVER_TRACING_ENABLED=false")
	}

	// Reset and test with enabled
	globalConfig = nil
	configOnce = sync.Once{}
	os.Setenv("KLEVER_TRACING_ENABLED", "true")

	if !IsEnabled() {
		t.Error("IsEnabled should return true when KLEVER_TRACING_ENABLED=true")
	}
}

func TestShutdown(t *testing.T) {
	// Reset global state before test
	testResetConfig(false)
	defer testResetConfig(false) // Clean up after test
	// Save original env
	originalEnabled := os.Getenv("KLEVER_TRACING_ENABLED")
	defer func() {
		if originalEnabled == "" {
			os.Unsetenv("KLEVER_TRACING_ENABLED")
		} else {
			os.Setenv("KLEVER_TRACING_ENABLED", originalEnabled)
		}
		// Reset globals
		globalConfig = nil
		configOnce = sync.Once{}
		configuredTracer = nil
		ResetZipkinTracer()
	}()

	// Reset global state
	globalConfig = nil
	configOnce = sync.Once{}
	configuredTracer = nil
	ResetZipkinTracer()

	// Initialize a tracer
	os.Setenv("KLEVER_TRACING_ENABLED", "true")

	tracer := InitializeFromConfig()
	if tracer == nil {
		t.Fatal("Failed to initialize tracer")
	}

	// Add some test spans
	tracer.Start("test-op")
	tracer.Stop("test-op")

	// Shutdown should succeed
	err := Shutdown()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// After shutdown, configuredTracer should be nil
	if configuredTracer != nil {
		t.Error("configuredTracer should be nil after shutdown")
	}
}

// Helper function
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestGenerateUniqueServiceName(t *testing.T) {
	// Test default generation
	name1 := generateUniqueServiceName()
	assert.Contains(t, name1, "KleverGo", "Service name should contain KleverGo")

	// Test with INSTANCE_ID environment variable
	os.Setenv("INSTANCE_ID", "test-instance-123")
	defer os.Unsetenv("INSTANCE_ID")

	name2 := generateUniqueServiceName()
	assert.Equal(t, "KleverGo-test-instance-123", name2, "Service name should use INSTANCE_ID when available")

	// Clear INSTANCE_ID and test NODE_NAME
	os.Unsetenv("INSTANCE_ID")
	os.Setenv("NODE_NAME", "node-456")
	defer os.Unsetenv("NODE_NAME")

	name3 := generateUniqueServiceName()
	assert.Contains(t, name3, "node-456", "Service name should contain NODE_NAME")

	// Ensure different processes get different names (when no env vars set)
	os.Unsetenv("NODE_NAME")
	os.Unsetenv("INSTANCE_ID")
	name4 := generateUniqueServiceName()
	pid := os.Getpid()
	assert.Contains(t, name4, fmt.Sprintf("%d", pid), "Service name should contain PID for uniqueness")
}

func TestServiceNameUniqueness(t *testing.T) {
	// Save and restore env
	originalServiceName := os.Getenv("KLEVER_TRACING_SERVICE_NAME")
	defer func() {
		if originalServiceName == "" {
			os.Unsetenv("KLEVER_TRACING_SERVICE_NAME")
		} else {
			os.Setenv("KLEVER_TRACING_SERVICE_NAME", originalServiceName)
		}
	}()

	// Test that when no service name is provided, a unique one is generated
	os.Unsetenv("KLEVER_TRACING_SERVICE_NAME")
	config := DefaultConfig()
	config.LoadFromEnv()

	// Should have a generated name with hostname and PID
	assert.NotEqual(t, "KleverGo", config.ServiceName, "Service name should be unique when not specified")
	assert.Contains(t, config.ServiceName, "KleverGo", "Generated name should contain base service name")
}
