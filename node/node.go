package node

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	syncGo "sync"
	"sync/atomic"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/facade"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/alarm"
	"github.com/klever-io/klever-go/core/consensus"
	broadcastFactory "github.com/klever-io/klever-go/core/consensus/broadcast"
	"github.com/klever-io/klever-go/core/consensus/chronology"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/consensus/slot/slotFactory"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/partitioning"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/dataValidators"
	procTx "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/core/versioning"
	"github.com/klever-io/klever-go/core/watchdog"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	disabledSig "github.com/klever-io/klever-go/crypto/signing/disabled/singlesig"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/kapps"

	// "github.com/klever-io/klever-go/data/epochStart"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/node/heartbeat/componentHandler"
	heartbeatData "github.com/klever-io/klever-go/node/heartbeat/data"
	heartbeatProcess "github.com/klever-io/klever-go/node/heartbeat/process"
	"github.com/klever-io/klever-go/ntp"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/debug"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

// SendTransactionsPipe is the pipe used for sending new transactions
const SendTransactionsPipe = "send transactions pipe"

// SoftRestartMessage is the custom message used when the node does a soft restart operation
var SoftRestartMessage = "Shuffled out - soft restart"

var log = logger.GetOrCreate("node")
var numSecondsBetweenPrints = 20

var _ facade.NodeHandler = (*Node)(nil)

// Option represents a functional configuration parameter that can operate
// over the None struct.
type Option func(*Node) error

// Node is a structure that passes the configuration parameters and initializes
// required services as requested
type Node struct {
	ctx                           context.Context
	internalMarshalizer           marshal.Marshalizer
	txSignMarshalizer             marshal.Marshalizer
	hasher                        hashing.Hasher
	initialNodesPubkeys           []string
	slotManager                   consensus.SlotManager
	consensusGroupSize            int
	messenger                     P2PMessenger
	syncTimer                     ntp.SyncTimer
	blockProcessor                process.BlockProcessor
	txProcessor                   process.TransactionProcessor
	kappController                kapp.KAppController
	kappControllerSimulator       kapp.KAppController
	feeHandler                    process.EconomicsDataHandler
	genesisTime                   time.Time
	epochStartTrigger             process.EpochStartTriggerHandler
	epochStartRegistrationHandler eventNotifier.RegistrationHandler
	accounts                      state.AccountsAdapter
	kapps                         state.AccountsAdapter
	addressPubkeyConverter        core.PubkeyConverter
	validatorPubkeyConverter      core.PubkeyConverter
	uint64ByteSliceConverter      typeConverters.Uint64ByteSliceConverter
	interceptorsContainer         process.InterceptorsContainer
	resolversFinder               retriever.ResolversFinder
	peerDenialEvaluator           p2p.PeerDenialEvaluator
	appStatusHandler              core.AppStatusHandler
	validatorStatistics           process.ValidatorStatisticsProcessor
	validatorsProvider            process.ValidatorsProvider
	whiteListRequest              process.WhiteListHandler
	whiteListerVerifiedTxs        process.WhiteListHandler

	pubKey            crypto.PublicKey
	privKey           crypto.PrivateKey
	keyGenForBlock    crypto.KeyGenerator
	keyGenForAccounts crypto.KeyGenerator
	singleSigner      crypto.SingleSigner
	txSingleSigner    crypto.SingleSigner
	multiSigner       crypto.MultiSigner
	peerSigHandler    crypto.PeerSignatureHandler
	forkDetector      process.ForkDetector

	blkc             data.ChainHandler
	dataPool         retriever.PoolsHolder
	store            retriever.StorageService
	economics        blockCache[models.EconomicsResponse]
	accountTotals    blockCache[models.AccountTotalsResponse]
	nodesCoordinator sharding.NodesCoordinator

	networkShardingCollector NetworkShardingCollector

	consensusTopic string
	consensusType  string

	currentSendingGoRoutines int32
	bootstrapSlotIndex       uint64

	indexer                 process.Indexer
	bootstrapper            process.Bootstrapper
	blocksBlackListHandler  process.TimeCacher
	bootStorer              process.BootStorer
	requestedItemsHandler   retriever.RequestedItemsHandler
	headerSigVerifier       consensus.HeaderSigVerifier
	headerIntegrityVerifier slot.HeaderIntegrityVerifier

	chainID               []byte
	minTransactionVersion uint32

	txSentCounter         uint32
	inputAntifloodHandler P2PAntifloodHandler
	txAccumulator         Accumulator

	//blockTracker             process.BlockTracker

	requestHandler process.RequestHandler

	encodedAddressLength   int
	validatorSignatureSize int
	publicKeySize          int

	chanStopNodeProcess chan endProcess.ArgEndProcess

	mutQueryHandlers syncGo.RWMutex
	queryHandlers    map[string]debug.QueryHandler

	heartbeatHandler        HeartbeatHandler
	peerHonestyHandler      consensus.PeerHonestyHandler
	fallbackHeaderValidator consensus.FallbackHeaderValidator

	watchdog core.WatchdogTimer
	//historyRepository dblookupext.HistoryRepository

	txSignHasher hashing.Hasher
	//txVersionChecker          process.TxVersionCheckerHandler
	isInImportMode        bool
	startWithInSync       bool
	nodeRedundancyHandler consensus.NodeRedundancyHandler

	proposalController kapps.ActiveProposalController
	forkController     core.ForkController
	enableEpochsConfig config.EnableEpochsConfig

	consensusMonitorList         []string
	networkDegradedThreshold     uint32
	networkDegradedCooldownSlots uint32

	// For Backtest Purpose
	syncUntil uint32
}

