package process

import "errors"

// ErrNilQuotaStatusHandler signals that a nil quota status handler has been provided
var ErrNilQuotaStatusHandler = errors.New("nil quota status handler")

// ErrEmptyFloodPreventerList signals that an empty flood preventer list has been provided
var ErrEmptyFloodPreventerList = errors.New("empty flood preventer provided")

// ErrNilTopicFloodPreventer signals that a nil topic flood preventer has been provided
var ErrNilTopicFloodPreventer = errors.New("nil topic flood preventer")

// ErrNilBlackListCacher signals that a nil black list cacher was provided
var ErrNilBlackListCacher = errors.New("nil black list cacher")

// ErrOriginatorIsBlacklisted signals that a message originator is blacklisted on the current node
var ErrOriginatorIsBlacklisted = errors.New("originator is blacklisted")

// ErrOnlyValidatorsCanUseThisTopic signals that topic can be used by validator only
var ErrOnlyValidatorsCanUseThisTopic = errors.New("only validators can use this topic")

// ErrNilPeerValidatorMapper signals that nil peer validator mapper has been provided
var ErrNilPeerValidatorMapper = errors.New("nil peer validator mapper")

// ErrNilDebugger signals that a nil debug handler has been provided
var ErrNilDebugger = errors.New("nil debug handler")

// ErrCacheConfigInvalidSize signals that the cache parameter "size" is invalid
var ErrCacheConfigInvalidSize = errors.New("cache parameter [size] is not valid, it must be a positive number")

// ErrEmptyTopic signals that an empty topic has been provided
var ErrEmptyTopic = errors.New("empty topic")

// ErrNilMarshalizer signals that an operation has been attempted to or with a nil marshalizer implementation
var ErrNilMarshalizer = errors.New("nil marshalizer")

// ErrNilInterceptedDataFactory signals that a nil intercepted data factory was provided
var ErrNilInterceptedDataFactory = errors.New("nil intercepted data factory")

// ErrNilInterceptedDataProcessor signals that a nil intercepted data processor was provided
var ErrNilInterceptedDataProcessor = errors.New("nil intercepted data processor")

// ErrNilInterceptorThrottler signals that a nil interceptor throttler was provided
var ErrNilInterceptorThrottler = errors.New("nil interceptor throttler")

// ErrNilAntifloodHandler signals that a nil antiflood handler has been provided
var ErrNilAntifloodHandler = errors.New("nil antiflood handler")

// ErrNilWhiteListHandler signals that white list handler is nil
var ErrNilWhiteListHandler = errors.New("nil whitelist handler")

// ErrEmptyPeerID signals that an empty peer ID has been provided
var ErrEmptyPeerID = errors.New("empty peer ID")

// ErrNoDataInMessage signals that no data was found after parsing received p2p message
var ErrNoDataInMessage = errors.New("no data found in received message")

// ErrInterceptedDataNotForCurrentShard signals that intercepted data is not for current shard
var ErrInterceptedDataNotForCurrentShard = errors.New("intercepted data not for current shard")

// ErrInvalidTransactionVersion signals  that an invalid transaction version has been provided
var ErrInvalidTransactionVersion = errors.New("invalid transaction version")

// ErrInvalidChainID signals that an invalid chain ID has been provided
var ErrInvalidChainID = errors.New("invalid chain ID")

// ErrNilInterceptorContainer signals that nil interceptor container has been provided
var ErrNilInterceptorContainer = errors.New("nil interceptor container")

// ErrNilArgumentStruct signals that a function has received nil instead of an instantiated Arg... structure
var ErrNilArgumentStruct = errors.New("nil argument struct")

// ErrNilBuffer signals that a provided byte buffer is nil
var ErrNilBuffer = errors.New("provided byte buffer is nil")

// ErrEpochDoesNotMatch signals that epoch does not match between headers
var ErrEpochDoesNotMatch = errors.New("epoch does not match")

// ErrValidatorStatsRootHashDoesNotMatch signals that the root hash for the validator statistics does not match
var ErrValidatorStatsRootHashDoesNotMatch = errors.New("root hash for validator statistics does not match")

// ErrInsufficientFunds signals the funds are insufficient for the move balance operation but the
// transaction fee is covered by the current balance
var ErrInsufficientFunds = errors.New("insufficient funds")

// ErrAccountNotFound signals that the account was not found for the provided address
var ErrAccountNotFound = errors.New("account not found")

// ErrNilValidatorInfos signals that a nil validator infos has been provided
var ErrNilValidatorInfos = errors.New("nil validator infos")

// ErrNilHdrValidator signals that a nil header validator has been provided
var ErrNilHdrValidator = errors.New("nil header validator")

// ErrHeaderIsBlackListed signals that the header provided is black listed
var ErrHeaderIsBlackListed = errors.New("header is black listed")

// ErrNilBlockPool signals that a nil blocks pool was used
var ErrNilBlockPool = errors.New("nil block pool")

// ErrNilTxHash signals that an operation has been attempted with a nil hash
var ErrNilTxHash = errors.New("nil transaction hash")

// ErrInvalidRcvAddr signals that an invalid receiver address was provided
var ErrInvalidRcvAddr = errors.New("invalid receiver address")

// ErrInvalidSndAddr signals that an invalid sender address was provided
var ErrInvalidSndAddr = errors.New("invalid sender address")

// ErrInvalidOwnerAddr signals that an invalid owner address was provided
var ErrInvalidOwnerAddr = errors.New("invalid owner address")

// ErrInvalidAdminAddr signals that an invalid admin address was provided
var ErrInvalidAdminAddr = errors.New("invalid admin address")

// ErrInvalidRoleAddr signals that an invalid role address was provided
var ErrInvalidRoleAddr = errors.New("invalid role address")

// ErrInvalidSplitRoyaltiesAddr signals that an invalid split royalties address was provided
var ErrInvalidSplitRoyaltiesAddr = errors.New("invalid split royalties address")

// ErrInvalidWhitelistAddr signals that an invalid whitelist address was provided
var ErrInvalidWhitelistAddr = errors.New("invalid whitelist address")

// ErrMissingHeader signals that searched header is missing
var ErrMissingHeader = errors.New("missing header")

// ErrInvalidPeerAccount signals that a peer account is invalid
var ErrInvalidPeerAccount = errors.New("invalid peer account")

// ErrNilPubKeysBitmap signals that a operation has been attempted with a nil public keys bitmap
var ErrNilPubKeysBitmap = errors.New("nil public keys bitmap")

// ErrNotEnoughValidBlocksInStorage signals that bootstrap from storage failed due to not enough valid blocks stored
var ErrNotEnoughValidBlocksInStorage = errors.New("not enough valid blocks to start from storage")

// ErrNilSlotManager -
var ErrNilSlotManager = errors.New("nil slot manager")

// ErrNilBlockHeader signals that an operation has been attempted to or with a nil block header
var ErrNilBlockHeader = errors.New("nil block header")

// ErrNilTxBlockBody signals that an operation has been attempted to or with a nil tx block body
var ErrNilTxBlockBody = errors.New("nil tx block body")

// ErrTimeIsOut signals that time is out
var ErrTimeIsOut = errors.New("time is out")

// ErrMissingBody signals that body of the block is missing
var ErrMissingBody = errors.New("missing body")

// ErrNilUint64Converter signals that uint64converter is nil
var ErrNilUint64Converter = errors.New("unit64converter is nil")

// ErrMissingHashForHeaderNonce signals that hash of the block is missing
var ErrMissingHashForHeaderNonce = errors.New("missing hash for header nonce")

// ErrNilHeaderHandler signals that a nil header handler has been provided
var ErrNilHeaderHandler = errors.New("nil header handler")

// ErrNilTransaction signals that an operation has been attempted to or with a nil transaction
var ErrNilTransaction = errors.New("nil transaction")

// ErrInvalidTransactionType signals  that an invalid transaction type has been provided
var ErrInvalidTransactionType = errors.New("invalid transaction type")

// ErrInvalidTransactionFees signals that transaction fee is invalid
var ErrInvalidTransactionFees = errors.New("invalid transaction fees")

// ErrWrongNonceInBlock signals the nonce in block is different than expected nonce
var ErrWrongNonceInBlock = errors.New("wrong nonce in block")

