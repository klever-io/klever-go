package txpool

import (
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/txcache"
)

type txCache interface {
	storage.Cacher

	AddTx(tx *txcache.WrappedTransaction) (ok bool, added bool)
	GetByTxHash(txHash []byte) (*txcache.WrappedTransaction, bool)
	RemoveTxByHash(txHash []byte) (*txcache.WrappedTransaction, bool)
	GetBySenderPaginated(sender []byte, page int, pageSize int) ([]interface{}, int)
	RemoveTxBySenderNonce(sender []byte, nonce uint64) int
	ImmunizeTxsAgainstEviction(keys [][]byte)
	ForEachTransaction(function txcache.ForEachTransaction)
	NumBytes() int
	Diagnose(deep bool)
}
