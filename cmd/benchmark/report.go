package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Shared types
// ---------------------------------------------------------------------------

// SystemInfo captures host hardware and Go runtime metadata.
type SystemInfo struct {
	CPUs      int
	GOOS      string
	GOARCH    string
	GoVersion string
}

// BenchmarkResults is the top-level container returned by Runner.Run().
type BenchmarkResults struct {
	RunAt           time.Time
	SystemInfo      SystemInfo
	GoroutineResult *GoroutineResult
	DiskResult      *DiskResult
	NetworkResult   *NetworkResult
	KVResult        *KVResult
	MemoryResult    *MemoryResult
	BigNumResult    *BigNumResult
	CryptoResult    *CryptoResult
}

// ---------------------------------------------------------------------------
// Verdict
// ---------------------------------------------------------------------------

// Kleverchain validator hardware thresholds.
//
// Goroutine (CPU) thresholds
//   - CPUEfficiency at numCPU workers ≥ 80 %  → PASS
//   - CPUEfficiency at numCPU workers ≥ 60 %  → WARN  (scheduling overhead / throttling)
//   - CPUEfficiency at numCPU workers < 60 %  → FAIL
//
// Disk I/O thresholds (calibrated against production Kleverchain validators)
//
//	Sequential write   ≥ 150 MB/s → PASS   |  < 50 MB/s → FAIL
//	Sequential read    ≥ 250 MB/s → PASS   |  < 80 MB/s → FAIL
//	Random write IOPS  ≥ 600      → PASS   |  < 100    → FAIL  (fsynced; NVMe cap ~600-1300)
//	Random read IOPS   ≥ 5 000    → PASS   |  < 1 000  → FAIL
//
// Network thresholds (TCP loopback — reflects OS networking stack health)
//
//	Latency P50  < 100 µs → PASS   |  ≥ 300 µs → FAIL
//	Latency P99  < 500 µs → PASS   |  ≥ 1 000 µs → FAIL
//	Throughput   ≥ 2 000 MB/s → PASS   |  < 500 MB/s → FAIL
//
// KV store thresholds (in-memory, sync.RWMutex-backed)
//
//	Sequential write  ≥ 500 K ops/s → PASS   |  < 100 K ops/s → FAIL
//	Random read       ≥ 2 M ops/s   → PASS   |  < 500 K ops/s → FAIL
//	Mixed workload    ≥ 1 M ops/s   → PASS   |  < 300 K ops/s → FAIL
//
// Memory thresholds (calibrated against production Kleverchain validators)
//
//	Sequential read    ≥ 5 GB/s  → PASS   |  < 1.5 GB/s → FAIL
//	Sequential write   ≥ 4 GB/s  → PASS   |  < 1.5 GB/s → FAIL
//	Random read latency < 250 ns → PASS   |  ≥ 500 ns   → FAIL
//	Alloc throughput   ≥ 3 M/s   → PASS   |  < 0.5 M/s  → FAIL
//
// BigNum thresholds (calibrated against production Kleverchain validators)
//
//	Integer division (uint64)  ≥ 13 M ops/s → PASS   |  < 3 M ops/s → FAIL

const (
	cpuEffPassPct = 0.80
	cpuEffWarnPct = 0.60

	seqWritePassMBps = 150.0
	seqWriteFailMBps = 50.0
	seqReadPassMBps  = 250.0
	seqReadFailMBps  = 80.0
	randWritePassIPS = 600.0
	randWriteFailIPS = 100.0
	randReadPassIPS  = 5_000.0
	randReadFailIPS  = 1_000.0

	netLatP50PassUs       = 100.0
	netLatP50FailUs       = 300.0
	netLatP99PassUs       = 500.0
	netLatP99FailUs       = 1_000.0
	netThroughputPassMBps = 2_000.0
	netThroughputFailMBps = 500.0

	kvWritePassOps = 500_000.0
	kvWriteFailOps = 100_000.0
	kvReadPassOps  = 2_000_000.0
	kvReadFailOps  = 500_000.0
	kvMixedPassOps = 1_000_000.0
	kvMixedFailOps = 300_000.0

	// Memory thresholds (256 MB DRAM buffer, 64-byte allocs)
	// Calibrated against production Kleverchain validators (top nodes: 5-11 GB/s read, 4-9 GB/s write).
	memSeqReadPassGBps  = 5.0
	memSeqReadFailGBps  = 1.5
	memSeqWritePassGBps = 4.0
	memSeqWriteFailGBps = 1.5
	memRandLatPassNs    = 250.0
	memRandLatFailNs    = 500.0
	memAllocPassMOps    = 3.0
	memAllocFailMOps    = 0.5

	// BigNum thresholds
	// IntDiv calibrated: top validators achieve 14-15 M ops/s (uint64, in-loop).
	bigModExpPassOps  = 100.0
	bigModExpFailOps  = 30.0
	bigModMulPassOps  = 500_000.0
	bigModMulFailOps  = 100_000.0
	bigFloat64PassOps = 5_000_000.0
	bigFloat64FailOps = 1_000_000.0
	bigIntDivPassOps  = 13_000_000.0
	bigIntDivFailOps  = 3_000_000.0

	// Crypto thresholds (calibrated against the validator-fleet investigation:
	// AMD EPYC Zen4 with SHA-NI hits ~1740 MB/s at 16 KiB; Intel Skylake-IBRS
	// without SHA-NI sits at ~310 MB/s on the same blocks. The fail floor is
	// set above the Skylake number so any SHA-NI-deficient amd64 host fails.)
	cryptoSHA256SmallPassMBps = 1_200.0 // SHA-256 on 1 KiB blocks
	cryptoSHA256SmallFailMBps = 500.0
	cryptoSHA256LargePassMBps = 1_500.0 // SHA-256 on 16 KiB blocks
	// cryptoSHA256LargeFailMBps drives the per-metric category verdict
	// (WARN/FAIL labels in the text report); minLeaderSHA256MBps in score.go
	// (500 MB/s) drives the hard grade-F veto. The category fail floor is
	// set slightly above the veto so a host in [500, 600) MB/s shows a
	// per-metric FAIL label without triggering the grade-cap veto path —
	// gradeToVerdict still surfaces the overall verdict as FAIL via the
	// category-fail route, so behavior is consistent across both paths.
	cryptoSHA256LargeFailMBps = 600.0
	cryptoBlake2bPassMBps     = 700.0 // Blake2b-512 on 16 KiB blocks (AVX2)
	cryptoBlake2bFailMBps     = 300.0
	// Pure-Go Keccak (no SIMD path); calibrated against three healthy AMD
	// Zen2/Zen4 chips that landed in the 295–390 MB/s range. The pass
	// threshold is set just below the slowest observed healthy value so a
	// production AMD chip does not show a misleading WARN.
	cryptoKeccak256PassMBps    = 350.0
	cryptoKeccak256FailMBps    = 100.0
	cryptoEd25519VerifyPassOps = 12_000.0 // Ed25519.Verify (SHA-512-bound)
	cryptoEd25519VerifyFailOps = 5_000.0
)

