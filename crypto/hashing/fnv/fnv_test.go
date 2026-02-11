package fnv_test

import (
	"testing"

	"github.com/klever-io/klever-go/crypto/hashing/fnv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFnv_Compute(t *testing.T) {
	t.Parallel()

	f := fnv.Fnv{}
	h1 := f.Compute("test")
	h2 := f.Compute("test")
	h3 := f.Compute("other")

	assert.Equal(t, h1, h2)
	assert.NotEqual(t, h1, h3)
	assert.Equal(t, f.Size(), len(h1))
}

func TestFnv_ComputeEmpty(t *testing.T) {
	t.Parallel()

	f := fnv.Fnv{}
	h1 := f.Compute("")
	h2 := f.EmptyHash()

	assert.Equal(t, h1, h2)
	assert.Equal(t, f.Size(), len(h1))
}

func TestFnv_Size(t *testing.T) {
	t.Parallel()

	f := fnv.Fnv{}
	require.Equal(t, 16, f.Size())
}

func TestFnv_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	f := fnv.Fnv{}
	assert.False(t, f.IsInterfaceNil())
}
