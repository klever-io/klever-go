package main

// RunCryptoBenchmark measures the cryptographic throughput a Kleverchain
// validator depends on for consensus, transaction hashing, and signature
// verification. The hashing primitives matter most — Klever's TX hot path
// hashes SHA-256 per-TX, per-header, and per-state-entry, and the difference
// between SHA-NI-equipped and SHA-NI-deficient hardware is roughly 5–6× on
// SHA-256 throughput, which translates almost linearly into wall-time on
// smart-contract transactions.
//
// Metrics:
//
//   - SHA256MBps      : 1 KiB blocks; sensitive to SHA-NI startup cost.
//   - SHA256LargeMBps : 16 KiB blocks; matches the openssl-speed reference
//     used during the original validator-fleet investigation, and reveals
//     sustained SHA-NI throughput separately from the small-block path.
//   - Blake2bMBps     : 16 KiB blocks; exercises the AVX2 path in
//     golang.org/x/crypto/blake2b.
//   - Keccak256MBps   : 16 KiB blocks; pure-Go reference (no SIMD), used
//     as a sanity check — Keccak should be roughly identical across CPUs
//     of the same generation, so a large gap here means non-CPU factors
//     are at play (frequency cap, thermal throttle, hypervisor masking).
//   - Ed25519VerifyOpsPerSec : stdlib Ed25519 signature verification,
//     SHA-512-bound; complements the SHA-256 numbers.
//
// CPU feature attestation is reported alongside the throughput numbers so
// operators can correlate measured perf to the underlying instruction set.
//
// BLS-verify (MCL/herumi) is intentionally not measured here — wiring MCL
// into the benchmark binary would force a CGO dependency on operators that
// just want to grade their hardware. The AVX-512 IFMA flag is reported
// instead as a proxy for the BLS-pairing fast path; that flag has a tight
// 1.5× correlation with herumi/MCL pairing throughput.

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"hash"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/klauspost/cpuid/v2"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/sha3"
)

const (
	cryptoBenchDuration = 2 * time.Second
	cryptoSmallBlock    = 1024      // 1 KiB
	cryptoLargeBlock    = 16 * 1024 // 16 KiB — matches openssl-speed reference
)

// CryptoResult holds the per-primitive throughput and CPU feature flags.
type CryptoResult struct {
	SHA256MBps             float64 // SHA-256 throughput on 1 KiB blocks
	SHA256LargeMBps        float64 // SHA-256 throughput on 16 KiB blocks
	Blake2bMBps            float64 // Blake2b-512 throughput on 16 KiB blocks
	Keccak256MBps          float64 // Keccak-256 throughput on 16 KiB blocks
	Ed25519VerifyOpsPerSec float64 // Ed25519 signature verifications per second

	// CPU feature flags — reported but not directly scored. The overall
	// score applies a hard veto when HasSHA_NI is false on amd64 (see
	// score.go). The other flags are informational; AVX-512 IFMA in
	// particular indicates whether the BLS pairing fast path is available.
	HasSHA_NI     bool
	HasAVX512IFMA bool
	HasVAES       bool
	HasGFNI       bool
}

