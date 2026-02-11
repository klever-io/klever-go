package websocket

import "github.com/klever-io/klever-go/data/api"

type WSFacade interface {
	GetTransaction(hash string, withResults bool) (*api.Transaction, error)
	GetBlockByHash(hash string, withTxs bool) (*api.Block, error)
	GetBlockByNonce(nonce uint64, withTxs bool) (*api.Block, error)
}
