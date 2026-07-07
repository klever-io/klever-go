package vmhooks_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/kvm/executor"
	mock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/hostCore"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBigIntNew(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected *big.Int
	}{
		{
			name:     "positive number",
			input:    42,
			expected: big.NewInt(42),
		},
		{
			name:     "negative number",
			input:    -42,
			expected: big.NewInt(-42),
		},
		{
			name:     "zero",
			input:    0,
			expected: big.NewInt(0),
		},
		{
			name:     "max int64",
			input:    9223372036854775807,
			expected: big.NewInt(9223372036854775807),
		},
		{
			name:     "min int64",
			input:    -9223372036854775808,
			expected: big.NewInt(-9223372036854775808),
		},
	}

	mockWorld := worldmock.NewMockWorld()
	vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
	hooks := vmhooks.NewVMHooksImpl(vmHost)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle := hooks.BigIntNew(tt.input)
			result, _ := vmHost.ManagedTypes().GetBigInt(handle)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBigIntArithmetic(t *testing.T) {
	tests := []struct {
		name      string
		op1       int64
		op2       int64
		expected  *big.Int
		operation func(*vmhooks.VMHooksImpl, int32, int32, int32)
	}{
		{
			name:     "add positive numbers",
			op1:      100,
			op2:      200,
			expected: big.NewInt(300),
			operation: func(h *vmhooks.VMHooksImpl, dest, a, b int32) {
				h.BigIntAdd(dest, a, b)
			},
		},
		{
			name:     "add negative and positive",
			op1:      -100,
			op2:      200,
			expected: big.NewInt(100),
			operation: func(h *vmhooks.VMHooksImpl, dest, a, b int32) {
				h.BigIntAdd(dest, a, b)
			},
		},
		{
			name:     "subtract positive numbers",
			op1:      200,
			op2:      100,
			expected: big.NewInt(100),
			operation: func(h *vmhooks.VMHooksImpl, dest, a, b int32) {
				h.BigIntSub(dest, a, b)
			},
		},
		{
			name:     "subtract to negative",
			op1:      100,
			op2:      200,
			expected: big.NewInt(-100),
			operation: func(h *vmhooks.VMHooksImpl, dest, a, b int32) {
				h.BigIntSub(dest, a, b)
			},
		},
		{
			name:     "multiply positive numbers",
			op1:      100,
			op2:      200,
			expected: big.NewInt(20000),
			operation: func(h *vmhooks.VMHooksImpl, dest, a, b int32) {
				h.BigIntMul(dest, a, b)
			},
		},
		{
			name:     "multiply by zero",
			op1:      100,
			op2:      0,
			expected: big.NewInt(0),
			operation: func(h *vmhooks.VMHooksImpl, dest, a, b int32) {
				h.BigIntMul(dest, a, b)
			},
		},
	}

	mockWorld := worldmock.NewMockWorld()
	vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
	hooks := vmhooks.NewVMHooksImpl(vmHost)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle1 := hooks.BigIntNew(tt.op1)
			handle2 := hooks.BigIntNew(tt.op2)
			destHandle := hooks.BigIntNew(0)
			tt.operation(hooks, destHandle, handle1, handle2)
			result, _ := vmHost.ManagedTypes().GetBigInt(destHandle)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBigIntDivision(t *testing.T) {
	tests := []struct {
		name        string
		op1         int64
		op2         int64
		expected    *big.Int
		expectError bool
	}{
		{
			name:        "normal division",
			op1:         100,
			op2:         20,
			expected:    big.NewInt(5),
			expectError: false,
		},
		{
			name:        "division with negative dividend",
			op1:         -100,
			op2:         20,
			expected:    big.NewInt(-5),
			expectError: false,
		},
		{
			name:        "division by one",
			op1:         100,
			op2:         1,
			expected:    big.NewInt(100),
			expectError: false,
		},
		{
			name:        "zero dividend",
			op1:         0,
			op2:         20,
			expected:    big.NewInt(0),
			expectError: false,
		},
		{
			name:        "division by zero",
			op1:         100,
			op2:         0,
			expected:    nil,
			expectError: true,
		},
	}

	mockWorld := worldmock.NewMockWorld()
	vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
	hooks := vmhooks.NewVMHooksImpl(vmHost)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle1 := hooks.BigIntNew(tt.op1)
			handle2 := hooks.BigIntNew(tt.op2)
			destHandle := hooks.BigIntNew(0)

			hooks.BigIntTDiv(destHandle, handle1, handle2)

			if tt.expectError {
				assert.Equal(t, vmcommon.VMExecutionFailed, vmHost.Output().ReturnCode())
				assert.Equal(t, "division by 0", vmHost.Output().ReturnMessage())
				return
			}

			result, _ := vmHost.ManagedTypes().GetBigInt(destHandle)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBigIntBitwise(t *testing.T) {
	tests := []struct {
		name        string
		input       int64
		bits        int32
		expected    *big.Int
		expectError bool
	}{
		{
			name:        "shift right by 2",
			input:       100,
			bits:        2,
			expected:    big.NewInt(25),
			expectError: false,
		},
		{
			name:        "shift right by zero",
			input:       100,
			bits:        0,
			expected:    big.NewInt(100),
			expectError: false,
		},
		{
			name:        "shift right negative number",
			input:       -100,
			bits:        2,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "shift right by negative bits",
			input:       100,
			bits:        -2,
			expected:    nil,
			expectError: true,
		},
	}

	mockWorld := worldmock.NewMockWorld()
	vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
	hooks := vmhooks.NewVMHooksImpl(vmHost)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle := hooks.BigIntNew(tt.input)
			destHandle := hooks.BigIntNew(0)

			hooks.BigIntShr(destHandle, handle, tt.bits)

			if tt.expectError {
				assert.Equal(t, vmcommon.VMExecutionFailed, vmHost.Output().ReturnCode())
				assert.Equal(t, "bitwise shift operations only allowed on positive integers and by a positive amount", vmHost.Output().ReturnMessage())
				return
			}

			result, _ := vmHost.ManagedTypes().GetBigInt(destHandle)
			assert.Equal(t, tt.expected, result)
		})
	}
}

