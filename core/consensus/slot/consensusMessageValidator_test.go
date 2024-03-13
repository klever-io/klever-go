package slot_test

import (
	"crypto/rand"
	"errors"
	"testing"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/consensus/slot/bls"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/tools"
	"github.com/stretchr/testify/assert"
)

func createDefaultConsensusMessageValidatorArgs() *slot.ArgsConsensusMessageValidator {
	consensusState := initConsensusState()
	blsService, _ := bls.NewConsensusService()
	singleSignerMock := &commonMock.SingleSignerMock{
		SignStub: func(private crypto.PrivateKey, msg []byte) ([]byte, error) {
			return []byte("signed"), nil
		},
		VerifyStub: func(public crypto.PublicKey, msg []byte, sig []byte) error {
			return nil
		},
	}
	keyGeneratorMock, _, _ := mock.InitKeys()
	peerSigHandler := &commonMock.PeerSignatureHandler{Signer: singleSignerMock, KeyGen: keyGeneratorMock}
	hasher := &commonMock.HasherMock{}

	argsConsensusMessageValidator := &slot.ArgsConsensusMessageValidator{
		ConsensusState:       consensusState,
		ConsensusService:     blsService,
		PeerSignatureHandler: peerSigHandler,
		SignatureSize:        SignatureSize,
		PublicKeySize:        PublicKeySize,
		HasherSize:           hasher.Size(),
		ChainID:              chainID,
	}

	return argsConsensusMessageValidator
}

func TestCheckConsensusMessageValidity_WrongChainID(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{ChainID: wrongChainID}
	err := cmv.CheckConsensusMessageValidity(cnsMsg, "")
	assert.True(t, errors.Is(err, slot.ErrInvalidChainID))
}

func TestCheckMessageWithFinalInfoValidity_InvalidMessage(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{Header: []byte("body")}
	err := cmv.CheckMessageWithFinalInfoValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidMessage))
}

func TestCheckMessageWithFinalInfoValidity_InvalidPubKeyBitmap(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{PubKeysBitmap: []byte("0")}
	err := cmv.CheckMessageWithFinalInfoValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidPublicKeyBitmapSize))
}

func TestCheckMessageWithFinalInfoValidity_InvalidAggregateSignatureSize(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{PubKeysBitmap: []byte("01"), AggregateSignature: []byte("0")}
	err := cmv.CheckMessageWithFinalInfoValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidSignatureSize))
}

func TestCheckMessageWithFinalInfoValidity_InvalidLeaderSignatureSize(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	sig := make([]byte, SignatureSize)
	_, _ = rand.Read(sig)
	cnsMsg := &consensus.Message{PubKeysBitmap: []byte("01"), AggregateSignature: sig, LeaderSignature: []byte("0")}
	err := cmv.CheckMessageWithFinalInfoValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidSignatureSize))
}

func TestCheckMessageWithFinalInfoValidity_ShouldWork(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	sig := make([]byte, SignatureSize)
	_, _ = rand.Read(sig)
	cnsMsg := &consensus.Message{PubKeysBitmap: []byte("01"), AggregateSignature: sig, LeaderSignature: sig}
	err := cmv.CheckMessageWithFinalInfoValidity(cnsMsg)
	assert.Nil(t, err)
}

func TestCheckMessageWithSignatureValidity_InvalidMessage(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{Header: []byte("body")}
	err := cmv.CheckMessageWithSignatureValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidMessage))
}

func TestCheckMessageWithSignatureValidity_InvalidSignatureShareSize(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{SignatureShare: []byte("0")}
	err := cmv.CheckMessageWithSignatureValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidSignatureSize))
}

func TestCheckMessageWithSignatureShareValidity_ShouldWork(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	sig := make([]byte, SignatureSize)
	_, _ = rand.Read(sig)
	cnsMsg := &consensus.Message{SignatureShare: sig}
	err := cmv.CheckMessageWithSignatureValidity(cnsMsg)
	assert.Nil(t, err)
}

func TestCheckMessageWithBlockHeaderValidity_InvalidMessage(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{SignatureShare: []byte("0")}
	err := cmv.CheckMessageWithBlockBodyAndHeaderValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidMessage))
}

func TestCheckMessageWithBlockHeaderValidity_InvalidHeaderSize(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{}
	err := cmv.CheckMessageWithBlockBodyAndHeaderValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidHeaderSize))
}

