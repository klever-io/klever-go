package tools

import (
	"fmt"
	"math"
	"strconv"

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
