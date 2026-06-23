package headerCheck_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/headerCheck"
	"github.com/klever-io/klever-go/crypto"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/sharding"
	"github.com/stretchr/testify/require"
)

const defaultChancesSelection = 1

func createHeaderSigVerifierArgs() *headerCheck.ArgsHeaderSigVerifier {
	return &headerCheck.ArgsHeaderSigVerifier{
		Marshalizer:             &cMock.MarshalizerMock{},
		Hasher:                  &cMock.HasherMock{},
		NodesCoordinator:        &cMock.NodesCoordinatorMock{},
		MultiSigVerifier:        cMock.NewMultiSigner(),
		SingleSigVerifier:       &cryptoMock.SignerMock{},
		KeyGen:                  &cryptoMock.SingleSignKeyGenMock{},
		FallbackHeaderValidator: &cMock.FallBackHeaderValidatorStub{},
	}
}

func TestNewHeaderSigVerifier_NilArgumentsShouldErr(t *testing.T) {
	t.Parallel()

	hdrSigVerifier, err := headerCheck.NewHeaderSigVerifier(nil)

	require.Nil(t, hdrSigVerifier)
	require.Equal(t, process.ErrNilArgumentStruct, err)
}

func TestNewHeaderSigVerifier_NilHasherShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	args.Hasher = nil
	hdrSigVerifier, err := headerCheck.NewHeaderSigVerifier(args)

	require.Nil(t, hdrSigVerifier)
	require.Equal(t, process.ErrNilHasher, err)
}

func TestNewHeaderSigVerifier_NilKeyGenShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	args.KeyGen = nil
	hdrSigVerifier, err := headerCheck.NewHeaderSigVerifier(args)

	require.Nil(t, hdrSigVerifier)
	require.Equal(t, common.ErrNilKeyGen, err)
}

func TestNewHeaderSigVerifier_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	args.Marshalizer = nil
	hdrSigVerifier, err := headerCheck.NewHeaderSigVerifier(args)

	require.Nil(t, hdrSigVerifier)
	require.Equal(t, process.ErrNilMarshalizer, err)
}

func TestNewHeaderSigVerifier_NilMultiSigShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	args.MultiSigVerifier = nil
	hdrSigVerifier, err := headerCheck.NewHeaderSigVerifier(args)

	require.Nil(t, hdrSigVerifier)
	require.Equal(t, common.ErrNilMultiSigVerifier, err)
}

func TestNewHeaderSigVerifier_NilNodesCoordinatorShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	args.NodesCoordinator = nil
	hdrSigVerifier, err := headerCheck.NewHeaderSigVerifier(args)

	require.Nil(t, hdrSigVerifier)
	require.Equal(t, common.ErrNilNodesCoordinator, err)
}

func TestNewHeaderSigVerifier_NilSingleSigShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	args.SingleSigVerifier = nil
	hdrSigVerifier, err := headerCheck.NewHeaderSigVerifier(args)

	require.Nil(t, hdrSigVerifier)
	require.Equal(t, common.ErrNilSingleSigner, err)
}

func TestNewHeaderSigVerifier_OkValsShouldWork(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	hdrSigVerifier, err := headerCheck.NewHeaderSigVerifier(args)

	require.Nil(t, err)
	require.NotNil(t, hdrSigVerifier)
	require.False(t, hdrSigVerifier.IsInterfaceNil())
}

func TestHeaderSigVerifier_VerifySignatureNilPrevRandSeedShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	header := &block.Block{Header: &block.BlockHeader{}}

	err := hdrSigVerifier.VerifyRandSeed(header)
	require.Equal(t, sharding.ErrNilRandomness, err)
}

