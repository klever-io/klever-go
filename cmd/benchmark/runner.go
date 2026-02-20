package main

import (
	"fmt"
	"runtime"
	"time"
)

// Config holds all benchmark configuration parameters.
type Config struct {
	MaxWorkers    int
	LevelDuration time.Duration
	DiskDir       string
	DiskSizeMB    int
	SkipGoroutine bool
	SkipDisk      bool
	SkipNetwork   bool
	SkipKV        bool
	SkipMemory    bool
	SkipBigNum    bool
	OutputFmt     string
}

// Runner coordinates the execution of all configured benchmarks.
type Runner struct {
	cfg Config
}

// NewRunner creates a Runner from the given config.
func NewRunner(cfg Config) *Runner {
	return &Runner{cfg: cfg}
}

// Run executes every enabled benchmark and returns the combined results.
func (r *Runner) Run() (*BenchmarkResults, error) {
	results := &BenchmarkResults{
		RunAt: time.Now(),
		SystemInfo: SystemInfo{
			CPUs:      runtime.NumCPU(),
			GOOS:      runtime.GOOS,
			GOARCH:    runtime.GOARCH,
			GoVersion: runtime.Version(),
		},
	}

	if !r.cfg.SkipNetwork {
		fmt.Printf("Running network benchmark (TCP loopback latency + throughput)...\n")
		nr, err := RunNetworkBenchmark()
		if err != nil {
			return nil, fmt.Errorf("network benchmark: %w", err)
		}
		results.NetworkResult = nr
	}

	if !r.cfg.SkipKV {
		fmt.Printf("Running KV store benchmark (%d keys, 80/20 read-write mixed)...\n", kvNumKeys)
		kr, err := RunKVBenchmark()
		if err != nil {
			return nil, fmt.Errorf("kv benchmark: %w", err)
		}
		results.KVResult = kr
	}

	if !r.cfg.SkipGoroutine {
		fmt.Printf("Running goroutine scalability benchmark (max %d workers, %s/level)...\n",
			r.cfg.MaxWorkers, r.cfg.LevelDuration)
		gr, err := RunGoroutineBenchmark(r.cfg.MaxWorkers, r.cfg.LevelDuration)
		if err != nil {
			return nil, fmt.Errorf("goroutine benchmark: %w", err)
		}
		results.GoroutineResult = gr
	}

	if !r.cfg.SkipDisk {
		fmt.Printf("Running disk I/O benchmark (dir: %s, size: %d MB)...\n",
			r.cfg.DiskDir, r.cfg.DiskSizeMB)
		dr, err := RunDiskBenchmark(r.cfg.DiskDir, r.cfg.DiskSizeMB)
		if err != nil {
			return nil, fmt.Errorf("disk benchmark: %w", err)
		}
		results.DiskResult = dr
	}

	if !r.cfg.SkipMemory {
		fmt.Printf("Running memory benchmark (%d MB buffer, %s alloc test)...\n",
			memBufSizeMB, memAllocDur)
		mr, err := RunMemoryBenchmark()
		if err != nil {
			return nil, fmt.Errorf("memory benchmark: %w", err)
		}
		results.MemoryResult = mr
	}

	if !r.cfg.SkipBigNum {
		fmt.Printf("Running big-number benchmark (2048-bit modexp/mulmul, float64, %s)...\n",
			bigNumDuration)
		br, err := RunBigNumBenchmark()
		if err != nil {
			return nil, fmt.Errorf("bignum benchmark: %w", err)
		}
		results.BigNumResult = br
	}

	return results, nil
}
