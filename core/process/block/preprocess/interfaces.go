package preprocess

import "github.com/klever-io/klever-go/storage/txcache"

// TxCache defines the functionality for the transactions cache
type TxCache interface {
	SelectTransactions(numRequested int, batchSizePerSender int, bandwidthPerSender uint64) []*txcache.WrappedTransaction
	NotifyAccountNonce(accountKey []byte, nonce uint64)
	IsInterfaceNil() bool
}
