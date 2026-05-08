package main

import (
	"testing"
)

// excellentResults returns a fully-populated BenchmarkResults whose values
// sit at the excellent ceiling for every category — used to verify the
// max-points totals.
func excellentResults() *BenchmarkResults {
	return &BenchmarkResults{
		GoroutineResult: &GoroutineResult{CPUEfficiency: cpuEffExcellentPct, NumCPU: 8},
		DiskResult: &DiskResult{
			SeqWriteMBps:  seqWriteExcellentMBps,
			SeqReadMBps:   seqReadExcellentMBps,
			RandWriteIOPS: randWriteExcellentIPS,
			RandReadIOPS:  randReadExcellentIPS,
		},
		NetworkResult: &NetworkResult{
			LatP50:         1, // 1 ns → effectively 0 µs (excellent)
			LatP99:         1,
			ThroughputMBps: netThroughputExcellentMBps,
		},
		KVResult: &KVResult{
			WriteOpsPerSec: kvWriteExcellentOps,
			ReadOpsPerSec:  kvReadExcellentOps,
			MixedOpsPerSec: kvMixedExcellentOps,
		},
		MemoryResult: &MemoryResult{
			SeqReadGBps:   memSeqReadExcellentGBps,
			SeqWriteGBps:  memSeqWriteExcellentGBps,
			RandLatencyNs: 1, // ≈ 0 ns → excellent
			AllocMOpsPerS: memAllocExcellentMOps,
		},
		BigNumResult: &BigNumResult{
			ModExpOpsPerSec:  bigModExpExcellentOps,
			ModMulOpsPerSec:  bigModMulExcellentOps,
			Float64OpsPerSec: bigFloat64ExcellentOps,
			IntDivOpsPerSec:  bigIntDivExcellentOps,
		},
		CryptoResult: &CryptoResult{
			SHA256MBps:             cryptoSHA256SmallExcellentMBps,
			SHA256LargeMBps:        cryptoSHA256LargeExcellentMBps,
			Blake2bMBps:            cryptoBlake2bExcellentMBps,
			Keccak256MBps:          cryptoKeccak256ExcellentMBps,
			Ed25519VerifyOpsPerSec: cryptoEd25519VerifyExcellentOps,
			HasSHAAccel:            true,
			HasAVX512IFMA:          true,
		},
	}
}

func TestComputeScore_TotalMaxIs1000WhenAllEnabled(t *testing.T) {
	s := ComputeScore(excellentResults())
	if s.MaxTotal != 1000 {
		t.Fatalf("MaxTotal = %d, want 1000 (rebalance must keep weights summing to 1000)", s.MaxTotal)
	}
	if s.Total != 1000 {
		t.Fatalf("Total = %d, want 1000 with excellent inputs across the board", s.Total)
	}
	if s.Grade != "S" {
		t.Fatalf("Grade = %q, want S with 100%% score", s.Grade)
	}
}

func TestComputeScore_CryptoWeightIs200(t *testing.T) {
	s := ComputeScore(excellentResults())
	if s.Crypto.Max != 200 {
		t.Fatalf("Crypto.Max = %d, want 200", s.Crypto.Max)
	}
	if s.Crypto.Points != 200 {
		t.Fatalf("Crypto.Points = %d, want 200 with excellent inputs", s.Crypto.Points)
	}
}

func TestComputeScore_RebalancedWeights(t *testing.T) {
	s := ComputeScore(excellentResults())
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"Disk", s.Disk.Max, 200},
		{"KV", s.KV.Max, 200},
		{"Crypto", s.Crypto.Max, 200},
		{"Goroutine", s.Goroutine.Max, 150},
		{"Network", s.Network.Max, 100},
		{"Memory", s.Memory.Max, 100},
		{"BigNum", s.BigNum.Max, 50},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s.Max = %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestComputeScore_ThroughputVeto_CapsGradeAtF(t *testing.T) {
	r := excellentResults()
	// Bench measured below the leader-mode floor (e.g., a Skylake/Haswell
	// without SHA-NI typically lands around 250 MB/s).
	r.CryptoResult.SHA256LargeMBps = 250

	s := ComputeScore(r)
	if !s.Vetoed {
		t.Fatal("expected Vetoed=true when SHA-256 16K throughput below the floor")
	}
	if s.Grade != "F" {
		t.Fatalf("Grade = %q, want F when veto applies", s.Grade)
	}
	if s.VetoedReason == "" {
		t.Fatal("expected VetoedReason to be populated")
	}
	// The numeric score should still be substantial — the veto is a grade-cap,
	// not a silent zero. Operators get to see how the rest of the system performs.
	// (The Crypto category itself will score low because of the bad throughput,
	// but the other six categories were set to excellent in this fixture.)
	if s.Total < 700 {
		t.Fatalf("Total = %d, expected non-veto categories to still score normally", s.Total)
	}
}

func TestComputeScore_ThroughputVeto_DoesNotApply_AboveFloor(t *testing.T) {
	r := excellentResults()
	// Throughput just above the floor — veto must not trigger even though
	// the host could in principle be a non-SHA-NI amd64.
	r.CryptoResult.SHA256LargeMBps = minLeaderSHA256MBps + 1
	r.CryptoResult.HasSHAAccel = false

	s := ComputeScore(r)
	if s.Vetoed {
		t.Fatalf("Vetoed must be false when throughput is above floor; reason: %s", s.VetoedReason)
	}
	if s.Grade == "F" {
		t.Fatalf("Grade = F unexpectedly when throughput is above floor")
	}
}

func TestComputeScore_ThroughputVeto_NoCryptoResult_NoVeto(t *testing.T) {
	r := excellentResults()
	r.CryptoResult = nil // crypto bench was skipped

	s := ComputeScore(r)
	if s.Vetoed {
		t.Fatal("Vetoed must be false when CryptoResult is nil (bench skipped)")
	}
}

func TestComputeScore_NilResults_GradeF(t *testing.T) {
	if got := ComputeScore(nil); got.Grade != "F" {
		t.Fatalf("ComputeScore(nil).Grade = %q, want F", got.Grade)
	}
}

func TestComputeScore_AllSkipped_GradeNotApplicable(t *testing.T) {
	s := ComputeScore(&BenchmarkResults{}) // all category results nil
	if s.Grade != "N/A" {
		t.Fatalf("Grade = %q, want N/A when nothing ran", s.Grade)
	}
}

func TestScoreGrade_Boundaries(t *testing.T) {
	cases := []struct {
		pct  float64
		want string
	}{
		{0.95, "S"},
		{0.90, "S"},
		{0.80, "A"},
		{0.65, "B"},
		{0.50, "C"},
		{0.35, "D"},
		{0.10, "F"},
		{0.0, "F"},
	}
	for _, c := range cases {
		if got := scoreGrade(c.pct); got != c.want {
			t.Errorf("scoreGrade(%.2f) = %q, want %q", c.pct, got, c.want)
		}
	}
}
