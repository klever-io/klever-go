package factory

import (
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/crypto/hashing/blake2b"
	"github.com/klever-io/klever-go/crypto/hashing/keccak"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
)

// NewDefaultHasher -
func NewDefaultHasher() (hashing.Hasher, error) {
	return NewHasher("blake2b")
}

// NewTXSignHasher -
func NewTXSignHasher() (hashing.Hasher, error) {
	return NewHasher("keccak")
}

// NewHasher will return a new instance of hasher based on the value stored in config
func NewHasher(name string) (hashing.Hasher, error) {
	switch name {
	case "sha256":
		return sha256.Sha256{}, nil
	case "keccak":
		return keccak.Keccak{}, nil
	case "blake2b":
		return &blake2b.Blake2b{}, nil
	}

	return nil, ErrNoHasherInConfig
}
