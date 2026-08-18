package config

import (
	"fmt"
	"math"
	"testing"

	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
)

type operations struct {
	OperationA uint64
	OperationB uint64
	OperationC uint64
	OperationD uint64
	OperationE uint64
}

func TestDecode(t *testing.T) {
	gasMap := make(map[string]uint64)
	gasMap["OperationB"] = 4
	gasMap["OperationA"] = 3
	gasMap["OperationC"] = 100
	gasMap["OperationD"] = 1000

	op := &operations{}
	err := mapstructure.Decode(gasMap, op)
	assert.Nil(t, err)

	fmt.Printf("%+v\n", op)
}

func TestDecode_VMGas(t *testing.T) {
	gasMap := make(map[string]uint64)
	gasMap["StorePerByte"] = 4
	gasMap["GetSCAddress"] = 4
	gasMap["GetExternalBalance"] = 4
	gasMap["BigIntByteLength"] = 4

	bigIntOp := &BigIntAPICost{}
	err := mapstructure.Decode(gasMap, bigIntOp)
	assert.Nil(t, err)

	fmt.Printf("%+v\n", bigIntOp)

	bigFloatOp := &BigFloatAPICost{}
	err = mapstructure.Decode(gasMap, bigFloatOp)
	assert.Nil(t, err)

	fmt.Printf("%+v\n", bigFloatOp)

	erdOp := &BaseOpsAPICost{}
	err = mapstructure.Decode(gasMap, erdOp)
	assert.Nil(t, err)

	fmt.Printf("%+v\n", erdOp)
}

func TestDecode_ZeroGasCostError(t *testing.T) {
	gasMap := FillGasMapWASMOpcodeValues(1)

	wasmCosts := &executor.WASMOpcodeCost{}
	err := mapstructure.Decode(gasMap, wasmCosts)
	assert.Nil(t, err)

	err = checkForZeroUint64Fields(*wasmCosts)
	assert.Nil(t, err)

	gasMap["BrIf"] = 0
	wasmCosts = &executor.WASMOpcodeCost{}
	err = mapstructure.Decode(gasMap, wasmCosts)
	assert.Nil(t, err)

	err = checkForZeroUint64Fields(*wasmCosts)
	assert.Error(t, err)
}

func TestDecode_Uint32OverflowError(t *testing.T) {
	gasMap := FillGasMapWASMOpcodeValues(1)
	assert.Nil(t, checkForUint32Overflow(gasMap))

	// Every WASMOpcodeCost field is uint32, but the gas schedule carries
	// uint64 values, and mapstructure truncates on overflow instead of
	// erroring. Without the range check a limit field such as
	// MaxDeclaredTableSize would silently become 1 and cap every contract
	// table at a single element.
	gasMap["MaxDeclaredTableSize"] = uint64(math.MaxUint32) + 2
	assert.Error(t, checkForUint32Overflow(gasMap))

	// Demonstrates the truncation this guard exists to prevent.
	wasmCosts := &executor.WASMOpcodeCost{}
	err := mapstructure.Decode(gasMap, wasmCosts)
	assert.Nil(t, err)
	assert.Equal(t, uint32(1), wasmCosts.MaxDeclaredTableSize)

	// Exactly MaxUint32 still fits and must remain accepted.
	gasMap["MaxDeclaredTableSize"] = uint64(math.MaxUint32)
	assert.Nil(t, checkForUint32Overflow(gasMap))
}

// TestCreateGasConfig_RejectsUint32Overflow covers the overflow guard through
// CreateGasConfig rather than by calling checkForUint32Overflow directly, so
// the wiring is exercised too: a guard that is correct but never called from
// the decode path would leave the truncation in place.
func TestCreateGasConfig_RejectsUint32Overflow(t *testing.T) {
	gasScheduleMap := MakeGasMap(1)
	gasCost, err := CreateGasConfig(gasScheduleMap)
	assert.Nil(t, err)
	assert.NotNil(t, gasCost)

	gasScheduleMap["WASMOpcodeCost"]["MaxDeclaredTableSize"] = uint64(math.MaxUint32) + 2

	gasCost, err = CreateGasConfig(gasScheduleMap)
	assert.Error(t, err)
	assert.Nil(t, gasCost, "an overflowing entry must abort the whole decode, not yield a truncated config")
}

func Test_getSignedCoefficient(t *testing.T) {
	gasScheduleMap := MakeGasMap(1)

	A := uint64(688)
	B := uint64(31858)
	C := uint64(15287)

	gasMap := make(map[string]uint64)
	gasMap["QuadraticCoefficient"] = A
	gasMap["SignOfQuadratic"] = 0
	gasMap["LinearCoefficient"] = B
	gasMap["SignOfLinear"] = 0
	gasMap["ConstantCoefficient"] = C
	gasMap["SignOfConstant"] = 0
	gasScheduleMap["DynamicStorageLoad"] = gasMap

	gasCost, err := CreateGasConfig(gasScheduleMap)
	assert.Nil(t, err)
	assert.Equal(t, int64(A), gasCost.DynamicStorageLoad.Quadratic)
	assert.Equal(t, int64(B), gasCost.DynamicStorageLoad.Linear)
	assert.Equal(t, int64(C), gasCost.DynamicStorageLoad.Constant)
}

func Test_isDynamicGasComputationFuncCorrectlyDefined(t *testing.T) {
	t.Parallel()

	t.Run("invalid stationary point", func(t *testing.T) {
		t.Parallel()

		params := &DynamicStorageLoadCostCoefficients{
			Quadratic:  5,
			Linear:     -5,
			Constant:   1,
			MinGasCost: 0,
		}

		ok := isDynamicGasComputationFuncCorrectlyDefined(params)
		assert.False(t, ok)
	})

	t.Run("concave func", func(t *testing.T) {
		t.Parallel()

		params := &DynamicStorageLoadCostCoefficients{
			Quadratic:  -5,
			Linear:     -5,
			Constant:   1,
			MinGasCost: 0,
		}

		ok := isDynamicGasComputationFuncCorrectlyDefined(params)
		assert.False(t, ok)
	})

	t.Run("constant parameter is negative", func(t *testing.T) {
		t.Parallel()

		params := &DynamicStorageLoadCostCoefficients{
			Quadratic:  5,
			Linear:     5,
			Constant:   -1,
			MinGasCost: 0,
		}

		ok := isDynamicGasComputationFuncCorrectlyDefined(params)
		assert.False(t, ok)
	})

	t.Run("ok params", func(t *testing.T) {
		t.Parallel()

		params := &DynamicStorageLoadCostCoefficients{
			Quadratic:  5,
			Linear:     5,
			Constant:   1,
			MinGasCost: 0,
		}

		ok := isDynamicGasComputationFuncCorrectlyDefined(params)
		assert.True(t, ok)
	})

	t.Run("benchmarked params", func(t *testing.T) {
		t.Parallel()

		params := &DynamicStorageLoadCostCoefficients{
			Quadratic:  688,
			Linear:     31858,
			Constant:   15287,
			MinGasCost: 0,
		}

		ok := isDynamicGasComputationFuncCorrectlyDefined(params)
		assert.True(t, ok)
	})
}
