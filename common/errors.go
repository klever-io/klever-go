package common

import "errors"

// ErrInvalidValue signals that a nil value has been provided
var ErrInvalidValue = errors.New("invalid value provided")

// ErrNilSignalChan returns whenever a nil signal channel is provided
var ErrNilSignalChan = errors.New("nil signal channel")

// ErrNotPositiveValue signals that a 0 or negative value has been provided
var ErrNotPositiveValue = errors.New("the provided value is not positive")

// ErrFileLoggingProcessIsClosed signals that the file logging process is closed
var ErrFileLoggingProcessIsClosed = errors.New("file logging process is closed")

// ErrInvalidLogFileMinLifeSpan signals that an invalid log file life span was provided
var ErrInvalidLogFileMinLifeSpan = errors.New("minimum log file life span is invalid")

// ErrInvalidLogFileSizeMinCheckSpan signals that an invalid log check file size span was provided
var ErrInvalidLogFileSizeMinCheckSpan = errors.New("minimum log check file size span is invalid")

// ErrReadingLogFileDirectory signals that it cant read the file directory
var ErrReadingLogFileDirectory = errors.New("can't read log file directory")

// ErrNilAppStatusHandler signals that a nil status handler has been provided
var ErrNilAppStatusHandler = errors.New("appStatusHandler is nil")

// ErrNilStatusTagProvider signals that a nil status tag provider has been given as parameter
var ErrNilStatusTagProvider = errors.New("nil status tag provider")

// ErrInvalidPollingInterval signals that an invalid polling interval has been provided
var ErrInvalidPollingInterval = errors.New("invalid polling interval ")

// ErrInvalidPubkeyConverterType signals that the provided pubkey converter type is invalid
var ErrInvalidPubkeyConverterType = errors.New("invalid pubkey converter type")

// ErrInvalidAddressLength signals that address length is invalid
var ErrInvalidAddressLength = errors.New("invalid address length")

// ErrWrongSize signals that a wrong size occurred
var ErrWrongSize = errors.New("wrong size")

// ErrBech32ConvertError signals that conversion the 5bit alphabet to 8bit failed
var ErrBech32ConvertError = errors.New("can't convert bech32 string")

// ErrInvalidKLVAddress signals that the provided address is not an KLV address
var ErrInvalidKLVAddress = errors.New("invalid KLV address")

// ErrNilPubkeyConverter signals that a nil public key converter has been provided
var ErrNilPubkeyConverter = errors.New("trying to set nil pubkey converter")

// ErrNilStatusHandler signals that a nil status handler has been provided
var ErrNilStatusHandler = errors.New("nil status handler")

// ErrNilCacher is raised when a nil cacher is provided
var ErrNilCacher = errors.New("expected not nil cacher")

// ErrNilBlackListCacher signals that a nil black list cacher was provided
var ErrNilBlackListCacher = errors.New("nil black list cacher")

// ErrNilPeerShardMapper signals that a nil peer shard mapper has been provided
var ErrNilPeerShardMapper = errors.New("nil peer shard mapper")

// ErrSystemBusy signals that the system is busy
var ErrSystemBusy = errors.New("system busy")

// ErrNilMarshalizer signals that a nil marshalizer has been provided
var ErrNilMarshalizer = errors.New("nil marshalizer")

// ErrNilUrl signals that the provided url is empty
var ErrNilUrl = errors.New("url is empty")

// ErrWrongTypeAssertion signals wrong type assertion
var ErrWrongTypeAssertion = errors.New("wrong type assertion")

// ErrNilNode signals that a nil node instance has been provided
var ErrNilNode = errors.New("nil node")

// ErrNilAPIResolver signals that a nil api resolver instance has been provided
var ErrNilAPIResolver = errors.New("nil api resolver")

// ErrNoAPIRoutesConfig signals that no configuration was found for API routes
var ErrNoAPIRoutesConfig = errors.New("no configuration found for API routes")

// ErrNilPeerState signals that a nil peer state has been provided
var ErrNilPeerState = errors.New("nil peer state")

// ErrNilAccountState signals that a nil account state has been provided
var ErrNilAccountState = errors.New("nil account state")

// ErrNilKappState signals that a nil asset state has been provided
var ErrNilKappState = errors.New("nil kapp state")

// ErrInvalidHeaderType signals an invalid header pointer was provided
var ErrInvalidHeaderType = errors.New("invalid header type")

// ErrWrongTypeInContainer signals that a wrong type of object was found in container
var ErrWrongTypeInContainer = errors.New("wrong type of object inside container")

// ErrInvalidContainerKey signals that an element does not exist in the container's map
var ErrInvalidContainerKey = errors.New("element does not exist in container")

// ErrNilContainerElement signals when trying to add a nil element in the container
var ErrNilContainerElement = errors.New("element cannot be nil")

// ErrContainerKeyAlreadyExists signals that an element was already set in the container's map
var ErrContainerKeyAlreadyExists = errors.New("provided key already exists in container")

// ErrLenMismatch signals that 2 or more slices have different lengths
var ErrLenMismatch = errors.New("lengths mismatch")

// ErrNilResolverContainer signals that a nil resolver container was provided
var ErrNilResolverContainer = errors.New("nil resolver container")

