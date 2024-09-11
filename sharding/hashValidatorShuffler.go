package sharding

import (
	"bytes"
	"sort"
	"sync"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
)

var _ NodesShuffler = (*randHashShuffler)(nil)

// NodesShufflerArgs defines the arguments required to create a nodes shuffler
type NodesShufflerArgs struct {
	Nodes                uint32
	MaxNodesEnableConfig []config.MaxNodesChangeConfig
}

type shuffleNodesArg struct {
	elected    []Validator
	eligible   []Validator
	randomness []byte
	// distributor             ValidatorsDistributor
	nodes          uint32
	maxNodesToSwap uint32
}

type randHashShuffler struct {
	nodes                 uint32
	activeNodesConfig     config.MaxNodesChangeConfig
	availableNodesConfigs []config.MaxNodesChangeConfig
	mutShufflerParams     sync.RWMutex
}

// NewHashValidatorsShuffler creates a validator shuffler that uses a hash between validator key and a given
// random number to do the shuffling
func NewHashValidatorsShuffler(args *NodesShufflerArgs) (*randHashShuffler, error) {
	if args == nil {
		return nil, ErrNilNodeShufflerArguments
	}

	var configs []config.MaxNodesChangeConfig

	if args.MaxNodesEnableConfig != nil {
		configs = make([]config.MaxNodesChangeConfig, len(args.MaxNodesEnableConfig))
		copy(configs, args.MaxNodesEnableConfig)
	}

	rxs := &randHashShuffler{
		availableNodesConfigs: configs,
	}

	rxs.UpdateParams(args.Nodes)

	rxs.sortConfigs()

	return rxs, nil
}

// UpdateParams updates the shuffler parameters
// Should be called when new params are agreed through governance
func (rhs *randHashShuffler) UpdateParams(
	nodes uint32,
) {
	rhs.mutShufflerParams.Lock()
	rhs.nodes = nodes
	rhs.mutShufflerParams.Unlock()
}

// UpdateNodeLists shuffles the nodes and returns the lists with the new nodes configuration
func (rhs *randHashShuffler) UpdateNodeLists(args ArgsUpdateNodes) (*ResUpdateNodes, error) {
	rhs.UpdateShufflerConfig(args.Epoch)

	rhs.mutShufflerParams.RLock()
	nodes := rhs.nodes
	rhs.mutShufflerParams.RUnlock()

	return shuffleNodes(shuffleNodesArg{
		elected:        args.Elected,
		eligible:       args.Eligible,
		randomness:     args.Rand,
		nodes:          nodes,
		maxNodesToSwap: rhs.activeNodesConfig.NodesToShuffle,
	})
}

// IsInterfaceNil verifies if the underlying object is nil
func (rhs *randHashShuffler) IsInterfaceNil() bool {
	return rhs == nil
}

func shuffleNodes(arg shuffleNodesArg) (*ResUpdateNodes, error) {

	electedCopy := arg.elected
	eligibleCopy := arg.eligible

	numToRemove, err := computeNumToRemove(arg)
	if err != nil {
		return nil, err
	}

	if numToRemove > int(arg.maxNodesToSwap) {
		numToRemove = int(arg.maxNodesToSwap)
	}

	//* Here we take out a number of validators(numToRemove) randomically and return the ones that
	//* are still elected the others that will be substituted
	shuffledOutList, newElected := shuffleOutNodes(electedCopy, numToRemove, arg.randomness)
	log.Trace("shuffleNodes", "shuffledOutList", len(shuffledOutList), "newElected", len(newElected))

	//* Here we take the eligibles and put inside the elected array
	shuffledEligible := shuffleList(eligibleCopy, arg.randomness)
	updatedElected, updatedEligible, err := moveMaxNumNodesToList(newElected, shuffledEligible, arg.nodes)
	if err != nil {
		log.Warn("moveNodesToList failed", "error", err)
	}

	//* Here we put the nodes that were shuffled out from the elected list inside the eligible list
	finalEligible, _, err := moveNodesToList(updatedEligible, shuffledOutList)
	if err != nil {
		log.Warn("moveNodesToList failed", "error", err)
	}

	return &ResUpdateNodes{
		Elected:  updatedElected,
		Eligible: finalEligible,
	}, nil
}

func computeNumToRemove(arg shuffleNodesArg) (int, error) {
	notEnoughValidators := len(arg.elected)+len(arg.eligible) < int(arg.nodes)

	if notEnoughValidators {
		return 0, ErrSmallElectedListSize
	}

	return len(arg.elected) + len(arg.eligible) - int(arg.nodes), nil
}

// shuffleOutNodes shuffles the list of elected validators in each shard and returns the map of shuffled out
// validators
func shuffleOutNodes(
	elected []Validator,
	numToShuffle int,
	randomness []byte,
) ([]Validator, []Validator) {

	validators := elected
	shuffledElected := shuffleList(validators, randomness)
	shuffledOut := shuffledElected[:numToShuffle]
	remainingValidators := shuffledElected[numToShuffle:]

	newElected, _ := removeValidatorsFromList(remainingValidators, shuffledOut, len(shuffledOut))

	return shuffledOut, newElected
}

