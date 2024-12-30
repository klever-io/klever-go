package mock

import (
	"bytes"
	"errors"

	"github.com/klever-io/klever-go/crypto"
)

// SinglesignMock -
type SinglesignMock struct {
}

// Sign -
func (s *SinglesignMock) Sign(_ crypto.PrivateKey, _ []byte) ([]byte, error) {
	return []byte("signed"), nil
}

// Verify -
func (s *SinglesignMock) Verify(_ crypto.PublicKey, _ []byte, sig []byte) error {
	verSig := []byte("signed")

	if !bytes.Equal(sig, verSig) {
		return crypto.ErrSigNotValid
	}
	return nil
}

func (s *SinglesignMock) SignatureSize() int {
	return len("signed")
}

// IsInterfaceNil -
func (s *SinglesignMock) IsInterfaceNil() bool {
	return s == nil
}

// SinglesignFailMock -
type SinglesignFailMock struct {
}

// Sign -
func (s *SinglesignFailMock) Sign(_ crypto.PrivateKey, _ []byte) ([]byte, error) {
	return nil, errors.New("signing failure")
}

// Verify -
func (s *SinglesignFailMock) Verify(_ crypto.PublicKey, _ []byte, _ []byte) error {
	return errors.New("signature verification failure")
}

func (s *SinglesignFailMock) SignatureSize() int {
	return 0
}

// IsInterfaceNil -
func (s *SinglesignFailMock) IsInterfaceNil() bool {
	return s == nil
}

// SinglesignStub -
type SinglesignStub struct {
	SignCalled    func(private crypto.PrivateKey, msg []byte) ([]byte, error)
	VerifyCalled  func(public crypto.PublicKey, msg []byte, sig []byte) error
	SigSizeCalled func() int
}

// Sign -
func (s *SinglesignStub) Sign(private crypto.PrivateKey, msg []byte) ([]byte, error) {
	return s.SignCalled(private, msg)
}

// Verify -
func (s *SinglesignStub) Verify(public crypto.PublicKey, msg []byte, sig []byte) error {
	return s.VerifyCalled(public, msg, sig)
}

func (s *SinglesignStub) SignatureSize() int {
	return s.SigSizeCalled()
}

// IsInterfaceNil -
func (s *SinglesignStub) IsInterfaceNil() bool {
	return s == nil
}