// ApplyOptions can set up different configurable options of a Node instance
func (n *Node) ApplyOptions(opts ...Option) error {
	for _, opt := range opts {
		err := opt(n)
		if err != nil {
			return errors.New("error applying option: " + err.Error())
		}
	}
	return nil
}

// NewNode creates a new Node instance
func NewNode(opts ...Option) (*Node, error) {
	node := &Node{
		ctx:                          context.Background(),
		currentSendingGoRoutines:     0,
		appStatusHandler:             statusHandler.NewNilStatusHandler(),
		queryHandlers:                make(map[string]debug.QueryHandler),
		consensusMonitorList:         make([]string, 0),
		networkDegradedThreshold:     0,
		networkDegradedCooldownSlots: 15,
	}
	for _, opt := range opts {
		err := opt(node)
		if err != nil {
			return nil, errors.New("error applying option: " + err.Error())
		}
	}

	return node, nil
}

func (n *Node) GetNodeState() core.NodeState {
	return n.bootstrapper.GetNodeState()
}

// GetAppStatusHandler will return the current status handler
func (n *Node) GetAppStatusHandler() core.AppStatusHandler {
	return n.appStatusHandler
}

// GetPeerHonestyHandler returns the peer honesty handler attached to this node.
// Returned for shutdown wiring so the caller can Close it on process exit.
func (n *Node) GetPeerHonestyHandler() consensus.PeerHonestyHandler {
	return n.peerHonestyHandler
}

// GetHeartbeatHandler returns the heartbeat handler attached to this node.
// Returned for shutdown wiring so the caller can Close it on process exit.
func (n *Node) GetHeartbeatHandler() HeartbeatHandler {
	return n.heartbeatHandler
}

// CreateStores instantiate cachers for Transactions and Headers
func (n *Node) CreateStores() error {
	if n.dataPool == nil {
		return common.ErrNilDataPool
	}

	transactionsDataStore := n.dataPool.Transactions()
	headersDataStore := n.dataPool.Headers()

	if transactionsDataStore == nil {
		return errors.New("nil transaction sharded data store")
	}

	if headersDataStore == nil {
		return errors.New("nil header sharded data store")
	}

	return nil
}