// shuffleList returns a shuffled list of validators.
// The shuffling is done by hash-ing the randomness concatenated with the
// public keys of validators and sorting the validators depending on
// the hash result.
func shuffleList(validators []Validator, randomness []byte) []Validator {
	keys := make([]string, len(validators))
	mapValidators := make(map[string]Validator)
	var concat []byte

	hasher := &sha256.Sha256{}
	for i, v := range validators {
		concat = append(v.PubKey(), randomness...)

		keys[i] = string(hasher.Compute(string(concat)))
		mapValidators[keys[i]] = v
	}

	sort.Strings(keys)

	result := make([]Validator, len(validators))
	for i := 0; i < len(validators); i++ {
		result[i] = mapValidators[keys[i]]
	}

	return result
}

func removeValidatorsFromList(
	validatorList []Validator,
	validatorsToRemove []Validator,
	maxToRemove int,
) ([]Validator, []Validator) {
	resultedList := make([]Validator, 0)
	resultedList = append(resultedList, validatorList...)
	removed := make([]Validator, 0)

	for _, valToRemove := range validatorsToRemove {
		if len(removed) == maxToRemove {
			break
		}

		for i := len(resultedList) - 1; i >= 0; i-- {
			val := resultedList[i]
			if bytes.Equal(val.PubKey(), valToRemove.PubKey()) {
				resultedList = removeValidatorFromList(resultedList, i)
				removed = append(removed, val)
				break
			}
		}
	}

	return resultedList, removed
}

// removeValidatorFromList replaces the element at given index with the last element in the slice and returns a slice
// with a decremented length.The order in the list is important as long as it is kept the same for all validators,
// so not critical to maintain the original order inside the list, as that would be slower.
//
// Attention: The slice given as parameter will have its element on position index swapped with the last element
func removeValidatorFromList(validatorList []Validator, index int) []Validator {
	indexNotOK := index > len(validatorList)-1 || index < 0
	if indexNotOK {
		return validatorList
	}

	validatorList[index] = validatorList[len(validatorList)-1]
	return validatorList[:len(validatorList)-1]
}

// moveNodesToList moves the validators in the source list to the corresponding destination list
func moveNodesToList(destination []Validator, source []Validator) ([]Validator, []Validator, error) {
	if destination == nil {
		return nil, nil, ErrNilOrEmptyDestinationForDistribute
	}

	destination = append(destination, source...)
	source = make([]Validator, 0)

	return destination, source, nil
}

// moveMaxNumNodesToMap moves the validators in the source list to the corresponding destination list
// but adding just enough nodes so that at most the number of nodes is kept in the destination list
// The parameter maxNodesToMove is a limiting factor and should limit the number of nodes
func moveMaxNumNodesToList(
	destination []Validator,
	source []Validator,
	numNodes uint32,
) ([]Validator, []Validator, error) {
	if destination == nil {
		return nil, nil, ErrNilOrEmptyDestinationForDistribute
	}

	numNeededNodes := computeNeededNodes(destination, source, numNodes)
	newDestination := append(destination, source[0:numNeededNodes]...)
	newSource := source[numNeededNodes:]

	return newDestination, newSource, nil
}

func computeNeededNodes(destination []Validator, source []Validator, maxNumNodes uint32) uint32 {
	numNeededNodes := uint32(0)
	numCurrentNodes := uint32(len(destination)) // #nosec G115
	numSourceNodes := uint32(len(source))       // #nosec G115

	if maxNumNodes > numCurrentNodes {
		numNeededNodes = maxNumNodes - numCurrentNodes
	}

	if numSourceNodes < numNeededNodes {
		return numSourceNodes
	}

	return numNeededNodes
}

// UpdateShufflerConfig updates the shuffler config according to the current epoch.
func (rhs *randHashShuffler) UpdateShufflerConfig(epoch uint32) {
	rhs.mutShufflerParams.Lock()
	defer rhs.mutShufflerParams.Unlock()
	rhs.activeNodesConfig.NodesToShuffle = rhs.nodes
	for _, maxNodesConfig := range rhs.availableNodesConfigs {
		if epoch >= maxNodesConfig.EpochEnable {
			rhs.activeNodesConfig = maxNodesConfig
		}
	}

	log.Debug("randHashShuffler: UpdateShufflerConfig",
		"epoch", epoch,
		"epochEnable", rhs.activeNodesConfig.EpochEnable,
		"maxNodesToShuffle", rhs.activeNodesConfig.NodesToShuffle,
	)
}

func (rhs *randHashShuffler) sortConfigs() {
	rhs.mutShufflerParams.Lock()
	sort.Slice(rhs.availableNodesConfigs, func(i, j int) bool {
		return rhs.availableNodesConfigs[i].EpochEnable < rhs.availableNodesConfigs[j].EpochEnable
	})
	rhs.mutShufflerParams.Unlock()
}
