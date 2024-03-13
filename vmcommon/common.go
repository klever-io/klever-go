package vmcommon

import (
	"math/big"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/multiformats/go-base36"
)

const tickerMinLength = core.MinLengthForAssetTicker
const tickerMaxLength = core.MaxLengthForAssetTicker
const additionalRandomCharsLength = kdautils.TickerRandomSequenceLength
const identifierMinLength = tickerMinLength + additionalRandomCharsLength + 1
const identifierMaxLength = tickerMaxLength + additionalRandomCharsLength + 1

// ValidateToken - validates the token ID
func ValidateToken(tokenID []byte) bool {
	tokenIDLen := len(tokenID)
	if tokenIDLen < identifierMinLength || tokenIDLen > identifierMaxLength {
		return false
	}

	tickerLen := tokenIDLen - additionalRandomCharsLength

	if !isTickerValid(tokenID[0 : tickerLen-1]) {
		return false
	}

	// dash char between the random chars and the ticker
	if tokenID[tickerLen-1] != kdautils.TickerSeparatorByte {
		return false
	}

	if !randomCharsAreValid(tokenID[tickerLen:tokenIDLen]) {
		return false
	}

	return true
}

// ticker must be all uppercase alphanumeric
func isTickerValid(tickerName []byte) bool {
	if len(tickerName) < tickerMinLength || len(tickerName) > tickerMaxLength {
		return false
	}
	for _, ch := range tickerName {
		isBigCharacter := ch >= 'A' && ch <= 'Z'
		isNumber := ch >= '0' && ch <= '9'
		isReadable := isBigCharacter || isNumber
		if !isReadable {
			return false
		}
	}

	return true
}

// random chars are alphanumeric lowercase
func randomCharsAreValid(chars []byte) bool {
	if len(chars) != additionalRandomCharsLength {
		return false
	}

	// check if random sequence is a valid base36 encoding
	_, err := base36.DecodeString(string(chars))

	return err == nil
}

// ZeroValueIfNil returns 0 if the input is nil, otherwise returns the input
func ZeroValueIfNil(value *big.Int) *big.Int {
	if value == nil {
		return big.NewInt(0)
	}

	return value
}

func ParseVMTypeFromContractAddress(contractAddress []byte) ([]byte, error) {
	if len(contractAddress) < core.NumInitCharactersForScAddress {
		return nil, ErrInvalidVMType
	}

	startIndex := core.NumInitCharactersForScAddress - core.VMTypeLen
	endIndex := core.NumInitCharactersForScAddress
	return contractAddress[startIndex:endIndex], nil
}