// StartConsensus will start the consensus service for the current node
func (n *Node) StartConsensus() error {
	isGenesisBlockNotInitialized := len(n.blkc.GetGenesisHeaderHash()) == 0 ||
		check.IfNil(n.blkc.GetGenesisHeader())
	if isGenesisBlockNotInitialized {
		return common.ErrGenesisBlockNotInitialized
	}

	if !n.indexer.IsNilIndexer() {
		log.Warn("node is running with a valid indexer. Chronology watchdog will be turned off as " +
			"it is incompatible with the indexing process.")
		n.watchdog = &watchdog.DisabledWatchdog{}
	}
	if n.isInImportMode {
		log.Warn("node is running in import mode. Chronology watchdog will be turned off as " +
			"it is incompatible with the import-db process.")
		n.watchdog = &watchdog.DisabledWatchdog{}
	}

	alarmScheduler := alarm.NewAlarmScheduler()

	broadcastMessenger, err := broadcastFactory.GetBroadcastMessenger(
		&broadcastFactory.GetBroadcastMessengerArgs{
			Marshalizer:           n.internalMarshalizer,
			Hasher:                n.hasher,
			Messenger:             n.messenger,
			PrivateKey:            n.privKey,
			PeerSignatureHandler:  n.peerSigHandler,
			HeadersSubscriber:     n.dataPool.Headers(),
			InterceptorsContainer: n.interceptorsContainer,
			AlarmScheduler:        alarmScheduler,
		},
	)
	if err != nil {
		return err
	}

	chronologyHandler, err := n.createChronologyHandler(
		n.appStatusHandler,
		broadcastMessenger,
		n.watchdog,
	)
	if err != nil {
		return err
	}

	n.bootstrapper, err = n.createBootstrapper(n.slotManager)
	if err != nil {
		return err
	}

	err = n.bootstrapper.SetStatusHandler(n.GetAppStatusHandler())
	if err != nil {
		log.Debug("cannot set app status handler for shard bootstrapper")
	}

	n.bootstrapper.LoadStorage()

	log.Trace("creating proposal kapp")

	acnt, err := n.kapps.LoadAccount(kapps.ProposalKAppAddress)
	if err != nil {
		return err
	}

	proposalKApp, ok := acnt.(state.KAppAccountHandler)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	epoch := n.blkc.GetGenesisHeader().GetEpoch()
	crtBlockHeader := n.blkc.GetCurrentBlockHeader()
	if !check.IfNil(crtBlockHeader) {
		epoch = crtBlockHeader.GetEpoch()
	}

	if !check.IfNil(crtBlockHeader) && !check.IfNil(n.heartbeatHandler) {
		n.heartbeatHandler.RefreshPeerTypeCache(epoch)
	}

	forkControllerSubscriber, ok := n.forkController.(core.EpochSubscriberHandler)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	forkControllerSubscriber.EpochConfirmed(epoch)

	n.proposalController, err = proposalKApp.StartProposalsKApp(n.forkController)
	if err != nil {
		return err
	}

	err = n.kapps.SaveAccount(proposalKApp)
	if err != nil {
		return err
	}

	err = n.feeHandler.SetProposalController(n.proposalController)
	if err != nil {
		return err
	}

	err = n.blockProcessor.SetProposalController(n.proposalController)
	if err != nil {
		return err
	}

	err = n.txProcessor.SetProposalController(n.proposalController)
	if err != nil {
		return err
	}

	err = n.kappController.SetProposalController(n.proposalController)
	if err != nil {
		return err
	}

	err = n.kappControllerSimulator.SetProposalController(n.proposalController)
	if err != nil {
		return err
	}

	n.bootstrapper.LoadSyncUntil(n.syncUntil)
	n.bootstrapper.LoadStopNodeChannel(n.chanStopNodeProcess)
	n.bootstrapper.StartSyncingBlocks()

	log.Info("starting consensus", "epoch", epoch)

	consensusState, err := n.createConsensusState(epoch)
	if err != nil {
		return err
	}

	consensusService, err := slotFactory.GetConsensusCoreFactory(n.consensusType)
	if err != nil {
		return err
	}

	netInputMarshalizer := n.internalMarshalizer

	workerArgs := &slot.WorkerArgs{
		ConsensusService:         consensusService,
		BlockChain:               n.blkc,
		BlockProcessor:           n.blockProcessor,
		Bootstrapper:             n.bootstrapper,
		BroadcastMessenger:       broadcastMessenger,
		ConsensusState:           consensusState,
		ForkDetector:             n.forkDetector,
		Marshalizer:              netInputMarshalizer,
		Hasher:                   n.hasher,
		SlotManager:              n.slotManager,
		PeerSignatureHandler:     n.peerSigHandler,
		SyncTimer:                n.syncTimer,
		HeaderSigVerifier:        n.headerSigVerifier,
		HeaderIntegrityVerifier:  n.headerIntegrityVerifier,
		ChainID:                  n.chainID,
		NetworkShardingCollector: n.networkShardingCollector,
		AntifloodHandler:         n.inputAntifloodHandler,
		TXPool:                   n.dataPool.Transactions(),
		OnRequestTransactionTo:   n.requestHandler.RequestTransactionTo,
		SignatureSize:            n.validatorSignatureSize,
		PublicKeySize:            n.publicKeySize,
		NodeRedundancyHandler:    n.nodeRedundancyHandler,
		ConsensusMonitorList:     n.consensusMonitorList,

		NetworkDegradedThreshold:     n.networkDegradedThreshold,
		NetworkDegradedCooldownSlots: n.networkDegradedCooldownSlots,
	}

	worker, err := slot.NewWorker(workerArgs)
	if err != nil {
		return err
	}

	worker.StartWorking() // starts a go routine

	n.dataPool.Headers().RegisterHandler(worker.ReceivedHeader)

	// apply consensus group size on the input antiflooder just before consensus creation topic
	n.inputAntifloodHandler.ApplyConsensusSize(n.nodesCoordinator.ConsensusGroupSize())
	err = n.createConsensusTopic(worker)
	if err != nil {
		return err
	}

	consensusArgs := &slot.ConsensusCoreArgs{
		BlockChain:                    n.blkc,
		BlockProcessor:                n.blockProcessor,
		Bootstrapper:                  n.bootstrapper,
		BroadcastMessenger:            broadcastMessenger,
		ChronologyHandler:             chronologyHandler,
		Hasher:                        n.hasher,
		Marshalizer:                   n.internalMarshalizer,
		BlsPrivateKey:                 n.privKey,
		BlsSingleSigner:               n.singleSigner,
		MultiSigner:                   n.multiSigner,
		SlotManager:                   n.slotManager,
		NodesCoordinator:              n.nodesCoordinator,
		SyncTimer:                     n.syncTimer,
		EpochStartRegistrationHandler: n.epochStartRegistrationHandler,
		AntifloodHandler:              n.inputAntifloodHandler,
		PeerHonestyHandler:            n.peerHonestyHandler,
		HeaderSigVerifier:             n.headerSigVerifier,
		FallbackHeaderValidator:       n.fallbackHeaderValidator,
		NodeRedundancyHandler:         n.nodeRedundancyHandler,
		ForkController:                n.forkController,
	}

	consensusDataContainer, err := slot.NewConsensusCore(
		consensusArgs,
	)
	if err != nil {
		return err
	}

	fct, err := slotFactory.GetSubslotsFactory(
		consensusDataContainer,
		consensusState,
		worker,
		n.consensusType,
		n.appStatusHandler,
		n.indexer,
		n.chainID,
		n.messenger.ID(),
	)
	if err != nil {
		return err
	}

	err = fct.GenerateSubslots()
	if err != nil {
		return err
	}

	chronologyHandler.StartSlots()
	return nil
}

