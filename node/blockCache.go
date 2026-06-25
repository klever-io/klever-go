package node

import (
	"sync"

	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/tools/check"
)

// blockCache memoizes a value per block, keyed on the chain header nonce: callers within a block share
// one result; the first call after a new block recomputes. The returned pointer is shared — read-only.
type blockCache[T any] struct {
	mu     sync.Mutex
	nonce  uint64
	valid  bool
	cached *T
}

// get returns the cached value, recomputing under lock when the block advanced (so concurrent callers
// on a fresh block share one compute).
func (c *blockCache[T]) get(blkc data.ChainHandler, compute func() (*T, error)) (*T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	currentNonce := uint64(0)
	if header := blkc.GetCurrentBlockHeader(); !check.IfNil(header) {
		currentNonce = header.GetNonce()
	}

	if c.valid && c.nonce == currentNonce && c.cached != nil {
		return c.cached, nil
	}

	computed, err := compute()
	if err != nil {
		return nil, err
	}

	c.cached = computed
	c.nonce = currentNonce
	c.valid = true

	return computed, nil
}
