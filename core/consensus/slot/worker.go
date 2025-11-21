package slot

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/closing"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/ntp"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ closing.Closer = (*Worker)(nil)

// sleepTime defines the time in milliseconds between each iteration made in checkChannels method
const sleepTime = 5 * time.Millisecond

// Worker defines the data needed by spos to communicate between nodes which are in the validators group
type Worker struct {
	consensusService        ConsensusService
	blockChain              data.ChainHandler
	blockProcessor          process.BlockProcessor
	bootstrapper            process.Bootstrapper
	broadcastMessenger      consensus.BroadcastMessenger
	consensusState          *ConsensusState
	forkDetector            process.ForkDetector
	marshalizer             marshal.Marshalizer
	hasher                  hashing.Hasher
	slotManager             consensus.SlotManager
	peerSignatureHandler    crypto.PeerSignatureHandler
	syncTimer               ntp.SyncTimer
	headerSigVerifier       RandSeedVerifier
	headerIntegrityVerifier HeaderIntegrityVerifier
	appStatusHandler        core.AppStatusHandler

	networkShardingCollector consensus.NetworkShardingCollector

	receivedMessages      map[consensus.MessageType][]*consensus.Message
	receivedMessagesCalls map[consensus.MessageType]func(*consensus.Message) bool

	executeMessageChannel        chan *consensus.Message
	consensusStateChangedChannel chan bool

	mutReceivedMessages      sync.RWMutex
	mutReceivedMessagesCalls sync.RWMutex

	mapDisplayHashConsensusMessage map[string][]*consensus.Message
	mutDisplayHashConsensusMessage sync.RWMutex

	receivedHeadersHandlers   []func(headerHandler data.HeaderHandler)
	mutReceivedHeadersHandler sync.RWMutex

	antifloodHandler consensus.P2PAntifloodHandler

	txPool                 retriever.ShardedDataCacherNotifier
	onRequestTransactionTo func(txHashes [][]byte, peer core.PeerID)

	cancelFunc                func()
	consensusMessageValidator *consensusMessageValidator
	nodeRedundancyHandler     consensus.NodeRedundancyHandler

	consensusMonitorList []string
}

// WorkerArgs holds the consensus worker arguments
type WorkerArgs struct {
	ConsensusService         ConsensusService
	BlockChain               data.ChainHandler
	BlockProcessor           process.BlockProcessor
	Bootstrapper             process.Bootstrapper
	BroadcastMessenger       consensus.BroadcastMessenger
	ConsensusState           *ConsensusState
	ForkDetector             process.ForkDetector
	Marshalizer              marshal.Marshalizer
	Hasher                   hashing.Hasher
	SlotManager              consensus.SlotManager
	PeerSignatureHandler     crypto.PeerSignatureHandler
	SyncTimer                ntp.SyncTimer
	HeaderSigVerifier        RandSeedVerifier
	HeaderIntegrityVerifier  HeaderIntegrityVerifier
	ChainID                  []byte
	NetworkShardingCollector consensus.NetworkShardingCollector
	AntifloodHandler         consensus.P2PAntifloodHandler
	TXPool                   retriever.ShardedDataCacherNotifier
	OnRequestTransactionTo   func(txHashes [][]byte, peer core.PeerID)
	SignatureSize            int
	PublicKeySize            int
	NodeRedundancyHandler    consensus.NodeRedundancyHandler
	ConsensusMonitorList     []string
}

