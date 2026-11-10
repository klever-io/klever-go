package kapps_test

import (
	"math"
	"reflect"
	strconv "strconv"
	"sync"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/assert"
)

func NewProposalControllerForTests() kapps.ActiveProposalController {
	mockForks := mock.NewForkControllerStub()

	controller, _ := kapps.NewProposalController(mockForks)

	return controller
}

// setParams edits the live set through the controller: GetActiveParameters
// hands out copies, so writing into the returned map alone changes nothing.
func setParams(controller kapps.ActiveProposalController, edit func(map[int32]*kapps.Parameter)) {
	params := controller.GetActiveParameters()
	edit(params)
	controller.RestoreParameters(params)
}

func TestNewProposalController(t *testing.T) {
	mockForks := mock.NewForkControllerStub()

	controller, err := kapps.NewProposalController(mockForks)

	assert.NoError(t, err)
	assert.NotNil(t, controller)
	pc := kapps.ProposalControllerOf(controller)
	assert.NotNil(t, pc)
	assert.Equal(t, uint64(0), pc.ProposalCount)
	assert.NotNil(t, pc.ActiveParameters)
	assert.NotNil(t, pc.ActiveProposals)
}

func TestUpdateParameters(t *testing.T) {
	controller, _ := kapps.NewProposalController(mock.NewForkControllerStub())

	newParams := map[int32]*kapps.Parameter{
		int32(kapps.EnumParameter_FeePerDataByte): {
			Type:  kapps.EnumType_Int64,
			Value: []byte("5000"),
		},
	}

	controller.UpdateParameters(newParams)
	updatedParam := controller.GetParameterUint(kapps.EnumParameter_FeePerDataByte)
	assert.Equal(t, uint64(5000), updatedParam)

	// update parameter not registered: a nil entry, as a trie-loaded controller
	// can hold. Written on the wrapped map since the copying API drops nils.
	kapps.ProposalControllerOf(controller).ActiveParameters[int32(kapps.EnumParameter_FeePerDataByte)] = nil

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
	setParams(controller, func(p map[int32]*kapps.Parameter) {
		p[int32(kapps.EnumParameter_FeePerDataByte)] = &kapps.Parameter{
			Type:  kapps.EnumType_Int64,
			Value: []byte("-1000"),
		}
	})
	assert.Equal(t, int64(-1000), controller.GetParameterInt(kapps.EnumParameter_FeePerDataByte))

	// update param if it doesn't exist
	setParams(controller, func(p map[int32]*kapps.Parameter) {
		delete(p, int32(kapps.EnumParameter_FeePerDataByte))
	})
	assert.Equal(t,
		int64(0),
		controller.GetParameterInt(kapps.EnumParameter_FeePerDataByte),
	)

	// parse error
	setParams(controller, func(p map[int32]*kapps.Parameter) {
		p[int32(kapps.EnumParameter_FeePerDataByte)] = &kapps.Parameter{
			Type:  kapps.EnumType_Int64,
			Value: []byte("invalid"),
		}
	})
	assert.Equal(t, int64(0), controller.GetParameterInt(kapps.EnumParameter_FeePerDataByte))

	// parse invalid type
	setParams(controller, func(p map[int32]*kapps.Parameter) {
		p[int32(kapps.EnumParameter_FeePerDataByte)] = &kapps.Parameter{
			Type:  99,
			Value: []byte("5000"),
		}
	})
	assert.Equal(t, int64(0), controller.GetParameterInt(kapps.EnumParameter_FeePerDataByte))
}

func TestGetParameterUint(t *testing.T) {
	controller := NewProposalControllerForTests()

	value := controller.GetParameterUint(kapps.EnumParameter_LeaderValidatorRewardsPercentage)
	assert.Equal(t, uint64(6000), value)

	// update param if it doesn't exist
	setParams(controller, func(p map[int32]*kapps.Parameter) {
		delete(p, int32(kapps.EnumParameter_LeaderValidatorRewardsPercentage))
	})
	// should be zero as it doesn't exist
	assert.Equal(t,
		uint64(0),
		controller.GetParameterUint(kapps.EnumParameter_LeaderValidatorRewardsPercentage),
	)

	// parse error
	setParams(controller, func(p map[int32]*kapps.Parameter) {
		p[int32(kapps.EnumParameter_LeaderValidatorRewardsPercentage)] = &kapps.Parameter{
			Type:  kapps.EnumType_Int64,
			Value: []byte("invalid"),
		}
	})
	assert.Equal(t, int64(0), controller.GetParameterInt(kapps.EnumParameter_LeaderValidatorRewardsPercentage))

	// parse invalid type
	setParams(controller, func(p map[int32]*kapps.Parameter) {
		p[int32(kapps.EnumParameter_LeaderValidatorRewardsPercentage)] = &kapps.Parameter{
			Type:  99,
			Value: []byte("5000"),
		}
	})
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
	setParams(controller, func(p map[int32]*kapps.Parameter) {
		p[int32(kapps.EnumParameter_LeaderValidatorRewardsPercentage)] = &kapps.Parameter{
			Type:  99,
			Value: []byte("5000"),
		}
	})

	_, err := controller.ParseParamAndValidate(kapps.EnumParameter_LeaderValidatorRewardsPercentage, []byte("5000"), mockForks)
	assert.Error(t, err)
}

