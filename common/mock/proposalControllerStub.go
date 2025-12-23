package mock

import (
	"reflect"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/kapps"
)

type ProposalControllerStub struct {
	GetActiveParametersCalled   func() map[int32]*kapps.Parameter
	GetParameterIntCalled       func(kapps.EnumParameter) int64
	GetParameterUintCalled      func(kapps.EnumParameter) uint64
	ParseParamAndValidateCalled func(parameter kapps.EnumParameter, value []byte, forks core.ForkController) (reflect.Value, error)
	UpdateParametersCalled      func(map[int32]*kapps.Parameter)
	IsInterfaceNilCalled        func() bool
}

func (stub *ProposalControllerStub) GetActiveParameters() map[int32]*kapps.Parameter {
	if stub.GetActiveParametersCalled != nil {
		return stub.GetActiveParametersCalled()
	}

	return nil
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

func (stub *ProposalControllerStub) ParseParamAndValidate(param kapps.EnumParameter, value []byte, fc core.ForkController) (reflect.Value, error) {
	if stub.ParseParamAndValidateCalled != nil {
		return stub.ParseParamAndValidateCalled(param, value, fc)
	}

	return reflect.Value{}, nil
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
