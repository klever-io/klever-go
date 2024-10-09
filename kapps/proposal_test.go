package kapps_test

import (
	"reflect"
	strconv "strconv"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/assert"
)

func NewProposalControllerForTests() kapps.ActiveProposalController {
	mockForks := mock.NewForkControllerStub()

	controller, _ := kapps.NewProposalController(mockForks)

	return controller
}

func TestNewProposalController(t *testing.T) {
	mockForks := mock.NewForkControllerStub()

	controller, err := kapps.NewProposalController(mockForks)

	assert.NoError(t, err)
	assert.NotNil(t, controller)
	assert.NotNil(t, controller.ProposalController)
	assert.Equal(t, uint64(0), controller.ProposalCount)
	assert.NotNil(t, controller.ActiveParameters)
	assert.NotNil(t, controller.ActiveProposals)
}

func TestUpdateParameters(t *testing.T) {
	controller := NewProposalControllerForTests()

	newParams := map[int32]*kapps.Parameter{
		int32(kapps.EnumParameter_FeePerDataByte): {
			Type:  kapps.EnumType_Int64,
			Value: []byte("5000"),
		},
	}

	controller.UpdateParameters(newParams)
	updatedParam := controller.GetParameterUint(kapps.EnumParameter_FeePerDataByte)
	assert.Equal(t, uint64(5000), updatedParam)

	// update parameter not registered
	controller.GetActiveParameters()[int32(kapps.EnumParameter_FeePerDataByte)] = nil

	updatedParam = controller.GetParameterUint(kapps.EnumParameter_FeePerDataByte)
	assert.Equal(t, uint64(0), updatedParam)

	// update the same parameter with a different value
	newParams = map[int32]*kapps.Parameter{
		int32(kapps.EnumParameter_FeePerDataByte): {
			Type:  kapps.EnumType_Int64,
			Value: []byte("6000"),
		},
	}

	controller.UpdateParameters(newParams)

	updatedParam = controller.GetParameterUint(kapps.EnumParameter_FeePerDataByte)
	assert.Equal(t, uint64(6000), updatedParam)
}

func TestGetParameterInt(t *testing.T) {
	controller := NewProposalControllerForTests()
	value := controller.GetParameterInt(kapps.EnumParameter_FeePerDataByte)

	assert.Equal(t, int64(4000), value)

	// parse negative value
	activeParam := controller.GetActiveParameters()
	activeParam[int32(kapps.EnumParameter_FeePerDataByte)] = &kapps.Parameter{
		Type:  kapps.EnumType_Int64,
		Value: []byte("-1000"),
	}
	assert.Equal(t, int64(-1000), controller.GetParameterInt(kapps.EnumParameter_FeePerDataByte))

	// update param if it doesn't exist
	delete(activeParam, int32(kapps.EnumParameter_FeePerDataByte))
	assert.Equal(t,
		int64(0),
		controller.GetParameterInt(kapps.EnumParameter_FeePerDataByte),
	)

	// parse error
	activeParam[int32(kapps.EnumParameter_FeePerDataByte)] = &kapps.Parameter{
		Type:  kapps.EnumType_Int64,
		Value: []byte("invalid"),
	}
	assert.Equal(t, int64(0), controller.GetParameterInt(kapps.EnumParameter_FeePerDataByte))

	// parse invalid type
	activeParam[int32(kapps.EnumParameter_FeePerDataByte)] = &kapps.Parameter{
		Type:  99,
		Value: []byte("5000"),
	}
	assert.Equal(t, int64(0), controller.GetParameterInt(kapps.EnumParameter_FeePerDataByte))
}

