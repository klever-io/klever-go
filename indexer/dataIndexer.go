package indexer

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	nodeData "github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/indexer/workItems"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

type dataIndexer struct {
	isNilIndexer     bool
	dispatcher       DispatcherHandler
	coordinator      sharding.NodesCoordinator
	elasticProcessor ElasticProcessor
	marshalizer      marshal.Marshalizer
	pubkeyConverter  core.PubkeyConverter
}

// NewDataIndexer will create a new data indexer
func NewDataIndexer(arguments ArgDataIndexer) (*dataIndexer, error) {
	err := checkIndexerArgs(arguments)
	if err != nil {
		return nil, err
	}

	dataIndexerObj := &dataIndexer{
		isNilIndexer:     false,
		dispatcher:       arguments.DataDispatcher,
		coordinator:      arguments.NodesCoordinator,
		elasticProcessor: arguments.ElasticProcessor,
		marshalizer:      arguments.Marshalizer,
		pubkeyConverter:  arguments.AddressPubkeyConverter,
	}

	return dataIndexerObj, nil
}

func checkIndexerArgs(arguments ArgDataIndexer) error {
	if check.IfNil(arguments.DataDispatcher) {
		return ErrNilDataDispatcher
	}
	if check.IfNil(arguments.ElasticProcessor) {
		return ErrNilElasticProcessor
	}
	if check.IfNil(arguments.NodesCoordinator) {
		return common.ErrNilNodesCoordinator
	}
	if check.IfNil(arguments.EpochStartNotifier) {
		return common.ErrNilEpochStartNotifier
	}
	if check.IfNil(arguments.Marshalizer) {
		return common.ErrNilMarshalizer
	}

	return nil
}

// SaveBlock saves the block info in the queue to be sent to elastic
func (di *dataIndexer) SaveBlock(args *indexer.ArgsSaveBlockData) {
	wi := workItems.NewItemBlock(
		di.elasticProcessor,
		di.marshalizer,
		args,
	)
	di.dispatcher.Add(wi)
}

// Close will stop goroutine that index data in database
func (di *dataIndexer) Close() error {
	return di.dispatcher.Close()
}

// RevertIndexedBlock will remove from database block and miniblocks
func (di *dataIndexer) RevertIndexedBlock(header nodeData.HeaderHandler) {
	wi := workItems.NewItemRemoveBlock(
		di.elasticProcessor,
		header,
	)
	di.dispatcher.Add(wi)
}

// SaveEpochInfo will save all epoch info to elasticsearch
func (di *dataIndexer) SaveEpochInfo(epoch uint32, validators []kapp.ValidatorAccountInfoHandler) {
	wi := workItems.NewItemEpoch(
		di.elasticProcessor,
		epoch,
		validators,
	)
	di.dispatcher.Add(wi)
}

// SaveAccounts will save the provided accounts
func (di *dataIndexer) UpdateProposalsAndParameters(proposalIDs []string) {
	wi := workItems.NewItemProposals(di.elasticProcessor, proposalIDs)
	di.dispatcher.Add(wi)
}

// SaveAsset will save an asset to elasticsearch
func (di *dataIndexer) SaveAssets(asset []*kapps.KDAData) {
	wi := workItems.NewItemAsset(
		di.elasticProcessor,
		asset,
		di.pubkeyConverter,
	)
	di.dispatcher.Add(wi)
}

// SaveAccounts will save the provided accounts
func (di *dataIndexer) SaveAccounts(timestamp int64, accounts []state.UserAccountHandler) {
	wi := workItems.NewItemAccounts(di.elasticProcessor, timestamp, accounts)
	di.dispatcher.Add(wi)
}

// SavePeersAccounts will save the provided peer accounts
func (di *dataIndexer) SavePeersAccounts(validators []kapp.ValidatorAccountInfoHandler) {
	wi := workItems.NewItemPeersAccounts(di.elasticProcessor, validators)
	di.dispatcher.Add(wi)
}

// IsNilIndexer will return a bool value that signals if the indexer's implementation is a NilIndexer
func (di *dataIndexer) IsNilIndexer() bool {
	return di.isNilIndexer
}

// IsInterfaceNil returns true if there is no value under the interface
func (di *dataIndexer) IsInterfaceNil() bool {
	return di == nil
}
