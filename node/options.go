package node

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/ntp"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/tools/typeConverters"
)

// WithMessenger sets up the messenger option for the Node
func WithMessenger(mes P2PMessenger) Option {
	return func(n *Node) error {
		if check.IfNil(mes) {
			return common.ErrNilMessenger
		}
		n.messenger = mes
		return nil
	}
}

// WithHasher sets up the hasher option for the Node
func WithHasher(hasher hashing.Hasher) Option {
	return func(n *Node) error {
		if check.IfNil(hasher) {
			return common.ErrNilHasher
		}
		n.hasher = hasher
		return nil
	}
}

// WithInternalMarshalizer sets up the marshalizer option for the Node
func WithInternalMarshalizer(marshalizer marshal.Marshalizer) Option {
	return func(n *Node) error {
		if check.IfNil(marshalizer) {
			return common.ErrNilMarshalizer
		}
		n.internalMarshalizer = marshalizer
		return nil
	}
}

// WithTxSignMarshalizer sets up the marshalizer used in transaction singning
func WithTxSignMarshalizer(marshalizer marshal.Marshalizer) Option {
	return func(n *Node) error {
		if check.IfNil(marshalizer) {
			return common.ErrNilMarshalizer
		}
		n.txSignMarshalizer = marshalizer
		return nil
	}
}

// WithAddressPubkeyConverter sets up the address public key converter adapter option for the Node
func WithAddressPubkeyConverter(pubkeyConverter core.PubkeyConverter) Option {
	return func(n *Node) error {
		if check.IfNil(pubkeyConverter) {
			return fmt.Errorf("%w in WithAddressPubkeyConverter", common.ErrNilPubkeyConverter)
		}
		n.addressPubkeyConverter = pubkeyConverter

		emptyAddress := bytes.Repeat([]byte{0}, pubkeyConverter.Len())
		encodedEmptyAddress := pubkeyConverter.Encode(emptyAddress)
		n.encodedAddressLength = len(encodedEmptyAddress)

		return nil
	}
}

// WithValidatorPubkeyConverter sets up the validator public key converter adapter option for the Node
func WithValidatorPubkeyConverter(pubkeyConverter core.PubkeyConverter) Option {
	return func(n *Node) error {
		if check.IfNil(pubkeyConverter) {
			return fmt.Errorf("%w in WithValidatorPubkeyConverter", common.ErrNilPubkeyConverter)
		}
		n.validatorPubkeyConverter = pubkeyConverter
		return nil
	}
}

// WithDataStore sets up the storage options for the Node
func WithDataStore(store retriever.StorageService) Option {
	return func(n *Node) error {
		if store == nil || store.IsInterfaceNil() {
			return common.ErrNilStore
		}
		n.store = store
		return nil
	}
}

// WithDataPool sets up the data pools option for the Node
func WithDataPool(dataPool retriever.PoolsHolder) Option {
	return func(n *Node) error {
		if check.IfNil(dataPool) {
			return common.ErrNilDataPool
		}
		n.dataPool = dataPool
		return nil
	}
}

// WithBlockChain sets up the blockchain option for the Node
func WithBlockChain(blkc data.ChainHandler) Option {
	return func(n *Node) error {
		if check.IfNil(blkc) {
			return common.ErrNilBlockchain
		}
		n.blkc = blkc
		return nil
	}
}

// WithAccountsAdapter sets up the accounts adapter option for the Node
func WithAccountsAdapter(accounts state.AccountsAdapter) Option {
	return func(n *Node) error {
		if check.IfNil(accounts) {
			return common.ErrNilAccountsAdapter
		}
		n.accounts = accounts
		return nil
	}
}

// WithKAppsAdapter sets up the assets adapter option for the Node
func WithKAppsAdapter(kapps state.AccountsAdapter) Option {
	return func(n *Node) error {
		if check.IfNil(kapps) {
			return common.ErrNilKAppAccountsAdapter
		}
		n.kapps = kapps
		return nil
	}
}

// WithInitialNodesPubKeys sets up the initial nodes public key option for the Node
func WithInitialNodesPubKeys(pubKeys []string) Option {
	return func(n *Node) error {
		n.initialNodesPubkeys = pubKeys
		return nil
	}
}

