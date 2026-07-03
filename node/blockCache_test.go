package node

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/stretchr/testify/require"
)

func TestBlockCache_get(t *testing.T) {
	t.Parallel()

	// nil header => sentinel key, so repeated calls stay within the "same block".
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

	t.Run("propagates compute error", func(t *testing.T) {
		t.Parallel()
		c := &blockCache[int]{}
		expectedErr := errors.New("boom")
		v, err := c.get(blkc, func() (*int, error) { return nil, expectedErr })
		require.ErrorIs(t, err, expectedErr)
		require.Nil(t, v)
	})

	t.Run("recomputes when the block nonce advances", func(t *testing.T) {
		t.Parallel()
		nonce := uint64(1)
		bc := &mock.BlockChainMock{GetCurrentBlockHeaderCalled: func() data.HeaderHandler {
			return &mock.HeaderHandlerStub{GetNonceCalled: func() uint64 { return nonce }}
		}}
		calls := 0
		c := &blockCache[int]{}
		compute := func() (*int, error) {
			calls++
			v := int(nonce)
			return &v, nil
		}

		v1, err := c.get(bc, compute)
		require.NoError(t, err)
		require.Equal(t, 1, *v1)

		_, err = c.get(bc, compute) // same nonce => cache hit
		require.NoError(t, err)
		require.Equal(t, 1, calls)

		nonce = 2 // block advanced => recompute
		v2, err := c.get(bc, compute)
		require.NoError(t, err)
		require.Equal(t, 2, *v2)
		require.Equal(t, 2, calls)
	})

	t.Run("recomputes on a reorg at the same nonce", func(t *testing.T) {
		t.Parallel()
		rootHash := []byte("root-a")
		bc := &mock.BlockChainMock{
			GetCurrentBlockHeaderCalled: func() data.HeaderHandler {
				return &mock.HeaderHandlerStub{GetNonceCalled: func() uint64 { return 5 }}
			},
			GetCurrentBlockRootHashCalled: func() []byte { return rootHash },
		}
		calls := 0
		c := &blockCache[int]{}
		compute := func() (*int, error) {
			calls++
			v := calls
			return &v, nil
		}

		_, err := c.get(bc, compute)
		require.NoError(t, err)
		require.Equal(t, 1, calls)

		rootHash = []byte("root-b") // same nonce, different state => recompute
		v, err := c.get(bc, compute)
		require.NoError(t, err)
		require.Equal(t, 2, *v)
		require.Equal(t, 2, calls)
	})

	t.Run("does not serve the no-header value for genesis", func(t *testing.T) {
		t.Parallel()
		var header data.HeaderHandler // no header yet
		bc := &mock.BlockChainMock{
			GetCurrentBlockHeaderCalled:   func() data.HeaderHandler { return header },
			GetCurrentBlockRootHashCalled: func() []byte { return []byte("genesis-root") },
		}
		calls := 0
		c := &blockCache[int]{}
		compute := func() (*int, error) {
			calls++
			v := calls
			return &v, nil
		}

		_, err := c.get(bc, compute)
		require.NoError(t, err)
		require.Equal(t, 1, calls)

		// genesis lands at nonce 0, which must not alias the pre-genesis sentinel
		header = &mock.HeaderHandlerStub{GetNonceCalled: func() uint64 { return 0 }}
		v, err := c.get(bc, compute)
		require.NoError(t, err)
		require.Equal(t, 2, *v)
		require.Equal(t, 2, calls)
	})

	t.Run("computes once under concurrent callers", func(t *testing.T) {
		t.Parallel()
		bc := &mock.BlockChainMock{GetCurrentBlockHeaderCalled: func() data.HeaderHandler { return nil }}
		c := &blockCache[int]{}
		var calls int32
		compute := func() (*int, error) {
			atomic.AddInt32(&calls, 1)
			v := 7
			return &v, nil
		}

		const n = 32
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(n)
		results := make([]*int, n)
		errs := make([]error, n)
		for i := 0; i < n; i++ {
			go func(i int) {
				defer wg.Done()
				<-start
				results[i], errs[i] = c.get(bc, compute)
			}(i)
		}
		close(start) // release all callers at once
		wg.Wait()

		require.Equal(t, int32(1), atomic.LoadInt32(&calls)) // single compute under the lock
		for i := 0; i < n; i++ {
			require.NoError(t, errs[i])
			require.Same(t, results[0], results[i]) // all share the one cached pointer
		}
	})
}
