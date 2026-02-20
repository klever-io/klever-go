package main

// ComputeScore assigns each benchmark category a weighted point score and
// produces a total out of the maximum possible points for enabled categories.
//
// Each category score is normalised 0–100 via linear interpolation between a
// "fail" floor (score = 0) and an "excellent" ceiling (score = 100).  The
// category raw score is then scaled by the category weight to produce points.
//
// Skipped categories (nil result) contribute 0 points and 0 to the max,
// keeping the percentage — and therefore the grade — fair regardless of which
// subset of benchmarks the user chooses to run.
//
// Point weights (total = 1000 when all categories are enabled):
//
//	Goroutine (CPU scalability)  200
//	Disk I/O                     200
//	KV Store (state access)      200
//	Network (P2P stack)          150
//	Memory (DRAM + allocator)    150
//	BigNum / FPU                 100
//
// Grade thresholds (% of enabled max):
//
//	≥ 90 %  → S   Elite — top-tier validator hardware
//	≥ 75 %  → A   Excellent — production-ready for high-traffic networks
//	≥ 60 %  → B   Good — suitable for standard validator operation
//	≥ 45 %  → C   Acceptable — meets minimum requirements
//	≥ 30 %  → D   Marginal — several metrics below recommended levels
//	< 30 %  → F   Insufficient — does not meet validator requirements

import (
	"math"
	"time"
)

// ---------------------------------------------------------------------------
// Excellent thresholds (score = 100)
// ---------------------------------------------------------------------------

const (
	cpuEffExcellentPct = 0.95

	seqWriteExcellentMBps = 500.0
	seqReadExcellentMBps  = 1_000.0
	randWriteExcellentIPS = 15_000.0
	randReadExcellentIPS  = 30_000.0

	netLatP50ExcellentUs      = 10.0
	netLatP99ExcellentUs      = 50.0
	netThroughputExcellentMBps = 10_000.0

	kvWriteExcellentOps = 2_000_000.0
	kvReadExcellentOps  = 10_000_000.0
	kvMixedExcellentOps = 5_000_000.0

	memSeqReadExcellentGBps  = 40.0
	memSeqWriteExcellentGBps = 30.0
	memRandLatExcellentNs    = 30.0
	memAllocExcellentMOps    = 100.0

	bigModExpExcellentOps  = 500.0
	bigModMulExcellentOps  = 2_000_000.0
	bigFloat64ExcellentOps = 50_000_000.0
	bigIntDivExcellentOps  = 300_000_000.0
)

// ---------------------------------------------------------------------------
// Category weights
// ---------------------------------------------------------------------------

