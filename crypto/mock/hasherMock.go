package mock

import (
	"crypto/sha256"
	"sync"
)

var (
	sha256EmptyHash []byte
	sha256Once      sync.Once
)

// HasherMock that will be used for testing
type HasherMock struct {
}

// Compute will output the SHA's equivalent of the input string
func (sha HasherMock) Compute(s string) []byte {
	h := sha256.New()
	_, _ = h.Write([]byte(s))
	return h.Sum(nil)
}

// EmptyHash will return the equivalent of empty string SHA's
func (sha HasherMock) EmptyHash() []byte {
	sha256Once.Do(func() {
		sha256EmptyHash = sha.Compute("")
	})
	result := make([]byte, len(sha256EmptyHash))
	copy(result, sha256EmptyHash)
	return result
}

// Size returns the required size in bytes
func (HasherMock) Size() int {
	return sha256.Size
}

// IsInterfaceNil returns true if there is no value under the interface
func (sha HasherMock) IsInterfaceNil() bool {
	return false
}
