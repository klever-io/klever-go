package processor

import (
	"github.com/klever-io/klever-go/data"
)

// InterceptedTransactionHandler defines an intercepted data wrapper over transaction handler that has
// receiver and sender shard getters
type InterceptedTransactionHandler interface {
	SenderAddress() []byte
	Nonce() uint64
	Fee() int64
	KDAFee() data.KDAFeeHandler
	Transaction() data.TransactionHandler
	PermissionID() int32
	ValidatePermissionOperation([]byte) error
	Signature() [][]byte
}

// TXDataPool is a perspective of the data pool
type TXDataPool interface {
	AddData(key []byte, data interface{}, sizeInBytes int, cacheID string)
	Notify(txHash []byte, value interface{}, sizeInBytes int)
}
