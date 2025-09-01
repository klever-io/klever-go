package tracing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveSpansLogWithDirectory(t *testing.T) {
	tracer := NewZipkinTracer()

	// Create some test spans
	tracer.Start("test.operation")
	time.Sleep(time.Millisecond)
	tracer.Stop("test.operation")

	// Test saving with directory path
	testDir := "/tmp/test_traces"
	testPath := filepath.Join(testDir, "mytest")

	// Save traces
	savedFile, err := tracer.SaveSpansLog(testPath)
	if err != nil {
		t.Fatalf("Failed to save traces: %v", err)
	}
	defer os.RemoveAll(testDir) // Clean up

	// Check that file was created in the correct directory
	if !strings.HasPrefix(savedFile, testDir) {
		t.Errorf("File should be saved in %s, but was saved to %s", testDir, savedFile)
	}

	// Check that file exists
	if _, err := os.Stat(savedFile); os.IsNotExist(err) {
		t.Errorf("Saved file does not exist: %s", savedFile)
	}

	// Check filename format
	base := filepath.Base(savedFile)
	if !strings.HasPrefix(base, "traces_mytest_") {
		t.Errorf("Filename should start with 'traces_mytest_', got: %s", base)
	}
	if !strings.HasSuffix(base, ".json") {
		t.Errorf("Filename should end with '.json', got: %s", base)
	}
}

func TestSaveSpansLogWithSpecialChars(t *testing.T) {
	tracer := NewZipkinTracer()

	// Create some test spans
	tracer.Start("test.operation")
	tracer.Stop("test.operation")

	// Test with path containing special characters
	testDir := "/tmp/test-traces-2024"
	testName := "test@trace#1"
	testPath := filepath.Join(testDir, testName)

	savedFile, err := tracer.SaveSpansLog(testPath)
	if err != nil {
		t.Fatalf("Failed to save traces: %v", err)
	}
	defer os.RemoveAll(testDir)

	// Check that directory was preserved
	if !strings.Contains(savedFile, "test-traces-2024") {
		t.Errorf("Directory name should be preserved, got: %s", savedFile)
	}

	// Check that filename was sanitized
	base := filepath.Base(savedFile)
	if strings.Contains(base, "@") || strings.Contains(base, "#") {
		t.Errorf("Special characters should be sanitized in filename: %s", base)
	}

	// Should have replaced special chars with underscores
	if !strings.Contains(base, "test_trace_1") {
		t.Errorf("Expected sanitized filename to contain 'test_trace_1', got: %s", base)
	}
}

func TestSaveSpansLogSimpleName(t *testing.T) {
	tracer := NewZipkinTracer()

	// Create some test spans
	tracer.Start("test.operation")
	tracer.Stop("test.operation")

	// Test with just a simple name (no directory)
	savedFile, err := tracer.SaveSpansLog("simple_test")
	if err != nil {
		t.Fatalf("Failed to save traces: %v", err)
	}
	defer os.Remove(savedFile)

	// Should be saved in current directory
	cwd, _ := os.Getwd()
	if !strings.HasPrefix(savedFile, cwd) {
		t.Errorf("File should be saved in current directory %s, got: %s", cwd, savedFile)
	}

	// Check filename
	base := filepath.Base(savedFile)
	if !strings.HasPrefix(base, "traces_simple_test_") {
		t.Errorf("Filename should start with 'traces_simple_test_', got: %s", base)
	}
}
