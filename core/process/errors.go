package process

import (
	"errors"

	"github.com/klever-io/klever-go/common"
)

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

// ErrTooManyItemsInBatch aliases common.ErrTooManyItemsInBatch so every Batch consumer
// shares a single sentinel under errors.Is.
var ErrTooManyItemsInBatch = common.ErrTooManyItemsInBatch

// ErrBatchWireTooLarge aliases common.ErrBatchWireTooLarge for the byte-size
// pre-Unmarshal rejection path; see common.ErrBatchWireTooLarge.
var ErrBatchWireTooLarge = common.ErrBatchWireTooLarge

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

// ErrNilTxBlockHeader signals that an operation has been attempted to or with a nil tx block header
var ErrNilTxBlockHeader = errors.New("nil tx block header")

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

// ErrTxRootHashInvalidForEmptyBlock signals that tx root hash is invalid for empty block case
var ErrTxRootHashInvalidForEmptyBlock = errors.New("invalid tx root hash for empty block")

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

// ErrNilForkController signals that provided fork controller is nil
var ErrNilForkController = errors.New("nil fork controller")

var ErrStoreProtectedKey = errors.New("cannot persist storage update to protected key")

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

// ErrContractAccountNotAllowed signals that transfers to uninitialized contract addresses are not allowed
var ErrContractAccountNotAllowed = errors.New("transfers to uninitialized contract addresses are not allowed")

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

// ErrTransactionResultMismatch signals that transaction execution result does not match consensus
var ErrTransactionResultMismatch = errors.New("transaction result does not match consensus")

// ErrTransactionResultMismatchAcceptLeader signals that transaction execution result does not match consensus - accept leader
var ErrTransactionResultMismatchAcceptLeader = errors.New("transaction result does not match consensus - accept leader")

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

// ErrProposalNotInitialized signals proposal controller is not initialized
var ErrProposalNotInitialized = errors.New("proposal controller not initialized")

// ErrComputeKDAFeeError signals can't compute kda fee error
var ErrComputeKDAFeeError = errors.New("can't compute kda fee error")

// ErrNilMessage signals that a nil message has been received
var ErrNilMessage = errors.New("nil message")

// ErrNilAccountsAdapter defines the error when trying to use a nil AccountsAddapter
var ErrNilAccountsAdapter = errors.New("nil AccountsAdapter")

// ErrNilPubkeyConverter signals that an operation has been attempted to or with a nil public key converter implementation
var ErrNilPubkeyConverter = errors.New("nil pubkey converter")

// ErrNilKAppsController signals that a nil KAppsController has been provided
var ErrNilKAppsController = errors.New("nil KAppsController")

// ErrNilGasSchedule signals that an operation has been attempted with a nil gas schedule
var ErrNilGasSchedule = errors.New("nil GasSchedule")

// ErrNoVM signals that no SCHandler has been set
var ErrNoVM = errors.New("no VM (hook not set)")

// ErrNilBlockChain signals that an operation has been attempted to or with a nil blockchain
var ErrNilBlockChain = errors.New("nil block chain")

// ErrNilStore signals that the provided storage service is nil
var ErrNilStore = errors.New("nil data storage service")

// ErrNilBootStorer signals that the provided boot storer is bil
var ErrNilBootStorer = errors.New("nil boot storer")

// ErrNilSignature signals that an operation has been attempted with a nil signature
var ErrNilSignature = errors.New("nil signature")

// ErrNilBlockProcessor signals that an operation has been attempted to or with a nil BlockProcessor implementation
var ErrNilBlockProcessor = errors.New("nil block processor")

// ErrNilNodesConfigProvider signals that an operation has been attempted to or with a nil nodes config provider
var ErrNilNodesConfigProvider = errors.New("nil nodes config provider")

// ErrNilMessenger signals that a nil Messenger object was provided
var ErrNilMessenger = errors.New("nil Messenger")

// ErrNilTxDataPool signals that a nil transaction pool has been provided
var ErrNilTxDataPool = errors.New("nil transaction data pool")

// ErrNilHeadersDataPool signals that a nil headers pool has been provided
var ErrNilHeadersDataPool = errors.New("nil headers data pool")

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

// ErrNilPrevRandSeed signals that a nil previous rand seed has been provided
var ErrNilPrevRandSeed = errors.New("provided previous rand seed is nil")

// ErrLowerNonceInBlock signals that a block with lower nonce than permitted has been provided
var ErrLowerNonceInBlock = errors.New("lower nonce in block")

// ErrHigherNonceInBlock signals that a block with higher nonce than permitted has been provided
var ErrHigherNonceInBlock = errors.New("higher nonce in block")

// ErrNilSmartContractProcessor signals that smart contract call executor is nil
var ErrNilSmartContractProcessor = errors.New("smart contract processor is nil")