// NewWorker creates a new Worker object
func NewWorker(args *WorkerArgs) (*Worker, error) {
	err := checkNewWorkerParams(args)
	if err != nil {
		return nil, err
	}

	argsConsensusMessageValidator := &ArgsConsensusMessageValidator{
		ConsensusState:       args.ConsensusState,
		ConsensusService:     args.ConsensusService,
		PeerSignatureHandler: args.PeerSignatureHandler,
		SignatureSize:        args.SignatureSize,
		PublicKeySize:        args.PublicKeySize,
		HasherSize:           args.Hasher.Size(),
		ChainID:              args.ChainID,
	}

	consensusMessageValidatorObj, err := NewConsensusMessageValidator(argsConsensusMessageValidator)
	if err != nil {
		return nil, err
	}

	wrk := Worker{
		consensusService:         args.ConsensusService,
		blockChain:               args.BlockChain,
		blockProcessor:           args.BlockProcessor,
		bootstrapper:             args.Bootstrapper,
		broadcastMessenger:       args.BroadcastMessenger,
		consensusState:           args.ConsensusState,
		forkDetector:             args.ForkDetector,
		marshalizer:              args.Marshalizer,
		hasher:                   args.Hasher,
		slotManager:              args.SlotManager,
		peerSignatureHandler:     args.PeerSignatureHandler,
		syncTimer:                args.SyncTimer,
		headerSigVerifier:        args.HeaderSigVerifier,
		headerIntegrityVerifier:  args.HeaderIntegrityVerifier,
		appStatusHandler:         statusHandler.NewNilStatusHandler(),
		networkShardingCollector: args.NetworkShardingCollector,
		antifloodHandler:         args.AntifloodHandler,
		txPool:                   args.TXPool,
		onRequestTransactionTo:   args.OnRequestTransactionTo,
		nodeRedundancyHandler:    args.NodeRedundancyHandler,
		consensusMonitorList:     args.ConsensusMonitorList,
	}

	wrk.consensusMessageValidator = consensusMessageValidatorObj
	wrk.executeMessageChannel = make(chan *consensus.Message)
	wrk.receivedMessagesCalls = make(map[consensus.MessageType]func(*consensus.Message) bool)
	wrk.receivedHeadersHandlers = make([]func(data.HeaderHandler), 0)
	wrk.consensusStateChangedChannel = make(chan bool, 1)
	wrk.bootstrapper.AddSyncStateListener(wrk.receivedSyncState)
	wrk.initReceivedMessages()

	// set the limit for the antiflood handler
	topic := common.ConsensusTopic
	maxMessagesInASlotPerPeer := wrk.consensusService.GetMaxMessagesInASlotPerPeer()
	wrk.antifloodHandler.SetMaxMessagesForTopic(topic, maxMessagesInASlotPerPeer)

	wrk.mapDisplayHashConsensusMessage = make(map[string][]*consensus.Message)

	return &wrk, nil
}

// StartWorking actually starts the consensus working mechanism
func (wrk *Worker) StartWorking() {
	var ctx context.Context
	ctx, wrk.cancelFunc = context.WithCancel(context.Background())
	go wrk.checkChannels(ctx)
}

func checkNewWorkerParams(args *WorkerArgs) error {
	if args == nil {
		return ErrNilWorkerArgs
	}
	if check.IfNil(args.ConsensusService) {
		return ErrNilConsensusService
	}
	if check.IfNil(args.BlockChain) {
		return ErrNilBlockChain
	}
	if check.IfNil(args.BlockProcessor) {
		return ErrNilBlockProcessor
	}
	if check.IfNil(args.Bootstrapper) {
		return ErrNilBootstrapper
	}
	if check.IfNil(args.BroadcastMessenger) {
		return ErrNilBroadcastMessenger
	}
	if args.ConsensusState == nil {
		return ErrNilConsensusState
	}
	if check.IfNil(args.ForkDetector) {
		return ErrNilForkDetector
	}
	if check.IfNil(args.Marshalizer) {
		return ErrNilMarshalizer
	}
	if check.IfNil(args.Hasher) {
		return ErrNilHasher
	}
	if check.IfNil(args.SlotManager) {
		return ErrNilSlotManager
	}
	if check.IfNil(args.PeerSignatureHandler) {
		return ErrNilPeerSignatureHandler
	}
	if check.IfNil(args.SyncTimer) {
		return ErrNilSyncTimer
	}
	if check.IfNil(args.HeaderSigVerifier) {
		return ErrNilHeaderSigVerifier
	}
	if check.IfNil(args.HeaderIntegrityVerifier) {
		return ErrNilHeaderIntegrityVerifier
	}
	if len(args.ChainID) == 0 {
		return ErrInvalidChainID
	}
	if check.IfNil(args.NetworkShardingCollector) {
		return ErrNilNetworkShardingCollector
	}
	if check.IfNil(args.AntifloodHandler) {
		return ErrNilAntifloodHandler
	}
	if check.IfNil(args.TXPool) {
		return common.ErrNilTxDataPool
	}
	if args.OnRequestTransactionTo == nil {
		return common.ErrNilRequestHandler
	}
	if check.IfNil(args.NodeRedundancyHandler) {
		return ErrNilNodeRedundancyHandler
	}

	return nil
}