// ErrNilMessenger signals that a nil Messenger object was provided
var ErrNilMessenger = errors.New("nil Messenger")

// ErrNilStore signals that the provided storage service is nil
var ErrNilStore = errors.New("nil data storage service")

// ErrNilDataPoolHolder signals that the data pool holder is nil
var ErrNilDataPoolHolder = errors.New("nil data pool holder")

// ErrNilUint64ByteSliceConverter signals that a nil byte slice converter was provided
var ErrNilUint64ByteSliceConverter = errors.New("nil byte slice converter")

// ErrNilDataPacker signals that a nil data packer has been provided
var ErrNilDataPacker = errors.New("nil data packer provided")

// ErrNilTrieDataGetter signals that a nil trie data getter has been provided
var ErrNilTrieDataGetter = errors.New("nil trie data getter provided")

// ErrNilAntifloodHandler signals that a nil antiflood handler has been provided
var ErrNilAntifloodHandler = errors.New("nil antiflood handler")

// ErrNilThrottler signals that a nil throttler has been provided
var ErrNilThrottler = errors.New("nil throttler")

// ErrNilMessage signals that a nil message has been received
var ErrNilMessage = errors.New("nil message")

// ErrNilDataToProcess signals that nil data was provided
var ErrNilDataToProcess = errors.New("nil data to process")

// ErrNilValue signals the value is nil
var ErrNilValue = errors.New("nil value")

// ErrNilResolverSender signals that a nil resolver sender object has been provided
var ErrNilResolverSender = errors.New("nil resolver sender")

// ErrNilHeadersDataPool signals that a nil header pool has been provided
var ErrNilHeadersDataPool = errors.New("nil headers data pool")

// ErrNilHeadersStorage signals that a nil header storage has been provided
var ErrNilHeadersStorage = errors.New("nil headers storage")

// ErrNilHeadersNoncesStorage signals that a nil header-nonce storage has been provided
var ErrNilHeadersNoncesStorage = errors.New("nil headers nonces storage")

// ErrNilEpochHandler signals that epoch handler is nil
var ErrNilEpochHandler = errors.New("nil epoch handler")

// ErrResolveTypeUnknown signals that an unknown resolve type was provided
var ErrResolveTypeUnknown = errors.New("unknown resolve type")

// ErrMissingData signals that the required data is missing
var ErrMissingData = errors.New("missing data")

// ErrInvalidNonceByteSlice signals that an invalid byte slice has been provided
// and an uint64 can not be decoded from that byte slice
var ErrInvalidNonceByteSlice = errors.New("invalid nonce byte slice")

// ErrInvalidIdentifierForEpochStartBlockRequest signals that an invalid identifier for epoch start block request
// has been provided
var ErrInvalidIdentifierForEpochStartBlockRequest = errors.New("invalid identifier for epoch start block request")

// ErrNilTxDataPool signals that a nil transaction pool has been provided
var ErrNilTxDataPool = errors.New("nil transaction data pool")

// ErrNilTxStorage signals that a nil transaction storage has been provided
var ErrNilTxStorage = errors.New("nil transaction storage")

// ErrRequestTypeNotImplemented signals that a not implemented type of request has been received
var ErrRequestTypeNotImplemented = errors.New("request type is not implemented")

// ErrEmptyString signals that an empty string has been provided
var ErrEmptyString = errors.New("empty string")

// ErrNilPeerListCreator signals that a nil peer list creator implementation has been provided
var ErrNilPeerListCreator = errors.New("nil peer list creator provided")

// ErrInvalidPeerList signals that a invalid peer list has been provided
var ErrInvalidPeerList = errors.New("invalid peer list")

// ErrNilRandomizer signals that a nil randomizer has been provided
var ErrNilRandomizer = errors.New("nil randomizer")

// ErrSendRequest signals that the connected peers list is empty or errors appeared when sending requests
var ErrSendRequest = errors.New("cannot send request: peer list is empty or errors during the sending")

// ErrNilResolverDebugHandler signals that a nil resolver debug handler has been provided
var ErrNilResolverDebugHandler = errors.New("nil resolver debug handler")

// ErrNilBlocksPool signals that a nil blocks pool has been provided
var ErrNilBlocksPool = errors.New("nil blocks pool")

// ErrNilBlocksStorage signals that a nil blocks storage has been provided
var ErrNilBlocksStorage = errors.New("nil blocks storage")

// ErrNilInputData signals that a nil data has been provided
var ErrNilInputData = errors.New("nil input data")

// ErrBadRequest signals that the request should not have happened
var ErrBadRequest = errors.New("request should not be done as it doesn't follow the protocol")

// ErrInvalidMaxTxRequest signals that max tx request is too small
var ErrInvalidMaxTxRequest = errors.New("max tx request number is invalid")

// ErrNilRequestedItemsHandler signals that a nil requested items handler was provided
var ErrNilRequestedItemsHandler = errors.New("nil requested items handler")

// ErrNilResolverFinder signals that a nil resolver finder has been provided
var ErrNilResolverFinder = errors.New("nil resolvers finder")

// ErrNilWhiteListHandler signals that white list handler is nil
var ErrNilWhiteListHandler = errors.New("nil whitelist handler")

// ErrRequestIntervalTooSmall signals that request interval is too small
var ErrRequestIntervalTooSmall = errors.New("request interval is too small")

