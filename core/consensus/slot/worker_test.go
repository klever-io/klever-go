package slot_test

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/consensus/slot/bls"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fromConnectedPeerId = core.PeerID("connected peer id")

const HashSize = 32
const SignatureSize = 48
const PublicKeySize = 96

var blockHeaderHash = make([]byte, HashSize)
var invalidBlockHeaderHash = make([]byte, HashSize+1)
var signature = make([]byte, SignatureSize)
var invalidSignature = make([]byte, SignatureSize+1)
var publicKey = make([]byte, PublicKeySize)

func createDefaultWorkerArgs() *slot.WorkerArgs {
	blockchainMock := &cMock.BlockChainMock{}
	blockProcessor := &mock.BlockProcessorMock{
		DecodeBlockHeaderCalled: func(dta []byte) data.HeaderHandler {
			return &cMock.HeaderHandlerStub{
				CheckChainIDCalled: func(reference []byte) error {
					return nil
				},
				GetParentHashCalled: func() []byte {
					return make([]byte, 0)
				},
			}
		},
		RevertStateToSnapshotCalled: func(header data.HeaderHandler) {
		},
	}
	bootstrapperMock := &mock.BootstrapperMock{}
	broadcastMessengerMock := &mock.BroadcastMessengerMock{}
	consensusState := initConsensusState()
	forkDetectorMock := &cMock.ForkDetectorMock{}
	forkDetectorMock.AddHeaderCalled = func(header data.HeaderHandler, hash []byte, state process.BlockHeaderState, selfNotarizedHeaders []data.HeaderHandler, selfNotarizedHeadersHashes [][]byte) error {
		return nil
	}
	keyGeneratorMock, _, _ := mock.InitKeys()
	marshalizerMock := mock.MarshalizerMock{}
	slotManagerMock := initSlotManagerMock()
	singleSignerMock := &cMock.SingleSignerMock{
		SignStub: func(private crypto.PrivateKey, msg []byte) ([]byte, error) {
			return []byte("signed"), nil
		},
		VerifyStub: func(public crypto.PublicKey, msg []byte, sig []byte) error {
			return nil
		},
	}
	syncTimerMock := &mock.SyncTimerMock{}
	hasher := &cMock.HasherMock{}
	blsService, _ := bls.NewConsensusService()
	txPool := cMock.NewPoolsHolderMock()
	reqHandler := &cMock.RequestHandlerStub{}

	peerSigHandler := &cMock.PeerSignatureHandler{Signer: singleSignerMock, KeyGen: keyGeneratorMock}
	workerArgs := &slot.WorkerArgs{
		ConsensusService:         blsService,
		BlockChain:               blockchainMock,
		BlockProcessor:           blockProcessor,
		Bootstrapper:             bootstrapperMock,
		BroadcastMessenger:       broadcastMessengerMock,
		ConsensusState:           consensusState,
		ForkDetector:             forkDetectorMock,
		Marshalizer:              marshalizerMock,
		Hasher:                   hasher,
		SlotManager:              slotManagerMock,
		PeerSignatureHandler:     peerSigHandler,
		SyncTimer:                syncTimerMock,
		HeaderSigVerifier:        &mock.HeaderSigVerifierStub{},
		HeaderIntegrityVerifier:  &mock.HeaderIntegrityVerifierStub{},
		ChainID:                  chainID,
		NetworkShardingCollector: createMockNetworkShardingCollector(),
		AntifloodHandler:         createMockP2PAntifloodHandler(),
		TXPool:                   txPool.Transactions(),
		OnRequestTransactionTo:   reqHandler.RequestTransactionTo,
		// PoolAdder:                headerPool,
		SignatureSize:         SignatureSize,
		PublicKeySize:         PublicKeySize,
		NodeRedundancyHandler: &mock.NodeRedundancyHandlerStub{},
	}

	return workerArgs
}

func createMockNetworkShardingCollector() *mock.NetworkShardingCollectorStub {
	return &mock.NetworkShardingCollectorStub{
		UpdatePeerIDPublicKeyCalled: func(pid core.PeerID, pk []byte) {},
	}
}

func createMockP2PAntifloodHandler() *cMock.P2PAntifloodHandlerStub {
	return &cMock.P2PAntifloodHandlerStub{
		CanProcessMessageCalled: func(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
			return nil
		},
		CanProcessMessagesOnTopicCalled: func(peer core.PeerID, topic string, numMessages uint32, totalSize uint64, sequence []byte) error {
			return nil
		},
	}
}

func initWorker() *slot.Worker {
	workerArgs := createDefaultWorkerArgs()
	sposWorker, _ := slot.NewWorker(workerArgs)

	return sposWorker
}

func initSlotManagerMock() *mock.SlotManagerMock {
	return &mock.SlotManagerMock{
		SlotIndex: 0,
		TimestampCalled: func() time.Time {
			return time.Unix(0, 0)
		},
		TimeDurationCalled: func() time.Duration {
			return slotTimeDuration
		},
	}
}

func TestWorker_NewWorkerConsensusServiceNilShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.ConsensusService = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilConsensusService, err)
}

func TestWorker_NewWorkerBlockChainNilShouldFail(t *testing.T) {
	t.Parallel()
	workerArgs := createDefaultWorkerArgs()
	workerArgs.BlockChain = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilBlockChain, err)
}

func TestWorker_NewWorkerBlockProcessorNilShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.BlockProcessor = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilBlockProcessor, err)
}

func TestWorker_NewWorkerBootstrapperNilShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.Bootstrapper = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilBootstrapper, err)
}

func TestWorker_NewWorkerBroadcastMessengerNilShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.BroadcastMessenger = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilBroadcastMessenger, err)
}

func TestWorker_NewWorkerConsensusStateNilShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.ConsensusState = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilConsensusState, err)
}

func TestWorker_NewWorkerForkDetectorNilShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.ForkDetector = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilForkDetector, err)
}

func TestWorker_NewWorkerMarshalizerNilShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.Marshalizer = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilMarshalizer, err)
}

func TestWorker_NewWorkerHasherNilShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.Hasher = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilHasher, err)
}

func TestWorker_NewWorkerSlotManagerNilShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.SlotManager = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilSlotManager, err)
}

func TestWorker_NewWorkerPeerSignatureHandlerNilShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.PeerSignatureHandler = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilPeerSignatureHandler, err)
}

func TestWorker_NewWorkerSyncTimerNilShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.SyncTimer = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilSyncTimer, err)
}

func TestWorker_NewWorkerHeaderSigVerifierNilShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.HeaderSigVerifier = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilHeaderSigVerifier, err)
}

func TestWorker_NewWorkerHeaderIntegrityVerifierShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.HeaderIntegrityVerifier = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilHeaderIntegrityVerifier, err)
}

func TestWorker_NewWorkerEmptyChainIDShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.ChainID = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrInvalidChainID, err)
}

func TestWorker_NewWorkerNilNetworkShardingCollectorShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.NetworkShardingCollector = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilNetworkShardingCollector, err)
}