func (n *Node) createBootstrapper(slotManager consensus.SlotManager) (process.Bootstrapper, error) {
	return n.createMetaChainBootstrapper(slotManager)
}

// createConsensusTopic creates a consensus topic for node
func (n *Node) createConsensusTopic(messageProcessor p2p.MessageProcessor) error {
	if check.IfNil(messageProcessor) {
		return common.ErrNilMessenger
	}

	n.consensusTopic = common.ConsensusTopic
	if !n.messenger.HasTopic(n.consensusTopic) {
		err := n.messenger.CreateTopic(n.consensusTopic, true)
		if err != nil {
			return err
		}
	}

	if n.messenger.HasTopicValidator(n.consensusTopic) {
		return common.ErrValidatorAlreadySet
	}

	return n.messenger.RegisterMessageProcessor(n.consensusTopic, messageProcessor)
}

func (n *Node) addTransactionsToSendPipe(txs []*transaction.Transaction) {
	if check.IfNil(n.txAccumulator) {
		log.Error("node has a nil tx accumulator instance")
		return
	}

	for _, tx := range txs {
		n.txAccumulator.AddData(tx)
	}
}

func (n *Node) sendFromTxAccumulator() {
	outputChannel := n.txAccumulator.OutputChannel()

	for objs := range outputChannel {
		//this will read continuously until the chan is closed

		if len(objs) == 0 {
			continue
		}

		txs := make([]*transaction.Transaction, 0, len(objs))
		for _, obj := range objs {
			tx, ok := obj.(*transaction.Transaction)
			if !ok {
				continue
			}

			txs = append(txs, tx)
		}

		atomic.AddUint32(&n.txSentCounter, uint32(len(txs))) // #nosec G115

		n.sendBulkTransactions(txs)
	}
}

