package mock

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/network/p2p"
)

// enforce interface
var _ process.Interceptor = (*InterceptorStub)(nil)

// InterceptorStub -
type InterceptorStub struct {
	ProcessReceivedMessageCalled     func(message p2p.MessageP2P) error
	SetInterceptedDebugHandlerCalled func(handler process.InterceptedDebugger) error
	RegisterHandlerCalled            func(handler func(topic string, hash []byte, data interface{}))
	CloseCalled                      func() error
}

// ProcessReceivedMessage -
func (is *InterceptorStub) ProcessReceivedMessage(message p2p.MessageP2P, _ core.PeerID) error {
	return is.ProcessReceivedMessageCalled(message)
}

// SetInterceptedDebugHandler -
func (is *InterceptorStub) SetInterceptedDebugHandler(handler process.InterceptedDebugger) error {
	if is.SetInterceptedDebugHandlerCalled != nil {
		return is.SetInterceptedDebugHandlerCalled(handler)
	}

	return nil
}

// RegisterHandler -
func (is *InterceptorStub) RegisterHandler(handler func(topic string, hash []byte, data interface{})) {
	if is.RegisterHandlerCalled != nil {
		is.RegisterHandlerCalled(handler)
	}
}

// Close -
func (is *InterceptorStub) Close() error {
	if is.CloseCalled != nil {
		return is.CloseCalled()
	}

	return nil
}

// IsInterfaceNil -
func (is *InterceptorStub) IsInterfaceNil() bool {
	return is == nil
}
