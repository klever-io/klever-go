package mock

import "github.com/klever-io/klever-go/data"

// HeadersCacherStub -
type HeadersCacherStub struct {
	AddCalled                 func(headerHash []byte, header data.HeaderHandler)
	RemoveHeaderByHashCalled  func(headerHash []byte)
	RemoveHeaderByNonceCalled func(hdrNonce uint64)
	GetHeaderByNonceCalled    func(hdrNonce uint64) ([]data.HeaderHandler, [][]byte, error)
	GetHeaderByHashCalled     func(hash []byte) (data.HeaderHandler, error)
	ClearCalled               func()
	RegisterHandlerCalled     func(handler func(header data.HeaderHandler, shardHeaderHash []byte))
	NoncesCalled              func() []uint64
	LenCalled                 func() int
	MaxSizeCalled             func() int
	GetNumHeadersCalled       func() int
}

// AddHeader -
func (hcs *HeadersCacherStub) AddHeader(headerHash []byte, header data.HeaderHandler) {
	if hcs.AddCalled != nil {
		hcs.AddCalled(headerHash, header)
	}
}

// RemoveHeaderByHash -
func (hcs *HeadersCacherStub) RemoveHeaderByHash(headerHash []byte) {
	if hcs.RemoveHeaderByHashCalled != nil {
		hcs.RemoveHeaderByHashCalled(headerHash)
	}
}

// RemoveHeaderByNonce -
func (hcs *HeadersCacherStub) RemoveHeaderByNonce(hdrNonce uint64) {
	if hcs.RemoveHeaderByNonceCalled != nil {
		hcs.RemoveHeaderByNonceCalled(hdrNonce)
	}
}

// GetHeadersByNonce -
func (hcs *HeadersCacherStub) GetHeadersByNonce(hdrNonce uint64) ([]data.HeaderHandler, [][]byte, error) {
	if hcs.GetHeaderByNonceCalled != nil {
		return hcs.GetHeaderByNonceCalled(hdrNonce)
	}
	return nil, nil, nil
}

// GetHeaderByHash -
func (hcs *HeadersCacherStub) GetHeaderByHash(hash []byte) (data.HeaderHandler, error) {
	if hcs.GetHeaderByHashCalled != nil {
		return hcs.GetHeaderByHashCalled(hash)
	}
	return nil, nil
}

// Clear -
func (hcs *HeadersCacherStub) Clear() {
	if hcs.ClearCalled != nil {
		hcs.ClearCalled()
	}
}

// RegisterHandler -
func (hcs *HeadersCacherStub) RegisterHandler(handler func(header data.HeaderHandler, shardHeaderHash []byte)) {
	if hcs.RegisterHandlerCalled != nil {
		hcs.RegisterHandlerCalled(handler)
	}
}

// Nonces -
func (hcs *HeadersCacherStub) Nonces() []uint64 {
	if hcs.NoncesCalled != nil {
		return hcs.NoncesCalled()
	}
	return nil
}

// Len -
func (hcs *HeadersCacherStub) Len() int {
	return 0
}

// MaxSize -
func (hcs *HeadersCacherStub) MaxSize() int {
	return 100
}

// IsInterfaceNil -
func (hcs *HeadersCacherStub) IsInterfaceNil() bool {
	return hcs == nil
}

// GetNumHeaders -
func (hcs *HeadersCacherStub) GetNumHeaders() int {
	if hcs.GetNumHeadersCalled != nil {
		return hcs.GetNumHeadersCalled()
	}

	return 0
}