type InstanceTracker interface {
	ReplaceInstance(instance executor.Instance)
}

func TestBigIntSetUnsignedBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected *big.Int
	}{
		{
			name:     "normal bytes",
			input:    []byte{0x01, 0x02, 0x03},
			expected: new(big.Int).SetBytes([]byte{0x01, 0x02, 0x03}),
		},
		{
			name:     "empty bytes",
			input:    []byte{},
			expected: big.NewInt(0),
		},
		{
			name:     "bytes with leading zeros",
			input:    []byte{0x00, 0x00, 0x01},
			expected: big.NewInt(1),
		},
		{
			name:     "single zero byte",
			input:    []byte{0x00},
			expected: big.NewInt(0),
		},
	}

	mockWorld := worldmock.NewMockWorld()
	vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
	hooks := vmhooks.NewVMHooksImpl(vmHost)

	// set new instance
	it, ok := vmHost.Runtime().GetInstanceTracker().(InstanceTracker)
	require.True(t, ok)
	it.ReplaceInstance(mock.NewInstanceMock(nil))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset := 0
			destHandle := hooks.BigIntNew(0)
			err := hooks.MemStore(executor.MemPtr(offset), tt.input)
			require.NoError(t, err)
			hooks.BigIntSetUnsignedBytes(destHandle, executor.MemPtr(offset), executor.MemLength(len(tt.input)))
			result, _ := vmHost.ManagedTypes().GetBigInt(destHandle)
			assert.Equal(t, 0, result.Cmp(tt.expected))
		})
	}
}