type verdict int

const (
	verdictPass verdict = iota
	verdictWarn
	verdictFail
	verdictSkip // category was not run; excluded from overall verdict
)

func (v verdict) String() string {
	switch v {
	case verdictPass:
		return "PASS"
	case verdictWarn:
		return "WARN"
	case verdictFail:
		return "FAIL"
	default:
		return "N/A"
	}
}

func (v verdict) Icon() string {
	switch v {
	case verdictPass:
		return "[OK]"
	case verdictWarn:
		return "[!!]"
	case verdictFail:
		return "[XX]"
	default:
		return "[--]"
	}
}

func goroutineVerdict(r *GoroutineResult) verdict {
	if r == nil {
		return verdictSkip
	}
	if r.CPUEfficiency < cpuEffWarnPct {
		return verdictFail
	}
	if r.CPUEfficiency < cpuEffPassPct {
		return verdictWarn
	}
	return verdictPass
}

func diskVerdict(r *DiskResult) verdict {
	if r == nil {
		return verdictSkip
	}
	if r.SeqWriteMBps < seqWriteFailMBps ||
		r.SeqReadMBps < seqReadFailMBps ||
		r.RandWriteIOPS < randWriteFailIPS ||
		r.RandReadIOPS < randReadFailIPS {
		return verdictFail
	}
	if r.SeqWriteMBps < seqWritePassMBps ||
		r.SeqReadMBps < seqReadPassMBps ||
		r.RandWriteIOPS < randWritePassIPS ||
		r.RandReadIOPS < randReadPassIPS {
		return verdictWarn
	}
	return verdictPass
}

func metricVerdict(val, passThreshold, failThreshold float64) verdict {
	if val < failThreshold {
		return verdictFail
	}
	if val < passThreshold {
		return verdictWarn
	}
	return verdictPass
}

func networkVerdict(r *NetworkResult) verdict {
	if r == nil {
		return verdictSkip
	}
	p50us := float64(r.LatP50) / float64(time.Microsecond)
	p99us := float64(r.LatP99) / float64(time.Microsecond)
	if p50us >= netLatP50FailUs || p99us >= netLatP99FailUs || r.ThroughputMBps < netThroughputFailMBps {
		return verdictFail
	}
	if p50us >= netLatP50PassUs || p99us >= netLatP99PassUs || r.ThroughputMBps < netThroughputPassMBps {
		return verdictWarn
	}
	return verdictPass
}

func kvVerdict(r *KVResult) verdict {
	if r == nil {
		return verdictSkip
	}
	if r.WriteOpsPerSec < kvWriteFailOps || r.ReadOpsPerSec < kvReadFailOps || r.MixedOpsPerSec < kvMixedFailOps {
		return verdictFail
	}
	if r.WriteOpsPerSec < kvWritePassOps || r.ReadOpsPerSec < kvReadPassOps || r.MixedOpsPerSec < kvMixedPassOps {
		return verdictWarn
	}
	return verdictPass
}

func memoryVerdict(r *MemoryResult) verdict {
	if r == nil {
		return verdictSkip
	}
	if r.SeqReadGBps < memSeqReadFailGBps || r.SeqWriteGBps < memSeqWriteFailGBps ||
		r.RandLatencyNs >= memRandLatFailNs || r.AllocMOpsPerS < memAllocFailMOps {
		return verdictFail
	}
	if r.SeqReadGBps < memSeqReadPassGBps || r.SeqWriteGBps < memSeqWritePassGBps ||
		r.RandLatencyNs >= memRandLatPassNs || r.AllocMOpsPerS < memAllocPassMOps {
		return verdictWarn
	}
	return verdictPass
}