func TestHeaderSigVerifier_VerifyRandSeedOk(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	wasCalled := false

	args.KeyGen = &cryptoMock.SingleSignKeyGenMock{
		PublicKeyFromByteArrayCalled: func(b []byte) (key crypto.PublicKey, err error) {
			return &cryptoMock.SingleSignPublicKey{}, nil
		},
	}
	args.SingleSigVerifier = &cryptoMock.SignerMock{
		VerifyStub: func(public crypto.PublicKey, msg []byte, sig []byte) error {
			wasCalled = true
			return nil
		},
	}

	pkAddr := []byte("aaa00000000000000000000000000000")
	nodesCoordinator := &cMock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			v, _ := sharding.NewValidator(pkAddr, pkAddr, 1, defaultChancesSelection)
			return []sharding.Validator{v}, nil
		},
	}
	args.NodesCoordinator = nodesCoordinator
	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	header := &block.Block{Header: &block.BlockHeader{}}

	err := hdrSigVerifier.VerifyRandSeed(header)
	require.Nil(t, err)
	require.True(t, wasCalled)
}

func TestHeaderSigVerifier_VerifyRandSeedShouldErrWhenVerificationFails(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	wasCalled := false
	localError := errors.New("err")

	args.KeyGen = &cryptoMock.SingleSignKeyGenMock{
		PublicKeyFromByteArrayCalled: func(b []byte) (key crypto.PublicKey, err error) {
			return &cryptoMock.SingleSignPublicKey{}, nil
		},
	}
	args.SingleSigVerifier = &cryptoMock.SignerMock{
		VerifyStub: func(public crypto.PublicKey, msg []byte, sig []byte) error {
			wasCalled = true
			return localError
		},
	}

	pkAddr := []byte("aaa00000000000000000000000000000")
	nodesCoordinator := &cMock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			v, _ := sharding.NewValidator(pkAddr, pkAddr, 1, defaultChancesSelection)
			return []sharding.Validator{v}, nil
		},
	}
	args.NodesCoordinator = nodesCoordinator
	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	header := &block.Block{Header: &block.BlockHeader{}}

	err := hdrSigVerifier.VerifyRandSeed(header)
	require.Equal(t, localError, err)
	require.True(t, wasCalled)
}

func TestHeaderSigVerifier_VerifyRandSeedAndLeaderSignatureNilRandomnessShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	header := &block.Block{Header: &block.BlockHeader{}}

	err := hdrSigVerifier.VerifyRandSeedAndLeaderSignature(header)
	require.Equal(t, sharding.ErrNilRandomness, err)
}

func TestHeaderSigVerifier_VerifyRandSeedAndLeaderSignatureVerifyShouldErrWhenValidationFails(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	count := 0
	localErr := errors.New("err")

	args.KeyGen = &cryptoMock.SingleSignKeyGenMock{
		PublicKeyFromByteArrayCalled: func(b []byte) (key crypto.PublicKey, err error) {
			return &cryptoMock.SingleSignPublicKey{}, nil
		},
	}
	args.SingleSigVerifier = &cryptoMock.SignerMock{
		VerifyStub: func(public crypto.PublicKey, msg []byte, sig []byte) error {
			count++
			return localErr
		},
	}

	pkAddr := []byte("aaa00000000000000000000000000000")
	nodesCoordinator := &cMock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			v, _ := sharding.NewValidator(pkAddr, pkAddr, 1, defaultChancesSelection)
			return []sharding.Validator{v}, nil
		},
	}
	args.NodesCoordinator = nodesCoordinator
	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	header := &block.Block{Header: &block.BlockHeader{}}

	err := hdrSigVerifier.VerifyRandSeedAndLeaderSignature(header)
	require.Equal(t, localErr, err)
	require.Equal(t, 1, count)
}

func TestHeaderSigVerifier_VerifyRandSeedAndLeaderSignatureVerifyLeaderSigShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	count := 0
	localErr := errors.New("err")
	leaderSig := []byte("signature")

	args.KeyGen = &cryptoMock.SingleSignKeyGenMock{
		PublicKeyFromByteArrayCalled: func(b []byte) (key crypto.PublicKey, err error) {
			return &cryptoMock.SingleSignPublicKey{}, nil
		},
	}
	args.SingleSigVerifier = &cryptoMock.SignerMock{
		VerifyStub: func(public crypto.PublicKey, msg []byte, sig []byte) error {
			count++
			if bytes.Equal(sig, leaderSig) {
				return localErr
			}
			return nil
		},
	}

	pkAddr := []byte("aaa00000000000000000000000000000")
	nodesCoordinator := &cMock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			v, _ := sharding.NewValidator(pkAddr, pkAddr, 1, defaultChancesSelection)
			return []sharding.Validator{v}, nil
		},
	}
	args.NodesCoordinator = nodesCoordinator
	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	header := &block.Block{Header: &block.BlockHeader{},
		ProducerSignature: leaderSig,
	}

	err := hdrSigVerifier.VerifyRandSeedAndLeaderSignature(header)
	require.Equal(t, localErr, err)
	require.Equal(t, 2, count)
}