// SendTransaction sends the provided transaction to the network
func (n *Node) SendTransaction(tx *transaction.Transaction) (string, error) {
	// Consensus account freeze: refuse to accept or relay a tx submitted to this
	// node's API from a frozen account. Local ingestion policy, off the block /
	// sync path, so it cannot affect replay or block validity — the fork-gated
	// check in ProcessTransaction stays the consensus guarantee. Blocks from binary
	// rollout rather than from the freeze fork, but honours the per-account thaw, so
	// an account released at a later fork is accepted here too.
	// Returns a generic rejection (not a freeze-specific error) so the API can't be
	// used to enumerate the frozen set; the real reason is logged for operators.
	if common.IsAccountFrozenAtAPI(tx.GetSender(), n.forkController) {
		log.Debug("rejected API tx from frozen account", "sender", hex.EncodeToString(tx.GetSender()))
		return "", process.ErrWrongTransaction
	}

	// compute hash
	txHash, err := tools.CalculateHash(n.internalMarshalizer, n.hasher, tx.GetRaw())
	if err != nil {
		return "", err
	}

	err = n.ValidateTransaction(tx, true)
	if err != nil {
		return "", err
	}

	tx.PrepareForProcessing()
	n.addTransactionsToSendPipe([]*transaction.Transaction{tx})

	return hex.EncodeToString(txHash), nil
}

// SendBulkTransactions sends the provided transactions as a bulk, optimizing transfer between nodes
func (n *Node) SendBulkTransactions(txs []*transaction.Transaction) ([]string, error) {
	if len(txs) == 0 {
		return nil, common.ErrNoTxToProcess
	}

	txsHashes := make([]string, 0)
	for i, tx := range txs {

		// Consensus account freeze: refuse frozen-account txs at the API (see SendTransaction).
		if common.IsAccountFrozenAtAPI(tx.GetSender(), n.forkController) {
			log.Debug("rejected API tx from frozen account", "sender", hex.EncodeToString(tx.GetSender()))
			return nil, fmt.Errorf("invalid transaction %d: %w", i, process.ErrWrongTransaction)
		}

		err := n.ValidateTransaction(tx, true)
		if err != nil {
			return nil, fmt.Errorf("invalid transaction %d: %w", i, err)
		}

		txHash, err := tools.CalculateHash(n.internalMarshalizer, n.hasher, tx.GetRaw())
		if err != nil {
			return nil, err
		}

		tx.PrepareForProcessing()

		txsHashes = append(txsHashes, hex.EncodeToString(txHash))
	}

	n.addTransactionsToSendPipe(txs)

	return txsHashes, nil
}

// sendBulkTransactions sends the provided transactions as a bulk, optimizing transfer between nodes
func (n *Node) sendBulkTransactions(txs []*transaction.Transaction) {
	transactions := make([][]byte, 0)
	log.Trace("node.sendBulkTransactions sending txs",
		"num", len(txs),
	)

	for _, tx := range txs {

		marshalizedTx, err := n.internalMarshalizer.Marshal(tx)
		if err != nil {
			log.Warn("node.sendBulkTransactions",
				"marshalizer error", err,
			)
			continue
		}

		transactions = append(transactions, marshalizedTx)
	}

	err := n.sendBulkTransactionsPack(transactions)
	if err != nil {
		log.Debug("sendBulkTransactions", "error", err.Error())
	}
}

