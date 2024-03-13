package er

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	scenexpressioninterpreter "github.com/klever-io/klever-go/kvm/scenarioexec/expression/interpreter"
)

// ExprReconstructorHint type definition
type ExprReconstructorHint uint64

const (
	// NoHint indicates that the type if not known
	NoHint ExprReconstructorHint = iota

	// NumberHint hints that value should be a number
	NumberHint

	// AddressHint hints that value should be an address
	AddressHint

	// StrHint hints that value should be a string expression, e.g. a username, "str:..."
	StrHint

	// CodeHint hints that value should be a smart contract code, normally loaded from a file
	CodeHint
)

const maxBytesInterpretedAsNumber = 15

// ExprReconstructor is a component that attempts to convert raw bytes to a human-readable format.
type ExprReconstructor struct {
	Bech32Addr bool
}

// Reconstruct will return the string representation of the provided value
func (er *ExprReconstructor) Reconstruct(value []byte, hint ExprReconstructorHint) string {
	switch hint {
	case NumberHint:
		return fmt.Sprintf("%d", big.NewInt(0).SetBytes(value))
	case StrHint:
		return fmt.Sprintf("str:%s", string(value))
	case AddressHint:
		return addressPretty(value, er.Bech32Addr)
	case CodeHint:
		return codePretty(value)
	default:
		return unknownByteArrayPretty(value)
	}
}

// ReconstructFromBigInt will return the string of the provided big int
func (er *ExprReconstructor) ReconstructFromBigInt(value *big.Int) string {
	return er.Reconstruct(value.Bytes(), NumberHint)
}

// ReconstructFromUint64 will return the string of the provided uint64
func (er *ExprReconstructor) ReconstructFromUint64(value uint64) string {
	return er.Reconstruct(big.NewInt(0).SetUint64(value).Bytes(), NumberHint)
}

// ReconstructList will return the string of the provided values list
func (er *ExprReconstructor) ReconstructList(values [][]byte, hint ExprReconstructorHint) string {
	var strs []string
	for _, value := range values {
		strs = append(strs, "\""+er.Reconstruct(value, hint)+"\"")
	}

	return "[" + strings.Join(strs, ", ") + "]"
}

func unknownByteArrayPretty(bytes []byte) string {
	if len(bytes) == 0 {
		return ""
	}

	// fully interpret as string
	if canInterpretAsString(bytes) {
		return fmt.Sprintf("0x%s (str:%s)", hex.EncodeToString(bytes), string(bytes))
	}

	// interpret as number
	if len(bytes) < maxBytesInterpretedAsNumber {
		asInt := big.NewInt(0).SetBytes(bytes)
		return fmt.Sprintf("0x%s (%d)", hex.EncodeToString(bytes), asInt)
	}

	// default interpret as string with escaped bytes
	return fmt.Sprintf("0x%s (str:%s)", hex.EncodeToString(bytes), strconv.Quote(string(bytes)))
}

func addressPretty(value []byte, bech32Addr bool) string {
	if len(value) != 32 {
		return unknownByteArrayPretty(value)
	}

	if bech32Addr {
		return bech32Pretty(value)
	}

	// smart contract addresses
	leadingZeros := make([]byte, scenexpressioninterpreter.SCAddressNumLeadingZeros)
	if bytes.Equal(value[:scenexpressioninterpreter.SCAddressNumLeadingZeros], leadingZeros) {
		if value[31] == byte('_') {
			addrStr := string(value[scenexpressioninterpreter.SCAddressReservedPrefixLength:])
			addrStr = strings.TrimRight(addrStr, "_")
			return fmt.Sprintf("sc:%s", addrStr)
		} else {
			// last byte is the shard id and is explicit
			addrStr := string(value[scenexpressioninterpreter.SCAddressReservedPrefixLength:31])
			addrStr = strings.TrimRight(addrStr, "_")
			shardID := value[31]
			return fmt.Sprintf("sc:%s#%x", addrStr, shardID)
		}
	}

	// regular addresses
	if value[31] == byte('_') {
		addrStr := string(value)
		addrStr = strings.TrimRight(addrStr, "_")
		return fmt.Sprintf("address:%s", addrStr)
	} else {
		// last byte is the shard id and is explicit
		addrStr := string(value[:31])
		addrStr = strings.TrimRight(addrStr, "_")
		shard_id := value[31]
		address_expr := fmt.Sprintf("address:%s#%02x", addrStr, shard_id)
		if !canInterpretAsString(value[:31]) {
			return fmt.Sprintf("0x%s (%s)", hex.EncodeToString(value), address_expr)
		}
		return address_expr
	}
}

func canInterpretAsString(bytes []byte) bool {
	if len(bytes) == 0 {
		return false
	}
	for _, b := range bytes {
		if b < 32 || b > 126 {
			return false
		}
	}
	return true
}

func codePretty(bytes []byte) string {
	if len(bytes) == 0 {
		return ""
	}
	encoded := hex.EncodeToString(bytes)

	if len(encoded) > 20 {
		return fmt.Sprintf("0x%s...", encoded[:20])
	}

	return fmt.Sprintf("0x%s", encoded)
}

func bech32Pretty(pkBytes []byte) string {
	if len(pkBytes) == 0 {
		return ""
	}
	addressLen := 32
	bpc, _ := pubkeyConverter.NewBech32PubkeyConverter(addressLen)
	encoded := bpc.Encode(pkBytes)

	if len(encoded) > 20 {
		return fmt.Sprintf("bech32:%s", encoded[:62])
	}

	return fmt.Sprintf("bech32:%s", encoded)
}
