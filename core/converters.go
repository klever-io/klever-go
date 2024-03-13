package core

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/klever-io/klever-go/common"
)

// IsUnknownEpochIdentifier return if the epoch identifier represents unknown epoch
func IsUnknownEpochIdentifier(identifier []byte) (bool, error) {
	splitString := strings.Split(string(identifier), "_")
	if len(splitString) < 2 || len(splitString[1]) == 0 {
		return false, common.ErrInvalidIdentifierForEpochStartBlockRequest
	}

	epoch, err := strconv.ParseUint(splitString[1], 10, 32)
	if err != nil {
		return false, common.ErrInvalidIdentifierForEpochStartBlockRequest
	}

	if epoch == math.MaxUint32 {
		return true, nil
	}

	return false, nil
}

// EpochStartIdentifier returns the string for the epoch start identifier
func EpochStartIdentifier(epoch uint32) string {
	return fmt.Sprintf("epochStartBlock_%d", epoch)
}

// ConvertToEvenHex converts the provided value in a hex string, even number of characters
func ConvertToEvenHex(value int) string {
	str := fmt.Sprintf("%x", value)
	if len(str)%2 != 0 {
		str = "0" + str
	}

	return str
}

// ConvertToEvenHexBigInt converts the provided value in a hex string, even number of characters
func ConvertToEvenHexBigInt(value *big.Int) string {
	str := value.Text(16)
	if len(str)%2 != 0 {
		str = "0" + str
	}

	return str
}
