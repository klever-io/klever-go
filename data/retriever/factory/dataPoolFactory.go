package factory

import (
	"fmt"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/retriever/dataPool"
	"github.com/klever-io/klever-go/data/retriever/dataPool/headersCache"
	"github.com/klever-io/klever-go/data/retriever/txpool"
	"github.com/klever-io/klever-go/storage/factory"
	"github.com/klever-io/klever-go/storage/storageUnit"
)

var log = logger.GetOrCreate("retriever/factory")

// ArgsDataPool holds the arguments needed for NewDataPoolFromConfig function
type ArgsDataPool struct {
	Config *config.Config
}

// NewDataPoolFromConfig will return a new instance of a PoolsHolder
func NewDataPoolFromConfig(args ArgsDataPool) (retriever.PoolsHolder, error) {
	log.Debug("creatingDataPool from config")

	if args.Config == nil {
		return nil, common.ErrNilConfig
	}

	mainConfig := args.Config

	txPool, err := txpool.NewShardedTxPool(txpool.ArgShardedTxPool{
		Config: factory.GetCacherFromConfig(mainConfig.TxDataPool),
	})
	if err != nil {
		log.Error("error creating txpool")
		return nil, err
	}

	hdrPool, err := headersCache.NewHeadersPool(mainConfig.HeadersPoolConfig)
	if err != nil {
		log.Error("error creating headers pool")
		return nil, err
	}

	cacherCfg := factory.GetCacherFromConfig(mainConfig.TrieNodesDataPool)
	trieNodes, err := storageUnit.NewCache(cacherCfg)
	if err != nil {
		log.Error("error creating trieNodes")
		return nil, err
	}

	cacherCfg = factory.GetCacherFromConfig(mainConfig.SmartContractDataPool)
	smartContracts, err := storageUnit.NewCache(cacherCfg)
	if err != nil {
		return nil, fmt.Errorf("%w while creating the cache for the smartcontract results", err)
	}

	currBlockTxs, err := dataPool.NewCurrentBlockPool()
	if err != nil {
		return nil, err
	}

	return dataPool.NewDataPool(
		txPool,
		hdrPool,
		trieNodes,
		smartContracts,
		currBlockTxs,
	)
}