func TestCheckMessageWithBlockHeaderValidity_HeaderTooBig(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, tools.MegabyteSize+1)
	_, _ = rand.Read(headerBytes)
	cnsMsg := &consensus.Message{Header: headerBytes}
	err := cmv.CheckMessageWithBlockBodyAndHeaderValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidHeaderSize))
}

func TestCheckMessageWithBlockHeaderValidity_HeaderSizeZero(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, 0)
	cnsMsg := &consensus.Message{Header: headerBytes}
	err := cmv.CheckMessageWithBlockBodyAndHeaderValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidHeaderSize))
}

func TestCheckMessageWithBlockHeaderValidity_ShouldWork(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{Header: []byte("header")}
	err := cmv.CheckMessageWithBlockBodyAndHeaderValidity(cnsMsg)
	assert.Nil(t, err)
}

func TestCheckMessageWithBlockBodyValidity_InvalidMessage(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	signatureShare := make([]byte, 100)
	_, _ = rand.Read(signatureShare)
	cnsMsg := &consensus.Message{SignatureShare: signatureShare}
	err := cmv.CheckMessageWithBlockBodyAndHeaderValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidMessage))
}

func TestCheckMessageWithBlockBodyValidity_InvalidBodySize(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	bodyBytes := make([]byte, tools.MegabyteSize+1)
	_, _ = rand.Read(bodyBytes)
	cnsMsg := &consensus.Message{Header: bodyBytes}
	err := cmv.CheckMessageWithBlockBodyAndHeaderValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidHeaderSize))
}

func TestCheckMessageWithBlockBodyValidity_ShouldWork(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	bodyBytes := make([]byte, 100)
	_, _ = rand.Read(bodyBytes)
	cnsMsg := &consensus.Message{Header: bodyBytes}
	err := cmv.CheckMessageWithBlockBodyAndHeaderValidity(cnsMsg)
	assert.Nil(t, err)
}

func TestCheckMessageWithBlockBodyAndHeaderValidity_InvalidMessage(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{SignatureShare: []byte("0")}
	err := cmv.CheckMessageWithBlockBodyAndHeaderValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidMessage))
}

func TestCheckMessageWithBlockBodyAndHeaderValidity_InvalidBodySize(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	bodyBytes := make([]byte, tools.MegabyteSize+1)
	_, _ = rand.Read(bodyBytes)
	cnsMsg := &consensus.Message{Header: bodyBytes}
	err := cmv.CheckMessageWithBlockBodyAndHeaderValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidHeaderSize))
}

func TestCheckMessageWithBlockBodyAndHeaderValidity_InvalidHeaderSize(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, tools.MegabyteSize+1)
	_, _ = rand.Read(headerBytes)
	cnsMsg := &consensus.Message{Header: headerBytes}
	err := cmv.CheckMessageWithBlockBodyAndHeaderValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidHeaderSize))
}

func TestCheckMessageWithBlockBodyAndHeaderValidity_ShouldWork(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, 100)
	_, _ = rand.Read(headerBytes)
	cnsMsg := &consensus.Message{Header: headerBytes}
	err := cmv.CheckMessageWithBlockBodyAndHeaderValidity(cnsMsg)
	assert.Nil(t, err)
}

func TestCheckConsensusMessageValidityForMessageType_MessageWithBlockBodyAndHeaderInvalid(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{MsgType: int64(bls.MtBlockBodyAndHeader), SignatureShare: []byte("1")}
	err := cmv.CheckConsensusMessageValidityForMessageType(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidMessage))
}

func TestCheckConsensusMessageValidityForMessageType_MessageWithBlockBodyInvalid(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{MsgType: int64(bls.MtBlockBodyAndHeader), SignatureShare: []byte("1")}
	err := cmv.CheckConsensusMessageValidityForMessageType(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidMessage))
}

func TestCheckConsensusMessageValidityForMessageType_MessageWithBlockHeaderInvalid(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{MsgType: int64(bls.MtBlockBodyAndHeader), SignatureShare: []byte("1")}
	err := cmv.CheckConsensusMessageValidityForMessageType(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidMessage))
}

func TestCheckConsensusMessageValidityForMessageType_MessageWithSignatureInvalid(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{MsgType: int64(bls.MtSignature), Header: []byte("1")}
	err := cmv.CheckConsensusMessageValidityForMessageType(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidMessage))
}