// WithPubKey sets up the multi sign pub key option for the Node
func WithPubKey(pk crypto.PublicKey) Option {
	return func(n *Node) error {
		if check.IfNil(pk) {
			return crypto.ErrNilPublicKey
		}
		n.pubKey = pk
		return nil
	}
}

// WithPrivKey sets up the multi sign private key option for the Node
func WithPrivKey(sk crypto.PrivateKey) Option {
	return func(n *Node) error {
		if check.IfNil(sk) {
			return crypto.ErrNilPrivateKey
		}
		n.privKey = sk
		return nil
	}
}

// WithKeyGen sets up the single sign key generator option for the Node
func WithKeyGen(keyGen crypto.KeyGenerator) Option {
	return func(n *Node) error {
		if check.IfNil(keyGen) {
			return crypto.ErrNilSingleSignKeyGen
		}
		n.keyGenForBlock = keyGen
		return nil
	}
}

// WithKeyGenForAccounts sets up the balances key generator option for the Node
func WithKeyGenForAccounts(keyGenForAccounts crypto.KeyGenerator) Option {
	return func(n *Node) error {
		if check.IfNil(keyGenForAccounts) {
			return crypto.ErrNilTXKeyGen
		}
		n.keyGenForAccounts = keyGenForAccounts
		return nil
	}
}

// WithForkDetector sets up the multiSigner option for the Node
func WithForkDetector(forkDetector process.ForkDetector) Option {
	return func(n *Node) error {
		if check.IfNil(forkDetector) {
			return common.ErrNilForkDetector
		}
		n.forkDetector = forkDetector
		return nil
	}
}

// WithValidatorsProvider sets up the validators provider for the node
func WithValidatorsProvider(validatorsProvider process.ValidatorsProvider) Option {
	return func(n *Node) error {
		if check.IfNil(validatorsProvider) {
			return common.ErrNilValidatorProvider
		}
		n.validatorsProvider = validatorsProvider
		return nil
	}
}

// WithChainID sets up the chain ID on which the current node is supposed to work on
func WithChainID(chainID []byte) Option {
	return func(n *Node) error {
		if len(chainID) == 0 {
			return common.ErrInvalidChainID
		}
		n.chainID = chainID

		return nil
	}
}

// WithConsensusGroupSize sets up the consensus group size option for the Node
func WithConsensusGroupSize(consensusGroupSize int) Option {
	return func(n *Node) error {
		if consensusGroupSize < 1 {
			return common.ErrNegativeOrZeroConsensusGroupSize
		}
		n.consensusGroupSize = consensusGroupSize
		return nil
	}
}

// WithSyncer sets up the syncTimer option for the Node
func WithSyncer(syncer ntp.SyncTimer) Option {
	return func(n *Node) error {
		if check.IfNil(syncer) {
			return common.ErrNilSyncTimer
		}
		n.syncTimer = syncer
		return nil
	}
}

// WithTxFeeHandler sets up the tx fee handler for the Node
func WithTxFeeHandler(feeHandler process.EconomicsDataHandler) Option {
	return func(n *Node) error {
		if check.IfNil(feeHandler) {
			return process.ErrNilTxFeeHandler
		}
		n.feeHandler = feeHandler
		return nil
	}
}

// WithBlockProcessor sets up the block processor option for the Node
func WithBlockProcessor(blockProcessor process.BlockProcessor) Option {
	return func(n *Node) error {
		if check.IfNil(blockProcessor) {
			return common.ErrNilBlockProcessor
		}
		n.blockProcessor = blockProcessor
		return nil
	}
}

// WithTransactionProcessor sets up the tx processor option for the Node
func WithTransactionProcessor(txProcessor process.TransactionProcessor) Option {
	return func(n *Node) error {
		if check.IfNil(txProcessor) {
			return common.ErrNilTransactionProcessor
		}
		n.txProcessor = txProcessor
		return nil
	}
}

// WithKAppController sets up the KApp Controller option for the Node
func WithKAppController(kAppController kapp.KAppController) Option {
	return func(n *Node) error {
		if check.IfNil(kAppController) {
			return common.ErrNilKAppController
		}
		n.kappController = kAppController
		return nil
	}
}