func (wrk *Worker) receivedSyncState(isNodeSynchronized bool) {
	if isNodeSynchronized {
		select {
		case wrk.consensusStateChangedChannel <- true:
		default:
		}
	}
}

// ReceivedHeader process the received header, calling each received header handler registered in worker instance
func (wrk *Worker) ReceivedHeader(headerHandler data.HeaderHandler, _ []byte) {
	headerCanNotBeProcessed := headerHandler.GetSlot() != tools.SafeI64ToU64(wrk.slotManager.Index())
	if headerCanNotBeProcessed {
		return
	}

	wrk.mutReceivedHeadersHandler.RLock()
	for _, handler := range wrk.receivedHeadersHandlers {
		handler(headerHandler)
	}
	wrk.mutReceivedHeadersHandler.RUnlock()

	select {
	case wrk.consensusStateChangedChannel <- true:
	default:
	}
}

// AddReceivedHeaderHandler adds a new handler function for a received header
func (wrk *Worker) AddReceivedHeaderHandler(handler func(data.HeaderHandler)) {
	wrk.mutReceivedHeadersHandler.Lock()
	wrk.receivedHeadersHandlers = append(wrk.receivedHeadersHandlers, handler)
	wrk.mutReceivedHeadersHandler.Unlock()
}

func (wrk *Worker) initReceivedMessages() {
	wrk.mutReceivedMessages.Lock()
	wrk.receivedMessages = wrk.consensusService.InitReceivedMessages()
	wrk.mutReceivedMessages.Unlock()
}

// AddReceivedMessageCall adds a new handler function for a received messege type
func (wrk *Worker) AddReceivedMessageCall(messageType consensus.MessageType, receivedMessageCall func(cnsDta *consensus.Message) bool) {
	wrk.mutReceivedMessagesCalls.Lock()
	wrk.receivedMessagesCalls[messageType] = receivedMessageCall
	wrk.mutReceivedMessagesCalls.Unlock()
}

// RemoveAllReceivedMessagesCalls removes all the functions handlers
func (wrk *Worker) RemoveAllReceivedMessagesCalls() {
	wrk.mutReceivedMessagesCalls.Lock()
	wrk.receivedMessagesCalls = make(map[consensus.MessageType]func(*consensus.Message) bool)
	wrk.mutReceivedMessagesCalls.Unlock()
}

func (wrk *Worker) getCleanedList(cnsDataList []*consensus.Message) []*consensus.Message {
	cleanedCnsDataList := make([]*consensus.Message, 0)

	for i := 0; i < len(cnsDataList); i++ {
		if cnsDataList[i] == nil {
			continue
		}

		if wrk.slotManager.Index() > cnsDataList[i].SlotIndex {
			continue
		}

		cleanedCnsDataList = append(cleanedCnsDataList, cnsDataList[i])
	}

	return cleanedCnsDataList
}