// ErrBlockHashDoesNotMatch signals that header hash does not match with the previous one
var ErrBlockHashDoesNotMatch = errors.New("block hash does not match")

// ErrLowerSlotInBlock signals that a header slot is too low for processing it
var ErrLowerSlotInBlock = errors.New("header slot is lower than last committed")

// ErrNilBlockSizeThrottler signals that block size throttler si nil
var ErrNilBlockSizeThrottler = errors.New("block size throttler is nil")

// ErrNilTpsBenchmark signals that tps benchmark object is nil
var ErrNilTpsBenchmark = errors.New("tps benchmark object is nil")

// ErrRandSeedDoesNotMatch signals that random seed does not match with the previous one
var ErrRandSeedDoesNotMatch = errors.New("random seed do not match")

// ErrNilHaveTimeHandler signals that a nil have time handler func was provided
var ErrNilHaveTimeHandler = errors.New("nil have time handler")

// ErrAccountStateDirty signals that the accounts were modified before starting the current modification
var ErrAccountStateDirty = errors.New("accountState was dirty before starting to change")

// ErrRootStateDoesNotMatch signals that root state does not match
var ErrRootStateDoesNotMatch = errors.New("root state does not match")

// ErrTxRootHashDoesNotMatch signals that tx root hash does not match
var ErrTxRootHashDoesNotMatch = errors.New("tx root hash does not match")

// ErrNilTransactionCoordinator signals that transaction coordinator is nil
var ErrNilTransactionCoordinator = errors.New("transaction coordinator is nil")

// ErrMarshalWithoutSuccess signals that marshal some data was not done with success
var ErrMarshalWithoutSuccess = errors.New("marshal without success")

// ErrMissingTransaction signals that one transaction is missing
var ErrMissingTransaction = errors.New("missing transaction")

// ErrWrongTypeAssertion signals that an type assertion failed
var ErrWrongTypeAssertion = errors.New("wrong type assertion")

// ErrNilTxProcessor signals that a nil transactions processor was used
var ErrNilTxProcessor = errors.New("nil transactions processor")

// ErrNilBalanceComputationHandler signals that a nil balance computation handler has been provided
var ErrNilBalanceComputationHandler = errors.New("nil balance computation handler")

// ErrNilPreProcessorsContainer signals that preprocessors container is nil
var ErrNilPreProcessorsContainer = errors.New("preprocessors container is nil")

// ErrNilEconomicsFeeHandler signals that fee handler is nil
var ErrNilEconomicsFeeHandler = errors.New("nil economics fee handler")

// ErrNilTxFeeHandler signals that tx fee handler is nil
var ErrNilTxFeeHandler = errors.New("nil tx fee handler")

// ErrNilTransactionVersionChecker signals that provided transaction version checker is nil
var ErrNilTransactionVersionChecker = errors.New("nil transaction version checker")

// ErrNilAddressContainer signals that an operation has been attempted to or with a nil AddressContainer implementation
var ErrNilAddressContainer = errors.New("nil AddressContainer")

// ErrNilTransactionPool signals that a nil transaction pool was used
var ErrNilTransactionPool = errors.New("nil transaction pool")

// ErrNilShardedDataCacherNotifier signals that a nil sharded data cacher notifier has been provided
var ErrNilShardedDataCacherNotifier = errors.New("nil sharded data cacher notifier")

// ErrTxNotFound signals that a transaction has not found
var ErrTxNotFound = errors.New("transaction not found")

// ErrNilStorage signals that a nil storage has been provided
var ErrNilStorage = errors.New("nil storage")

// ErrInvalidTxInPool signals an invalid transaction in the transactions pool
var ErrInvalidTxInPool = errors.New("invalid transaction in the transactions pool")

// ErrNilIntermediateProcessorContainer signals that intermediate processors container is nil
var ErrNilIntermediateProcessorContainer = errors.New("intermediate processor container is nil")

// ErrNilTxValidator signals that a nil tx validator has been provided
var ErrNilTxValidator = errors.New("nil transaction validator")

// ErrFailedTransaction signals that transaction is of type failed.
var ErrFailedTransaction = errors.New("failed transaction")

// ErrInsufficientFee signals that the current balance doesn't have the required transaction fee
var ErrInsufficientFee = errors.New("insufficient balance for fees")

// ErrInvalidTransactionNoContract signals that transaction has no contract attached
var ErrInvalidTransactionNoContract = errors.New("no contract found in transaction")

// ErrInvalidTransactionNoContract signals that transaction time has expired
var ErrTransactionTimeExpired = errors.New("transaction time expired")

// ErrNilHasher signals that an operation has been attempted to or with a nil hasher implementation
var ErrNilHasher = errors.New("nil hasher")

// ErrNilPauseHandler signals that nil pause handler has been provided
var ErrNilPauseHandler = errors.New("nil pause handler")

// ErrNilRolesHandler signals that nil roles handler has been provided
var ErrNilRolesHandler = errors.New("nil roles handler")

// ErrKDATokenIsPaused signals that kda token is paused
var ErrKDATokenIsPaused = errors.New("kda token is paused")

// ErrKDAIsFrozenForAccount signals that account is frozen for given kda token
var ErrKDAIsFrozenForAccount = errors.New("account is frozen for this kda token")

// ErrCannotWipeAccountNotFrozen signals that account isn't frozen so the wipe is not possible
var ErrCannotWipeAccountNotFrozen = errors.New("cannot wipe because the account is not frozen for this kda token")

// ErrNilPayableHandler signals that nil payableHandler was provided
var ErrNilPayableHandler = errors.New("nil payableHandler was provided")

// ErrAccountNotPayable will be sent when trying to send money to a non-payable account
var ErrAccountNotPayable = errors.New("sending value to non payable contract")

// ErrNilValue signals the value is nil
var ErrNilValue = errors.New("nil value")

// ErrNilUserAccount signals that nil user account was provided
var ErrNilUserAccount = errors.New("nil user account")

// ErrActionNotAllowed signals that action is not allowed
var ErrActionNotAllowed = errors.New("action is not allowed")

// ErrNegativeValue signals that a negative value has been detected and it is not allowed
var ErrNegativeValue = errors.New("negative value")

// ErrOnlyFungibleTokensHaveBalanceTransfer signals that only fungible tokens have balance transfer
var ErrOnlyFungibleTokensHaveBalanceTransfer = errors.New("only fungible tokens have balance transfer")

// ErrNilAddressPubKeyConverter signals that the provided public key converter is nil
var ErrNilAddressPubKeyConverter = errors.New("nil address public key converter")

// ErrTokenNameNotHumanReadable signals that token name is not human readable
var ErrTokenNameNotHumanReadable = errors.New("token name is not human readable")

// ErrTickerNameNotValid signals that ticker name is not valid
var ErrTickerNameNotValid = errors.New("ticker name is not valid")

// ErrSupplyNotValid signals that supply is not valid
var ErrSupplyNotValid = errors.New("supply is not valid")

// ErrAssetIsPaused signals that the asset is paused
var ErrAssetIsPaused = errors.New("asset is paused")

// ErrKDATransferNotAllowed signals that kda transfer is not allowed and account from/to has no role to override
var ErrKDATransferNotAllowed = errors.New("kda transfer is not allowed")

// ErrInvalidArgument signals that invalid argument has been provided
var ErrInvalidArgument = errors.New("invalid argument")

var ErrInvalidContractOrRawDataSize = errors.New("RawData size does not match with len(contractID)")

var ErrNFTMetadataChangeStopped = errors.New("NFT metadata is immutable")

// ErrBlockProducerSignatureNotValid signals that block producer signature is not valid
var ErrBlockProducerSignatureNotValid = errors.New("block producer signature is not valid")

// ErrSameSenderAndReceiverAddress signals that the transfer can not be created with the same sender and receiver address
var ErrSameSenderAndReceiverAddress = errors.New("transfer can not be created with the same sender and receiver address")

// ErrOverflow signals that an overflow occured
var ErrOverflow = errors.New("type overflow occured")

// ErrIncreaseStepLowerThanOne signals that an increase step lower than one has been provided
var ErrIncreaseStepLowerThanOne = errors.New("increase step is lower than one")

// ErrNilCacher signals that a nil cache has been provided
var ErrNilCacher = errors.New("nil cacher")

// ErrNilBlackListedPkCache signals that a nil black listed public key cache has been provided
var ErrNilBlackListedPkCache = errors.New("nil black listed public key cache")