// WithSimulatorKAppController sets up the KApp Controller option for the Node TX Simulator
func WithSimulatorKAppController(kAppController kapp.KAppController) Option {
	return func(n *Node) error {
		if check.IfNil(kAppController) {
			return common.ErrNilKAppController
		}
		n.kappControllerSimulator = kAppController
		return nil
	}
}

// WithGenesisTime sets up the genesis time option for the Node
func WithGenesisTime(genesisTime time.Time) Option {
	return func(n *Node) error {
		n.genesisTime = genesisTime
		return nil
	}
}

// WithNodesCoordinator sets up the nodes coordinator
func WithNodesCoordinator(nodesCoordinator sharding.NodesCoordinator) Option {
	return func(n *Node) error {
		if check.IfNil(nodesCoordinator) {
			return common.ErrNilNodesCoordinator
		}
		n.nodesCoordinator = nodesCoordinator
		return nil
	}
}

// WithUint64ByteSliceConverter sets up the uint64 <-> []byte converter
func WithUint64ByteSliceConverter(converter typeConverters.Uint64ByteSliceConverter) Option {
	return func(n *Node) error {
		if check.IfNil(converter) {
			return common.ErrNilUint64ByteSliceConverter
		}
		n.uint64ByteSliceConverter = converter
		return nil
	}
}

// WithSingleSigner sets up a singleSigner option for the Node
func WithSingleSigner(singleSigner crypto.SingleSigner) Option {
	return func(n *Node) error {
		if check.IfNil(singleSigner) {
			return common.ErrNilSingleSig
		}
		n.singleSigner = singleSigner
		return nil
	}
}

// WithMultiSigner sets up the multiSigner option for the Node
func WithMultiSigner(multiSigner crypto.MultiSigner) Option {
	return func(n *Node) error {
		if check.IfNil(multiSigner) {
			return common.ErrNilMultiSig
		}
		n.multiSigner = multiSigner
		return nil
	}
}

// WithInterceptorsContainer sets up the interceptors container option for the Node
func WithInterceptorsContainer(interceptorsContainer process.InterceptorsContainer) Option {
	return func(n *Node) error {
		if check.IfNil(interceptorsContainer) {
			return common.ErrNilInterceptorsContainer
		}
		n.interceptorsContainer = interceptorsContainer
		return nil
	}
}

// WithAppStatusHandler sets up which handler will monitor the status of the node
func WithAppStatusHandler(aph core.AppStatusHandler) Option {
	return func(n *Node) error {
		if check.IfNil(aph) {
			return common.ErrNilStatusHandler

		}
		n.appStatusHandler = aph
		return nil
	}
}

// WithPublicKeySize sets up a publicKeySize option for the Node
func WithPublicKeySize(publicKeySize int) Option {
	return func(n *Node) error {
		n.publicKeySize = publicKeySize
		return nil
	}
}

// WithValidatorSignatureSize sets up a validatorSignatureSize option for the Node
func WithValidatorSignatureSize(signatureSize int) Option {
	return func(n *Node) error {
		n.validatorSignatureSize = signatureSize
		return nil
	}
}

// WithPeerHonestyHandler sets up a peer honesty handler for the Node
func WithPeerHonestyHandler(peerHonestyHandler consensus.PeerHonestyHandler) Option {
	return func(n *Node) error {
		if check.IfNil(peerHonestyHandler) {
			return common.ErrNilPeerHonestyHandler
		}
		n.peerHonestyHandler = peerHonestyHandler
		return nil
	}
}

// WithNodeStopChannel sets up the channel which will handle closing the node
func WithNodeStopChannel(channel chan endProcess.ArgEndProcess) Option {
	return func(n *Node) error {
		if channel == nil {
			return common.ErrNilNodeStopChannel
		}
		n.chanStopNodeProcess = channel

		return nil
	}
}

// WithWatchdogTimer sets up a watchdog for the Node
func WithWatchdogTimer(watchdog core.WatchdogTimer) Option {
	return func(n *Node) error {
		if check.IfNil(watchdog) {
			return common.ErrNilWatchdog
		}

		n.watchdog = watchdog
		return nil
	}
}

