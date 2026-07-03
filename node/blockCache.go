package node

import (
	"fmt"
	"sync"

	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/tools/check"
)

// blockCache memoizes a value per block, keyed on header nonce plus state root: callers within a
// block share one result; a new block — or a reorg swapping the block at the same nonce — triggers
// a recompute. The returned pointer is shared — read-only.
type blockCache[T any] struct {
	mu     sync.Mutex
	key    string
	cached *T
}

// blockCacheKey derives the memoization key. The sentinel keeps the no-header state distinct from
// any real block (genesis also sits at nonce 0).
func blockCacheKey(blkc data.ChainHandler) string {
	header := blkc.GetCurrentBlockHeader()
	if check.IfNil(header) {
		return "no-header"
	}
	return fmt.Sprintf("%d:%x", header.GetNonce(), blkc.GetCurrentBlockRootHash())
}

// get returns the cached value, recomputing under lock when the block changed (so concurrent callers
// on a fresh block share one compute).
func (c *blockCache[T]) get(blkc data.ChainHandler, compute func() (*T, error)) (*T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := blockCacheKey(blkc)
	if c.cached != nil && c.key == key {
		return c.cached, nil
	}

	computed, err := compute()
	if err != nil {
		return nil, err
	}

	c.cached = computed
	c.key = key

	return computed, nil
}
