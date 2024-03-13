package mock

import "github.com/klever-io/klever-go/crypto"

// PrivateKeyMock mocks a private key implementation
type PrivateKeyMock struct {
	GeneratePublicMock func() crypto.PublicKey
	ToByteArrayMock    func() ([]byte, error)
	SuiteMock          func() crypto.Suite
	ScalarMock         func() crypto.Scalar
}

// PublicKeyMock mocks a public key implementation
type PublicKeyMock struct {
	ToByteArrayMock func() ([]byte, error)
	SuiteMock       func() crypto.Suite
	PointMock       func() crypto.Point
}

// GeneratePublic mocks generating a public key from the private key
func (privKey *PrivateKeyMock) GeneratePublic() crypto.PublicKey {
	return privKey.GeneratePublicMock()
}

// ToByteArray mocks converting the private key to a byte array
func (privKey *PrivateKeyMock) ToByteArray() ([]byte, error) {
	return []byte("privateKeyMock"), nil
}

// Suite -
func (privKey *PrivateKeyMock) Suite() crypto.Suite {
	return privKey.SuiteMock()
}

// Scalar -
func (privKey *PrivateKeyMock) Scalar() crypto.Scalar {
	return privKey.ScalarMock()
}

// IsInterfaceNil returns true if there is no value under the interface
func (privKey *PrivateKeyMock) IsInterfaceNil() bool {
	return privKey == nil
}

// ToByteArray mocks converting a public key to a byte array
func (pubKey *PublicKeyMock) ToByteArray() ([]byte, error) {
	return []byte("publicKeyMock"), nil
}

// Suite -
func (pubKey *PublicKeyMock) Suite() crypto.Suite {
	return pubKey.SuiteMock()
}

// Point -
func (pubKey *PublicKeyMock) Point() crypto.Point {
	return pubKey.PointMock()
}

// IsInterfaceNil returns true if there is no value under the interface
func (pubKey *PublicKeyMock) IsInterfaceNil() bool {
	return pubKey == nil
}
