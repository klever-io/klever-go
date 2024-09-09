package processor

import (
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/tools/marshal"
)

// ArgHdrInterceptorProcessor is the argument for the interceptor processor used for headers (shard, meta and so on)
type ArgHdrInterceptorProcessor struct {
	Headers          retriever.HeadersPool
	BlockBlackList   process.TimeCacher
	Marshalizer      marshal.Marshalizer
	Hasher           hashing.Hasher
	WhiteListHandler process.WhiteListHandler
}
