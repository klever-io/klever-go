package process

import (
	"math/big"
	"reflect"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process/block/bootstrapStorage"
	"github.com/klever-io/klever-go/core/process/txsimulator"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"
)

// PeerValidatorMapper can determine the peer info from a peer id
type PeerValidatorMapper interface {
	GetPeerInfo(pid core.PeerID) core.P2PPeerInfo
	IsInterfaceNil() bool
}

// AntifloodDebugger defines an interface for debugging the antiflood behavior
type AntifloodDebugger interface {
	AddData(pid core.PeerID, topic string, numRejected uint32, sizeRejected uint64, sequence []byte, isBlacklisted bool)
	Close() error
	IsInterfaceNil() bool
}

// PeerBlackListCacher can determine if a certain peer id is or not blacklisted
type PeerBlackListCacher interface {
	Upsert(pid core.PeerID, span time.Duration) error
	Has(pid core.PeerID) bool
	Sweep()
	IsInterfaceNil() bool
}

// TimeCacher defines the cache that can keep a record for a bounded time
type TimeCacher interface {
	Add(key string) error
	Upsert(key string, span time.Duration) error
	Has(key string) bool
	Sweep()
	ResetAll()
	Len() int
	IsInterfaceNil() bool
}

// ForkDetector is an interface that defines the behaviour of a struct that is able
// to detect forks
type ForkDetector interface {
	AddHeader(header data.HeaderHandler, headerHash []byte, state BlockHeaderState, selfNotarizedHeaders []data.HeaderHandler, selfNotarizedHeadersHashes [][]byte) error
	RemoveHeader(nonce uint64, hash []byte)
	CheckFork() *ForkInfo
	GetHighestFinalBlockNonce() uint64
	GetHighestFinalBlockHash() []byte
	ProbableHighestNonce() uint64
	ResetFork()
	SoftResetFork(nonce uint64)
	SetRollBackNonce(nonce uint64)
	RestoreToGenesis()
	GetNotarizedHeaderHash(nonce uint64) []byte
	ResetProbableHighestNonce()
	SetFinalToLastCheckpoint()
	IsInterfaceNil() bool
}

// InterceptedData represents the interceptor's view of the received data
type InterceptedData interface {
	CheckValidity() error
	IsInterfaceNil() bool
	Hash() []byte
	Type() string
	Identifiers() [][]byte
	String() string
}

// WhiteListHandler is the interface needed to add whitelisted data
type WhiteListHandler interface {
	Remove(keys [][]byte)
	Add(keys [][]byte)
	IsWhiteListed(interceptedData InterceptedData) bool
	IsInterfaceNil() bool
}

// BlockSizeThrottler defines the functionality of adapting the node to the network speed/latency when it should send a
// block to its peers which should be received in a limited time frame
type BlockSizeThrottler interface {
	GetCurrentMaxSize() uint32
	Add(slot uint64, size uint32)
	Succeed(slot uint64)
	ComputeCurrentMaxSize()
	IsInterfaceNil() bool
}

// TopicFloodPreventer defines the behavior of a component that is able to signal that too many events occurred
// on a provided identifier between Reset calls, on a given topic
type TopicFloodPreventer interface {
	IncreaseLoad(pid core.PeerID, topic string, numMessages uint32) error
	ResetForTopic(topic string)
	ResetForNotRegisteredTopics()
	SetMaxMessagesForTopic(topic string, maxNum uint32)
	IsInterfaceNil() bool
}

// P2PAntifloodHandler defines the behavior of a component able to signal that the system is too busy (or flooded) processing
// p2p messages
type P2PAntifloodHandler interface {
	CanProcessMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error
	CanProcessMessagesOnTopic(pid core.PeerID, topic string, numMessages uint32, totalSize uint64, sequence []byte) error
	ApplyConsensusSize(size int)
	SetDebugger(debugger AntifloodDebugger) error
	BlacklistPeer(peer core.PeerID, reason string, duration time.Duration)
	IsOriginatorElectedForTopic(pid core.PeerID, topic string) error
	IsInterfaceNil() bool
}

