package tools_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/tools"
	"github.com/stretchr/testify/assert"
)

func TestSafeAddU64(t *testing.T) {
	tests := []struct {
		name     string
		a        uint64
		b        uint64
		expected uint64
		err      error
	}{
		{"basic addition", 1, 2, 3, nil},
		{"zero values", 0, 0, 0, nil},
		{"max value", math.MaxUint64, 0, math.MaxUint64, nil},
		{"overflow", math.MaxUint64, 1, 0, common.ErrUint64Overflow},
		{"near overflow", math.MaxUint64 - 1, 1, math.MaxUint64, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tools.SafeAddU64(tt.a, tt.b)
			assert.Equal(t, tt.err, err)
			if err == nil {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSafeAddI64(t *testing.T) {
	tests := []struct {
		name     string
		a        int64
		b        interface{}
		expected int64
		err      error
	}{
		{"add int", 1, 2, 3, nil},
		{"add int64", 1, int64(2), 3, nil},
		{"add uint64", 1, uint64(2), 3, nil},
		{"max int64", math.MaxInt64, 0, math.MaxInt64, nil},
		{"min int64", math.MinInt64, 0, math.MinInt64, nil},
		{"overflow positive", math.MaxInt64, 1, 0, common.ErrInt64Overflow},
		{"overflow negative", math.MinInt64, -1, 0, common.ErrInt64Overflow},
		{"invalid type", 1, "2", 0, common.ErrInvalidType},
		{"uint64 too large", 1, uint64(math.MaxUint64), 0, common.ErrInt64Overflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tools.SafeAddI64(tt.a, tt.b)
			assert.Equal(t, tt.err, err)
			if err == nil {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSafeU64ToI64(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		expected int64
	}{
		{"zero value", 0, 0},
		{"max int64", uint64(math.MaxInt64), math.MaxInt64},
		{"above max int64", math.MaxUint64, math.MaxInt64},
		{"normal value", 42, 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tools.SafeU64ToI64(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSafeI64ToU64(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected uint64
	}{
		{"zero value", 0, 0},
		{"positive value", 42, 42},
		{"negative value", -42, 0},
		{"max value", math.MaxInt64, uint64(math.MaxInt64)},
		{"min value", math.MinInt64, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tools.SafeI64ToU64(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSafeU64ToI32(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		expected int32
	}{
		{"zero value", 0, 0},
		{"normal value", 42, 42},
		{"max int32", uint64(math.MaxInt32), math.MaxInt32},
		{"above max int32", math.MaxUint64, math.MaxInt32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tools.SafeU64ToI32(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSafeU64ToU32(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		expected uint32
	}{
		{"zero value", 0, 0},
		{"normal value", 42, 42},
		{"max uint32", uint64(math.MaxUint32), math.MaxUint32},
		{"above max uint32", math.MaxUint64, math.MaxUint32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tools.SafeU64ToU32(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSafeU32ToI32(t *testing.T) {
	tests := []struct {
		name     string
		input    uint32
		expected int32
	}{
		{"zero value", 0, 0},
		{"normal value", 42, 42},
		{"max int32", uint32(math.MaxInt32), math.MaxInt32},
		{"above max int32", math.MaxUint32, math.MaxInt32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tools.SafeU32ToI32(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSafeF64ToU32(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected uint32
	}{
		{"zero value", 0.0, 0},
		{"normal value", 42.0, 42},
		{"decimal value", 42.5, 42},
		{"max uint32", float64(math.MaxUint32), math.MaxUint32},
		{"above max uint32", math.MaxFloat64, math.MaxUint32},
		{"negative value", -42.0, 0xffffffd6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tools.SafeF64ToU32(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSafeI64ToI32(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected int32
	}{
		{"zero value", 0, 0},
		{"normal value", 42, 42},
		{"negative value", -42, -42},
		{"max int32", int64(math.MaxInt32), math.MaxInt32},
		{"min int32", int64(math.MinInt32), math.MinInt32},
		{"above max int32", math.MaxInt64, math.MaxInt32},
		{"below min int32", math.MinInt64, math.MinInt32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tools.SafeI64ToI32(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSafeI64ToU32(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected uint32
	}{
		{"zero value", 0, 0},
		{"normal value", 42, 42},
		{"negative value", -42, 0},
		{"max uint32", int64(math.MaxUint32), math.MaxUint32},
		{"above max uint32", math.MaxInt64, math.MaxUint32},
		{"min value", math.MinInt64, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tools.SafeI64ToU32(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSafeStringToU64(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected uint64
		err      error
	}{
		{"zero value", "0", 0, nil},
		{"normal value", "42", 42, nil},
		{"max uint64", "18446744073709551615", math.MaxUint64, nil},
		{"negative value", "-42", 0, &strconv.NumError{Func: "ParseUint", Num: "-42", Err: strconv.ErrSyntax}},
		{"invalid value", "abc", 0, &strconv.NumError{Func: "ParseUint", Num: "abc", Err: strconv.ErrSyntax}},
		{"overflow", "18446744073709551616", 0, &strconv.NumError{Func: "ParseUint", Num: "18446744073709551616", Err: strconv.ErrRange}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tools.SafeStringToU64(tt.input)
			if tt.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.err.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSafeStringToI32(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int32
		err      error
	}{
		{"zero value", "0", 0, nil},
		{"normal value", "42", 42, nil},
		{"negative value", "-42", -42, nil},
		{"max int32", "2147483647", math.MaxInt32, nil},
		{"min int32", "-2147483648", math.MinInt32, nil},
		{"invalid value", "abc", 0, &strconv.NumError{Func: "ParseInt", Num: "abc", Err: strconv.ErrSyntax}},
		{"overflow", "2147483648", math.MaxInt32, &strconv.NumError{Func: "ParseInt", Num: "2147483648", Err: strconv.ErrRange}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tools.SafeStringToI32(tt.input)
			if tt.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.err.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSafeStringToU32(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected uint32
		err      error
	}{
		{"zero value", "0", 0, nil},
		{"normal value", "42", 42, nil},
		{"max uint32", "4294967295", math.MaxUint32, nil},
		{"negative value", "-42", 0, &strconv.NumError{Func: "ParseUint", Num: "-42", Err: strconv.ErrSyntax}},
		{"invalid value", "abc", 0, &strconv.NumError{Func: "ParseUint", Num: "abc", Err: strconv.ErrSyntax}},
		{"overflow", "4294967296", math.MaxUint32, &strconv.NumError{Func: "ParseUint", Num: "4294967296", Err: strconv.ErrRange}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tools.SafeStringToU32(tt.input)
			if tt.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.err.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSafeStringToI64(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		err      error
	}{
		{"zero value", "0", 0, nil},
		{"normal value", "42", 42, nil},
		{"negative value", "-42", -42, nil},
		{"max int64", "9223372036854775807", math.MaxInt64, nil},
		{"min int64", "-9223372036854775808", math.MinInt64, nil},
		{"invalid value", "abc", 0, &strconv.NumError{Func: "ParseInt", Num: "abc", Err: strconv.ErrSyntax}},
		{"overflow positive", "9223372036854775808", 0, &strconv.NumError{Func: "ParseInt", Num: "9223372036854775808", Err: strconv.ErrRange}},
		{"overflow negative", "-9223372036854775809", 0, &strconv.NumError{Func: "ParseInt", Num: "-9223372036854775809", Err: strconv.ErrRange}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tools.SafeStringToI64(tt.input)
			if tt.err != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.err.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