func TestBigIntGetUnsignedBytes(t *testing.T) {
	tests := []struct {
		name          string
		input         int64
		expectedBytes []byte
	}{
		{
			name:          "positive number",
			input:         257,
			expectedBytes: []byte{0x01, 0x01},
		},
		{
			name:          "zero",
			input:         0,
			expectedBytes: []byte{},
		},
		{
			name:          "negative number",
			input:         -257,
			expectedBytes: []byte{0x01, 0x01}, // should be treated as positive
		},
		{
			name:          "large number",
			input:         1<<32 - 1,
			expectedBytes: []byte{0xff, 0xff, 0xff, 0xff},
		},
	}

	mockWorld := worldmock.NewMockWorld()
	vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
	hooks := vmhooks.NewVMHooksImpl(vmHost)

	// set new instance
	it, ok := vmHost.Runtime().GetInstanceTracker().(InstanceTracker)
	require.True(t, ok)
	it.ReplaceInstance(mock.NewInstanceMock(nil))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle := hooks.BigIntNew(tt.input)

			length := hooks.BigIntGetUnsignedBytes(handle, executor.MemPtr(0))

			result, err := hooks.MemLoad(executor.MemPtr(0), length)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBytes, result)
		})
	}
}

func TestBigIntGetSignedBytes(t *testing.T) {
	tests := []struct {
		name          string
		input         int64
		expectedBytes []byte
	}{
		{
			name:          "positive small number",
			input:         127,
			expectedBytes: []byte{0x7f},
		},
		{
			name:          "negative small number",
			input:         -128,
			expectedBytes: []byte{0x80},
		},
		{
			name:          "zero",
			input:         0,
			expectedBytes: []byte{},
		},
		{
			name:          "positive number requiring two bytes",
			input:         128,
			expectedBytes: []byte{0x00, 0x80},
		},
		{
			name:          "negative number requiring two bytes",
			input:         -129,
			expectedBytes: []byte{0xff, 0x7f},
		},
	}

	mockWorld := worldmock.NewMockWorld()
	vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
	hooks := vmhooks.NewVMHooksImpl(vmHost)

	// set new instance
	it, ok := vmHost.Runtime().GetInstanceTracker().(InstanceTracker)
	require.True(t, ok)
	it.ReplaceInstance(mock.NewInstanceMock(nil))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle := hooks.BigIntNew(tt.input)
			length := hooks.BigIntGetSignedBytes(handle, executor.MemPtr(0))

			result, err := hooks.MemLoad(executor.MemPtr(0), length)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBytes, result)
		})
	}
}