// ErrInvalidNumberOfPersisters signals that an invalid number of persisters has been provided
var ErrInvalidNumberOfPersisters = errors.New("invalid number of active persisters")

// ErrNilEpochStartNotifier signals that a nil epoch start notifier has been provided
var ErrNilEpochStartNotifier = errors.New("nil epoch start notifier")

// ErrNilWasmChangeLocker signals that a nil wasm lock has been provided
var ErrNilWasmChangeLocker = errors.New("nil wasm change locker")

// ErrEmptyGasSchedule signals that the gas schedule is empty
var ErrEmptyGasSchedule = errors.New("empty gas schedule")

// ErrNilPathManager signals that a nil path manager has been provided
var ErrNilPathManager = errors.New("nil path manager")

// ErrNilPersisterFactory signals that a nil persister factory has been provided
var ErrNilPersisterFactory = errors.New("nil persister factory")

// ErrNilConfig signals that a nil configuration has been received
var ErrNilConfig = errors.New("nil config")

// ErrInvalidNumberOfEpochsToSave signals that an invalid number of epochs to save has been provided
var ErrInvalidNumberOfEpochsToSave = errors.New("invalid number of epochs to save")

// ErrInvalidNumberOfActivePersisters signals that an invalid number of active persisters has been provided
var ErrInvalidNumberOfActivePersisters = errors.New("invalid number of active persisters")

// ErrNoSuchStorageUnit defines the error for using an invalid storage unit
var ErrNoSuchStorageUnit = errors.New("no such unit type")

// ErrNilEconomicsData signals that a nil economics data handler has been provided
var ErrNilEconomicsData = errors.New("nil economics data provided")

// ErrInvalidCacheSize signals an invalid size provided for cache
var ErrInvalidCacheSize = errors.New("invalid cache size")

// ErrNilDatabase is raised when a database operation is called, but no database is provided
var ErrNilDatabase = errors.New("no database provided")

// ErrSuffixNotPresentOrInIncorrectPosition signals that the suffix is not present in the data field or its position is incorrect
var ErrSuffixNotPresentOrInIncorrectPosition = errors.New("suffix is not present or the position is incorrect")

// ErrNilTxBlockDataPool signals that a nil tx block body pool has been provided
var ErrNilTxBlockDataPool = errors.New("nil tx block data pool")

// ErrNilCurrBlockTxs signals that nil current blocks txs holder was provided
var ErrNilCurrBlockTxs = errors.New("nil current block txs holder")

// ErrNilManualEpochStartNotifier signals that a nil manual epoch start notifier has been provided
var ErrNilManualEpochStartNotifier = errors.New("nil manual epoch start notifier")

// ErrNilGracefullyCloseChannel signals that a nil gracefully close channel has been provided
var ErrNilGracefullyCloseChannel = errors.New("nil gracefully close channel")

// ErrNilTrieNodesPool signals that a nil trie nodes data pool was provided
var ErrNilTrieNodesPool = errors.New("nil trie nodes data pool")

// ErrNilSmartContractsPool signals that a nil smart contracts pool has been provided
var ErrNilSmartContractsPool = errors.New("nil smart contracts pool")

// ErrTxNotFoundInBlockPool signals the value is nil
var ErrTxNotFoundInBlockPool = errors.New("cannot find tx in current block pool")

// ErrCacheConfigInvalidSizeInBytes signals that the cache parameter "sizeInBytes" is invalid
var ErrCacheConfigInvalidSizeInBytes = errors.New("cache parameter [sizeInBytes] is not valid, it must be a positive, and large enough number")

// ErrCacheConfigInvalidSize signals that the cache parameter "size" is invalid
var ErrCacheConfigInvalidSize = errors.New("cache parameter [size] is not valid, it must be a positive number")

// ErrCacheConfigInvalidShards signals that the cache parameter "shards" is invalid
var ErrCacheConfigInvalidShards = errors.New("cache parameter [shards] is not valid, it must be a positive number")

// ErrCacheConfigInvalidEconomics signals that an economics parameter required by the cache is invalid
var ErrCacheConfigInvalidEconomics = errors.New("cache-economics parameter is not valid")

// ErrNilNodesCoordinator signals a nil nodes coordinator has been provided
var ErrNilNodesCoordinator = errors.New("nil nodes coordinator")

// ErrValidatorNotFound signals that the validator has not been found
var ErrValidatorNotFound = errors.New("validator not found")

// ErrNilAddress defines the error when trying to work with a nil address
var ErrNilAddress = errors.New("nil address")

// ErrNilAssetID defines the error when trying to work with a nil address
var ErrNilAssetID = errors.New("nil asset id")

// ErrNilTrie signals that a trie is nil and no operation can be made
var ErrNilTrie = errors.New("nil trie")

// ErrNilHasher signals that an operation has been attempted to or with a nil hasher implementation
var ErrNilHasher = errors.New("nil hasher")

// ErrSnapshotValueOutOfBounds signals that the snapshot value is out of bounds
var ErrSnapshotValueOutOfBounds = errors.New("snapshot value out of bounds")

// ErrAccNotFound signals that account was not found in state trie
var ErrAccNotFound = errors.New("account was not found")

// ErrAccNotOwner signals that account is not the owner
var ErrAccNotOwner = errors.New("account is not the owner")

