package workItems

import (
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
)

type itemRemoveBlock struct {
	indexer       removeIndexer
	headerHandler data.HeaderHandler
}

// NewItemRemoveBlock will create a new instance of itemRemoveBlock
func NewItemRemoveBlock(
	indexer removeIndexer,
	headerHandler data.HeaderHandler,
) WorkItemHandler {
	return &itemRemoveBlock{
		indexer:       indexer,
		headerHandler: headerHandler,
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (wirb *itemRemoveBlock) IsInterfaceNil() bool {
	return wirb == nil
}

// Save will remove a block and miniblocks from elasticsearch database
func (wirb *itemRemoveBlock) Save() error {
	blk, ok := wirb.headerHandler.(*block.Block)
	if !ok {
		log.Warn("elasticProcessor.RemoveTransactions body", "error", ErrBodyTypeAssertion.Error())
		return ErrBodyTypeAssertion
	}

	blockNonce := blk.GetNonce()
	blockTimestamp := blk.GetTimestamp()

	// Step 1: Revert account balances to their previous state
	err := wirb.indexer.RevertAccountBalances(blockTimestamp)
	if err != nil {
		log.Warn("itemRemoveBlock.Save could not revert account balances", "nonce", blockNonce, "error", err.Error())
		return err
	}

	// Step 2: Remove account history entries for this block
	err = wirb.indexer.RemoveAccountsHistory(blockTimestamp)
	if err != nil {
		log.Warn("itemRemoveBlock.Save could not remove accounts history", "nonce", blockNonce, "error", err.Error())
		return err
	}

	// Step 3: Remove transactions
	if len(blk.TxHashes) > 0 {
		log.Info("Rollback: Step 3 - Removing transactions", "count", len(blk.TxHashes), "nonce", blockNonce)
		err = wirb.indexer.RemoveTransactions(blk)
		if err != nil {
			log.Warn("itemRemoveBlock.Save could not remove block transactions", "nonce", blockNonce, "error", err.Error())
			return err
		}
	}

	// Step 4: Remove block header
	err = wirb.indexer.RemoveHeader(wirb.headerHandler)
	if err != nil {
		log.Warn("itemRemoveBlock.Save could not remove block", "nonce", blockNonce, "error", err.Error())
		return err
	}

	return nil
}