// RunCryptoBenchmark executes all crypto sub-benchmarks and detects CPU
// feature flags. Returns a populated CryptoResult on success.
func RunCryptoBenchmark() (*CryptoResult, error) {
	r := &CryptoResult{
		HasSHA_NI:     hasSHAAcceleration(),
		HasAVX512IFMA: cpuid.CPU.Has(cpuid.AVX512IFMA),
		HasVAES:       cpuid.CPU.Has(cpuid.VAES),
		HasGFNI:       cpuid.CPU.Has(cpuid.GFNI),
	}

	clearLine("  Crypto: SHA-256 1 KiB blocks (%s)...", cryptoBenchDuration)
	r.SHA256MBps = benchHashMBps(sha256.New, cryptoSmallBlock)

	clearLine("  Crypto: SHA-256 16 KiB blocks (%s)...", cryptoBenchDuration)
	r.SHA256LargeMBps = benchHashMBps(sha256.New, cryptoLargeBlock)

	clearLine("  Crypto: Blake2b 16 KiB blocks (%s)...", cryptoBenchDuration)
	r.Blake2bMBps = benchHashMBps(newBlake2b512, cryptoLargeBlock)

	clearLine("  Crypto: Keccak-256 16 KiB blocks (%s)...", cryptoBenchDuration)
	r.Keccak256MBps = benchHashMBps(sha3.NewLegacyKeccak256, cryptoLargeBlock)

	clearLine("  Crypto: Ed25519 verify (%s)...", cryptoBenchDuration)
	ops, err := benchEd25519Verify()
	if err != nil {
		return nil, fmt.Errorf("ed25519 verify: %w", err)
	}
	r.Ed25519VerifyOpsPerSec = ops

	fmt.Fprintf(os.Stderr, "  %s\r", strings.Repeat(" ", 60))
	return r, nil
}

// hasSHAAcceleration reports whether SHA-256 hardware acceleration is
// available on the current architecture: SHA-NI on amd64, ARMv8 SHA2 on
// arm64. Returns false on every other architecture.
func hasSHAAcceleration() bool {
	switch runtime.GOARCH {
	case "amd64":
		return cpuid.CPU.Has(cpuid.SHA)
	case "arm64":
		return cpuid.CPU.Has(cpuid.SHA2)
	default:
		return false
	}
}

// newBlake2b512 returns a fresh Blake2b-512 hash. Wrapped to match the
// stdlib `func() hash.Hash` constructor signature so it can be passed to
// benchHashMBps directly. blake2b.New512(nil) only errors on a non-nil key
// of invalid length; passing nil never errors.
func newBlake2b512() hash.Hash {
	h, err := blake2b.New512(nil)
	if err != nil {
		panic(fmt.Sprintf("blake2b.New512(nil) unexpectedly errored: %v", err))
	}
	return h
}

// benchHashMBps hashes blockSize-byte buffers for cryptoBenchDuration and
// returns sustained throughput in MB/s.
//
// The buffer is seeded once with crypto/rand so data-dependent hash
// implementations (Blake2b, Keccak) measure realistic throughput rather
// than the all-zero special case. SHA-256's amd64/arm64 fast paths are
// data-independent, so seeding does not alter their numbers.
//
// The hot loop checks the deadline once per innerLoop iterations to
// minimise time.Now() syscall overhead on fast hosts.
func benchHashMBps(newHash func() hash.Hash, blockSize int) float64 {
	buf := make([]byte, blockSize)
	if _, err := rand.Read(buf); err != nil {
		for i := range buf {
			buf[i] = byte(i)
		}
	}
	h := newHash()
	deadline := time.Now().Add(cryptoBenchDuration)
	start := time.Now()
	var bytes int64
	const innerLoop = 256
	for time.Now().Before(deadline) {
		for range innerLoop {
			h.Reset()
			_, _ = h.Write(buf)
			_ = h.Sum(nil)
		}
		bytes += int64(blockSize) * innerLoop
	}
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return float64(bytes) / (1024 * 1024) / elapsed
}

// benchEd25519Verify generates a key pair + signs once, then measures how
// many verifications per second the host can sustain. Verify is the path
// that runs on every TX received by the validator, so it is more relevant
// to fleet behavior than Sign.
func benchEd25519Verify() (float64, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return 0, err
	}
	msg := make([]byte, 64)
	if _, err := rand.Read(msg); err != nil {
		return 0, err
	}
	sig := ed25519.Sign(priv, msg)

	deadline := time.Now().Add(cryptoBenchDuration)
	start := time.Now()
	var count int64
	for time.Now().Before(deadline) {
		if !ed25519.Verify(pub, msg, sig) {
			return 0, fmt.Errorf("ed25519 self-verify failed unexpectedly")
		}
		count++
	}
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		return 0, nil
	}
	return float64(count) / elapsed, nil
}