// ProcessReceivedMessage method redirects the received message to the channel which should handle it
func (wrk *Worker) ProcessReceivedMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
	if err := wrk.validateMessage(message, fromConnectedPeer); err != nil {
		return err
	}

	nodeState := wrk.bootstrapper.GetNodeState()
	if nodeState != core.NsSynchronized { // if node is not synchronized yet, it has to continue the bootstrapping mechanism
		log.Trace("Skipping consensus message due to unsynchronized state")
		return nil
	}

	err := wrk.antifloodHandler.CanProcessMessagesOnTopic(message.Peer(), common.ConsensusTopic, 1, uint64(len(message.Data())), message.SeqNo())
	if err != nil {
		return err
	}

	cnsMsg := &consensus.Message{}

	defer func() {
		errNotCritical := wrk.checkSelfState(cnsMsg)
		if errNotCritical == nil && wrk.shouldBlacklistPeer(err) {
			//this situation is so severe that we have to black list both the message originator and the connected peer
			//that disseminated this message.
			reason := fmt.Sprintf("blacklisted due to invalid consensus message: %s", err.Error())
			wrk.antifloodHandler.BlacklistPeer(message.Peer(), reason, core.InvalidMessageBlacklistDuration)
			wrk.antifloodHandler.BlacklistPeer(fromConnectedPeer, reason, core.InvalidMessageBlacklistDuration)
		}
	}()

	err = wrk.marshalizer.Unmarshal(cnsMsg, message.Data())
	if err != nil {
		return err
	}

	if wrk.nodeRedundancyHandler.IsRedundancyNode() {
		wrk.nodeRedundancyHandler.ResetInactivityIfNeeded(
			wrk.consensusState.SelfPubKey(),
			string(cnsMsg.PubKey),
			message.Peer(),
		)
	}

	msgType := consensus.MessageType(cnsMsg.MsgType)

	log.Trace("received message from consensus topic",
		"msg type", wrk.consensusService.GetStringValue(msgType),
		"from", cnsMsg.PubKey,
		"header hash", cnsMsg.BlockHeaderHash,
		"slot", cnsMsg.SlotIndex,
		"size", len(message.Data()),
	)

	err = wrk.consensusMessageValidator.checkConsensusMessageValidity(cnsMsg, message.Peer())
	if err != nil {
		return err
	}

	if cnsMsg.SlotIndex > wrk.consensusState.SlotIndex {
		log.Trace("storing consensus message due to slot mismatch",
			"wrk.consensusState.SlotIndex", wrk.consensusState.SlotIndex,
			"cnsDta.SlotIndex", cnsMsg.SlotIndex,
		)
		wrk.storeMessage(cnsMsg)

		return nil
	}

	wrk.updateNetworkShardingVals(message, cnsMsg)
	IsMessageWithBlockHeader := wrk.consensusService.IsMessageWithBlockHeader(msgType)
	if IsMessageWithBlockHeader {
		err = wrk.doJobOnMessageWithHeader(cnsMsg)
		if err != nil {
			return err
		}
	}

	if wrk.consensusService.IsMessageWithSignature(msgType) {
		wrk.doJobOnMessageWithSignature(cnsMsg)
	}

	errNotCritical := wrk.checkSelfState(cnsMsg)
	if errNotCritical != nil {
		log.Trace("checkSelfState", "error", errNotCritical.Error())
		//in this case should return nil but do not process the message
		//nil error will mean that the interceptor will validate this message and broadcast it to the connected peers
		return nil
	}

	go wrk.executeReceivedMessages(cnsMsg)

	return nil
}

func (wrk *Worker) storeMessage(cnsDta *consensus.Message) {
	wrk.mutReceivedMessages.Lock()

	msgType := consensus.MessageType(cnsDta.MsgType)
	cnsDataList := wrk.receivedMessages[msgType]
	cnsDataList = append(cnsDataList, cnsDta)
	wrk.receivedMessages[msgType] = cnsDataList
	wrk.mutReceivedMessages.Unlock()
}

func (wrk *Worker) shouldBlacklistPeer(err error) bool {
	if err == nil ||
		errors.Is(err, ErrMessageForPastSlot) ||
		errors.Is(err, ErrMessageForFutureSlot) ||
		errors.Is(err, ErrNodeIsNotInConsensusGroup) ||
		errors.Is(err, crypto.ErrPIDMismatch) ||
		errors.Is(err, crypto.ErrSignatureMismatch) ||
		errors.Is(err, sharding.ErrEpochNodesConfigDoesNotExist) ||
		errors.Is(err, ErrMessageTypeLimitReached) {
		return false
	}

	return true
}

