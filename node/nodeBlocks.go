package node

import (
	"encoding/hex"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/node/blockAPI"
)

// GetBlockByHash return the block for a given hash
func (n *Node) GetBlockByHash(hash string, withTxs bool) (*api.Block, error) {
	decodedHash, err := hex.DecodeString(hash)
	if err != nil {
		return nil, err
	}

	apiBlockProcessor := n.createAPIBlockProcessor()
	return apiBlockProcessor.GetBlockByHash(decodedHash, withTxs)
}

// GetBlockByNonce returns the block for a given nonce
func (n *Node) GetBlockByNonce(nonce uint64, withTxs bool) (*api.Block, error) {
	apiBlockProcessor := n.createAPIBlockProcessor()

	return apiBlockProcessor.GetBlockByNonce(nonce, withTxs)
}

func (n *Node) createAPIBlockProcessor() blockAPI.APIBlockHandler {
	blockAPIArgs := &blockAPI.APIBlockProcessorArg{
		Store:                    n.store,
		Marshalizer:              n.internalMarshalizer,
		Uint64ByteSliceConverter: n.uint64ByteSliceConverter,
		UnmarshalTx:              n.unmarshalTransaction,
	}

	return blockAPI.NewAPIBlockProcessor(blockAPIArgs)
}

func (n *Node) GetForkController() core.ForkController {
	return n.forkController
}
