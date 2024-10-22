package core

import (
	"math"
	"time"
)

// ZeroAddress represents the zero Address
var ZeroAddress = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

// BlackHoleAddress represents the black hole Address
var BlackHoleAddress = []byte{123, 222, 247, 189, 239, 123, 222, 247, 189, 239, 123, 222, 247, 189, 239, 123, 222, 247, 189, 239, 123, 222, 247, 189, 239, 123, 222, 247, 189, 239, 123, 222}

// OneYearTimestamp represents the relative timestamp of an year
const OneYearTimestamp = int64(31556926)

// HundredPercent represents the precision of 100%
const HundredPercent = uint32(10000)

// PathEpochPlaceholder represents the placeholder for the epoch number in paths
const PathEpochPlaceholder = "[E]"

// PathIdentifierPlaceholder represents the placeholder for the identifier in paths
const PathIdentifierPlaceholder = "[I]"

// Max Assets Roles allowed
const MaxAssetRoles = 20

// UnVersionedAppString represents the default app version that indicate that the binary wasn't build by setting
// the appVersion flag
const UnVersionedAppString = "undefined"

const (
	// StorerOrder defines the order of storers to be notified of a start of epoch event
	StorerOrder = iota
	// NodesCoordinatorOrder defines the order in which NodesCoordinator is notified of a start of epoch event
	NodesCoordinatorOrder
	// ConsensusOrder defines the order in which Consensus is notified of a start of epoch event
	ConsensusOrder
	// NetworkShardingOrder defines the order in which the network sharding subsystem is notified of a start of epoch event
	NetworkShardingOrder
	// IndexerOrder defines the order in which Indexer is notified of a start of epoch event
	IndexerOrder
	// NetStatisticsOrder defines the order in which netStatistic component is notified of a start of epoch event
	NetStatisticsOrder
	// OldDatabaseCleanOrder defines the order in which oldDatabaseCleaner component is notified of a start of epoch event
	OldDatabaseCleanOrder
)

// NodeType represents the node's role in the network
type NodeType string

// NodeTypeObserver signals that a node is running as observer node
const NodeTypeObserver NodeType = "observer"

// NodeTypeValidator signals that a node is running as validator node
const NodeTypeValidator NodeType = "validator"

// PeerType represents the type of a peer
type PeerType string

// ElectedList represents the list of peers who participate in consensus
const ElectedList PeerType = "elected"

// EligibleList represents the list of peers who don't participate in consensus but will in the following epochs
const EligibleList PeerType = "eligible"

// WaitingList represents the list of peers who don't participate in consensus but has the min self-staked amount
const WaitingList PeerType = "waiting"

// LeavingList represents the list of peers who were taken out of eligible and waiting because of rating
const LeavingList PeerType = "leaving"

// InactiveList represents the list of peers who doesn't have the minimun threshold amount
const InactiveList PeerType = "inactive"

// JailedList represents the list of peers who have stake but are in jail
const JailedList PeerType = "jailed"

// ObserverList represents the list of peers who don't participate in consensus but will join the next epoch
const ObserverList PeerType = "observer"

// RevokedList represents the list of peers who have been assgined to an validator but revoked
const RevokedList PeerType = "revoked"

// MetachainName will be used to identify a shard ID as metachain string
const MetachainName = "meta"

// WrongP2PMessageBlacklistDuration represents the time to keep a peer id in the blacklist if it sends a message that
// do not follow this protocol
const WrongP2PMessageBlacklistDuration = time.Second * 7200

// DefaultLogProfileIdentifier represents the default log profile used when the logviewer/connector applications do not
// need to change the current logging profile
const DefaultLogProfileIdentifier = "[default log profile]"

// MaxRetriesToCreateDB represents the maximum number of times to try to create DB if it failed
const MaxRetriesToCreateDB = 10

// SleepTimeBetweenCreateDBRetries represents the number of seconds to sleep between DB creates
const SleepTimeBetweenCreateDBRetries = 5 * time.Second

// LastNonceKeyMetricsStorage holds the key used for storing the last nonce for stored metrics
const LastNonceKeyMetricsStorage = "lastNonce"

// InvalidMessageBlacklistDuration represents the time to keep a peer in the black list if it sends a message that
// does not follow the protocol: example not useing the same marshaler as the other peers
const InvalidMessageBlacklistDuration = time.Second * 3600

// ImportComplete signals that a node restart will be done because the import did complete
const ImportComplete = "importComplete"

// MaxUserNameLength represents the maximum number of bytes a UserName can have
const MaxUserNameLength = 32

// MaxTxNonceDeltaAllowed specifies the maximum difference between an account's nonce and a received transaction's nonce
// in order to mark the transaction as valid.
const MaxTxNonceDeltaAllowed = 100

// DefaultUnstakedEpoch represents the default epoch that is set for a validator that has not unstaked yet
const DefaultUnstakedEpoch = math.MaxUint32

// DefaultUndelegatedEpoch represents the default epoch that is set for a validator that has not undelegated yet
const DefaultUndelegatedEpoch = math.MaxUint32

// MaxBulkTransactionSize specifies the maximum size of one bulk with txs which can be send over the network
// TODO convert this const into a var and read it from config when this code moves to another binary
const MaxBulkTransactionSize = 1 << 18 //256KB bulks

// NodeState specifies what type of state a node could have
type NodeState int

const (
	// NsSynchronized defines ID of a state of synchronized
	NsSynchronized NodeState = iota
	// NsNotSynchronized defines ID of a state of not synchronized
	NsNotSynchronized
	// NsNotCalculated defines ID of a state which is not calculated
	NsNotCalculated
)

