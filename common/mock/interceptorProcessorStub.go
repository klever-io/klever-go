package mock

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
)

// InterceptorProcessorStub -
type InterceptorProcessorStub struct {
	ValidateCalled        func(data process.InterceptedData) error
	SaveCalled            func(data process.InterceptedData) error
	NotifyCalled          func(data process.InterceptedData) error
	RegisterHandlerCalled func(handler func(topic string, hash []byte, data interface{}))
}

// Validate -
func (ips *InterceptorProcessorStub) Validate(data process.InterceptedData, _ core.PeerID) error {
	return ips.ValidateCalled(data)
}

// Save -
func (ips *InterceptorProcessorStub) Save(data process.InterceptedData, _ core.PeerID, _ string) error {
	return ips.SaveCalled(data)
}

// Save -
func (ips *InterceptorProcessorStub) Notify(data process.InterceptedData, _ core.PeerID, _ string) error {
	return ips.NotifyCalled(data)
}

// RegisterHandler -
func (ips *InterceptorProcessorStub) RegisterHandler(handler func(topic string, hash []byte, data interface{})) {
	if ips.RegisterHandlerCalled != nil {
		ips.RegisterHandlerCalled(handler)
	}
}

// IsInterfaceNil -
func (ips *InterceptorProcessorStub) IsInterfaceNil() bool {
	return ips == nil
}