func TestBigIntGetCallValue(t *testing.T) {
	cases := []struct {
		title        string
		kdaTransfers []*vmcommon.KDATransfer
	}{
		{
			title:        "With 10 transfers",
			kdaTransfers: generateTransfersSlice(9),
		},
		{
			title:        "With 20 transfers",
			kdaTransfers: generateTransfersSlice(19),
		},
		{
			title:        "With 30 transfers",
			kdaTransfers: generateTransfersSlice(29),
		},
		{
			title:        "With 40 transfers",
			kdaTransfers: generateTransfersSlice(39),
		},
		{
			title:        "With 50 transfers",
			kdaTransfers: generateTransfersSlice(49),
		},
	}

	t.Run("Sucessful", func(t *testing.T) {
		for _, tt := range cases {
			t.Run(tt.title, func(t *testing.T) {
				mockWorld := worldmock.NewMockWorld()
				vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
				hooks := vmhooks.NewVMHooksImpl(vmHost)

				// Set mock instance
				it, ok := vmHost.Runtime().GetInstanceTracker().(InstanceTracker)
				require.True(t, ok)
				it.ReplaceInstance(mock.NewInstanceMock(nil))

				destHandle := int32(1)

				// As it does not exist, it will be created
				initialValue := hooks.GetManagedTypesContext().GetBigIntOrCreate(destHandle)
				assert.Equal(t, initialValue, big.NewInt(0))

				expected := big.NewInt(100)
				tt.kdaTransfers = append(tt.kdaTransfers, &vmcommon.KDATransfer{
					KDATokenName: kdautils.KLVIdentifier, // BigIntGetCallValue hook only retrieves KLV
					KDAValue:     expected,
				})

				shuffleTransferSlice(tt.kdaTransfers)
				hooks.GetRuntimeContext().SetVMInput(&vmcommon.ContractCallInput{
					VMInput: vmcommon.VMInput{
						KDATransfers: tt.kdaTransfers,
					},
				})

				// Invoking test target hook who will set *big.Int value on destHandle
				hooks.BigIntGetCallValue(destHandle)

				// As it now exists the value retrieved must be non-zero
				result := hooks.GetManagedTypesContext().GetBigIntOrCreate(destHandle)
				assert.Equal(t, result, expected)

				// Calculate gas used
				loopGas := hooks.GetMeteringContext().GasSchedule().WASMOpcodeCost.Loop
				callValueGas := hooks.GetMeteringContext().
					GasSchedule().
					BigIntAPICost.BigIntGetCallValue
				totalGas := (uint64(loopGas) * uint64(len(tt.kdaTransfers))) + callValueGas

				assert.Equal(t, hooks.GetRuntimeContext().GetPointsUsed(), totalGas)
			})
		}

		t.Run("With only one KLV transfer", func(t *testing.T) {
			mockWorld := worldmock.NewMockWorld()
			vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
			hooks := vmhooks.NewVMHooksImpl(vmHost)

			// Set mock instance
			it, ok := vmHost.Runtime().GetInstanceTracker().(InstanceTracker)
			require.True(t, ok)
			it.ReplaceInstance(mock.NewInstanceMock(nil))

			destHandle := int32(1)

			// As it does not exist, it will be created
			initialValue := hooks.GetManagedTypesContext().GetBigIntOrCreate(destHandle)
			assert.Equal(t, initialValue, big.NewInt(0))

			expected := big.NewInt(100)
			kdaTransfers := []*vmcommon.KDATransfer{
				{
					KDATokenName: kdautils.KLVIdentifier, // BigIntGetCallValue hook only retrieves KLV
					KDAValue:     expected,
				},
			}
			hooks.GetRuntimeContext().SetVMInput(&vmcommon.ContractCallInput{
				VMInput: vmcommon.VMInput{
					KDATransfers: kdaTransfers,
				},
			})

			// Invoking test target hook who will set *big.Int value on destHandle
			hooks.BigIntGetCallValue(destHandle)

			// As it now exists the value retrieved must be non-zero
			result := hooks.GetManagedTypesContext().GetBigIntOrCreate(destHandle)
			assert.Equal(t, result, expected)

			// Calculate gas used
			loopGas := hooks.GetMeteringContext().GasSchedule().WASMOpcodeCost.Loop
			callValueGas := hooks.GetMeteringContext().
				GasSchedule().
				BigIntAPICost.BigIntGetCallValue
			totalGas := (uint64(loopGas) * uint64(len(kdaTransfers))) + callValueGas

			assert.Equal(t, hooks.GetRuntimeContext().GetPointsUsed(), totalGas)
		})
	})

	casesNoKLV := []struct {
		title        string
		kdaTransfers []*vmcommon.KDATransfer
	}{
		{
			title:        "With 10 transfers",
			kdaTransfers: generateTransfersSlice(10),
		},
		{
			title:        "With 20 transfers",
			kdaTransfers: generateTransfersSlice(20),
		},
		{
			title:        "With 30 transfers",
			kdaTransfers: generateTransfersSlice(30),
		},
		{
			title:        "With 40 transfers",
			kdaTransfers: generateTransfersSlice(40),
		},
		{
			title:        "With 50 transfers",
			kdaTransfers: generateTransfersSlice(50),
		},
	}

	t.Run("With no KLV call value", func(t *testing.T) {
		for _, tt := range casesNoKLV {
			t.Run(tt.title, func(t *testing.T) {
				mockWorld := worldmock.NewMockWorld()
				vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
				hooks := vmhooks.NewVMHooksImpl(vmHost)

				// Set mock instance
				it, ok := vmHost.Runtime().GetInstanceTracker().(InstanceTracker)
				require.True(t, ok)
				it.ReplaceInstance(mock.NewInstanceMock(nil))

				destHandle := int32(1)
				// As it does not exist, it will be created
				initialValue := hooks.GetManagedTypesContext().GetBigIntOrCreate(destHandle)
				assert.Equal(t, initialValue, big.NewInt(0))

				shuffleTransferSlice(tt.kdaTransfers)
				hooks.GetRuntimeContext().SetVMInput(&vmcommon.ContractCallInput{
					VMInput: vmcommon.VMInput{
						KDATransfers: tt.kdaTransfers,
					},
				})

				// Invoking test target hook who will set *big.Int value on destHandle
				hooks.BigIntGetCallValue(destHandle)

				// Invoking test target hook with empty transfers slice, so no value will be set
				result := hooks.GetManagedTypesContext().GetBigIntOrCreate(destHandle)
				assert.Equal(t, result, big.NewInt(0))

				// Calculate gas used
				loopGas := hooks.GetMeteringContext().GasSchedule().WASMOpcodeCost.Loop
				callValueGas := hooks.GetMeteringContext().
					GasSchedule().
					BigIntAPICost.BigIntGetCallValue
				totalGas := (uint64(loopGas) * uint64(len(tt.kdaTransfers))) + callValueGas

				assert.Equal(t, hooks.GetRuntimeContext().GetPointsUsed(), totalGas)
			})
		}

		t.Run("only one asset transfer", func(t *testing.T) {
			mockWorld := worldmock.NewMockWorld()
			vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
			hooks := vmhooks.NewVMHooksImpl(vmHost)

			// Set mock instance
			it, ok := vmHost.Runtime().GetInstanceTracker().(InstanceTracker)
			require.True(t, ok)
			it.ReplaceInstance(mock.NewInstanceMock(nil))

			destHandle := int32(1)

			// As it does not exist, it will be created
			initialValue := hooks.GetManagedTypesContext().GetBigIntOrCreate(destHandle)
			assert.Equal(t, initialValue, big.NewInt(0))

			kdaTransfers := []*vmcommon.KDATransfer{
				{
					KDATokenName: []byte("TEST-H7K7"),
					KDAValue:     big.NewInt(100),
				},
			}
			hooks.GetRuntimeContext().SetVMInput(&vmcommon.ContractCallInput{
				VMInput: vmcommon.VMInput{
					KDATransfers: kdaTransfers,
				},
			})

			// Invoking test target hook who will set *big.Int value on destHandle
			hooks.BigIntGetCallValue(destHandle)

			// As there is no transfer of klv the value must be zero
			result := hooks.GetManagedTypesContext().GetBigIntOrCreate(destHandle)
			assert.Equal(t, result, big.NewInt(0))

			// Calculate gas used
			loopGas := hooks.GetMeteringContext().GasSchedule().WASMOpcodeCost.Loop
			callValueGas := hooks.GetMeteringContext().
				GasSchedule().
				BigIntAPICost.BigIntGetCallValue
			totalGas := (uint64(loopGas) * uint64(len(kdaTransfers))) + callValueGas

			assert.Equal(t, hooks.GetRuntimeContext().GetPointsUsed(), totalGas)
		})
	})

	t.Run("with empty call values", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, _ := hostCore.NewVMHost(mockWorld, makeHostParameters())
		hooks := vmhooks.NewVMHooksImpl(vmHost)

		// Set mock instance
		it, ok := vmHost.Runtime().GetInstanceTracker().(InstanceTracker)
		require.True(t, ok)
		it.ReplaceInstance(mock.NewInstanceMock(nil))

		destHandle := int32(1)

		// As it does not exist, it will be created
		initialValue := hooks.GetManagedTypesContext().GetBigIntOrCreate(destHandle)
		assert.Equal(t, initialValue, big.NewInt(0))

		hooks.GetRuntimeContext().SetVMInput(&vmcommon.ContractCallInput{
			VMInput: vmcommon.VMInput{
				KDATransfers: []*vmcommon.KDATransfer{},
			},
		})

		hooks.BigIntGetCallValue(destHandle)

		// As there is no transfer of klv the value must be zero
		result := hooks.GetManagedTypesContext().GetBigIntOrCreate(destHandle)
		assert.Equal(t, result, big.NewInt(0))

		// Calculate gas used
		loopGas := hooks.GetMeteringContext().GasSchedule().WASMOpcodeCost.Loop
		callValueGas := hooks.GetMeteringContext().
			GasSchedule().
			BigIntAPICost.BigIntGetCallValue
		totalGas := (uint64(loopGas) * uint64(len(
			hooks.GetRuntimeContext().GetVMInput().KDATransfers,
		))) + callValueGas

		assert.Equal(t, hooks.GetRuntimeContext().GetPointsUsed(), totalGas)
	})
}

