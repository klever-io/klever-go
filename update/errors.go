package update

import "errors"

// ErrUnknownType signals that type is unknown
var ErrUnknownType = errors.New("unknown type")

// ErrNilStateSyncer signals that state syncer is nil
var ErrNilStateSyncer = errors.New("nil state syncer")

// ErrInvalidFolderName signals that folder name is nil
var ErrInvalidFolderName = errors.New("invalid folder name")

// ErrNotSynced signals that syncing has not been finished yet
var ErrNotSynced = errors.New("not synced")

// ErrNilTrieSyncers signals that trie syncers container is nil
var ErrNilTrieSyncers = errors.New("nil trie syncers")

// ErrNilHeaderSyncHandler signals that nil header sync handler was provided
var ErrNilHeaderSyncHandler = errors.New("nil header sync handler")

// ErrNilTransactionsSyncHandler signals that nil transactions sync handler was provided
var ErrNilTransactionsSyncHandler = errors.New("nil transaction sync handler")

// ErrWrongUnFinishedMetaHdrsMap signals that wrong unFinished meta headers map was provided
var ErrWrongUnFinishedMetaHdrsMap = errors.New("wrong unFinished meta headers map")

// ErrNilMultiSigner signals that nil multi signer was provided
var ErrNilMultiSigner = errors.New("nil multi signer")

// ErrNilStorageManager signals that nil storage manager has been provided
var ErrNilStorageManager = errors.New("nil trie storage manager")

// ErrNilAccountsDBSyncContainer signals that nil accounts sync container was provided
var ErrNilAccountsDBSyncContainer = errors.New("nil accounts db sync container")

// ErrTriggerNotEnabled signals that the trigger is not enabled
var ErrTriggerNotEnabled = errors.New("trigger is not enabled")

// ErrNilCloser signals that a nil closer instance was provided
var ErrNilCloser = errors.New("nil closer instance")

// ErrInvalidValue signals that the value provided is invalid
var ErrInvalidValue = errors.New("invalid value")

// ErrTriggerPubKeyMismatch signals that there is a mismatch between the public key received and the one read from the config
var ErrTriggerPubKeyMismatch = errors.New("trigger public key mismatch")

// ErrNilAntiFloodHandler signals that nil anti flood handler has been provided
var ErrNilAntiFloodHandler = errors.New("nil anti flood handler")

// ErrIncorrectHardforkMessage signals that the hardfork message is incorrectly formatted
var ErrIncorrectHardforkMessage = errors.New("incorrect hardfork message")

// ErrNilRwdTxProcessor signals that nil reward transaction processor has been provided
var ErrNilRwdTxProcessor = errors.New("nil reward transaction processor")

// ErrNilImportHandler signals that nil import handler has been provided
var ErrNilImportHandler = errors.New("nil import handler")

// ErrNilTxCoordinator signals that nil tx coordinator has been provided
var ErrNilTxCoordinator = errors.New("nil tx coordinator")

// ErrNilPendingTxProcessor signals that nil pending tx processor has been provided
var ErrNilPendingTxProcessor = errors.New("nil pending tx processor")

// ErrNilHardForkBlockProcessor signals that nil hard fork block processor has been provided
var ErrNilHardForkBlockProcessor = errors.New("nil hard fork block processor")

// ErrNilTrieStorageManagers signals that nil trie storage managers has been provided
var ErrNilTrieStorageManagers = errors.New("nil trie storage managers")

// ErrEmptyChainID signals that empty chain ID was provided
var ErrEmptyChainID = errors.New("empty chain ID")

// ErrNilArgumentParser signals that nil argument parser was provided
var ErrNilArgumentParser = errors.New("nil argument parser")

// ErrNilExportFactoryHandler signals that nil export factory handler has been provided
var ErrNilExportFactoryHandler = errors.New("nil export factory handler")

// ErrNilChanStopNodeProcess signals that nil channel to stop node was provided
var ErrNilChanStopNodeProcess = errors.New("nil channel to stop node")