// PeerShardMapper can return the public key of a provided peer ID
type PeerShardMapper interface {
	GetPeerInfo(pid core.PeerID) core.P2PPeerInfo
	IsInterfaceNil() bool
}

// FloodPreventer defines the behavior of a component that is able to signal that too many events occurred
// on a provided identifier between Reset calls
type FloodPreventer interface {
	IncreaseLoad(pid core.PeerID, size uint64) error
	ApplyConsensusSize(size int)
	Reset()
	IsInterfaceNil() bool
}

// EpochNotifier can notify upon an epoch change and provide the current epoch
type EpochNotifier interface {
	RegisterNotifyHandler(handler core.EpochSubscriberHandler)
	CurrentEpoch() uint32
	CheckEpoch(epoch uint32)
	IsInterfaceNil() bool
}

// InterceptedDebugger defines an interface for debugging the intercepted data
type InterceptedDebugger interface {
	LogReceivedHashes(topic string, hashes [][]byte)
	LogProcessedHashes(topic string, hashes [][]byte, err error)
	IsInterfaceNil() bool
}

// Interceptor defines what a data interceptor should do
// It should also adhere to the p2p.MessageProcessor interface so it can wire to a p2p.Messenger
type Interceptor interface {
	ProcessReceivedMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error
	SetInterceptedDebugHandler(handler InterceptedDebugger) error
	RegisterHandler(handler func(topic string, hash []byte, data interface{}))
	IsInterfaceNil() bool
}

// InterceptorsContainer defines an interceptors holder data type with basic functionality
type InterceptorsContainer interface {
	Get(key string) (Interceptor, error)
	Add(key string, val Interceptor) error
	AddMultiple(keys []string, interceptors []Interceptor) error
	Replace(key string, val Interceptor) error
	Remove(key string)
	Len() int
	Iterate(handler func(key string, interceptor Interceptor) bool)
	IsInterfaceNil() bool
}

// InterceptorsContainerFactory defines the functionality to create an interceptors container
type InterceptorsContainerFactory interface {
	Create() (InterceptorsContainer, error)
	IsInterfaceNil() bool
}

// TopicHandler defines the functionality needed by structs to manage topics and message processors
type TopicHandler interface {
	HasTopic(name string) bool
	CreateTopic(name string, createChannelForTopic bool) error
	RegisterMessageProcessor(topic string, handler p2p.MessageProcessor) error
	ID() core.PeerID
	IsInterfaceNil() bool
}

// InterceptorThrottler can monitor the number of the currently running interceptor go routines
type InterceptorThrottler interface {
	CanProcess() bool
	StartProcessing()
	EndProcessing()
	IsInterfaceNil() bool
}

// InterceptorProcessor further validates and saves received data
type InterceptorProcessor interface {
	Validate(data InterceptedData, fromConnectedPeer core.PeerID) error
	Save(data InterceptedData, fromConnectedPeer core.PeerID, topic string) error
	// Notify(data InterceptedData, fromConnectedPeer core.PeerID, topic string) error
	RegisterHandler(handler func(topic string, hash []byte, data interface{}))
	IsInterfaceNil() bool
}

// InterceptedDataFactory can create new instances of InterceptedData
type InterceptedDataFactory interface {
	Create(buff []byte) (InterceptedData, error)
	IsInterfaceNil() bool
}

// PreProcessor is an interface used to prepare and process transaction data
type PreProcessor interface {
	IsDataPrepared(requestedTxs int, haveTime func() time.Duration) error
	RemoveTxsFromPools(block *block.Block) error
	RestoreBlockDataIntoPools(block *block.Block) (int, error)
	CreateBlockStarted()
	ProcessBlockTransactions(block *block.Block, haveTime func() bool) (data.ProcessResults, error)
	CreateAndProcessBlockTransactions(
		blk *block.Block,
		haveTime func() bool,
	) (data.ProcessResults, error)
	SaveTxsToStorage(block *block.Block) error
	CreateMarshalizedData(txHashes [][]byte) ([][]byte, error)
	GetAllCurrentUsedTxs() map[string]data.TransactionHandler
	RequestBlockTransactions(block *block.Block) int
	IsInterfaceNil() bool
}