// ErrRoleNotFound signals that role was not found in state trie
var ErrRoleNotFound = errors.New("role was not found")

// ErrAssetNotFound signals that asset was not found in state trie
var ErrAssetNotFound = errors.New("asset was not found")

// ErrMarketplaceNotFound signals that marketplace was not found in state trie
var ErrMarketplaceNotFound = errors.New("marketplace was not found")

// ErrStakingNotFound signals that stakingount was not found in state trie
var ErrStakingNotFound = errors.New("staking was not found")

// ErrProposalNotFound signals that proposal was not found in state trie
var ErrProposalNotFound = errors.New("proposal was not found")

// ErrBalance signals a balance error
var ErrBalance = errors.New("balance error")

// ErrNilUpdater signals that a nil updater has been provided
var ErrNilUpdater = errors.New("updater is nil")

// ErrNilOrEmptyDataTrieUpdates signals that there are no data trie updates
var ErrNilOrEmptyDataTrieUpdates = errors.New("no data trie updates")

// ErrNilAccountHandler signals that a nil account wrapper was provided
var ErrNilAccountHandler = errors.New("account wrapper is nil")

// ErrNilAccountHandler signals that a nil account wrapper was provided
var ErrNilAssetHandler = errors.New("asset wrapper is nil")

// ErrInvalidRootHash signals that the provided root hash is invalid
var ErrInvalidRootHash = errors.New("invalid root hash")

// ErrNilMapOfHashes signals that the provided map of hashes is nil
var ErrNilMapOfHashes = errors.New("nil map of hashes")

// ErrNilTrackableDataTrie signals that a nil trackable data trie has been provided
var ErrNilTrackableDataTrie = errors.New("nil trackable data trie")

// ErrNilAccountFactory signals that a nil account factory was provided
var ErrNilAccountFactory = errors.New("account factory is nil")

// ErrNilAssetFactory signals that a nil account factory was provided
var ErrNilAssetFactory = errors.New("asset factory is nil")

// ErrLeafSizeTooBig signals that the value size of the leaf is too big
var ErrLeafSizeTooBig = errors.New("leaf size too big")

// ErrNegativeValue signals that a negative value has been detected and it is not allowed
var ErrNegativeValue = errors.New("negative value")

// ErrNilBLSPublicKey signals that the provided BLS public key is nil
var ErrNilBLSPublicKey = errors.New("bls public key is nil")

// ErrNilEpochNotifier signals that the provided EpochNotifier is nil
var ErrNilEpochNotifier = errors.New("nil EpochNotifier")

// ErrNilProposalController signals that the provided ProposalController is nil
var ErrNilProposalController = errors.New("nil ProposalController")

// ErrNilForkController signals that the provided ForkController is nil
var ErrNilForkController = errors.New("nil ForkController")

// ErrNilEpochStartTrigger signals that a nil start of epoch trigger was provided
var ErrNilEpochStartTrigger = errors.New("nil start of epoch trigger")

// ErrNilSingleSigner is raised when a valid singleSigner is expected but nil used
var ErrNilSingleSigner = errors.New("singleSigner is nil")

// ErrNilPubKeyConverter signals that a nil public key converter has been provided
var ErrNilPubKeyConverter = errors.New("nil public key converter")

// ErrNilHeaderSigVerifier signals that a nil header sig verifier has been provided
var ErrNilHeaderSigVerifier = errors.New("nil header sig verifier")

// ErrNilHeaderIntegrityVerifier signals that a nil header integrity verifier has been provided
var ErrNilHeaderIntegrityVerifier = errors.New("nil header integrity verifier")

// ErrNilKeyGen signals that an operation has been attempted to or with a nil single sign key generator
var ErrNilKeyGen = errors.New("nil key generator")

// ErrNilDataPool signals that a nil data pool has been provided
var ErrNilDataPool = errors.New("trying to set nil data pool")

// ErrGenesisBlockNotInitialized signals that genesis block is not initialized
var ErrGenesisBlockNotInitialized = errors.New("genesis block is not initialized")

// ErrNilPoolsHolder signals that an operation has been attempted to or with a nil pools holder object
var ErrNilPoolsHolder = errors.New("nil pools holder")

// ErrNilStorage signals that a nil storage has been provided
var ErrNilStorage = errors.New("nil storage")

// ErrValidatorAlreadySet signals that a topic validator has already been set
var ErrValidatorAlreadySet = errors.New("topic validator has already been set")

// ErrNoTxToProcess signals that no transaction were sent for processing
var ErrNoTxToProcess = errors.New("no transaction to process")

// ErrAccountNotFound signals that an account was not found in trie
var ErrAccountNotFound = errors.New("account not found")

// ErrNilAccountsAdapter defines the error when trying to revert on nil accounts
var ErrNilAccountsAdapter = errors.New("nil AccountsAdapter")

// ErrInvalidChainIDInTransaction signals that an invalid chain id has been provided in transaction
var ErrInvalidChainIDInTransaction = errors.New("invalid chain ID")

// ErrInvalidChainID signals that an invalid chain ID has been provided
var ErrInvalidChainID = errors.New("invalid chain ID")

// ErrInvalidSignatureLength signals that an invalid signature length has been provided
var ErrInvalidSignatureLength = errors.New("invalid signature length")