func (wrk *Worker) doJobOnMessageWithHeader(cnsMsg *consensus.Message) error {
	headerHash := cnsMsg.BlockHeaderHash

	header := wrk.blockProcessor.DecodeBlockHeader(cnsMsg.Header)
	isHeaderInvalid := headerHash == nil || check.IfNil(header)
	if isHeaderInvalid {
		return fmt.Errorf("%w : received header from consensus topic is invalid",
			ErrInvalidHeader)
	}

	hash, err := tools.CalculateHash(wrk.marshalizer, wrk.hasher, header.GetBlockHeader())
	if err != nil {
		return fmt.Errorf("%w : compute header hash",
			err)
	}

	if !bytes.Equal(headerHash, hash) {
		return ErrHeaderkHashNotMatch
	}

	log.Debug("received proposed block",
		"from", tools.GetTrimmedPk(hex.EncodeToString(cnsMsg.PubKey)),
		"header hash", cnsMsg.BlockHeaderHash,
		"epoch", header.GetEpoch(),
		"slot", header.GetSlot(),
		"nonce", header.GetNonce(),
		"prev hash", header.GetParentHash(),
		"nbTxs", header.GetTxCount(),
		"val trie root", header.GetValidatorsTrieRoot())

	err = wrk.headerIntegrityVerifier.Verify(header)
	if err != nil {
		return fmt.Errorf("%w : verify header integrity from consensus topic failed", err)
	}

	err = wrk.headerSigVerifier.VerifyRandSeed(header)
	if err != nil {
		return fmt.Errorf("%w : verify rand seed for received header from consensus topic failed",
			err)
	}

	wrk.processReceivedHeaderMetric(cnsMsg)

	errNotCritical := wrk.forkDetector.AddHeader(header, headerHash, process.BHProposed, nil, nil)
	if errNotCritical != nil {
		log.Debug("add received header from consensus topic to fork detector failed",
			"error", errNotCritical.Error())
		//we should not return error here because the other peers connected to self might need this message
		//to advance the consensus
	}

	// compute missing TXs
	startTime := time.Now()
	missingTXs := make([][]byte, 0)
	for _, txHash := range header.GetTxHashes() {
		err := process.CheckIfInTxPool(txHash, wrk.txPool)
		if err != nil {
			missingTXs = append(missingTXs, txHash)
		}
	}
	elapsedTime := time.Since(startTime)
	log.Debug("doJobOnMessageWithHeader.MissingTXs", "elapsedTime", elapsedTime, "total", len(header.GetTxHashes()), "missing", len(missingTXs), "reqTo", core.PeerFromBytes(cnsMsg.OriginatorPid).Pretty())

	go wrk.requestMissingTxs(missingTXs, core.PeerFromBytes(cnsMsg.OriginatorPid))

	return nil
}

func (wrk *Worker) requestMissingTxs(missingTXs [][]byte, peer core.PeerID) {
	if len(missingTXs) == 0 {
		return
	}

	// option to get TX faster from producer
	wrk.onRequestTransactionTo(missingTXs, peer)
}

func (wrk *Worker) doJobOnMessageWithSignature(cnsMsg *consensus.Message) {
	wrk.mutDisplayHashConsensusMessage.Lock()
	defer wrk.mutDisplayHashConsensusMessage.Unlock()

	hash := string(cnsMsg.BlockHeaderHash)
	wrk.mapDisplayHashConsensusMessage[hash] = append(wrk.mapDisplayHashConsensusMessage[hash], cnsMsg)
}

func (wrk *Worker) processReceivedHeaderMetric(cnsDta *consensus.Message) {
	if wrk.consensusState.ConsensusGroup() == nil || !wrk.consensusState.IsNodeLeaderInCurrentSlot(string(cnsDta.PubKey)) {
		return
	}

	sinceSlotStart := time.Since(wrk.slotManager.Timestamp())
	percent := sinceSlotStart * 100 / wrk.slotManager.TimeDuration()
	wrk.appStatusHandler.SetUInt64Value(core.MetricReceivedProposedBlock, uint64(percent)) // #nosec G115
}

func (wrk *Worker) updateNetworkShardingVals(message p2p.MessageP2P, cnsMsg *consensus.Message) {
	wrk.networkShardingCollector.UpdatePeerIDPublicKey(message.Peer(), cnsMsg.PubKey)
}

func (wrk *Worker) checkSelfState(cnsDta *consensus.Message) error {
	if wrk.consensusState.SelfPubKey() == string(cnsDta.PubKey) {
		return ErrMessageFromItself
	}

	if wrk.consensusState.SlotCanceled && wrk.consensusState.SlotIndex == cnsDta.SlotIndex {
		return ErrSlotCanceled
	}

	return nil
}

func (wrk *Worker) executeReceivedMessages(cnsDta *consensus.Message) {
	wrk.mutReceivedMessages.Lock()

	msgType := consensus.MessageType(cnsDta.MsgType)
	cnsDataList := wrk.receivedMessages[msgType]
	cnsDataList = append(cnsDataList, cnsDta)
	wrk.receivedMessages[msgType] = cnsDataList
	wrk.executeStoredMessages()

	wrk.mutReceivedMessages.Unlock()
}

func (wrk *Worker) executeStoredMessages() {
	for _, i := range wrk.consensusService.GetMessageRange() {
		cnsDataList := wrk.receivedMessages[i]
		if len(cnsDataList) == 0 {
			continue
		}
		wrk.executeMessage(cnsDataList)
		cleanedCnsDtaList := wrk.getCleanedList(cnsDataList)
		wrk.receivedMessages[i] = cleanedCnsDtaList
	}
}

