package blockAPI

import (
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

// APIBlockProcessorArg is structure that store components that are needed to create an api block procesosr
type APIBlockProcessorArg struct {
	Store                    retriever.StorageService
	Marshalizer              marshal.Marshalizer
	Uint64ByteSliceConverter typeConverters.Uint64ByteSliceConverter
	//HistoryRepo              dblookupext.HistoryRepository
	UnmarshalTx func(txBytes []byte) (*api.Transaction, error)
}
