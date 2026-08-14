package sharding

import (
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/marshal"
)

// ArgNodesCoordinator holds all dependencies required by the nodes coordinator in order to create new instances
type ArgNodesCoordinator struct {
	ConsensusGroupSize int
	Marshalizer        marshal.Marshalizer
	Hasher             hashing.Hasher
	Shuffler           NodesShuffler
	EpochStartNotifier EpochStartEventNotifier
	BootStorer         storage.Storer
	ElectedNodes       []Validator
	EligibleNodes      []Validator
	SelfPublicKey      []byte
	Epoch              uint32
	StartEpoch         uint32
	// FixJailedPromotionOrderEpoch is the enable epoch for sorting the leaving
	// list before the consensus backfill promotion (enableEpochs.fixJailedPromotionOrder)
	FixJailedPromotionOrderEpoch uint32
	ConsensusGroupCache          Cacher
	ShuffledOutHandler           ShuffledOutHandler
	CurrValidatorsInfo           []*state.ValidatorInfo
	PrevValidatorsInfo           []*state.ValidatorInfo
}