// ErrInvalidSenderUsernameLength signals that the length of the sender username is invalid
var ErrInvalidSenderUsernameLength = errors.New("invalid sender username length")

// ErrInvalidReceiverUsernameLength signals that the length of the receiver username is invalid
var ErrInvalidReceiverUsernameLength = errors.New("invalid receiver username length")

// ErrDataFieldTooBig signals that the data field is too big
var ErrDataFieldTooBig = errors.New("data field is too big")

// ErrTransactionValueLengthTooBig signals that a too big value has been given to a transaction
var ErrTransactionValueLengthTooBig = errors.New("value length is too big")

// ErrNilQueryHandler signals that a nil query handler has been provided
var ErrNilQueryHandler = errors.New("nil query handler")

// ErrQueryHandlerAlreadyExists signals that the query handler is already registered
var ErrQueryHandlerAlreadyExists = errors.New("query handler already exists")

// ErrEmptyQueryHandlerName signals that an empty string can not be used to be used in the query handler container
var ErrEmptyQueryHandlerName = errors.New("empty query handler name")

// ErrUnknownPeerID signals that the provided peer is unknown by the current node
var ErrUnknownPeerID = errors.New("unknown peer ID")

// ErrNilValidatorStatistics signals that a nil validator statistics has been provided
var ErrNilValidatorStatistics = errors.New("nil validator statistics")

// ErrNilValidatorProvider signals that a nil validator provider has been provided
var ErrNilValidatorProvider = errors.New("nil validator provider")

// ErrMaxRatingZero signals that maxrating with a value of zero has been provided
var ErrMaxRatingZero = errors.New("max rating is zero")

// ErrInvalidCacheRefreshIntervalInSec signals that the cacheRefreshIntervalInSec is invalid - zero or less
var ErrInvalidCacheRefreshIntervalInSec = errors.New("invalid cacheRefreshIntervalInSec")

// ErrNilMultiSigVerifier signals that a nil multi-signature verifier is used
var ErrNilMultiSigVerifier = errors.New("nil multi-signature verifier")

// ErrTransactionIsNotWhitelisted signals that a transaction is not whitelisted
var ErrTransactionIsNotWhitelisted = errors.New("transaction is not whitelisted")

// ErrZeroSlotIntervalNotSupported signals that 0 seconds slot duration is not supported
var ErrZeroSlotIntervalNotSupported = errors.New("0 slot duration time is not supported")

// ErrNegativeOrZeroConsensusGroupSize signals that 0 elements consensus group is not supported
var ErrNegativeOrZeroConsensusGroupSize = errors.New("group size should be a strict positive number")

// ErrNilSyncTimer signals that a nil sync timer has been provided
var ErrNilSyncTimer = errors.New("trying to set nil sync timer")

// ErrNilBlockProcessor signals that a nil block processor has been provided
var ErrNilBlockProcessor = errors.New("trying to set nil block processor")

// ErrNilTransactionProcessor signals that a nil transaction processor has been provided
var ErrNilTransactionProcessor = errors.New("trying to set nil tx processor")

// ErrNilKAppController signals that a nil kapp controller has been provided
var ErrNilKAppController = errors.New("trying to set nil kapp controller")

// ErrNilBroadcastMessenger is raised when a valid broadcast messenger is expected but nil used
var ErrNilBroadcastMessenger = errors.New("trying to set nil broadcast messenger")

// ErrNilSingleSig signals that a nil singlesig object has been provided
var ErrNilSingleSig = errors.New("trying to set nil singlesig")

// ErrNilMultiSig signals that a nil multiSigner object has been provided
var ErrNilMultiSig = errors.New("trying to set nil multiSigner")

// ErrNilInterceptorsContainer signals that a nil interceptors container has been provided
var ErrNilInterceptorsContainer = errors.New("nil interceptors container")

// ErrNilNodeStopChannel signals that a nil channel for node process stop has been provided
var ErrNilNodeStopChannel = errors.New("nil node stop channel")

// ErrNilWatchdog signals that a nil watchdog has been provided
var ErrNilWatchdog = errors.New("nil watchdog")

// ErrNilPeerSignatureHandler signals that a nil peerSignatureHandler object has been provided
var ErrNilPeerSignatureHandler = errors.New("trying to set nil peerSignatureHandler")

// ErrNilBlockchain signals that a nil blockchain structure has been provided
var ErrNilBlockchain = errors.New("nil blockchain")

// ErrNilBlockchainHooks signals that a nil blockchain hooks structure has been provided
var ErrNilBlockchainHooks = errors.New("nil blockchain hooks")

// ErrNilResolversFinder signals that a nil resolvers finder has been provided
var ErrNilResolversFinder = errors.New("nil resolvers finder")

// ErrNilPeerDenialEvaluator signals that a nil peer denial evaluator was provided
var ErrNilPeerDenialEvaluator = errors.New("nil peer denial evaluator")

// ErrNilNetworkShardingCollector defines the error for setting a nil network sharding collector
var ErrNilNetworkShardingCollector = errors.New("nil network sharding collector")

// ErrNilRequestHandler signals that a nil request handler has been provided
var ErrNilRequestHandler = errors.New("trying to set nil request handler")

