package factory

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCryptoSigningParamsLoader_NilPubKeyConverterShoulldErr(t *testing.T) {
	t.Parallel()

	cspf, err := NewCryptoSigningParamsLoader(nil, 0, "name", &mock.SuiteStub{}, false)
	require.Nil(t, cspf)
	require.Equal(t, common.ErrNilPubkeyConverter, err)
}

func TestNewCryptoSigningParamsLoader_NilSuiteShouldErr(t *testing.T) {
	t.Parallel()

	cspf, err := NewCryptoSigningParamsLoader(&mock.PubkeyConverterStub{}, 0, "name", nil, false)
	require.Nil(t, cspf)
	require.Equal(t, crypto.ErrNilSuite, err)
}

func TestNewCryptoSigningParamsLoader_OkValsShouldWork(t *testing.T) {
	t.Parallel()

	cspf, err := NewCryptoSigningParamsLoader(&mock.PubkeyConverterStub{}, 0, "name", &mock.SuiteStub{}, false)
	require.NoError(t, err)
	require.NotNil(t, cspf)
}

func TestCryptoSigningParamsLoader_Create_GetSkPkErrorsShouldErr(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("error while getting the sk and pk")
	cspf, _ := NewCryptoSigningParamsLoader(&mock.PubkeyConverterStub{}, 0, "name", &mock.SuiteStub{}, false)

	cspf.SetSkPkProviderHandler(func() ([]byte, []byte, error) {
		return nil, nil, expectedErr
	})
	cp, err := cspf.Get()
	require.Equal(t, expectedErr, err)
	require.Nil(t, cp)
}

func TestCryptoSigningParamsLoader_Create_PubKeyMissmatchShouldErr(t *testing.T) {
	t.Parallel()

	diffPubKey1, diffPubkey2 := []byte("public key1"), []byte("public key2")
	suite := &mock.SuiteStub{
		CreatePointStub: func() crypto.Point {
			return nil
		},
		CreatePointForScalarStub: func(_ crypto.Scalar) (crypto.Point, error) {
			return &mock.PointMock{
				MarshalBinaryStub: func(_, _ int) ([]byte, error) {
					return diffPubKey1, nil
				},
			}, nil
		},
		CreateScalarStub: func() crypto.Scalar {
			return &mock.ScalarMock{
				UnmarshalBinaryStub: func(bytes []byte) (int, error) {
					return 2, nil
				},
			}
		},
	}
	cspf, _ := NewCryptoSigningParamsLoader(&mock.PubkeyConverterStub{}, 0, "name", suite, false)

	cspf.SetSkPkProviderHandler(func() ([]byte, []byte, error) {
		return []byte("sk"), diffPubkey2, nil
	})
	cp, err := cspf.Get()
	require.Equal(t, ErrPublicKeyMismatch, err)
	require.Nil(t, cp)
}

func TestCryptoSigningParamsLoader_CreateShouldWork(t *testing.T) {
	t.Parallel()

	pubKey := []byte("public key")
	suite := &mock.SuiteStub{
		CreatePointStub: func() crypto.Point {
			return nil
		},
		CreatePointForScalarStub: func(_ crypto.Scalar) (crypto.Point, error) {
			return &mock.PointMock{
				MarshalBinaryStub: func(_, _ int) ([]byte, error) {
					return pubKey, nil
				},
			}, nil
		},
		CreateScalarStub: func() crypto.Scalar {
			return &mock.ScalarMock{
				UnmarshalBinaryStub: func(bytes []byte) (int, error) {
					return 2, nil
				},
			}
		},
	}
	cspf, _ := NewCryptoSigningParamsLoader(&mock.PubkeyConverterStub{}, 0, "name", suite, false)

	cspf.SetSkPkProviderHandler(func() ([]byte, []byte, error) {
		return []byte("sk"), pubKey, nil
	})
	cp, err := cspf.Get()
	require.NoError(t, err)
	require.NotNil(t, cp)
}

var invalidStr = []byte("invalid key")

const initScalar = 10
const initPointX = 2
const initPointY = 3

func unmarshalPrivate(val []byte) (int, error) {
	if reflect.DeepEqual(invalidStr, val) {
		return 0, crypto.ErrInvalidPrivateKey
	}

	return initScalar, nil
}

func marshalPrivate(x int) ([]byte, error) {
	res := []byte(strconv.Itoa(x))
	return res, nil
}

func unmarshalPublic(val []byte) (x, y int, err error) {
	if reflect.DeepEqual(invalidStr, val) {
		return 0, 0, crypto.ErrInvalidPublicKey
	}
	return initPointX, initPointY, nil
}

