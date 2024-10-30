package tools

import (
	"math"
	"strconv"

	"github.com/klever-io/klever-go/common"
)

// SafeAddU64 add two uint64 values, if overflow return error.
func SafeAddU64(a, b uint64) (uint64, error) {
	if a > math.MaxUint64-b {
		return 0, common.ErrUint64Overflow
	}
	return a + b, nil
}

func SafeAddI64(a int64, b interface{}) (int64, error) {
	switch b := b.(type) {
	case int:
		return safeAddI64(a, int64(b))
	case int64:
		return safeAddI64(a, b)
	case uint64:
		if b > math.MaxInt64 {
			return 0, common.ErrInt64Overflow
		}
		return safeAddI64(a, SafeU64ToI64(b))
	default:
		return 0, common.ErrInvalidType
	}
}

func safeAddI64(a, b int64) (int64, error) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, common.ErrInt64Overflow
	}
	return a + b, nil
}

// SafeU64ToI64 converts a uint64 to an int64, capping the value at math.MaxInt64.
func SafeU64ToI64(value uint64) int64 {
	if value > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(value) // #nosec G115
}

// SafeI64ToU64 converts a int64 to an uint64, if < 0 return 0.
func SafeI64ToU64(value int64) uint64 {
	if value < 0 {
		return 0
	}
	return uint64(value) // #nosec G115
}

// SafeU64ToI32 converts a uint64 to an int32, capping the value at math.MaxInt32.
func SafeU64ToI32(value uint64) int32 {
	if value > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(value) // #nosec G115
}

// SafeU64ToU32 converts a uint64 to an uint32, capping the value at math.MaxUint32.
func SafeU64ToU32(value uint64) uint32 {
	if value > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(value) // #nosec G115
}

// SafeU32ToU32 converts a uint32 to an uint32, capping the value at math.MaxInt32.
func SafeU32ToI32(value uint32) int32 {
	if value > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(value) // #nosec G115
}

// SafeF64ToU32 converts a float64 to an uint32, capping the value at math.MaxUint32.
func SafeF64ToU32(value float64) uint32 {
	if value > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(value) // #nosec G115
}

// SafeI64ToI32 converts a int64 to an int32, capping the value at math.MaxInt32.
func SafeI64ToI32(value int64) int32 {
	if value > math.MaxInt32 {
		return math.MaxInt32
	}
	if value < math.MinInt32 {
		return math.MinInt32
	}
	return int32(value) // #nosec G115
}

// SafeI64ToU32 converts a int64 to an uint32, capping the value at math.MaxUint32.
func SafeI64ToU32(value int64) uint32 {
	if value > math.MaxUint32 {
		return math.MaxUint32
	}
	if value < 0 {
		return 0
	}
	return uint32(value) // #nosec G115
}

// SafeStringToU64 converts a string to a uint64, if overflow return error.
func SafeStringToU64(value string) (uint64, error) {
	return strconv.ParseUint(value, 10, 64)
}

// SafeStringToI64 converts a string to a int64, if overflow return error.
func SafeStringToI64(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

// SafeStringToI32 converts a string to a int32, if overflow return error.
func SafeStringToI32(value string) (int32, error) {
	i, err := strconv.ParseInt(value, 10, 32)
	return SafeI64ToI32(i), err
}

// SafeStringToU32 converts a string to a uint32, if overflow return error.
func SafeStringToU32(value string) (uint32, error) {
	i, err := strconv.ParseUint(value, 10, 32)
	return SafeU64ToU32(i), err
}
