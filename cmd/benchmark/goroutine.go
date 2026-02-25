package main

// RunGoroutineBenchmark measures how well this machine scales CPU-bound work
// across increasing numbers of goroutines.
//
// Work unit: each goroutine repeatedly computes SHA-256 of a 1 KB block for
// the entire duration of its level.  SHA-256 is chosen because it closely
// mirrors the cryptographic primitives a Kleverchain validator runs on every
// block (transaction signature verification, merkle-tree hashing, BLS
// operations).
//
// Metric — "Scaling Efficiency" at N workers:
//
//	efficiency = throughput(N) / (N × throughput(1))
//
// A perfectly parallel workload reaches 100 %.  Real machines typically
// saturate between 70–90 % at numCPU workers, then drop sharply once
// goroutines start contending for CPU time.
//
// The benchmark tests at exponential power-of-two levels and also at exactly
// runtime.NumCPU() and MaxWorkers so the report always shows the inflection
// point that matters most for validators.

import (
	"crypto/sha256"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// GoroutineLevel holds the result for a single concurrency level.
type GoroutineLevel struct {
	Workers    int
	OpsPerSec  float64       // SHA-256 hashes completed per second
	AvgLatency time.Duration // average time per hash
	Efficiency float64       // throughput / (workers × baseline_throughput)
}

// GoroutineResult aggregates the scaling curve across all tested levels.
type GoroutineResult struct {
	Levels        []GoroutineLevel
	NumCPU        int
	BaselineOps   float64 // ops/s at 1 worker
	PeakOps       float64 // highest ops/s observed
	CPUEfficiency float64 // efficiency at exactly numCPU workers (key verdict metric)
}

// hashDataSize is the size of the block hashed by each goroutine per operation.
const hashDataSize = 1024 // 1 KB

// RunGoroutineBenchmark runs the goroutine scalability benchmark.
// maxWorkers caps the highest concurrency level tested.
// levelDuration is how long each concurrency level runs before moving on.
func RunGoroutineBenchmark(maxWorkers int, levelDuration time.Duration) (*GoroutineResult, error) {
	levels := buildConcurrencyLevels(maxWorkers)

	// Seed data: non-zero, non-compressible.
	data := make([]byte, hashDataSize)
	for i := range data {
		data[i] = byte(i*7 + 13)
	}

	result := &GoroutineResult{
		NumCPU: runtime.NumCPU(),
		Levels: make([]GoroutineLevel, 0, len(levels)),
	}

	// Warm up the runtime (avoid JIT / scheduler cold-start bias on level 1).
	_ = hashLoop(1, 500*time.Millisecond, data)

	var baseline float64

	for i, n := range levels {
		fmt.Printf("  [%d/%d] Testing %d goroutine(s)...   \r", i+1, len(levels), n)

		ops := hashLoop(n, levelDuration, data)
		opsPerSec := float64(ops) / levelDuration.Seconds()

		if baseline == 0 || n == 1 {
			baseline = opsPerSec
		}

		var efficiency float64
		if n == 1 {
			efficiency = 1.0
		} else {
			expected := baseline * float64(n)
			if expected > 0 {
				efficiency = opsPerSec / expected
			}
		}

		var avgLat time.Duration
		if ops > 0 {
			avgLat = time.Duration(int64(levelDuration) / ops)
		}

		lv := GoroutineLevel{
			Workers:    n,
			OpsPerSec:  opsPerSec,
			AvgLatency: avgLat,
			Efficiency: efficiency,
		}
		result.Levels = append(result.Levels, lv)

		if opsPerSec > result.PeakOps {
			result.PeakOps = opsPerSec
		}
		if n == runtime.NumCPU() {
			result.CPUEfficiency = efficiency
		}
	}

	// If numCPU was not an exact tested level, interpolate from the nearest.
	if result.CPUEfficiency == 0 && len(result.Levels) > 0 {
		result.CPUEfficiency = nearestEfficiency(result.Levels, runtime.NumCPU())
	}

	// Erase the progress line.
	fmt.Fprintf(os.Stderr, "  %s\r", strings.Repeat(" ", 50))

	result.BaselineOps = baseline
	return result, nil
}

// hashLoop spawns n goroutines that each compute SHA-256 hashes for the given
// duration, then returns the total number of hashes completed.
func hashLoop(n int, duration time.Duration, data []byte) int64 {
	deadline := time.Now().Add(duration)
	var total atomic.Int64
	var wg sync.WaitGroup

	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var count int64
			for time.Now().Before(deadline) {
				_ = sha256.Sum256(data)
				count++
			}
			total.Add(count)
		}()
	}

	wg.Wait()
	return total.Load()
}

// buildConcurrencyLevels produces a sorted, deduplicated list of worker counts
// to benchmark: powers of two plus runtime.NumCPU() and maxWorkers.
func buildConcurrencyLevels(max int) []int {
	seen := make(map[int]bool)
	add := func(n int) {
		if n >= 1 && n <= max {
			seen[n] = true
		}
	}

	// Powers of two: 1, 2, 4, 8, …
	for n := 1; n <= max; n <<= 1 {
		add(n)
	}

	// Always include numCPU and max.
	add(runtime.NumCPU())
	add(max)

	levels := make([]int, 0, len(seen))
	for n := range seen {
		levels = append(levels, n)
	}
	sort.Ints(levels)
	return levels
}

// nearestEfficiency finds the efficiency of the level closest to target.
func nearestEfficiency(levels []GoroutineLevel, target int) float64 {
	best := levels[0]
	for _, lv := range levels[1:] {
		if abs(lv.Workers-target) < abs(best.Workers-target) {
			best = lv
		}
	}
	return best.Efficiency
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