// PreProcessorsContainer defines an PreProcessors holder data type with basic functionality
type PreProcessorsContainer interface {
	Get(key block.Type) (PreProcessor, error)
	Add(key block.Type, val PreProcessor) error
	AddMultiple(keys []block.Type, preprocessors []PreProcessor) error
	Replace(key block.Type, val PreProcessor) error
	Remove(key block.Type)
	Len() int
	Keys() []block.Type
	IsInterfaceNil() bool
}

// RequestHandler defines the methods through which request to data can be made
type RequestHandler interface {
	SetEpoch(epoch uint32)
	RequestHeader(hash []byte)
	RequestHeaderByNonce(nonce uint64)
	RequestTransaction(txHashes [][]byte)
	RequestTransactionTo(txHashes [][]byte, peer core.PeerID)
	RequestTrieNodes(hashes [][]byte, topic string)
	RequestStartOfEpochBlock(epoch uint32)
	RequestInterval() time.Duration
	SetNumPeersToQuery(key string, intra int, cross int) error
	GetNumPeersToQuery(key string) (int, int, error)
	ResetRequests()
	IsInterfaceNil() bool
}

// BlockProcessor is the main interface for block execution engine
type BlockProcessor interface {
	SetProposalController(controller kapps.ActiveProposalController) error
	ProcessBlock(header data.HeaderHandler, haveTime func() time.Duration) error
	CommitBlock(header data.HeaderHandler) error
	RevertStateToSnapshot(header data.HeaderHandler)
	PruneStateOnRollback(currHeader data.HeaderHandler, prevHeader data.HeaderHandler)
	RevertStateToBlock(header data.HeaderHandler) error
	CreateNewHeader(slot uint64, nonce uint64) data.HeaderHandler
	RestoreBlockIntoPools(header data.HeaderHandler) error
	CreateBlock(initialHdr data.HeaderHandler, haveTime func() bool) (data.HeaderHandler, error)
	MarshalizedDataToBroadcast(header data.HeaderHandler) ([]byte, [][]byte, error)
	DecodeBlockHeader(dta []byte) data.HeaderHandler
	SetNumProcessedObj(numObj uint64)
	IsInterfaceNil() bool
}

// BlockAndHash holds the info related to a block and its hash
type BlockAndHash struct {
	Block *block.Block
	Hash  []byte
}

// Bootstrapper is an interface that defines the behaviour of a struct that is able
// to synchronize the node
type Bootstrapper interface {
	Close() error
	AddSyncStateListener(func(isSyncing bool))
	GetNodeState() core.NodeState
	LoadStorage()
	LoadSyncUntil(syncUntil uint32)
	LoadStopNodeChannel(chanStopNodeProcess chan endProcess.ArgEndProcess)
	StartSyncingBlocks()
	SetStatusHandler(handler core.AppStatusHandler) error
	IsInterfaceNil() bool
}

// ValidatorsProvider is the main interface for validators' provider
type ValidatorsProvider interface {
	GetLatestValidators() map[string]*state.ValidatorApiResponse
	GetLatestPeers() []state.PeerAccountHandler
	RefreshCache(epoch uint32)
	IsInterfaceNil() bool
}