func TestCheckConsensusMessageValidityForMessageType_MessageWithFinalInfoInvalid(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{MsgType: int64(bls.MtBlockHeaderFinalInfo), Header: []byte("1")}
	err := cmv.CheckConsensusMessageValidityForMessageType(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidMessage))
}

func TestCheckConsensusMessageValidityForMessageType_MessageUnknownInvalid(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{MsgType: int64(bls.MtUnknown), Header: []byte("1")}
	err := cmv.CheckConsensusMessageValidityForMessageType(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidMessageType))
}

func TestIsBlockHeaderHashSizeValid_NotValid(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{MsgType: int64(bls.MtBlockBody), BlockHeaderHash: []byte("hash")}
	result := cmv.IsBlockHeaderHashSizeValid(cnsMsg)
	assert.False(t, result)
}

func TestIsBlockHeaderHashSizeValid(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerHash := make([]byte, consensusMessageValidatorArgs.HasherSize)
	_, _ = rand.Read(headerHash)
	cnsMsg := &consensus.Message{MsgType: int64(bls.MtBlockHeader), BlockHeaderHash: headerHash}
	result := cmv.IsBlockHeaderHashSizeValid(cnsMsg)
	assert.True(t, result)
}

func TestCheckConsensusMessageValidity_InvalidMessage(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{ChainID: chainID, MsgType: int64(bls.MtBlockBodyAndHeader), SignatureShare: []byte("1")}
	err := cmv.CheckConsensusMessageValidity(cnsMsg, "")
	assert.True(t, errors.Is(err, slot.ErrInvalidMessage))
}

func TestCheckConsensusMessageValidity_InvalidHeaderHashSize(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, 100)
	_, _ = rand.Read(headerBytes)
	cnsMsg := &consensus.Message{ChainID: chainID, MsgType: int64(bls.MtBlockBodyAndHeader), Header: headerBytes}
	err := cmv.CheckConsensusMessageValidity(cnsMsg, "")
	assert.True(t, errors.Is(err, slot.ErrInvalidHeaderHashSize))
}

func TestCheckConsensusMessageValidity_InvalidPublicKeySize(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, 100)
	_, _ = rand.Read(headerBytes)
	headerHash := make([]byte, consensusMessageValidatorArgs.HasherSize)
	_, _ = rand.Read(headerHash)
	cnsMsg := &consensus.Message{ChainID: chainID, MsgType: int64(bls.MtBlockBodyAndHeader), Header: headerBytes, BlockHeaderHash: headerHash}
	err := cmv.CheckConsensusMessageValidity(cnsMsg, "")
	assert.True(t, errors.Is(err, slot.ErrInvalidPublicKeySize))
}

func TestCheckConsensusMessageValidity_InvalidSignatureSize(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, 100)
	_, _ = rand.Read(headerBytes)
	headerHash := make([]byte, consensusMessageValidatorArgs.HasherSize)
	_, _ = rand.Read(headerHash)
	pubKey := make([]byte, PublicKeySize)
	_, _ = rand.Read(pubKey)

	cnsMsg := &consensus.Message{
		ChainID: chainID, MsgType: int64(bls.MtBlockBodyAndHeader), Header: headerBytes, BlockHeaderHash: headerHash, PubKey: pubKey,
	}
	err := cmv.CheckConsensusMessageValidity(cnsMsg, "")
	assert.True(t, errors.Is(err, slot.ErrInvalidSignatureSize))
}

func TestCheckConsensusMessageValidity_NodeIsNotEligible(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, 100)
	_, _ = rand.Read(headerBytes)
	headerHash := make([]byte, consensusMessageValidatorArgs.HasherSize)
	_, _ = rand.Read(headerHash)
	pubKey := make([]byte, PublicKeySize)
	_, _ = rand.Read(pubKey)
	sig := make([]byte, SignatureSize)
	_, _ = rand.Read(sig)

	cnsMsg := &consensus.Message{
		ChainID: chainID, MsgType: int64(bls.MtBlockBodyAndHeader),
		Header: headerBytes, BlockHeaderHash: headerHash, PubKey: pubKey, Signature: sig,
	}
	err := cmv.CheckConsensusMessageValidity(cnsMsg, "")
	assert.True(t, errors.Is(err, slot.ErrNodeIsNotInElectedList))
}

