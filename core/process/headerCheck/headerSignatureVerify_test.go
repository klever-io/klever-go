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
	header := &block.Block{Header: &block.BlockHeader{},
		PubKeysBitmap: []byte("A"),
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
	header := &block.Block{Header: &block.BlockHeader{},
		PubKeysBitmap: []byte("1"),
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
	header := &block.Block{Header: &block.BlockHeader{},
		PubKeysBitmap: []byte("C"),
	}

	err := hdrSigVerifier.VerifySignature(header)
	require.Equal(t, headerCheck.ErrNotEnoughSignatures, err)
	require.False(t, wasCalled)
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
	header := &block.Block{Header: &block.BlockHeader{},
		PubKeysBitmap: []byte("C"),
	}

	err := hdrSigVerifier.VerifySignature(header)
	require.Nil(t, err)
	require.True(t, wasCalled)
}
