package processor

import (
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/retriever"
)

// ArgTxInterceptorProcessor is the argument for the interceptor processor used for transactions
// (balance txs, reward and so on)
type ArgTxInterceptorProcessor struct {
	TxDataCache           retriever.ShardedDataCacherNotifier
	TxValidator           process.TxValidator
	RequestedItemsHandler retriever.RequestedItemsHandler
}
