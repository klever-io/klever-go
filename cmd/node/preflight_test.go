package main

import (
	"strings"
	"sync"
	"testing"
	"time"

	logger "github.com/klever-io/klever-go-logger"
)

// recordingLogger is a Logger that captures messages by level so tests can
// assert which branches of the preflight produced output.
type recordingLogger struct {
	mu    sync.Mutex
	infos []string
	warns []string
}

func (r *recordingLogger) record(slot *[]string, msg string) {
	r.mu.Lock()
	*slot = append(*slot, msg)
	r.mu.Unlock()
}

func (r *recordingLogger) Trace(msg string, _ ...interface{}) {}
func (r *recordingLogger) Debug(msg string, _ ...interface{}) {}
func (r *recordingLogger) Info(msg string, _ ...interface{}) {
	r.record(&r.infos, msg)
}
func (r *recordingLogger) Warn(msg string, _ ...interface{}) {
	r.record(&r.warns, msg)
}
func (r *recordingLogger) Error(msg string, _ ...interface{})                {}
func (r *recordingLogger) LogIfError(_ error, _ ...interface{})              {}
func (r *recordingLogger) Log(_ logger.LogLevel, _ string, _ ...interface{}) {}
func (r *recordingLogger) LogLine(_ *logger.LogLine)                         {}
func (r *recordingLogger) SetLevel(_ logger.LogLevel)                        {}
func (r *recordingLogger) GetLevel() logger.LogLevel                         { return logger.LogTrace }
func (r *recordingLogger) IsInterfaceNil() bool                              { return r == nil }