func TestHeaderSigVerifier_VerifyRandSeedAndLeaderSignatureOk(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	count := 0

	args.KeyGen = &cryptoMock.SingleSignKeyGenMock{
		PublicKeyFromByteArrayCalled: func(b []byte) (key crypto.PublicKey, err error) {
			return &cryptoMock.SingleSignPublicKey{}, nil
		},
	}
	args.SingleSigVerifier = &cryptoMock.SignerMock{
		VerifyStub: func(public crypto.PublicKey, msg []byte, sig []byte) error {
			count++
			return nil
		},
	}

	pkAddr := []byte("aaa00000000000000000000000000000")
	nodesCoordinator := &cMock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			v, _ := sharding.NewValidator(pkAddr, pkAddr, 1, defaultChancesSelection)
			return []sharding.Validator{v}, nil
		},
	}
	args.NodesCoordinator = nodesCoordinator
	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	header := &block.Block{Header: &block.BlockHeader{}}

	err := hdrSigVerifier.VerifyRandSeedAndLeaderSignature(header)
	require.Nil(t, err)
	require.Equal(t, 2, count)
}

func TestHeaderSigVerifier_VerifyLeaderSignatureNilRandomnessShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	header := &block.Block{Header: &block.BlockHeader{}}

	err := hdrSigVerifier.VerifyLeaderSignature(header)
	require.Equal(t, sharding.ErrNilRandomness, err)
}

func TestHeaderSigVerifier_VerifyLeaderSignatureVerifyShouldErrWhenValidationFails(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	count := 0
	localErr := errors.New("err")

	args.KeyGen = &cryptoMock.SingleSignKeyGenMock{
		PublicKeyFromByteArrayCalled: func(b []byte) (key crypto.PublicKey, err error) {
			return &cryptoMock.SingleSignPublicKey{}, nil
		},
	}
	args.SingleSigVerifier = &cryptoMock.SignerMock{
		VerifyStub: func(public crypto.PublicKey, msg []byte, sig []byte) error {
			count++
			return localErr
		},
	}

	pkAddr := []byte("aaa00000000000000000000000000000")
	nodesCoordinator := &cMock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			v, _ := sharding.NewValidator(pkAddr, pkAddr, 1, defaultChancesSelection)
			return []sharding.Validator{v}, nil
		},
	}
	args.NodesCoordinator = nodesCoordinator
	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	header := &block.Block{Header: &block.BlockHeader{}}

	err := hdrSigVerifier.VerifyLeaderSignature(header)
	require.Equal(t, localErr, err)
	require.Equal(t, 1, count)
}

func TestHeaderSigVerifier_VerifyLeaderSignatureVerifyLeaderSigShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	count := 0
	localErr := errors.New("err")
	leaderSig := []byte("signature")

	args.KeyGen = &cryptoMock.SingleSignKeyGenMock{
		PublicKeyFromByteArrayCalled: func(b []byte) (key crypto.PublicKey, err error) {
			return &cryptoMock.SingleSignPublicKey{}, nil
		},
	}
	args.SingleSigVerifier = &cryptoMock.SignerMock{
		VerifyStub: func(public crypto.PublicKey, msg []byte, sig []byte) error {
			count++
			if bytes.Equal(sig, leaderSig) {
				return localErr
			}
			return nil
		},
	}

	pkAddr := []byte("aaa00000000000000000000000000000")
	nodesCoordinator := &cMock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			v, _ := sharding.NewValidator(pkAddr, pkAddr, 1, defaultChancesSelection)
			return []sharding.Validator{v}, nil
		},
	}
	args.NodesCoordinator = nodesCoordinator
	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	header := &block.Block{Header: &block.BlockHeader{},
		ProducerSignature: leaderSig,
	}

	err := hdrSigVerifier.VerifyLeaderSignature(header)
	require.Equal(t, localErr, err)
	require.Equal(t, 1, count)
}