func (wrk *Worker) executeMessage(cnsDtaList []*consensus.Message) {
	for i, cnsDta := range cnsDtaList {
		if cnsDta == nil {
			continue
		}
		if wrk.consensusState.SlotIndex != cnsDta.SlotIndex {
			continue
		}

		msgType := consensus.MessageType(cnsDta.MsgType)
		if !wrk.consensusService.CanProceed(wrk.consensusState, msgType) {
			log.Trace("Cant proceed with the consensus message execution with status mismatch",
				"MSG Type:", wrk.consensusService.GetStringValue(msgType),
			)
			continue
		}

		cnsDtaList[i] = nil
		wrk.executeMessageChannel <- cnsDta
	}
}

// checkChannels method is used to listen to the channels through which node receives and consumes,
// during the slot, different messages from the nodes which are in the validators group
func (wrk *Worker) checkChannels(ctx context.Context) {
	var rcvDta *consensus.Message

	for {
		timer := time.NewTimer(sleepTime)
		select {
		case <-ctx.Done():
			timer.Stop()
			log.Debug("worker's go routine is stopping...")
			return
		case rcvDta = <-wrk.executeMessageChannel:
		case <-timer.C:
			continue
		}

		msgType := consensus.MessageType(rcvDta.MsgType)
		if callReceivedMessage, exist := wrk.receivedMessagesCalls[msgType]; exist {
			if callReceivedMessage(rcvDta) {
				select {
				case wrk.consensusStateChangedChannel <- true:
				default:
				}
			}
		}
	}
}

// Extend does an extension for the subslot with subslotId
func (wrk *Worker) Extend(subslotId int) {
	wrk.consensusState.ExtendedCalled = true
	log.Debug("extend function is called",
		"subslot", wrk.consensusService.GetSubslotName(subslotId))

	wrk.DisplayStatistics()

	if wrk.consensusService.IsSubslotStartSlot(subslotId) {
		return
	}

	for wrk.consensusState.ProcessingBlock() {
		time.Sleep(time.Millisecond)
	}

	log.Debug("account state is reverted to snapshot")

	reportErr := fmt.Errorf("consensus extended")
	leader, err := wrk.consensusState.GetLeader()
	if err != nil {
		reportErr = fmt.Errorf("%s: %+w", reportErr.Error(), err)
	}

	// check if in monitor list
	if wrk.checkInMonitorList(leader) {
		_ = bugsnag.Notify(reportErr, bugsnag.MetaData{
			"consensus": {
				"leader":    hex.EncodeToString([]byte(leader)),
				"slot":      wrk.slotManager.Index(),
				"subslotId": wrk.consensusService.GetSubslotName(subslotId),
			}})
	}

	wrk.blockProcessor.RevertStateToSnapshot(wrk.consensusState.Header)

	if wrk.consensusState.Header != nil &&
		wrk.consensusState.Header.GetNonce() == wrk.forkDetector.ProbableHighestNonce() {
		wrk.forkDetector.ResetProbableHighestNonce()
	}
}

// DisplayStatistics logs the consensus messages split on proposed headers
func (wrk *Worker) DisplayStatistics() {
	wrk.mutDisplayHashConsensusMessage.Lock()
	for hash, consensusMessages := range wrk.mapDisplayHashConsensusMessage {
		leader, err := wrk.consensusState.GetLeader()

		if len(consensusMessages) == 0 {
			log.Error("consensusMessages DisplayStatistics", "len", len(consensusMessages))
			reportErr := fmt.Errorf("consensusMessages DisplayStatistics no messages")
			if err != nil {
				reportErr = fmt.Errorf("%s: %+w", reportErr.Error(), err)
			}

			_ = bugsnag.Notify(reportErr, bugsnag.MetaData{
				"consensus": {
					"sigsNum": len(consensusMessages),
					"slot":    wrk.slotManager.Index(),
					"leader":  hex.EncodeToString([]byte(leader)),
				}})
			continue
		}

		log.Debug("proposed header with signatures",
			"hash", []byte(hash),
			"sigs num", len(consensusMessages),
			"slot", consensusMessages[0].SlotIndex,
		)

		// report if less then consensus size without leader
		if len(consensusMessages) < (wrk.consensusState.consensusGroupSize - 1) {
			reportErr := fmt.Errorf("small consensus quorum")
			if err != nil {
				reportErr = fmt.Errorf("%s: %+w", reportErr.Error(), err)
			}

			log.Warn("worker statistics",
				"sigsNum", len(consensusMessages),
				"slot", wrk.slotManager.Index(),
				"leader", hex.EncodeToString([]byte(leader)),
				"nonce", consensusMessages[0].Nonce,
				"info", reportErr.Error(),
			)

			wrk.reportValidatorFail(leader, consensusMessages)
		}

		for _, consensusMessage := range consensusMessages {
			log.Trace(tools.GetTrimmedPk(hex.EncodeToString(consensusMessage.PubKey)))
		}

	}

	wrk.mapDisplayHashConsensusMessage = make(map[string][]*consensus.Message)

	wrk.mutDisplayHashConsensusMessage.Unlock()
}