// ErrNilTxAccumulator signals that a nil Accumulator instance has been provided
var ErrNilTxAccumulator = errors.New("nil tx accumulator")

// ErrNilPeerHonestyHandler signals that a nil peer honesty handler has been provided
var ErrNilPeerHonestyHandler = errors.New("nil peer honesty handler")

// ErrNilBootStorer signals that a nil boot storer was provided
var ErrNilBootStorer = errors.New("nil boot storer")

// ErrNilPeerAccountsAdapter signals that a nil peer accounts database was provided
var ErrNilPeerAccountsAdapter = errors.New("nil peer accounts database")

// ErrNilKAppAccountsAdapter signals that a nil kapps accounts database was provided
var ErrNilKAppAccountsAdapter = errors.New("nil kapp accounts adapter")

// ErrNilNodesSetup signals that nil nodes setup has been provided
var ErrNilNodesSetup = errors.New("nil nodes setup")

// ErrNilRewardsHandler signals that rewards handler is nil
var ErrNilRewardsHandler = errors.New("rewards handler is nil")

// ErrZeroMaxComputableSlots signals that a value of zero was provided on the maxComputableSlots
var ErrZeroMaxComputableSlots = errors.New("max computable slots is zero")

// ErrNilRater signals that nil rater has been provided
var ErrNilRater = errors.New("nil rater")

// ErrNilSignature is raised when a valid signature was expected but nil was used
var ErrNilSignature = errors.New("signature is nil")

// ErrNilPreviousBlockHash signals that a operation has been attempted with a nil previous block header hash
var ErrNilPreviousBlockHash = errors.New("nil previous block header hash")

// ErrNilRootHash signals that an operation has been attempted with a nil root hash
var ErrNilRootHash = errors.New("root hash is nil")

// ErrNilRandSeed signals that a nil rand seed has been provided
var ErrNilRandSeed = errors.New("provided rand seed is nil")

// ErrNilPrevRandSeed signals that a nil previous rand seed has been provided
var ErrNilPrevRandSeed = errors.New("provided previous rand seed is nil")

// ErrUnmarshalWithoutSuccess signals that unmarshal some data was not done with success
var ErrUnmarshalWithoutSuccess = errors.New("unmarshal without success")

// ErrNilBlockChain signals that an operation has been attempted to or with a nil blockchain
var ErrNilBlockChain = errors.New("nil block chain")

// ErrNilHeadersNonceHashStorage signals that a nil header nonce hash storage has been provided
var ErrNilHeadersNonceHashStorage = errors.New("nil headers nonce hash storage")

// ErrNilForkDetector signals that a nil forkdetector object has been provided
var ErrNilForkDetector = errors.New("nil fork detector")

// ErrNilUint64Converter signals that uint64converter is nil
var ErrNilUint64Converter = errors.New("nil unit64converter")

// ErrNilSlotManager -
var ErrNilSlotManager = errors.New("nil slot manager")

// ErrNilShuffler signals that a nil shuffler was provided
var ErrNilShuffler = errors.New("nil nodes shuffler provided")

// ErrNilStorageUnitOpener signals that a nil storage unit opener was provided
var ErrNilStorageUnitOpener = errors.New("nil storage unit opener")

// ErrNilLatestStorageDataProvider signals that a nil latest storage data provider was provided
var ErrNilLatestStorageDataProvider = errors.New("nil latest storage data provider")

// ErrInvalidWorkingDir signals that an invalid working directory has been provided
var ErrInvalidWorkingDir = errors.New("invalid working directory")

// ErrInvalidDefaultEpochString signals that an invalid default epoch string has been provided
var ErrInvalidDefaultEpochString = errors.New("invalid default epoch string")

// ErrInvalidDefaultDBPath signals that an invalid default database path has been provided
var ErrInvalidDefaultDBPath = errors.New("invalid default db path")

// ErrNilBlockKeyGen signals that a nil block key generator has been provided
var ErrNilBlockKeyGen = errors.New("nil block key generator")

// ErrNilBlockSigner signals the nil block signer was provided
var ErrNilBlockSigner = errors.New("nil block signer")

// ErrNilBlockSingleSigner signals that a nil block single signer has been provided
var ErrNilBlockSingleSigner = errors.New("nil block single signer")

// ErrNilGenesisNodesConfig signals that a nil genesis nodes config has been provided
var ErrNilGenesisNodesConfig = errors.New("nil genesis nodes config")

// ErrNilTxSignMarshalizer signals that nil tx sign marshalizer has been provided
var ErrNilTxSignMarshalizer = errors.New("nil tx sign marshalizer")

// ErrInvalidConsensusThreshold signals that an invalid consensus threshold has been provided
var ErrInvalidConsensusThreshold = errors.New("invalid consensus threshold")

// ErrNotEnoughNumConnectedPeers signals that config is invalid for num of connected peers
var ErrNotEnoughNumConnectedPeers = errors.New("not enough min num of connected peers from config")

// ErrNotEnoughNumOfPeersToConsiderBlockValid signals that config is invalid for num of peer to consider block valid
var ErrNotEnoughNumOfPeersToConsiderBlockValid = errors.New("not enough num of peers to consider block valid from config")

// ErrNotEpochStartBlock signals that block is not of type epoch start
var ErrNotEpochStartBlock = errors.New("not epoch start block")