func (n *Node) sendBulkTransactionsPack(transactions [][]byte) error {
	dataPacker, err := partitioning.NewSimpleDataPacker(n.internalMarshalizer)
	if err != nil {
		return err
	}

	identifier := common.TransactionTopic

	packets, err := dataPacker.PackDataInChunks(transactions, core.MaxBulkTransactionSize)
	if err != nil {
		return err
	}

	atomic.AddInt32(&n.currentSendingGoRoutines, int32(len(packets))) // #nosec G115
	for _, buff := range packets {
		go func(bufferToSend []byte) {
			log.Trace("node.sendBulkTransactions",
				"topic", identifier,
				"size", len(bufferToSend),
			)
			err = n.messenger.BroadcastOnChannelBlocking(
				SendTransactionsPipe,
				identifier,
				bufferToSend,
			)
			if err != nil {
				log.Debug("node.BroadcastOnChannelBlocking", "error", err.Error())
			}

			atomic.AddInt32(&n.currentSendingGoRoutines, -1)
		}(buff)
	}

	return nil
}

// printTxSentCounter prints the peak transaction counter from a time frame of about 'numSecondsBetweenPrints' seconds
// if this peak value is 0 (no transaction was sent through the REST API interface), the print will not be done
// the peak counter resets after each print. There is also a total number of transactions sent to p2p
func (n *Node) printTxSentCounter() {
	maxTxCounter := uint32(0)
	totalTxCounter := uint64(0)
	counterSeconds := 0

	for {
		time.Sleep(time.Second)

		txSent := atomic.SwapUint32(&n.txSentCounter, 0)
		if txSent > maxTxCounter {
			maxTxCounter = txSent
		}
		totalTxCounter += uint64(txSent)

		counterSeconds++
		if counterSeconds > numSecondsBetweenPrints {
			counterSeconds = 0

			if maxTxCounter > 0 {
				log.Info("sent transactions on network",
					"max/sec", maxTxCounter,
					"total", totalTxCounter,
				)
			}
			maxTxCounter = 0
		}
	}
}

func (n *Node) commonTransactionValidation(
	tx *transaction.Transaction,
	whiteListerVerifiedTxs process.WhiteListHandler,
	whiteListRequest process.WhiteListHandler,
	checkSignature bool,
) (process.TxValidator, process.TxValidatorHandler, error) {

	txSingleSigner := n.txSingleSigner
	if !checkSignature {
		txSingleSigner = &disabledSig.DisabledSingleSig{}
	}

	txValidator, err := dataValidators.NewTxValidator(
		n.accounts,
		n.store.GetStorer(retriever.TransactionUnit),
		n.dataPool,
		whiteListRequest,
		n.addressPubkeyConverter,
		txSingleSigner,
		n.keyGenForAccounts,
		n.kappController,
		core.MaxTxNonceDeltaAllowed,
	)

	if err != nil {
		log.Warn("node.ValidateTransaction: can not instantiate a TxValidator",
			"error", err)
		return nil, nil, err
	}

	marshalizedTx, err := n.internalMarshalizer.Marshal(tx)
	if err != nil {
		return nil, nil, err
	}

	intTx, err := procTx.NewInterceptedTransaction(
		&procTx.InterceptedTransactionArgs{
			TxBuff:                 marshalizedTx,
			ProtoMarshalizer:       n.internalMarshalizer,
			SignMarshalizer:        n.txSignMarshalizer,
			Hasher:                 n.hasher,
			KeyGen:                 n.keyGenForAccounts,
			Signer:                 txSingleSigner,
			PubkeyConv:             n.addressPubkeyConverter,
			WhiteListerVerifiedTxs: whiteListerVerifiedTxs,
			ChainID:                n.chainID,
			TxSignHasher:           n.txSignHasher,
			FeeHandler:             n.feeHandler,
			TxVersionChecker:       versioning.NewTxVersionChecker(n.minTransactionVersion),
			ForkController:         n.forkController,
		},
	)
	if err != nil {
		return nil, nil, err
	}

	err = txValidator.CheckDup(intTx.Hash())
	if err != nil {
		return nil, nil, err
	}

	err = intTx.CheckValidity()
	if err != nil {
		return nil, nil, err
	}

	if checkSignature {
		err = intTx.CheckTXSignature()
		if err != nil {
			return nil, nil, err
		}
	}

	return txValidator, intTx, nil
}