func (wrk *Worker) checkInConsensusMessages(validator []byte, consensusMessages []*consensus.Message) bool {
	for _, cm := range consensusMessages {
		if bytes.Equal(validator, cm.PubKey) {
			return true
		}
	}
	return false
}

func (wrk *Worker) checkInMonitorList(validator string) bool {
	for _, ml := range wrk.consensusMonitorList {
		if validator == ml {
			return true
		}
	}

	return false
}

func (wrk *Worker) reportValidatorFail(leader string, consensusMessages []*consensus.Message) {
	// only compile data if node has validator report fail list
	if len(wrk.consensusMonitorList) == 0 {
		return
	}

	consensusGroup := make([]string, 0)
	failList := make([]string, 0)
	for _, validator := range wrk.consensusState.consensusGroup {
		// consensusGroup handles string cast of PubKey bytes, convert to hex for easy read on bugsnag
		validatorHex := hex.EncodeToString([]byte(validator))
		consensusGroup = append(consensusGroup, validatorHex)

		// check if in monitor list
		if !wrk.checkInMonitorList(validator) {
			continue
		}

		// if validator is leader it will not be in consensus message
		if validator == leader ||
			wrk.checkInConsensusMessages([]byte(validator), consensusMessages) {
			continue
		}
		failList = append(failList, validatorHex)
	}

	if len(failList) > 0 {
		_ = bugsnag.Notify(fmt.Errorf("small consensus quorum"), bugsnag.MetaData{
			"consensus": {
				"sigsNum":   len(consensusMessages),
				"leader":    hex.EncodeToString([]byte(leader)),
				"consensus": consensusGroup,
				"failed":    failList,
				"slot":      wrk.slotManager.Index(),
				"nonce":     consensusMessages[0].Nonce,
			}})
	}
}

// GetConsensusStateChangedChannel gets the channel for the consensusStateChanged
func (wrk *Worker) GetConsensusStateChangedChannel() chan bool {
	return wrk.consensusStateChangedChannel
}

// ExecuteStoredMessages tries to execute all the messages received which are valid for execution
func (wrk *Worker) ExecuteStoredMessages() {
	wrk.mutReceivedMessages.Lock()
	wrk.executeStoredMessages()
	wrk.mutReceivedMessages.Unlock()
}

// SetAppStatusHandler sets the status metric handler
func (wrk *Worker) SetAppStatusHandler(ash core.AppStatusHandler) error {
	if check.IfNil(ash) {
		return ErrNilAppStatusHandler
	}
	wrk.appStatusHandler = ash

	return nil
}

// Close will close the endless running go routine
func (wrk *Worker) Close() error {
	if wrk.cancelFunc != nil {
		wrk.cancelFunc()
	}

	return nil
}

// ResetConsensusMessages resets at the start of each slot all the previous consensus messages received
func (wrk *Worker) ResetConsensusMessages() {
	wrk.consensusMessageValidator.resetConsensusMessages()
}

// IsInterfaceNil returns true if there is no value under the interface
func (wrk *Worker) IsInterfaceNil() bool {
	return wrk == nil
}

// ValidateMessage validates the received message
func (wrk *Worker) validateMessage(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
	if check.IfNil(message) {
		return ErrNilMessage
	}
	if message.Data() == nil {
		return ErrNilDataToProcess
	}

	// early check to prevent process messages from untrusted peers
	if err := wrk.antifloodHandler.CanProcessMessage(message, fromConnectedPeer); err != nil {
		return err
	}

	return nil
}
