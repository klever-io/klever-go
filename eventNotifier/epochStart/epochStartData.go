package epochStart

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

type epochStartData struct {
	marshalizer marshal.Marshalizer
	hasher      hashing.Hasher
	store       retriever.StorageService
	dataPool    retriever.PoolsHolder
	//blockTracker      process.BlockTracker
	epochStartTrigger process.EpochStartTriggerHandler
	requestHandler    RequestHandler
	genesisEpoch      uint32
}

// ArgsNewEpochStartData defines the input parameters for epoch start data creator
type ArgsNewEpochStartData struct {
	Marshalizer marshal.Marshalizer
	Hasher      hashing.Hasher
	Store       retriever.StorageService
	DataPool    retriever.PoolsHolder
	//BlockTracker      process.BlockTracker
	EpochStartTrigger process.EpochStartTriggerHandler
	RequestHandler    RequestHandler
	GenesisEpoch      uint32
}

// NewEpochStartData creates a new epoch start creator
func NewEpochStartData(args ArgsNewEpochStartData) (*epochStartData, error) {
	if check.IfNil(args.Marshalizer) {
		return nil, process.ErrNilMarshalizer
	}
	if check.IfNil(args.Hasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(args.Store) {
		return nil, common.ErrNilStorage
	}
	if check.IfNil(args.DataPool) {
		return nil, common.ErrNilDataPoolHolder
	}
	if check.IfNil(args.RequestHandler) {
		return nil, common.ErrNilRequestHandler
	}

	e := &epochStartData{
		marshalizer: args.Marshalizer,
		hasher:      args.Hasher,
		store:       args.Store,
		dataPool:    args.DataPool,
		//blockTracker:      args.BlockTracker,
		epochStartTrigger: args.EpochStartTrigger,
		requestHandler:    args.RequestHandler,
		genesisEpoch:      args.GenesisEpoch,
	}

	return e, nil
}

// VerifyEpochStartDataForMetablock verifies if epoch start data given by leader is the same as the one should be created
func (e *epochStartData) VerifyEpochStartDataForMetablock(metaBlock *block.Block) error {
	if !metaBlock.GetIsEpochStart() {
		return nil
	}
	// TODO:
	return nil
}

// IsInterfaceNil returns true if underlying object is nil
func (e *epochStartData) IsInterfaceNil() bool {
	return e == nil
}