// ValidateTransaction will validate a transaction
func (n *Node) ValidateTransaction(tx *transaction.Transaction, checkSignature bool) error {
	txValidator, intTx, err := n.commonTransactionValidation(tx, n.whiteListerVerifiedTxs, n.whiteListRequest, checkSignature)
	if err != nil {
		return err
	}

	return txValidator.CheckTxValidity(intTx)
}

// StartHeartbeat starts the node's heartbeat processing/signaling module
func (n *Node) StartHeartbeat(hbConfig config.HeartbeatConfig, versionNumber string, prefsConfig config.PreferencesConfig) error {
	arg := componentHandler.ArgHeartbeat{
		HeartbeatConfig:          hbConfig,
		PrefsConfig:              prefsConfig,
		Marshalizer:              n.internalMarshalizer,
		Messenger:                n.messenger,
		NodesCoordinator:         n.nodesCoordinator,
		AppStatusHandler:         n.appStatusHandler,
		Storer:                   n.store.GetStorer(retriever.HeartbeatUnit),
		ValidatorStatistics:      n.validatorStatistics,
		PeerSignatureHandler:     n.peerSigHandler,
		PrivKey:                  n.privKey,
		AntifloodHandler:         n.inputAntifloodHandler,
		ValidatorPubkeyConverter: n.validatorPubkeyConverter,
		EpochStartTrigger:        n.epochStartTrigger,
		EpochStartRegistration:   n.epochStartRegistrationHandler,
		Timer:                    &heartbeatProcess.RealTimer{},
		GenesisTime:              n.genesisTime,
		VersionNumber:            versionNumber,
		PeerShardMapper:          n.networkShardingCollector,
		CurrentBlockProvider:     n.blkc,
		RedundancyHandler:        n.nodeRedundancyHandler,
	}

	var err error
	n.heartbeatHandler, err = componentHandler.NewHeartbeatHandler(arg)

	return err
}

// GetHeartbeats returns the heartbeat status for each public key defined in genesis.json
func (n *Node) GetHeartbeats() []heartbeatData.PubKeyHeartbeat {
	if check.IfNil(n.heartbeatHandler) {
		return make([]heartbeatData.PubKeyHeartbeat, 0)
	}
	mon := n.heartbeatHandler.Monitor()
	if check.IfNil(mon) {
		return make([]heartbeatData.PubKeyHeartbeat, 0)
	}

	return mon.GetHeartbeats()
}

// ValidatorStatisticsAPI will return the statistics for all the validators from the initial nodes pub keys
func (n *Node) ValidatorStatisticsAPI() (map[string]*state.ValidatorApiResponse, error) {
	return n.validatorsProvider.GetLatestValidators(), nil
}

// PeersAPI will return the statistics for all the validators from the initial nodes pub keys
func (n *Node) PeersAPI() ([]state.PeerAccountHandler, error) {
	return n.validatorsProvider.GetLatestPeers(), nil
}

// EncodeAddressPubkey will encode the provided address public key bytes to string
func (n *Node) EncodeAddressPubkey(pk []byte) (string, error) {
	if n.addressPubkeyConverter == nil {
		return "", fmt.Errorf("%w for addressPubkeyConverter", common.ErrNilPubkeyConverter)
	}

	return n.addressPubkeyConverter.Encode(pk), nil
}

// DecodeAddressPubkey will try to decode the provided address public key string
func (n *Node) DecodeAddressPubkey(pk string) ([]byte, error) {
	if n.addressPubkeyConverter == nil {
		return nil, fmt.Errorf("%w for addressPubkeyConverter", common.ErrNilPubkeyConverter)
	}

	return n.addressPubkeyConverter.Decode(pk)
}

