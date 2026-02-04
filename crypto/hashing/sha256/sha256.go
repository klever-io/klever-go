package sha256

import (
	"crypto/sha256"
	"sync"

	"github.com/klever-io/klever-go/crypto/hashing"
)

var _ hashing.Hasher = (*Sha256)(nil)

var (
	sha256EmptyHash []byte
	sha256Once      sync.Once
)

// Sha256 is a sha256 implementation of the hasher interface.
type Sha256 struct {
}

// Compute takes a string, and returns the sha256 hash of that string
func (sha Sha256) Compute(s string) []byte {
	if len(s) == 0 {
		return sha.EmptyHash()
	}
	h := sha256.New()
	_, _ = h.Write([]byte(s))
	return h.Sum(nil)
}

// EmptyHash returns the sha256 hash of the empty string
func (sha Sha256) EmptyHash() []byte {
	sha256Once.Do(func() {
		h := sha256.New()
		sha256EmptyHash = h.Sum(nil)
	})
	result := make([]byte, len(sha256EmptyHash))
	copy(result, sha256EmptyHash)
	return result
}

// Size returns the size, in number of bytes, of a sha256 hash
func (Sha256) Size() int {
	return sha256.Size
}

// IsInterfaceNil returns true if there is no value under the interface
func (sha Sha256) IsInterfaceNil() bool {
	return false
}