func cryptoVerdict(r *CryptoResult) verdict {
	if r == nil {
		return verdictSkip
	}
	if r.SHA256MBps < cryptoSHA256SmallFailMBps ||
		r.SHA256LargeMBps < cryptoSHA256LargeFailMBps ||
		r.Blake2bMBps < cryptoBlake2bFailMBps ||
		r.Keccak256MBps < cryptoKeccak256FailMBps ||
		r.Ed25519VerifyOpsPerSec < cryptoEd25519VerifyFailOps {
		return verdictFail
	}
	if r.SHA256MBps < cryptoSHA256SmallPassMBps ||
		r.SHA256LargeMBps < cryptoSHA256LargePassMBps ||
		r.Blake2bMBps < cryptoBlake2bPassMBps ||
		r.Keccak256MBps < cryptoKeccak256PassMBps ||
		r.Ed25519VerifyOpsPerSec < cryptoEd25519VerifyPassOps {
		return verdictWarn
	}
	return verdictPass
}

func bigNumVerdict(r *BigNumResult) verdict {
	if r == nil {
		return verdictSkip
	}
	if r.ModExpOpsPerSec < bigModExpFailOps || r.ModMulOpsPerSec < bigModMulFailOps ||
		r.Float64OpsPerSec < bigFloat64FailOps || r.IntDivOpsPerSec < bigIntDivFailOps {
		return verdictFail
	}
	if r.ModExpOpsPerSec < bigModExpPassOps || r.ModMulOpsPerSec < bigModMulPassOps ||
		r.Float64OpsPerSec < bigFloat64PassOps || r.IntDivOpsPerSec < bigIntDivPassOps {
		return verdictWarn
	}
	return verdictPass
}