// AddQueryHandler adds a query handler in cache
func (n *Node) AddQueryHandler(name string, handler debug.QueryHandler) error {
	if check.IfNil(handler) {
		return common.ErrNilQueryHandler
	}
	if len(name) == 0 {
		return common.ErrEmptyQueryHandlerName
	}

	n.mutQueryHandlers.Lock()
	defer n.mutQueryHandlers.Unlock()

	_, ok := n.queryHandlers[name]
	if ok {
		return fmt.Errorf("%w with name %s", common.ErrQueryHandlerAlreadyExists, name)
	}

	n.queryHandlers[name] = handler

	return nil
}

// GetQueryHandler returns the query handler if existing
func (n *Node) GetQueryHandler(name string) (debug.QueryHandler, error) {
	n.mutQueryHandlers.RLock()
	defer n.mutQueryHandlers.RUnlock()

	qh, ok := n.queryHandlers[name]
	if !ok || check.IfNil(qh) {
		return nil, fmt.Errorf("%w for name %s", common.ErrNilQueryHandler, name)
	}

	return qh, nil
}

// SetRedundancy set node internal redundancy level
func (n *Node) SetRedundancy(level int64) error {
	return n.nodeRedundancyHandler.SetInternalRedundancyLevel(level)
}

// GetRedundancy get node internal redundancy level
func (n *Node) GetRedundancy() int64 {
	return n.nodeRedundancyHandler.GetInternalRedundancyLevel()
}

// GetEnableEpochs -
func (n *Node) GetEnableEpochs() (config.EnableEpochsConfig, error) {
	return n.enableEpochsConfig, nil
}

// GetPeerInfo returns information about a peer id
func (n *Node) GetPeerInfo(pid string) ([]core.QueryP2PPeerInfo, error) {
	peers := n.messenger.Peers()
	pidsFound := make([]core.PeerID, 0)
	for _, p := range peers {
		if strings.Contains(p.Pretty(), pid) {
			pidsFound = append(pidsFound, p)
		}
	}

	if len(pidsFound) == 0 {
		return nil, fmt.Errorf("%w for provided peer %s", common.ErrUnknownPeerID, pid)
	}

	sort.Slice(pidsFound, func(i, j int) bool {
		return pidsFound[i].Pretty() < pidsFound[j].Pretty()
	})

	peerInfoSlice := make([]core.QueryP2PPeerInfo, 0, len(pidsFound))
	for _, p := range pidsFound {
		pidInfo := n.createPidInfo(p)
		if pidInfo.PeerType != "unknown" && len(pidInfo.Addresses) > 0 {
			peerInfoSlice = append(peerInfoSlice, pidInfo)
		}
	}

	return peerInfoSlice, nil
}

func (n *Node) GetProposalParameters() (map[int32]*kapps.Parameter, error) {
	return n.proposalController.GetActiveParameters(), nil
}

func (n *Node) createPidInfo(p core.PeerID) core.QueryP2PPeerInfo {
	result := core.QueryP2PPeerInfo{
		Pid:           p.Pretty(),
		Addresses:     n.messenger.PeerAddresses(p),
		IsBlacklisted: n.peerDenialEvaluator.IsDenied(p),
	}

	peerInfo := n.networkShardingCollector.GetPeerInfo(p)
	result.PeerType = peerInfo.PeerType.String()
	if len(peerInfo.PkBytes) == 0 {
		result.Pk = ""
	} else {
		result.Pk = n.validatorPubkeyConverter.Encode(peerInfo.PkBytes)
	}

	return result
}

// createChronologyHandler method creates a chronology object
func (n *Node) createChronologyHandler(
	appStatusHandler core.AppStatusHandler,
	broadcastMessenger consensus.BroadcastMessenger,
	watchdog core.WatchdogTimer,
) (consensus.ChronologyHandler, error) {
	chr, err := chronology.NewChronology(
		n.slotManager,
		n.blkc,
		n.nodesCoordinator,
		n.blockProcessor,
		broadcastMessenger,
		n.syncTimer,
		watchdog,
	)
	if err != nil {
		return nil, err
	}

	err = chr.SetAppStatusHandler(appStatusHandler)
	if err != nil {
		return nil, err
	}

	return chr, nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (n *Node) IsInterfaceNil() bool {
	return n == nil
}