// ErrInvalidDecayCoefficient signals that the provided decay coefficient is invalid
var ErrInvalidDecayCoefficient = errors.New("decay coefficient is invalid")

// ErrInvalidDecayIntervalInSeconds signals that an invalid interval in seconds was provided
var ErrInvalidDecayIntervalInSeconds = errors.New("invalid decay interval in seconds")

// ErrInvalidMinScore signals that an invalid minimum score was provided
var ErrInvalidMinScore = errors.New("invalid minimum score")

// ErrInvalidMaxScore signals that an invalid maximum score was provided
var ErrInvalidMaxScore = errors.New("invalid maximum score")

// ErrInvalidUnitValue signals that an invalid unit value was provided
var ErrInvalidUnitValue = errors.New("invalid unit value")

// ErrInvalidBadPeerThreshold signals that an invalid bad peer threshold has been provided
var ErrInvalidBadPeerThreshold = errors.New("invalid bad peer threshold")

// ErrMaxRatingIsSmallerThanMinRating signals that the max rating is smaller than the min rating value
var ErrMaxRatingIsSmallerThanMinRating = errors.New("max rating is smaller than min rating")

// ErrMinRatingSmallerThanOne signals that the min rating is smaller than the min value of 1
var ErrMinRatingSmallerThanOne = errors.New("min rating is smaller than one")

// ErrStartRatingNotBetweenMinAndMax signals that the start rating is not between min and max rating
var ErrStartRatingNotBetweenMinAndMax = errors.New("start rating is not between min and max rating")

// ErrSignedBlocksThresholdNotBetweenZeroAndOne signals that the signed blocks threshold is not between 0 and 1
var ErrSignedBlocksThresholdNotBetweenZeroAndOne = errors.New("signed blocks threshold is not between 0 and 1")

// ErrConsecutiveMissedBlocksPenaltyLowerThanOne signals that the ConsecutiveMissedBlocksPenalty is lower than 1
var ErrConsecutiveMissedBlocksPenaltyLowerThanOne = errors.New("consecutive missed blocks penalty lower than 1")

// ErrDecreaseRatingsStepMoreThanMinusOne signals that the decrease rating step has a vale greater than -1
var ErrDecreaseRatingsStepMoreThanMinusOne = errors.New("decrease rating step has a value greater than -1")

// ErrHoursToMaxRatingFromStartRatingZero signals that the number of hours to reach max rating step is zero
var ErrHoursToMaxRatingFromStartRatingZero = errors.New("hours to reach max rating is zero")

// ErrDuplicateThreshold signals that two thresholds are the same
var ErrDuplicateThreshold = errors.New("two thresholds are the same")

// ErrNoChancesForMaxThreshold signals that the max threshold has no chance defined
var ErrNoChancesForMaxThreshold = errors.New("max threshold has no chances")

// ErrNoChancesProvided signals that there were no chances provided
var ErrNoChancesProvided = errors.New("no chances are provided")

// ErrNilMinChanceIfZero signals that there was no min chance provided if a chance is still needed
var ErrNilMinChanceIfZero = errors.New("no min chance ")

// ErrNilRatingsInfoHandler signals that nil ratings info handler has been provided
var ErrNilRatingsInfoHandler = errors.New("nil ratings info handler")

// ErrDuplicateBucketDelegation signals that two bucket delegations are being made to the same address
var ErrDuplicateBucketDelegation = errors.New("two bucket delegations to the same address")

// ErrNilFallbackHeaderValidator signals that a nil fallback header validator has been provided
var ErrNilFallbackHeaderValidator = errors.New("nil fallback header validator")

// ErrReservedFieldNotSupportedYet signals that reserved field is not empty
var ErrReservedFieldNotSupportedYet = errors.New("reserved field not supported yet")

// ErrBlockProposerSignatureMissing signals that block proposer signature is missing from the block aggregated sig
var ErrBlockProposerSignatureMissing = errors.New("block proposer signature is missing")

// ErrNilNetworkWatcher signals that a nil network watcher has been provided
var ErrNilNetworkWatcher = errors.New("nil network watcher")

// ErrHigherNonceInTransaction signals the nonce in transaction is higher than the account's nonce
var ErrHigherNonceInTransaction = errors.New("higher nonce in transaction")

// ErrLowerNonceInTransaction signals the nonce in transaction is lower than the account's nonce
var ErrLowerNonceInTransaction = errors.New("lower nonce in transaction")

// ErrWrongTransaction signals that transaction is invalid
var ErrWrongTransaction = errors.New("invalid transaction")

// ErrMinKFIStaked signals that minimun KFI staked requirement has not been reached
var ErrMinKFIStaked = errors.New("min KFI staked unreached")

// ErrInvalidMiningRewards signals a block was proposed with invalid rewards
var ErrInvalidMiningRewards = errors.New("invalid block mining rewards")

// ErrInvalidTXCount signals a block was proposed with invalid tx count
var ErrInvalidTXCount = errors.New("invalid block tx count")

// ErrInvalidTXFees signals a block was proposed with invalid tx fees
var ErrInvalidTXFees = errors.New("invalid block tx fees")

// ErrInvalidKAppsFees signals a block was proposed with invalid tx kapps fees
var ErrInvalidKAppsFees = errors.New("invalid block tx kapps fees")

// ErrInvalidBlockTimestamp signals invalid block timestamp
var ErrInvalidBlockTimestamp = errors.New("invalid block timestamp")

// ErrInvalidWithdrawType signals invalid withdraw type
var ErrInvalidWithdrawType = errors.New("invalid withdraw type")

// ErrProposalNotInitialized signals proposal controller is not initialized
var ErrProposalNotInitialized = errors.New("proposal controller not initialized")

// ErrComputeKDAFeeError signals can't compute kda fee error
var ErrComputeKDAFeeError = errors.New("can't compute kda fee error")

// ErrNilMessage signals that a nil message has been received
var ErrNilMessage = errors.New("nil message")

// ErrNilAccountsAdapter defines the error when trying to use a nil AccountsAddapter
var ErrNilAccountsAdapter = errors.New("nil AccountsAdapter")

// ErrNilCoreComponentsHolder signals that a nil core components holder was provided
var ErrNilCoreComponentsHolder = errors.New("nil core components holder")

// ErrNilBootstrapComponentsHolder signals that a nil bootstrap components holder was provided
var ErrNilBootstrapComponentsHolder = errors.New("nil bootstrap components holder")

// ErrNilStatusComponentsHolder signals that a nil status components holder was provided
var ErrNilStatusComponentsHolder = errors.New("nil status components holder")

// ErrNilStatusCoreComponentsHolder signals that a nil status core components holder was provided
var ErrNilStatusCoreComponentsHolder = errors.New("nil status core components holder")

// ErrNilCryptoComponentsHolder signals that a nil crypto components holder was provided
var ErrNilCryptoComponentsHolder = errors.New("nil crypto components holder")

// ErrNilDataComponentsHolder signals that a nil data components holder was provided
var ErrNilDataComponentsHolder = errors.New("nil data components holder")

// ErrNilPubkeyConverter signals that an operation has been attempted to or with a nil public key converter implementation
var ErrNilPubkeyConverter = errors.New("nil pubkey converter")

// ErrNilGasSchedule signals that an operation has been attempted with a nil gas schedule
var ErrNilGasSchedule = errors.New("nil GasSchedule")

// ErrNoVM signals that no SCHandler has been set
var ErrNoVM = errors.New("no VM (hook not set)")

// ErrNilBlockChain signals that an operation has been attempted to or with a nil blockchain
var ErrNilBlockChain = errors.New("nil block chain")

// ErrNilMetaBlockHeader signals that an operation has been attempted to or with a nil metablock
var ErrNilMetaBlockHeader = errors.New("nil metablock header")

// ErrNilStore signals that the provided storage service is nil
var ErrNilStore = errors.New("nil data storage service")

// ErrNilBootStorer signals that the provided boot storer is bil
var ErrNilBootStorer = errors.New("nil boot storer")

// ErrNilBlockBody signals that an operation has been attempted to or with a nil block body
var ErrNilBlockBody = errors.New("nil block body")

// ErrNilPreviousBlockHash signals that an operation has been attempted with a nil previous block header hash
var ErrNilPreviousBlockHash = errors.New("nil previous block header hash")

