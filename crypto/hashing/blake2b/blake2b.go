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
// HashSize (0 means 256-bit, else 1..64) must not change after construction; must not be copied after first use.
type Blake2b struct {
	HashSize int
	// a mutex, not sync.Once: Once would cache a panicking init and later return an empty digest
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
		// the returned hash.Hash is non-nil but wraps a nil *digest
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