func TestHeaderSigVerifier_VerifyLeaderSignatureOk(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	count := 0

	args.KeyGen = &cryptoMock.SingleSignKeyGenMock{
		PublicKeyFromByteArrayCalled: func(b []byte) (key crypto.PublicKey, err error) {
			return &cryptoMock.SingleSignPublicKey{}, nil
		},
	}
	args.SingleSigVerifier = &cryptoMock.SignerMock{
		VerifyStub: func(public crypto.PublicKey, msg []byte, sig []byte) error {
			count++
			return nil
		},
	}

	pkAddr := []byte("aaa00000000000000000000000000000")
	nodesCoordinator := &cMock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			v, _ := sharding.NewValidator(pkAddr, pkAddr, 1, defaultChancesSelection)
			return []sharding.Validator{v}, nil
		},
	}
	args.NodesCoordinator = nodesCoordinator
	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	header := &block.Block{Header: &block.BlockHeader{}}

	err := hdrSigVerifier.VerifyLeaderSignature(header)
	require.Nil(t, err)
	require.Equal(t, 1, count)
}

func TestHeaderSigVerifier_VerifySignatureNilBitmapShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	header := &block.Block{Header: &block.BlockHeader{}}

	err := hdrSigVerifier.VerifySignature(header)
	require.Equal(t, process.ErrNilPubKeysBitmap, err)
}

func TestHeaderSigVerifier_VerifySignatureBlockProposerSigMissingShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	header := &block.Block{Header: &block.BlockHeader{},
		PubKeysBitmap: []byte("0"),
	}

	err := hdrSigVerifier.VerifySignature(header)
	require.Equal(t, process.ErrBlockProposerSignatureMissing, err)
}

func TestHeaderSigVerifier_VerifySignatureNilRandomnessShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	header := &block.Block{Header: &block.BlockHeader{},
		PubKeysBitmap: []byte("1"),
	}

	err := hdrSigVerifier.VerifySignature(header)
	require.Equal(t, sharding.ErrNilRandomness, err)
}

func TestHeaderSigVerifier_VerifySignatureWrongSizeBitmapShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	pkAddr := []byte("aaa00000000000000000000000000000")
	nodesCoordinator := &cMock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			v, _ := sharding.NewValidator(pkAddr, pkAddr, 1, defaultChancesSelection)
			return []sharding.Validator{v}, nil
		},
	}
	args.NodesCoordinator = nodesCoordinator

	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	header := &block.Block{Header: &block.BlockHeader{},
		PubKeysBitmap: []byte("11"),
	}

	err := hdrSigVerifier.VerifySignature(header)
	require.Equal(t, headerCheck.ErrWrongSizeBitmap, err)
}

func TestHeaderSigVerifier_VerifySignatureNotEnoughSigsShouldErr(t *testing.T) {
	t.Parallel()

	args := createHeaderSigVerifierArgs()
	pkAddr := []byte("aaa00000000000000000000000000000")
	nodesCoordinator := &cMock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			v, _ := sharding.NewValidator(pkAddr, pkAddr, 1, defaultChancesSelection)
			return []sharding.Validator{v, v, v, v, v}, nil
		},
	}
	args.NodesCoordinator = nodesCoordinator

	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	// 5 validators (indices 0..4); bitmap 0x03 sets bits 0,1 - canonical (no padding) but
	// below the PBFT threshold, so it must fail with ErrNotEnoughSignatures.
	header := &block.Block{Header: &block.BlockHeader{},
		PubKeysBitmap: []byte{0x03},
	}

	err := hdrSigVerifier.VerifySignature(header)
	require.Equal(t, headerCheck.ErrNotEnoughSignatures, err)
}

