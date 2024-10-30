package vmhooks_test

import (
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/kvm/executor"
	mock "github.com/klever-io/klever-go/kvm/mock/context"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
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

			result := make([]byte, length)
			result, err := hooks.MemLoad(executor.MemPtr(0), length)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBytes, result)
		})
	}
}