func TestCheckConsensusMessageValidity_ErrMessageForFutureSlot(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, 100)
	_, _ = rand.Read(headerBytes)
	headerHash := make([]byte, consensusMessageValidatorArgs.HasherSize)
	_, _ = rand.Read(headerHash)
	pubKey := []byte(consensusMessageValidatorArgs.ConsensusState.ConsensusGroup()[0])
	sig := make([]byte, SignatureSize)
	_, _ = rand.Read(sig)

	cnsMsg := &consensus.Message{
		ChainID: chainID, MsgType: int64(bls.MtBlockBodyAndHeader),
		Header: headerBytes, BlockHeaderHash: headerHash, PubKey: pubKey, Signature: sig, SlotIndex: 10,
	}
	err := cmv.CheckConsensusMessageValidity(cnsMsg, "")
	assert.True(t, errors.Is(err, slot.ErrMessageForFutureSlot))
}

func TestCheckConsensusMessageValidity_ErrMessageForPastSlot(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	consensusMessageValidatorArgs.ConsensusState.SlotIndex = 100
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, 100)
	_, _ = rand.Read(headerBytes)
	headerHash := make([]byte, consensusMessageValidatorArgs.HasherSize)
	_, _ = rand.Read(headerHash)
	pubKey := []byte(consensusMessageValidatorArgs.ConsensusState.ConsensusGroup()[0])
	sig := make([]byte, SignatureSize)
	_, _ = rand.Read(sig)

	cnsMsg := &consensus.Message{
		ChainID: chainID, MsgType: int64(bls.MtBlockBodyAndHeader),
		Header: headerBytes, BlockHeaderHash: headerHash, PubKey: pubKey, Signature: sig, SlotIndex: 10,
	}
	err := cmv.CheckConsensusMessageValidity(cnsMsg, "")
	assert.True(t, errors.Is(err, slot.ErrMessageForPastSlot))
}

func TestCheckConsensusMessageValidity_ErrMessageTypeLimitReached(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	consensusMessageValidatorArgs.ConsensusState.SlotIndex = 10
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, 100)
	_, _ = rand.Read(headerBytes)
	headerHash := make([]byte, consensusMessageValidatorArgs.HasherSize)
	_, _ = rand.Read(headerHash)
	pubKey := []byte(consensusMessageValidatorArgs.ConsensusState.ConsensusGroup()[0])
	sig := make([]byte, SignatureSize)
	_, _ = rand.Read(sig)

	cmv.AddMessageTypeToPublicKey(pubKey, 10, bls.MtBlockBodyAndHeader)

	cnsMsg := &consensus.Message{
		ChainID: chainID, MsgType: int64(bls.MtBlockBodyAndHeader),
		Header: headerBytes, BlockHeaderHash: headerHash, PubKey: pubKey, Signature: sig, SlotIndex: 10,
	}
	err := cmv.CheckConsensusMessageValidity(cnsMsg, "")
	assert.True(t, errors.Is(err, slot.ErrMessageTypeLimitReached))
}

func TestCheckConsensusMessageValidity_InvalidSignature(t *testing.T) {
	t.Parallel()

	localErr := errors.New("local error")
	signer := &commonMock.SingleSignerMock{
		VerifyStub: func(public crypto.PublicKey, msg []byte, sig []byte) error {
			return localErr
		},
	}

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	consensusMessageValidatorArgs.PeerSignatureHandler = &commonMock.PeerSignatureHandler{
		Signer: signer,
	}
	consensusMessageValidatorArgs.ConsensusState.SlotIndex = 10
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, 100)
	_, _ = rand.Read(headerBytes)
	headerHash := make([]byte, consensusMessageValidatorArgs.HasherSize)
	_, _ = rand.Read(headerHash)
	pubKey := []byte(consensusMessageValidatorArgs.ConsensusState.ConsensusGroup()[0])
	sig := make([]byte, SignatureSize)
	_, _ = rand.Read(sig)

	cnsMsg := &consensus.Message{
		ChainID: chainID, MsgType: int64(bls.MtBlockBodyAndHeader),
		Header: headerBytes, BlockHeaderHash: headerHash, PubKey: pubKey, Signature: sig, SlotIndex: 10,
	}
	err := cmv.CheckConsensusMessageValidity(cnsMsg, "")
	assert.True(t, errors.Is(err, crypto.ErrInvalidSignature))
}

