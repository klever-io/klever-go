package factory

import (
	"errors"
	"os"
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
