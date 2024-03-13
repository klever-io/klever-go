package sharding

import "errors"

// ErrNilPubkeyConverter signals that a nil public key converter has been provided
var ErrNilPubkeyConverter = errors.New("trying to set nil pubkey converter")

// ErrCouldNotParsePubKey signals that a given public key could not be parsed
var ErrCouldNotParsePubKey = errors.New("could not parse node's public key")

// ErrCouldNotParseAddress signals that a given address could not be parsed
var ErrCouldNotParseAddress = errors.New("could not parse node's address")

// ErrNegativeOrZeroConsensusGroupSize signals that an invalid consensus group size has been provided
var ErrNegativeOrZeroConsensusGroupSize = errors.New("negative or zero consensus group size")

// ErrMinNodesSmallerThanConsensusSize signals that an invalid min nodes has been provided
var ErrMinNodesSmallerThanConsensusSize = errors.New("minimum nodes is smaller than consensus group size")

// ErrNodesSizeSmallerThanMinNoOfNodes signals that there are not enough nodes defined in genesis file
var ErrNodesSizeSmallerThanMinNoOfNodes = errors.New("length of nodes defined is smaller than min nodes per shard required")

// ErrNilElected signals nil elected list
var ErrNilElected = errors.New("nil elected")

// ErrNoPubKeys signals an error when public keys are missing
var ErrNoPubKeys = errors.New("no public keys defined")

// ErrPublicKeyNotFoundInGenesis signals an error when the public key is not in genesis file
var ErrPublicKeyNotFoundInGenesis = errors.New("public key is not valid, it is missing from genesis file")

// ErrValidatorNotFound signals that the validator has not been found
var ErrValidatorNotFound = errors.New("validator not found")

// ErrNilPubKey signals that the public key is nil
var ErrNilPubKey = errors.New("nil public key")

// ErrNilRandomness signals that a nil randomness source has been provided
var ErrNilRandomness = errors.New("nil randomness source")

// ErrNilRandomSelector signals that a nil selector was provided
var ErrNilRandomSelector = errors.New("nil selector")

// ErrNilInputNodesMap signals that a nil nodes map was provided
var ErrNilInputNodesMap = errors.New("nil input nodes map")

// ErrEpochNodesConfigDoesNotExist signals that the epoch nodes configuration is missing
var ErrEpochNodesConfigDoesNotExist = errors.New("epoch nodes configuration does not exist")

// ErrListSizeZero signals that there are no elements in the list
var ErrListSizeZero = errors.New("list size zero")

// ErrInvalidWeight signals an invalid weight was provided
var ErrInvalidWeight = errors.New("invalid weight")

// ErrNilWeights signals that nil weights list was provided
var ErrNilWeights = errors.New("nil weights")

// ErrInvalidConsensusGroupSize signals that the consensus size is invalid (e.g. value is negative)
var ErrInvalidConsensusGroupSize = errors.New("invalid consensus group size")

// ErrNilMarshalizer signals that a nil marshalizer has been provided
var ErrNilMarshalizer = errors.New("nil marshalizer")

// ErrNilHasher signals that a nil hasher has been provided
var ErrNilHasher = errors.New("nil hasher")

// ErrNilShuffler signals that a nil shuffler was provided
var ErrNilShuffler = errors.New("nil nodes shuffler provided")

// ErrNilBootStorer signals that a nil boot storer was provided
var ErrNilBootStorer = errors.New("nil boot storer provided")

// ErrNilCacher signals that a nil cacher has been provided
var ErrNilCacher = errors.New("nil cacher")

// ErrInvalidSampleSize signals that an invalid sample size was provided
var ErrInvalidSampleSize = errors.New("invalid sample size")

// ErrSmallElectedListSize signals that the elected validators list's size is less than the consensus size
var ErrSmallElectedListSize = errors.New("small elected list size")

// ErrNilOrEmptyDestinationForDistribute signals that a nil or empty value was provided for destination of distributedNodes
var ErrNilOrEmptyDestinationForDistribute = errors.New("nil or empty destination list for distributeNodes")

// ErrNilNodeShufflerArguments signals that a nil argument pointer was provided for creating the nodes shuffler instance
var ErrNilNodeShufflerArguments = errors.New("nil arguments for the creation of a node shuffler")

// ErrValidatorListNotFound signals that the validator list has not been found
var ErrValidatorListNotFound = errors.New("validator list not found")