// ErrNilSignature signals that an operation has been attempted with a nil signature
var ErrNilSignature = errors.New("nil signature")

// ErrNilMiniBlocks signals that an operation has been attempted with a nil mini-block
var ErrNilMiniBlocks = errors.New("nil mini blocks")

// ErrNilMiniBlock signals that an operation has been attempted with a nil miniblock
var ErrNilMiniBlock = errors.New("nil mini block")

// ErrNilRootHash signals that an operation has been attempted with a nil root hash
var ErrNilRootHash = errors.New("root hash is nil")

// ErrInvalidShardId signals that the shard id is invalid
var ErrInvalidShardId = errors.New("invalid shard id")

// ErrNilBlockProcessor signals that an operation has been attempted to or with a nil BlockProcessor implementation
var ErrNilBlockProcessor = errors.New("nil block processor")

// ErrNilNodesConfigProvider signals that an operation has been attempted to or with a nil nodes config provider
var ErrNilNodesConfigProvider = errors.New("nil nodes config provider")

// ErrNilRoundHandler signals that an operation has been attempted to or with a nil RoundHandler implementation
var ErrNilRoundHandler = errors.New("nil RoundHandler")

// ErrNilRoundTimeDurationHandler signals that an operation has been attempted to or with a nil RoundTimeDurationHandler implementation
var ErrNilRoundTimeDurationHandler = errors.New("nil RoundTimeDurationHandler")

// ErrNilMessenger signals that a nil Messenger object was provided
var ErrNilMessenger = errors.New("nil Messenger")

// ErrNilTxDataPool signals that a nil transaction pool has been provided
var ErrNilTxDataPool = errors.New("nil transaction data pool")

// ErrNilHeadersDataPool signals that a nil headers pool has been provided
var ErrNilHeadersDataPool = errors.New("nil headers data pool")

// ErrNilRcvAddr signals that an operation has been attempted to or with a nil receiver address
var ErrNilRcvAddr = errors.New("nil receiver address")

// ErrNilSndAddr signals that an operation has been attempted to or with a nil sender address
var ErrNilSndAddr = errors.New("nil sender address")

// ErrNilShardCoordinator signals that an operation has been attempted to or with a nil shard coordinator
var ErrNilShardCoordinator = errors.New("nil shard coordinator")

// ErrNilNodesCoordinator signals that an operation has been attempted to or with a nil nodes coordinator
var ErrNilNodesCoordinator = errors.New("nil nodes coordinator")

// ErrNilKeyGen signals that an operation has been attempted to or with a nil single sign key generator
var ErrNilKeyGen = errors.New("nil key generator")

// ErrNilSingleSigner signals that a nil single signer is used
var ErrNilSingleSigner = errors.New("nil single signer")

// ErrNilMultiSigVerifier signals that a nil multi-signature verifier is used
var ErrNilMultiSigVerifier = errors.New("nil multi-signature verifier")

// ErrNilDataToProcess signals that nil data was provided
var ErrNilDataToProcess = errors.New("nil data to process")

// ErrNilPoolsHolder signals that an operation has been attempted to or with a nil pools holder object
var ErrNilPoolsHolder = errors.New("nil pools holder")

// ErrNilTxStorage signals that a nil transaction storage has been provided
var ErrNilTxStorage = errors.New("nil transaction storage")

// ErrNilMiniBlockPool signals that a nil mini blocks pool was used
var ErrNilMiniBlockPool = errors.New("nil mini block pool")

// ErrNilMetaBlocksPool signals that a nil meta blocks pool was used
var ErrNilMetaBlocksPool = errors.New("nil meta blocks pool")

// ErrNilDataPoolHolder signals that the data pool holder is nil
var ErrNilDataPoolHolder = errors.New("nil data pool holder")

// ErrNilForkDetector signals that the fork detector is nil
var ErrNilForkDetector = errors.New("nil fork detector")

// ErrNilContainerElement signals when trying to add a nil element in the container
var ErrNilContainerElement = errors.New("element cannot be nil")

// ErrInvalidContainerKey signals that an element does not exist in the container's map
var ErrInvalidContainerKey = errors.New("element does not exist in container")

// ErrContainerKeyAlreadyExists signals that an element was already set in the container's map
var ErrContainerKeyAlreadyExists = errors.New("provided key already exists in container")

// ErrNilRequestHandler signals that a nil request handler interface was provided
var ErrNilRequestHandler = errors.New("nil request handler")

// ErrWrongTypeInContainer signals that a wrong type of object was found in container
var ErrWrongTypeInContainer = errors.New("wrong type of object inside container")

// ErrLenMismatch signals that 2 or more slices have different lengths
var ErrLenMismatch = errors.New("lengths mismatch")

// ErrHeaderShardDataMismatch signals that shard header does not match created shard info
var ErrHeaderShardDataMismatch = errors.New("shard header does not match shard info")

// ErrNilRandSeed signals that a nil rand seed has been provided
var ErrNilRandSeed = errors.New("provided rand seed is nil")

// ErrNilPrevRandSeed signals that a nil previous rand seed has been provided
var ErrNilPrevRandSeed = errors.New("provided previous rand seed is nil")

// ErrReservedFieldInvalid signals that reserved field has an invalid content
var ErrReservedFieldInvalid = errors.New("reserved field content is invalid")

// ErrLowerRoundInBlock signals that a header round is too low for processing it
var ErrLowerRoundInBlock = errors.New("header round is lower than last committed")

// ErrHigherRoundInBlock signals that a block with higher round than permitted has been provided
var ErrHigherRoundInBlock = errors.New("higher round in block")

// ErrLowerNonceInBlock signals that a block with lower nonce than permitted has been provided
var ErrLowerNonceInBlock = errors.New("lower nonce in block")

// ErrHigherNonceInBlock signals that a block with higher nonce than permitted has been provided
var ErrHigherNonceInBlock = errors.New("higher nonce in block")

// ErrHeaderNotFinal signals that header is not final, and it should be
var ErrHeaderNotFinal = errors.New("header in metablock is not final")

// ErrShardIdMissmatch signals shard ID does not match expectations
var ErrShardIdMissmatch = errors.New("shard ID missmatch")

// ErrNotarizedHeadersSliceIsNil signals that the slice holding notarized headers is nil
var ErrNotarizedHeadersSliceIsNil = errors.New("notarized headers slice is nil")

// ErrNotarizedHeadersSliceForShardIsNil signals that the slice holding notarized headers for shard is nil
var ErrNotarizedHeadersSliceForShardIsNil = errors.New("notarized headers slice for shard is nil")

// ErrCrossShardMBWithoutConfirmationFromMeta signals that miniblock was not yet notarized by metachain
var ErrCrossShardMBWithoutConfirmationFromMeta = errors.New("cross shard miniblock with destination current shard is not confirmed by metachain")

// ErrHeaderBodyMismatch signals that the header does not attest all data from the block
var ErrHeaderBodyMismatch = errors.New("body cannot be validated from header data")

// ErrScheduledMiniBlocksMismatch signals that scheduled mini blocks created and executed in the last block, which are not yet final,
// do not match with the ones received in the next proposed body
var ErrScheduledMiniBlocksMismatch = errors.New("scheduled miniblocks does not match")

// ErrNilSmartContractProcessor signals that smart contract call executor is nil
var ErrNilSmartContractProcessor = errors.New("smart contract processor is nil")

// ErrNilArgumentParser signals that the argument parser is nil
var ErrNilArgumentParser = errors.New("argument parser is nil")

// ErrNilSCDestAccount signals that destination account is nil
var ErrNilSCDestAccount = errors.New("nil destination SC account")

// ErrWrongNonceInVMOutput signals that nonce in vm output is wrong
var ErrWrongNonceInVMOutput = errors.New("nonce invalid from SC run")

// ErrNilVMOutput signals that vmoutput is nil
var ErrNilVMOutput = errors.New("nil vm output")

// ErrNilValueFromRewardTransaction signals that the transfered value is nil
var ErrNilValueFromRewardTransaction = errors.New("transferred value is nil in reward transaction")

// ErrNilTemporaryAccountsHandler signals that temporary accounts handler is nil
var ErrNilTemporaryAccountsHandler = errors.New("temporary accounts handler is nil")

// ErrNilSmartContractResult signals that the smart contract result is nil
var ErrNilSmartContractResult = errors.New("smart contract result is nil")

