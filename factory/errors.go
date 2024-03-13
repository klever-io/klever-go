package factory

import "errors"

// ErrPublicKeyMismatch signals that the read public key mismatch the one read
var ErrPublicKeyMismatch = errors.New("public key mismatch between the computed and the one read from the file")

// ErrNilEconomicsData signals that a nil economics data handler has been provided
var ErrNilEconomicsData = errors.New("nil economics data provided")

// ErrNilCoreComponents signals that nil core components have been provided
var ErrNilCoreComponents = errors.New("nil core components provided")

// ErrNilPathManager signals that a nil path manager has been provided
var ErrNilPathManager = errors.New("nil path manager provided")

// ErrNilEpochStartNotifier signals that a nil epoch start notifier has been provided
var ErrNilEpochStartNotifier = errors.New("nil epoch start notifier provided")

// ErrHasherCreation signals that the hasher cannot be created based on provided data
var ErrHasherCreation = errors.New("error creating hasher")

// ErrMarshalizerCreation signals that the marshalizer cannot be created based on provided data
var ErrMarshalizerCreation = errors.New("error creating marshalizer")

// ErrDataPoolCreation signals that the data pool cannot be created
var ErrDataPoolCreation = errors.New("can not create data pool")

// ErrBlockchainCreation signals that the blockchain cannot be created
var ErrBlockchainCreation = errors.New("can not create blockchain")

// ErrDataStoreCreation signals that the data store cannot be created
var ErrDataStoreCreation = errors.New("can not create data store")

// ErrMultiSigCreation signals that the multisigner couldn't be created
var ErrMultiSigCreation = errors.New("could not start creation of multiSigner")

// ErrNilNodesConfig signals that a nil nodes configuration has been provided
var ErrNilNodesConfig = errors.New("nil nodes configuration provided")

// ErrMissingConsensusConfig signals that consensus type isn't specified in the configuration file
var ErrMissingConsensusConfig = errors.New("no consensus type provided in config file")

// ErrMultiSigHasherMissmatch signals that an invalid multisig hasher was provided
var ErrMultiSigHasherMissmatch = errors.New("wrong multisig hasher provided for bls consensus type")

// ErrMissingMultiHasherConfig signals that the multihasher type isn't specified in the configuration file
var ErrMissingMultiHasherConfig = errors.New("no multisig hasher provided in config file")

// ErrNilTriesComponents signals that nil tries components have been provided
var ErrNilTriesComponents = errors.New("nil tries components provided")

// ErrPubKeyConverterCreation signals that the public key converter cannot be created based on provided data
var ErrPubKeyConverterCreation = errors.New("error creating public key converter")

// ErrAccountsAdapterCreation signals that the accounts adapter cannot be created based on provided data
var ErrAccountsAdapterCreation = errors.New("error creating accounts adapter")

// ErrFileNotFound signals that specified file does not exists
var ErrFileNotFound = errors.New("no such file or directory")
