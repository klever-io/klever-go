package txpool

import (
	"encoding/json"
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/storage/storageUnit"
)

// ArgShardedTxPool is the argument for ShardedTxPool's constructor
type ArgShardedTxPool struct {
	Config storageUnit.CacheConfig
}

// TODO: Upon further analysis and brainstorming, add some sensible minimum accepted values for the appropriate fields.
func (args *ArgShardedTxPool) verify() error {
	config := args.Config

	if config.SizeInBytes == 0 {
		return fmt.Errorf("%w: config.SizeInBytes is not valid", common.ErrCacheConfigInvalidSizeInBytes)
	}
	if config.SizeInBytesPerSender == 0 {
		return fmt.Errorf("%w: config.SizeInBytesPerSender is not valid", common.ErrCacheConfigInvalidSizeInBytes)
	}
	if config.Capacity == 0 {
		return fmt.Errorf("%w: config.Capacity is not valid", common.ErrCacheConfigInvalidSize)
	}
	if config.SizePerSender == 0 {
		return fmt.Errorf("%w: config.SizePerSender is not valid", common.ErrCacheConfigInvalidSize)
	}
	if config.Shards == 0 {
		return fmt.Errorf("%w: config.Shards (map chunks) is not valid", common.ErrCacheConfigInvalidShards)
	}

	return nil
}

// String returns a readable representation of the object
func (args *ArgShardedTxPool) String() string {
	bytes, err := json.Marshal(args)
	if err != nil {
		log.Error("ArgShardedTxPool.String()", "err", err)
	}

	return string(bytes)
}
