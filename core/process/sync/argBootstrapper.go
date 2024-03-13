package sync

import (
	"time"

	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

// ArgBaseBootstrapper holds all dependencies required by the bootstrap data factory in order to create
// new instances
type ArgBaseBootstrapper struct {
	PoolsHolder         retriever.PoolsHolder
	Store               retriever.StorageService
	ChainHandler        data.ChainHandler
	SlotManager         consensus.SlotManager
	BlockProcessor      process.BlockProcessor
	WaitTime            time.Duration
	Hasher              hashing.Hasher
	Marshalizer         marshal.Marshalizer
	ForkDetector        process.ForkDetector
	RequestHandler      process.RequestHandler
	Accounts            state.AccountsAdapter
	BlackListHandler    process.TimeCacher
	NetworkWatcher      process.NetworkConnectionWatcher
	BootStorer          process.BootStorer
	StorageBootstrapper process.BootstrapperFromStorage
	EpochHandler        retriever.EpochHandler
	Uint64Converter     typeConverters.Uint64ByteSliceConverter
	Indexer             process.Indexer
	IsInImportMode      bool
	StartWithInSync     bool
}

// ArgShardBootstrapper holds all dependencies required by the bootstrap data factory in order to create
// new instances of shard bootstrapper
type ArgShardBootstrapper struct {
	ArgBaseBootstrapper
}

// ArgMetaBootstrapper holds all dependencies required by the bootstrap data factory in order to create
// new instances of meta bootstrapper
type ArgMetaBootstrapper struct {
	ArgBaseBootstrapper
	EpochBootstrapper process.EpochBootstrapper
}
