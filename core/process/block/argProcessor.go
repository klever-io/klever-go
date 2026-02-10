package block

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/statistics"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

// ArgBaseProcessor holds all dependencies required by the process data factory in order to create
// new instances
type ArgBaseProcessor struct {
	AccountsDB              map[state.AccountsDbIdentifier]state.AccountsAdapter
	ForkDetector            process.ForkDetector
	Hasher                  hashing.Hasher
	Marshalizer             marshal.Marshalizer
	Store                   retriever.StorageService
	NodesCoordinator        sharding.NodesCoordinator
	Uint64Converter         typeConverters.Uint64ByteSliceConverter
	RequestHandler          process.RequestHandler
	EpochStartTrigger       process.EpochStartTriggerHandler
	SlotManager             consensus.SlotManager
	BootStorer              process.BootStorer
	DataPool                retriever.PoolsHolder
	BlockChain              data.ChainHandler
	StateCheckpointModulus  uint
	BlockSizeThrottler      process.BlockSizeThrottler
	TpsBenchmark            statistics.TPSBenchmark
	EpochNotifier           process.EpochNotifier
	TxCoordinator           process.TransactionCoordinator
	FeeHandler              process.TransactionFeeHandler
	EventsProcessor         process.EventsProcessor
	BlockChainHook          process.BlockChainHookHandler
	HeaderIntegrityVerifier process.HeaderIntegrityVerifier
	KAppController          kapp.KAppController
	ProcessingMode          core.NodeProcessingMode
}

// ArgMetaProcessor holds all dependencies required by the process data factory in order to create
// new instances of meta processor
type ArgMetaProcessor struct {
	ArgBaseProcessor
	EpochStartDataCreator        process.EpochStartDataCreator
	EconomicsData                process.EconomicsDataHandler
	ValidatorStatisticsProcessor process.ValidatorStatisticsProcessor
	IndexRating                  bool
	ForkController               core.ForkController
}