func overallVerdict(vs ...verdict) verdict {
	best := verdictPass
	anyRan := false
	for _, v := range vs {
		if v == verdictSkip {
			continue
		}
		anyRan = true
		if v > best {
			best = v
		}
	}
	if !anyRan {
		return verdictSkip
	}
	return best
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

// PrintReport formats and prints the benchmark results to stdout.
func PrintReport(results *BenchmarkResults, format string) {
	if format == "json" {
		printJSON(results)
		return
	}
	printText(results)
}

// ---------------------------------------------------------------------------
// Text report
// ---------------------------------------------------------------------------

const reportWidth = 70

// metricThroughputMBpsRowFmt is the printf template for MB/s metric rows
// (network throughput + every crypto MB/s metric). Centralised so column
// width and pass/fail label format stay consistent across sections.
const metricThroughputMBpsRowFmt = "  %-32s  %7.1f MB/s  %s  (pass≥%.0f, fail<%.0f MB/s)\n"

func printText(results *BenchmarkResults) {
	sep := strings.Repeat("─", reportWidth)
	si := results.SystemInfo
	gv := goroutineVerdict(results.GoroutineResult)
	dv := diskVerdict(results.DiskResult)
	nv := networkVerdict(results.NetworkResult)
	kv := kvVerdict(results.KVResult)
	mv := memoryVerdict(results.MemoryResult)
	bv := bigNumVerdict(results.BigNumResult)
	cv := cryptoVerdict(results.CryptoResult)
	sc := ComputeScore(results)

	fmt.Println()
	fmt.Println(sep)
	fmt.Println("  Klever Blockchain Node Benchmark Report")
	fmt.Printf("  Run at : %s\n", results.RunAt.Format(time.RFC3339))
	fmt.Println(sep)
	fmt.Printf("  System : %s/%s   CPUs: %d   Go: %s\n",
		si.GOOS, si.GOARCH, si.CPUs, si.GoVersion)
	if c := results.CryptoResult; c != nil {
		fmt.Printf("  CPU    : SHA-NI=%s  AVX-512 IFMA=%s  VAES=%s  GFNI=%s\n",
			yesNo(c.HasSHA_NI), yesNo(c.HasAVX512IFMA), yesNo(c.HasVAES), yesNo(c.HasGFNI))
	}
	fmt.Println(sep)

	if results.GoroutineResult != nil {
		printGoroutineSection(results.GoroutineResult, gv, sep)
	}
	if results.DiskResult != nil {
		printDiskSection(results.DiskResult, dv, sep)
	}
	if results.NetworkResult != nil {
		printNetworkSection(results.NetworkResult, nv, sep)
	}
	if results.KVResult != nil {
		printKVSection(results.KVResult, kv, sep)
	}
	if results.MemoryResult != nil {
		printMemorySection(results.MemoryResult, mv, sep)
	}
	if results.BigNumResult != nil {
		printBigNumSection(results.BigNumResult, bv, sep)
	}
	if results.CryptoResult != nil {
		printCryptoSection(results.CryptoResult, cv, sep)
	}

	printScoreSection(sc, sep)
	fmt.Println()
}

func printGoroutineSection(r *GoroutineResult, v verdict, sep string) {
	fmt.Printf("  GOROUTINE SCALABILITY   %s %s\n", v.Icon(), v)
	fmt.Println()
	fmt.Printf("  %-28s  %s\n", "Baseline (1 worker):", humanOps(r.BaselineOps))
	fmt.Printf("  %-28s  %s\n", "Peak throughput:", humanOps(r.PeakOps))
	fmt.Printf("  %-28s  %.1f%%\n",
		fmt.Sprintf("Efficiency at %d CPUs:", r.NumCPU), r.CPUEfficiency*100)
	fmt.Println()

	// Scaling table
	fmt.Printf("  %-10s  %-18s  %-13s  %s\n",
		"Workers", "Throughput", "Avg Latency", "Efficiency")
	fmt.Printf("  %-10s  %-18s  %-13s  %s\n",
		strings.Repeat("-", 7),
		strings.Repeat("-", 14),
		strings.Repeat("-", 11),
		strings.Repeat("-", 22),
	)
	for _, lv := range r.Levels {
		marker := "  "
		if lv.Workers == r.NumCPU {
			marker = "* " // highlight the numCPU row
		}
		bar := efficiencyBar(lv.Efficiency, 18)
		fmt.Printf("  %s%-8d  %-18s  %-13s  %s %.0f%%\n",
			marker,
			lv.Workers,
			humanOps(lv.OpsPerSec),
			lv.AvgLatency.Round(time.Nanosecond),
			bar,
			lv.Efficiency*100,
		)
	}
	fmt.Println()
	fmt.Println("  (* marks the numCPU row — key efficiency metric)")
	fmt.Println(sep)
}

func printDiskSection(r *DiskResult, v verdict, sep string) {
	fmt.Printf("  DISK I/O   %s %s   (dir: %s)\n", v.Icon(), v, r.Dir)
	fmt.Printf("  Sequential file size: %d MB   Random ops: %d × 4 KB\n",
		r.FileSizeMB, numRandOps)
	fmt.Println()

	seqWV := metricVerdict(r.SeqWriteMBps, seqWritePassMBps, seqWriteFailMBps)
	seqRV := metricVerdict(r.SeqReadMBps, seqReadPassMBps, seqReadFailMBps)
	rwV := metricVerdict(r.RandWriteIOPS, randWritePassIPS, randWriteFailIPS)
	rrV := metricVerdict(r.RandReadIOPS, randReadPassIPS, randReadFailIPS)

	printDiskRow("Sequential write (64KB blocks)", fmt.Sprintf("%7.1f MB/s", r.SeqWriteMBps),
		seqWV, seqWritePassMBps, seqWriteFailMBps, "MB/s")
	printDiskRow("Sequential read  (64KB blocks)", fmt.Sprintf("%7.1f MB/s", r.SeqReadMBps),
		seqRV, seqReadPassMBps, seqReadFailMBps, "MB/s")
	printDiskRow("Random write IOPS (4KB+fsync) ", fmt.Sprintf("%7.0f IOPS", r.RandWriteIOPS),
		rwV, randWritePassIPS, randWriteFailIPS, "IOPS")
	printDiskRow("Random read  IOPS (4KB)       ", fmt.Sprintf("%7.0f IOPS", r.RandReadIOPS),
		rrV, randReadPassIPS, randReadFailIPS, "IOPS")

	fmt.Println()
	fmt.Println("  Note: writes are fsynced to disk; reads may benefit from OS page cache.")
	fmt.Println(sep)
}

func printDiskRow(name, value string, v verdict, pass, fail float64, unit string) {
	fmt.Printf("  %-32s  %s  %s  (pass≥%.0f, fail<%.0f %s)\n",
		name, value, v.Icon(), pass, fail, unit)
}

func printNetworkSection(r *NetworkResult, v verdict, sep string) {
	fmt.Printf("  NETWORK (TCP loopback)   %s %s\n", v.Icon(), v)
	fmt.Println()

	p50us := float64(r.LatP50) / float64(time.Microsecond)
	p99us := float64(r.LatP99) / float64(time.Microsecond)

	p50v := latVerdict(p50us, netLatP50PassUs, netLatP50FailUs)
	p99v := latVerdict(p99us, netLatP99PassUs, netLatP99FailUs)
	thrV := metricVerdict(r.ThroughputMBps, netThroughputPassMBps, netThroughputFailMBps)

	fmt.Printf("  %-32s  %8.1f µs  %s  (pass<%.0f, fail≥%.0f µs)\n",
		"Latency P50:", p50us, p50v.Icon(), netLatP50PassUs, netLatP50FailUs)
	fmt.Printf("  %-32s  %8.1f µs  %s  (pass<%.0f, fail≥%.0f µs)\n",
		"Latency P99:", p99us, p99v.Icon(), netLatP99PassUs, netLatP99FailUs)
	fmt.Printf(metricThroughputMBpsRowFmt,
		"Throughput:", r.ThroughputMBps, thrV.Icon(), netThroughputPassMBps, netThroughputFailMBps)

	fmt.Println()
	fmt.Println(sep)
}

// latVerdict returns a verdict for a lower-is-better metric (e.g. latency).
func latVerdict(val, passThreshold, failThreshold float64) verdict {
	if val >= failThreshold {
		return verdictFail
	}
	if val >= passThreshold {
		return verdictWarn
	}
	return verdictPass
}

func printKVSection(r *KVResult, v verdict, sep string) {
	fmt.Printf("  KV STORE (in-memory)   %s %s   (%d keys, %d B values)\n",
		v.Icon(), v, r.NumKeys, kvValueSize)
	fmt.Println()

	wv := metricVerdict(r.WriteOpsPerSec, kvWritePassOps, kvWriteFailOps)
	rv := metricVerdict(r.ReadOpsPerSec, kvReadPassOps, kvReadFailOps)
	mv := metricVerdict(r.MixedOpsPerSec, kvMixedPassOps, kvMixedFailOps)

	printKVRow("Sequential write:", r.WriteOpsPerSec, wv, kvWritePassOps, kvWriteFailOps)
	printKVRow("Random read:", r.ReadOpsPerSec, rv, kvReadPassOps, kvReadFailOps)
	printKVRow(fmt.Sprintf("Mixed (%d goroutines):", runtime.NumCPU()), r.MixedOpsPerSec, mv, kvMixedPassOps, kvMixedFailOps)

	fmt.Println()
	fmt.Println(sep)
}

func printKVRow(name string, ops float64, v verdict, pass, fail float64) {
	fmt.Printf("  %-32s  %s  %s  (pass≥%.0fK, fail<%.0fK ops/s)\n",
		name, humanOps(ops), v.Icon(), pass/1000, fail/1000)
}

func printMemorySection(r *MemoryResult, v verdict, sep string) {
	fmt.Printf("  MEMORY   %s %s   (%d MB buffer, %d B allocs)\n",
		v.Icon(), v, memBufSizeMB, memAllocSzB)
	fmt.Println()

	srV := metricVerdict(r.SeqReadGBps, memSeqReadPassGBps, memSeqReadFailGBps)
	swV := metricVerdict(r.SeqWriteGBps, memSeqWritePassGBps, memSeqWriteFailGBps)
	rlV := latVerdict(r.RandLatencyNs, memRandLatPassNs, memRandLatFailNs)
	alV := metricVerdict(r.AllocMOpsPerS, memAllocPassMOps, memAllocFailMOps)

	fmt.Printf("  %-32s  %7.2f GB/s  %s  (pass≥%.0f, fail<%.0f GB/s)\n",
		"Sequential read:", r.SeqReadGBps, srV.Icon(), memSeqReadPassGBps, memSeqReadFailGBps)
	fmt.Printf("  %-32s  %7.2f GB/s  %s  (pass≥%.0f, fail<%.0f GB/s)\n",
		"Sequential write:", r.SeqWriteGBps, swV.Icon(), memSeqWritePassGBps, memSeqWriteFailGBps)
	fmt.Printf("  %-32s  %7.1f ns    %s  (pass<%.0f, fail≥%.0f ns)\n",
		"Random read latency:", r.RandLatencyNs, rlV.Icon(), memRandLatPassNs, memRandLatFailNs)
	fmt.Printf("  %-32s  %7.2f M/s   %s  (pass≥%.0f, fail<%.0f M allocs/s)\n",
		"Alloc throughput:", r.AllocMOpsPerS, alV.Icon(), memAllocPassMOps, memAllocFailMOps)

	fmt.Println()
	fmt.Println(sep)
}

func printBigNumSection(r *BigNumResult, v verdict, sep string) {
	fmt.Printf("  BIG NUMBER / FPU   %s %s\n", v.Icon(), v)
	fmt.Println()

	meV := metricVerdict(r.ModExpOpsPerSec, bigModExpPassOps, bigModExpFailOps)
	mmV := metricVerdict(r.ModMulOpsPerSec, bigModMulPassOps, bigModMulFailOps)
	f64V := metricVerdict(r.Float64OpsPerSec, bigFloat64PassOps, bigFloat64FailOps)
	idV := metricVerdict(r.IntDivOpsPerSec, bigIntDivPassOps, bigIntDivFailOps)

	fmt.Printf("  %-32s  %7.1f ops/s  %s  (pass≥%.0f, fail<%.0f ops/s)\n",
		"ModExp 2048-bit:", r.ModExpOpsPerSec, meV.Icon(), bigModExpPassOps, bigModExpFailOps)
	printKVRow("ModMul 2048-bit:", r.ModMulOpsPerSec, mmV, bigModMulPassOps, bigModMulFailOps)
	fmt.Printf("  %-32s  %s  %s  (pass≥%.0fM, fail<%.0fM ops/s)\n",
		"Float64 (pow/sqrt/log):", humanOps(r.Float64OpsPerSec), f64V.Icon(),
		bigFloat64PassOps/1e6, bigFloat64FailOps/1e6)
	fmt.Printf("  %-32s  %s  %s  (pass≥%.0fM, fail<%.0fM ops/s)\n",
		"Integer division (uint64):", humanOps(r.IntDivOpsPerSec), idV.Icon(),
		bigIntDivPassOps/1e6, bigIntDivFailOps/1e6)

	fmt.Println()
	fmt.Println(sep)
}

func printCryptoSection(r *CryptoResult, v verdict, sep string) {
	fmt.Printf("  CRYPTO / HASHING   %s %s\n", v.Icon(), v)
	fmt.Println()

	s256V := metricVerdict(r.SHA256MBps, cryptoSHA256SmallPassMBps, cryptoSHA256SmallFailMBps)
	s256LV := metricVerdict(r.SHA256LargeMBps, cryptoSHA256LargePassMBps, cryptoSHA256LargeFailMBps)
	b2V := metricVerdict(r.Blake2bMBps, cryptoBlake2bPassMBps, cryptoBlake2bFailMBps)
	kV := metricVerdict(r.Keccak256MBps, cryptoKeccak256PassMBps, cryptoKeccak256FailMBps)
	edV := metricVerdict(r.Ed25519VerifyOpsPerSec, cryptoEd25519VerifyPassOps, cryptoEd25519VerifyFailOps)

	fmt.Printf(metricThroughputMBpsRowFmt,
		"SHA-256 (1 KiB blocks):", r.SHA256MBps, s256V.Icon(),
		cryptoSHA256SmallPassMBps, cryptoSHA256SmallFailMBps)
	fmt.Printf(metricThroughputMBpsRowFmt,
		"SHA-256 (16 KiB blocks):", r.SHA256LargeMBps, s256LV.Icon(),
		cryptoSHA256LargePassMBps, cryptoSHA256LargeFailMBps)
	fmt.Printf(metricThroughputMBpsRowFmt,
		"Blake2b-512 (16 KiB):", r.Blake2bMBps, b2V.Icon(),
		cryptoBlake2bPassMBps, cryptoBlake2bFailMBps)
	fmt.Printf(metricThroughputMBpsRowFmt,
		"Keccak-256 (16 KiB):", r.Keccak256MBps, kV.Icon(),
		cryptoKeccak256PassMBps, cryptoKeccak256FailMBps)
	fmt.Printf("  %-32s  %s  %s  (pass≥%.0fK, fail<%.0fK ops/s)\n",
		"Ed25519 verify:", humanOps(r.Ed25519VerifyOpsPerSec), edV.Icon(),
		cryptoEd25519VerifyPassOps/1000, cryptoEd25519VerifyFailOps/1000)

	if runtime.GOARCH == "amd64" && !r.HasSHA_NI {
		fmt.Println()
		fmt.Println("  ! CPU lacks SHA-NI; this is the most common cause of low SHA-256 throughput.")
		fmt.Println("  ! If the throughput numbers above are below the pass thresholds, migrate to")
		fmt.Println("  ! AMD Zen, Intel Ice Lake-SP+, or modern ARM (with ARMv8 SHA2).")
	}
	fmt.Println()
	fmt.Println(sep)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// ---------------------------------------------------------------------------
// Score section
// ---------------------------------------------------------------------------

func printScoreSection(s BenchmarkScore, sep string) {
	fmt.Printf("  SCORE : %d / %d   Grade: %s   %s\n",
		s.Total, s.MaxTotal, s.Grade, scoreGradeSummary(s.Grade))
	if s.Vetoed {
		fmt.Printf("  ! Hard veto: %s\n", s.VetoedReason)
	}
	fmt.Println()

	printScoreRow("Goroutine (CPU)", s.Goroutine, weightGoroutine)
	printScoreRow("Disk I/O", s.Disk, weightDisk)
	printScoreRow("Network", s.Network, weightNetwork)
	printScoreRow("KV Store", s.KV, weightKV)
	printScoreRow("Memory", s.Memory, weightMemory)
	printScoreRow("BigNum / FPU", s.BigNum, weightBigNum)
	printScoreRow("Crypto / Hashing", s.Crypto, weightCrypto)
}

func printScoreRow(name string, c CategoryScore, weight int) {
	if c.Skipped {
		fmt.Printf("  %-20s  %s\n", name, "—  SKIPPED")
		return
	}
	bar := scoreBar(c.Pct(), 24)
	fmt.Printf("  %-20s  %3d / %3d  %s  %5.1f%%\n",
		name, c.Points, weight, bar, c.Pct()*100)
}

// scoreBar renders a filled/empty bar of the given width for a 0.0–1.0 ratio.
func scoreBar(pct float64, width int) string {
	filled := int(math.Round(pct * float64(width)))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

// ---------------------------------------------------------------------------
// Visual helpers
// ---------------------------------------------------------------------------

func efficiencyBar(eff float64, width int) string {
	filled := int(math.Round(eff * float64(width)))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat(".", width-filled) + "]"
}

func humanOps(ops float64) string {
	switch {
	case ops >= 1_000_000:
		return fmt.Sprintf("%.2f M ops/s", ops/1_000_000)
	case ops >= 1_000:
		return fmt.Sprintf("%.1f K ops/s", ops/1_000)
	default:
		return fmt.Sprintf("%.0f ops/s", ops)
	}
}

// gradeToVerdict derives the overall verdict from the BenchmarkScore grade.
// If any individual category is verdictFail that hard-overrides the grade —
// a node with a broken subsystem must not be promoted by a high aggregate score.
// Otherwise the grade is the source of truth so both systems stay consistent.
func gradeToVerdict(grade string, overallCategoryVerdict verdict) verdict {
	if overallCategoryVerdict == verdictFail {
		return verdictFail
	}
	switch grade {
	case "S", "A", "B":
		return verdictPass
	case "C":
		return verdictWarn
	case "N/A":
		return verdictSkip
	default: // D, F
		return verdictFail
	}
}

func verdictSummary(v verdict) string {
	switch v {
	case verdictPass:
		return "This node meets Kleverchain validator requirements."
	case verdictWarn:
		return "Performance meets minimum requirements but is below recommended levels — consider a hardware upgrade and review individual sections before deploying."
	case verdictFail:
		return "This node does NOT meet Kleverchain validator requirements."
	default:
		return "No benchmarks were run; results are unavailable."
	}
}

// ---------------------------------------------------------------------------
// JSON report
// ---------------------------------------------------------------------------

type jsonReport struct {
	RunAt          string         `json:"run_at"`
	System         SystemInfo     `json:"system"`
	Goroutine      *jsonGoroutine `json:"goroutine,omitempty"`
	Disk           *jsonDisk      `json:"disk,omitempty"`
	Network        *jsonNetwork   `json:"network,omitempty"`
	KV             *jsonKV        `json:"kv,omitempty"`
	Memory         *jsonMemory    `json:"memory,omitempty"`
	BigNum         *jsonBigNum    `json:"bignum,omitempty"`
	Crypto         *jsonCrypto    `json:"crypto,omitempty"`
	Score          jsonScore      `json:"score"`
	OverallVerdict string         `json:"overall_verdict"`
}

type jsonCategoryScore struct {
	Points  int     `json:"points"`
	Max     int     `json:"max"`
	Pct     float64 `json:"pct"`
	Skipped bool    `json:"skipped,omitempty"`
}

type jsonScore struct {
	Total        int               `json:"total"`
	MaxTotal     int               `json:"max_total"`
	Pct          float64           `json:"pct"`
	Grade        string            `json:"grade"`
	Vetoed       bool              `json:"vetoed"`
	VetoedReason string            `json:"veto_reason,omitempty"`
	Goroutine    jsonCategoryScore `json:"goroutine"`
	Disk         jsonCategoryScore `json:"disk"`
	Network      jsonCategoryScore `json:"network"`
	KV           jsonCategoryScore `json:"kv"`
	Memory       jsonCategoryScore `json:"memory"`
	BigNum       jsonCategoryScore `json:"bignum"`
	Crypto       jsonCategoryScore `json:"crypto"`
}

type jsonGoroutineLevel struct {
	Workers    int     `json:"workers"`
	OpsPerSec  float64 `json:"ops_per_sec"`
	AvgLatNs   int64   `json:"avg_latency_ns"`
	Efficiency float64 `json:"efficiency"`
}

type jsonGoroutine struct {
	BaselineOps   float64              `json:"baseline_ops_per_sec"`
	PeakOps       float64              `json:"peak_ops_per_sec"`
	CPUEfficiency float64              `json:"cpu_efficiency"`
	Levels        []jsonGoroutineLevel `json:"levels"`
	Verdict       string               `json:"verdict"`
}

type jsonDisk struct {
	Dir           string  `json:"dir"`
	FileSizeMB    int     `json:"file_size_mb"`
	SeqWriteMBps  float64 `json:"seq_write_mbps"`
	SeqReadMBps   float64 `json:"seq_read_mbps"`
	RandWriteIOPS float64 `json:"rand_write_iops"`
	RandReadIOPS  float64 `json:"rand_read_iops"`
	Verdict       string  `json:"verdict"`
}

type jsonNetwork struct {
	LatP50Us       float64 `json:"lat_p50_us"`
	LatP99Us       float64 `json:"lat_p99_us"`
	ThroughputMBps float64 `json:"throughput_mbps"`
	Verdict        string  `json:"verdict"`
}

type jsonKV struct {
	NumKeys        int     `json:"num_keys"`
	WriteOpsPerSec float64 `json:"write_ops_per_sec"`
	ReadOpsPerSec  float64 `json:"read_ops_per_sec"`
	MixedOpsPerSec float64 `json:"mixed_ops_per_sec"`
	Verdict        string  `json:"verdict"`
}

type jsonMemory struct {
	SeqReadGBps   float64 `json:"seq_read_gbps"`
	SeqWriteGBps  float64 `json:"seq_write_gbps"`
	RandLatencyNs float64 `json:"rand_latency_ns"`
	AllocMOpsPerS float64 `json:"alloc_mops_per_sec"`
	Verdict       string  `json:"verdict"`
}

type jsonBigNum struct {
	ModExpOpsPerSec  float64 `json:"modexp_ops_per_sec"`
	ModMulOpsPerSec  float64 `json:"modmul_ops_per_sec"`
	Float64OpsPerSec float64 `json:"float64_ops_per_sec"`
	IntDivOpsPerSec  float64 `json:"intdiv_ops_per_sec"`
	Verdict          string  `json:"verdict"`
}

type jsonCPUFeatures struct {
	HasSHA_NI     bool `json:"sha_ni"`
	HasAVX512IFMA bool `json:"avx512_ifma"`
	HasVAES       bool `json:"vaes"`
	HasGFNI       bool `json:"gfni"`
}

type jsonCrypto struct {
	SHA256MBps             float64         `json:"sha256_1k_mbps"`
	SHA256LargeMBps        float64         `json:"sha256_16k_mbps"`
	Blake2bMBps            float64         `json:"blake2b_16k_mbps"`
	Keccak256MBps          float64         `json:"keccak256_16k_mbps"`
	Ed25519VerifyOpsPerSec float64         `json:"ed25519_verify_ops_per_sec"`
	CPUFeatures            jsonCPUFeatures `json:"cpu_features"`
	Verdict                string          `json:"verdict"`
}

func printJSON(results *BenchmarkResults) {
	gv := goroutineVerdict(results.GoroutineResult)
	dv := diskVerdict(results.DiskResult)
	nv := networkVerdict(results.NetworkResult)
	kv := kvVerdict(results.KVResult)
	mv := memoryVerdict(results.MemoryResult)
	bv := bigNumVerdict(results.BigNumResult)
	cv := cryptoVerdict(results.CryptoResult)
	sc := ComputeScore(results)
	ov := gradeToVerdict(sc.Grade, overallVerdict(gv, dv, nv, kv, mv, bv, cv))

	report := jsonReport{
		RunAt:          results.RunAt.Format(time.RFC3339),
		System:         results.SystemInfo,
		OverallVerdict: ov.String(),
	}

	if r := results.GoroutineResult; r != nil {
		levels := make([]jsonGoroutineLevel, len(r.Levels))
		for i, lv := range r.Levels {
			levels[i] = jsonGoroutineLevel{
				Workers:    lv.Workers,
				OpsPerSec:  lv.OpsPerSec,
				AvgLatNs:   lv.AvgLatency.Nanoseconds(),
				Efficiency: lv.Efficiency,
			}
		}
		report.Goroutine = &jsonGoroutine{
			BaselineOps:   r.BaselineOps,
			PeakOps:       r.PeakOps,
			CPUEfficiency: r.CPUEfficiency,
			Levels:        levels,
			Verdict:       gv.String(),
		}
	}

	if r := results.DiskResult; r != nil {
		report.Disk = &jsonDisk{
			Dir:           r.Dir,
			FileSizeMB:    r.FileSizeMB,
			SeqWriteMBps:  r.SeqWriteMBps,
			SeqReadMBps:   r.SeqReadMBps,
			RandWriteIOPS: r.RandWriteIOPS,
			RandReadIOPS:  r.RandReadIOPS,
			Verdict:       dv.String(),
		}
	}

	if r := results.NetworkResult; r != nil {
		report.Network = &jsonNetwork{
			LatP50Us:       float64(r.LatP50) / float64(time.Microsecond),
			LatP99Us:       float64(r.LatP99) / float64(time.Microsecond),
			ThroughputMBps: r.ThroughputMBps,
			Verdict:        nv.String(),
		}
	}

	if r := results.KVResult; r != nil {
		report.KV = &jsonKV{
			NumKeys:        r.NumKeys,
			WriteOpsPerSec: r.WriteOpsPerSec,
			ReadOpsPerSec:  r.ReadOpsPerSec,
			MixedOpsPerSec: r.MixedOpsPerSec,
			Verdict:        kv.String(),
		}
	}

	if r := results.MemoryResult; r != nil {
		report.Memory = &jsonMemory{
			SeqReadGBps:   r.SeqReadGBps,
			SeqWriteGBps:  r.SeqWriteGBps,
			RandLatencyNs: r.RandLatencyNs,
			AllocMOpsPerS: r.AllocMOpsPerS,
			Verdict:       mv.String(),
		}
	}

	if r := results.BigNumResult; r != nil {
		report.BigNum = &jsonBigNum{
			ModExpOpsPerSec:  r.ModExpOpsPerSec,
			ModMulOpsPerSec:  r.ModMulOpsPerSec,
			Float64OpsPerSec: r.Float64OpsPerSec,
			IntDivOpsPerSec:  r.IntDivOpsPerSec,
			Verdict:          bv.String(),
		}
	}

	if r := results.CryptoResult; r != nil {
		report.Crypto = &jsonCrypto{
			SHA256MBps:             r.SHA256MBps,
			SHA256LargeMBps:        r.SHA256LargeMBps,
			Blake2bMBps:            r.Blake2bMBps,
			Keccak256MBps:          r.Keccak256MBps,
			Ed25519VerifyOpsPerSec: r.Ed25519VerifyOpsPerSec,
			CPUFeatures: jsonCPUFeatures{
				HasSHA_NI:     r.HasSHA_NI,
				HasAVX512IFMA: r.HasAVX512IFMA,
				HasVAES:       r.HasVAES,
				HasGFNI:       r.HasGFNI,
			},
			Verdict: cv.String(),
		}
	}

	report.Score = jsonScore{
		Total:        sc.Total,
		MaxTotal:     sc.MaxTotal,
		Pct:          sc.Pct,
		Grade:        sc.Grade,
		Vetoed:       sc.Vetoed,
		VetoedReason: sc.VetoedReason,
		Goroutine:    jsonCategoryScore{Points: sc.Goroutine.Points, Max: sc.Goroutine.Max, Pct: sc.Goroutine.Pct(), Skipped: sc.Goroutine.Skipped},
		Disk:         jsonCategoryScore{Points: sc.Disk.Points, Max: sc.Disk.Max, Pct: sc.Disk.Pct(), Skipped: sc.Disk.Skipped},
		Network:      jsonCategoryScore{Points: sc.Network.Points, Max: sc.Network.Max, Pct: sc.Network.Pct(), Skipped: sc.Network.Skipped},
		KV:           jsonCategoryScore{Points: sc.KV.Points, Max: sc.KV.Max, Pct: sc.KV.Pct(), Skipped: sc.KV.Skipped},
		Memory:       jsonCategoryScore{Points: sc.Memory.Points, Max: sc.Memory.Max, Pct: sc.Memory.Pct(), Skipped: sc.Memory.Skipped},
		BigNum:       jsonCategoryScore{Points: sc.BigNum.Points, Max: sc.BigNum.Max, Pct: sc.BigNum.Pct(), Skipped: sc.BigNum.Skipped},
		Crypto:       jsonCategoryScore{Points: sc.Crypto.Points, Max: sc.Crypto.Max, Pct: sc.Crypto.Pct(), Skipped: sc.Crypto.Skipped},
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to encode benchmark report as JSON: %v\n", err)
	}
}