// WithPeerSignatureHandler sets up a peerSignatureHandler for the Node
func WithPeerSignatureHandler(peerSignatureHandler crypto.PeerSignatureHandler) Option {
	return func(n *Node) error {
		if check.IfNil(peerSignatureHandler) {
			return common.ErrNilPeerSignatureHandler
		}
		n.peerSigHandler = peerSignatureHandler
		return nil
	}
}

// WithResolversFinder sets up the resolvers finder option for the Node
func WithResolversFinder(resolversFinder retriever.ResolversFinder) Option {
	return func(n *Node) error {
		if check.IfNil(resolversFinder) {
			return common.ErrNilResolversFinder
		}
		n.resolversFinder = resolversFinder
		return nil
	}
}

// WithTxSingleSigner sets up a txSingleSigner option for the Node
func WithTxSingleSigner(txSingleSigner crypto.SingleSigner) Option {
	return func(n *Node) error {
		if check.IfNil(txSingleSigner) {
			return common.ErrNilSingleSig
		}
		n.txSingleSigner = txSingleSigner
		return nil
	}
}

// WithEpochStartTrigger sets up an start of epoch trigger option for the node
func WithEpochStartTrigger(epochStartTrigger process.EpochStartTriggerHandler) Option {
	return func(n *Node) error {
		if check.IfNil(epochStartTrigger) {
			return common.ErrNilEpochStartTrigger
		}
		n.epochStartTrigger = epochStartTrigger
		return nil
	}
}

// WithEpochStartEventNotifier sets up the notifier for the epoch start event
func WithEpochStartEventNotifier(epochStartEventNotifier eventNotifier.RegistrationHandler) Option {
	return func(n *Node) error {
		if epochStartEventNotifier == nil {
			return common.ErrNilEpochStartTrigger
		}
		n.epochStartRegistrationHandler = epochStartEventNotifier
		return nil
	}
}

// WithBlockBlackListHandler sets up a block black list handler for the Node
func WithBlockBlackListHandler(blackListHandler process.TimeCacher) Option {
	return func(n *Node) error {
		if check.IfNil(blackListHandler) {
			return fmt.Errorf("%w for WithBlockBlackListHandler", storage.ErrNilTimeCache)
		}
		n.blocksBlackListHandler = blackListHandler
		return nil
	}
}

// WithPeerDenialEvaluator sets up a peer denial evaluator for the Node
func WithPeerDenialEvaluator(handler p2p.PeerDenialEvaluator) Option {
	return func(n *Node) error {
		if check.IfNil(handler) {
			return fmt.Errorf("%w for WithPeerDenialEvaluator", common.ErrNilPeerDenialEvaluator)
		}
		n.peerDenialEvaluator = handler
		return nil
	}
}

// WithNetworkShardingCollector sets up a network sharding updater for the Node
func WithNetworkShardingCollector(networkShardingCollector NetworkShardingCollector) Option {
	return func(n *Node) error {
		if check.IfNil(networkShardingCollector) {
			return common.ErrNilNetworkShardingCollector
		}
		n.networkShardingCollector = networkShardingCollector
		return nil
	}
}

// WithConsensusType sets up the consensus type option for the Node
func WithConsensusType(consensusType string) Option {
	return func(n *Node) error {
		n.consensusType = consensusType
		return nil
	}
}

// WithBootstrapSlotIndex sets up a bootstrapSlotIndex option for the Node
func WithBootstrapSlotIndex(bootstrapSlotIndex uint64) Option {
	return func(n *Node) error {
		n.bootstrapSlotIndex = bootstrapSlotIndex
		return nil
	}
}

// WithRequestedItemsHandler sets up a requested items handler for the Node
func WithRequestedItemsHandler(requestedItemsHandler retriever.RequestedItemsHandler) Option {
	return func(n *Node) error {
		if check.IfNil(requestedItemsHandler) {
			return common.ErrNilRequestedItemsHandler
		}
		n.requestedItemsHandler = requestedItemsHandler
		return nil
	}
}

