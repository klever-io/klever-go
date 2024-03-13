package mock

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/network/p2p"
)

// InterceptorStub -
type InterceptorStub struct {
	ProcessReceivedMessageCalled     func(message p2p.MessageP2P) error
	SetInterceptedDebugHandlerCalled func(handler process.InterceptedDebugger) error
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
func (is *InterceptorStub) RegisterHandler(_ func(topic string, hash []byte, data interface{})) {
}

// IsInterfaceNil returns true if there is no value under the interface
func (is *InterceptorStub) IsInterfaceNil() bool {
	return is == nil
}
