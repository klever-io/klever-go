package hashing_test

import (
	"sync"
	"testing"

	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/hashing/blake2b"
	"github.com/klever-io/klever-go/crypto/hashing/fnv"
	"github.com/klever-io/klever-go/crypto/hashing/keccak"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/stretchr/testify/assert"
)

func TestSha256(t *testing.T) {
	Suite(t, sha256.Sha256{})
}

func TestBlake2b(t *testing.T) {
	Suite(t, &blake2b.Blake2b{})
}

func TestKeccak(t *testing.T) {
	Suite(t, keccak.Keccak{})
}

func TestFnv(t *testing.T) {
	Suite(t, fnv.Fnv{})
}

func Suite(t *testing.T, h hashing.Hasher) {
	testNilInterface(t, h)
	testSize(t, h)
	testCalculateHash(t, h)
	testCalculateEmptyHash(t, h)
	testNilReturn(t, h)
}

func testNilInterface(t *testing.T, h hashing.Hasher) {
	res := h.IsInterfaceNil()

	assert.False(t, res)
}

func testSize(t *testing.T, h hashing.Hasher) {
	input := "test"
	res := h.Compute(input)
	hasherSize := h.Size()

	assert.Equal(t, hasherSize, len(res))
}

func testCalculateHash(t *testing.T, h hashing.Hasher) {
	h1 := h.Compute("a")
	h2 := h.Compute("b")

	assert.NotEqual(t, h1, h2)
}

func testCalculateEmptyHash(t *testing.T, h hashing.Hasher) {
	h1 := h.Compute("")
	h2 := h.EmptyHash()

	assert.Equal(t, h1, h2)
	assert.Equal(t, h.Size(), len(h1))
}

func testNilReturn(t *testing.T, h hashing.Hasher) {
	h1 := h.Compute("a")
	assert.NotNil(t, h1)
}

func TestKeccak_ConcurrentEmptyHash(t *testing.T) {
	t.Parallel()
	testConcurrentEmptyHash(t, keccak.Keccak{})
}

func TestSha256_ConcurrentEmptyHash(t *testing.T) {
	t.Parallel()
	testConcurrentEmptyHash(t, sha256.Sha256{})
}

func TestFnv_ConcurrentEmptyHash(t *testing.T) {
	t.Parallel()
	testConcurrentEmptyHash(t, fnv.Fnv{})
}

func testConcurrentEmptyHash(t *testing.T, h hashing.Hasher) {
	expected := h.EmptyHash()
	goroutines := 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	results := make([][]byte, goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			results[idx] = h.EmptyHash()
		}(i)
	}

	wg.Wait()

	for i, result := range results {
		assert.Equal(t, expected, result, "goroutine %d returned different hash", i)
	}
}

func TestKeccak_EmptyHashReturnsCopy(t *testing.T) {
	t.Parallel()
	testEmptyHashReturnsCopy(t, keccak.Keccak{})
}

func TestSha256_EmptyHashReturnsCopy(t *testing.T) {
	t.Parallel()
	testEmptyHashReturnsCopy(t, sha256.Sha256{})
}

func TestFnv_EmptyHashReturnsCopy(t *testing.T) {
	t.Parallel()
	testEmptyHashReturnsCopy(t, fnv.Fnv{})
}

func testEmptyHashReturnsCopy(t *testing.T, h hashing.Hasher) {
	h1 := h.EmptyHash()
	original := make([]byte, len(h1))
	copy(original, h1)

	// mutate the returned slice
	for i := range h1 {
		h1[i] = 0xFF
	}

	// subsequent call should return the correct hash, not the mutated one
	h2 := h.EmptyHash()
	assert.Equal(t, original, h2)
	assert.NotEqual(t, h1, h2)
}