// WithMinTransactionVersion sets up the minimum transaction version on which the current node is supposed to work on
func WithMinTransactionVersion(minTransactionVersion uint32) Option {
	return func(n *Node) error {
		if minTransactionVersion == 0 {
			return process.ErrInvalidTransactionVersion
		}
		n.minTransactionVersion = minTransactionVersion

		return nil
	}
}

// WithRequestHandler sets up the request handler for the Node
func WithRequestHandler(requestHandler process.RequestHandler) Option {
	return func(n *Node) error {
		if check.IfNil(requestHandler) {
			return common.ErrNilRequestHandler
		}
		n.requestHandler = requestHandler
		return nil
	}
}

// WithInputAntifloodHandler sets up an antiflood handler for the Node on the input side
func WithInputAntifloodHandler(antifloodHandler P2PAntifloodHandler) Option {
	return func(n *Node) error {
		if check.IfNil(antifloodHandler) {
			return fmt.Errorf("%w on input", common.ErrNilAntifloodHandler)
		}
		n.inputAntifloodHandler = antifloodHandler
		return nil
	}
}

// WithTxAccumulator sets up a transaction accumulator handler for the Node
func WithTxAccumulator(accumulator Accumulator) Option {
	return func(n *Node) error {
		if check.IfNil(accumulator) {
			return common.ErrNilTxAccumulator
		}
		n.txAccumulator = accumulator

		go n.sendFromTxAccumulator()
		go n.printTxSentCounter()

		return nil
	}
}

// WithWhiteListHandler sets up a white list handler option
func WithWhiteListHandler(whiteListHandler process.WhiteListHandler) Option {
	return func(n *Node) error {
		if check.IfNil(whiteListHandler) {
			return common.ErrNilWhiteListHandler
		}

		n.whiteListRequest = whiteListHandler

		return nil
	}
}

// WithWhiteListHandlerVerified sets up a white list handler option
func WithWhiteListHandlerVerified(whiteListHandler process.WhiteListHandler) Option {
	return func(n *Node) error {
		if check.IfNil(whiteListHandler) {
			return common.ErrNilWhiteListHandler
		}

		n.whiteListerVerifiedTxs = whiteListHandler

		return nil
	}
}

// WithBootStorer sets up a boot storer for the Node
func WithBootStorer(bootStorer process.BootStorer) Option {
	return func(n *Node) error {
		if check.IfNil(bootStorer) {
			return common.ErrNilBootStorer
		}
		n.bootStorer = bootStorer
		return nil
	}
}

// WithSlotManager sets up the slot manager option for the Node
func WithSlotManager(sm consensus.SlotManager) Option {
	return func(n *Node) error {
		if check.IfNil(sm) {
			return process.ErrNilSlotManager
		}
		n.slotManager = sm
		return nil
	}
}

// WithTxSignHasher sets up a transaction sign hasher for the node
func WithTxSignHasher(txSignHasher hashing.Hasher) Option {
	return func(n *Node) error {
		if check.IfNil(txSignHasher) {
			return common.ErrNilHasher
		}
		n.txSignHasher = txSignHasher
		return nil
	}
}

// WithIndexer sets up a indexer for the Node
func WithIndexer(indexer process.Indexer) Option {
	return func(n *Node) error {
		n.indexer = indexer
		return nil
	}
}

// WithFallbackHeaderValidator sets up a fallback header validator for the Node
func WithFallbackHeaderValidator(fallbackHeaderValidator consensus.FallbackHeaderValidator) Option {
	return func(n *Node) error {
		if check.IfNil(fallbackHeaderValidator) {
			return process.ErrNilFallbackHeaderValidator
		}
		n.fallbackHeaderValidator = fallbackHeaderValidator
		return nil
	}
}

// WithHeaderSigVerifier sets up a header sig verifier for the Node
func WithHeaderSigVerifier(headerSigVerifier consensus.HeaderSigVerifier) Option {
	return func(n *Node) error {
		if check.IfNil(headerSigVerifier) {
			return slot.ErrNilHeaderSigVerifier
		}
		n.headerSigVerifier = headerSigVerifier
		return nil
	}
}

