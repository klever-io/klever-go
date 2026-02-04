package sha256_test

import (
	"testing"

	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSha256_Compute(t *testing.T) {
	t.Parallel()

	s := sha256.Sha256{}
	h1 := s.Compute("test")
	h2 := s.Compute("test")
	h3 := s.Compute("other")

	assert.Equal(t, h1, h2)
	assert.NotEqual(t, h1, h3)
	assert.Equal(t, s.Size(), len(h1))
}

func TestSha256_ComputeEmpty(t *testing.T) {
	t.Parallel()

	s := sha256.Sha256{}
	h1 := s.Compute("")
	h2 := s.EmptyHash()

	assert.Equal(t, h1, h2)
	assert.Equal(t, s.Size(), len(h1))
}

func TestSha256_Size(t *testing.T) {
	t.Parallel()

	s := sha256.Sha256{}
	require.Equal(t, 32, s.Size())
}

func TestSha256_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	s := sha256.Sha256{}
	assert.False(t, s.IsInterfaceNil())
}
