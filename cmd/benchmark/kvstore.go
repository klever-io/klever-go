package main

// RunKVBenchmark measures in-memory key-value store throughput that reflects
// the state-access performance of a Kleverchain validator node.  The store
// mimics LevelDB/BadgerDB access patterns (32-byte hash keys, ~256-byte
// account/state values) using a sync.RWMutex-guarded map so no external
// dependency is needed; results reflect raw Go runtime + memory subsystem
// performance rather than storage-engine specifics.
//
// Three sub-benchmarks are run:
//
//   - Sequential writes (Put): bulk-loading accounts or smart-contract state.
//     Timed as a one-shot insertion of kvNumKeys entries.
//
//   - Random reads (Get): per-transaction account-state lookup.
//     Run for kvBenchDuration seconds; key selection uses a fast LCG to
//     avoid allocation overhead.
//
//   - Mixed workload (80 % Get / 20 % Put, runtime.NumCPU() goroutines):
//     simulates steady-state block processing under concurrent access.
//     Exposes lock-contention characteristics of the runtime scheduler.

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	kvNumKeys       = 100_000        // keys pre-loaded before read/mixed tests
	kvValueSize     = 256            // bytes per value (avg account/state entry)
	kvBenchDuration = 3 * time.Second
)

// KVResult holds all key-value store benchmark metrics.
type KVResult struct {
	WriteOpsPerSec float64 // sequential put throughput (one-shot over kvNumKeys)
	ReadOpsPerSec  float64 // random get throughput (time-bounded)
	MixedOpsPerSec float64 // 80/20 read-write concurrent throughput
	NumKeys        int     // number of keys loaded into the store
}

// kvStore is a minimal thread-safe key-value store backed by a plain map.
type kvStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func newKVStore(capacity int) *kvStore {
	return &kvStore{data: make(map[string][]byte, capacity)}
}

func (s *kvStore) put(k string, v []byte) {
	s.mu.Lock()
	s.data[k] = v
	s.mu.Unlock()
}

func (s *kvStore) get(k string) ([]byte, bool) {
	s.mu.RLock()
	v, ok := s.data[k]
	s.mu.RUnlock()
	return v, ok
}

// RunKVBenchmark executes all three sub-benchmarks and returns a KVResult.
func RunKVBenchmark() (*KVResult, error) {
	store := newKVStore(kvNumKeys)

	// Pre-generate all keys and a shared value to avoid allocation overhead
	// during timing.
	keys := make([]string, kvNumKeys)
	for i := range keys {
		keys[i] = kvKey(i)
	}
	value := make([]byte, kvValueSize)
	for i := range value {
		value[i] = byte(i)
	}

	result := &KVResult{NumKeys: kvNumKeys}

	// 1. Sequential write (one-shot) -------------------------------------------
	clearLine("  KV Store: sequential write (%d keys)...", kvNumKeys)
	wOps, err := kvWriteBench(store, keys, value)
	if err != nil {
		return nil, err
	}
	result.WriteOpsPerSec = wOps

	// 2. Random read (time-bounded) --------------------------------------------
	clearLine("  KV Store: random read (%d keys, %s)...", kvNumKeys, kvBenchDuration)
	rOps, err := kvReadBench(store, keys)
	if err != nil {
		return nil, err
	}
	result.ReadOpsPerSec = rOps

	// 3. Mixed workload (concurrent, 80 % read / 20 % write) ------------------
	clearLine("  KV Store: mixed workload (%d goroutines, %s)...", runtime.NumCPU(), kvBenchDuration)
	mOps, err := kvMixedBench(store, keys, value)
	if err != nil {
		return nil, err
	}
	result.MixedOpsPerSec = mOps

	fmt.Printf("  %s\r", strings.Repeat(" ", 60))
	return result, nil
}

// kvKey returns a fixed-length key string for the given index.
// Using a 32-byte key mimics the hash-sized keys used by state tries.
func kvKey(i int) string {
	return fmt.Sprintf("key_%028d", i) // 4 + 1 + 27 = 32 chars
}

// kvWriteBench times a single sequential pass that inserts all keys.
func kvWriteBench(store *kvStore, keys []string, value []byte) (float64, error) {
	start := time.Now()
	for _, k := range keys {
		store.put(k, value)
	}
	elapsed := time.Since(start)
	return float64(len(keys)) / elapsed.Seconds(), nil
}

// kvReadBench performs random gets for kvBenchDuration using a fast LCG to
// choose keys without requiring a shuffle or extra allocations.
func kvReadBench(store *kvStore, keys []string) (float64, error) {
	n := len(keys)
	deadline := time.Now().Add(kvBenchDuration)
	var ops int64
	var idx uint64 = 1
	for time.Now().Before(deadline) {
		idx = idx*6364136223846793005 + 1442695040888963407 // Knuth multiplicative LCG
		store.get(keys[int(idx>>33)%n])
		ops++
	}
	return float64(ops) / kvBenchDuration.Seconds(), nil
}

// kvMixedBench runs runtime.NumCPU() goroutines each performing 80 % reads and
// 20 % writes for kvBenchDuration to simulate concurrent block-processing load.
func kvMixedBench(store *kvStore, keys []string, value []byte) (float64, error) {
	workers := runtime.NumCPU()
	n := len(keys)
	deadline := time.Now().Add(kvBenchDuration)

	var total atomic.Int64
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		seed := uint64(w*13 + 7)
		go func() {
			defer wg.Done()
			var ops int64
			idx := seed
			var counter uint64
			for time.Now().Before(deadline) {
				idx = idx*6364136223846793005 + 1442695040888963407
				key := keys[int(idx>>33)%n]
				counter++
				if counter%5 == 0 { // 20 % writes
					store.put(key, value)
				} else { // 80 % reads
					store.get(key)
				}
				ops++
			}
			total.Add(ops)
		}()
	}
	wg.Wait()
	return float64(total.Load()) / kvBenchDuration.Seconds(), nil
}