func TestHeaderSigVerifier_VerifySignatureOk(t *testing.T) {
	t.Parallel()

	wasCalled := false
	args := createHeaderSigVerifierArgs()
	pkAddr := []byte("aaa00000000000000000000000000000")
	nodesCoordinator := &cMock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			v, _ := sharding.NewValidator(pkAddr, pkAddr, 1, defaultChancesSelection)
			return []sharding.Validator{v}, nil
		},
	}
	args.NodesCoordinator = nodesCoordinator

	args.MultiSigVerifier = &cMock.BelNevMock{
		CreateMock: func(pubKeys []string, index uint16) (signer crypto.MultiSigner, err error) {
			return &cMock.BelNevMock{
				VerifyMock: func(msg []byte, bitmap []byte) error {
					wasCalled = true
					return nil
				}}, nil
		},
	}

	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	// single validator (index 0); bitmap 0x01 sets only bit 0 - canonical, no padding bits.
	header := &block.Block{Header: &block.BlockHeader{},
		PubKeysBitmap: []byte{0x01},
	}

	err := hdrSigVerifier.VerifySignature(header)
	require.Nil(t, err)
	require.True(t, wasCalled)
}

func TestHeaderSigVerifier_VerifySignatureNotEnoughSigsShouldErrWhenFallbackThresholdCouldNotBeApplied(t *testing.T) {
	t.Parallel()

	wasCalled := false
	args := createHeaderSigVerifierArgs()
	pkAddr := []byte("aaa00000000000000000000000000000")
	nodesCoordinator := &cMock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			v, _ := sharding.NewValidator(pkAddr, pkAddr, 1, defaultChancesSelection)
			return []sharding.Validator{v, v, v, v, v}, nil
		},
	}
	fallbackHeaderValidator := &cMock.FallBackHeaderValidatorStub{
		ShouldApplyFallbackValidationCalled: func(headerHandler data.HeaderHandler) bool {
			return false
		},
	}
	multiSigVerifier := &cMock.BelNevMock{
		CreateMock: func(pubKeys []string, index uint16) (signer crypto.MultiSigner, err error) {
			return &cMock.BelNevMock{
				VerifyMock: func(msg []byte, bitmap []byte) error {
					wasCalled = true
					return nil
				}}, nil
		},
	}

	args.NodesCoordinator = nodesCoordinator
	args.FallbackHeaderValidator = fallbackHeaderValidator
	args.MultiSigVerifier = multiSigVerifier

	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	// 5 validators (indices 0..4); bitmap 0x07 sets bits 0,1,2 - canonical (no padding) but
	// below the PBFT threshold, so it must fail with ErrNotEnoughSignatures.
	header := &block.Block{Header: &block.BlockHeader{},
		PubKeysBitmap: []byte{0x07},
	}

	err := hdrSigVerifier.VerifySignature(header)
	require.Equal(t, headerCheck.ErrNotEnoughSignatures, err)
	require.False(t, wasCalled)
}

func TestHeaderSigVerifier_VerifySignature21ValidatorsPaddingBitsShouldErr(t *testing.T) {
	t.Parallel()

	wasCalled := false
	args := createHeaderSigVerifierArgs()
	pkAddr := []byte("aaa00000000000000000000000000000")
	nodesCoordinator := &cMock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			validatorsGroup := make([]sharding.Validator, 0, 21)
			for range 21 {
				v, _ := sharding.NewValidator(pkAddr, pkAddr, 1, defaultChancesSelection)
				validatorsGroup = append(validatorsGroup, v)
			}
			return validatorsGroup, nil
		},
	}
	args.NodesCoordinator = nodesCoordinator

	args.MultiSigVerifier = &cMock.BelNevMock{
		CreateMock: func(pubKeys []string, index uint16) (signer crypto.MultiSigner, err error) {
			return &cMock.BelNevMock{
				VerifyMock: func(msg []byte, bitmap []byte) error {
					wasCalled = true
					return nil
				}}, nil
		},
	}

	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	// 21 validators (indices 0..20) span 3 bytes; the last byte holds 5 real bits (positions
	// 16..20) and 3 padding bits (21..23). PBFT quorum is 21*2/3+1 = 15.
	// A malicious leader gathers only 14 real signers (bytes 0xFF, 0x3F -> bits 0..13) and sets
	// padding bit 21 (0x20) to forge the 15th "signature": the raw count reaches quorum even
	// though no real validator backs that bit. The padding guard must reject it (KLR-04).
	header := &block.Block{Header: &block.BlockHeader{},
		PubKeysBitmap: []byte{0xFF, 0x3F, 0x20},
	}

	err := hdrSigVerifier.VerifySignature(header)
	require.Equal(t, headerCheck.ErrBitmapWithPaddingNotZero, err)
	require.False(t, wasCalled)
}