// ErrNilRewardTransaction signals that the reward transaction is nil
var ErrNilRewardTransaction = errors.New("reward transaction is nil")

// ErrNilUTxDataPool signals that unsigned transaction pool is nil
var ErrNilUTxDataPool = errors.New("unsigned transactions pool is nil")

// ErrNilRewardTxDataPool signals that the reward transactions pool is nil
var ErrNilRewardTxDataPool = errors.New("reward transactions pool is nil")

// ErrNilUnsignedTxDataPool signals that the unsigned transactions pool is nil
var ErrNilUnsignedTxDataPool = errors.New("unsigned transactions pool is nil")

// ErrNilUTxStorage signals that unsigned transaction storage is nil
var ErrNilUTxStorage = errors.New("unsigned transactions storage is nil")

// ErrNilScAddress signals that a nil smart contract address has been provided
var ErrNilScAddress = errors.New("nil SC address")

// ErrEmptyFunctionName signals that an empty function name has been provided
var ErrEmptyFunctionName = errors.New("empty function name")

// ErrMiniBlockHashMismatch signals that miniblock hashes does not match
var ErrMiniBlockHashMismatch = errors.New("miniblocks does not match")

// ErrWrongTypeInMiniBlock signals that type is not correct for processing
var ErrWrongTypeInMiniBlock = errors.New("type in miniblock is not correct for processing")

// ErrNilSmartContractResultProcessor signals that smart contract result processor is nil
var ErrNilSmartContractResultProcessor = errors.New("nil smart contract result processor")

// ErrNilRewardsTxProcessor signals that the rewards transaction processor is nil
var ErrNilRewardsTxProcessor = errors.New("nil rewards transaction processor")

// ErrNilPreProcessor signals that preprocessors is nil
var ErrNilPreProcessor = errors.New("preprocessor is nil")

// ErrNilGasHandler signals that gas handler is nil
var ErrNilGasHandler = errors.New("nil gas handler")

// ErrUnknownBlockType signals that block type is not correct
var ErrUnknownBlockType = errors.New("block type is unknown")

// ErrMissingPreProcessor signals that required pre-processor is missing
var ErrMissingPreProcessor = errors.New("pre processor is missing")

// ErrNilAppStatusHandler defines the error for setting a nil AppStatusHandler
var ErrNilAppStatusHandler = errors.New("nil AppStatusHandler")

// ErrNilUnsignedTxHandler signals that the unsigned tx handler is nil
var ErrNilUnsignedTxHandler = errors.New("nil unsigned tx handler")

// ErrNilPeerAccountsAdapter signals that a nil peer accounts database was provided
var ErrNilPeerAccountsAdapter = errors.New("nil peer accounts database")

// ErrInvalidMetaHeader signals that a wrong implementation of HeaderHandler was provided
var ErrInvalidMetaHeader = errors.New("invalid header provided, expected MetaBlock")

// ErrNilEpochStartTrigger signals that a nil start of epoch trigger was provided
var ErrNilEpochStartTrigger = errors.New("nil start of epoch trigger")

// ErrNilEpochHandler signals that a nil epoch handler was provided
var ErrNilEpochHandler = errors.New("nil epoch handler")

// ErrNilEpochStartNotifier signals that the provided epochStartNotifier is nil
var ErrNilEpochStartNotifier = errors.New("nil epochStartNotifier")

// ErrNilEpochNotifier signals that the provided EpochNotifier is nil
var ErrNilEpochNotifier = errors.New("nil EpochNotifier")

// ErrInvalidCacheRefreshIntervalInSec signals that the cacheRefreshIntervalInSec is invalid - zero or less
var ErrInvalidCacheRefreshIntervalInSec = errors.New("invalid cacheRefreshIntervalInSec")

// ErrOverallBalanceChangeFromSC signals that all sumed balance changes are not zero
var ErrOverallBalanceChangeFromSC = errors.New("SC output balance updates are wrong")

// ErrNilPendingMiniBlocksHandler signals that a nil pending miniblocks handler has been provided
var ErrNilPendingMiniBlocksHandler = errors.New("nil pending miniblocks handler")

// ErrSystemBusy signals that the system is busy
var ErrSystemBusy = errors.New("system busy")

// ErrInsufficientGasLimitInTx signals that a lower gas limit than required was provided
var ErrInsufficientGasLimitInTx = errors.New("insufficient gas limit in tx")

// ErrInvalidMaxGasLimitPerBlock signals that an invalid max gas limit per block has been read from config file
var ErrInvalidMaxGasLimitPerBlock = errors.New("invalid max gas limit per block")

// ErrInvalidMaxGasLimitPerMiniBlock signals that an invalid max gas limit per mini block has been read from config file
var ErrInvalidMaxGasLimitPerMiniBlock = errors.New("invalid max gas limit per mini block")

// ErrInvalidMaxGasLimitPerMetaBlock signals that an invalid max gas limit per meta block has been read from config file
var ErrInvalidMaxGasLimitPerMetaBlock = errors.New("invalid max gas limit per meta block")

// ErrInvalidMaxGasLimitPerMetaMiniBlock signals that an invalid max gas limit per meta mini block has been read from config file
var ErrInvalidMaxGasLimitPerMetaMiniBlock = errors.New("invalid max gas limit per meta mini block")

// ErrInvalidMaxGasLimitPerTx signals that an invalid max gas limit per tx has been read from config file
var ErrInvalidMaxGasLimitPerTx = errors.New("invalid max gas limit per tx")

// ErrInvalidGasPerDataByte signals that an invalid gas per data byte has been read from config file
var ErrInvalidGasPerDataByte = errors.New("invalid gas per data byte")

// ErrMaxGasLimitPerMiniBlockInReceiverShardIsReached signals that max gas limit per mini block in receiver shard has been reached
var ErrMaxGasLimitPerMiniBlockInReceiverShardIsReached = errors.New("max gas limit per mini block in receiver shard is reached")

// ErrMaxGasLimitPerOneTxInReceiverShardIsReached signals that max gas limit per one transaction in receiver shard has been reached
var ErrMaxGasLimitPerOneTxInReceiverShardIsReached = errors.New("max gas limit per one transaction in receiver shard is reached")

// ErrMaxGasLimitPerBlockInSelfShardIsReached signals that max gas limit per block in self shard has been reached
var ErrMaxGasLimitPerBlockInSelfShardIsReached = errors.New("max gas limit per block in self shard is reached")

// ErrMaxGasLimitUsedForDestMeTxsIsReached signals that max gas limit used for dest me txs has been reached
var ErrMaxGasLimitUsedForDestMeTxsIsReached = errors.New("max gas limit used for dest me txs is reached")

// ErrInvalidMinimumGasLimitForTx signals that an invalid minimum gas limit for transactions has been read from config file
var ErrInvalidMinimumGasLimitForTx = errors.New("invalid minimum gas limit for transactions")

// ErrEmptyEpochRewardsConfig signals that the epoch rewards config is empty
var ErrEmptyEpochRewardsConfig = errors.New("the epoch rewards config is empty")

// ErrEmptyGasLimitSettings signals that the gas limit settings is empty
var ErrEmptyGasLimitSettings = errors.New("the gas limit settings is empty")

// ErrEmptyYearSettings signals that the year settings is empty
var ErrEmptyYearSettings = errors.New("the year settings is empty")

// ErrInvalidRewardsPercentages signals that rewards percentages are not correct
var ErrInvalidRewardsPercentages = errors.New("invalid rewards percentages")

// ErrInvalidInflationPercentages signals that inflation percentages are not correct
var ErrInvalidInflationPercentages = errors.New("invalid inflation percentages")

// ErrInvalidNonceRequest signals that invalid nonce was requested
var ErrInvalidNonceRequest = errors.New("invalid nonce request")

// ErrInvalidBlockRequestOldEpoch signals that invalid block was requested from old epoch
var ErrInvalidBlockRequestOldEpoch = errors.New("invalid block request from old epoch")

// ErrNilBlockChainHook signals that nil blockchain hook has been provided
var ErrNilBlockChainHook = errors.New("nil blockchain hook")

// ErrNilTxForCurrentBlockHandler signals that nil tx for current block handler has been provided
var ErrNilTxForCurrentBlockHandler = errors.New("nil tx for current block handler")