func TestWorker_NewWorkerNilAntifloodHandlerShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.AntifloodHandler = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilAntifloodHandler, err)
}

func TestWorker_NewWorkerNodeRedundancyHandlerShouldFail(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	workerArgs.NodeRedundancyHandler = nil
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, wrk)
	assert.Equal(t, slot.ErrNilNodeRedundancyHandler, err)
}

func TestWorker_NewWorkerShouldWork(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	wrk, err := slot.NewWorker(workerArgs)

	assert.Nil(t, err)
	assert.False(t, check.IfNil(wrk))
}

func TestWorker_ProcessReceivedMessageShouldErrIfFloodIsDetectedOnTopic(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("flood detected")
	workerArgs := createDefaultWorkerArgs()
	antifloodHandler := &cMock.P2PAntifloodHandlerStub{
		CanProcessMessageCalled: func(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
			return nil
		},
		CanProcessMessagesOnTopicCalled: func(peer core.PeerID, topic string, numMessages uint32, totalSize uint64, sequence []byte) error {
			return expectedErr
		},
	}

	workerArgs.AntifloodHandler = antifloodHandler
	wrk, _ := slot.NewWorker(workerArgs)

	msg := &cMock.P2PMessageMock{DataField: []byte("aaa"), TopicField: "topic1"}
	err := wrk.ProcessReceivedMessage(msg, "peer")
	assert.Equal(t, expectedErr, err)
}

func TestWorker_ReceivedSyncStateShouldNotSendOnChannelWhenInputIsFalse(t *testing.T) {
	t.Parallel()
	wrk := initWorker()
	wrk.ReceivedSyncState(false)
	rcv := false
	select {
	case rcv = <-wrk.ConsensusStateChangedChannel():
	case <-time.After(100 * time.Millisecond):
	}

	assert.False(t, rcv)
}

func TestWorker_ReceivedSyncStateShouldNotSendOnChannelWhenChannelIsBusy(t *testing.T) {
	t.Parallel()
	wrk := initWorker()
	wrk.ConsensusStateChangedChannel() <- false
	wrk.ReceivedSyncState(true)
	rcv := false
	select {
	case rcv = <-wrk.ConsensusStateChangedChannel():
	case <-time.After(100 * time.Millisecond):
	}

	assert.False(t, rcv)
}

func TestWorker_ReceivedSyncStateShouldSendOnChannel(t *testing.T) {
	t.Parallel()
	wrk := initWorker()
	wrk.ReceivedSyncState(true)
	rcv := false
	select {
	case rcv = <-wrk.ConsensusStateChangedChannel():
	case <-time.After(100 * time.Millisecond):
	}

	assert.True(t, rcv)
}

func TestWorker_InitReceivedMessagesShouldInitMap(t *testing.T) {
	t.Parallel()
	wrk := initWorker()
	wrk.NilReceivedMessages()
	wrk.InitReceivedMessages()

	assert.NotNil(t, wrk.ReceivedMessages()[bls.MtBlockHeader])
}

func TestWorker_AddReceivedMessageCallShouldWork(t *testing.T) {
	t.Parallel()
	wrk := initWorker()
	receivedMessageCall := func(*consensus.Message) bool {
		return true
	}
	wrk.AddReceivedMessageCall(bls.MtBlockHeader, receivedMessageCall)
	receivedMessageCalls := wrk.ReceivedMessagesCalls()

	assert.Equal(t, 1, len(receivedMessageCalls))
	assert.NotNil(t, receivedMessageCalls[bls.MtBlockHeader])
	assert.True(t, receivedMessageCalls[bls.MtBlockHeader](nil))
}

func TestWorker_RemoveAllReceivedMessageCallsShouldWork(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	receivedMessageCall := func(*consensus.Message) bool {
		return true
	}
	wrk.AddReceivedMessageCall(bls.MtBlockHeader, receivedMessageCall)
	receivedMessageCalls := wrk.ReceivedMessagesCalls()

	assert.Equal(t, 1, len(receivedMessageCalls))
	assert.NotNil(t, receivedMessageCalls[bls.MtBlockHeader])
	assert.True(t, receivedMessageCalls[bls.MtBlockHeader](nil))

	wrk.RemoveAllReceivedMessagesCalls()
	receivedMessageCalls = wrk.ReceivedMessagesCalls()

	assert.Equal(t, 0, len(receivedMessageCalls))
	assert.Nil(t, receivedMessageCalls[bls.MtBlockHeader])
}

