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

	// canonicalBitmap is a correctly sized bitmap (2 bytes for the 9-validator group) with every
	// padding bit (positions 9..15) cleared, so it passes the size and padding gates and lets the
	// test reach the signature-size checks.
	canonicalBitmap := []byte{0x01, 0x01}
	cnsMsg := &consensus.Message{PubKeysBitmap: canonicalBitmap, AggregateSignature: []byte("0")}
	err := cmv.CheckMessageWithFinalInfoValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidSignatureSize))
}

func TestCheckMessageWithFinalInfoValidity_InvalidLeaderSignatureSize(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	sig := make([]byte, SignatureSize)
	_, _ = rand.Read(sig)
	cnsMsg := &consensus.Message{PubKeysBitmap: []byte{0x01, 0x01}, AggregateSignature: sig, LeaderSignature: []byte("0")}
	err := cmv.CheckMessageWithFinalInfoValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidSignatureSize))
}

// TestCheckMessageWithFinalInfoValidity_NonZeroPaddingBits is the KLR-04 regression at the consensus
// message layer: a final-info message correctly sized but with padding bits (positions 9..15 for the
// 9-validator group) set must be rejected before the bitmap is copied into a header and verified.
func TestCheckMessageWithFinalInfoValidity_NonZeroPaddingBits(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	sig := make([]byte, SignatureSize)
	_, _ = rand.Read(sig)
	// 0xfe in the final byte sets padding bits 9..15; bit 8 (index for validator 9's slot) is unset.
	cnsMsg := &consensus.Message{PubKeysBitmap: []byte{0x01, 0xfe}, AggregateSignature: sig, LeaderSignature: sig}
	err := cmv.CheckMessageWithFinalInfoValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidPublicKeyBitmapSize))
}

func TestCheckMessageWithFinalInfoValidity_ShouldWork(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	sig := make([]byte, SignatureSize)
	_, _ = rand.Read(sig)
	cnsMsg := &consensus.Message{PubKeysBitmap: []byte{0x01, 0x01}, AggregateSignature: sig, LeaderSignature: sig}
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

func TestCheckMessageWithBlockHeaderValidity_HeaderTooBig(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, tools.MegabyteSize+1)
	_, _ = rand.Read(headerBytes)
	cnsMsg := &consensus.Message{Header: headerBytes}
	err := cmv.CheckMessageWithBlockHeaderValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidHeaderSize))
}

func TestCheckMessageWithBlockHeaderValidity_HeaderSizeZero(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, 0)
	cnsMsg := &consensus.Message{Header: headerBytes}
	err := cmv.CheckMessageWithBlockHeaderValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidHeaderSize))
}

func TestCheckMessageWithBlockHeaderValidity_InvalidMessage(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{SignatureShare: []byte("0")}
	err := cmv.CheckMessageWithBlockHeaderValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidMessage))
}

func TestCheckMessageWithBlockHeaderValidity_InvalidBodySize(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	bodyBytes := make([]byte, tools.MegabyteSize+1)
	_, _ = rand.Read(bodyBytes)
	cnsMsg := &consensus.Message{Header: bodyBytes}
	err := cmv.CheckMessageWithBlockHeaderValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidHeaderSize))
}

func TestCheckMessageWithBlockHeaderValidity_InvalidHeaderSize(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, tools.MegabyteSize+1)
	_, _ = rand.Read(headerBytes)
	cnsMsg := &consensus.Message{Header: headerBytes}
	err := cmv.CheckMessageWithBlockHeaderValidity(cnsMsg)
	assert.True(t, errors.Is(err, slot.ErrInvalidHeaderSize))
}

func TestCheckMessageWithBlockHeaderValidity_ShouldWork(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, 100)
	_, _ = rand.Read(headerBytes)
	cnsMsg := &consensus.Message{Header: headerBytes}
	err := cmv.CheckMessageWithBlockHeaderValidity(cnsMsg)
	assert.Nil(t, err)
}

func TestCheckConsensusMessageValidityForMessageType_MessageWithBlockHeaderInvalid(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{MsgType: int64(bls.MtBlockHeader), SignatureShare: []byte("1")}
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

func TestCheckConsensusMessageValidity_InvalidMessage(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	cnsMsg := &consensus.Message{ChainID: chainID, MsgType: int64(bls.MtBlockHeader), SignatureShare: []byte("1")}
	err := cmv.CheckConsensusMessageValidity(cnsMsg, "")
	assert.True(t, errors.Is(err, slot.ErrInvalidMessage))
}

func TestCheckConsensusMessageValidity_InvalidHeaderHashSize(t *testing.T) {
	t.Parallel()

	consensusMessageValidatorArgs := createDefaultConsensusMessageValidatorArgs()
	cmv, _ := slot.NewConsensusMessageValidator(consensusMessageValidatorArgs)

	headerBytes := make([]byte, 100)
	_, _ = rand.Read(headerBytes)
	cnsMsg := &consensus.Message{ChainID: chainID, MsgType: int64(bls.MtBlockHeader), Header: headerBytes}
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
	cnsMsg := &consensus.Message{ChainID: chainID, MsgType: int64(bls.MtBlockHeader), Header: headerBytes, BlockHeaderHash: headerHash}
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
		ChainID: chainID, MsgType: int64(bls.MtBlockHeader), Header: headerBytes, BlockHeaderHash: headerHash, PubKey: pubKey,
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
		ChainID: chainID, MsgType: int64(bls.MtBlockHeader),
		Header: headerBytes, BlockHeaderHash: headerHash, PubKey: pubKey, Signature: sig,
	}
	err := cmv.CheckConsensusMessageValidity(cnsMsg, "")
	assert.True(t, errors.Is(err, slot.ErrNodeIsNotInConsensusGroup))
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
		ChainID: chainID, MsgType: int64(bls.MtBlockHeader),
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
		ChainID: chainID, MsgType: int64(bls.MtBlockHeader),
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

	cmv.AddMessageTypeToPublicKey(pubKey, 10, bls.MtBlockHeader)

	cnsMsg := &consensus.Message{
		ChainID: chainID, MsgType: int64(bls.MtBlockHeader),
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
		ChainID: chainID, MsgType: int64(bls.MtBlockHeader),
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
		ChainID: chainID, MsgType: int64(bls.MtBlockHeader),
		Header: headerBytes, BlockHeaderHash: headerHash, PubKey: pubKey, Signature: sig, SlotIndex: 10,
	}
	err := cmv.CheckConsensusMessageValidity(cnsMsg, "")
	assert.Nil(t, err)
}