// hasInfo reports whether any captured Info message contains s. The slice
// is read under the lock to keep the helper race-safe even if a future
// caller invokes the preflight in a goroutine.
func (r *recordingLogger) hasInfo(s string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.infos {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

// hasWarn reports whether any captured Warn message contains s.
func (r *recordingLogger) hasWarn(s string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.warns {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

// fastBench returns a fixed throughput regardless of duration. Useful for
// asserting on the bench-too-slow path without waiting on the real bench.
func fastBench(mbps float64) func(time.Duration) float64 {
	return func(time.Duration) float64 { return mbps }
}

func TestValidatorCPUPreflight_HappyPath_Amd64(t *testing.T) {
	t.Setenv(envSkipCPUCheck, "")
	log := &recordingLogger{}
	info := cpuInfo{arch: "amd64", hasSHA: true, hasAVX512IFMA: true}

	if err := validatorCPUPreflightWithInfo(log, info, fastBench(2000)); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if !log.hasInfo("validator CPU preflight measurement") {
		t.Fatalf("expected a measurement info log, got infos=%v", log.infos)
	}
	if log.hasWarn("AVX-512 IFMA") {
		t.Fatalf("did not expect AVX-512 IFMA warning when feature is present, got warns=%v", log.warns)
	}
	if log.hasWarn("SHA-256 hardware acceleration") {
		t.Fatalf("did not expect SHA-NI warning when feature is present, got warns=%v", log.warns)
	}
}

func TestValidatorCPUPreflight_HappyPath_Arm64(t *testing.T) {
	t.Setenv(envSkipCPUCheck, "")
	log := &recordingLogger{}
	info := cpuInfo{arch: "arm64", hasSHA: true}

	if err := validatorCPUPreflightWithInfo(log, info, fastBench(2000)); err != nil {
		t.Fatalf("expected nil error on arm64 with fast bench, got: %v", err)
	}
	if !log.hasInfo("validator CPU preflight measurement") {
		t.Fatalf("expected measurement info log, got infos=%v", log.infos)
	}
	if log.hasWarn("AVX-512 IFMA") {
		t.Fatalf("did not expect AVX-512 IFMA warn on arm64, got warns=%v", log.warns)
	}
}

func TestValidatorCPUPreflight_HappyPath_Amd64_NoIFMA_Warns(t *testing.T) {
	t.Setenv(envSkipCPUCheck, "")
	log := &recordingLogger{}
	// Skylake-X case: SHA-NI present but AVX-512 IFMA missing — should pass with a warn.
	info := cpuInfo{arch: "amd64", hasSHA: true, hasAVX512IFMA: false}

	if err := validatorCPUPreflightWithInfo(log, info, fastBench(2000)); err != nil {
		t.Fatalf("expected nil error on Skylake-X-shaped CPU, got: %v", err)
	}
	if !log.hasWarn("AVX-512 IFMA") {
		t.Fatalf("expected AVX-512 IFMA warn, got warns=%v", log.warns)
	}
}

func TestValidatorCPUPreflight_MissingSHA_FastBench_Passes_WithWarn(t *testing.T) {
	// Missing SHA-NI is no longer a hard fail on its own — only the measured
	// throughput is. A (hypothetical) host without SHA-NI but with somehow
	// fast-enough SHA-256 should pass with a Warn note.
	t.Setenv(envSkipCPUCheck, "")
	log := &recordingLogger{}
	info := cpuInfo{arch: "amd64", hasSHA: false, hasAVX512IFMA: true}

	if err := validatorCPUPreflightWithInfo(log, info, fastBench(2000)); err != nil {
		t.Fatalf("expected nil error when bench is fast enough, even without SHA-NI; got: %v", err)
	}
	if !log.hasWarn("SHA-256 hardware acceleration") {
		t.Fatalf("expected SHA-NI absence warn, got warns=%v", log.warns)
	}
}

func TestValidatorCPUPreflight_MissingSHA_SlowBench_Errors(t *testing.T) {
	// Realistic Skylake/Haswell case: no SHA-NI plus low measured throughput.
	t.Setenv(envSkipCPUCheck, "")
	log := &recordingLogger{}
	info := cpuInfo{arch: "amd64", hasSHA: false, hasAVX512IFMA: false}

	err := validatorCPUPreflightWithInfo(log, info, fastBench(250))
	if err == nil {
		t.Fatal("expected error when measured throughput is below the floor")
	}
	if !strings.Contains(err.Error(), "throughput") {
		t.Fatalf("error message should mention throughput, got: %v", err)
	}
	if !strings.Contains(err.Error(), envSkipCPUCheck) {
		t.Fatalf("error should mention the override env var, got: %v", err)
	}
}

func TestValidatorCPUPreflight_MissingSHA2_Arm64_SlowBench_Errors(t *testing.T) {
	t.Setenv(envSkipCPUCheck, "")
	log := &recordingLogger{}
	info := cpuInfo{arch: "arm64", hasSHA: false}

	err := validatorCPUPreflightWithInfo(log, info, fastBench(200))
	if err == nil {
		t.Fatal("expected error when arm64 measured throughput is below the floor")
	}
	if !strings.Contains(err.Error(), "throughput") {
		t.Fatalf("error message should mention throughput, got: %v", err)
	}
}

func TestValidatorCPUPreflight_BenchTooSlow_Errors(t *testing.T) {
	t.Setenv(envSkipCPUCheck, "")
	log := &recordingLogger{}
	info := cpuInfo{arch: "amd64", hasSHA: true, hasAVX512IFMA: true}

	err := validatorCPUPreflightWithInfo(log, info, fastBench(minSHA256ThroughputMBps-1))
	if err == nil {
		t.Fatal("expected error when measured throughput is below the minimum")
	}
	if !strings.Contains(err.Error(), "throughput") {
		t.Fatalf("error message should mention throughput, got: %v", err)
	}
}

func TestValidatorCPUPreflight_EnvBypass_NilEvenWithSlowBench(t *testing.T) {
	t.Setenv(envSkipCPUCheck, "1")
	log := &recordingLogger{}
	info := cpuInfo{arch: "amd64", hasSHA: false}

	if err := validatorCPUPreflightWithInfo(log, info, fastBench(0)); err != nil {
		t.Fatalf("expected nil error when env bypass is active, got: %v", err)
	}
	if !log.hasWarn("bypassed via env var") {
		t.Fatalf("expected bypass warn log, got warns=%v", log.warns)
	}
}

func TestValidatorCPUPreflight_UnsupportedArch_NilAndSkips(t *testing.T) {
	t.Setenv(envSkipCPUCheck, "")
	log := &recordingLogger{}
	info := cpuInfo{arch: "386"}

	if err := validatorCPUPreflightWithInfo(log, info, fastBench(0)); err != nil {
		t.Fatalf("expected nil error on unsupported arch, got: %v", err)
	}
	if !log.hasInfo("unsupported arch") {
		t.Fatalf("expected unsupported-arch info log, got infos=%v", log.infos)
	}
}

func TestBenchSHA256_NonPositiveDuration_ReturnsZero(t *testing.T) {
	if got := benchSHA256(0); got != 0 {
		t.Fatalf("benchSHA256(0) = %.2f, want 0", got)
	}
	if got := benchSHA256(-time.Second); got != 0 {
		t.Fatalf("benchSHA256(-1s) = %.2f, want 0", got)
	}
}

func TestBenchSHA256_RealMeasurement_Positive(t *testing.T) {
	// Smoke test: a 100 ms run on any modern CPU should produce a positive
	// throughput. We do not assert a specific MB/s number to keep the test
	// portable across CI runners; 100 ms (vs 50 ms) gives heavily-throttled
	// cgroup runners enough wall time to complete a full inner-loop batch.
	if got := benchSHA256(100 * time.Millisecond); got <= 0 {
		t.Fatalf("benchSHA256(100ms) = %.2f, want > 0", got)
	}
}

// TestValidatorCPUPreflight_RealEntry_NoPanic exercises the production
// signature (which calls detectCPU() and the real benchSHA256). It does
// not assert pass/fail because the result depends on host hardware — it
// only ensures the wiring between detectCPU, benchSHA256, and the
// WithInfo seam does not panic and produces a self-consistent outcome.
func TestValidatorCPUPreflight_RealEntry_NoPanic(t *testing.T) {
	t.Setenv(envSkipCPUCheck, "1") // bypass so we don't fail on slow CI hosts
	log := &recordingLogger{}
	if err := validatorCPUPreflight(log); err != nil {
		t.Fatalf("env bypass should make validatorCPUPreflight nil, got: %v", err)
	}
	if !log.hasWarn("bypassed via env var") {
		t.Fatalf("expected bypass warn log, got warns=%v", log.warns)
	}
}
