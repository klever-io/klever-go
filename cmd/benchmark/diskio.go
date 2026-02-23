package main

// RunDiskBenchmark measures four disk I/O characteristics that directly
// affect Kleverchain validator performance:
//
//   - Sequential write throughput (MB/s)
//     Validator writes full blocks and WAL entries in large sequential bursts.
//
//   - Sequential read throughput (MB/s)
//     Node startup, snapshot loading, and historical queries are sequential.
//
//   - Random write IOPS  (4 KB blocks, one fsync per write)
//     State-trie mutations (LevelDB/BadgerDB SST compaction) generate many
//     small random writes.  Each write is fsynced to simulate durable commits.
//
//   - Random read IOPS  (4 KB blocks)
//     Transaction lookup, account-state retrieval, and peer state queries
//     all result in random 4 KB reads from the trie store.
//
// All writes call f.Sync() to flush data out of the OS page cache and onto
// the physical medium, giving accurate durable-write throughput rather than
// memory-write throughput.  Reads are performed in a shuffled (non-sequential)
// order to defeat readahead prefetch heuristics.

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	seqBlockSize  = 64 * 1024 // 64 KB — typical WAL / SST write unit
	randBlockSize = 4 * 1024  // 4 KB  — page size used by most key-value stores
	numRandOps    = 1_000     // number of random write + read operations
)

// DiskResult holds all four I/O performance metrics.
type DiskResult struct {
	Dir           string
	FileSizeMB    int
	SeqWriteMBps  float64 // sequential write throughput
	SeqReadMBps   float64 // sequential read throughput
	RandWriteIOPS float64 // random write operations per second (with fsync)
	RandReadIOPS  float64 // random read operations per second
}

// RunDiskBenchmark executes all four I/O tests and returns a DiskResult.
// dir is the directory where temporary files are created; it must be on the
// filesystem to be measured.  sizeMB controls the sequential test file size.
func RunDiskBenchmark(dir string, sizeMB int) (*DiskResult, error) {
	if sizeMB <= 0 {
		return nil, fmt.Errorf("invalid sizeMB: must be > 0")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("creating test directory %q: %w", dir, err)
	}

	result := &DiskResult{Dir: dir, FileSizeMB: sizeMB}
	uniqueSuffix := fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
	seqPath := filepath.Join(dir, fmt.Sprintf("klever_bench_seq_%s.tmp", uniqueSuffix))
	randDir := filepath.Join(dir, fmt.Sprintf("klever_bench_rand_%s", uniqueSuffix))

	defer func() {
		err := os.Remove(seqPath)
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: removing sequential test file: %v\n", err)
		}
		err = os.RemoveAll(randDir)
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: removing random I/O test directory: %v\n", err)
		}
	}()

	// 1. Sequential write -------------------------------------------------------
	clearLine("  Sequential write (%d MB in %d KB blocks)...", sizeMB, seqBlockSize/1024)
	mbps, err := seqWrite(seqPath, sizeMB)
	if err != nil {
		return nil, fmt.Errorf("sequential write: %w", err)
	}
	result.SeqWriteMBps = mbps

	// 2. Sequential read --------------------------------------------------------
	clearLine("  Sequential read (%d MB in %d KB blocks)...", sizeMB, seqBlockSize/1024)
	mbps, err = seqRead(seqPath)
	if err != nil {
		return nil, fmt.Errorf("sequential read: %w", err)
	}
	result.SeqReadMBps = mbps

	// 3. Random write IOPS (with fsync) -----------------------------------------
	if err = os.MkdirAll(randDir, 0o750); err != nil {
		return nil, fmt.Errorf("creating random I/O directory: %w", err)
	}
	clearLine("  Random write (%d × %d KB, fsync per op)...", numRandOps, randBlockSize/1024)
	iops, err := randWrite(randDir)
	if err != nil {
		return nil, fmt.Errorf("random write: %w", err)
	}
	result.RandWriteIOPS = iops

	// 4. Random read IOPS -------------------------------------------------------
	clearLine("  Random read (%d × %d KB, shuffled order)...", numRandOps, randBlockSize/1024)
	iops, err = randRead(randDir)
	if err != nil {
		return nil, fmt.Errorf("random read: %w", err)
	}
	result.RandReadIOPS = iops

	// Erase progress line.
	fmt.Fprintf(os.Stderr, "  %s\r", strings.Repeat(" ", 60))
	return result, nil
}

