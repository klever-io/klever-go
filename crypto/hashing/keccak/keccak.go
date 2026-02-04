package keccak

import (
	"sync"

	"github.com/klever-io/klever-go/crypto/hashing"
	"golang.org/x/crypto/sha3"
)

var _ hashing.Hasher = (*Keccak)(nil)

var (
	keccakEmptyHash []byte
	keccakOnce      sync.Once
)

// Keccak is a sha3-Keccak implementation of the hasher interface.
type Keccak struct {
}

// Compute takes a string, and returns the sha3-Keccak hash of that string
func (k Keccak) Compute(s string) []byte {
	if len(s) == 0 {
		return k.EmptyHash()
	}
	h := sha3.NewLegacyKeccak256()
	_, _ = h.Write([]byte(s))
	return h.Sum(nil)
}

// EmptyHash returns the sha3-Keccak hash of the empty string
func (k Keccak) EmptyHash() []byte {
	keccakOnce.Do(func() {
		h := sha3.NewLegacyKeccak256()
		keccakEmptyHash = h.Sum(nil)
	})
	result := make([]byte, len(keccakEmptyHash))
	copy(result, keccakEmptyHash)
	return result
}

// Size returns the size, in number of bytes, of a sha3-Keccak hash
func (Keccak) Size() int {
	return sha3.NewLegacyKeccak256().Size()
}

// IsInterfaceNil returns true if there is no value under the interface
func (k Keccak) IsInterfaceNil() bool {
	return false
}