// ValidatorStatisticsProcessor is the main interface for validators' consensus participation statistics
type ValidatorStatisticsProcessor interface {
	UpdatePeerState(header data.HeaderHandler, previousHeader data.HeaderHandler) ([]byte, error)
	RevertPeerState(header data.HeaderHandler) error
	IsInterfaceNil() bool
	RootHash() ([]byte, error)
	ResetValidatorStatisticsAtNewEpoch(vInfos []*state.ValidatorInfo) ([]*state.ValidatorInfo, error)
	GetValidatorInfoForRootHash(rootHash []byte) ([]*state.ValidatorInfo, error)
	GetValidatorAccountRootHash(rootHash []byte) ([]kapp.ValidatorAccountInfoHandler, error)
	GetPeersAccountsPubkeysFromRootHash(rootHash []byte) ([][]byte, error)
	ListPeerAccounts(rootHash []byte) ([]state.PeerAccountHandler, error)
	ProcessRatingsEndOfEpoch(validatorInfos []*state.ValidatorInfo, epoch uint32) error
	Commit() ([]byte, error)
	DisplayRatings(epoch uint32)
	SetLastFinalizedRootHash([]byte)
	LastFinalizedRootHash() []byte
	SaveNodesCoordinatorUpdates(epoch uint32) (bool, error)
	LeaderRewards(accumulatedFees int64) int64
}

// NodesCoordinator provides Validator methods needed for the peer processing
type NodesCoordinator interface {
	GetAllEligibleValidatorsKeys(epoch uint32, ownerKey bool) ([][]byte, error)
	GetAllElectedValidatorsKeys(epoch uint32, ownerKey bool) ([][]byte, error)
	IsInterfaceNil() bool
}

// HdrValidatorHandler defines the functionality that is needed for a HdrValidator to validate a header
type HdrValidatorHandler interface {
	Hash() []byte
	HeaderHandler() data.HeaderHandler
}

// TxValidator can determine if a provided transaction handler is valid or not from the process point of view
type TxValidator interface {
	CheckTxValidity(txHandler TxValidatorHandler) error
	CheckDup(txHash []byte) error
	CheckTxWhiteList(data InterceptedData) error
	IsInterfaceNil() bool
}

// TxValidatorHandler defines the functionality that is needed for a TxValidator to validate a transaction
type TxValidatorHandler interface {
	SenderAddress() []byte
	Nonce() uint64
	Fee() int64
	KDAFee() data.KDAFeeHandler
	PermissionID() int32
	ValidatePermissionOperation([]byte) error
	Signature() [][]byte
}

// TxVersionCheckerHandler defines the functionality that is needed for a TxVersionChecker to validate transaction version
type TxVersionCheckerHandler interface {
	CheckTxVersion(tx *transaction.Transaction) error
	IsInterfaceNil() bool
}

// BootStorer is the interface needed by bootstrapper to read/write data in storage
type BootStorer interface {
	SaveLastSlot(slot int64) error
	Put(slot int64, bootData *bootstrapStorage.BootstrapData) error
	Get(slot int64) (*bootstrapStorage.BootstrapData, error)
	GetHighestSlot() int64
	IsInterfaceNil() bool
}

// BootstrapperFromStorage is the interface needed by boot component to load data from storage
type BootstrapperFromStorage interface {
	LoadFromStorage() error
	GetHighestBlockNonce() uint64
	IsInterfaceNil() bool
}

// EpochBootstrapper defines the actions needed by bootstrapper
type EpochBootstrapper interface {
	SetCurrentEpochStartSlot(slot uint64)
	IsInterfaceNil() bool
}

// TransactionProcessor is the main interface for transaction execution engine
type TransactionProcessor interface {
	SetProposalController(controller kapps.ActiveProposalController) error
	PreProcessTransaction(tx *transaction.Transaction) (state.UserAccountHandler, []byte, error)
	ProcessTransaction(block *block.Block, txHash []byte, tx *transaction.Transaction) error
	ProcessBandwidthFee(txHash []byte, tx *transaction.Transaction, ownerAcc state.UserAccountHandler) (int64, error)
	ProcessKAppFee(txHash []byte, tx *transaction.Transaction, ownerAcc state.UserAccountHandler) (int64, error)
	GetAccounts(adrSrc, adrDst []byte) (acntSrc, acntDst state.UserAccountHandler, err error)
	IsInterfaceNil() bool
}

