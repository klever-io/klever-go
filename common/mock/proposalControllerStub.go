package mock

import (
	"reflect"

	"github.com/klever-io/klever-go/kapps"
)

type ProposalControllerStub struct {
	GetParametersCalled       func() (kapps.ProposalParameters, error)
	GetActiveParametersCalled func() map[int32]*kapps.Parameter
	GetParameterCalled        func(kapps.EnumParameter) (reflect.Value, error)
	GetParameterIntCalled     func(kapps.EnumParameter) int64
	GetParameterUintCalled    func(kapps.EnumParameter) uint64
	UpdateParametersCalled    func(map[int32]*kapps.Parameter)
	IsInterfaceNilCalled      func() bool
}

func (stub *ProposalControllerStub) GetParameters() (kapps.ProposalParameters, error) {
	if stub.GetParametersCalled != nil {
		return stub.GetParametersCalled()
	}

	return kapps.ProposalParameters{}, nil
}

func (stub *ProposalControllerStub) GetActiveParameters() map[int32]*kapps.Parameter {
	if stub.GetActiveParametersCalled != nil {
		return stub.GetActiveParametersCalled()
	}

	return nil
}

func (stub *ProposalControllerStub) GetParameter(param kapps.EnumParameter) (reflect.Value, error) {
	if stub.GetParameterCalled != nil {
		return stub.GetParameterCalled(param)
	}

	return reflect.Value{}, nil
}

func (stub *ProposalControllerStub) GetParameterInt(param kapps.EnumParameter) int64 {
	if stub.GetParameterIntCalled != nil {
		return stub.GetParameterIntCalled(param)
	}

	return 0
}

func (stub *ProposalControllerStub) GetParameterUint(param kapps.EnumParameter) uint64 {
	if stub.GetParameterUintCalled != nil {
		return stub.GetParameterUintCalled(param)
	}

	return 0
}

func (stub *ProposalControllerStub) UpdateParameters(params map[int32]*kapps.Parameter) {
	if stub.UpdateParametersCalled != nil {
		stub.UpdateParametersCalled(params)
	}
}

func (stub *ProposalControllerStub) IsInterfaceNil() bool {
	if stub.IsInterfaceNilCalled != nil {
		return stub.IsInterfaceNilCalled()
	}

	return stub == nil
}