func TestHeaderSigVerifier_VerifySignature21ValidatorsCanonicalBitmapOk(t *testing.T) {
	t.Parallel()

	wasCalled := false
	args := createHeaderSigVerifierArgs()
	pkAddr := []byte("aaa00000000000000000000000000000")
	nodesCoordinator := &cMock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			validatorsGroup := make([]sharding.Validator, 0, 21)
			for range 21 {
				v, _ := sharding.NewValidator(pkAddr, pkAddr, 1, defaultChancesSelection)
				validatorsGroup = append(validatorsGroup, v)
			}
			return validatorsGroup, nil
		},
	}
	args.NodesCoordinator = nodesCoordinator

	args.MultiSigVerifier = &cMock.BelNevMock{
		CreateMock: func(pubKeys []string, index uint16) (signer crypto.MultiSigner, err error) {
			return &cMock.BelNevMock{
				VerifyMock: func(msg []byte, bitmap []byte) error {
					wasCalled = true
					return nil
				}}, nil
		},
	}

	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	// 21 validators all sign: bytes 0xFF, 0xFF set bits 0..15 and 0x1F sets the 5 real bits
	// (positions 16..20) of the final byte with every padding bit (21..23) left at zero.
	// 21 signatures clears the quorum of 15 and the canonical bitmap passes the padding guard.
	header := &block.Block{Header: &block.BlockHeader{},
		PubKeysBitmap: []byte{0xFF, 0xFF, 0x1F},
	}

	err := hdrSigVerifier.VerifySignature(header)
	require.Nil(t, err)
	require.True(t, wasCalled)
}

func TestHeaderSigVerifier_VerifySignatureOkWhenFallbackThresholdCouldBeApplied(t *testing.T) {
	t.Parallel()

	wasCalled := false
	args := createHeaderSigVerifierArgs()
	pkAddr := []byte("aaa00000000000000000000000000000")
	nodesCoordinator := &cMock.NodesCoordinatorMock{
		ComputeValidatorsGroupCalled: func(randomness []byte, round uint64, epoch uint32) (validators []sharding.Validator, err error) {
			v, _ := sharding.NewValidator(pkAddr, pkAddr, 1, defaultChancesSelection)
			return []sharding.Validator{v, v, v, v, v}, nil
		},
	}
	fallbackHeaderValidator := &cMock.FallBackHeaderValidatorStub{
		ShouldApplyFallbackValidationCalled: func(headerHandler data.HeaderHandler) bool {
			return true
		},
	}
	multiSigVerifier := &cMock.BelNevMock{
		CreateMock: func(pubKeys []string, index uint16) (signer crypto.MultiSigner, err error) {
			return &cMock.BelNevMock{
				VerifyMock: func(msg []byte, bitmap []byte) error {
					wasCalled = true
					return nil
				}}, nil
		},
	}

	args.NodesCoordinator = nodesCoordinator
	args.FallbackHeaderValidator = fallbackHeaderValidator
	args.MultiSigVerifier = multiSigVerifier

	hdrSigVerifier, _ := headerCheck.NewHeaderSigVerifier(args)
	// 5 validators (indices 0..4); bitmap 0x07 sets bits 0,1,2 - canonical (no padding) and
	// enough to satisfy the lower fallback threshold.
	header := &block.Block{Header: &block.BlockHeader{},
		PubKeysBitmap: []byte{0x07},
	}

	err := hdrSigVerifier.VerifySignature(header)
	require.Nil(t, err)
	require.True(t, wasCalled)
}