// GetActiveParameters hands out copies: neither editing an entry nor dropping
// one from the returned map may reach the live set.
func TestActiveProposalController_ReturnsCopies(t *testing.T) {
	t.Parallel()

	controller := NewProposalControllerForTests()
	feeKey := int32(kapps.EnumParameter_FeePerDataByte)
	rewardsKey := int32(kapps.EnumParameter_LeaderValidatorRewardsPercentage)

	leaked := controller.GetActiveParameters()
	leaked[feeKey].Value[0] = 'X'
	leaked[feeKey] = nil
	delete(leaked, rewardsKey)

	assert.Equal(t, int64(4000), controller.GetParameterInt(kapps.EnumParameter_FeePerDataByte))
	assert.Equal(t, uint64(6000), controller.GetParameterUint(kapps.EnumParameter_LeaderValidatorRewardsPercentage))

	fresh := controller.GetActiveParameters()
	assert.Equal(t, []byte("4000"), fresh[feeKey].Value)
	assert.Contains(t, fresh, rewardsKey)

	// the same holds for what is passed in: the caller keeps ownership
	incoming := map[int32]*kapps.Parameter{feeKey: {Type: kapps.EnumType_Int64, Value: []byte("7000")}}
	controller.UpdateParameters(incoming)
	incoming[feeKey].Value[0] = '9'
	assert.Equal(t, int64(7000), controller.GetParameterInt(kapps.EnumParameter_FeePerDataByte))

	controller.RestoreParameters(fresh)
	fresh[feeKey].Value[0] = '9'
	assert.Equal(t, int64(4000), controller.GetParameterInt(kapps.EnumParameter_FeePerDataByte))
}

// KLR-18 residual: the epoch start rollback rewrites the live set while API and
// fee readers run on other goroutines. Every reader must see a complete set,
// never a half-restored one. Meaningful under -race.
func TestActiveProposalController_ConcurrentReadersDuringRestore(t *testing.T) {
	t.Parallel()

	controller := NewProposalControllerForTests()
	mockForks := mock.NewForkControllerStub()
	feeKey := int32(kapps.EnumParameter_FeePerDataByte)

	initial := controller.GetActiveParameters()
	settled := controller.GetActiveParameters()
	settled[feeKey] = &kapps.Parameter{Type: kapps.EnumType_Int64, Value: []byte("1234")}
	// a settled proposal may also introduce a key the snapshot never had
	settled[999] = &kapps.Parameter{Type: kapps.EnumType_Int64, Value: []byte("1")}

	done := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}

				// API read: one copy is either the initial or the settled set, never a mix
				params := controller.GetActiveParameters()
				assert.GreaterOrEqual(t, len(params), len(initial))
				fee := string(params[feeKey].GetValue())
				_, has999 := params[999]
				assert.True(t, (fee == "4000" && !has999) || (fee == "1234" && has999),
					"torn parameter set: fee=%s has999=%v", fee, has999)

				// fee reads: only the values that were ever written
				value := controller.GetParameterInt(kapps.EnumParameter_FeePerDataByte)
				assert.Contains(t, []int64{4000, 1234}, value)
				assert.Equal(t, uint64(6000), controller.GetParameterUint(kapps.EnumParameter_LeaderValidatorRewardsPercentage))
				_, err := controller.ParseParamAndValidate(kapps.EnumParameter_FeePerDataByte, []byte("1"), mockForks)
				assert.NoError(t, err)
			}
		}()
	}

	for i := 0; i < 200; i++ {
		controller.UpdateParameters(settled)
		controller.RestoreParameters(initial)
	}

	close(done)
	wg.Wait()

	assert.Equal(t, int64(4000), controller.GetParameterInt(kapps.EnumParameter_FeePerDataByte))
	assert.NotContains(t, controller.GetActiveParameters(), int32(999))
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
		mockForks.FixAuditChangesV6Value = false

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
			{"valid at min limit", []byte("500000000"), false},  // core.MaxGasLimitPerTx
			{"valid higher value", []byte("1500000000"), false}, // 1.5B
			{"valid large value", []byte("10000000000"), false}, // 10B
			{"invalid below min", []byte("499999999"), true},
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