// ErrNilArgumentParser signals that the argument parser is nil
var ErrNilArgumentParser = errors.New("argument parser is nil")

// ErrNilSCDestAccount signals that destination account is nil
var ErrNilSCDestAccount = errors.New("nil destination SC account")

// ErrNilVMOutput signals that vmoutput is nil
var ErrNilVMOutput = errors.New("nil vm output")

// ErrInvalidVMOutput signals that vmoutput is invalid
var ErrInvalidVMOutput = errors.New("invalid vm output")

// ErrNilTemporaryAccountsHandler signals that temporary accounts handler is nil
var ErrNilTemporaryAccountsHandler = errors.New("temporary accounts handler is nil")

// ErrNilScAddress signals that a nil smart contract address has been provided
var ErrNilScAddress = errors.New("nil SC address")

// ErrEmptyFunctionName signals that an empty function name has been provided
var ErrEmptyFunctionName = errors.New("empty function name")

// ErrNilPreProcessor signals that preprocessors is nil
var ErrNilPreProcessor = errors.New("preprocessor is nil")

// ErrNilAppStatusHandler defines the error for setting a nil AppStatusHandler
var ErrNilAppStatusHandler = errors.New("nil AppStatusHandler")

// ErrNilUnsignedTxHandler signals that the unsigned tx handler is nil
var ErrNilUnsignedTxHandler = errors.New("nil unsigned tx handler")

// ErrNilPeerAccountsAdapter signals that a nil peer accounts database was provided
var ErrNilPeerAccountsAdapter = errors.New("nil peer accounts database")

// ErrNilEpochStartTrigger signals that a nil start of epoch trigger was provided
var ErrNilEpochStartTrigger = errors.New("nil start of epoch trigger")

// ErrNilEpochHandler signals that a nil epoch handler was provided
var ErrNilEpochHandler = errors.New("nil epoch handler")

// ErrNilEpochStartNotifier signals that the provided epochStartNotifier is nil
var ErrNilEpochStartNotifier = errors.New("nil epochStartNotifier")

// ErrNilEpochNotifier signals that the provided EpochNotifier is nil
var ErrNilEpochNotifier = errors.New("nil EpochNotifier")

// ErrOverallBalanceChangeFromSC signals that all sumed balance changes are not zero
var ErrOverallBalanceChangeFromSC = errors.New("SC output balance updates are wrong")

// ErrNilPendingMiniBlocksHandler signals that a nil pending miniblocks handler has been provided
var ErrNilPendingMiniBlocksHandler = errors.New("nil pending miniblocks handler")

// ErrSystemBusy signals that the system is busy
var ErrSystemBusy = errors.New("system busy")

// ErrInvalidMaxGasLimitPerTx signals that an invalid max gas limit per tx has been read from config file
var ErrInvalidMaxGasLimitPerTx = errors.New("invalid max gas limit per tx")

// ErrInvalidNonceRequest signals that invalid nonce was requested
var ErrInvalidNonceRequest = errors.New("invalid nonce request")

// ErrInvalidBlockRequestOldEpoch signals that invalid block was requested from old epoch
var ErrInvalidBlockRequestOldEpoch = errors.New("invalid block request from old epoch")

// ErrNilBlockChainHook signals that nil blockchain hook has been provided
var ErrNilBlockChainHook = errors.New("nil blockchain hook")

// ErrNilNodesSetup signals that nil nodes setup has been provided
var ErrNilNodesSetup = errors.New("nil nodes setup")

// ErrNilPeerShardMapper signals that a nil peer shard mapper has been provided
var ErrNilPeerShardMapper = errors.New("nil peer shard mapper")

// ErrNilRater signals that nil rater has been provided
var ErrNilRater = errors.New("nil rater")

// ErrNotEnoughGas signals that not enough gas has been provided
var ErrNotEnoughGas = errors.New("not enough gas was sent in the transaction")

// ErrNilHeaderSigVerifier signals that a nil header sig verifier has been provided
var ErrNilHeaderSigVerifier = errors.New("nil header sig verifier")

// ErrNilHeaderIntegrityVerifier signals that a nil header integrity verifier has been provided
var ErrNilHeaderIntegrityVerifier = errors.New("nil header integrity verifier")

// ErrNotEpochStartBlock signals that block is not of type epoch start
var ErrNotEpochStartBlock = errors.New("not epoch start block")

// ErrInvalidArguments signals that invalid arguments were given to process built-in function
var ErrInvalidArguments = errors.New("invalid arguments to process built-in function")

// ErrNilBuiltInFunction signals that built-in function is nil
var ErrNilBuiltInFunction = errors.New("built in function is nil")

// ErrNilRewardsHandler signals that rewards handler is nil
var ErrNilRewardsHandler = errors.New("rewards handler is nil")

