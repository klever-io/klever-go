package factory

import (
	cryptoRand "crypto/rand"
	"io"

	"github.com/klever-io/klever-go/network/p2p/libp2p/rand"
)

// NewRandFactory will create a reader based on the provided seed string
func NewRandFactory(seed string) (io.Reader, error) {
	if len(seed) == 0 {
		return cryptoRand.Reader, nil
	}

	return rand.NewSeedRandReader([]byte(seed))
}

// NewLegacyRandFactory returns a reader using a frozen Go 1.18 PRNG.
// Output is identical regardless of Go compiler version.
// When seed is empty, crypto/rand.Reader is used (non-deterministic).
func NewLegacyRandFactory(seed string) (io.Reader, error) {
	if len(seed) == 0 {
		return cryptoRand.Reader, nil
	}

	return rand.NewLegacySeedRandReader([]byte(seed))
}