// ErrNilEpochConfirmedNotifier signals that nil epoch confirmed notifier was provided
var ErrNilEpochConfirmedNotifier = errors.New("nil epoch confirmed notifier")

// ErrTriggerAlreadyInAction signals that the trigger is already in action, can not re-enter
var ErrTriggerAlreadyInAction = errors.New("trigger already in action")

// ErrInvalidTimeToWaitAfterHardfork signals that an invalid time to wait after hardfork was provided
var ErrInvalidTimeToWaitAfterHardfork = errors.New("invalid time to wait after hard fork")

// ErrInvalidEpoch signals that an invalid epoch has been provided
var ErrInvalidEpoch = errors.New("invalid epoch")

// ErrEmptyVersionString signals that the provided version string is empty
var ErrEmptyVersionString = errors.New("empty version string")

// ErrNilHardforkStorer signals that a nil hardfork storer has been provided
var ErrNilHardforkStorer = errors.New("nil hardfork storer")

// ErrExpectedOneStartOfEpochMetaBlock signals that exactly one start of epoch metaBlock should have been used
var ErrExpectedOneStartOfEpochMetaBlock = errors.New("expected one start of epoch metaBlock")

// ErrImportingData signals that an import error occurred
var ErrImportingData = errors.New("error importing data")

// ErrKeyTypeMismatch signals that key type was mismatch during import
var ErrKeyTypeMismatch = errors.New("key type mismatch while importing")

// ErrEmptyExportFolderPath signals that the provided export folder's length is empty
var ErrEmptyExportFolderPath = errors.New("empty export folder path")

// ErrNilGenesisNodesSetupHandler signals that a nil genesis nodes setup handler has been provided
var ErrNilGenesisNodesSetupHandler = errors.New("nil genesis nodes setup handler")

// ErrNilEpochNotifier signals that the provided EpochNotifier is nil
var ErrNilEpochNotifier = errors.New("nil EpochNotifier")

// ErrWrongImportedMiniBlocksMap signals that wrong imported miniBlocks map was provided
var ErrWrongImportedMiniBlocksMap = errors.New("wrong imported miniBlocks map was provided")

// ErrWrongImportedTransactionsMap signals that wrong imported transactions map was provided
var ErrWrongImportedTransactionsMap = errors.New("wrong imported transactions map was provided")

// ErrMiniBlockNotFoundInImportedMap signals that the given miniBlock was not found in imported map
var ErrMiniBlockNotFoundInImportedMap = errors.New("miniBlock was not found in imported map")

// ErrTransactionNotFoundInImportedMap signals that the given transaction was not found in imported map
var ErrTransactionNotFoundInImportedMap = errors.New("transaction was not found in imported map")

// ErrNilEpochStartMetaBlock signals that a nil epoch start metaBlock was provided
var ErrNilEpochStartMetaBlock = errors.New("nil epoch start metaBlock was provided")

// ErrPostProcessTransactionNotFound signals that the given transaction was not found in post process map
var ErrPostProcessTransactionNotFound = errors.New("transaction was not found in post process map")

// ErrNilHeaderHandler signals that a nil header handler has been provided
var ErrNilHeaderHandler = errors.New("nil header handler")

// ErrInvalidMaxHardCapForMissingNodes signals that the maximum hardcap value for missing nodes is invalid
var ErrInvalidMaxHardCapForMissingNodes = errors.New("invalid max hardcap for missing nodes")

// ErrInvalidNumConcurrentTrieSyncers signals that the number of concurrent trie syncers is invalid
var ErrInvalidNumConcurrentTrieSyncers = errors.New("invalid num concurrent trie syncers")

// ErrTokenizeFailed signals that data splitting into arguments and code failed
var ErrTokenizeFailed = errors.New("tokenize failed")

// ErrNilFunction signals that the function name from transaction data is nil
var ErrNilFunction = errors.New("function is nil")

// ErrNilArguments signals that arguments from transactions data is nil
var ErrNilArguments = errors.New("arguments are nil")
