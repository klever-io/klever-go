package tracing

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func setupTestEnvironment() func() {
	// Reset all global state BEFORE the test
	testResetConfig(false)

	// Save and restore environment
	originalEnv := os.Environ()

	// Clear env first
	os.Clearenv()

	// Reset config state to force reload from environment
	configMu.Lock()
	globalConfig = nil
	configOnce = sync.Once{}
	configuredTracer = nil
	configMu.Unlock()

	// Also reset the singleton tracer
	ResetZipkinTracer()

	return func() {
		os.Clearenv()
		for _, env := range originalEnv {
			if len(env) > 0 {
				pair := strings.SplitN(env, "=", 2)
				if len(pair) == 2 {
					os.Setenv(pair[0], pair[1])
				}
			}
		}
		// Reset globals after test
		testResetConfig(false)
	}
}

func setTestEnvironmentVariables() {
	os.Setenv("KLEVER_TRACING_ENABLED", "true")
	os.Setenv("KLEVER_TRACING_SERVICE_NAME", "test-service")
	os.Setenv("KLEVER_TRACING_BATCH_SIZE", "50")
	os.Setenv("KLEVER_TRACING_PUSH_INTERVAL", "2s")
	os.Setenv("KLEVER_TRACING_SAVE_ON_EXIT", "true")
	os.Setenv("KLEVER_TRACING_SAVE_PATH", "/tmp/test-traces")
}

func verifyConfiguration(t *testing.T) {
	config := GetConfig()
	if !config.Enabled {
		t.Error("Tracing should be enabled")
	}
	if config.ServiceName != "test-service" {
		t.Errorf("Expected service name 'test-service', got %s", config.ServiceName)
	}
	if config.BatchSize != 50 {
		t.Errorf("Expected batch size 50, got %d", config.BatchSize)
	}
}

func simulateConsensusOperations(tracer *ZipkinTracer) {
	tracer.Start("test.consensus.slot")

	tracer.Start("test.consensus.block")
	time.Sleep(time.Millisecond)
	tracer.Stop("test.consensus.block")

	tracer.Start("test.consensus.signature")
	time.Sleep(time.Millisecond)
	tracer.Stop("test.consensus.signature")

	tracer.Stop("test.consensus.slot")
}

func verifyMeasurements(t *testing.T, tracer *ZipkinTracer) {
	slotDuration := tracer.GetMeasurement("test.consensus.slot")
	if slotDuration <= 0 {
		t.Error("Slot duration should be positive")
	}

	blockDuration := tracer.GetMeasurement("test.consensus.block")
	if blockDuration <= 0 {
		t.Error("Block duration should be positive")
	}
}

func TestIntegrationWithEnvironmentConfig(t *testing.T) {
	cleanup := setupTestEnvironment()
	defer cleanup()

	setTestEnvironmentVariables()

	// Initialize tracing
	MustInitialize()

	verifyConfiguration(t)

	// Get the configured tracer
	tracer := GetConfiguredTracer()
	if tracer == nil {
		t.Fatal("Failed to get configured tracer")
	}

	simulateConsensusOperations(tracer)
	verifyMeasurements(t, tracer)

	// Verify singleton behavior - GetConfiguredTracer should return same instance
	tracer2 := GetConfiguredTracer()
	if tracer != tracer2 {
		t.Error("Should return the same configured tracer instance")
	}

	// Test shutdown (won't actually save to /tmp in test)
	err := Shutdown()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestConsensusWithAutoConfig(t *testing.T) {
	// Reset all global state BEFORE the test
	testResetConfig(false)

	// Save and restore environment
	originalEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, env := range originalEnv {
			if len(env) > 0 {
				pair := strings.SplitN(env, "=", 2)
				if len(pair) == 2 {
					os.Setenv(pair[0], pair[1])
				}
			}
		}
		// Reset globals after test
		testResetConfig(false)
	}()

	// Clear env first
	os.Clearenv()

	// Enable tracing
	os.Setenv("KLEVER_TRACING_ENABLED", "true")
	os.Setenv("KLEVER_TRACING_SERVICE_NAME", "consensus-test")

	// The consensus code will automatically use the configured tracer
	tracer := GetZipkinTracer() // This will auto-configure from env

	// Simulate consensus subslot operations
	// These would normally be called by the actual consensus code
	simulateConsensusSubslot(tracer)

	// Verify all consensus operations were traced
	expectedOps := []string{
		"consensus.subslot.StartSlot",
		"consensus.subslot.Block",
		"consensus.subslot.Signature",
		"consensus.subslot.EndSlot",
	}

	for _, op := range expectedOps {
		duration := tracer.GetMeasurement(op)
		if duration <= 0 {
			t.Errorf("Expected positive duration for %s, got %v", op, duration)
		}
	}
}

func simulateConsensusSubslot(tracer *ZipkinTracer) {
	// StartSlot
	tracer.Start("consensus.subslot.StartSlot")
	tracer.Start("consensus.startSlot.resetState")
	time.Sleep(time.Millisecond)
	tracer.Stop("consensus.startSlot.resetState")
	time.Sleep(time.Microsecond)
	tracer.Stop("consensus.subslot.StartSlot")

	// Block phase
	tracer.Start("consensus.subslot.Block")
	tracer.Start("consensus.block.createHeader")
	time.Sleep(time.Millisecond)
	tracer.Stop("consensus.block.createHeader")
	tracer.Start("consensus.block.createBlock")
	time.Sleep(2 * time.Millisecond)
	tracer.Stop("consensus.block.createBlock")
	tracer.Stop("consensus.subslot.Block")

	// Signature phase
	tracer.Start("consensus.subslot.Signature")
	tracer.Start("consensus.signature.createSignatureShare")
	time.Sleep(time.Millisecond)
	tracer.Stop("consensus.signature.createSignatureShare")
	tracer.Stop("consensus.subslot.Signature")

	// EndSlot
	tracer.Start("consensus.subslot.EndSlot")
	tracer.Start("consensus.endSlot.commitBlock")
	time.Sleep(3 * time.Millisecond)
	tracer.Stop("consensus.endSlot.commitBlock")
	tracer.Stop("consensus.subslot.EndSlot")
}
