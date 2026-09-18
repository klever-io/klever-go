package blake2b

import (
	"fmt"
	"hash"
	"sync"

	"github.com/klever-io/klever-go/crypto/hashing"
	"golang.org/x/crypto/blake2b"
)

var _ hashing.Hasher = (*Blake2b)(nil)

// Blake2b is a blake2b implementation of the hasher interface.
//
// HashSize must be 0 (meaning blake2b-256) or in the range 1..64, and must be set at
// construction and never mutated afterwards — it is read without synchronization. Given that,
// the zero value is ready to use and a single instance is safe for concurrent use. Instances
// must not be copied after first use.
type Blake2b struct {
	HashSize int
	// emptyHash caches the digest of the empty string. It is computed lazily because it
	// depends on HashSize, which is only known once the caller has built the struct.
	// emptyMut guards it, and the cached digest is never handed out directly, only copied,
	// so a caller mutating a result cannot corrupt later calls.
	//
	// A mutex rather than a sync.Once: sync.Once marks itself done even when its function
	// panics, so an out-of-range HashSize would panic on the first call and then silently
	// yield a zero-length digest on every call after. Re-checking under a mutex keeps that
	// misconfiguration loud on every call.
	emptyMut  sync.Mutex
	emptyHash []byte
}

func (b2b *Blake2b) getHasher() hash.Hash {
	if b2b.HashSize == 0 {
		h, _ := blake2b.New256(nil)
		return h
	}

	h, err := blake2b.New(b2b.HashSize, nil)
	if err != nil {
		// blake2b.New returns a nil *digest boxed in a non-nil hash.Hash alongside the
		// error, so returning it would nil-dereference inside Sum. Fail with the cause.
		panic(fmt.Sprintf("blake2b: invalid HashSize %d: %v", b2b.HashSize, err))
	}

	return h
}

// Compute takes a string, and returns the blake2b hash of that string
func (b2b *Blake2b) Compute(s string) []byte {
	if len(s) == 0 {
		return b2b.EmptyHash()
	}

	h := b2b.getHasher()
	_, _ = h.Write([]byte(s))
	return h.Sum(nil)
}

// EmptyHash returns the blake2b hash of the empty string
func (b2b *Blake2b) EmptyHash() []byte {
	b2b.emptyMut.Lock()
	defer b2b.emptyMut.Unlock()

	if len(b2b.emptyHash) == 0 {
		b2b.emptyHash = b2b.getHasher().Sum(nil)
	}

	result := make([]byte, len(b2b.emptyHash))
	copy(result, b2b.emptyHash)
	return result
}

// Size returns the size, in number of bytes, of a blake2b hash
func (b2b *Blake2b) Size() int {
	if b2b.HashSize == 0 {
		return blake2b.Size256
	}

	return b2b.HashSize
}

// IsInterfaceNil returns true if there is no value under the interface
func (b2b *Blake2b) IsInterfaceNil() bool {
	return b2b == nil
}