// ErrNilValidatorStatistics signals that a nil validator statistics has been provided
var ErrNilValidatorStatistics = errors.New("nil validator statistics")

// ErrMaxRatingZero signals that maxrating with a value of zero has been provided
var ErrMaxRatingZero = errors.New("max rating is zero")

// ErrLogNotFound is the error returned when a transaction has no logs
var ErrLogNotFound = errors.New("no logs for queried transaction")

// ErrNilTxLogsProcessor is the error returned when a transaction has no logs
var ErrNilTxLogsProcessor = errors.New("nil transaction logs processor")

// ErrNilVmInput signals that provided vm input is nil
var ErrNilVmInput = errors.New("nil vm input")

// ErrNilDnsAddresses signals that nil dns addresses map was provided
var ErrNilDnsAddresses = errors.New("nil dns addresses map")

// ErrUserNameDoesNotMatch signals that username does not match
var ErrUserNameDoesNotMatch = errors.New("user name does not match")

// ErrInvalidVMType signals that invalid vm type was provided
var ErrInvalidVMType = errors.New("invalid VM type")

// ErrSmartContractDeploymentIsDisabled signals that smart contract deployment was disabled
var ErrSmartContractDeploymentIsDisabled = errors.New("smart Contract deployment is disabled")

// ErrUpgradeNotAllowed signals that upgrade is not allowed
var ErrUpgradeNotAllowed = errors.New("upgrade is allowed only for owner")

// ErrEmptyConsensusGroup is raised when an operation is attempted with an empty consensus group
var ErrEmptyConsensusGroup = errors.New("consensusGroup is empty")

// ErrNilOrEmptyList signals that a nil or empty list was provided
var ErrNilOrEmptyList = errors.New("nil or empty provided list")

// ErrNilScQueryElement signals that a nil sc query service element was provided
var ErrNilScQueryElement = errors.New("nil SC query service element")

// ErrNilLocker signals that a nil locker was provided
var ErrNilLocker = errors.New("nil locker")

// ErrNilAllowExternalQueriesChan signals that a nil channel for signaling the allowance of external queries provided is nil
var ErrNilAllowExternalQueriesChan = errors.New("nil channel for signaling the allowance of external queries")

// ErrQueriesNotAllowedYet signals that the node is not ready yet to process VM Queries
var ErrQueriesNotAllowedYet = errors.New("node is not ready yet to process VM Queries")

// ErrNilKDATransferParser signals that a nil KDA transfer parser has been provided
var ErrNilKDATransferParser = errors.New("nil kda transfer parser")

// ErrNilBootstrapper signals that a nil bootstraper has been provided
var ErrNilBootstrapper = errors.New("nil bootstrapper")

// ErrNodeIsNotSynced signals that the VM query cannot be executed because the node is not synced and the request required this
var ErrNodeIsNotSynced = errors.New("node is not synced")

// ErrStateChangedWhileExecutingVmQuery signals that the state has been changed while executing a vm query and the request required not to
var ErrStateChangedWhileExecutingVmQuery = errors.New("state changed while executing vm query")

// ErrNilSyncTimer signals that the sync timer is nil
var ErrNilSyncTimer = errors.New("sync timer is nil")

// ErrNoTxToProcess signals that no transaction were sent for processing
var ErrNoTxToProcess = errors.New("no transaction to process")

// ErrNilPeerSignatureHandler signals that a nil peer signature handler was provided
var ErrNilPeerSignatureHandler = errors.New("nil peer signature handler")

// ErrNilEnableEpochsHandler signals that a nil enable epochs handler has been provided
var ErrNilEnableEpochsHandler = errors.New("nil enable epochs handler")

// ErrPropertyTooLong signals that a heartbeat property was too long
var ErrPropertyTooLong = errors.New("property too long")

// ErrPropertyTooShort signals that a heartbeat property was too short
var ErrPropertyTooShort = errors.New("property too short")

// ErrMaxCallsReached signals that the allowed max number of calls was reached
var ErrMaxCallsReached = errors.New("max calls reached")

// ErrInvalidNumberOfBlockTxs signals a block was proposed with invalid number of txs
var ErrInvalidNumberOfBlockTxs = errors.New("invalid number of block txs")

// ErrSmartContractDeploymentFailed signals that smart contract deployment has failed
var ErrSmartContractDeploymentFailed = errors.New("smart Contract deployment failed")

// ErrSmartContractInvokeFailed signals that smart contract invoke has failed
var ErrSmartContractInvokeFailed = errors.New("smart Contract invoke failed")

// ErrSmartContractFailMaxContracts signals that smart contract invoke failed due to exceeding max contracts
var ErrSmartContractFailMaxContracts = errors.New("smart Contract invoke failed due to exceeding max contracts")
