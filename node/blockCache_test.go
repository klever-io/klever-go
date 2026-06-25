package node

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/stretchr/testify/require"
)

func TestBlockCache_get(t *testing.T) {
	t.Parallel()

	// nil header => nonce 0, so repeated calls stay within the "same block".
	blkc := &mock.BlockChainMock{GetCurrentBlockHeaderCalled: func() data.HeaderHandler { return nil }}

	t.Run("memoizes within the same block", func(t *testing.T) {
		t.Parallel()
		calls := 0
		c := &blockCache[int]{}
		compute := func() (*int, error) {
			calls++
			v := 42
			return &v, nil
		}
		v1, err := c.get(blkc, compute)
		require.NoError(t, err)
		require.Equal(t, 42, *v1)

		v2, err := c.get(blkc, compute)
		require.NoError(t, err)
		require.Same(t, v1, v2) // served from cache
		require.Equal(t, 1, calls)
	})

	t.Run("nil compute result is an error", func(t *testing.T) {
		t.Parallel()
		c := &blockCache[int]{}
		v, err := c.get(blkc, func() (*int, error) { return nil, nil })
		require.ErrorIs(t, err, errNilComputeResult)
		require.Nil(t, v)
	})

	t.Run("propagates compute error", func(t *testing.T) {
		t.Parallel()
		c := &blockCache[int]{}
		expectedErr := errors.New("boom")
		v, err := c.get(blkc, func() (*int, error) { return nil, expectedErr })
		require.ErrorIs(t, err, expectedErr)
		require.Nil(t, v)
	})
}