// ErrNilSCToProtocol signals that nil smart contract to protocol handler has been provided
var ErrNilSCToProtocol = errors.New("nil sc to protocol")

// ErrNilNodesSetup signals that nil nodes setup has been provided
var ErrNilNodesSetup = errors.New("nil nodes setup")

// ErrNilPeerShardMapper signals that a nil peer shard mapper has been provided
var ErrNilPeerShardMapper = errors.New("nil peer shard mapper")

// ErrNilBlockTracker signals that a nil block tracker was provided
var ErrNilBlockTracker = errors.New("nil block tracker")

// ErrNilEconomicsData signals that nil economics data has been provided
var ErrNilEconomicsData = errors.New("nil economics data")

// ErrNilOutportDataProvider signals that a nil outport data provider has been given
var ErrNilOutportDataProvider = errors.New("nil outport data  provider")

// ErrZeroMaxComputableRounds signals that a value of zero was provided on the maxComputableRounds
var ErrZeroMaxComputableRounds = errors.New("max computable rounds is zero")

// ErrZeroMaxConsecutiveRoundsOfRatingDecrease signals that a value of zero was provided on the MaxConsecutiveRoundsOfRatingDecrease
var ErrZeroMaxConsecutiveRoundsOfRatingDecrease = errors.New("max consecutive number of rounds, in which we can decrease a validator rating, is zero")

// ErrNilRater signals that nil rater has been provided
var ErrNilRater = errors.New("nil rater")

// ErrNilHeaderValidator signals that nil header validator has been provided
var ErrNilHeaderValidator = errors.New("nil header validator")

// ErrSCDeployFromSCRIsNotPermitted signals that operation is not permitted
var ErrSCDeployFromSCRIsNotPermitted = errors.New("it is not permitted to deploy a smart contract from another smart contract cross shard")

// ErrNotEnoughGas signals that not enough gas has been provided
var ErrNotEnoughGas = errors.New("not enough gas was sent in the transaction")

// ErrInvalidValue signals that an invalid value was provided
var ErrInvalidValue = errors.New("invalid value provided")

// ErrNilHeaderSigVerifier signals that a nil header sig verifier has been provided
var ErrNilHeaderSigVerifier = errors.New("nil header sig verifier")

// ErrNilHeaderIntegrityVerifier signals that a nil header integrity verifier has been provided
var ErrNilHeaderIntegrityVerifier = errors.New("nil header integrity verifier")

// ErrNilBadTxHandler signals that bad tx handler is nil
var ErrNilBadTxHandler = errors.New("nil bad tx handler")

// ErrNilReceiptHandler signals that receipt handler is nil
var ErrNilReceiptHandler = errors.New("nil receipt handler")

// ErrTooManyReceiptsMiniBlocks signals that there were too many receipts miniblocks created
var ErrTooManyReceiptsMiniBlocks = errors.New("too many receipts miniblocks")

// ErrReceiptsHashMissmatch signals that overall receipts has does not match
var ErrReceiptsHashMissmatch = errors.New("receipts hash missmatch")

// ErrMiniBlockNumMissMatch signals that number of miniblocks does not match
var ErrMiniBlockNumMissMatch = errors.New("num miniblocks does not match")

// ErrEpochStartDataDoesNotMatch signals that EpochStartData is not the same as the leader created
var ErrEpochStartDataDoesNotMatch = errors.New("epoch start data does not match")

// ErrNotEpochStartBlock signals that block is not of type epoch start
var ErrNotEpochStartBlock = errors.New("not epoch start block")

// ErrGettingShardDataFromEpochStartData signals that could not get shard data from previous epoch start block
var ErrGettingShardDataFromEpochStartData = errors.New("could not find shard data from previous epoch start metablock")

// ErrNilValidityAttester signals that a nil validity attester has been provided
var ErrNilValidityAttester = errors.New("nil validity attester")

// ErrNilMiniBlocksProvider signals that a nil miniblocks data provider has been passed over
var ErrNilMiniBlocksProvider = errors.New("nil miniblocks provider")

// ErrNilPreferredPeersHolder signals that preferred peers holder is nil
var ErrNilPreferredPeersHolder = errors.New("nil preferred peers holder")

// ErrMiniBlocksInWrongOrder signals the miniblocks are in wrong order
var ErrMiniBlocksInWrongOrder = errors.New("miniblocks in wrong order, should have been only from me")

// ErrInvalidArguments signals that invalid arguments were given to process built-in function
var ErrInvalidArguments = errors.New("invalid arguments to process built-in function")

// ErrNilBuiltInFunction signals that built-in function is nil
var ErrNilBuiltInFunction = errors.New("built in function is nil")

// ErrRewardMiniBlockNotFromMeta signals that miniblock has a different sender shard than meta
var ErrRewardMiniBlockNotFromMeta = errors.New("rewards miniblocks should come only from meta")

// ErrValidatorInfoMiniBlockNotFromMeta signals that miniblock has a different sender shard than meta
var ErrValidatorInfoMiniBlockNotFromMeta = errors.New("validatorInfo miniblocks should come only from meta")

// ErrAccumulatedFeesDoNotMatch signals that accumulated fees do not match
var ErrAccumulatedFeesDoNotMatch = errors.New("accumulated fees do not match")

// ErrDeveloperFeesDoNotMatch signals that developer fees do not match
var ErrDeveloperFeesDoNotMatch = errors.New("developer fees do not match")

// ErrAccumulatedFeesInEpochDoNotMatch signals that accumulated fees in epoch do not match
var ErrAccumulatedFeesInEpochDoNotMatch = errors.New("accumulated fees in epoch do not match")

// ErrDevFeesInEpochDoNotMatch signals that developer fees in epoch do not match
var ErrDevFeesInEpochDoNotMatch = errors.New("developer fees in epoch do not match")

// ErrNilRewardsHandler signals that rewards handler is nil
var ErrNilRewardsHandler = errors.New("rewards handler is nil")

// ErrNilEpochEconomics signals that nil end of epoch econimics was provided
var ErrNilEpochEconomics = errors.New("nil epoch economics")

// ErrNilEpochStartDataCreator signals that nil epoch start data creator was provided
var ErrNilEpochStartDataCreator = errors.New("nil epoch start data creator")

// ErrNilRewardsCreator signals that nil epoch start rewards creator was provided
var ErrNilRewardsCreator = errors.New("nil epoch start rewards creator")

// ErrNilEpochStartValidatorInfoCreator signals that nil epoch start validator info creator was provided
var ErrNilEpochStartValidatorInfoCreator = errors.New("nil epoch start validator info creator")

// ErrInvalidGenesisTotalSupply signals that invalid genesis total supply was provided
var ErrInvalidGenesisTotalSupply = errors.New("invalid genesis total supply")

// ErrInvalidShardCacherIdentifier signals an invalid identifier
var ErrInvalidShardCacherIdentifier = errors.New("invalid identifier for shard cacher")

// ErrMaxBlockSizeReached signals that max block size has been reached
var ErrMaxBlockSizeReached = errors.New("max block size has been reached")

// ErrBlockBodyHashMismatch signals that block body hashes does not match
var ErrBlockBodyHashMismatch = errors.New("block bodies does not match")

// ErrInvalidMiniBlockType signals that an invalid miniblock type has been provided
var ErrInvalidMiniBlockType = errors.New("invalid miniblock type")

// ErrInvalidBody signals that an invalid body has been provided
var ErrInvalidBody = errors.New("invalid body")

// ErrNilBlockSizeComputationHandler signals that a nil block size computation handler has been provided
var ErrNilBlockSizeComputationHandler = errors.New("nil block size computation handler")

// ErrNilValidatorStatistics signals that a nil validator statistics has been provided
var ErrNilValidatorStatistics = errors.New("nil validator statistics")

// ErrMaxRatingZero signals that maxrating with a value of zero has been provided
var ErrMaxRatingZero = errors.New("max rating is zero")

// ErrNilHistoryRepository signals that history processor is nil
var ErrNilHistoryRepository = errors.New("history repository is nil")

// ErrInvalidMetaTransaction signals that meta transaction is invalid
var ErrInvalidMetaTransaction = errors.New("meta transaction is invalid")

// ErrLogNotFound is the error returned when a transaction has no logs
var ErrLogNotFound = errors.New("no logs for queried transaction")

