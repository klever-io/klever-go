package tools

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

// MegabyteSize represents the size in bytes of a megabyte
const MegabyteSize = 1024 * 1024

// ConvertBytes converts the input bytes in a readable string using multipliers (k, M, G)
func ConvertBytesInt(bytes int64) string {
	var preSign string
	// apply sign
	if bytes < 0 {
		bytes = -bytes
		preSign = "-"
	}

	return preSign + ConvertBytes(uint64(bytes)) // #nosec G115
}

// ConvertBytes converts the input bytes in a readable string using multipliers (k, M, G)
func ConvertBytes(bytes uint64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(bytes)/1024.0)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(bytes)/1024.0/1024.0)
	}
	return fmt.Sprintf("%.2f GB", float64(bytes)/1024.0/1024.0/1024.0)
}

// CalculateHash marshalizes the interface and calculates its hash
func CalculateHash(
	marshalizer marshal.Marshalizer,
	hasher hashing.Hasher,
	object interface{},
) ([]byte, error) {
	if check.IfNil(marshalizer) {
		return nil, ErrNilMarshalizer
	}
	if check.IfNil(hasher) {
		return nil, ErrNilHasher
	}

	mrsData, err := marshalizer.Marshal(object)
	if err != nil {
		return nil, err
	}

	hash := hasher.Compute(string(mrsData))
	return hash, nil
}

func plural(count int, singular string) (result string) {
	if count < 2 {
		result = strconv.Itoa(count) + " " + singular + " "
	} else {
		result = strconv.Itoa(count) + " " + singular + "s "
	}
	return
}

// SecondsToHourMinSec transform seconds input in a human friendly format
func SecondsToHourMinSec(input int) string {
	numSecondsInAMinute := 60
	numMinutesInAHour := 60
	numSecondsInAHour := numSecondsInAMinute * numMinutesInAHour
	result := ""

	hours := math.Floor(float64(input) / float64(numSecondsInAMinute) / float64(numMinutesInAHour))
	seconds := input % (numSecondsInAHour)
	minutes := math.Floor(float64(seconds) / float64(numSecondsInAMinute))
	seconds = input % numSecondsInAMinute

	if hours > 0 {
		result = plural(int(hours), "hour")
	}
	if minutes > 0 {
		result += plural(int(minutes), "minute")
	}
	if seconds > 0 {
		result += plural(seconds, "second")
	}

	return result
}

func ComputePercentageI64(value int64, percentage int64, checkOverflow bool) (int64, error) {
	if !checkOverflow {
		return int64(float64(value) * float64(percentage) / float64(core.HundredPercent)), nil
	}

	valueBig := big.NewInt(value)
	percentageBig := big.NewInt(percentage)

	result := new(big.Int).Mul(valueBig, percentageBig)
	result.Div(result, big.NewInt(int64(core.HundredPercent)))

	if !result.IsInt64() { // check overflow
		return 0, common.ErrInt64Overflow
	}

	return result.Int64(), nil
}

// BytesToUint64LE converts a byte slice to a uint64 using binary.LittleEndian.
// It handles slices larger than 8 bytes by truncating them, and pads smaller ones.
func BytesToUint64LE(b []byte) uint64 {
	// If the slice is smaller than 8 bytes, we need to pad it with zeros
	var tmp [8]byte
	copy(tmp[:], b)

	// Use the highly optimized LittleEndian.Uint64 function
	return binary.LittleEndian.Uint64(tmp[:])
}

// Uint64ToBytesLE converts a uint64 to a byte slice
// It truncates the result to the minimum number of bytes needed.
func Uint64ToBytesLETruncate(u uint64) []byte {
	if u == 0 {
		return []byte{0}
	}

	var result []byte
	for u > 0 {
		result = append(result, byte(u&0xFF))
		u >>= 8
	}

	return result
}

// Uint64ToBytesLEPadded converts a uint64 to a byte slice using binary.LittleEndian.
// It pads the result with zeros to 8 bytes.
func Uint64ToBytesLE(u uint64) []byte {
	// If the slice is smaller than 8 bytes, we need to pad it with zeros
	var padded [8]byte
	binary.LittleEndian.PutUint64(padded[:], u)

	return padded[:]
}