// TriggerHandler defines the functionalities for an start of epoch trigger
type TriggerHandler interface {
	Close() error
	ForceEpochStart(slot uint64)
	IsEpochStart() bool
	Epoch() uint32
	Update(slot uint64, nonce uint64)
	EpochStartSlot() uint64
	PrevEpochStartSlot() uint64
	EpochStartHdrHash() []byte
	GetSavedStateKey() []byte
	LoadState(key []byte) error
	SetProcessed(header data.HeaderHandler)
	SetFinalityAttestingSlot(slot uint64)
	EpochFinalityAttestingSlot() uint64
	RevertStateToBlock(header data.HeaderHandler) error
	SetCurrentEpochStartSlot(slot uint64)
	RequestEpochStartIfNeeded(interceptedHeader data.HeaderHandler)
	SetAppStatusHandler(handler core.AppStatusHandler) error
	IsInterfaceNil() bool
}

// EpochStartTriggerHandler defines that actions which are needed by processor for start of epoch
type EpochStartTriggerHandler interface {
	Update(slot uint64, nonce uint64)
	IsEpochStart() bool
	Epoch() uint32
	EpochStartSlot() uint64
	PrevEpochStartSlot() uint64
	SetProcessed(header data.HeaderHandler)
	RevertStateToBlock(header data.HeaderHandler) error
	EpochStartHdrHash() []byte
	GetSavedStateKey() []byte
	LoadState(key []byte) error
	IsInterfaceNil() bool
	SetCurrentEpochStartSlot(slot uint64)
	SetFinalityAttestingSlot(slot uint64)
	EpochFinalityAttestingSlot() uint64
	RequestEpochStartIfNeeded(interceptedHeader data.HeaderHandler)
}

// EpochStartDataCreator defines the functionality for node to create epoch start data
type EpochStartDataCreator interface {
	VerifyEpochStartDataForMetablock(metaBlock *block.Block) error
	IsInterfaceNil() bool
}

// DataMarshalizer defines the behavior of a structure that is able to marshalize containing data
type DataMarshalizer interface {
	CreateMarshalizedData(txHashes [][]byte) ([][]byte, error)
}

// TransactionCoordinator is an interface to coordinate transaction processing using multiple processors
type TransactionCoordinator interface {
	SaveTxsToStorage(blk *block.Block) error
	CreateBlockStarted()
	CreateMarshalizedData(blk *block.Block) ([][]byte, error)
	CreateAndProcessBlockTransactions(blk *block.Block, haveTime func() bool) (data.ProcessResults, error)

	RequestBlockTransactions(blk *block.Block)
	IsDataPreparedForProcessing(haveTime func() time.Duration) error
	ProcessBlockTransactions(blk *block.Block, haveTime func() time.Duration) (data.ProcessResults, error)
	GetAllCurrentUsedTxs() map[string]data.TransactionHandler
	GetAllCurrentLogs() []*data.LogData

	RestoreBlockDataFromStorage(body *block.Block) (int, error)
	RemoveBlockDataFromPool(body *block.Block) error
	RemoveTxsFromPool(blk *block.Block) error

	IsInterfaceNil() bool
}

// TransactionFeeHandler processes the transaction fee
type TransactionFeeHandler interface {
	CreateBlockStarted()
	GetAccumulatedTxFees() int64
	GetAccumulatedKAppFees() int64
	ProcessTransactionFee(txCost int64, kAppCost int64, txHash []byte)
	RevertFees(txHashes [][]byte)
	RevertTransactionFee(txHash []byte, txCost int64, kAppCost int64)
	IsInterfaceNil() bool
}

// TransactionWithFeeHandler represents a transaction structure that has economics variables defined
type TransactionWithFeeHandler interface {
	GetTransaction() *transaction.Transaction
	GetContracts() []*transaction.TXContract
	GetDataSize() int64
	GetKAppFee() int64
	GetBandwidthFee() int64
}

