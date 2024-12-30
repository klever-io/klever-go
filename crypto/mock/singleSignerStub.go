package mock

import "github.com/klever-io/klever-go/crypto"

// SingleSignerStub -
type SingleSignerStub struct {
	SignCalled    func(private crypto.PrivateKey, msg []byte) ([]byte, error)
	VerifyCalled  func(public crypto.PublicKey, msg []byte, sig []byte) error
	SigSizeCalled func() int
}

// Sign -
func (s *SingleSignerStub) Sign(private crypto.PrivateKey, msg []byte) ([]byte, error) {
	if s.SignCalled != nil {
		return s.SignCalled(private, msg)
	}

	return nil, nil
}

// Verify -
func (s *SingleSignerStub) Verify(public crypto.PublicKey, msg []byte, sig []byte) error {
	if s.VerifyCalled != nil {
		return s.VerifyCalled(public, msg, sig)
	}

	return nil
}

// SignatureSize returns the size of the signature
func (s *SingleSignerStub) SignatureSize() int {
	if s.SigSizeCalled != nil {
		return s.SigSizeCalled()
	}

	return 0
}

// IsInterfaceNil returns true if there is no value under the interface
func (s *SingleSignerStub) IsInterfaceNil() bool {
	return s == nil
}
