package blake2b_test

import (
	"sync"
	"testing"

	"github.com/klever-io/klever-go/crypto/hashing/blake2b"
	"github.com/stretchr/testify/assert"
)

func TestBlake2b_ComputeWithDifferentHashSizes(t *testing.T) {
	t.Parallel()

	input := "dummy string"
	sizes := []int{2, 5, 8, 16, 32, 37, 64}
	for _, size := range sizes {
		testComputeOk(t, input, size)
	}
}

func testComputeOk(t *testing.T, input string, size int) {
	hasher := blake2b.Blake2b{HashSize: size}
	res := hasher.Compute(input)
	assert.Equal(t, size, len(res))
}

func TestBlake2b_Empty(t *testing.T) {

	hasher := &blake2b.Blake2b{HashSize: 64}

	var nilStr string
	resNil := hasher.Compute(nilStr)
	assert.Equal(t, 64, len(resNil))

	resEmpty := hasher.Compute("")
	assert.Equal(t, 64, len(resEmpty))

	assert.Equal(t, resEmpty, resNil)

	// force recompute
	hasher = &blake2b.Blake2b{HashSize: 64}

	resEmpty = hasher.Compute("")
	assert.Equal(t, 64, len(resEmpty))

	assert.Equal(t, resEmpty, resNil)
}

// The shared suite in crypto/hashing covers the EmptyHash() entry point for every hasher
// (TestBlake2b_EmptyHashReturnsCopy). This covers the Compute("") entry point, which routes
// through the same cache but is not exercised there.
func TestBlake2b_ComputeEmptyReturnsCopy(t *testing.T) {
	t.Parallel()

	hasher := &blake2b.Blake2b{HashSize: 32}

	first := hasher.Compute("")
	expected := make([]byte, len(first))
	copy(expected, first)

	// a caller mutating the digest it received must not corrupt later results
	for i := range first {
		first[i] ^= 0xff
	}

	assert.Equal(t, expected, hasher.Compute(""))
	assert.Equal(t, expected, hasher.EmptyHash())
}

func TestBlake2b_InvalidHashSizePanicsOnEveryCall(t *testing.T) {
	t.Parallel()

	// blake2b accepts 1..64; an out-of-range size is a programming error and must stay loud.
	// A failed initialization must not be cached: if it were, the first call would panic and
	// every later call would silently hand back a zero-length digest while Size() still
	// reported 65.
	hasher := &blake2b.Blake2b{HashSize: 65}

	assert.Panics(t, func() { _ = hasher.EmptyHash() })
	assert.Panics(t, func() { _ = hasher.EmptyHash() })
	assert.Panics(t, func() { _ = hasher.Compute("") })
}

// The shared suite's testConcurrentEmptyHash calls EmptyHash() once before spawning its
// goroutines, so the cache is already warm and the lazy initialization itself never races.
// That is why it never caught this bug. This test leaves the instance cold so the goroutines
// race the initialization, and mixes both entry points into it.
func TestBlake2b_ConcurrentEmptyInputIsRaceFree(t *testing.T) {
	t.Parallel()

	const numGoroutines = 64

	expected := (&blake2b.Blake2b{HashSize: 32}).Compute("")

	// one instance shared by every goroutine, as NewAuthenticationFunc and the trie do
	hasher := &blake2b.Blake2b{HashSize: 32}

	start := make(chan struct{})
	results := make([][]byte, numGoroutines)

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start

			// exercise both entry points into the lazy initialization
			if idx%2 == 0 {
				results[idx] = hasher.Compute("")
				return
			}
			results[idx] = hasher.EmptyHash()
		}(i)
	}

	close(start)
	wg.Wait()

	for i, res := range results {
		assert.Equal(t, expected, res, "goroutine %d got a different digest", i)
	}

	// fails without -race: the old code handed every caller the same cached slice
	for i := 0; i < numGoroutines; i++ {
		for j := i + 1; j < numGoroutines; j++ {
			assert.NotSame(t, &results[i][0], &results[j][0], "goroutines %d and %d share a backing array", i, j)
		}
	}
}
