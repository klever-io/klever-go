package hooks_test

import (
	"encoding/hex"
	"testing"

	"github.com/klever-io/klever-go/core/process/smartContract/hooks"
	"github.com/stretchr/testify/assert"
)

func TestVMCrypto_Ripemd160(t *testing.T) {
	t.Parallel()

	cryptoHook := hooks.NewVMCryptoHook()

	// See https://en.wikipedia.org/wiki/RIPEMD#RIPEMD-160_hashes
	input := []byte("The quick brown fox jumps over the lazy dog")
	expected, _ := hex.DecodeString("37f332f68db77bd9d7edd4969571ad671cf9dd3b")
	result, err := cryptoHook.Ripemd160(input)

	assert.Nil(t, err)
	assert.Equal(t, expected, result)
}

func TestVMCrypto_Ecrecover_ReturnsNotImplemented(t *testing.T) {
	t.Parallel()

	cryptoHook := hooks.NewVMCryptoHook()

	_, err := cryptoHook.Ecrecover(nil, nil, nil, nil)
	assert.Equal(t, hooks.ErrNotImplemented, err)
}