func TestBigIntPow(t *testing.T) {
	t.Run("positive base and exponent", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost)
		provideGas(hooks, 1_000_000)

		baseHandle := hooks.BigIntNew(2)
		expHandle := hooks.BigIntNew(10)
		destHandle := hooks.BigIntNew(0)

		hooks.BigIntPow(destHandle, baseHandle, expHandle)

		result, err := vmHost.ManagedTypes().GetBigInt(destHandle)
		require.NoError(t, err)
		require.Equal(t, 0, big.NewInt(1024).Cmp(result), "2^10 should be 1024")
	})

	t.Run("zero base rejected", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost)

		baseHandle := hooks.BigIntNew(0)
		expHandle := hooks.BigIntNew(5)
		destHandle := hooks.BigIntNew(7) // sentinel; must stay unchanged

		hooks.BigIntPow(destHandle, baseHandle, expHandle)

		result, err := vmHost.ManagedTypes().GetBigInt(destHandle)
		require.NoError(t, err)
		require.Equal(t, 0, big.NewInt(7).Cmp(result), "destination must be untouched on rejection")
	})

	t.Run("cancelled context panics before writing result", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		hooks := vmhooks.NewVMHooksImpl(&hostWithExecutionContext{VMHost: vmHost, ctx: ctx})
		provideGas(hooks, 1_000_000)

		baseHandle := hooks.BigIntNew(2)
		expHandle := hooks.BigIntNew(10)
		destHandle := hooks.BigIntNew(7) // sentinel; must stay unchanged

		require.PanicsWithValue(t, vmhost.ErrExecutionFailedWithTimeout, func() {
			hooks.BigIntPow(destHandle, baseHandle, expHandle)
		}, "cancelled context must panic ErrExecutionFailedWithTimeout")

		result, err := vmHost.ManagedTypes().GetBigInt(destHandle)
		require.NoError(t, err)
		require.Equal(t, 0, big.NewInt(7).Cmp(result), "destination must stay unset on timeout")
	})

	t.Run("nil context runs to completion", func(t *testing.T) {
		mockWorld := worldmock.NewMockWorld()
		vmHost, err := hostCore.NewVMHost(mockWorld, makeHostParameters())
		require.NoError(t, err)
		hooks := vmhooks.NewVMHooksImpl(vmHost) // real host => GetExecutionContext() == nil
		provideGas(hooks, 1_000_000)

		baseHandle := hooks.BigIntNew(2)
		expHandle := hooks.BigIntNew(10)
		destHandle := hooks.BigIntNew(0)

		require.NotPanics(t, func() {
			hooks.BigIntPow(destHandle, baseHandle, expHandle)
		}, "nil context must not panic")

		result, err := vmHost.ManagedTypes().GetBigInt(destHandle)
		require.NoError(t, err)
		require.Equal(t, 0, big.NewInt(1024).Cmp(result), "2^10 should be 1024")
	})
}
