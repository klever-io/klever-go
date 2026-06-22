package interceptedBlocks_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/consensus"
	cMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/interceptedBlocks"
	"github.com/klever-io/klever-go/core/process/headerCheck"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/crypto/hashing/blake2b"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/crypto/signing"
	"github.com/klever-io/klever-go/crypto/signing/mcl"
	llsig "github.com/klever-io/klever-go/crypto/signing/mcl/multisig"
	"github.com/klever-io/klever-go/crypto/signing/mcl/singlesig"
	"github.com/klever-io/klever-go/crypto/signing/multisig"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/storage/storageUnit"
	"github.com/klever-io/klever-go/tools"
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

// Regression test for GHSA-rm5c-5x2p-48wr (block variant): a nil Header must not panic
// Identifiers()/String() (they run before CheckValidity, which still rejects the block).
func TestInterceptedBlock_NilHeaderIdentifiersAndStringShouldNotPanic(t *testing.T) {
	t.Parallel()

	arg := createDefaultBlockArgument()
	// Use the proto marshaler to mirror the real gossip/direct-send wire path. Header
	// omitted — stays nil after unmarshal, mirroring the PoC payload.
	protoMarshalizer := &mock.ProtoMarshalizerMock{}
	arg.Marshalizer = protoMarshalizer
	arg.BlockBuff, _ = protoMarshalizer.Marshal(&block.Block{PubKeysBitmap: []byte{1}})

	inBlk, err := interceptedBlocks.NewInterceptedBlock(arg)
	require.Nil(t, err)
	require.False(t, check.IfNil(inBlk))

	require.NotPanics(t, func() {
		_ = inBlk.Identifiers()
		_ = inBlk.String()
	})

	// CheckValidity must still reject the malformed (nil-header) block.
	err = inBlk.CheckValidity()
	require.Error(t, err)
	assert.ErrorIs(t, err, common.ErrNilPreviousBlockHash)
}

// Regression guard for KLR-04 / KLC-2477 (GHSA-f9h7-4mmq-vgcq): InterceptedBlock.CheckValidity
// must surface a padding-bitmap rejection raised by the header signature verifier instead of
// swallowing it. This is the wrapper-level unit test (verifier stubbed); the full real-BLS
// end-to-end proof (a forged single-signer block with a padded bitmap being rejected through the
// production HeaderSigVerifier) lives in
// TestInterceptedBlock_RealBLSPaddingBitmapForgedQuorumRejected below.
func TestInterceptedBlock_PaddingBitmapForgedQuorumRejectedByCheckValidity(t *testing.T) {
	t.Parallel()

	// Forged block: bitmap 0x01,0xfe (bit 0 real + padding bits 9..15 set).
	blk := createMockBlock()
	blk.PubKeysBitmap = []byte{0x01, 0xfe}
	buff, _ := testMarshalizer.Marshal(blk)

	arg := createDefaultBlockArgument()
	arg.BlockBuff = buff
	arg.HeaderSigVerifier = &cMock.HeaderSigVerifierStub{
		VerifySignatureCalled: verifyBitmapPaddingFor(9),
	}

	inBlk, err := interceptedBlocks.NewInterceptedBlock(arg)
	require.NoError(t, err)
	require.ErrorIs(t, inBlk.CheckValidity(), headerCheck.ErrBitmapWithPaddingNotZero)
}

// Counterpart to the padding rejection guard: a canonical bitmap (correctly sized, every padding
// bit cleared) must pass InterceptedBlock.CheckValidity. The verifier stub applies the exact same
// padding rule used in the rejection test, so the accept/reject outcome turns purely on the
// padding bits - here 0x01,0x00 for a 9-validator group (bit 0 real, bits 9..15 zero).
func TestInterceptedBlock_CanonicalBitmapPassesCheckValidity(t *testing.T) {
	t.Parallel()

	blk := createMockBlock()
	blk.PubKeysBitmap = []byte{0x01, 0x00}
	buff, _ := testMarshalizer.Marshal(blk)

	arg := createDefaultBlockArgument()
	arg.BlockBuff = buff
	arg.HeaderSigVerifier = &cMock.HeaderSigVerifierStub{
		VerifySignatureCalled: verifyBitmapPaddingFor(9),
	}

	inBlk, err := interceptedBlocks.NewInterceptedBlock(arg)
	require.NoError(t, err)
	require.NoError(t, inBlk.CheckValidity())
}

// verifyBitmapPaddingFor mirrors verifyConsensusSize's padding rule for a consensus group of the
// given size: it rejects bitmaps whose padding bits (positions >= consensusSize) are set and
// accepts canonical bitmaps. Used to drive InterceptedBlock wrapper tests without real crypto.
func verifyBitmapPaddingFor(consensusSize int) func(header data.HeaderHandler) error {
	return func(header data.HeaderHandler) error {
		bitmap := header.GetPubKeysBitmap()
		if remainder := consensusSize % 8; remainder != 0 {
			allowedLastByteMask := byte((1 << uint(remainder)) - 1)
			if bitmap[len(bitmap)-1]&^allowedLastByteMask != 0 {
				return headerCheck.ErrBitmapWithPaddingNotZero
			}
		}
		return nil
	}
}

