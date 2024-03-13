package pruning

import (
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/storageUnit"
)

// StorerArgs will hold the arguments needed for PruningStorer
type StorerArgs struct {
	Identifier                string
	CacheConf                 storageUnit.CacheConfig
	PathManager               storage.PathManagerHandler
	DbPath                    string
	PersisterFactory          DbFactoryHandler
	BloomFilterConf           storageUnit.BloomConfig
	Notifier                  EpochStartNotifier
	MaxBatchSize              int
	NumOfEpochsToKeep         uint32
	NumOfActivePersisters     uint32
	StartingEpoch             uint32
	PruningEnabled            bool
	CleanOldEpochsData        bool
	EnabledDbLookupExtensions bool
}