func marshalPublic(x, y int) ([]byte, error) {
	resStr := strconv.Itoa(x)
	resStr += strconv.Itoa(y)
	res := []byte(resStr)

	return res, nil
}

func createScalar() crypto.Scalar {
	return &mock.ScalarMock{
		X:                   initScalar,
		UnmarshalBinaryStub: unmarshalPrivate,
		MarshalBinaryStub:   marshalPrivate,
	}
}

func createPoint() crypto.Point {
	return &mock.PointMock{
		X:                   initPointX,
		Y:                   initPointY,
		UnmarshalBinaryStub: unmarshalPublic,
		MarshalBinaryStub:   marshalPublic,
	}
}

func createKeyPair() (crypto.Scalar, crypto.Point) {
	scalar := createScalar()
	point, _ := createPoint().Mul(scalar)
	return scalar, point
}

func TestCryptoSigningParamsLoader_GetSkPk_PathNotFound_CreateNew(t *testing.T) {
	t.Parallel()

	tempDir, _ := os.MkdirTemp("", "new_pk")

	cspf, _ := NewCryptoSigningParamsLoader(
		&mock.PubkeyConverterStub{},
		0,
		tempDir+"/name.pem",
		&mock.SuiteStub{
			CreateKeyPairStub: createKeyPair,
		},
		false,
	)
	sk, pk, err := cspf.GetSkPk()
	require.Nil(t, err)
	assert.NotNil(t, sk)
	assert.NotNil(t, pk)
}

const corruptPem = "this is not a pem file at all\n"

func newLoaderFor(t *testing.T, pemPath string) *cryptoSigningParamsLoader {
	t.Helper()
	cspf, err := NewCryptoSigningParamsLoader(
		&mock.PubkeyConverterStub{},
		0,
		pemPath,
		&mock.SuiteStub{CreateKeyPairStub: createKeyPair},
		false,
	)
	require.NoError(t, err)
	return cspf
}

// SAFETY: an existing-but-unloadable pem (corrupt file, wrong KEY_PASSWORD, bad
// permissions) must NEVER be replaced by a freshly generated key. Doing so would
// destroy a validator's key material irrecoverably.
func TestKeyFileSafety_CorruptPemIsNeverOverwritten(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pem := filepath.Join(dir, "validatorKey.pem")
	require.NoError(t, os.WriteFile(pem, []byte(corruptPem), 0o600))

	_, _, err := newLoaderFor(t, pem).GetSkPk()
	require.Error(t, err, "an unloadable pem must surface an error, not be swallowed")
	require.ErrorContains(t, err, "loading validator key",
		"the error must name what failed, not surface as a bare deserialize error")

	after, rerr := os.ReadFile(pem)
	require.NoError(t, rerr)
	require.Equal(t, corruptPem, string(after),
		"CATASTROPHIC: an unloadable pem was overwritten with a generated key")
}

// Regression: the not-found check must match by error TYPE, not by searching the
// message for "no such file or directory". That text appears in every PEM error whose
// path happens to contain it, which sent a corrupt key file down the
// generate-and-replace branch and destroyed it.
func TestKeyFileSafety_NotFoundIsMatchedByTypeNotMessage(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "no such file or directory")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	pem := filepath.Join(dir, "validatorKey.pem")
	require.NoError(t, os.WriteFile(pem, []byte(corruptPem), 0o600))

	_, _, err := newLoaderFor(t, pem).GetSkPk()
	require.Error(t, err, "a corrupt pem must fail even when its path contains the not-found text")

	after, rerr := os.ReadFile(pem)
	require.NoError(t, rerr)
	require.Equal(t, corruptPem, string(after),
		"CATASTROPHIC: a corrupt pem on a path containing the not-found text was replaced")
}

// The intentional observer path writes the generated key to disk. GetSkPk_PathNotFound
// above pins the returned key pair; this pins CreateWallet's side effect, so removing
// the write would not pass unnoticed.
func TestKeyFileSafety_MissingPemWritesKeyFile(t *testing.T) {
	t.Parallel()

	pem := filepath.Join(t.TempDir(), "validatorKey.pem")

	_, _, err := newLoaderFor(t, pem).GetSkPk()
	require.NoError(t, err, "observer auto-generation must keep working")

	info, serr := os.Stat(pem)
	require.NoError(t, serr, "a key file should have been created")
	require.Greater(t, info.Size(), int64(0))
}
