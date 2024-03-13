package mock

import "github.com/klever-io/klever-go/core/process"

// WhiteListHandlerStub -
type WhiteListHandlerStub struct {
	RemoveCalled        func(keys [][]byte)
	AddCalled           func(keys [][]byte)
	IsWhiteListedCalled func(interceptedData process.InterceptedData) bool
}

// IsWhiteListed -
func (w *WhiteListHandlerStub) IsWhiteListed(interceptedData process.InterceptedData) bool {
	if w.IsWhiteListedCalled != nil {
		return w.IsWhiteListedCalled(interceptedData)
	}
	return false
}

// Remove -
func (w *WhiteListHandlerStub) Remove(keys [][]byte) {
	if w.RemoveCalled != nil {
		w.RemoveCalled(keys)
	}
}

// Add -
func (w *WhiteListHandlerStub) Add(keys [][]byte) {
	if w.AddCalled != nil {
		w.AddCalled(keys)
	}
}

// IsInterfaceNil -
func (w *WhiteListHandlerStub) IsInterfaceNil() bool {
	return w == nil
}
