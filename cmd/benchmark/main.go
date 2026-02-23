package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	logger "github.com/klever-io/klever-go-logger"
)

var log = logger.GetOrCreate("benchmark")

// appVersion is injected at build time via ldflags.
var appVersion = "dev"

func main() {
	var (
		// Goroutine benchmark
		maxWorkers       = flag.Int("goroutines", runtime.NumCPU()*4, "Maximum number of concurrent goroutines to test")
		levelDurationSec = flag.Int("duration", 3, "Duration in seconds for each concurrency level")

		// Disk I/O benchmark
		diskDir  = flag.String("disk-dir", "", "Directory to use for disk I/O tests (default: auto temp dir next to binary)")
		diskSize = flag.Int("disk-size", 256, "Size in MB for sequential read/write test")

		// Control
		skipGoroutine = flag.Bool("skip-goroutine", false, "Skip goroutine scaling benchmark")
		skipDisk      = flag.Bool("skip-disk", false, "Skip disk I/O benchmark")
		skipNetwork   = flag.Bool("skip-network", false, "Skip network TCP loopback benchmark")
		skipKV        = flag.Bool("skip-kv", false, "Skip KV store benchmark")
		skipMemory    = flag.Bool("skip-memory", false, "Skip memory bandwidth and latency benchmark")
		skipBigNum    = flag.Bool("skip-bignum", false, "Skip big-number / FPU benchmark")
		outputFmt     = flag.String("output", "text", "Output format: text or json")
		verbose       = flag.Bool("verbose", false, "Enable verbose logging")
		version       = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *version {
		fmt.Printf("klever-benchmark version %s\n", appVersion)
		os.Exit(0)
	}

	if !*verbose {
		_ = logger.SetLogLevel("*:NONE")
	}

	if *maxWorkers < 1 {
		fmt.Fprintln(os.Stderr, "warning: --goroutines must be >= 1; clamped to 1")
		*maxWorkers = 1
	}
	if *levelDurationSec < 1 {
		fmt.Fprintln(os.Stderr, "warning: --duration must be >= 1; clamped to 1")
		*levelDurationSec = 1
	}
	if *diskSize < 1 {
		fmt.Fprintln(os.Stderr, "error: --disk-size must be >= 1")
		os.Exit(1)
	}

	// If no disk dir was specified and the disk benchmark will run, create a
	// unique temp dir next to the binary and remove it when done.
	if *diskDir == "" && !*skipDisk {
		execPath, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error resolving executable path: %v\n", err)
			os.Exit(1)
		}
		tmp, err := os.MkdirTemp(filepath.Dir(execPath), "bench-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating temp dir: %v\n", err)
			os.Exit(1)
		}
		defer os.RemoveAll(tmp)
		*diskDir = tmp
	}
	if *outputFmt != "text" && *outputFmt != "json" {
		fmt.Fprintln(os.Stderr, "error: --output must be 'text' or 'json'")
		os.Exit(1)
	}

	cfg := Config{
		MaxWorkers:    *maxWorkers,
		LevelDuration: time.Duration(*levelDurationSec) * time.Second,
		DiskDir:       *diskDir,
		DiskSizeMB:    *diskSize,
		SkipGoroutine: *skipGoroutine,
		SkipDisk:      *skipDisk,
		SkipNetwork:   *skipNetwork,
		SkipKV:        *skipKV,
		SkipMemory:    *skipMemory,
		SkipBigNum:    *skipBigNum,
		OutputFmt:     *outputFmt,
	}

	runner := NewRunner(cfg)

	results, err := runner.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	PrintReport(results, cfg.OutputFmt)
}
