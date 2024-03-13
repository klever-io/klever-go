package resolverscontainer

import (
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

// FactoryArgs will hold the arguments for ResolversContainerFactory for both shard and meta
type FactoryArgs struct {
	NumConcurrentResolvingJobs int32
	Messenger                  retriever.TopicMessageHandler
	Store                      retriever.StorageService
	Marshalizer                marshal.Marshalizer
	DataPools                  retriever.PoolsHolder
	Uint64ByteSliceConverter   typeConverters.Uint64ByteSliceConverter
	DataPacker                 retriever.DataPacker
	TriesContainer             state.TriesHolder
	InputAntifloodHandler      process.P2PAntifloodHandler
	OutputAntifloodHandler     process.P2PAntifloodHandler
}