// WithHeaderIntegrityVerifier sets up a header integrity verifier for the Node
func WithHeaderIntegrityVerifier(headerIntegrityVerifier slot.HeaderIntegrityVerifier) Option {
	return func(n *Node) error {
		if check.IfNil(headerIntegrityVerifier) {
			return slot.ErrNilHeaderIntegrityVerifier
		}
		n.headerIntegrityVerifier = headerIntegrityVerifier
		return nil
	}
}

// WithNodeRedundancyHandler sets up a node redundancy handler for the node
func WithNodeRedundancyHandler(nodeRedundancyHandler consensus.NodeRedundancyHandler) Option {
	return func(n *Node) error {
		if check.IfNil(nodeRedundancyHandler) {
			return common.ErrNilNodeRedundancyHandler
		}
		n.nodeRedundancyHandler = nodeRedundancyHandler
		return nil
	}
}

// WithValidatorStatistics sets up the validator statistics for the node
func WithValidatorStatistics(validatorStatistics process.ValidatorStatisticsProcessor) Option {
	return func(n *Node) error {
		if check.IfNil(validatorStatistics) {
			return common.ErrNilValidatorStatistics
		}
		n.validatorStatistics = validatorStatistics
		return nil
	}
}

// WithImportMode sets up the flag if the node is running in import mode
func WithImportMode(importMode bool) Option {
	return func(n *Node) error {
		n.isInImportMode = importMode
		return nil
	}
}

// WithProposalController sets up the proposalController
func WithProposalController(controller kapps.ActiveProposalController) Option {
	return func(n *Node) error {
		if check.IfNil(controller) {
			return common.ErrNilProposalController
		}

		n.proposalController = controller

		return nil
	}
}

// WithForkController sets up the forkController
func WithForkController(controller core.ForkController) Option {
	return func(n *Node) error {
		if check.IfNil(controller) {
			return common.ErrNilForkController
		}

		n.forkController = controller

		return nil
	}
}

// WithEnableEpochsConfig sets up enable epochs config info
func WithEnableEpochsConfig(cfg *config.EnableEpochsConfig) Option {
	return func(n *Node) error {

		n.enableEpochsConfig = *cfg

		return nil
	}
}

// WithConsensusMonitorList sets list of pubkeys to report if fail on consensus messages
func WithConsensusMonitorList(list []string) Option {
	return func(n *Node) error {
		n.consensusMonitorList = make([]string, 0)

		// convert hex pubkey to byte string
		for i, pubkey := range list {
			data, err := hex.DecodeString(pubkey)
			if err != nil {
				log.Error("node.withConsensusMonitorList", "index", i, "data", pubkey, "pubkeyLen", len(data), "error", err.Error())
				continue
			}

			if len(data) != n.publicKeySize {
				log.Error("node.withConsensusMonitorList", "index", i, "data", pubkey, "pubkeyLen", len(data))
				continue
			}

			log.Info("monitoring consensus messages for", "pubkey", pubkey)

			n.consensusMonitorList = append(n.consensusMonitorList, string(data))
		}

		return nil
	}
}

// WithConsensusMonitoring sets the consensus monitoring configuration
func WithConsensusMonitoring(cfg *config.ConsensusMonitoringConfig) Option {
	return func(n *Node) error {
		// if nil, monitoring is disabled
		if cfg == nil || cfg.NetworkDegradedThreshold == 0 {
			n.networkDegradedThreshold = 0
			n.networkDegradedCooldownSlots = 0
			return nil
		}

		n.networkDegradedThreshold = cfg.NetworkDegradedThreshold
		if cfg.NetworkDegradedCooldownSlots > 0 {
			n.networkDegradedCooldownSlots = cfg.NetworkDegradedCooldownSlots
		}
		return nil
	}
}

// WithSyncUntil sets epochs to end config info
func WithSyncUntil(syncUntil uint32) Option {
	return func(n *Node) error {
		n.syncUntil = syncUntil

		return nil
	}
}

// WithStartInSync sets node to start with inSync state
func WithStartInSync(startWithInSync bool) Option {
	return func(n *Node) error {
		n.startWithInSync = startWithInSync

		return nil
	}
}