func TestCheckConsensusMessageValidity_Ok(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	consensusMessageValidatorArgs.ConsensusState.SlotIndex = 10
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, 100)
	_, _ = rand.Read(headerBytes)
	headerHash := make([]byte, consensusMessageValidatorArgs.HasherSize)
	_, _ = rand.Read(headerHash)
	pubKey := []byte(consensusMessageValidatorArgs.ConsensusState.ConsensusGroup()[0])
	sig := make([]byte, SignatureSize)
	_, _ = rand.Read(sig)

	cnsMsg := &consensus.Message{
		ChainID: chainID, MsgType: int64(bls.MtBlockBodyAndHeader),
		Header: headerBytes, BlockHeaderHash: headerHash, PubKey: pubKey, Signature: sig, SlotIndex: 10,
	}
	err := cmv.CheckConsensusMessageValidity(cnsMsg, "")
	assert.Nil(t, err)
}

func TestIsMessageTypeLimitReached_ShouldWork(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	assert.False(t, cmv.IsMessageTypeLimitReached([]byte("pk1"), 1, bls.MtBlockBody))

	cmv.AddMessageTypeToPublicKey([]byte("pk1"), 1, bls.MtBlockHeader)

	assert.False(t, cmv.IsMessageTypeLimitReached([]byte("pk1"), 1, bls.MtBlockBody))
	assert.True(t, cmv.IsMessageTypeLimitReached([]byte("pk1"), 1, bls.MtBlockHeader))
	assert.False(t, cmv.IsMessageTypeLimitReached([]byte("pk1"), 2, bls.MtBlockHeader))
}

func TestAddMessageTypeToPublicKey_ShouldWork(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	assert.Equal(t, uint32(0), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk1"), 1, bls.MtBlockBody))

	cmv.AddMessageTypeToPublicKey([]byte("pk1"), 1, bls.MtBlockHeader)

	assert.Equal(t, uint32(0), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk1"), 1, bls.MtBlockBody))
	assert.Equal(t, uint32(0), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk1"), 2, bls.MtBlockHeader))

	cmv.AddMessageTypeToPublicKey([]byte("pk1"), 1, bls.MtBlockBody)
	cmv.AddMessageTypeToPublicKey([]byte("pk1"), 1, bls.MtBlockHeader)

	assert.Equal(t, uint32(1), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk1"), 1, bls.MtBlockBody))
	assert.Equal(t, uint32(2), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk1"), 1, bls.MtBlockHeader))
	assert.Equal(t, uint32(0), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk1"), 2, bls.MtBlockHeader))

	cmv.AddMessageTypeToPublicKey([]byte("pk2"), 1, bls.MtBlockHeaderFinalInfo)

	assert.Equal(t, uint32(0), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk1"), 1, bls.MtBlockHeaderFinalInfo))
	assert.Equal(t, uint32(0), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk2"), 2, bls.MtBlockHeaderFinalInfo))
	assert.Equal(t, uint32(1), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk2"), 1, bls.MtBlockHeaderFinalInfo))
}

func TestResetConsensusMessages_ShouldWork(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cmv.AddMessageTypeToPublicKey([]byte("pk1"), 1, bls.MtBlockBody)
	cmv.AddMessageTypeToPublicKey([]byte("pk1"), 1, bls.MtBlockBody)
	cmv.AddMessageTypeToPublicKey([]byte("pk1"), 2, bls.MtBlockBody)
	cmv.AddMessageTypeToPublicKey([]byte("pk1"), 1, bls.MtBlockHeader)
	cmv.AddMessageTypeToPublicKey([]byte("pk2"), 1, bls.MtBlockHeaderFinalInfo)

	assert.Equal(t, uint32(2), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk1"), 1, bls.MtBlockBody))
	assert.Equal(t, uint32(1), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk1"), 1, bls.MtBlockHeader))
	assert.Equal(t, uint32(1), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk1"), 2, bls.MtBlockBody))
	assert.Equal(t, uint32(1), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk2"), 1, bls.MtBlockHeaderFinalInfo))

	cmv.ResetConsensusMessages()

	assert.Equal(t, uint32(0), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk1"), 1, bls.MtBlockBody))
	assert.Equal(t, uint32(0), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk1"), 1, bls.MtBlockHeader))
	assert.Equal(t, uint32(0), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk1"), 2, bls.MtBlockBody))
	assert.Equal(t, uint32(0), cmv.GetNumOfMessageTypeForPublicKey([]byte("pk2"), 1, bls.MtBlockHeaderFinalInfo))
}
