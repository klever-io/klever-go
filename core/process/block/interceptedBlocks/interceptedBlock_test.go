package interceptedBlocks_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	cMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/interceptedBlocks"
	"github.com/klever-io/klever-go/crypto"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testMarshalizer = &mock.MarshalizerMock{}
var testHasher = mock.HasherMock{}

var hdrNonce = uint64(56)
var hdrSlot = uint64(67)
var hdrEpoch = uint32(78)

func createDefaultBlockArgument() *interceptedBlocks.ArgInterceptedBlock {
	arg := &interceptedBlocks.ArgInterceptedBlock{
		Hasher:                  testHasher,
		Marshalizer:             testMarshalizer,
		KeyGen:                  createMockKeyGen(),
		HeaderSigVerifier:       &cMock.HeaderSigVerifierStub{},
		HeaderIntegrityVerifier: &mock.HeaderIntegrityVerifierStub{},
		EpochStartTrigger:       &mock.EpochStartTriggerStub{},
	}

	blck := createMockBlock()

	arg.BlockBuff, _ = testMarshalizer.Marshal(blck)

	return arg
}

func createMockBlock() *block.Block {
	return &block.Block{
		Header: &block.BlockHeader{
			Slot:            hdrSlot,
			Nonce:           hdrNonce,
			Epoch:           hdrEpoch,
			ParentHash:      []byte("prev hash"),
			PrevRandSeed:    []byte("prev rand seed"),
			RandSeed:        []byte("rand seed"),
			ChainID:         []byte("chain ID"),
			SoftwareVersion: []byte("version"),
		},
		PubKeysBitmap: []byte{1},
		Signature:     []byte("signature"),
		TxHashes: [][]byte{
			[]byte("hash1"),
			[]byte("hash2"),
		},
	}
}

func createMockKeyGen() crypto.KeyGenerator {
	return &cryptoMock.SingleSignKeyGenMock{
		PublicKeyFromByteArrayCalled: func(b []byte) (key crypto.PublicKey, e error) {
			return &cryptoMock.SingleSignPublicKey{}, nil
		},
	}
}

//------- NewInterceptedBlock

func TestNewInterceptedBlock_NilArgumentShouldErr(t *testing.T) {
	t.Parallel()

	inBlk, err := interceptedBlocks.NewInterceptedBlock(nil)

	assert.True(t, check.IfNil(inBlk))
	assert.Equal(t, process.ErrNilArgumentStruct, err)
}

func TestNewInterceptedBlock_MarshalizerFailShouldErr(t *testing.T) {
	t.Parallel()

	arg := createDefaultBlockArgument()
	arg.BlockBuff = []byte("invalid buffer")

	inBlk, err := interceptedBlocks.NewInterceptedBlock(arg)

	assert.True(t, check.IfNil(inBlk))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "invalid character")
}

func TestNewInterceptedBlock_ShouldWork(t *testing.T) {
	t.Parallel()

	arg := createDefaultBlockArgument()
	inBlk, err := interceptedBlocks.NewInterceptedBlock(arg)

	assert.False(t, check.IfNil(inBlk))
	assert.Nil(t, err)
}

//------- CheckValidity

func TestInterceptedBlock_CheckValidityNilPubKeyBitmapShouldErr(t *testing.T) {
	t.Parallel()

	blk := createMockBlock()
	blk.PubKeysBitmap = nil
	buff, _ := testMarshalizer.Marshal(blk)

	arg := createDefaultBlockArgument()
	arg.BlockBuff = buff
	inBlk, _ := interceptedBlocks.NewInterceptedBlock(arg)

	err := inBlk.CheckValidity()

	assert.Equal(t, process.ErrNilPubKeysBitmap, err)
}

func TestInterceptedBlock_ContainsNilHashShouldErr(t *testing.T) {
	t.Parallel()

	blk := createMockBlock()
	blk.TxHashes[1] = nil
	buff, _ := testMarshalizer.Marshal(blk)

	arg := createDefaultBlockArgument()
	arg.BlockBuff = buff
	inBlk, _ := interceptedBlocks.NewInterceptedBlock(arg)

	err := inBlk.CheckValidity()

	assert.Equal(t, process.ErrNilTxHash, err)
}

func TestInterceptedBlock_CheckValidityLeaderSignatureNotCorrectShouldErr(t *testing.T) {
	t.Parallel()

	blk := createMockBlock()
	expectedError := errors.New("expected err")
	buff, _ := testMarshalizer.Marshal(blk)

	arg := createDefaultBlockArgument()
	arg.HeaderSigVerifier = &cMock.HeaderSigVerifierStub{
		VerifyRandSeedAndLeaderSignatureCalled: func(header data.HeaderHandler) error {
			return expectedError
		},
	}
	arg.EpochStartTrigger = &mock.EpochStartTriggerStub{}
	arg.BlockBuff = buff
	inBlk, _ := interceptedBlocks.NewInterceptedBlock(arg)

	err := inBlk.CheckValidity()

	assert.Equal(t, fmt.Errorf("%w : verify rand seed and leader signature for intercepted block failed",
		expectedError), err)
}

func TestInterceptedBlock_CheckValidityLeaderSignatureOkShouldWork(t *testing.T) {
	t.Parallel()

	blk := createMockBlock()
	expectedSignature := []byte("ran")
	blk.ProducerSignature = expectedSignature
	buff, _ := testMarshalizer.Marshal(blk)

	arg := createDefaultBlockArgument()
	arg.BlockBuff = buff
	inBlk, _ := interceptedBlocks.NewInterceptedBlock(arg)

	err := inBlk.CheckValidity()
	assert.Nil(t, err)
}

func TestInterceptedBlock_ShouldWork(t *testing.T) {
	t.Parallel()

	arg := createDefaultBlockArgument()
	inBlk, _ := interceptedBlocks.NewInterceptedBlock(arg)

	err := inBlk.CheckValidity()

	assert.Nil(t, err)
}

//------- Getters

func TestInterceptedBlock_Getters(t *testing.T) {
	t.Parallel()

	arg := createDefaultBlockArgument()
	inBlk, err := interceptedBlocks.NewInterceptedBlock(arg)
	assert.Nil(t, err)

	var blck block.Block
	err = arg.Marshalizer.Unmarshal(&blck, arg.BlockBuff)
	require.Nil(t, err)

	header, _ := arg.Marshalizer.Marshal(blck.Header)

	hash := testHasher.Compute(string(header))

	assert.Equal(t, hash, inBlk.Hash())
	assert.Equal(t, createMockBlock(), inBlk.Block())
}

//------- IsInterfaceNil

func TestInterceptedBlock_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	var inBlk *interceptedBlocks.InterceptedBlock

	assert.True(t, check.IfNil(inBlk))
}