// EconomicsDataHandler is able to perform economics calculations and return economics data
type EconomicsDataHandler interface {
	SetProposalController(controller kapps.ActiveProposalController) error
	SetTXSimulatorProcessor(txSimulatorProcessor txsimulator.TransactionSimulatorProcessor) error
	ComputeTransactionCost(tx TransactionWithFeeHandler, simulateSC bool) (*transaction.CostResponse, error)
	CheckValidityTxValues(tx TransactionWithFeeHandler) (*transaction.CostResponse, error)
	ComputeGas(tx *transaction.Transaction, computedCost *transaction.CostResponse) (uint64, uint64, error)
	LeaderPercentage() float64
	MaxGasLimitPerBlock() uint64
	MaxGasLimitPerTX() uint64
	IsInterfaceNil() bool
}

// SelectionChance defines the actions which should be handled by a slot implementation
type SelectionChance interface {
	GetMaxThreshold() uint32
	GetChancePercent() uint32
}

// RatingsInfoHandler defines the information needed for the rating computation
type RatingsInfoHandler interface {
	StartRating() uint32
	MaxRating() uint32
	MinRating() uint32
	SignedBlocksThreshold() float32
	ChainRatingsStepHandler() RatingsStepHandler
	SelectionChances() []SelectionChance
	IsInterfaceNil() bool
}

// RatingsStepHandler defines the information needed for the rating computation on shards or meta
type RatingsStepHandler interface {
	ProposerIncreaseRatingStep() int32
	ProposerDecreaseRatingStep() int32
	ValidatorIncreaseRatingStep() int32
	ValidatorDecreaseRatingStep() int32
	ConsecutiveMissedBlocksPenalty() float32
}

// RatingChanceHandler provides the methods needed for the computation of chances from the Rating
type RatingChanceHandler interface {
	//GetMaxThreshold returns the threshold until this ChancePercentage holds
	GetMaxThreshold() uint32
	//GetChancePercentage returns the percentage for the RatingChanceHandler
	GetChancePercentage() uint32
	//IsInterfaceNil verifies if the interface is nil
	IsInterfaceNil() bool
}

// Indexer is an interface for saving node specific data to other storage.
// This could be an elastic search index, a MySql database or any other external services.
type Indexer interface {
	SaveBlock(args *indexer.ArgsSaveBlockData)
	RevertIndexedBlock(header data.HeaderHandler)
	SaveEpochInfo(epoch uint32, validators []kapp.ValidatorAccountInfoHandler)
	UpdateProposalsAndParameters(proposalIDs []string)
	SaveAccounts(blockTimestamp int64, acc []state.UserAccountHandler)
	SavePeersAccounts(validators []kapp.ValidatorAccountInfoHandler)
	SaveAssets(asset []*kapps.KDAData)
	Close() error
	IsInterfaceNil() bool
	IsNilIndexer() bool
}

// EventsProcessor is an interface for dispatching block events to all subscribers (websocket, indexer, etc.)
type EventsProcessor interface {
	Enabled() bool
	SaveBlock(args *indexer.ArgsSaveBlockData)
	RevertIndexedBlock(header data.HeaderHandler)
	SaveValidatorsRating(validators []kapp.ValidatorAccountInfoHandler)
	SaveEpochInfo(epoch uint32, validators []kapp.ValidatorAccountInfoHandler)
	SaveAccounts(blockTimestamp int64, acc []state.UserAccountHandler)
	UpdateProposalsAndParameters(proposalIDs []string)
	IsInterfaceNil() bool
}

// FallbackHeaderValidator defines the behaviour of a component able to signal when a fallback header validation could be applied
type FallbackHeaderValidator interface {
	ShouldApplyFallbackValidation(headerHandler data.HeaderHandler) bool
	IsInterfaceNil() bool
}