// ---------------------------------------------------------------------------
// Sequential I/O
// ---------------------------------------------------------------------------

// seqWrite creates a file at path and writes sizeMB of random data in
// seqBlockSize chunks, finishing with an fsync.
func seqWrite(path string, sizeMB int) (float64, error) {
	f, err := os.Create(path) // #nosec G304 — path is constructed via filepath.Join from a user-chosen benchmark directory
	if err != nil {
		return 0, err
	}
	defer f.Close()

	block, err := randomBlock(seqBlockSize)
	if err != nil {
		return 0, err
	}
	target := sizeMB * 1024 * 1024
	written := 0

	start := time.Now()
	for written < target {
		remaining := target - written
		chunk := block
		if remaining < len(chunk) {
			chunk = block[:remaining]
		}
		n, werr := f.Write(chunk)
		if werr != nil {
			return 0, werr
		}
		written += n
	}
	if err = f.Sync(); err != nil {
		return 0, err
	}
	elapsed := time.Since(start)

	return float64(written) / (1024 * 1024) / elapsed.Seconds(), nil
}

// seqRead reads an existing file sequentially in seqBlockSize chunks.
func seqRead(path string) (float64, error) {
	f, err := os.Open(path) // #nosec G304 — path is constructed via filepath.Join from a user-chosen benchmark directory
	if err != nil {
		return 0, err
	}
	defer f.Close()

	buf := make([]byte, seqBlockSize)
	var totalRead int64

	start := time.Now()
	for {
		n, rerr := f.Read(buf)
		totalRead += int64(n)
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return 0, rerr
		}
	}
	elapsed := time.Since(start)

	return float64(totalRead) / (1024 * 1024) / elapsed.Seconds(), nil
}

// ---------------------------------------------------------------------------
// Random I/O
// ---------------------------------------------------------------------------

// randWrite creates numRandOps files of randBlockSize bytes, fsyncing each one.
// Files are written in sequential order; the randomness comes from the data
// content and the independent fsync latency of each file.
func randWrite(dir string) (float64, error) {
	block, err := randomBlock(randBlockSize)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	for i := range numRandOps {
		path := filepath.Join(dir, fmt.Sprintf("r%06d.tmp", i))
		f, err := os.Create(path) // #nosec G304 — path is constructed via filepath.Join from a user-chosen benchmark directory
		if err != nil {
			return 0, err
		}
		if _, err = f.Write(block); err != nil {
			_ = f.Close()
			return 0, err
		}
		if err = f.Sync(); err != nil {
			_ = f.Close()
			return 0, err
		}
		_ = f.Close()
	}
	elapsed := time.Since(start)

	return float64(numRandOps) / elapsed.Seconds(), nil
}

// randRead reads numRandOps files in a randomly shuffled order to defeat
// OS readahead prefetching and simulate true random access patterns.
func randRead(dir string) (float64, error) {
	// Fisher-Yates shuffle using crypto/rand for unbiased ordering.
	indices := make([]int, numRandOps)
	for i := range indices {
		indices[i] = i
	}
	if err := shuffle(indices); err != nil {
		return 0, err
	}

	buf := make([]byte, randBlockSize)

	start := time.Now()
	for _, idx := range indices {
		path := filepath.Join(dir, fmt.Sprintf("r%06d.tmp", idx))
		f, err := os.Open(path) // #nosec G304 — path is constructed via filepath.Join from a user-chosen benchmark directory
		if err != nil {
			return 0, err
		}
		_, rerr := io.ReadFull(f, buf)
		_ = f.Close()
		if rerr != nil && rerr != io.ErrUnexpectedEOF {
			return 0, rerr
		}
	}
	elapsed := time.Since(start)

	return float64(numRandOps) / elapsed.Seconds(), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// randomBlock returns a slice of n random bytes.
// Using crypto/rand produces non-compressible data, preventing the filesystem
// or storage layer from applying transparent compression that would inflate
// throughput numbers.
func randomBlock(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, fmt.Errorf("reading random bytes: %w", err)
	}
	return b, nil
}

// shuffle randomly reorders a slice of ints using crypto/rand.
func shuffle(s []int) error {
	for i := len(s) - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return fmt.Errorf("generating random index: %w", err)
		}
		j := int(jBig.Int64())
		s[i], s[j] = s[j], s[i]
	}
	return nil
}

// clearLine prints a progress message that overwrites the current terminal line.
// Output goes to stderr so it never corrupts JSON written to stdout.
func clearLine(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "  %-60s\r", msg)
}
