package keccak_test

import (
	"testing"

	"github.com/klever-io/klever-go/crypto/hashing/keccak"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeccak_Compute(t *testing.T) {
	t.Parallel()

	k := keccak.Keccak{}
	h1 := k.Compute("test")
	h2 := k.Compute("test")
	h3 := k.Compute("other")

	assert.Equal(t, h1, h2)
	assert.NotEqual(t, h1, h3)
	assert.Equal(t, k.Size(), len(h1))
}

func TestKeccak_ComputeEmpty(t *testing.T) {
	t.Parallel()

	k := keccak.Keccak{}
	h1 := k.Compute("")
	h2 := k.EmptyHash()

	assert.Equal(t, h1, h2)
	assert.Equal(t, k.Size(), len(h1))
}

func TestKeccak_Size(t *testing.T) {
	t.Parallel()

	k := keccak.Keccak{}
	require.Equal(t, 32, k.Size())
}

func TestKeccak_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	k := keccak.Keccak{}
	assert.False(t, k.IsInterfaceNil())
}