// InterceptedHeaderSigVerifier is the interface needed at interceptors level to check that a header's signature is correct
type InterceptedHeaderSigVerifier interface {
	VerifyRandSeed(header data.HeaderHandler) error
	VerifyRandSeedAndLeaderSignature(header data.HeaderHandler) error
	VerifySignature(header data.HeaderHandler) error
	VerifyLeaderSignature(header data.HeaderHandler) error
	IsInterfaceNil() bool
}

// HeaderIntegrityVerifier is the interface needed to check that a header's integrity
// is correct
type HeaderIntegrityVerifier interface {
	Verify(header data.HeaderHandler) error
	GetVersion(epoch uint32) string
	IsInterfaceNil() bool
}

// NetworkConnectionWatcher defines a watchdog functionality used to specify if the current node
// is still connected to the rest of the network
type NetworkConnectionWatcher interface {
	IsConnectedToTheNetwork() bool
	IsInterfaceNil() bool
}

// SlotManager defines the actions which should be handled by a slotManager implementation
type SlotManager interface {
	Index() int64
	IsInterfaceNil() bool
}

// ProposalController will return information about proposals
type ProposalController interface {
	Validate(parameter kapps.EnumParameter, value []byte) (reflect.Value, error)
	GetParameter(parameter kapps.EnumParameter) (reflect.Value, error)
	GetParameters() (kapps.ProposalParameters, error)
	IsInterfaceNil() bool
}

// VirtualMachinesContainerFactory defines the functionality to create a virtual machine container
type VirtualMachinesContainerFactory interface {
	Create() (VirtualMachinesContainer, error)
	Close() error
	BlockChainHookImpl() BlockChainHookHandler
	IsInterfaceNil() bool
}

// SmartContractProcessor is the main interface for the smart contract caller engine
type SmartContractProcessor interface {
	ExecuteSmartContractTransaction(ctx kapp.KappContext, tc data.SmartContractHandler, acntSrc, acntDst state.UserAccountHandler) (vmcommon.ReturnCode, error)
	DeploySmartContract(ctx kapp.KappContext, tc data.SmartContractHandler) (vmcommon.ReturnCode, error)
	ProcessIfError(ctx kapp.KappContext, tc data.SmartContractHandler, returnCode string, returnMessage []byte) error
	IsPayable(sndAddress []byte, recvAddress []byte) (bool, error)
	LastBlock() data.HeaderHandler
	SetVMExecutionMode(mode vmcommon.ExecutionMode)
	GetVMExecutionMode() vmcommon.ExecutionMode
	IsInterfaceNil() bool
}

// BlockChainHookHandler defines the actions which should be performed by implementation
type BlockChainHookHandler interface {
	GetCode(account state.UserAccountHandler) []byte
	GetKAppController() kapp.KAppController
	GetUserAccount(address []byte) (state.UserAccountHandler, error)
	GetStorageData(accountAddress []byte, index []byte) ([]byte, uint32, error)
	GetBlockHash(nonce uint64) ([]byte, error)
	LastBlock() data.HeaderHandler
	LastNonce() uint64
	LastSlot() uint64
	LastTimeStamp() int64
	LastRandomSeed() []byte
	LastEpoch() uint32
	GetStateRootHash() []byte
	CurrentNonce() uint64
	CurrentSlot() uint64
	CurrentTimeStamp() int64
	CurrentRandomSeed() []byte
	CurrentEpoch() uint32
	NewAddress(creatorAddress []byte, creatorNonce uint64, vmType []byte, randomSeed []byte) ([]byte, error)
	ProcessBuiltInFunction(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error)
	IsSmartContract(address []byte) bool
	GetBuiltinFunctionNames() vmcommon.FunctionNames
	GetBuiltinFunctionsContainer() vmcommon.BuiltInFunctionContainer
	GetAllState(_ []byte) (map[string][]byte, error)
	GetKDAToken(address []byte, tokenID []byte, nonce uint64) (*kapps.KDAData, *kapps.UserKDA, error)
	GetSFTMeta(tokenID []byte, nonce uint64) (*kapps.MetaV2, error)
	SetCurrentHeader(hdr data.HeaderHandler)
	SaveCompiledCode(codeHash []byte, code []byte)
	GetCompiledCode(codeHash []byte) (bool, []byte)
	IsPayable(sndAddress []byte, recvAddress []byte) (bool, error)
	DeleteCompiledCode(codeHash []byte)
	ClearCompiledCodes()
	GetSnapshot() int
	RevertToSnapshot(snapshot int) error
	ExecuteSmartContractCallOnOtherVM(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error)
	TransferValueOnly(destination []byte, sender []byte, value *big.Int) error
	KDATransfer(sender []byte, tc *transaction.TransferContract) error
	IncreaseNonce(address []byte) error
	Close() error
	FilterCodeMetadataForUpgrade(input []byte) ([]byte, error)
	ResetCounters()
	GetCounterValues() map[string]uint64
	IsInterfaceNil() bool
}