func TestWorker_ProcessReceivedMessageTxBlockHeaderShouldRetNil(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	hdr := &block.Block{Header: &block.BlockHeader{ChainID: chainID}}
	hdrHash, _ := tools.CalculateHash(mock.MarshalizerMock{}, cMock.HasherMock{}, hdr.GetBlockHeader())
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(hdr)
	wrk.NewBlockProcessor(hdr.GetBlockHeader())
	cnsMsg := consensus.NewConsensusMessage(
		hdrHash,
		nil,
		hdrStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		signature,
		int(bls.MtBlockHeader),
		0, 0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	time.Sleep(time.Second)
	msg := &cMock.P2PMessageMock{
		DataField: buff,
		PeerField: currentPid,
	}
	err := wrk.ProcessReceivedMessage(msg, fromConnectedPeerId)

	assert.Nil(t, err)
}

func TestWorker_ProcessReceivedMessageNilMessageShouldErr(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	err := wrk.ProcessReceivedMessage(nil, fromConnectedPeerId)
	time.Sleep(time.Second)

	assert.Equal(t, 0, len(wrk.ReceivedMessages()[bls.MtBlockHeader]))
	assert.Equal(t, slot.ErrNilMessage, err)
}

func TestWorker_ProcessReceivedMessageNilMessageDataFieldShouldErr(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	err := wrk.ProcessReceivedMessage(&cMock.P2PMessageMock{}, fromConnectedPeerId)
	time.Sleep(time.Second)

	assert.Equal(t, 0, len(wrk.ReceivedMessages()[bls.MtBlockHeader]))
	assert.Equal(t, slot.ErrNilDataToProcess, err)
}

func TestWorker_ProcessReceivedMessageRedundancyNodeShouldResetInactivityIfNeeded(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	var wasCalled bool
	nodeRedundancyMock := &mock.NodeRedundancyHandlerStub{
		IsRedundancyNodeCalled: func() bool {
			return true
		},
		ResetInactivityIfNeededCalled: func(selfPubKey string, consensusMsgPubKey string, consensusMsgPeerID core.PeerID) {
			wasCalled = true
		},
	}
	wrk.SetNodeRedundancyHandler(nodeRedundancyMock)
	buff, _ := wrk.Marshalizer().Marshal(&consensus.Message{})
	_ = wrk.ProcessReceivedMessage(&cMock.P2PMessageMock{DataField: buff}, fromConnectedPeerId)

	assert.True(t, wasCalled)
}

func TestWorker_ProcessReceivedMessageNodeNotInConsensusGroupShouldErr(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	hdr := &block.Block{Header: &block.BlockHeader{ChainID: chainID}}
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(hdr)
	hdrHash := cMock.HasherMock{}.Compute(string(hdrStr))

	cnsMsg := consensus.NewConsensusMessage(
		hdrHash,
		nil,
		hdrStr,
		publicKey,
		signature,
		int(bls.MtBlockHeader),
		0, 0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	err := wrk.ProcessReceivedMessage(&cMock.P2PMessageMock{DataField: buff}, fromConnectedPeerId)
	time.Sleep(time.Second)

	assert.Equal(t, 0, len(wrk.ReceivedMessages()[bls.MtBlockHeader]))
	assert.True(t, errors.Is(err, slot.ErrNodeIsNotInConsensusGroup))
}

func TestWorker_ProcessReceivedMessageComputeReceivedProposedBlockMetric(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	wrk.SetBlockProcessor(&mock.BlockProcessorMock{
		DecodeBlockHeaderCalled: func(dta []byte) data.HeaderHandler {
			return &block.Block{
				Header: &block.BlockHeader{
					ChainID:         chainID,
					SoftwareVersion: []byte("version"),
				},
			}
		},
		RevertStateToSnapshotCalled: func(header data.HeaderHandler) {
		},
	})
	slotDuration := time.Millisecond * 1000
	delay := time.Millisecond * 430
	slotStartTimestamp := time.Now()
	wrk.SetSlotManager(&mock.SlotManagerMock{
		SlotIndex: 0,
		TimeDurationCalled: func() time.Duration {
			return slotDuration
		},
		TimestampCalled: func() time.Time {
			return slotStartTimestamp
		},
	})
	hdr := &block.Block{Header: &block.BlockHeader{ChainID: chainID}}
	hdrHash, _ := tools.CalculateHash(mock.MarshalizerMock{}, cMock.HasherMock{}, hdr.GetBlockHeader())
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(hdr)
	wrk.NewBlockProcessor(hdr.GetBlockHeader())
	cnsMsg := consensus.NewConsensusMessage(
		hdrHash,
		nil,
		hdrStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		signature,
		int(bls.MtBlockHeader),
		0,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	receivedValue := uint64(0)
	_ = wrk.SetAppStatusHandler(&cMock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {
			receivedValue = value
		},
	})

	time.Sleep(delay)

	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	msg := &cMock.P2PMessageMock{
		DataField: buff,
		PeerField: currentPid,
	}
	_ = wrk.ProcessReceivedMessage(msg, "")

	minimumExpectedValue := uint64(delay * 100 / slotDuration)
	assert.True(t,
		receivedValue >= minimumExpectedValue,
		fmt.Sprintf("minimum expected was %d, got %d", minimumExpectedValue, receivedValue),
	)
}

func TestWorker_ProcessReceivedMessageInconsistentChainIDInConsensusMessageShouldErr(t *testing.T) {
	t.Parallel()

	wrk := *initWorker()
	blk := &block.Block{Header: &block.BlockHeader{}}
	blkStr, _ := mock.MarshalizerMock{}.Marshal(blk)
	cnsMsg := consensus.NewConsensusMessage(
		blockHeaderHash,
		nil,
		blkStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtBlockHeader),
		1,
		1,
		[]byte("inconsistent chain ID"),
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	err := wrk.ProcessReceivedMessage(&cMock.P2PMessageMock{DataField: buff}, fromConnectedPeerId)

	assert.True(t, errors.Is(err, slot.ErrInvalidChainID))
}

func TestWorker_ProcessReceivedMessageTypeInvalidShouldErr(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	blk := &block.Block{Header: &block.BlockHeader{}}
	blkStr, _ := mock.MarshalizerMock{}.Marshal(blk)
	cnsMsg := consensus.NewConsensusMessage(
		blockHeaderHash,
		nil,
		blkStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		[]byte("sig"),
		666,
		0,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	err := wrk.ProcessReceivedMessage(&cMock.P2PMessageMock{DataField: buff}, fromConnectedPeerId)
	time.Sleep(time.Second)

	assert.Equal(t, 0, len(wrk.ReceivedMessages()[666]))
	assert.True(t, errors.Is(err, slot.ErrInvalidMessageType), err)
}

func TestWorker_ProcessReceivedHeaderHashSizeInvalidShouldErr(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	blk := &block.Block{Header: &block.BlockHeader{}}
	blkStr, _ := mock.MarshalizerMock{}.Marshal(blk)
	cnsMsg := consensus.NewConsensusMessage(
		invalidBlockHeaderHash,
		nil,
		blkStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtBlockHeader),
		0, 0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	err := wrk.ProcessReceivedMessage(&cMock.P2PMessageMock{DataField: buff}, fromConnectedPeerId)
	time.Sleep(time.Second)

	assert.Equal(t, 0, len(wrk.ReceivedMessages()[bls.MtBlockHeader]))
	assert.True(t, errors.Is(err, slot.ErrInvalidHeaderHashSize), err)
}

func TestWorker_ProcessReceivedMessageForFutureSlotShouldErr(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	hdr := &block.Block{Header: &block.BlockHeader{ChainID: chainID}}
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(hdr)
	hdrHash := cMock.HasherMock{}.Compute(string(hdrStr))

	cnsMsg := consensus.NewConsensusMessage(
		hdrHash,
		nil,
		hdrStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		signature,
		int(bls.MtBlockHeader),
		2,
		2,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	err := wrk.ProcessReceivedMessage(&cMock.P2PMessageMock{DataField: buff}, fromConnectedPeerId)
	time.Sleep(time.Second)

	assert.Equal(t, 0, len(wrk.ReceivedMessages()[bls.MtBlockHeader]))
	assert.True(t, errors.Is(err, slot.ErrMessageForFutureSlot))
}

func TestWorker_ProcessReceivedMessageForPastSlotShouldErr(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	hdr := &block.Block{Header: &block.BlockHeader{ChainID: chainID}}
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(hdr)
	hdrHash := cMock.HasherMock{}.Compute(string(hdrStr))

	cnsMsg := consensus.NewConsensusMessage(
		hdrHash,
		nil,
		hdrStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		signature,
		int(bls.MtBlockHeader),
		-1,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	err := wrk.ProcessReceivedMessage(&cMock.P2PMessageMock{DataField: buff}, fromConnectedPeerId)
	time.Sleep(time.Second)

	assert.Equal(t, 0, len(wrk.ReceivedMessages()[bls.MtBlockHeader]))
	assert.True(t, errors.Is(err, slot.ErrMessageForPastSlot))
}

func TestWorker_ProcessReceivedMessageTypeLimitReachedShouldErr(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	blk := &block.Block{Header: &block.BlockHeader{}}
	hdrHash, _ := tools.CalculateHash(mock.MarshalizerMock{}, cMock.HasherMock{}, blk.GetBlockHeader())
	blkStr, _ := mock.MarshalizerMock{}.Marshal(blk)
	wrk.NewBlockProcessor(blk.GetBlockHeader())
	cnsMsg := consensus.NewConsensusMessage(
		hdrHash,
		nil,
		blkStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		signature,
		int(bls.MtBlockHeader),
		0, 0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	msg := &cMock.P2PMessageMock{
		DataField: buff,
		PeerField: currentPid,
	}

	err := wrk.ProcessReceivedMessage(msg, fromConnectedPeerId)
	time.Sleep(time.Second)
	assert.Equal(t, 1, len(wrk.ReceivedMessages()[bls.MtBlockHeader]))
	assert.Nil(t, err)

	err = wrk.ProcessReceivedMessage(&cMock.P2PMessageMock{DataField: buff}, fromConnectedPeerId)
	time.Sleep(time.Second)
	assert.Equal(t, 1, len(wrk.ReceivedMessages()[bls.MtBlockHeader]))
	assert.True(t, errors.Is(err, slot.ErrMessageTypeLimitReached))
}

func TestWorker_ProcessReceivedMessageInvalidSignatureShouldErr(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	hdr := &block.Block{Header: &block.BlockHeader{ChainID: chainID}}
	hdrHash, _ := tools.CalculateHash(mock.MarshalizerMock{}, cMock.HasherMock{}, hdr.GetBlockHeader())
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(hdr)

	cnsMsg := consensus.NewConsensusMessage(
		hdrHash,
		nil,
		hdrStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		invalidSignature,
		int(bls.MtBlockHeader),
		0, 0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	err := wrk.ProcessReceivedMessage(&cMock.P2PMessageMock{DataField: buff}, fromConnectedPeerId)
	time.Sleep(time.Second)

	assert.Equal(t, 0, len(wrk.ReceivedMessages()[bls.MtBlockHeader]))
	assert.True(t, errors.Is(err, slot.ErrInvalidSignatureSize))
}

func TestWorker_ProcessReceivedMessageReceivedMessageIsFromSelfShouldRetNilAndNotProcess(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	hdr := &block.Block{Header: &block.BlockHeader{ChainID: chainID}}
	hdrHash, _ := tools.CalculateHash(mock.MarshalizerMock{}, cMock.HasherMock{}, hdr.GetBlockHeader())
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(hdr)
	wrk.NewBlockProcessor(hdr.GetBlockHeader())

	cnsMsg := consensus.NewConsensusMessage(
		hdrHash,
		nil,
		hdrStr,
		[]byte(wrk.ConsensusState().SelfPubKey()),
		signature,
		int(bls.MtBlockHeader),
		0, 0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	msg := &cMock.P2PMessageMock{
		DataField: buff,
		PeerField: currentPid,
	}
	err := wrk.ProcessReceivedMessage(msg, fromConnectedPeerId)
	time.Sleep(time.Second)

	assert.Equal(t, 0, len(wrk.ReceivedMessages()[bls.MtBlockHeader]))
	assert.Nil(t, err)
}

func TestWorker_ProcessReceivedMessageWhenSlotIsCanceledShouldRetNilAndNotProcess(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	wrk.ConsensusState().SlotCanceled = true
	hdr := &block.Block{Header: &block.BlockHeader{ChainID: chainID}}
	wrk.NewBlockProcessor(hdr.GetBlockHeader())
	hdrHash, _ := tools.CalculateHash(mock.MarshalizerMock{}, cMock.HasherMock{}, hdr.GetBlockHeader())
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(hdr)

	cnsMsg := consensus.NewConsensusMessage(
		hdrHash,
		nil,
		hdrStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		signature,
		int(bls.MtBlockHeader),
		0, 0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	msg := &cMock.P2PMessageMock{
		DataField: buff,
		PeerField: currentPid,
	}
	err := wrk.ProcessReceivedMessage(msg, fromConnectedPeerId)
	time.Sleep(time.Second)

	assert.Equal(t, 0, len(wrk.ReceivedMessages()[bls.MtBlockHeader]))
	assert.Nil(t, err)
}

func TestWorker_ProcessReceivedMessageWrongChainIDInProposedBlockShouldError(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	wrk.SetBlockProcessor(
		&mock.BlockProcessorMock{
			DecodeBlockHeaderCalled: func(dta []byte) data.HeaderHandler {
				return &cMock.HeaderHandlerStub{
					CheckChainIDCalled: func(reference []byte) error {
						return slot.ErrInvalidChainID
					},
					GetParentHashCalled: func() []byte {
						return make([]byte, 0)
					},
				}
			},
			RevertStateToSnapshotCalled: func(header data.HeaderHandler) {
			},
		},
	)

	hdr := &block.Block{Header: &block.BlockHeader{ChainID: wrongChainID}}
	hdrHash, _ := tools.CalculateHash(mock.MarshalizerMock{}, cMock.HasherMock{}, hdr.GetBlockHeader())
	cnsMsg := consensus.NewConsensusMessage(
		hdrHash,
		nil,
		nil,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtBlockHeader),
		0,
		0,
		wrongChainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	err := wrk.ProcessReceivedMessage(&cMock.P2PMessageMock{DataField: buff}, fromConnectedPeerId)
	time.Sleep(time.Second)

	assert.True(t, errors.Is(err, slot.ErrInvalidChainID))
}

func TestWorker_ProcessReceivedMessageWithABadOriginatorShouldErr(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	wrk.SetBlockProcessor(
		&mock.BlockProcessorMock{
			DecodeBlockHeaderCalled: func(dta []byte) data.HeaderHandler {
				return &cMock.HeaderHandlerStub{
					CheckChainIDCalled: func(reference []byte) error {
						return nil
					},
					GetParentHashCalled: func() []byte {
						return make([]byte, 0)
					},
				}
			},
			RevertStateToSnapshotCalled: func(header data.HeaderHandler) {
			},
		},
	)

	hdr := &block.Block{Header: &block.BlockHeader{ChainID: chainID}}
	hdrHash, _ := tools.CalculateHash(mock.MarshalizerMock{}, cMock.HasherMock{}, hdr.GetBlockHeader())
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(hdr)
	cnsMsg := consensus.NewConsensusMessage(
		hdrHash,
		nil,
		hdrStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		signature,
		int(bls.MtBlockHeader),
		0, 0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	msg := &cMock.P2PMessageMock{
		DataField: buff,
		PeerField: "other originator",
	}
	err := wrk.ProcessReceivedMessage(msg, fromConnectedPeerId)
	time.Sleep(time.Second)

	assert.Equal(t, 0, len(wrk.ReceivedMessages()[bls.MtBlockHeader]))
	assert.True(t, errors.Is(err, slot.ErrOriginatorMismatch))
}

func TestWorker_ProcessReceivedMessageOkValsShouldWork(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	hdr := &block.Block{Header: &block.BlockHeader{ChainID: chainID}}

	wrk.SetBlockProcessor(
		&mock.BlockProcessorMock{
			DecodeBlockHeaderCalled: func(dta []byte) data.HeaderHandler {
				return &cMock.HeaderHandlerStub{
					CheckChainIDCalled: func(reference []byte) error {
						return nil
					},
					GetParentHashCalled: func() []byte {
						return make([]byte, 0)
					},
					GetBlockHeaderCalled: func() interface{} {
						return hdr.Header
					},
				}
			},
			RevertStateToSnapshotCalled: func(header data.HeaderHandler) {
			},
		},
	)

	hdrHash, _ := tools.CalculateHash(mock.MarshalizerMock{}, cMock.HasherMock{}, hdr.GetBlockHeader())
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(hdr)
	cnsMsg := consensus.NewConsensusMessage(
		hdrHash,
		nil,
		hdrStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		signature,
		int(bls.MtBlockHeader),
		0, 0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	msg := &cMock.P2PMessageMock{
		DataField: buff,
		PeerField: currentPid,
	}
	err := wrk.ProcessReceivedMessage(msg, fromConnectedPeerId)
	time.Sleep(time.Second)

	assert.Equal(t, 1, len(wrk.ReceivedMessages()[bls.MtBlockHeader]))
	assert.Nil(t, err)
}

func TestWorker_CheckSelfStateShouldErrMessageFromItself(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		nil,
		[]byte(wrk.ConsensusState().SelfPubKey()),
		nil,
		0,
		0,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	err := wrk.CheckSelfState(cnsMsg)
	assert.Equal(t, slot.ErrMessageFromItself, err)
}

func TestWorker_CheckSelfStateShouldErrSlotCanceled(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	wrk.ConsensusState().SlotCanceled = true
	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		nil,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		nil,
		0,
		0,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	err := wrk.CheckSelfState(cnsMsg)
	assert.Equal(t, slot.ErrSlotCanceled, err)
}

func TestWorker_CheckSelfStateShouldNotErr(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		nil,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		nil,
		0,
		0,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	err := wrk.CheckSelfState(cnsMsg)
	assert.Nil(t, err)
}

func TestWorker_CheckSignatureShouldReturnNilErr(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	blk := &block.Block{Header: &block.BlockHeader{}}
	blkStr, _ := mock.MarshalizerMock{}.Marshal(blk)
	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		blkStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtBlockHeader),
		0, 0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	err := wrk.CheckSignature(cnsMsg)

	assert.Nil(t, err)
}

func TestWorker_ExecuteMessagesShouldNotExecuteWhenConsensusDataIsNil(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	blk := &block.Block{Header: &block.BlockHeader{}}
	blkStr, _ := mock.MarshalizerMock{}.Marshal(blk)
	wrk.InitReceivedMessages()
	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		blkStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtBlockHeader),
		0, 0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	msgType := consensus.MessageType(cnsMsg.MsgType)
	cnsDataList := wrk.ReceivedMessages()[msgType]
	cnsDataList = append(cnsDataList, nil)
	wrk.SetReceivedMessages(msgType, cnsDataList)
	wrk.ExecuteMessage(cnsDataList)

	assert.Nil(t, wrk.ReceivedMessages()[msgType][0])
}

func TestWorker_ExecuteMessagesShouldNotExecuteWhenMessageIsForOtherSlot(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	blk := &block.Block{Header: &block.BlockHeader{}}
	blkStr, _ := mock.MarshalizerMock{}.Marshal(blk)
	wrk.InitReceivedMessages()
	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		blkStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtBlockHeader),
		-1,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	msgType := consensus.MessageType(cnsMsg.MsgType)
	cnsDataList := wrk.ReceivedMessages()[msgType]
	cnsDataList = append(cnsDataList, cnsMsg)
	wrk.SetReceivedMessages(msgType, cnsDataList)
	wrk.ExecuteMessage(cnsDataList)

	assert.NotNil(t, wrk.ReceivedMessages()[msgType][0])
}

func TestWorker_ExecuteSignatureMessagesShouldNotExecuteWhenBlockIsNotFinished(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	blk := &block.Block{Header: &block.BlockHeader{}}
	blkStr, _ := mock.MarshalizerMock{}.Marshal(blk)
	wrk.InitReceivedMessages()
	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		blkStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtSignature),
		0,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	msgType := consensus.MessageType(cnsMsg.MsgType)
	cnsDataList := wrk.ReceivedMessages()[msgType]
	cnsDataList = append(cnsDataList, cnsMsg)
	wrk.SetReceivedMessages(msgType, cnsDataList)
	wrk.ExecuteMessage(cnsDataList)

	assert.NotNil(t, wrk.ReceivedMessages()[msgType][0])
}

func TestWorker_ExecuteMessagesShouldExecute(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	wrk.StartWorking()
	blk := &block.Block{Header: &block.BlockHeader{}}
	blkStr, _ := mock.MarshalizerMock{}.Marshal(blk)
	wrk.InitReceivedMessages()
	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		blkStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtBlockHeader),
		0, 0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	msgType := consensus.MessageType(cnsMsg.MsgType)
	cnsDataList := wrk.ReceivedMessages()[msgType]
	cnsDataList = append(cnsDataList, cnsMsg)
	wrk.SetReceivedMessages(msgType, cnsDataList)
	wrk.ConsensusState().SetStatus(bls.SrStartSlot, slot.SsFinished)
	wrk.ExecuteMessage(cnsDataList)

	assert.Nil(t, wrk.ReceivedMessages()[msgType][0])

	_ = wrk.Close()
}

func TestWorker_CheckChannelsShouldWork(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	wrk.StartWorking()
	wrk.SetReceivedMessagesCalls(bls.MtBlockHeader, func(cnsMsg *consensus.Message) bool {
		_ = wrk.ConsensusState().SetJobDone(wrk.ConsensusState().ConsensusGroup()[0], bls.SrBlock, true)
		return true
	})
	rnd := wrk.SlotManager()
	slotDuration := rnd.TimeDuration()
	rnd.UpdateSlot(time.Now().Add(slotDuration))
	cnsGroup := wrk.ConsensusState().ConsensusGroup()
	hdr := &block.Block{Header: &block.BlockHeader{ChainID: chainID}}
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(hdr)
	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		hdrStr,
		[]byte(cnsGroup[0]),
		[]byte("sig"),
		int(bls.MtBlockHeader),
		1,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	wrk.ExecuteMessageChannel() <- cnsMsg
	time.Sleep(1000 * time.Millisecond)
	isBlockJobDone, err := wrk.ConsensusState().JobDone(cnsGroup[0], bls.SrBlock)

	assert.Nil(t, err)
	assert.True(t, isBlockJobDone)

	_ = wrk.Close()
}

func TestWorker_ExtendShouldReturnWhenSlotIsCanceled(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	executed := false
	bootstrapperMock := &mock.BootstrapperMock{
		GetNodeStateCalled: func() core.NodeState {
			return core.NsNotSynchronized
		},
		CreateAndCommitEmptyBlockCalled: func() (data.HeaderHandler, error) {
			executed = true
			return nil, errors.New("error")
		},
	}
	wrk.SetBootstrapper(bootstrapperMock)
	wrk.ConsensusState().SlotCanceled = true
	wrk.Extend(0)

	assert.False(t, executed)
}

func TestWorker_ExtendShouldReturnWhenGetNodeStateNotReturnSynchronized(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	executed := false
	bootstrapperMock := &mock.BootstrapperMock{
		GetNodeStateCalled: func() core.NodeState {
			return core.NsNotSynchronized
		},
		CreateAndCommitEmptyBlockCalled: func() (data.HeaderHandler, error) {
			executed = true
			return nil, errors.New("error")
		},
	}
	wrk.SetBootstrapper(bootstrapperMock)
	wrk.Extend(0)

	assert.False(t, executed)
}

func TestWorker_ExtendShouldReturnWhenCreateEmptyBlockFail(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	executed := false
	bmm := &mock.BroadcastMessengerMock{
		BroadcastBlockCalled: func(handler data.HeaderHandler) error {
			executed = true
			return nil
		},
	}
	wrk.SetBroadcastMessenger(bmm)
	bootstrapperMock := &mock.BootstrapperMock{
		CreateAndCommitEmptyBlockCalled: func() (data.HeaderHandler, error) {
			return nil, errors.New("error")
		}}
	wrk.SetBootstrapper(bootstrapperMock)
	wrk.Extend(0)

	assert.False(t, executed)
}

func TestWorker_ExtendShouldWorkAfterAWhile(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	executed := int32(0)
	blockProcessor := &mock.BlockProcessorMock{
		RevertStateToSnapshotCalled: func(header data.HeaderHandler) {
			atomic.AddInt32(&executed, 1)
		},
	}
	wrk.SetBlockProcessor(blockProcessor)
	wrk.ConsensusState().SetProcessingBlock(true)
	n := 10
	go func() {
		for n > 0 {
			time.Sleep(100 * time.Millisecond)
			n--
		}
		wrk.ConsensusState().SetProcessingBlock(false)
	}()
	wrk.Extend(1)

	assert.Equal(t, int32(1), atomic.LoadInt32(&executed))
	assert.Equal(t, 0, n)
}

func TestWorker_ExtendShouldWork(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	executed := int32(0)
	blockProcessor := &mock.BlockProcessorMock{
		RevertStateToSnapshotCalled: func(header data.HeaderHandler) {
			atomic.AddInt32(&executed, 1)
		},
	}
	wrk.SetBlockProcessor(blockProcessor)
	wrk.Extend(1)
	time.Sleep(1000 * time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&executed))
}

func TestWorker_ExecuteStoredMessagesShouldWork(t *testing.T) {
	t.Parallel()
	wrk := *initWorker()
	wrk.StartWorking()
	blk := &block.Block{Header: &block.BlockHeader{}}
	blkStr, _ := mock.MarshalizerMock{}.Marshal(blk)
	wrk.InitReceivedMessages()
	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		blkStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtBlockHeader),
		0, 0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	msgType := consensus.MessageType(cnsMsg.MsgType)
	cnsDataList := wrk.ReceivedMessages()[msgType]
	cnsDataList = append(cnsDataList, cnsMsg)
	wrk.SetReceivedMessages(msgType, cnsDataList)
	wrk.ConsensusState().SetStatus(bls.SrStartSlot, slot.SsFinished)

	rcvMsg := wrk.ReceivedMessages()
	assert.Equal(t, 1, len(rcvMsg[msgType]))

	wrk.ExecuteStoredMessages()

	rcvMsg = wrk.ReceivedMessages()
	assert.Equal(t, 0, len(rcvMsg[msgType]))

	_ = wrk.Close()
}

func TestWorker_SetAppStatusHandlerNilShouldErr(t *testing.T) {
	t.Parallel()

	wrk := slot.Worker{}
	err := wrk.SetAppStatusHandler(nil)

	assert.Equal(t, slot.ErrNilAppStatusHandler, err)
}

func TestWorker_SetAppStatusHandlerShouldWork(t *testing.T) {
	t.Parallel()

	wrk := slot.Worker{}
	handler := &cMock.AppStatusHandlerStub{}
	err := wrk.SetAppStatusHandler(handler)

	assert.Nil(t, err)
	assert.True(t, handler == wrk.AppStatusHandler())
}

func TestWorker_ProcessReceivedMessageWrongHeaderShouldErr(t *testing.T) {
	t.Parallel()

	workerArgs := createDefaultWorkerArgs()
	headerSigVerifier := &mock.HeaderSigVerifierStub{}
	headerSigVerifier.VerifyRandSeedCalled = func(header data.HeaderHandler) error {
		return process.ErrRandSeedDoesNotMatch
	}

	workerArgs.HeaderSigVerifier = headerSigVerifier
	wrk, _ := slot.NewWorker(workerArgs)
	hdr := &block.Block{Header: &block.BlockHeader{Nonce: 1, Timestamp: wrk.SlotManager().Timestamp().Unix()}}
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(hdr)

	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		hdrStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		signature,
		int(bls.MtBlockHeader),
		0, 0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)
	time.Sleep(time.Second)
	msg := &cMock.P2PMessageMock{
		DataField: buff,
		PeerField: currentPid,
	}
	err := wrk.ProcessReceivedMessage(msg, "")
	assert.True(t, errors.Is(err, slot.ErrInvalidHeaderHashSize))
}

// ErrEpochNodesConfigDoesNotExist signals that the epoch nodes configuration is missing
var ErrEpochNodesConfigDoesNotExist = errors.New("epoch nodes configuration does not exist")

func ComputeConsensusGroup() error {
	return fmt.Errorf("%w epoch=%v", ErrEpochNodesConfigDoesNotExist, 10)
}

func getLeader() error {
	err := ComputeConsensusGroup()
	return err
}

func VerifyRandSeed() error {
	err := getLeader()
	return fmt.Errorf("%w : verify rand seed for received header from consensus topic failed",
		err)
}

func TestErrorWrapper(t *testing.T) {
	err := getLeader()
	assert.True(t, errors.Is(err, ErrEpochNodesConfigDoesNotExist))

}

func TestWorker_ProcessReceivedMessageShouldErrIfAntifloodHandlerRejects(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("cannot process message")
	antifloodHandler := &cMock.P2PAntifloodHandlerStub{
		CanProcessMessageCalled: func(message p2p.MessageP2P, fromConnectedPeer core.PeerID) error {
			return expectedErr
		},
	}

	workerArgs := createDefaultWorkerArgs()
	workerArgs.AntifloodHandler = antifloodHandler
	wrk, _ := slot.NewWorker(workerArgs)

	msg := &cMock.P2PMessageMock{
		DataField: []byte("test data"),
		PeerField: core.PeerID("test peer"),
	}
	err := wrk.ProcessReceivedMessage(msg, fromConnectedPeerId)

	assert.Equal(t, expectedErr, err)
}

func TestWorker_ReportNetworkDegraded_CooldownBehavior(t *testing.T) {
	t.Parallel()

	// Setup
	workerArgs := createDefaultWorkerArgs()
	workerArgs.NetworkDegradedThreshold = 3
	workerArgs.NetworkDegradedCooldownSlots = 10

	slotMgr := &mock.SlotManagerMock{
		SlotIndex: 100,
		TimestampCalled: func() time.Time {
			return time.Unix(0, 0)
		},
		TimeDurationCalled: func() time.Duration {
			return 5 * time.Second
		},
	}
	workerArgs.SlotManager = slotMgr

	wrk, _ := slot.NewWorker(workerArgs)

	// Setup consensus state with 20 validators
	consensusGroupSize := 20
	wrk.ConsensusState().SetConsensusGroupSize(consensusGroupSize)

	leader := "leader"
	// Create messages from only 14 validators (20 - 1 leader - 5 failed = 14)
	// This means 5 validators failed (expectedSignatures=19, actual=14, failed=5 >= threshold=3)
	msgs := make([]*consensus.Message, 14)
	for i := 0; i < 14; i++ {
		msgs[i] = &consensus.Message{
			PubKey: []byte(fmt.Sprintf("validator%d", i)),
			Nonce:  1,
		}
	}

	// First call should fire (updates last alert slot)
	wrk.ReportNetworkDegraded(leader, msgs)
	assert.Equal(t, int64(100), wrk.LastNetworkDegradedAlertSlot())

	// Same slot: should be suppressed by cooldown
	wrk.ReportNetworkDegraded(leader, msgs)
	assert.Equal(t, int64(100), wrk.LastNetworkDegradedAlertSlot())

	// Advance slot but still within cooldown (100 + 10 = 110, slot 105 < 110)
	slotMgr.SlotIndex = 105
	wrk.ReportNetworkDegraded(leader, msgs)
	assert.Equal(t, int64(100), wrk.LastNetworkDegradedAlertSlot())

	// Advance past cooldown (100 + 10 = 110, slot 115 >= 110)
	slotMgr.SlotIndex = 115
	wrk.ReportNetworkDegraded(leader, msgs)
	assert.Equal(t, int64(115), wrk.LastNetworkDegradedAlertSlot())
}

func TestWorker_ReportNetworkDegraded_ThresholdZeroDisablesAlerting(t *testing.T) {
	t.Parallel()

	// Setup
	workerArgs := createDefaultWorkerArgs()
	workerArgs.NetworkDegradedThreshold = 0 // Disabled
	workerArgs.NetworkDegradedCooldownSlots = 10

	slotMgr := &mock.SlotManagerMock{
		SlotIndex: 100,
	}
	workerArgs.SlotManager = slotMgr

	wrk, _ := slot.NewWorker(workerArgs)

	consensusGroupSize := 20
	wrk.ConsensusState().SetConsensusGroupSize(consensusGroupSize)

	leader := "leader"
	msgs := make([]*consensus.Message, 10) // 9 failed (should trigger if enabled)

	// Should not update last alert slot because threshold is 0
	wrk.ReportNetworkDegraded(leader, msgs)
	assert.Equal(t, int64(0), wrk.LastNetworkDegradedAlertSlot())
}

func TestWorker_ReportNetworkDegraded_BelowThresholdDoesNotAlert(t *testing.T) {
	t.Parallel()

	// Setup
	workerArgs := createDefaultWorkerArgs()
	workerArgs.NetworkDegradedThreshold = 5 // Need 5+ failures
	workerArgs.NetworkDegradedCooldownSlots = 10

	slotMgr := &mock.SlotManagerMock{
		SlotIndex: 100,
	}
	workerArgs.SlotManager = slotMgr

	wrk, _ := slot.NewWorker(workerArgs)

	consensusGroupSize := 20
	wrk.ConsensusState().SetConsensusGroupSize(consensusGroupSize)

	leader := "leader"
	// Create messages from 15 validators (20 - 1 leader - 4 failed = 15)
	// This means 4 validators failed (< threshold of 5)
	msgs := make([]*consensus.Message, 15)

	// Should not alert because failed count (4) < threshold (5)
	wrk.ReportNetworkDegraded(leader, msgs)
	assert.Equal(t, int64(0), wrk.LastNetworkDegradedAlertSlot())
}

func TestWorker_ReportNetworkDegraded_AtThresholdDoesAlert(t *testing.T) {
	t.Parallel()

	// Setup
	workerArgs := createDefaultWorkerArgs()
	workerArgs.NetworkDegradedThreshold = 3
	workerArgs.NetworkDegradedCooldownSlots = 10

	slotMgr := &mock.SlotManagerMock{
		SlotIndex: 100,
	}
	workerArgs.SlotManager = slotMgr

	wrk, _ := slot.NewWorker(workerArgs)

	consensusGroupSize := 20
	wrk.ConsensusState().SetConsensusGroupSize(consensusGroupSize)

	leader := "leader"
	// Create messages from 16 validators (20 - 1 leader - 3 failed = 16)
	// This means exactly 3 validators failed (== threshold)
	msgs := make([]*consensus.Message, 16)

	// Should alert because failed count (3) >= threshold (3)
	wrk.ReportNetworkDegraded(leader, msgs)
	assert.Equal(t, int64(100), wrk.LastNetworkDegradedAlertSlot())
}

// One-slot-ahead MtBlockHeader messages are cached and later dispatched to the block-header
// callback without re-running the header checks. The header-validation invariant must therefore be
// enforced before caching, so a next-slot leader cannot smuggle a header with an invalid rand seed
// (or malformed integrity fields) past honest validators through the optimized cache path.
func TestWorker_ProcessReceivedMessageOneSlotAheadInvalidRandSeedShouldErrAndNotCache(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("invalid rand seed")
	hdr := &block.Block{Header: &block.BlockHeader{ChainID: chainID}}
	hdrHash, _ := tools.CalculateHash(mock.MarshalizerMock{}, cMock.HasherMock{}, hdr.GetBlockHeader())
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(hdr)

	var verifyRandSeedCalled int32
	workerArgs := createDefaultWorkerArgs()
	workerArgs.BlockProcessor = &mock.BlockProcessorMock{
		DecodeBlockHeaderCalled: func(_ []byte) data.HeaderHandler {
			return &cMock.HeaderHandlerStub{
				GetBlockHeaderCalled: func() interface{} {
					return hdr.GetBlockHeader()
				},
			}
		},
	}
	workerArgs.HeaderSigVerifier = &mock.HeaderSigVerifierStub{
		VerifyRandSeedCalled: func(_ data.HeaderHandler) error {
			atomic.AddInt32(&verifyRandSeedCalled, 1)
			return expectedErr
		},
	}
	wrk, _ := slot.NewWorker(workerArgs)

	cnsMsg := consensus.NewConsensusMessage(
		hdrHash,
		nil,
		hdrStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		signature,
		int(bls.MtBlockHeader),
		1, // one slot ahead of the current slot (0) -> hits the future-slot cache branch
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)

	err := wrk.ProcessReceivedMessage(&cMock.P2PMessageMock{DataField: buff, PeerField: currentPid}, fromConnectedPeerId)
	time.Sleep(time.Second)

	assert.True(t, errors.Is(err, expectedErr))
	assert.Equal(t, int32(1), atomic.LoadInt32(&verifyRandSeedCalled))
	assert.Equal(t, 0, len(wrk.ReceivedMessages()[bls.MtBlockHeader]))
}

// One-slot-ahead MtBlockHeader messages are cached and later dispatched to the block-header
// callback without re-running the header checks. The header-validation invariant must therefore be
// enforced before caching, so a next-slot leader cannot smuggle a header with an invalid rand seed
// (or malformed integrity fields) past honest validators through the optimized cache path.
func TestWorker_ProcessReceivedMessageOneSlotAheadValidHeaderShouldCache(t *testing.T) {
	t.Parallel()

	wrk := *initWorker()
	hdr := &block.Block{Header: &block.BlockHeader{ChainID: chainID}}
	hdrHash, _ := tools.CalculateHash(mock.MarshalizerMock{}, cMock.HasherMock{}, hdr.GetBlockHeader())
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(hdr)
	wrk.NewBlockProcessor(hdr.GetBlockHeader())

	cnsMsg := consensus.NewConsensusMessage(
		hdrHash,
		nil,
		hdrStr,
		[]byte(wrk.ConsensusState().ConsensusGroup()[0]),
		signature,
		int(bls.MtBlockHeader),
		1, // one slot ahead of the current slot (0) -> hits the future-slot cache branch
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	buff, _ := wrk.Marshalizer().Marshal(cnsMsg)

	err := wrk.ProcessReceivedMessage(&cMock.P2PMessageMock{DataField: buff, PeerField: currentPid}, fromConnectedPeerId)
	time.Sleep(time.Second)

	assert.Nil(t, err)
	assert.Equal(t, 1, len(wrk.ReceivedMessages()[bls.MtBlockHeader]))
}

// buildHeaderMessage returns a consensus message carrying header and a block header hash
// that matches the header when computed with the default worker marshalizer/hasher.
func buildHeaderMessage(header *block.BlockHeader) (*consensus.Message, []byte) {
	hdrHash, _ := tools.CalculateHash(mock.MarshalizerMock{}, cMock.HasherMock{}, header)
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(header)
	return &consensus.Message{
		BlockHeaderHash: hdrHash,
		Header:          hdrStr,
	}, hdrHash
}

func TestWorker_ValidateMessageWithHeaderValidHeaderShouldReturnHeader(t *testing.T) {
	t.Parallel()

	wrk := *initWorker()
	header := &block.BlockHeader{ChainID: chainID}
	wrk.NewBlockProcessor(header)
	cnsMsg, _ := buildHeaderMessage(header)

	hdr, err := wrk.ValidateMessageWithHeader(cnsMsg)

	require.NoError(t, err)
	require.NotNil(t, hdr)
}

func TestWorker_ValidateMessageWithHeaderNilHashShouldErr(t *testing.T) {
	t.Parallel()

	wrk := *initWorker()
	header := &block.BlockHeader{ChainID: chainID}
	wrk.NewBlockProcessor(header)
	cnsMsg, _ := buildHeaderMessage(header)
	cnsMsg.BlockHeaderHash = nil

	hdr, err := wrk.ValidateMessageWithHeader(cnsMsg)

	assert.Nil(t, hdr)
	assert.True(t, errors.Is(err, slot.ErrInvalidHeader))
}

func TestWorker_ValidateMessageWithHeaderNilDecodedHeaderShouldErr(t *testing.T) {
	t.Parallel()

	wrk := *initWorker()
	wrk.SetBlockProcessor(&mock.BlockProcessorMock{
		DecodeBlockHeaderCalled: func(dta []byte) data.HeaderHandler {
			return nil
		},
	})
	cnsMsg := &consensus.Message{
		BlockHeaderHash: blockHeaderHash,
		Header:          []byte("header"),
	}

	hdr, err := wrk.ValidateMessageWithHeader(cnsMsg)

	assert.Nil(t, hdr)
	assert.True(t, errors.Is(err, slot.ErrInvalidHeader))
}

func TestWorker_ValidateMessageWithHeaderHashMismatchShouldErr(t *testing.T) {
	t.Parallel()

	wrk := *initWorker()
	header := &block.BlockHeader{ChainID: chainID}
	wrk.NewBlockProcessor(header)
	cnsMsg, _ := buildHeaderMessage(header)
	// tamper the announced hash so it no longer matches the recomputed one
	cnsMsg.BlockHeaderHash = []byte("this-hash-does-not-match-the-header")

	hdr, err := wrk.ValidateMessageWithHeader(cnsMsg)

	assert.Nil(t, hdr)
	assert.Equal(t, slot.ErrHeaderkHashNotMatch, err)
}

func TestWorker_ValidateMessageWithHeaderComputeHashErrorShouldErr(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("marshal failed")
	workerArgs := createDefaultWorkerArgs()
	workerArgs.Marshalizer = &cMock.MarshalizerStub{
		MarshalCalled: func(any) ([]byte, error) {
			return nil, expectedErr
		},
	}
	wrk, err := slot.NewWorker(workerArgs)
	require.NoError(t, err)
	wrk.NewBlockProcessor(&block.BlockHeader{ChainID: chainID})

	cnsMsg := &consensus.Message{
		BlockHeaderHash: blockHeaderHash,
		Header:          []byte("header"),
	}

	hdr, err := wrk.ValidateMessageWithHeader(cnsMsg)

	assert.Nil(t, hdr)
	assert.True(t, errors.Is(err, expectedErr))
}

func TestWorker_ValidateMessageWithHeaderIntegrityVerifyFailsShouldErr(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("integrity check failed")
	workerArgs := createDefaultWorkerArgs()
	workerArgs.HeaderIntegrityVerifier = &mock.HeaderIntegrityVerifierStub{
		VerifyCalled: func(header data.HeaderHandler) error {
			return expectedErr
		},
	}
	wrk, err := slot.NewWorker(workerArgs)
	require.NoError(t, err)

	header := &block.BlockHeader{ChainID: chainID}
	wrk.NewBlockProcessor(header)
	cnsMsg, _ := buildHeaderMessage(header)

	hdr, err := wrk.ValidateMessageWithHeader(cnsMsg)

	assert.Nil(t, hdr)
	assert.True(t, errors.Is(err, expectedErr))
}

func TestWorker_ValidateMessageWithHeaderVerifyRandSeedFailsShouldErr(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("rand seed signature invalid")
	workerArgs := createDefaultWorkerArgs()
	workerArgs.HeaderSigVerifier = &mock.HeaderSigVerifierStub{
		VerifyRandSeedCalled: func(header data.HeaderHandler) error {
			return expectedErr
		},
	}
	wrk, err := slot.NewWorker(workerArgs)
	require.NoError(t, err)

	header := &block.BlockHeader{ChainID: chainID}
	wrk.NewBlockProcessor(header)
	cnsMsg, _ := buildHeaderMessage(header)

	hdr, err := wrk.ValidateMessageWithHeader(cnsMsg)

	assert.Nil(t, hdr)
	assert.True(t, errors.Is(err, expectedErr))
}