func TestParseParamAndValidate_EveryParameterIsBoundedFromFixAuditChangesV6(t *testing.T) {
	mockForks := mock.NewForkControllerStub()
	controller := NewProposalControllerForTests()

	params := kapps.InitialProposalParameters(mockForks)
	assert.Equal(t, len(kapps.EnumParameter_name), len(params))

	for key, param := range params {
		name := kapps.EnumParameter(key)

		// a parameter without a bounds row is rejected, so this also proves every row exists
		_, err := controller.ParseParamAndValidate(name, param.Value, mockForks)
		assert.NoError(t, err, "%s default %s must sit inside its bounds", name, param.Value)

		_, err = controller.ParseParamAndValidate(name, []byte("-1"), mockForks)
		assert.Error(t, err, "%s must reject -1", name)

		_, err = controller.ParseParamAndValidate(name, []byte(strconv.FormatInt(math.MaxInt64, 10)), mockForks)
		assert.Error(t, err, "%s must reject MaxInt64", name)
	}
}

func TestInitialProposalParameters_MinKFIVoteAmountExistsOnlyAfterTheFork(t *testing.T) {
	key := int32(kapps.EnumParameter_MinKFIVoteAmount)

	afterFork := mock.NewForkControllerStub()
	param, ok := kapps.InitialProposalParameters(afterFork)[key]
	assert.True(t, ok)
	assert.Equal(t, kapps.EnumType_Int64, param.GetType())
	assert.Equal(t, []byte("10000000"), param.GetValue())

	beforeFork := mock.NewForkControllerStub()
	beforeFork.FixAuditChangesV7Value = false
	assert.NotContains(t, kapps.InitialProposalParameters(beforeFork), key)
	assert.Len(t, kapps.InitialProposalParameters(afterFork), len(kapps.InitialProposalParameters(beforeFork))+1)
}

func TestParseParamAndValidate_MinKFIVoteAmountBounds(t *testing.T) {
	controller := NewProposalControllerForTests()
	mockForks := mock.NewForkControllerStub()

	cases := []struct {
		value string
		err   error
	}{
		{"0", common.ErrInvalidParameter},
		{"-1", common.ErrInvalidParameter},
		{"1", nil},
		{"10000000", nil},
		{"1000000000", nil},
		{"1000000001", common.ErrInvalidParameter},
	}

	for _, tc := range cases {
		_, err := controller.ParseParamAndValidate(kapps.EnumParameter_MinKFIVoteAmount, []byte(tc.value), mockForks)
		assert.Equal(t, tc.err, err, "value %s", tc.value)
	}
}

// TestParseParamAndValidate_NegativeFees is the KLC-2691 reproducer: negative fees passed
// validation. They are rejected from FixAuditChangesV6 and still accepted before it, so
// replaying a pre-fork block reaches the same decision the chain did.
func TestParseParamAndValidate_NegativeFees(t *testing.T) {
	controller := NewProposalControllerForTests()

	cases := map[kapps.EnumParameter]string{
		kapps.EnumParameter_FeePerDataByte:     "-4000",
		kapps.EnumParameter_KAppFeeTransfer:    "-500000",
		kapps.EnumParameter_KAppFeeCreateAsset: "-20000000000",
		kapps.EnumParameter_KAppFeeProposal:    "-1",
	}

	afterFork := mock.NewForkControllerStub()
	beforeFork := mock.NewForkControllerStub()
	beforeFork.FixAuditChangesV6Value = false

	for name, value := range cases {
		_, err := controller.ParseParamAndValidate(name, []byte(value), afterFork)
		assert.Equal(t, common.ErrInvalidParameter, err, "%s = %s after FixAuditChangesV6", name, value)

		_, err = controller.ParseParamAndValidate(name, []byte(value), beforeFork)
		assert.NoError(t, err, "%s = %s before FixAuditChangesV6", name, value)
	}
}

