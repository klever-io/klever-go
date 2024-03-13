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
	err := wirb.indexer.RemoveHeader(wirb.headerHandler)
	if err != nil {
		log.Warn("itemRemoveBlock.Save could not remove block", "error", err.Error())
		return err
	}

	blk, ok := wirb.headerHandler.(*block.Block)
	if !ok {
		log.Warn("elasticProcessor.RemoveTransactions body", "error", ErrBodyTypeAssertion.Error())
		return ErrBodyTypeAssertion
	}

	err = wirb.indexer.RemoveTransactions(blk)
	if err != nil {
		log.Warn("itemRemoveBlock.Save could not remove block transactions", "error", err.Error())
		return err
	}

	return nil
}
