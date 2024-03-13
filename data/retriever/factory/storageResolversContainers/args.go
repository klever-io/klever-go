package storageResolversContainers

import (
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

// FactoryArgs will hold the arguments for ResolversContainerFactory for both shard and meta
type FactoryArgs struct {
	Messenger                retriever.TopicMessageHandler
	Store                    retriever.StorageService
	Marshalizer              marshal.Marshalizer
	Uint64ByteSliceConverter typeConverters.Uint64ByteSliceConverter
	DataPacker               retriever.DataPacker
	ManualEpochStartNotifier retriever.ManualEpochStartNotifier
	ChanGracefullyClose      chan endProcess.ArgEndProcess
	GeneralConfig            config.Config
	ChainID                  string
	WorkingDirectory         string
	Hasher                   hashing.Hasher
	// DisableOldTrieStorageEpoch uint32
	EpochNotifier process.EpochNotifier
}