// ErrNilTxLogsProcessor is the error returned when a transaction has no logs
var ErrNilTxLogsProcessor = errors.New("nil transaction logs processor")

// ErrNilVmInput signals that provided vm input is nil
var ErrNilVmInput = errors.New("nil vm input")

// ErrNilDnsAddresses signals that nil dns addresses map was provided
var ErrNilDnsAddresses = errors.New("nil dns addresses map")

// ErrNilProtocolSustainabilityAddress signals that a nil protocol sustainability address was provided
var ErrNilProtocolSustainabilityAddress = errors.New("nil protocol sustainability address")

// ErrUserNameDoesNotMatch signals that username does not match
var ErrUserNameDoesNotMatch = errors.New("user name does not match")

// ErrUserNameDoesNotMatchInCrossShardTx signals that username does not match in case of cross shard tx
var ErrUserNameDoesNotMatchInCrossShardTx = errors.New("mismatch between receiver username and address")

// ErrShardIsStuck signals that a shard is stuck
var ErrShardIsStuck = errors.New("shard is stuck")

// ErrRelayedTxBeneficiaryDoesNotMatchReceiver signals that an invalid address was provided in the relayed tx
var ErrRelayedTxBeneficiaryDoesNotMatchReceiver = errors.New("invalid address in relayed tx")

// ErrInvalidVMType signals that invalid vm type was provided
var ErrInvalidVMType = errors.New("invalid VM type")

// ErrRecursiveRelayedTxIsNotAllowed signals that recursive relayed tx is not allowed
var ErrRecursiveRelayedTxIsNotAllowed = errors.New("recursive relayed tx is not allowed")

// ErrRelayedTxValueHigherThenUserTxValue signals that relayed tx value is higher than user tx value
var ErrRelayedTxValueHigherThenUserTxValue = errors.New("relayed tx value is higher than user tx value")

// ErrTxValueTooBig signals that transaction value is too big
var ErrTxValueTooBig = errors.New("tx value is too big")

// ErrInvalidUserNameLength signals that provided username length is invalid
var ErrInvalidUserNameLength = errors.New("invalid user name length")

// ErrTxValueOutOfBounds signals that transaction value is out of bounds
var ErrTxValueOutOfBounds = errors.New("tx value is out of bounds")

// ErrTransactionIsNotWhitelisted signals that a transaction is not whitelisted
var ErrTransactionIsNotWhitelisted = errors.New("transaction is not whitelisted")

// ErrTrieNodeIsNotWhitelisted signals that a trie node is not whitelisted
var ErrTrieNodeIsNotWhitelisted = errors.New("trie node is not whitelisted")

// ErrNilOutportHandler signals that outport is nil
var ErrNilOutportHandler = errors.New("outport handler is nil")

// ErrSmartContractDeploymentIsDisabled signals that smart contract deployment was disabled
var ErrSmartContractDeploymentIsDisabled = errors.New("smart Contract deployment is disabled")

// ErrUpgradeNotAllowed signals that upgrade is not allowed
var ErrUpgradeNotAllowed = errors.New("upgrade is allowed only for owner")

// ErrBuiltInFunctionsAreDisabled signals that built-in functions are disabled
var ErrBuiltInFunctionsAreDisabled = errors.New("built in functions are disabled")

// ErrRelayedTxDisabled signals that relayed tx are disabled
var ErrRelayedTxDisabled = errors.New("relayed tx is disabled")

// ErrRelayedTxV2Disabled signals that the v2 version of relayed tx is disabled
var ErrRelayedTxV2Disabled = errors.New("relayed tx v2 is disabled")

// ErrRelayedTxV2ZeroVal signals that the v2 version of relayed tx should be created with 0 as value
var ErrRelayedTxV2ZeroVal = errors.New("relayed tx v2 value should be 0")

// ErrEmptyConsensusGroup is raised when an operation is attempted with an empty consensus group
var ErrEmptyConsensusGroup = errors.New("consensusGroup is empty")

// ErrNilEpochStartSystemSCProcessor signals that nil epoch start system sc processor was provided
var ErrNilEpochStartSystemSCProcessor = errors.New("nil epoch start system sc processor")

// ErrTransactionSignedWithHashIsNotEnabled signals that a transaction signed with hash is not enabled
var ErrTransactionSignedWithHashIsNotEnabled = errors.New("transaction signed with hash is not enabled")

// ErrInvalidRewardsTopUpGradientPoint signals that the top-up gradient point is invalid
var ErrInvalidRewardsTopUpGradientPoint = errors.New("rewards top up gradient point is invalid")

// ErrInvalidVMInputGasComputation signals that invalid vm input gas computation was provided
var ErrInvalidVMInputGasComputation = errors.New("invalid vm input gas computation")

// ErrMoreGasConsumedThanProvided signals that VM used more gas than provided
var ErrMoreGasConsumedThanProvided = errors.New("more gas used than provided")

// ErrInvalidGasModifier signals that provided gas modifier is invalid
var ErrInvalidGasModifier = errors.New("invalid gas modifier")

// ErrMoreGasThanGasLimitPerBlock signals that more gas was provided than gas limit per block
var ErrMoreGasThanGasLimitPerBlock = errors.New("more gas was provided than gas limit per block")

// ErrMoreGasThanGasLimitPerMiniBlockForSafeCrossShard signals that more gas was provided than gas limit per mini block for safe cross shard
var ErrMoreGasThanGasLimitPerMiniBlockForSafeCrossShard = errors.New("more gas was provided than gas limit per mini block for safe cross shard")

// ErrNotEnoughGasInUserTx signals that not enough gas was provided in user tx
var ErrNotEnoughGasInUserTx = errors.New("not enough gas provided in user tx")

// ErrNegativeBalanceDeltaOnCrossShardAccount signals that negative balance delta was given on cross shard account
var ErrNegativeBalanceDeltaOnCrossShardAccount = errors.New("negative balance delta on cross shard account")

// ErrNilOrEmptyList signals that a nil or empty list was provided
var ErrNilOrEmptyList = errors.New("nil or empty provided list")

// ErrNilScQueryElement signals that a nil sc query service element was provided
var ErrNilScQueryElement = errors.New("nil SC query service element")

// ErrMaxAccumulatedFeesExceeded signals that max accumulated fees has been exceeded
var ErrMaxAccumulatedFeesExceeded = errors.New("max accumulated fees has been exceeded")

// ErrMaxDeveloperFeesExceeded signals that max developer fees has been exceeded
var ErrMaxDeveloperFeesExceeded = errors.New("max developer fees has been exceeded")

// ErrNilBuiltInFunctionsCostHandler signals that a nil built-in functions cost handler has been provided
var ErrNilBuiltInFunctionsCostHandler = errors.New("nil built in functions cost handler")

// ErrNilArgsBuiltInFunctionsConstHandler signals that a nil arguments struct for built-in functions cost handler has been provided
var ErrNilArgsBuiltInFunctionsConstHandler = errors.New("nil arguments for built in functions cost handler")

// ErrInvalidEpochStartMetaBlockConsensusPercentage signals that a small epoch start meta block consensus percentage has been provided
var ErrInvalidEpochStartMetaBlockConsensusPercentage = errors.New("invalid epoch start meta block consensus percentage")

// ErrNilNumConnectedPeersProvider signals that a nil number of connected peers provider has been provided
var ErrNilNumConnectedPeersProvider = errors.New("nil number of connected peers provider")

// ErrNilLocker signals that a nil locker was provided
var ErrNilLocker = errors.New("nil locker")

// ErrNilAllowExternalQueriesChan signals that a nil channel for signaling the allowance of external queries provided is nil
var ErrNilAllowExternalQueriesChan = errors.New("nil channel for signaling the allowance of external queries")

// ErrQueriesNotAllowedYet signals that the node is not ready yet to process VM Queries
var ErrQueriesNotAllowedYet = errors.New("node is not ready yet to process VM Queries")

// ErrNilChunksProcessor signals that a nil chunks processor has been provided
var ErrNilChunksProcessor = errors.New("nil chunks processor")

// ErrIncompatibleReference signals that an incompatible reference was provided when processing a batch
var ErrIncompatibleReference = errors.New("incompatible reference when processing batch")

// ErrProcessClosed signals that an incomplete processing occurred due to the early process closing
var ErrProcessClosed = errors.New("incomplete processing: process is closing")