// ErrTimeoutWaitingForBlock signals that a timeout event was raised while waiting for the epoch start block
var ErrTimeoutWaitingForBlock = errors.New("timeout while waiting for epoch start block")

// ErrNilHeadersSubscriber is raised when a valid headers subscriber is expected but nil is provided
var ErrNilHeadersSubscriber = errors.New("nil headers subscriber")

// ErrNilHeader signals that a nil header slice has been provided
var ErrNilHeader = errors.New("nil header")

// ErrEmptyAddress signals that an empty address was found in genesis file
var ErrEmptyAddress = errors.New("empty address")

// ErrInvalidTransactionVersion signals that an invalid transaction version has been provided
var ErrInvalidTransactionVersion = errors.New("invalid transaction version")

// ErrTransactionNotFound signals that a transaction was not found
var ErrTransactionNotFound = errors.New("transaction not found")

// ErrDupTransaction signals that a transaction is already processed
var ErrDupTransaction = errors.New("duplicated transaction")

// ErrAssetAlreadyExists signals that an asset already exists
var ErrAssetAlreadyExists = errors.New("asset already exists")

// ErrAssetNameInvalid signals that an asset name is invalid
var ErrAssetNameInvalid = errors.New("invalid asset name")

// ErrAssetTypeInvalid signals that an asset type is invalid
var ErrAssetTypeInvalid = errors.New("invalid asset type")

// ErrClaimTypeInvalid signals that an claim type is invalid
var ErrClaimTypeInvalid = errors.New("invalid claim type")

// ErrSmartContractTypeInvalid signals that an smartContract type is invalid
var ErrSmartContractTypeInvalid = errors.New("invalid smart contract type")

// ErrITOTypeInvalid signals that an ito type is invalid
var ErrITOTypeInvalid = errors.New("invalid ito type")

// ErrITOStatusInvalid signals that an ito status is invalid
var ErrITOStatusInvalid = errors.New("invalid ito status")

// ErrITOWhitelistStatusInvalid signals that an ito whitelist status is invalid
var ErrITOWhitelistStatusInvalid = errors.New("invalid ito whitelist status")

// ErrWhitelistStatusInvalid signals that the whitelist status is invalid
var ErrWhitelistStatusInvalid = errors.New("invalid whitelist status")

// ErrITOTriggerInvalid signals that an ito trigger is invalid
var ErrITOTriggerInvalid = errors.New("invalid ito trigger")

// ErrBuyTypeInvalid signals that an buy type is invalid
var ErrBuyTypeInvalid = errors.New("invalid buy type")

// ErrWithdrawTypeInvalid signals that an withdraw type is invalid
var ErrWithdrawTypeInvalid = errors.New("invalid withdraw type")

// ErrDepositTypeInvalid signals that an deposit type is invalid
var ErrDepositTypeInvalid = errors.New("invalid deposit type")

// ErrAssetTriggerInvalid signals that an asset trigger is invalid
var ErrAssetTriggerInvalid = errors.New("invalid asset trigger")

// ErrRoleLimitReached signals that a role limit has been reached
var ErrRoleLimitReached = errors.New("role limit reached")

// ErrAssetTickerLengthInvalid signals that an asset ticker is invalid
var ErrAssetTickerLengthInvalid = errors.New("invalid asset ticker length")

// ErrAssetPrecision signals that an asset precision is invalid
var ErrAssetPrecision = errors.New("invalid asset precision")

// ErrInvalidTransactionType signals  that an invalid transaction type has been provided
var ErrInvalidTransactionType = errors.New("invalid transaction type")

// ErrTokenNameNotHumanReadable signals that token name is not human readable
var ErrTokenNameNotHumanReadable = errors.New("token name is not human readable")

// ErrTickerNameNotValid signals that ticker name is not valid
var ErrTickerNameNotValid = errors.New("ticker name is not valid")

// ErrInvalidContract signals an invalid contract sent
var ErrInvalidContract = errors.New("invalid contract")

// ErrNoPermission signals no permission to access contract
var ErrNoPermission = errors.New("no contract permission")

// ErrInvalidPermission signals an invalid permission buffer
var ErrInvalidPermission = errors.New("invalid permission buffer")

// ErrAssetIDInvalid signals an invalid asset id
var ErrAssetIDInvalid = errors.New("invalid asset id")

// ErrNilNodeRedundancyHandler signals that provided node redundancy handler is nil
var ErrNilNodeRedundancyHandler = errors.New("nil node redundancy handler")

// ErrTimeIsOut signals that time is out
var ErrTimeIsOut = errors.New("time is out")

// ErrNilStorageManager signals that nil storage manager has been provided
var ErrNilStorageManager = errors.New("nil trie storage manager")

// ErrInvalidMaxHardCapForMissingNodes signals that the maximum hardcap value for missing nodes is invalid
var ErrInvalidMaxHardCapForMissingNodes = errors.New("invalid max hardcap for missing nodes")

// ErrInvalidTransactionNoContract signals that transaction time has expired
var ErrTransactionTimeExpired = errors.New("transaction time expired")

// ErrNotCompressed ...
var ErrNotCompressed = errors.New("not compressed")

// ErrAlreadyCompressed ...
var ErrAlreadyCompressed = errors.New("already compressed")

