package interceptedBlocks_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
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
		ForkController:          &mock.ForkControllerStub{},
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

func TestNewInterceptedBlock_ForkControllerFailShouldErr(t *testing.T) {
	t.Parallel()

	arg := createDefaultBlockArgument()
	arg.ForkController = nil

	inBlk, err := interceptedBlocks.NewInterceptedBlock(arg)

	assert.True(t, check.IfNil(inBlk))
	assert.NotNil(t, err)
	assert.Equal(t, common.ErrNilForkController, err)
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

func TestInterceptedBlock_CheckValidity(t *testing.T) {
	tests := []struct {
		name    string
		block   *block.Block
		wantErr bool
		err     error
	}{
		{
			name:    "invalid PubKeysBitmap",
			block:   &block.Block{},
			wantErr: true,
			err:     process.ErrNilPubKeysBitmap,
		},
		{
			name: "invalid ParentHash",
			block: &block.Block{
				PubKeysBitmap: []byte{1},
			},
			wantErr: true,
			err:     common.ErrNilPreviousBlockHash,
		},
		{
			name: "invalid Signature",
			block: &block.Block{
				Header: &block.BlockHeader{
					ParentHash: []byte("prev hash"),
				},
				PubKeysBitmap: []byte{1},
			},
			wantErr: true,
			err:     common.ErrNilSignature,
		},
		{
			name: "invalid RandomSeed",
			block: &block.Block{
				Header: &block.BlockHeader{
					ParentHash: []byte("prev hash"),
				},
				Signature:     []byte("signature"),
				PubKeysBitmap: []byte{1},
			},
			wantErr: true,
			err:     common.ErrNilRandSeed,
		},
		{
			name: "invalid PrevRandomSeed",
			block: &block.Block{
				Header: &block.BlockHeader{
					ParentHash: []byte("prev hash"),
					RandSeed:   []byte("rand seed"),
				},
				Signature:     []byte("signature"),
				PubKeysBitmap: []byte{1},
			},
			wantErr: true,
			err:     common.ErrNilPrevRandSeed,
		},
		{
			name: "isEpochStarted active but slot is not equal to epochStartedSlot",
			block: &block.Block{
				Header: &block.BlockHeader{
					ParentHash:   []byte("prev hash"),
					RandSeed:     []byte("rand seed"),
					PrevRandSeed: []byte("prev"),
					IsEpochStart: true,
					Epoch:        0,
					Slot:         100,
				},
				Signature:     []byte("signature"),
				PubKeysBitmap: []byte{1},
			},
			wantErr: true,
			err:     process.ErrEpochDoesNotMatch,
		},
		{
			name: "isEpochStarted active but prevStartSlot is greater than epochStartedSlot",
			block: &block.Block{
				Header: &block.BlockHeader{
					ParentHash:         []byte("prev hash"),
					RandSeed:           []byte("rand seed"),
					PrevRandSeed:       []byte("prev"),
					IsEpochStart:       true,
					Epoch:              0,
					Slot:               10,
					PrevEpochStartSlot: 20,
				},
				Signature:     []byte("signature"),
				PubKeysBitmap: []byte{1},
			},
			wantErr: true,
			err:     process.ErrEpochDoesNotMatch,
		},
		{
			name: "isEpochStarted active should work",
			block: &block.Block{
				Header: &block.BlockHeader{
					ParentHash:         []byte("prev hash"),
					RandSeed:           []byte("rand seed"),
					PrevRandSeed:       []byte("prev"),
					IsEpochStart:       true,
					Epoch:              0,
					Slot:               10,
					PrevEpochStartSlot: 0,
				},
				Signature:     []byte("signature"),
				PubKeysBitmap: []byte{1},
			},
			wantErr: false,
			err:     nil,
		},
		{
			name: "isEpochStarted inactive but slot is lesser than is lesser than epochStartedSlot",
			block: &block.Block{
				Header: &block.BlockHeader{
					ParentHash:         []byte("prev hash"),
					RandSeed:           []byte("rand seed"),
					PrevRandSeed:       []byte("prev"),
					IsEpochStart:       false,
					Epoch:              0,
					Slot:               5,
					PrevEpochStartSlot: 0,
				},
				Signature:     []byte("signature"),
				PubKeysBitmap: []byte{1},
			},
			wantErr: true,
			err:     process.ErrEpochDoesNotMatch,
		},
		{
			name: "isEpochStarted inactive but prevStartSlot isn't zero",
			block: &block.Block{
				Header: &block.BlockHeader{
					ParentHash:         []byte("prev hash"),
					RandSeed:           []byte("rand seed"),
					PrevRandSeed:       []byte("prev"),
					IsEpochStart:       false,
					Epoch:              0,
					Slot:               15,
					PrevEpochStartSlot: 1,
				},
				Signature:     []byte("signature"),
				PubKeysBitmap: []byte{1},
			},
			wantErr: true,
			err:     process.ErrEpochDoesNotMatch,
		},
		{
			name: "isEpochStarted inactive should work",
			block: &block.Block{
				Header: &block.BlockHeader{
					ParentHash:         []byte("prev hash"),
					RandSeed:           []byte("rand seed"),
					PrevRandSeed:       []byte("prev"),
					IsEpochStart:       false,
					Epoch:              0,
					Slot:               15,
					PrevEpochStartSlot: 0,
				},
				Signature:     []byte("signature"),
				PubKeysBitmap: []byte{1},
			},
			wantErr: false,
			err:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)

			arg := createDefaultBlockArgument()
			arg.ForkController = &mock.ForkControllerStub{
				EnableSmartContractsValue: true,
			}
			arg.EpochStartTrigger = &mock.EpochStartTriggerStub{
				EpochStartSlotCalled: func() uint64 {
					return 10
				},
				PrevEpochStartSlotCalled: func() uint64 {
					return 0
				},
			}

			buff, err := arg.Marshalizer.Marshal(tt.block)
			require.Nil(err)
			arg.BlockBuff = buff

			inBlk, err := interceptedBlocks.NewInterceptedBlock(arg)
			require.Nil(err)

			err = inBlk.CheckValidity()
			if tt.wantErr {
				require.Error(err)
				assert.ErrorIs(err, tt.err)
			} else {
				require.NoError(err)
			}
		})
	}

}
