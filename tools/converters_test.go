package tools

import (
	"math"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ComputePercentageI64(t *testing.T) {
	tests := []struct {
		description    string
		value          int64
		percentage     int64
		expectedErr    error
		expectedResult int64
		forkActive     bool
	}{
		{
			description:    "should work pre fork",
			value:          1000000,
			percentage:     2000,
			expectedErr:    nil,
			expectedResult: 200000,
			forkActive:     false,
		},
		{
			description:    "should overflow pre fork",
			value:          math.MaxInt64,
			percentage:     math.MaxInt64,
			expectedErr:    nil,
			expectedResult: 0,
			forkActive:     false,
		},
		{
			description:    "should work",
			value:          1000000,
			percentage:     2000,
			expectedErr:    nil,
			expectedResult: 200000,
			forkActive:     true,
		},
		{
			description:    "Dust value",
			value:          1001,
			percentage:     3333,
			expectedResult: 333,
			expectedErr:    nil,
		},
		{
			description:    "Zero percentage",
			value:          1000,
			percentage:     0,
			expectedResult: 0,
			expectedErr:    nil,
		},
		{
			description:    "should overflow",
			value:          math.MaxInt64,
			percentage:     math.MaxInt64,
			expectedErr:    common.ErrInt64Overflow,
			expectedResult: 0,
			forkActive:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			result, err := ComputePercentageI64(tt.value, tt.percentage, tt.forkActive)
			require.Equal(tt.expectedErr, err)

			if tt.forkActive { // pre-fork overflow value can change
				assert.Equal(tt.expectedResult, result)
			}

		})
	}

}

func TestBytesToUint64LE(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected uint64
	}{
		{
			name:     "Empty slice",
			input:    []byte{},
			expected: 0,
		},
		{
			name:     "Nil slice",
			input:    nil,
			expected: 0,
		},
		{
			name:     "Slice smaller than 8 bytes",
			input:    []byte{0x1, 0x2},
			expected: 0x0201, // 0x02 shifted left by 8 and 0x01 in least significant byte
		},
		{
			name:     "Slice exactly 8 bytes",
			input:    []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8},
			expected: 0x0807060504030201, // Little-endian interpretation
		},
		{
			name:     "Slice just over 8 bytes",
			input:    []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9},
			expected: 0x0807060504030201, // Extra byte (0x09) is ignored
		},
		{
			name:     "Slice larger than 8 bytes",
			input:    []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xA},
			expected: 0x0807060504030201, // Extra bytes (0x09, 0x0A) are ignored
		},
		{
			name:     "Slice with all 0xFF",
			input:    []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			expected: 0xFFFFFFFFFFFFFFFF, // Maximum uint64 value
		},
		{
			name:     "Slice with mixed values",
			input:    []byte{0x01, 0xFF, 0x00, 0xAA, 0x55, 0x66, 0x77, 0x88},
			expected: 0x88776655AA00FF01, // Little-endian interpretation
		},
		{
			name:     "Single byte slice",
			input:    []byte{0x42},
			expected: 0x42, // Only one byte, so it's the entire uint64
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BytesToUint64LE(tt.input)
			assert.Equal(t, tt.expected, result, "For input %v, expected %d, but got %d", tt.input, tt.expected, result)
		})
	}
}

func TestUint64ToBytesLETruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		expected []byte
	}{
		{"Zero", 0, []byte{0}},
		{"One", 1, []byte{1}},
		{"MaxUint8", 255, []byte{255}},
		{"OneByteMore", 256, []byte{0, 1}},
		{"MaxUint16", 65535, []byte{255, 255}},
		{"ThreeBytes", 16777215, []byte{255, 255, 255}},
		{"MaxUint32", 4294967295, []byte{255, 255, 255, 255}},
		{"LargeValue", 1099511627775, []byte{255, 255, 255, 255, 255}},
		{"MaxUint64", 18446744073709551615, []byte{255, 255, 255, 255, 255, 255, 255, 255}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Uint64ToBytesLETruncate(tt.input)
			assert.Equal(t, tt.expected, result, "For input %d, expected %v, but got %v", tt.input, tt.expected, result)
		})
	}
}

func TestUint64ToBytesLE(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		expected []byte
	}{
		{"Zero", 0, []byte{0, 0, 0, 0, 0, 0, 0, 0}},
		{"One", 1, []byte{1, 0, 0, 0, 0, 0, 0, 0}},
		{"MaxUint8", 255, []byte{255, 0, 0, 0, 0, 0, 0, 0}},
		{"OneByteMore", 256, []byte{0, 1, 0, 0, 0, 0, 0, 0}},
		{"MaxUint16", 65535, []byte{255, 255, 0, 0, 0, 0, 0, 0}},
		{"ThreeBytes", 16777215, []byte{255, 255, 255, 0, 0, 0, 0, 0}},
		{"MaxUint32", 4294967295, []byte{255, 255, 255, 255, 0, 0, 0, 0}},
		{"LargeValue", 1099511627775, []byte{255, 255, 255, 255, 255, 0, 0, 0}},
		{"MaxUint64", 18446744073709551615, []byte{255, 255, 255, 255, 255, 255, 255, 255}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Uint64ToBytesLE(tt.input)
			assert.Equal(t, tt.expected, result, "For input %d, expected %v, but got %v", tt.input, tt.expected, result)
		})
	}
}

func TestUint64ToBytesLEPaddedLength(t *testing.T) {
	input := uint64(123456789)
	result := Uint64ToBytesLE(input)
	assert.Len(t, result, 8, "Expected length of result to be 8, but got %d", len(result))
}

func TestUint64ToBytesLETruncateLength(t *testing.T) {
	input := uint64(123456789)
	result := Uint64ToBytesLETruncate(input)
	assert.Len(t, result, 4, "Expected length of truncated result to be 5, but got %d", len(result))
}
