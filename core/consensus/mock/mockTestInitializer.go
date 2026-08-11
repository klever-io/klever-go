package mock

import (
	"time"

	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/crypto"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
)

// InitChronologyHandlerMock -
func InitChronologyHandlerMock() consensus.ChronologyHandler {
	chr := &ChronologyHandlerMock{}
	return chr
}

// InitBlockProcessorMock -
func InitBlockProcessorMock() *BlockProcessorMock {
	blockProcessorMock := &BlockProcessorMock{}
	blockProcessorMock.CreateBlockCalled = func(header data.HeaderHandler, haveTime func() bool) (data.HeaderHandler, error) {
		emptyBlock := &block.Block{
			Header: &block.BlockHeader{},
		}
		emptyBlock.SetTrieRoot([]byte{})
		return emptyBlock, nil
	}
	blockProcessorMock.CommitBlockCalled = func(header data.HeaderHandler) error {
		return nil
	}
	blockProcessorMock.RevertStateToSnapshotCalled = func(header data.HeaderHandler) {}
	blockProcessorMock.ProcessBlockCalled = func(header data.HeaderHandler, haveTime func() time.Duration) error {
		return nil
	}
	blockProcessorMock.DecodeBlockHeaderCalled = func(dta []byte) data.HeaderHandler {
		return &block.Block{
			Header: &block.BlockHeader{},
		}
	}
	blockProcessorMock.MarshalizedDataToBroadcastCalled = func(header data.HeaderHandler) ([]byte, [][]byte, error) {
		return make([]byte, 0), make([][]byte, 0), nil
	}
	blockProcessorMock.CreateNewHeaderCalled = func(slot uint64, nonce uint64) data.HeaderHandler {
		return &block.Block{
			Header: &block.BlockHeader{
				Slot:  slot,
				Nonce: nonce,
			},
		}
	}

	return blockProcessorMock
}

// InitMultiSignerMock -
func InitMultiSignerMock() *cMock.BelNevMock {
	multiSigner := cMock.NewMultiSigner()
	multiSigner.CreateCommitmentMock = func() ([]byte, []byte) {
		return []byte("commSecret"), []byte("commitment")
	}
	multiSigner.VerifySignatureShareMock = func(index uint16, sig []byte, msg []byte) error {
		return nil
	}
	multiSigner.VerifyMock = func(msg []byte, bitmap []byte) error {
		return nil
	}
	multiSigner.AggregateSigsMock = func(bitmap []byte) ([]byte, error) {
		return []byte("aggregatedSig"), nil
	}
	multiSigner.AggregateCommitmentsMock = func(bitmap []byte) error {
		return nil
	}
	multiSigner.CreateSignatureShareMock = func(msg []byte, bitmap []byte) ([]byte, error) {
		return []byte("partialSign"), nil
	}
	return multiSigner
}

// InitKeys -
func InitKeys() (*cryptoMock.KeyGenMock, *cryptoMock.PrivateKeyMock, *cryptoMock.PublicKeyMock) {
	toByteArrayMock := func() ([]byte, error) {
		return []byte("byteArray"), nil
	}
	privKeyMock := &cryptoMock.PrivateKeyMock{
		ToByteArrayMock: toByteArrayMock,
	}
	pubKeyMock := &cryptoMock.PublicKeyMock{
		ToByteArrayMock: toByteArrayMock,
	}
	privKeyFromByteArr := func(b []byte) (crypto.PrivateKey, error) {
		return privKeyMock, nil
	}
	pubKeyFromByteArr := func(b []byte) (crypto.PublicKey, error) {
		return pubKeyMock, nil
	}
	keyGenMock := &cryptoMock.KeyGenMock{
		PrivateKeyFromByteArrayMock: privKeyFromByteArr,
		PublicKeyFromByteArrayMock:  pubKeyFromByteArr,
	}
	return keyGenMock, privKeyMock, pubKeyMock
}

// InitConsensusCore -
func InitConsensusCore() *ConsensusCoreMock {

	blockChain := &cMock.BlockChainMock{
		GetGenesisHeaderCalled: func() data.HeaderHandler {
			return &block.Block{Header: &block.BlockHeader{RandSeed: []byte("")}}
		},
	}
	blockProcessorMock := InitBlockProcessorMock()
	bootstrapperMock := &BootstrapperMock{}
	broadcastMessengerMock := &BroadcastMessengerMock{
		BroadcastConsensusMessageCalled: func(message *consensus.Message) error {
			return nil
		},
	}

	chronologyHandlerMock := InitChronologyHandlerMock()
	hasherMock := &cMock.HasherMock{}
	marshalizerMock := &cMock.MarshalizerMock{}
	blsPrivateKeyMock := &cryptoMock.PrivateKeyMock{}
	blsSingleSignerMock := &cMock.SingleSignerMock{
		SignStub: func(private crypto.PrivateKey, msg []byte) (bytes []byte, e error) {
			return make([]byte, 0), nil
		},
	}
	multiSignerMock := InitMultiSignerMock()
	slotManagerMock := &SlotManagerMock{}
	syncTimerMock := &SyncTimerMock{}
	validatorGroupSelector := cMock.NewNodesCoordinatorMock()
	epochStartSubscriber := &cMock.EpochStartNotifierStub{}
	antifloodHandler := &cMock.P2PAntifloodHandlerStub{}
	headerPoolSubscriber := &cMock.HeadersCacherStub{}
	peerHonestyHandler := &cMock.PeerHonestyHandlerStub{}
	headerSigVerifier := &HeaderSigVerifierStub{}
	fallbackHeaderValidator := &cMock.FallBackHeaderValidatorStub{}
	nodeRedundancyHandler := &NodeRedundancyHandlerStub{}
	// every fork active unless a test opts out explicitly via SetForkController
	// or by mutating this instance
	forkControllerStub := cMock.NewForkControllerStub()

	container := &ConsensusCoreMock{
		blockChain:              blockChain,
		blockProcessor:          blockProcessorMock,
		headersSubscriber:       headerPoolSubscriber,
		bootstrapper:            bootstrapperMock,
		broadcastMessenger:      broadcastMessengerMock,
		chronologyHandler:       chronologyHandlerMock,
		hasher:                  hasherMock,
		marshalizer:             marshalizerMock,
		blsPrivateKey:           blsPrivateKeyMock,
		blsSingleSigner:         blsSingleSignerMock,
		multiSigner:             multiSignerMock,
		slotManager:             slotManagerMock,
		syncTimer:               syncTimerMock,
		validatorGroupSelector:  validatorGroupSelector,
		epochStartNotifier:      epochStartSubscriber,
		antifloodHandler:        antifloodHandler,
		peerHonestyHandler:      peerHonestyHandler,
		headerSigVerifier:       headerSigVerifier,
		fallbackHeaderValidator: fallbackHeaderValidator,
		nodeRedundancyHandler:   nodeRedundancyHandler,
		forkController:          forkControllerStub,
	}

	return container
}