// ErrInvalidParameter signals that a wrong parameter has been provided
var ErrInvalidParameter = errors.New("invalid parameter")

// ErrNotFoundInKApp signals that a trie is nil and no operation can be made
var ErrNotFoundInKApp = errors.New("not found in kapp")

// ErrAccountValidatorSet signals that account already has a BLS pubkey set
var ErrAccountValidatorSet = errors.New("validator already set for this account")

// ErrAccountValidatorNotOwner signals that account is not validator owner
var ErrAccountValidatorNotOwner = errors.New("account is not validator owner")

// ErrKAppController signals error on KApp Controller start
var ErrKAppController = errors.New("not able to start kapp controller")

// ErrNilKAppValidator signals error on nil Validator KApp
var ErrNilKAppValidator = errors.New("validator kapp is nil")

// ErrBLSKeyRevoked signals that BLS key has been revoked
var ErrBLSKeyRevoked = errors.New("blsKey revoked")

// ErrDupSignature signals that a transaction was signed twice by the same address
var ErrDupSignature = errors.New("duplicated signature")

// ErrSignatureThreshold signals that TX signature weight is lower than threshold
var ErrSignatureThreshold = errors.New("invalid signature threshold")

// ErrInvalidSignature signals that the given signature is invalid
var ErrInvalidSignature = errors.New("invalid signature")

// ErrGenesisNodeWithoutOwner signals that the given signature is invalid
var ErrGenesisNodeWithoutOwner = errors.New("initial nodes must refer to a valid genesis owner account")

// ErrMaxSupplyExceeded signals that KDA max supply has been exceeded
var ErrMaxSupplyExceeded = errors.New("max supply exceeded")

// ErrMaxBucketsExceeded max buckets exceeded for the asset
var ErrMaxBucketsExceeded = errors.New("max bucket exceeded")

// ErrITONotActive signals that ITO is not active
var ErrITONotActive = errors.New("ito not active")

// ErrInvalidAsset signals invalid asset
var ErrInvalidAsset = errors.New("invalid asset")

// ErrAssetPoolNotFound signals asset pool does not exists
var ErrAssetPoolNotFound = errors.New("asset pool not found")

// ErrAssetPoolInvalidAmount signals invalid amount asked to asset pool
var ErrAssetPoolInvalidAmount = errors.New("asset pool invalid amount")

// ErrAssetPoolInvalidAsset signals invalid asset to pool
var ErrAssetPoolInvalidAsset = errors.New("asset pool invalid kda")

// ErrAssetPoolNotActive signals asset pool is not active
var ErrAssetPoolNotActive = errors.New("asset pool is not active")

// ErrAssetPoolOutOfFunds signals asset pool does not have enough KLV to exchange
var ErrAssetPoolOutOfFunds = errors.New("asset pool out of funds")

// ErrAssetPoolAmountError signals amount sent for asset exchange does not cover min amount
var ErrAssetPoolAmountError = errors.New("asset pool amount error")

// ErrAssetPoolInvalidAddress signals invalid address accessing fee pool
var ErrAssetPoolInvalidAddress = errors.New("asset pool invalid address")

// ErrDeprecatedContract signals a contract that has been deprecated
var ErrDeprecatedContract = errors.New("deprecated contract")

// ErrITOAlreadyExists signals that an ito already exists
var ErrITOAlreadyExists = errors.New("ito already exists")

// ErrKappIteratorExceed signals that kapp iterator exceed the limit define
var ErrKappIteratorExceed = errors.New("kapp iterator exceed limit")

// ErrNilTxSimulatorProcessor signals that a nil transaction simulator processor has been provided
var ErrNilTxSimulatorProcessor = errors.New("nil transaction simulator processor")

// ErrNilTxLogsProcessor is the error returned when a transaction has no logs
var ErrNilTxLogsProcessor = errors.New("nil transaction logs processor")

// ErrNotEnoughGas signals that not enough gas has been provided
var ErrNotEnoughGas = errors.New("not enough gas was sent in the transaction")

// ErrEstimateGasTooBig signals that an error occurred while estimating gas
var ErrEstimateGasTooBig = errors.New("error while estimating gas, invalid amount")

// ErrInvalidType signals that an invalid type has been provided
var ErrInvalidType = errors.New("invalid type")

// ErrUint64Overflow signals that an overflow occurred while adding uint64 values
var ErrUint64Overflow = errors.New("uint64 overflow")

// ErrInt64Overflow signals that an overflow occurred while adding int64 values
var ErrInt64Overflow = errors.New("int64 overflow")

// ErrInvalidReceiverAddress signals that an invalid receiver address has been provided
var ErrInvalidReceiverAddress = errors.New("could not create receiver address from provided parameters")

// ErrInvalidContractSize signals contract size is too big
var ErrInvalidContractSize = errors.New("contract size is too big")

// ErrInvalidTransactionSize signals transaction size is too big
var ErrInvalidTransactionSize = errors.New("transaction size is too big")

// ErrInvalidTransactionRawSize signals transaction raw data size is too big
var ErrInvalidTransactionRawSize = errors.New("transaction raw size is too big")

// ErrMaxBytesExceeded signals that max bytes exceeded for decode buffer
var ErrMaxBytesExceeded = errors.New("max bytes exceeded")