func TestGetParameterUint(t *testing.T) {
	controller := NewProposalControllerForTests()

	value := controller.GetParameterUint(kapps.EnumParameter_LeaderValidatorRewardsPercentage)
	assert.Equal(t, uint64(6000), value)

	// update param if it doesn't exist
	activeParam := controller.GetActiveParameters()
	delete(activeParam, int32(kapps.EnumParameter_LeaderValidatorRewardsPercentage))
	// should be zero as it doesn't exist
	assert.Equal(t,
		uint64(0),
		controller.GetParameterUint(kapps.EnumParameter_LeaderValidatorRewardsPercentage),
	)

	// parse error
	activeParam[int32(kapps.EnumParameter_LeaderValidatorRewardsPercentage)] = &kapps.Parameter{
		Type:  kapps.EnumType_Int64,
		Value: []byte("invalid"),
	}
	assert.Equal(t, int64(0), controller.GetParameterInt(kapps.EnumParameter_LeaderValidatorRewardsPercentage))

	// parse invalid type
	activeParam[int32(kapps.EnumParameter_LeaderValidatorRewardsPercentage)] = &kapps.Parameter{
		Type:  99,
		Value: []byte("5000"),
	}
	assert.Equal(t, int64(0), controller.GetParameterInt(kapps.EnumParameter_LeaderValidatorRewardsPercentage))
}

func TestParseParamAndValidate(t *testing.T) {
	controller := NewProposalControllerForTests()

	tests := []struct {
		name        string
		parameter   kapps.EnumParameter
		value       []byte
		resultValue interface{}
		wantErr     bool
		err         error
	}{
		{"Valid LeaderValidatorRewardsPercentage", kapps.EnumParameter_LeaderValidatorRewardsPercentage, []byte("5000"), uint64(5000), false, nil},
		{"Invalid LeaderValidatorRewardsPercentage", kapps.EnumParameter_LeaderValidatorRewardsPercentage, []byte("15000"), nil, true, common.ErrInvalidParameter},
		{"Valid GasMultiplier", kapps.EnumParameter_GasMultiplier, []byte("5"), int64(5), false, nil},
		{"Invalid GasMultiplier", kapps.EnumParameter_GasMultiplier, []byte("0"), nil, true, common.ErrInvalidParameter},
		{"Valid MaxGasPerBlock", kapps.EnumParameter_MaxGasPerBlock, []byte("2000000000"), int64(2000000000), false, nil},
		{"Invalid MaxGasPerBlock", kapps.EnumParameter_MaxGasPerBlock, []byte("10"), nil, true, common.ErrInvalidParameter},
		{"Valid MaxGasPerTX", kapps.EnumParameter_MaxGasPerTX, []byte("50000000"), int64(50000000), false, nil},
		{"Invalid MaxGasPerTX (too low)", kapps.EnumParameter_MaxGasPerTX, []byte("10"), nil, true, common.ErrInvalidParameter},
		{"Invalid MaxGasPerTX (too high)", kapps.EnumParameter_MaxGasPerTX, []byte("10000000000000"), nil, true, common.ErrInvalidParameter},
		{"Invalid Parameter", 999, []byte("5000"), nil, true, common.ErrInvalidParameter},
		{"Error parsing value", kapps.EnumParameter_MaxGasPerTX, []byte("invalid"), nil, true, &strconv.NumError{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultValue, err := controller.ParseParamAndValidate(tt.parameter, tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				if _, ok := tt.err.(*strconv.NumError); ok {
					assert.IsType(t, &strconv.NumError{}, err)
				} else {
					assert.Equal(t, tt.err, err)
				}
			} else {

				assert.NoError(t, err)
				rvValue := resultValue.Interface()
				assert.True(t, reflect.DeepEqual(rvValue, tt.resultValue))
			}
		})
	}
}

func TestParseParamAndValidate_InvalidTypes(t *testing.T) {
	controller := NewProposalControllerForTests()

	// overwriting the parameter with a different type
	controller.GetActiveParameters()[int32(kapps.EnumParameter_LeaderValidatorRewardsPercentage)] = &kapps.Parameter{
		Type:  99,
		Value: []byte("5000"),
	}

	_, err := controller.ParseParamAndValidate(kapps.EnumParameter_LeaderValidatorRewardsPercentage, []byte("5000"))
	assert.Error(t, err)

}

func TestIsInterfaceNil(t *testing.T) {
	var controller *kapps.ProposalController
	assert.True(t, controller.IsInterfaceNil())

	newController := NewProposalControllerForTests()
	assert.False(t, newController.IsInterfaceNil())
}
