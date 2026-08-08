package sharding

import (
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/eventNotifier"
)

// GenesisNodeInfoHandler defines the public methods for the genesis nodes info
type GenesisNodeInfoHandler interface {
	AddressBytes() []byte
	PubKeyBytes() []byte
	GetInitialRating() uint32
	IsInterfaceNil() bool
}

// Validator defines a node that can be allocated to a shard for participation in a consensus group as validator
// or block proposer
type Validator interface {
	OwnerAddress() []byte
	PubKey() []byte
	Chances() uint32
	Index() uint32
	Size() int
}

// NodesCoordinator defines the behavior of a struct able to do validator group selection
type NodesCoordinator interface {
	ComputeConsensusGroup(randomness []byte, slot uint64, epoch uint32) (validatorsGroup []Validator, err error)
	GetValidatorWithPublicKey(publicKey []byte) (validator Validator, err error)
	ConsensusGroupSize() int
	LoadState(key []byte) error
	IsReady() bool
	LoadStateFailed() bool
	GetSavedStateKey() []byte
	GetOwnPublicKey() []byte
	GetConsensusWhitelistedNodes(epoch uint32) (map[string]struct{}, error)
	GetAllElectedValidatorsKeys(epoch uint32, ownerKey bool) ([][]byte, error)
	GetAllEligibleValidatorsKeys(epoch uint32, ownerKey bool) ([][]byte, error)
	GetAllWaitingValidatorsKeys(epoch uint32, ownerKey bool) ([][]byte, error)
	SetEpochValidatorsInfo(epoch uint32, validatorsInfo []*state.ValidatorInfo) error
	CheckValidatorSlot(epoch uint32, slotIndex int64, pubkey []byte) bool
	GetConsensusValidatorsPublicKeys(randomness []byte, slot uint64, epoch uint32) ([]string, error)
	IsInterfaceNil() bool
}

// RandomSelector selects randomly a subset of elements from a set of data
type RandomSelector interface {
	Select(randSeed []byte, sampleSize uint32) ([]uint32, error)
	IsInterfaceNil() bool
}

// Cacher provides the capabilities needed to store and retrieve information needed in the NodesCoordinator
type Cacher interface {
	// Clear is used to completely clear the cache.
	Clear()
	// Put adds a value to the cache.  Returns true if an eviction occurred.
	Put(key []byte, value interface{}, sizeInBytes int) (evicted bool)
	// Get looks up a key's value from the cache.
	Get(key []byte) (value interface{}, ok bool)
}

// EpochHandler defines what a component which handles current epoch should be able to do
type EpochHandler interface {
	Epoch() uint32
	IsInterfaceNil() bool
}

// EpochStartEventNotifier provides Register and Unregister functionality for the end of epoch events
type EpochStartEventNotifier interface {
	RegisterHandler(handler eventNotifier.ActionHandler)
	UnregisterHandler(handler eventNotifier.ActionHandler)
	IsInterfaceNil() bool
}

// ArgsUpdateNodes holds the parameters required by the shuffler to generate a new nodes configuration
type ArgsUpdateNodes struct {
	Elected  []Validator
	Eligible []Validator
	Rand     []byte
	Epoch    uint32
}

// ResUpdateNodes holds the result of the UpdateNodes method
type ResUpdateNodes struct {
	Elected  []Validator
	Eligible []Validator
}

// NodesShuffler provides shuffling functionality for nodes
type NodesShuffler interface {
	UpdateParams(nodesMeta uint32)
	UpdateNodeLists(args ArgsUpdateNodes) (*ResUpdateNodes, error)
	IsInterfaceNil() bool
}

// ShuffledOutHandler defines the methods needed for the computation of a shuffled out event
type ShuffledOutHandler interface {
	Process() error
	RegisterHandler(handler func())
	IsInterfaceNil() bool
}

// GenesisNodesSetupHandler returns the genesis nodes info
type GenesisNodesSetupHandler interface {
	InitialNodesInfo() ([]GenesisNodeInfoHandler, []GenesisNodeInfoHandler, error)
	GetStartTime() int64
	GetSlotInterval() uint64
	GetChainID() string
	GetMinTransactionVersion() uint32
	GetConsensusGroupSize() uint32
	MinNumberOfNodes() uint32
	GetSlotsPerEpoch() uint64
	IsInterfaceNil() bool
}

// EpochStartActionHandler defines the action taken on epoch start event
type EpochStartActionHandler interface {
	EpochStartAction(hdr data.HeaderHandler)
	EpochStartPrepare(metaHdr data.HeaderHandler)
	NotifyOrder() uint32
}

// PeerAccountListAndRatingHandler provides Rating Computation Capabilites for the Nodes Coordinator and ValidatorStatistics
type PeerAccountListAndRatingHandler interface {
	//GetChance returns the chances for the the rating
	GetChance(uint32) uint32
	//GetStartRating gets the start rating values
	GetStartRating() uint32
	//GetSignedBlocksThreshold gets the threshold for the minimum signed blocks
	GetSignedBlocksThreshold() float32
	//ComputeIncreaseProposer computes the new rating for the increaseLeader
	ComputeIncreaseProposer(currentRating uint32) uint32
	//ComputeDecreaseProposer computes the new rating for the decreaseLeader
	ComputeDecreaseProposer(currentRating uint32, consecutiveMisses uint32) uint32
	//RevertIncreaseValidator computes the new rating if a revert for increaseProposer should be done
	RevertIncreaseValidator(currentRating uint32, nrReverts uint32) uint32
	//ComputeIncreaseValidator computes the new rating for the increaseValidator
	ComputeIncreaseValidator(currentRating uint32) uint32
	//ComputeDecreaseValidator computes the new rating for the decreaseValidator
	ComputeDecreaseValidator(currentRating uint32) uint32
	//IsInterfaceNil verifies if the interface is nil
	IsInterfaceNil() bool
}

// ChanceComputer provides chance computation capabilities based on a rating
type ChanceComputer interface {
	//GetChance returns the chances for the the rating
	GetChance(uint32) uint32
	//IsInterfaceNil verifies if the interface is nil
	IsInterfaceNil() bool
}

// ValidatorsDistributor distributes validators across shards
type ValidatorsDistributor interface {
	DistributeValidators(destination []Validator, source []Validator, rand []byte) error
	IsInterfaceNil() bool
}