// ErrNilAccountsDBSyncer signals that a nil accounts db syncer has been provided
var ErrNilAccountsDBSyncer = errors.New("nil accounts DB syncer")

// ErrNilCurrentNetworkEpochProvider signals that a nil CurrentNetworkEpochProvider handler has been provided
var ErrNilCurrentNetworkEpochProvider = errors.New("nil current network epoch provider")

// ErrNilKDATransferParser signals that a nil KDA transfer parser has been provided
var ErrNilKDATransferParser = errors.New("nil kda transfer parser")

// ErrResultingSCRIsTooBig signals that resulting smart contract result is too big
var ErrResultingSCRIsTooBig = errors.New("resulting SCR is too big")

// ErrNotAllowedToWriteUnderProtectedKey signals that writing under protected key is not allowed
var ErrNotAllowedToWriteUnderProtectedKey = errors.New("not allowed to write under protected key")

// ErrNilBootstrapper signals that a nil bootstraper has been provided
var ErrNilBootstrapper = errors.New("nil bootstrapper")

// ErrNodeIsNotSynced signals that the VM query cannot be executed because the node is not synced and the request required this
var ErrNodeIsNotSynced = errors.New("node is not synced")

// ErrStateChangedWhileExecutingVmQuery signals that the state has been changed while executing a vm query and the request required not to
var ErrStateChangedWhileExecutingVmQuery = errors.New("state changed while executing vm query")

// ErrNilEnableRoundsHandler signals a nil enable rounds handler has been provided
var ErrNilEnableRoundsHandler = errors.New("nil enable rounds handler has been provided")

// ErrNilScheduledTxsExecutionHandler signals that scheduled txs execution handler is nil
var ErrNilScheduledTxsExecutionHandler = errors.New("nil scheduled txs execution handler")

// ErrNilVersionedHeaderFactory signals that the versioned header factory is nil
var ErrNilVersionedHeaderFactory = errors.New("nil versioned header factory")

// ErrNilIntermediateProcessor signals that intermediate processors is nil
var ErrNilIntermediateProcessor = errors.New("intermediate processor is nil")

// ErrNilSyncTimer signals that the sync timer is nil
var ErrNilSyncTimer = errors.New("sync timer is nil")

// ErrNilIsShardStuckHandler signals a nil shard stuck handler
var ErrNilIsShardStuckHandler = errors.New("nil handler for checking stuck shard")

// ErrNilIsMaxBlockSizeReachedHandler signals a nil max block size reached handler
var ErrNilIsMaxBlockSizeReachedHandler = errors.New("nil handler for max block size reached")

// ErrNilTxMaxTotalCostHandler signals a nil transaction max total cost
var ErrNilTxMaxTotalCostHandler = errors.New("nil transaction max total cost")

// ErrNilAccountTxsPerShard signals a nil mapping for account transactions to shard
var ErrNilAccountTxsPerShard = errors.New("nil account transactions per shard mapping")

// ErrScheduledRootHashDoesNotMatch signals that scheduled root hash does not match
// var ErrScheduledRootHashDoesNotMatch = errors.New("scheduled root hash does not match")

// ErrNilAdditionalData signals that additional data is nil
var ErrNilAdditionalData = errors.New("nil additional data")

// ErrNumOfMiniBlocksAndMiniBlocksHeadersMismatch signals that number of mini blocks and mini blocks headers does not match
var ErrNumOfMiniBlocksAndMiniBlocksHeadersMismatch = errors.New("num of mini blocks and mini blocks headers does not match")

// ErrNilDoubleTransactionsDetector signals that a nil double transactions detector has been provided
var ErrNilDoubleTransactionsDetector = errors.New("nil double transactions detector")

// ErrNoTxToProcess signals that no transaction were sent for processing
var ErrNoTxToProcess = errors.New("no transaction to process")

// ErrInvalidPeerSubType signals that an invalid peer subtype was provided
var ErrInvalidPeerSubType = errors.New("invalid peer subtype")

// ErrNilSignaturesHandler signals that a nil signatures handler was provided
var ErrNilSignaturesHandler = errors.New("nil signatures handler")

// ErrMessageExpired signals that a received message is expired
var ErrMessageExpired = errors.New("message expired")

// ErrInvalidExpiryTimespan signals that an invalid expiry timespan was provided
var ErrInvalidExpiryTimespan = errors.New("invalid expiry timespan")

// ErrNilPeerSignatureHandler signals that a nil peer signature handler was provided
var ErrNilPeerSignatureHandler = errors.New("nil peer signature handler")

// ErrNilPeerAuthenticationCacher signals that a nil peer authentication cacher was provided
var ErrNilPeerAuthenticationCacher = errors.New("nil peer authentication cacher")

// ErrNilHeartbeatCacher signals that a nil heartbeat cacher was provided
var ErrNilHeartbeatCacher = errors.New("nil heartbeat cacher")

// ErrInvalidProcessWaitTime signals that an invalid process wait time was provided
var ErrInvalidProcessWaitTime = errors.New("invalid process wait time")

// ErrMetaHeaderEpochOutOfRange signals that the given header is out of accepted range
var ErrMetaHeaderEpochOutOfRange = errors.New("epoch out of range for meta block header")

// ErrNilHardforkTrigger signals that a nil hardfork trigger has been provided
var ErrNilHardforkTrigger = errors.New("nil hardfork trigger")

// ErrMissingMiniBlockHeader signals that mini block header is missing
var ErrMissingMiniBlockHeader = errors.New("missing mini block header")

// ErrMissingMiniBlock signals that mini block is missing
var ErrMissingMiniBlock = errors.New("missing mini block")

// ErrIndexIsOutOfBound signals that the given index is out of bound
var ErrIndexIsOutOfBound = errors.New("index is out of bound")

// ErrIndexDoesNotMatchWithPartialExecutedMiniBlock signals that the given index does not match with a partial executed mini block
var ErrIndexDoesNotMatchWithPartialExecutedMiniBlock = errors.New("index does not match with a partial executed mini block")

// ErrIndexDoesNotMatchWithFullyExecutedMiniBlock signals that the given index does not match with a fully executed mini block
var ErrIndexDoesNotMatchWithFullyExecutedMiniBlock = errors.New("index does not match with a fully executed mini block")

// ErrNilProcessedMiniBlocksTracker signals that a nil processed mini blocks tracker has been provided
var ErrNilProcessedMiniBlocksTracker = errors.New("nil processed mini blocks tracker")

// ErrNilReceiptsRepository signals that a nil receipts repository has been provided
var ErrNilReceiptsRepository = errors.New("nil receipts repository")

// ErrNilEnableEpochsHandler signals that a nil enable epochs handler has been provided
var ErrNilEnableEpochsHandler = errors.New("nil enable epochs handler")

// ErrNilMultiSignerContainer signals that the given multisigner container is nil
var ErrNilMultiSignerContainer = errors.New("nil multiSigner container")

// ErrNilCrawlerAllowedAddress signals that no crawler allowed address was found
var ErrNilCrawlerAllowedAddress = errors.New("nil crawler allowed address")

// ErrNilPayloadValidator signals that a nil payload validator was provided
var ErrNilPayloadValidator = errors.New("nil payload validator")

// ErrNilValidatorInfoPool signals that a nil validator info pool has been provided
var ErrNilValidatorInfoPool = errors.New("nil validator info pool")

// ErrPropertyTooLong signals that a heartbeat property was too long
var ErrPropertyTooLong = errors.New("property too long")

// ErrPropertyTooShort signals that a heartbeat property was too short
var ErrPropertyTooShort = errors.New("property too short")

// ErrNilProcessDebugger signals that a nil process debugger was provided
var ErrNilProcessDebugger = errors.New("nil process debugger")

// ErrMaxCallsReached signals that the allowed max number of calls was reached
var ErrMaxCallsReached = errors.New("max calls reached")

// ErrInvalidNumberOfBlockTxs signals a block was proposed with invalid number of txs
var ErrInvalidNumberOfBlockTxs = errors.New("invalid number of block txs")

// ErrSmartContractDeploymentFailed signals that smart contract deployment has failed
var ErrSmartContractDeploymentFailed = errors.New("smart Contract deployment failed")

// ErrSmartContractInvokeFailed signals that smart contract invoke has failed
var ErrSmartContractInvokeFailed = errors.New("smart Contract invoke failed")
