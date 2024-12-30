package mock

import (
	"github.com/klever-io/klever-go/crypto"
)

// SignerMock -
type SignerMock struct {
	SignStub    func(private crypto.PrivateKey, msg []byte) ([]byte, error)
	VerifyStub  func(public crypto.PublicKey, msg []byte, sig []byte) error
	SigSizeStub func() int
}

// Sign -
func (s *SignerMock) Sign(private crypto.PrivateKey, msg []byte) ([]byte, error) {
	return s.SignStub(private, msg)
}

// Verify -
func (s *SignerMock) Verify(public crypto.PublicKey, msg []byte, sig []byte) error {
	return s.VerifyStub(public, msg, sig)
}

// SignatureSize returns the size of the signature
func (s *SignerMock) SignatureSize() int {
	if s.SigSizeStub != nil {
		return s.SigSizeStub()
	}

	return 0
}

// IsInterfaceNil returns true if there is no value under the interface
func (s *SignerMock) IsInterfaceNil() bool {
	return s == nil
}