// CombinedPeerType - represents the combination of two peerTypes
const CombinedPeerType = "%s (%s)"

// CommitMaxTime represents max time accepted for a commit action, after which a warn message is displayed
const CommitMaxTime = 1 * time.Second

// TriggerRegistryKeyPrefix is the key prefix to save epoch start registry to storage
const TriggerRegistryKeyPrefix = "epochStartTrigger_"

// NodesCoordinatorRegistryKeyPrefix is the key prefix to save epoch start registry to storage
const NodesCoordinatorRegistryKeyPrefix = "indexHashed_"

// TriggerRegistryInitialKeyPrefix is the key prefix to save initial data to storage
const TriggerRegistryInitialKeyPrefix = "initial_value_epoch_"

// PutInStorerMaxTime represents max time accepted for a put action, after which a warn message is displayed
const PutInStorerMaxTime = time.Second

// ExtraDelayBetweenBroadcastMbsAndTxs represents the number of seconds to wait since miniblocks have been broadcast
// and the moment when theirs transactions would be broadcast too
const ExtraDelayBetweenBroadcastMbsAndTxs = 1 * time.Second

// ExtraDelayForBroadcastBlockInfo represents the number of seconds to wait since a block has been broadcast and the
// moment when its components, like mini blocks and transactions, would be broadcast too
const ExtraDelayForBroadcastBlockInfo = 1 * time.Second

// MaxLengthOfContracts defines max length of contracts allowed
const MaxLengthOfContracts = 20

// MinLengthForAssetName defines min length for asset name
const MinLengthForAssetName = 1

// MaxLengthForAssetName defines max length for asset name
const MaxLengthForAssetName = 32

// MinLengthForAssetTicker defines min length for asset ticker
const MinLengthForAssetTicker = 3

// MaxLengthForAssetTicker defines max length for asset ticker
const MaxLengthForAssetTicker = 10

// MaxNameSize defines max name size
const MaxNameSize = 100

// MaxLogoURISize defines max logo URI size
const MaxLogoURISize = 250

// MaxURIKeySize defines max URI key size
const MaxURIKeySize = 30

// MaxURIValueSize defines max URI value size
const MaxURIValueSize = 250

// MaxURIMapSize defines max number of URIs allowed
const MaxURIMapSize = 10

// MaxProposalsLength defines max proposals allowed
const MaxProposalsLength = 10

// MaxProposalParamLength defines max proposal parameter size
const MaxProposalParamLength = 50

// MaxDescriptionLength defines max description size
const MaxDescriptionLength = 1000

// MaxOperationsSize defines max operations size
const MaxOperationsSize = 30

// MaxTransferRoyalties defines max transfer royalties size
const MaxTransferRoyalties = 20

// MaxRoles definesmax roles size
const MaxRoles = 50

// MaxPacks defines max packs allowed
const MaxPacks = 10

// MaxPackItems defines max items per pack allowed
const MaxPackItems = 20

// MaxWhitelistSize defines max whitelist size
const MaxWhitelistSize = 10000

// MinNumberOfDecimals defines min asset precision
const MinNumberOfDecimals = 0

// MaxNumberOfDecimals defines max asset precision
const MaxNumberOfDecimals = 8

// MaxSoftwareVersionLengthInBytes represents the maximum length for the software version to be saved in block header
const MaxSoftwareVersionLengthInBytes = 10

// MaxSlotsWithoutCommittedStartInEpochBlock defines the maximum rounds to wait for start in epoch block to be committed,
// before a special action to be applied
const MaxSlotsWithoutCommittedStartInEpochBlock = 50

// NodesSetupJsonFileName specifies the name of the json file which contains the setup of the nodes
const NodesSetupJsonFileName = "nodesSetup.json"

// MaxAccountPermission define max number of permission in one account
const MaxAccountPermission = 20

// MaxPermissionSigners define max number of signers in one account permission role
const MaxPermissionSigners = 10

// MaxDepositKDAs define max number of kdas allowed in deposits
const MaxDepositKDAs = 20

// MaxRetriesAvoidingSameRollback define max number of times to avoid repeat the same rollback
const MaxRetriesAvoidingSameRollback = 2

// MaxKappIterator define the max number of operations a kapp can do
const MaxKappIterator = 1000

// NodeProcessingMode represents the processing mode in which the node was started
type NodeProcessingMode int

const (
	// Normal means that the node has started in the normal processing mode
	Normal NodeProcessingMode = iota

	// ImportDb means that the node has started in the import-db mode
	ImportDb
)

// BaseTxSize define the base size for transaction
const BaseTxSize = 250

// MetricAreVMQueriesReady will hold the string representation of the boolean that indicated if the node is ready
// to process VM queries
const MetricAreVMQueriesReady = "erd_are_vm_queries_ready"

// Maximum amount of APR updates that will be saved by the blockchain and the indexer
const MaxAPRPercentageUpdates = 100

// MaxCallValueSize defines the maximum number of currencies that can be used in a smart contract call value
const MaxCallValueSize = 50

// PubKeyLen defines the length of a public key
const PubKeyLen = 32

// MinGasLimit defines the value that a basic protocol transaction compared to a sc transaction should have for block gas limitation
const MinGasLimit = 50_000

// MaxGasBandwidthPerBatchPerSender defines the maximum gas bandwidth that should be selected for a sender per batch from the cache
const MaxGasBandwidthPerBatchPerSender = 50_000_000

// MaxTxSize defines the maximum size of a tx without contracts and message data
const MaxTxSize = 420
