package bootstrap

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/bootstrap/disabled"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/lrucache"
	"github.com/klever-io/klever-go/tools/marshal"
)

const consensusGroupCacheSize = 50

type syncValidatorStatus struct {
	dataPool           retriever.PoolsHolder
	marshalizer        marshal.Marshalizer
	requestHandler     process.RequestHandler
	nodeCoordinator    StartInEpochNodesCoordinator
	genesisNodesConfig sharding.GenesisNodesSetupHandler
	memDB              storage.Storer
}

// ArgsNewSyncValidatorStatus holds the arguments needed for creating a new validator status process component
type ArgsNewSyncValidatorStatus struct {
	DataPool       retriever.PoolsHolder
	Marshalizer    marshal.Marshalizer
	Hasher         hashing.Hasher
	RequestHandler process.RequestHandler
	//ChanceComputer     sharding.ChanceComputer
	GenesisNodesConfig sharding.GenesisNodesSetupHandler
	NodeShuffler       sharding.NodesShuffler
	PubKey             []byte
}

// NewSyncValidatorStatus creates a new validator status process component
func NewSyncValidatorStatus(args ArgsNewSyncValidatorStatus) (*syncValidatorStatus, error) {
	s := &syncValidatorStatus{
		dataPool:           args.DataPool,
		marshalizer:        args.Marshalizer,
		requestHandler:     args.RequestHandler,
		genesisNodesConfig: args.GenesisNodesConfig,
	}

	electedNodesInfo, eligibleNodesInfo, err := args.GenesisNodesConfig.InitialNodesInfo()
	if err != nil {
		return nil, err
	}

	electedValidators, err := sharding.NodesInfoToValidators(electedNodesInfo)
	if err != nil {
		return nil, err
	}

	eligibleValidators, err := sharding.NodesInfoToValidators(eligibleNodesInfo)
	if err != nil {
		return nil, err
	}

	consensusGroupCache, err := lrucache.NewCache(consensusGroupCacheSize)
	if err != nil {
		return nil, err
	}

	s.memDB = disabled.CreateMemUnit()

	argsNodesCoordinator := sharding.ArgNodesCoordinator{
		ConsensusGroupSize:  int(args.GenesisNodesConfig.GetConsensusGroupSize()),
		Marshalizer:         args.Marshalizer,
		Hasher:              args.Hasher,
		Shuffler:            args.NodeShuffler,
		EpochStartNotifier:  &disabled.EpochStartNotifier{},
		BootStorer:          s.memDB,
		ElectedNodes:        electedValidators,
		EligibleNodes:       eligibleValidators,
		SelfPublicKey:       args.PubKey,
		ConsensusGroupCache: consensusGroupCache,
	}

	// TODO: add changerate
	baseNodesCoordinator, err := sharding.NewNodesCoordinator(argsNodesCoordinator)
	if err != nil {
		return nil, err
	}

	s.nodeCoordinator = baseNodesCoordinator

	return s, nil
}

// NodesConfigFromMetaBlock synces and creates registry from epoch start metablock
func (s *syncValidatorStatus) NodesConfigFromMetaBlock(
	currMetaBlock *block.Block,
	prevMetaBlock *block.Block,
	currEpochStartValidators []*state.ValidatorInfo,
	prevEpochStartValidators []*state.ValidatorInfo,
) (*sharding.NodesCoordinatorRegistry, error) {

	if currMetaBlock.GetNonce() > 1 && !currMetaBlock.GetIsEpochStart() {
		return nil, common.ErrNotEpochStartBlock
	}

	err := s.nodeCoordinator.SetEpochValidatorsInfo(currMetaBlock.GetEpoch(), currEpochStartValidators)
	if err != nil {
		return nil, err
	}

	err = s.processValidatorChangesFor(currMetaBlock)
	if err != nil {
		return nil, err
	}

	err = s.nodeCoordinator.SetEpochValidatorsInfo(prevMetaBlock.GetEpoch(), prevEpochStartValidators)
	if err != nil {
		return nil, err
	}

	err = s.processValidatorChangesFor(prevMetaBlock)
	if err != nil {
		return nil, err
	}

	nodesConfig := s.nodeCoordinator.NodesCoordinatorToRegistry()
	nodesConfig.CurrentEpoch = currMetaBlock.GetEpoch()

	return nodesConfig, nil
}

func (s *syncValidatorStatus) processValidatorChangesFor(metaBlock *block.Block) error {
	if metaBlock.GetEpoch() == 0 {
		// no need to process for genesis - already created
		return nil
	}

	s.nodeCoordinator.EpochStartPrepare(metaBlock)

	return nil
}

// IsInterfaceNil returns true if underlying object is nil
func (s *syncValidatorStatus) IsInterfaceNil() bool {
	return s == nil
}