// Every bound sits at its exact edge: the floor and the ceiling are accepted, one past
// either is not. The table covers every parameter, so a row widened by mistake fails here.
func TestParseParamAndValidate_EveryBoundAtItsEdges(t *testing.T) {
	controller := NewProposalControllerForTests()
	mockForks := mock.NewForkControllerStub()

	const (
		maxGasPerBlock = int64(core.MaxGasLimitPerTx) * core.MaxKappIterator
		maxKLVSupply   = 10_000_000_000_000_000
		maxKFISupply   = 21_000_000_000_000

		maxMinKFIVoteAmount = 1_000_000_000
	)

	kappFee := [2]int64{0, 1_000_000_000_000} // 1M KLV
	emission := [2]int64{0, 1_000_000_000}
	klvStake := [2]int64{1, maxKLVSupply}
	iteration := [2]int64{1, core.MaxKappIterator}

	edges := map[kapps.EnumParameter][2]int64{
		kapps.EnumParameter_FeePerDataByte: {0, 4_000_000},

		kapps.EnumParameter_KAppFeeCreateValidator:         kappFee,
		kapps.EnumParameter_KAppFeeCreateAsset:             kappFee,
		kapps.EnumParameter_KAppFeeTransfer:                kappFee,
		kapps.EnumParameter_KAppFeeAssetTrigger:            kappFee,
		kapps.EnumParameter_KAppFeeValidatorConfig:         kappFee,
		kapps.EnumParameter_KAppFeeFreeze:                  kappFee,
		kapps.EnumParameter_KAppFeeUnfreeze:                kappFee,
		kapps.EnumParameter_KAppFeeDelegate:                kappFee,
		kapps.EnumParameter_KAppFeeUndelegate:              kappFee,
		kapps.EnumParameter_KAppFeeWithdraw:                kappFee,
		kapps.EnumParameter_KAppFeeClaim:                   kappFee,
		kapps.EnumParameter_KAppFeeUnjail:                  kappFee,
		kapps.EnumParameter_KAppFeeSetAccountName:          kappFee,
		kapps.EnumParameter_KAppFeeProposal:                kappFee,
		kapps.EnumParameter_KAppFeeVote:                    kappFee,
		kapps.EnumParameter_KAppFeeConfigITO:               kappFee,
		kapps.EnumParameter_KAppFeeSetITOPrices:            kappFee,
		kapps.EnumParameter_KAppFeeBuy:                     kappFee,
		kapps.EnumParameter_KAppFeeSell:                    kappFee,
		kapps.EnumParameter_KAppFeeCancelMarketOrder:       kappFee,
		kapps.EnumParameter_KAppFeeCreateMarketplace:       kappFee,
		kapps.EnumParameter_KAppFeeConfigMarketplace:       kappFee,
		kapps.EnumParameter_KAppFeeUpdateAccountPermission: kappFee,
		kapps.EnumParameter_KAppFeeITOTrigger:              kappFee,
		kapps.EnumParameter_KAppFeeDeposit:                 kappFee,
		kapps.EnumParameter_KAppFeeSmartContract:           kappFee,

		kapps.EnumParameter_BlockRewards:   emission,
		kapps.EnumParameter_StakingRewards: emission,

		kapps.EnumParameter_MinSelfDelegatedAmount:  klvStake,
		kapps.EnumParameter_MinTotalDelegatedAmount: klvStake,
		kapps.EnumParameter_MinKLVBucketAmount:      klvStake,

		kapps.EnumParameter_MinKFIStakedToEnableProposals: {1, maxKFISupply},
		kapps.EnumParameter_MinKFIVoteAmount:              {1, maxMinKFIVoteAmount},

		kapps.EnumParameter_MaxEpochsUnclaimed: iteration,
		kapps.EnumParameter_MaxNFTMintBatch:    iteration,
		kapps.EnumParameter_MaxBucketSize:      iteration,

		kapps.EnumParameter_ProposalMaxEpochsDuration:        {1, 1_000},
		kapps.EnumParameter_LeaderValidatorRewardsPercentage: {0, int64(core.HundredPercent)},

		kapps.EnumParameter_GasMultiplier:  {1, 10_000},
		kapps.EnumParameter_MaxGasPerBlock: {core.MaxGasLimitPerTx, maxGasPerBlock},
		kapps.EnumParameter_MaxGasPerTX:    {core.MinGasLimit, core.MaxGasLimitPerTx},
	}
	assert.Equal(t, len(kapps.EnumParameter_name), len(edges), "every parameter needs a row")

	for name, edge := range edges {
		floor, ceiling := edge[0], edge[1]

		for _, accepted := range []int64{floor, ceiling} {
			_, err := controller.ParseParamAndValidate(name, []byte(strconv.FormatInt(accepted, 10)), mockForks)
			assert.NoError(t, err, "%s at %d", name, accepted)
		}

		for _, rejected := range []int64{floor - 1, ceiling + 1} {
			_, err := controller.ParseParamAndValidate(name, []byte(strconv.FormatInt(rejected, 10)), mockForks)
			if rejected < 0 && name == kapps.EnumParameter_LeaderValidatorRewardsPercentage {
				// a uint32: the parser rejects -1 before the bounds see it
				assert.Error(t, err, "%s at %d", name, rejected)
				continue
			}
			assert.ErrorIs(t, err, common.ErrInvalidParameter, "%s at %d", name, rejected)
		}
	}
}
