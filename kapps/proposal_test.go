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
	mockForks := mock.NewForkControllerStub()

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
			resultValue, err := controller.ParseParamAndValidate(tt.parameter, tt.value, mockForks)
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
	mockForks := mock.NewForkControllerStub()

	// overwriting the parameter with a different type
	controller.GetActiveParameters()[int32(kapps.EnumParameter_LeaderValidatorRewardsPercentage)] = &kapps.Parameter{
		Type:  99,
		Value: []byte("5000"),
	}

	_, err := controller.ParseParamAndValidate(kapps.EnumParameter_LeaderValidatorRewardsPercentage, []byte("5000"), mockForks)
	assert.Error(t, err)

}

func TestParseParamAndValidate_MaxGasPerTX_ForkAware(t *testing.T) {
	controller := NewProposalControllerForTests()

	t.Run("FixAuditChanges enabled - uses MaxGasLimitPerTx", func(t *testing.T) {
		mockForks := mock.NewForkControllerStub()
		mockForks.FixAuditChangesValue = true

		tests := []struct {
			name    string
			value   []byte
			wantErr bool
		}{
			{"valid at min limit", []byte("50000"), false},            // core.MinGasLimit
			{"valid mid range", []byte("250000000"), false},           // 250M
			{"valid at max limit", []byte("500000000"), false},        // core.MaxGasLimitPerTx
			{"invalid below min", []byte("49999"), true},              // below MinGasLimit
			{"invalid above max", []byte("500000001"), true},          // above MaxGasLimitPerTx
			{"invalid way above max", []byte("10000000000000"), true}, // way above max
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := controller.ParseParamAndValidate(kapps.EnumParameter_MaxGasPerTX, tt.value, mockForks)
				if tt.wantErr {
					assert.Error(t, err)
					assert.Equal(t, common.ErrInvalidParameter, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("FixAuditChanges disabled - uses MaxGasBandwidthPerBatchPerSender", func(t *testing.T) {
		mockForks := mock.NewForkControllerStub()
		mockForks.FixAuditChangesValue = false

		tests := []struct {
			name    string
			value   []byte
			wantErr bool
		}{
			{"valid at min limit", []byte("50000"), false},                    // core.MinGasLimit
			{"valid mid range", []byte("25000000"), false},                    // 25M
			{"valid at old max limit", []byte("50000000"), false},             // core.MaxGasBandwidthPerBatchPerSender
			{"invalid below min", []byte("49999"), true},                      // below MinGasLimit
			{"invalid above old max", []byte("50000001"), true},               // above MaxGasBandwidthPerBatchPerSender
			{"invalid at new max (but above old)", []byte("500000000"), true}, // MaxGasLimitPerTx but invalid pre-fork
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := controller.ParseParamAndValidate(kapps.EnumParameter_MaxGasPerTX, tt.value, mockForks)
				if tt.wantErr {
					assert.Error(t, err)
					assert.Equal(t, common.ErrInvalidParameter, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestValidateConstraints_AllParameters(t *testing.T) {
	controller := NewProposalControllerForTests()
	mockForks := mock.NewForkControllerStub()
	mockForks.FixAuditChangesValue = true

	t.Run("LeaderValidatorRewardsPercentage", func(t *testing.T) {
		tests := []struct {
			name    string
			value   []byte
			wantErr bool
		}{
			{"valid zero", []byte("0"), false},
			{"valid mid range", []byte("5000"), false},
			{"valid at max (HundredPercent)", []byte("10000"), false},
			{"invalid above max", []byte("10001"), true},
			{"invalid way above max", []byte("15000"), true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := controller.ParseParamAndValidate(kapps.EnumParameter_LeaderValidatorRewardsPercentage, tt.value, mockForks)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("GasMultiplier", func(t *testing.T) {
		tests := []struct {
			name    string
			value   []byte
			wantErr bool
		}{
			{"valid minimum (1)", []byte("1"), false},
			{"valid higher value", []byte("10"), false},
			{"valid large value", []byte("1000"), false},
			{"invalid zero", []byte("0"), true},
			{"invalid negative", []byte("-1"), true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := controller.ParseParamAndValidate(kapps.EnumParameter_GasMultiplier, tt.value, mockForks)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("MaxGasPerBlock", func(t *testing.T) {
		tests := []struct {
			name    string
			value   []byte
			wantErr bool
		}{
			{"valid at min limit", []byte("50000"), false},      // core.MinGasLimit
			{"valid higher value", []byte("1500000000"), false}, // 1.5B
			{"valid large value", []byte("10000000000"), false}, // 10B
			{"invalid below min", []byte("49999"), true},
			{"invalid zero", []byte("0"), true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := controller.ParseParamAndValidate(kapps.EnumParameter_MaxGasPerBlock, tt.value, mockForks)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestIsInterfaceNil(t *testing.T) {
	var controller *kapps.ProposalController
	assert.True(t, controller.IsInterfaceNil())

	newController := NewProposalControllerForTests()
	assert.False(t, newController.IsInterfaceNil())
}

func TestExecuteScript_DefaultAndConstraint(t *testing.T) {
	mockForks := mock.NewForkControllerStub()
	controller, err := kapps.NewProposalController(mockForks)
	assert.NoError(t, err)

	// Default-initialized to an empty string (no script pending).
	param, ok := controller.GetActiveParameters()[int32(kapps.EnumParameter_ExecuteScript)]
	assert.True(t, ok)
	assert.Empty(t, param.GetValue())

	// A registered, enabled script name validates. The data layer only checks the script is
	// known and enabled; the one-time guarantee is enforced higher up against proposal history.
	_, err = controller.ParseParamAndValidate(kapps.EnumParameter_ExecuteScript, []byte(kapps.ScriptBurnKLV), mockForks)
	assert.NoError(t, err)

	// Unknown script names are rejected.
	_, err = controller.ParseParamAndValidate(kapps.EnumParameter_ExecuteScript, []byte("NotARealScript"), mockForks)
	assert.Equal(t, common.ErrInvalidParameter, err)
}

func TestScriptExecutedInHistory(t *testing.T) {
	scriptKey := int32(kapps.EnumParameter_ExecuteScript)

	// history models the persisted proposals keyed by ID.
	history := map[uint64]*kapps.ProposalData{
		1: {ProposalStatus: kapps.ProposalData_ApprovedProposal, Parameters: map[int32][]byte{scriptKey: []byte(kapps.ScriptBurnKLV)}},
		2: {ProposalStatus: kapps.ProposalData_DeniedProposal, Parameters: map[int32][]byte{scriptKey: []byte("DeniedScript")}},
	}
	load := func(id uint64) (*kapps.ProposalData, error) { return history[id], nil }

	// An approved proposal carrying the trigger counts as executed.
	executed, err := kapps.ScriptExecutedInHistory(kapps.ScriptBurnKLV, 2, load)
	assert.NoError(t, err)
	assert.True(t, executed)

	// A trigger that only appears on a denied proposal does not count.
	executed, err = kapps.ScriptExecutedInHistory("DeniedScript", 2, load)
	assert.NoError(t, err)
	assert.False(t, executed)

	// A trigger never seen in history does not count.
	executed, err = kapps.ScriptExecutedInHistory("NeverProposed", 2, load)
	assert.NoError(t, err)
	assert.False(t, executed)
}

func TestScriptPendingOrExecuted(t *testing.T) {
	scriptKey := int32(kapps.EnumParameter_ExecuteScript)

	history := map[uint64]*kapps.ProposalData{
		1: {ProposalStatus: kapps.ProposalData_ActiveProposal, Parameters: map[int32][]byte{scriptKey: []byte("PendingScript")}},
		2: {ProposalStatus: kapps.ProposalData_ApprovedProposal, Parameters: map[int32][]byte{scriptKey: []byte(kapps.ScriptBurnKLV)}},
		3: {ProposalStatus: kapps.ProposalData_DeniedProposal, Parameters: map[int32][]byte{scriptKey: []byte("DeniedScript")}},
	}
	load := func(id uint64) (*kapps.ProposalData, error) { return history[id], nil }

	// A still-active (pending) proposal blocks a new one — this is the creation guard that
	// ScriptExecutedInHistory (approved-only) would miss.
	exists, err := kapps.ScriptPendingOrExecuted("PendingScript", 3, load)
	assert.NoError(t, err)
	assert.True(t, exists)

	// An approved (executed) proposal also blocks.
	exists, err = kapps.ScriptPendingOrExecuted(kapps.ScriptBurnKLV, 3, load)
	assert.NoError(t, err)
	assert.True(t, exists)

	// A denied proposal does NOT block — the script may be proposed again.
	exists, err = kapps.ScriptPendingOrExecuted("DeniedScript", 3, load)
	assert.NoError(t, err)
	assert.False(t, exists)

	// A trigger never seen does not block.
	exists, err = kapps.ScriptPendingOrExecuted("NeverProposed", 3, load)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestExecuteScript_ScriptRegistryOneTimeFlags(t *testing.T) {
	cases := map[string]bool{
		kapps.ScriptBurnKLV: true, // one-time
	}
	for name, wantOneTime := range cases {
		def, ok := kapps.LookupScript(name)
		assert.True(t, ok, "script %s must be registered", name)
		assert.Equal(t, wantOneTime, def.OneTime, "script %s OneTime flag", name)
	}
}

func TestExecuteScript_ForkGated(t *testing.T) {
	mockForks := mock.NewForkControllerStub()
	mockForks.SetFork("ProposalScriptExecution", false)

	controller, err := kapps.NewProposalController(mockForks)
	assert.NoError(t, err)

	// When the fork is off the parameter is not initialized at all.
	_, ok := controller.GetActiveParameters()[int32(kapps.EnumParameter_ExecuteScript)]
	assert.False(t, ok)
}
