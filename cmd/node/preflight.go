package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/klauspost/cpuid/v2"
	logger "github.com/klever-io/klever-go-logger"
)

const (
	envSkipCPUCheck         = "KLEVER_SKIP_CPU_CHECK"
	minSHA256ThroughputMBps = 800
	preflightBenchDuration  = 200 * time.Millisecond
	benchBlockSize          = 16 * 1024
)

// cpuInfo captures the CPU features the preflight cares about. It is its own
// type (rather than reading klauspost/cpuid globals directly inside the
// preflight) so tests can construct synthetic CPUs on any host.
type cpuInfo struct {
	arch          string
	hasSHA        bool
	hasAVX512IFMA bool
}

// detectCPU reads the runtime architecture and the relevant feature bits.
// Architectures other than amd64 and arm64 are treated as "skip" — preflight
// is a no-op there.
func detectCPU() cpuInfo {
	info := cpuInfo{arch: runtime.GOARCH}
	switch runtime.GOARCH {
	case "amd64":
		info.hasSHA = cpuid.CPU.Has(cpuid.SHA)
		info.hasAVX512IFMA = cpuid.CPU.Has(cpuid.AVX512IFMA)
	case "arm64":
		info.hasSHA = cpuid.CPU.Has(cpuid.SHA2)
	}
	return info
}

// validatorCPUPreflight verifies the host CPU is capable of validator-grade
// SHA-256 throughput. Returns a non-nil error when the check fails; the call
// site decides whether to block startup or downgrade to a warning depending
// on the EnforceCPUPreflight config flag.
//
// Outcomes:
//   - skipped (returns nil) on unsupported architectures or when the
//     KLEVER_SKIP_CPU_CHECK=1 env var is set;
//   - failed (returns error) when measured SHA-256 throughput is below
//     minSHA256ThroughputMBps;
//   - passed (returns nil) otherwise, with an informational log line that
//     includes the measured throughput.
//
// Missing SHA-NI on amd64 is logged as a Warn but is no longer a hard fail
// on its own — the field investigation that motivated this preflight could
// not conclusively prove SHA-NI absence is the sole cause of the observed
// consensus-time disparity, so the gate is grounded in the measured number
// instead of the CPU feature flag. SHA-NI absence is the most common cause
// of low SHA-256 throughput in practice and is called out in the warn line.
func validatorCPUPreflight(log logger.Logger) error {
	return validatorCPUPreflightWithInfo(log, detectCPU(), benchSHA256)
}

// validatorCPUPreflightWithInfo is the test seam for validatorCPUPreflight.
// Passing the cpuInfo and bench function as parameters keeps the package free
// of mutable globals while still letting tests cover every branch.
//
// The bench is run twice and the maximum is reported, so a single transient
// throttle event (thermal, hypervisor noisy neighbor) does not refuse startup
// on a host that would otherwise pass.
func validatorCPUPreflightWithInfo(
	log logger.Logger,
	info cpuInfo,
	bench func(time.Duration) float64,
) error {
	if os.Getenv(envSkipCPUCheck) == "1" {
		log.Warn("validator CPU preflight bypassed via env var",
			"env", envSkipCPUCheck,
			"risk", "consensus latency may exceed peer median; not for production")
		return nil
	}

	if info.arch != "amd64" && info.arch != "arm64" {
		log.Info("validator CPU preflight skipped on unsupported arch", "arch", info.arch)
		return nil
	}

	mbps := bench(preflightBenchDuration)
	if second := bench(preflightBenchDuration); second > mbps {
		mbps = second
	}
	log.Info("validator CPU preflight measurement",
		"arch", info.arch,
		"sha_accel", info.hasSHA,
		"avx512_ifma", info.hasAVX512IFMA,
		"sha256_mbps", fmt.Sprintf("%.1f", mbps))

	if info.arch == "amd64" && !info.hasAVX512IFMA {
		log.Warn("CPU lacks AVX-512 IFMA; BLS verify ~1.5x slower than Zen4 peers (informational only)")
	}

	if !info.hasSHA {
		log.Warn("CPU lacks SHA-256 hardware acceleration "+
			"("+shaAccelName(info.arch)+"); "+
			"this is the most common cause of low SHA-256 throughput",
			"arch", info.arch)
	}

	if mbps < minSHA256ThroughputMBps {
		return fmt.Errorf(
			"validator CPU preflight failed: measured SHA-256 throughput %.1f MB/s < %d MB/s minimum. "+
				"This typically indicates missing %s%s "+
				"or a degraded host (frequency cap, thermal throttle, hypervisor masking). "+
				"Migrate to AMD Zen, Intel Ice Lake-SP+, or modern ARM with ARMv8 SHA2. "+
				"To downgrade this failure to a warning during a coordinated fleet migration, "+
				"set preferences.enforceCpuPreflight=false in the validator config. "+
				"Emergency override (NOT for production): %s=1",
			mbps, minSHA256ThroughputMBps,
			shaAccelName(info.arch),
			shaAccelArchSuffix(info.arch),
			envSkipCPUCheck)
	}
	return nil
}

// shaAccelName returns the operator-facing name of the SHA-256 hardware
// acceleration ISA on the given architecture (so log lines on arm64 do
// not misdirect operators to "SHA-NI" — the arm64 instruction set is
// ARMv8 SHA2, not Intel's SHA-NI).
func shaAccelName(arch string) string {
	switch arch {
	case "amd64":
		return "SHA-NI"
	case "arm64":
		return "ARMv8 SHA2"
	default:
		return "SHA-256 hardware acceleration"
	}
}

// shaAccelArchSuffix returns the arch-specific "common cause" suffix for
// the preflight error message. Empty on unknown archs so the label stands
// alone.
func shaAccelArchSuffix(arch string) string {
	switch arch {
	case "amd64":
		return " (Skylake-X / Cascade Lake / Haswell on amd64)"
	case "arm64":
		return " (any ARMv7 or ARMv8 chip without the SHA2 feature flag)"
	default:
		return ""
	}
}

// benchSHA256 hashes 16 KiB blocks for d and returns sustained throughput in
// megabytes per second. Returns 0 on a non-positive duration. The block size
// matches the openssl-speed reference used during the original investigation
// so operators can compare numbers directly.
//
// The hot loop checks the deadline once per innerLoop iterations to avoid
// time.Now() syscall overhead dominating on SHA-NI hosts (~3 GB/s ≈ 3M
// hashes/s).
func benchSHA256(d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	// Deterministic init — avoids crypto/rand which can block on entropy
	// starvation during early boot (validator startup may run before the
	// kernel RNG pool is fully initialised). SHA-256's amd64/arm64 fast
	// paths are data-independent so the contents do not affect throughput.
	buf := make([]byte, benchBlockSize)
	for i := range buf {
		buf[i] = byte(i)
	}
	h := sha256.New()
	digest := make([]byte, 0, h.Size())
	var bytes int64
	start := time.Now()
	deadline := start.Add(d)
	const innerLoop = 256
	for time.Now().Before(deadline) {
		for i := 0; i < innerLoop; i++ {
			h.Reset()
			_, _ = h.Write(buf)
			digest = h.Sum(digest[:0])
		}
		bytes += benchBlockSize * innerLoop
	}
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return float64(bytes) / (1024 * 1024) / elapsed
}