// TestInterceptedBlock_RealBLSPaddingBitmapForgedQuorumRejected is the full real-BLS end-to-end
// regression for KLR-04 / KLC-2477 (GHSA-f9h7-4mmq-vgcq). It drives the production
// HeaderSigVerifier (real BLS12-381 keygen / signature shares / aggregation / verify; only the
// NodesCoordinator is a data stub) through InterceptedBlock.CheckValidity, the same entrypoint a
// received block hits.
//
// The attack it reproduces: a malicious slot leader collects only its own (validator 0) signature
// share, but sets the unused padding bits 9..15 in the final bitmap byte so the naive
// ones-count quorum would read 8 >= PBFT threshold 7. The leader rand seed and leader signature
// are genuine (the attacker really is the leader), so the block clears those gates and reaches
// aggregate verification. Before the fix this block was accepted; now verifyConsensusSize rejects
// the non-zero padding before any quorum count, so CheckValidity must fail with
// ErrBitmapWithPaddingNotZero.
func TestInterceptedBlock_RealBLSPaddingBitmapForgedQuorumRejected(t *testing.T) {
	t.Parallel()

	const consensusSize = 9
	require.Equal(t, 7, consensus.GetPBFTThreshold(consensusSize))

	marshalizer := testMarshalizer
	hasher := &blake2b.Blake2b{}
	keyGen := signing.NewKeyGenerator(mcl.NewSuiteBLS12())
	singleSigner := singlesig.NewBlsSigner()
	lowLevelSigner := &llsig.BlsMultiSigner{Hasher: &blake2b.Blake2b{HashSize: multisig.BlsHashSize}}

	privateKeys := make([]crypto.PrivateKey, consensusSize)
	publicKeys := make([]string, consensusSize)
	validators := make([]sharding.Validator, consensusSize)
	for i := range consensusSize {
		sk, pk := keyGen.GeneratePair()
		privateKeys[i] = sk
		pubKeyBytes, err := pk.ToByteArray()
		require.NoError(t, err)
		publicKeys[i] = string(pubKeyBytes)
		validators[i], err = sharding.NewValidator(
			fmt.Appendf(nil, "owner-%d", i),
			pubKeyBytes,
			1,
			uint32(i),
		)
		require.NoError(t, err)
	}

	nodesCoordinator := &mock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(_ []byte, _ uint64, _ uint32) ([]sharding.Validator, error) {
			return validators, nil
		},
	}

	header := &block.BlockHeader{
		Nonce:           1,
		Slot:            1,
		Epoch:           0,
		ParentHash:      []byte("parent"),
		PrevRandSeed:    []byte("previous-rand-seed"),
		ChainID:         []byte("chain"),
		SoftwareVersion: []byte("version"),
	}
	forged := &block.Block{Header: header}

	// Genuine leader (validator 0) rand seed and producer signature - these gates must pass so the
	// block reaches aggregate verification, where the padding is caught.
	var err error
	header.RandSeed, err = singleSigner.Sign(privateKeys[0], header.PrevRandSeed)
	require.NoError(t, err)
	leaderSignedBytes, err := marshalizer.Marshal(forged.GetBlockHeader())
	require.NoError(t, err)
	forged.ProducerSignature, err = singleSigner.Sign(privateKeys[0], leaderSignedBytes)
	require.NoError(t, err)

	headerHash, err := tools.CalculateHash(marshalizer, hasher, forged.GetBlockHeader())
	require.NoError(t, err)

	// Aggregate is built over the single genuine signer (canonical bitmap), exactly the signature
	// a one-signer block carries. The forgery lives only in the header bitmap below.
	multiSigner, err := multisig.NewBLSMultisig(lowLevelSigner, publicKeys, privateKeys[0], keyGen, 0)
	require.NoError(t, err)
	_, err = multiSigner.CreateSignatureShare(headerHash, nil)
	require.NoError(t, err)
	aggregateSignature, err := multiSigner.AggregateSigs([]byte{0x01, 0x00})
	require.NoError(t, err)

	// Forge the quorum: bit 0 is the real proposer, bits 9..15 are padding set to 1 so the naive
	// ones-count reads 8 (>= threshold 7) for a single genuine signer.
	forged.Signature = aggregateSignature
	forged.PubKeysBitmap = []byte{0x01, 0xfe}

	hdrSigVerifier, err := headerCheck.NewHeaderSigVerifier(&headerCheck.ArgsHeaderSigVerifier{
		Marshalizer:             marshalizer,
		Hasher:                  hasher,
		NodesCoordinator:        nodesCoordinator,
		MultiSigVerifier:        multiSigner,
		SingleSigVerifier:       singleSigner,
		KeyGen:                  keyGen,
		FallbackHeaderValidator: &mock.FallBackHeaderValidatorStub{},
	})
	require.NoError(t, err)

	blockBuff, err := marshalizer.Marshal(forged)
	require.NoError(t, err)

	arg := createDefaultBlockArgument()
	arg.Hasher = hasher
	arg.BlockBuff = blockBuff
	arg.HeaderSigVerifier = hdrSigVerifier
	arg.HeaderIntegrityVerifier = newRealHeaderIntegrityVerifier(t)

	inBlk, err := interceptedBlocks.NewInterceptedBlock(arg)
	require.NoError(t, err)
	require.ErrorIs(t, inBlk.CheckValidity(), headerCheck.ErrBitmapWithPaddingNotZero)
}

// newRealHeaderIntegrityVerifier builds the production HeaderIntegrityVerifier matching the chain
// ID and software version used by the forged block in the real-BLS regression, so CheckValidity
// reaches signature verification rather than failing earlier on integrity checks.
func newRealHeaderIntegrityVerifier(t *testing.T) process.HeaderIntegrityVerifier {
	t.Helper()
	cache, err := storageUnit.NewCache(storageUnit.CacheConfig{Type: storageUnit.LRUCache, Capacity: 10})
	require.NoError(t, err)
	verifier, err := headerCheck.NewHeaderIntegrityVerifier(
		[]byte("chain"),
		[]config.VersionByEpochs{{StartEpoch: 0, Version: "version"}},
		"version",
		cache,
	)
	require.NoError(t, err)
	return verifier
}
