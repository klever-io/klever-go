package main

// RunMemoryBenchmark measures the memory subsystem characteristics that
// directly affect Kleverchain validator performance:
//
//   - Sequential read bandwidth (GB/s): how fast the CPU can stream data
//     already in RAM — relevant for iterating over large in-memory tries,
//     mempool scanning, and merkle-tree traversal.
//
//   - Sequential write bandwidth (GB/s): sustained write throughput —
//     relevant for bulk state updates during block application.
//
//   - Random read latency (ns): time to dereference a pointer to a random
//     cache-line.  This stresses the DRAM row-access latency and TLB,
//     exposing NUMA issues or severely throttled memory controllers.
//
//   - Allocation throughput (M allocs/s): how fast the Go runtime can
//     satisfy small heap allocations.  Validators create many short-lived
//     objects (transaction structs, trie nodes) per block; a slow allocator
//     or aggressive GC will stall block processing.
//
// Buffer sizes are chosen to exceed typical L3 cache capacity (256 MB for
// read/write tests) so measurements reflect DRAM bandwidth rather than
// cache-hit rates.

import (
	"fmt"
	"math/rand"
	"runtime"
	"strings"
	"time"
	"unsafe"
)

const (
	memBufSizeMB   = 256              // buffer size for bandwidth tests (exceeds L3)
	memBufSize     = memBufSizeMB * 1024 * 1024
	memWordSize    = 8                // uint64 — natural word for 64-bit CPUs
	memRandProbes  = 10_000_000      // pointer-chase probes for latency test
	memAllocDur    = 3 * time.Second  // duration for allocation throughput test
	memAllocSzB    = 64              // bytes per allocation (small object, like a trie node)
)

// MemoryResult holds all memory subsystem performance metrics.
type MemoryResult struct {
	SeqReadGBps   float64 // sequential read bandwidth in GB/s
	SeqWriteGBps  float64 // sequential write bandwidth in GB/s
	RandLatencyNs float64 // random read latency in nanoseconds
	AllocMOpsPerS float64 // heap allocation throughput in millions of allocs/s
}

// RunMemoryBenchmark executes all four memory sub-benchmarks.
func RunMemoryBenchmark() (*MemoryResult, error) {
	result := &MemoryResult{}

	// 1. Sequential read bandwidth --------------------------------------------
	clearLine("  Memory: sequential read (%d MB buffer)...", memBufSizeMB)
	gbps, err := memSeqRead()
	if err != nil {
		return nil, fmt.Errorf("memory seq read: %w", err)
	}
	result.SeqReadGBps = gbps

	// 2. Sequential write bandwidth -------------------------------------------
	clearLine("  Memory: sequential write (%d MB buffer)...", memBufSizeMB)
	gbps, err = memSeqWrite()
	if err != nil {
		return nil, fmt.Errorf("memory seq write: %w", err)
	}
	result.SeqWriteGBps = gbps

	// 3. Random read latency (pointer chase) ----------------------------------
	clearLine("  Memory: random read latency (%d probes)...", memRandProbes)
	latNs, err := memRandLatency()
	if err != nil {
		return nil, fmt.Errorf("memory rand latency: %w", err)
	}
	result.RandLatencyNs = latNs

	// 4. Allocation throughput ------------------------------------------------
	clearLine("  Memory: allocation throughput (%d B allocs, %s)...", memAllocSzB, memAllocDur)
	mops, err := memAllocThroughput()
	if err != nil {
		return nil, fmt.Errorf("memory alloc: %w", err)
	}
	result.AllocMOpsPerS = mops

	fmt.Printf("  %s\r", strings.Repeat(" ", 60))
	return result, nil
}

// ---------------------------------------------------------------------------
// Sequential bandwidth
// ---------------------------------------------------------------------------

// memSeqRead allocates a memBufSize buffer, touches every element once to
// fault the pages in, then times a full sequential read pass.
func memSeqRead() (float64, error) {
	buf := make([]uint64, memBufSize/memWordSize)
	// Warm up: fault all pages in so the measurement is pure read bandwidth.
	for i := range buf {
		buf[i] = uint64(i)
	}
	runtime.GC()

	var sink uint64
	start := time.Now()
	for i := range buf {
		sink += buf[i]
	}
	elapsed := time.Since(start)

	// Prevent the compiler from optimising away the loop.
	if sink == 0 {
		buf[0] = 1
	}

	return float64(memBufSize) / 1e9 / elapsed.Seconds(), nil
}

// memSeqWrite times a full sequential write pass over a pre-faulted buffer.
func memSeqWrite() (float64, error) {
	buf := make([]uint64, memBufSize/memWordSize)
	// Fault pages in.
	for i := range buf {
		buf[i] = uint64(i)
	}
	runtime.GC()

	start := time.Now()
	for i := range buf {
		buf[i] = uint64(i) ^ 0xDEADBEEF
	}
	elapsed := time.Since(start)

	return float64(memBufSize) / 1e9 / elapsed.Seconds(), nil
}

// ---------------------------------------------------------------------------
// Random read latency (pointer chase)
// ---------------------------------------------------------------------------

// node is a single element in the pointer-chase linked list.
// The padding ensures each node occupies exactly one cache line (64 bytes).
type node struct {
	next *node
	_    [64 - unsafe.Sizeof((*node)(nil))]byte
}

// memRandLatency builds a shuffled linked list that spans the full memBufSize
// buffer and measures the average time to follow a single pointer hop.
// Because each hop lands on a random cache line the measurement reflects DRAM
// latency rather than L1/L2/L3 cache hit rates.
func memRandLatency() (float64, error) {
	const nodeSize = 64 // must equal unsafe.Sizeof(node{}) = 64 bytes
	numNodes := memBufSize / nodeSize
	nodes := make([]node, numNodes)

	// Build a random permutation and chain nodes in that order so the CPU
	// cannot prefetch the next pointer before each load completes.
	perm := rand.Perm(numNodes)
	for i := 0; i < numNodes-1; i++ {
		nodes[perm[i]].next = &nodes[perm[i+1]]
	}
	nodes[perm[numNodes-1]].next = &nodes[perm[0]] // close the cycle

	runtime.GC()

	cur := &nodes[perm[0]]
	start := time.Now()
	for range memRandProbes {
		cur = cur.next
	}
	elapsed := time.Since(start)

	// Prevent the compiler from eliminating the loop.
	if cur == nil {
		return 0, nil
	}

	nsPerHop := float64(elapsed.Nanoseconds()) / float64(memRandProbes)
	return nsPerHop, nil
}

// ---------------------------------------------------------------------------
// Allocation throughput
// ---------------------------------------------------------------------------

// memAllocThroughput measures how many memAllocSzB-byte heap allocations per
// second the Go runtime can sustain over memAllocDur.  Allocations are kept
// alive in a ring buffer to force real heap activity and GC pressure.
func memAllocThroughput() (float64, error) {
	const ringSize = 1024
	ring := make([]*[]byte, ringSize)

	deadline := time.Now().Add(memAllocDur)
	var ops int64
	for time.Now().Before(deadline) {
		b := make([]byte, memAllocSzB)
		ring[ops%ringSize] = &b
		ops++
	}

	// Keep ring alive so the compiler cannot eliminate the allocations.
	runtime.KeepAlive(ring)

	return float64(ops) / 1e6 / memAllocDur.Seconds(), nil
}