const (
	weightGoroutine = 200
	weightDisk      = 200
	weightKV        = 200
	weightNetwork   = 150
	weightMemory    = 150
	weightBigNum    = 100
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// CategoryScore holds the points earned and the maximum possible for one
// benchmark category.  Skipped is true when the category was not run.
type CategoryScore struct {
	Points  int
	Max     int
	Skipped bool
}

// Pct returns the category percentage (0.0–1.0).  Returns 0 if skipped.
func (c CategoryScore) Pct() float64 {
	if c.Skipped || c.Max == 0 {
		return 0
	}
	return float64(c.Points) / float64(c.Max)
}

// BenchmarkScore is the aggregate scoring result.
type BenchmarkScore struct {
	Goroutine CategoryScore
	Disk      CategoryScore
	Network   CategoryScore
	KV        CategoryScore
	Memory    CategoryScore
	BigNum    CategoryScore

	Total    int     // sum of enabled category points
	MaxTotal int     // sum of enabled category maxes
	Pct      float64 // Total / MaxTotal (0.0–1.0); 0 if nothing enabled
	Grade    string  // S / A / B / C / D / F
}

// ComputeScore builds a BenchmarkScore from all benchmark results.
func ComputeScore(r *BenchmarkResults) BenchmarkScore {
	var s BenchmarkScore

	s.Goroutine = scoreCategory(goroutineCatScore(r.GoroutineResult), weightGoroutine, r.GoroutineResult == nil)
	s.Disk = scoreCategory(diskCatScore(r.DiskResult), weightDisk, r.DiskResult == nil)
	s.Network = scoreCategory(networkCatScore(r.NetworkResult), weightNetwork, r.NetworkResult == nil)
	s.KV = scoreCategory(kvCatScore(r.KVResult), weightKV, r.KVResult == nil)
	s.Memory = scoreCategory(memoryCatScore(r.MemoryResult), weightMemory, r.MemoryResult == nil)
	s.BigNum = scoreCategory(bigNumCatScore(r.BigNumResult), weightBigNum, r.BigNumResult == nil)

	for _, c := range []CategoryScore{s.Goroutine, s.Disk, s.Network, s.KV, s.Memory, s.BigNum} {
		s.Total += c.Points
		s.MaxTotal += c.Max
	}
	if s.MaxTotal > 0 {
		s.Pct = float64(s.Total) / float64(s.MaxTotal)
	}
	s.Grade = scoreGrade(s.Pct)
	return s
}

// ---------------------------------------------------------------------------
// Category normalised scores (0.0 – 100.0)
// ---------------------------------------------------------------------------

func goroutineCatScore(r *GoroutineResult) float64 {
	if r == nil {
		return 0
	}
	return normHigh(r.CPUEfficiency, cpuEffWarnPct, cpuEffExcellentPct)
}

func diskCatScore(r *DiskResult) float64 {
	if r == nil {
		return 0
	}
	return mean(
		normHigh(r.SeqWriteMBps, seqWriteFailMBps, seqWriteExcellentMBps),
		normHigh(r.SeqReadMBps, seqReadFailMBps, seqReadExcellentMBps),
		normHigh(r.RandWriteIOPS, randWriteFailIPS, randWriteExcellentIPS),
		normHigh(r.RandReadIOPS, randReadFailIPS, randReadExcellentIPS),
	)
}

func networkCatScore(r *NetworkResult) float64 {
	if r == nil {
		return 0
	}
	p50us := float64(r.LatP50) / float64(time.Microsecond)
	p99us := float64(r.LatP99) / float64(time.Microsecond)
	return mean(
		normLow(p50us, netLatP50FailUs, netLatP50ExcellentUs),
		normLow(p99us, netLatP99FailUs, netLatP99ExcellentUs),
		normHigh(r.ThroughputMBps, netThroughputFailMBps, netThroughputExcellentMBps),
	)
}

func kvCatScore(r *KVResult) float64 {
	if r == nil {
		return 0
	}
	return mean(
		normHigh(r.WriteOpsPerSec, kvWriteFailOps, kvWriteExcellentOps),
		normHigh(r.ReadOpsPerSec, kvReadFailOps, kvReadExcellentOps),
		normHigh(r.MixedOpsPerSec, kvMixedFailOps, kvMixedExcellentOps),
	)
}

func memoryCatScore(r *MemoryResult) float64 {
	if r == nil {
		return 0
	}
	return mean(
		normHigh(r.SeqReadGBps, memSeqReadFailGBps, memSeqReadExcellentGBps),
		normHigh(r.SeqWriteGBps, memSeqWriteFailGBps, memSeqWriteExcellentGBps),
		normLow(r.RandLatencyNs, memRandLatFailNs, memRandLatExcellentNs),
		normHigh(r.AllocMOpsPerS, memAllocFailMOps, memAllocExcellentMOps),
	)
}

func bigNumCatScore(r *BigNumResult) float64 {
	if r == nil {
		return 0
	}
	return mean(
		normHigh(r.ModExpOpsPerSec, bigModExpFailOps, bigModExpExcellentOps),
		normHigh(r.ModMulOpsPerSec, bigModMulFailOps, bigModMulExcellentOps),
		normHigh(r.Float64OpsPerSec, bigFloat64FailOps, bigFloat64ExcellentOps),
		normHigh(r.IntDivOpsPerSec, bigIntDivFailOps, bigIntDivExcellentOps),
	)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// normHigh converts a higher-is-better metric to 0–100.
// val ≤ fail → 0,  val ≥ excellent → 100,  linear in between.
func normHigh(val, fail, excellent float64) float64 {
	if val <= fail {
		return 0
	}
	if val >= excellent {
		return 100
	}
	return (val - fail) / (excellent - fail) * 100
}

// normLow converts a lower-is-better metric to 0–100.
// val ≥ fail → 0,  val ≤ excellent → 100,  linear in between.
func normLow(val, fail, excellent float64) float64 {
	if val >= fail {
		return 0
	}
	if val <= excellent {
		return 100
	}
	return (fail - val) / (fail - excellent) * 100
}

// mean returns the arithmetic mean of the provided values.
func mean(vals ...float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// scoreCategory converts a 0–100 raw score into a CategoryScore.
func scoreCategory(raw float64, weight int, skipped bool) CategoryScore {
	if skipped {
		return CategoryScore{Skipped: true}
	}
	return CategoryScore{
		Points: int(math.Round(raw / 100 * float64(weight))),
		Max:    weight,
	}
}

// scoreGrade maps a percentage (0–1) to a letter grade.
func scoreGrade(pct float64) string {
	switch {
	case pct >= 0.90:
		return "S"
	case pct >= 0.75:
		return "A"
	case pct >= 0.60:
		return "B"
	case pct >= 0.45:
		return "C"
	case pct >= 0.30:
		return "D"
	default:
		return "F"
	}
}

// scoreGradeSummary returns a one-line description for a letter grade.
func scoreGradeSummary(g string) string {
	switch g {
	case "S":
		return "Elite — top-tier validator hardware"
	case "A":
		return "Excellent — production-ready for high-traffic networks"
	case "B":
		return "Good — suitable for standard validator operation"
	case "C":
		return "Acceptable — meets minimum validator requirements"
	case "D":
		return "Marginal — several metrics below recommended levels"
	default:
		return "Insufficient — hardware does not meet validator requirements"
	}
}