// VirtualMachinesContainer defines a virtual machine holder data type with basic functionality
type VirtualMachinesContainer interface {
	Close() error
	Get(key []byte) (vmcommon.VMExecutionHandler, error)
	Add(key []byte, val vmcommon.VMExecutionHandler) error
	AddMultiple(keys [][]byte, vms []vmcommon.VMExecutionHandler) error
	Replace(key []byte, val vmcommon.VMExecutionHandler) error
	Remove(key []byte)
	Len() int
	Keys() [][]byte
	IsInterfaceNil() bool
}

// CallArgumentsParser defines the functionality to parse transaction data into call arguments
type CallArgumentsParser interface {
	ParseData(data string) (string, [][]byte, error)
	ParseArguments(data string) ([][]byte, error)
	IsInterfaceNil() bool
}

// DeployArgumentsParser defines the functionality to parse transaction data into call arguments
type DeployArgumentsParser interface {
	ParseData(data string) (*parsers.DeployArgs, error)
	IsInterfaceNil() bool
}

// StorageArgumentsParser defines the functionality to parse transaction data into call arguments
type StorageArgumentsParser interface {
	CreateDataFromStorageUpdate(storageUpdates []*vmcommon.StorageUpdate) string
	GetStorageUpdates(data string) ([]*vmcommon.StorageUpdate, error)
	IsInterfaceNil() bool
}

// ArgumentsParser defines the functionality to parse transaction data into arguments and code for smart contracts
type ArgumentsParser interface {
	ParseCallData(data string) (string, [][]byte, error)
	ParseArguments(data string) ([][]byte, error)
	ParseDeployData(data string) (*parsers.DeployArgs, error)

	CreateDataFromStorageUpdate(storageUpdates []*vmcommon.StorageUpdate) string
	GetStorageUpdates(data string) ([]*vmcommon.StorageUpdate, error)
	IsInterfaceNil() bool
}

// TransactionLogProcessor is the main interface for saving logs generated by smart contract calls
type TransactionLogProcessor interface {
	GetAllCurrentLogs() []*data.LogData
	GetLog(txHash []byte) (data.LogHandler, error)
	SaveLog(txHash []byte, acntSender []byte, tc data.SmartContractHandler, contractID int, vmLogs []*vmcommon.LogEntry) error
	Clean()
	IsInterfaceNil() bool
}

// SCQuery represents a prepared query for executing a function of the smart contract
type SCQuery struct {
	ScAddress      []byte
	FuncName       string
	CallerAddr     []byte
	CallValue      map[string]int64
	Arguments      [][]byte
	SameScState    bool
	ShouldBeSynced bool
}

// SCQueryService defines how data should be get from a SC account
type SCQueryService interface {
	ExecuteQuery(query *SCQuery) (*vmcommon.VMOutput, error)
	Close() error
	IsInterfaceNil() bool
}
