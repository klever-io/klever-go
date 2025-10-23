package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/btcsuite/btcutil/bech32"
)

// Hex length
const (
	HexLength8Bits  int = 2
	HexLength16Bits int = 4
	HexLength32Bits int = 8
	HexLength64Bits int = 16

	AddressHexLen int = 64
)

// Bits count
const (
	Bits5   int = 5
	Bits8   int = 8
	Bits16  int = 16
	Bits32  int = 32
	Bits64  int = 64
	Bits128 int = 128
)

// Numerical bases
const (
	BaseHex     int = 16
	BaseDecimal int = 10
)

// Types wrappers
const (
	List     string = "List"
	Option   string = "Option"
	Tuple    string = "tuple"
	Variadic string = "variadic"
)

// Possible Types
const (
	Int8            string = "i8"
	Uint8           string = "u8"
	Int16           string = "i16"
	Uint16          string = "u16"
	Int32           string = "i32"
	Isize           string = "isize"
	Uint32          string = "u32"
	Usize           string = "usize"
	Int64           string = "i64"
	Uint64          string = "u64"
	BigInt          string = "BigInt"
	BigUint         string = "BigUint"
	BigFloat        string = "BigFloat"
	Number          string = "Number"
	Address         string = "Address"
	Boolean         string = "bool"
	ManagedBuffer   string = "ManagedBuffer"
	TokenIdentifier string = "TokenIdentifier"
	Bytes           string = "bytes"
	BoxedBytes      string = "BoxedBytes"
	String          string = "String"
	StrRef          string = "&str"
	VecU8           string = "Vec<u8>"
	SliceU8         string = "&[u8]"
)

// Others
const (
	LengthHexSizer int = 8

	BitsByHexDigit int = 4

	HRP string = "klv"

	AddressBytesLen int = 32

	BigFloatVMPrecision uint = 53
)

type output struct {
	Type string `json:"type"`
}

type endpoint struct {
	Name       string   `json:"name"`
	Mutability string   `json:"mutability"`
	Outputs    []output `json:"outputs"`
}

type field struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type typeInfos struct {
	Type   string  `json:"type"` // struct, tuple, variadic, List, Option
	Fields []field `json:"fields"`
}

type vmOutputData struct {
	Endpoints []endpoint           `json:"endpoints"`
	Types     map[string]typeInfos `json:"types"`
	AbiLoaded bool
}

type VMOutputData interface {
	DecodeHex(endpoint string, hex []string) (interface{}, error)
	DecodeQuery(endpoint string, base64 []string) (interface{}, error)
	LoadAbi(r io.Reader) error
}

func NewVMOutputHandler() VMOutputData {
	return &vmOutputData{}
}

func (a *vmOutputData) LoadAbi(r io.Reader) error {
	jsonBytes, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(jsonBytes, a); err != nil {
		return err
	}

	a.AbiLoaded = true

	return nil
}

// only for single result outputs
func (a *vmOutputData) DecodeHex(endpoint string, hexData []string) (interface{}, error) {
	if !a.AbiLoaded {
		return nil, fmt.Errorf("before decode any value load your abi with `LoadAbi`")
	}

	endpointIndex, err := a.findEndpoint(endpoint)
	if err != nil {
		return nil, err
	}

	if len(hexData) == 1 {
		return a.doDecode(&hexData[0], a.Endpoints[*endpointIndex].Outputs[0].Type, 0)
	}

	var decodedValues []interface{}

	var outputIndex int
	isMultipleOutputs := len(a.Endpoints[*endpointIndex].Outputs) > 1

	for i, h := range hexData {
		if isMultipleOutputs {
			outputIndex = i
		}
		decoded, err := a.initDecode(h, a.Endpoints[*endpointIndex].Outputs[outputIndex].Type, 0)
		if err != nil {
			return nil, fmt.Errorf("error decoding hex value: %w", err)
		}
		decodedValues = append(decodedValues, decoded)
	}

	return decodedValues, nil
}

// mainly for multi result outputs
func (a *vmOutputData) DecodeQuery(endpoint string, base64Data []string) (interface{}, error) {
	var hexData []string

	for _, data := range base64Data {
		dataBytes, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 string: %w", err)
		}
		hexData = append(hexData, hex.EncodeToString(dataBytes))
	}

	return a.DecodeHex(endpoint, hexData)
}

func (a *vmOutputData) findEndpoint(endpointName string) (*int, error) {
	var endpointIndex *int

	for index, endpoint := range a.Endpoints {
		if endpoint.Name == endpointName {
			endpointIndex = &index
			break
		}
	}

	if endpointIndex == nil {
		return nil, fmt.Errorf("endpoint %s not found", endpointName)
	}

	return endpointIndex, nil
}

func SplitTypes(fullType string) (string, string) {
	index := strings.Index(fullType, "<")
	if index == -1 {
		return "", fullType
	}

	wrapperType := fullType[:index]
	valueType := fullType[index+1:]
	valueType = valueType[:len(valueType)-1]

	return wrapperType, valueType
}

func (a *vmOutputData) initDecode(hexValue, fullType string, trim int) (interface{}, error) {
	return a.doDecode(&hexValue, fullType, trim)
}

func (a *vmOutputData) doDecode(hexRef *string, fullType string, trim int) (interface{}, error) {
	wrapperType, valueType := SplitTypes(fullType)

	return a.selectDecoder(hexRef, wrapperType, valueType, trim)
}

func (a *vmOutputData) handleTrim(hexRef *string, trim int) string {
	hexToDecode := (*hexRef)[:trim]

	if trim == 0 {
		hexToDecode = *hexRef
	}

	*hexRef = (*hexRef)[trim:]

	return hexToDecode
}

func (a *vmOutputData) selectDecoder(
	hexRef *string,
	typeWrapper, valueType string,
	trim int,
) (interface{}, error) {
	switch typeWrapper {
	case List:
		return a.decodeList(hexRef, valueType)
	case Option:
		return a.decodeOption(hexRef, valueType)
	case Tuple:
		return a.decodeTuple(hexRef, valueType)
	case Variadic:
		return a.decodeVariadic(hexRef, valueType)
	default:
		return a.decodeSingleValue(hexRef, valueType, trim)
	}
}

func (a *vmOutputData) decodeSingleValue(
	hexRef *string,
	valueType string,
	trim int,
) (interface{}, error) {
	switch valueType {
	case Int8:
		decodedValue, err := a.decodeInt(hexRef, HexLength8Bits)
		return int8(decodedValue), err // #nosec G115 - typed && bit length is checked
	case Int16:
		decodedValue, err := a.decodeInt(hexRef, HexLength16Bits)
		return int16(decodedValue), err // #nosec G115 - typed && bit length is checked
	case Int32, Isize:
		decodedValue, err := a.decodeInt(hexRef, HexLength32Bits)
		return int32(decodedValue), err // #nosec G115 - typed && bit length is checked
	case Int64:
		decodedValue, err := a.decodeInt(hexRef, HexLength64Bits)
		return int64(decodedValue), err // #nosec G115 - typed && bit length is checked
	case Uint8:
		decodedValue, err := a.decodeUint(hexRef, HexLength8Bits)
		return uint8(decodedValue), err // #nosec G115 - typed && bit length is checked
	case Uint16:
		decodedValue, err := a.decodeUint(hexRef, HexLength16Bits)
		return uint16(decodedValue), err // #nosec G115 - typed && bit length is checked
	case Uint32, Usize:
		decodedValue, err := a.decodeUint(hexRef, HexLength32Bits)
		return uint32(decodedValue), err // #nosec G115 - typed && bit length is checked
	case Uint64:
		decodedValue, err := a.decodeUint(hexRef, HexLength64Bits)
		return uint64(decodedValue), err // #nosec G115 - typed && bit length is checked
	case Address:
		return a.decodeAddress(hexRef)
	case BigInt:
		return a.decodeBigInt(hexRef, trim)
	case BigUint:
		return a.decodeBigUint(hexRef, trim)
	case BigFloat:
		return a.decodeBigFloat(hexRef, trim)
	case Boolean:
		return a.decodeBoolean(hexRef), nil
	case
		ManagedBuffer,
		TokenIdentifier,
		Bytes,
		BoxedBytes,
		String,
		StrRef,
		VecU8,
		SliceU8:
		return a.decodeString(hexRef, trim)
	default:
		return a.decodeStruct(hexRef, valueType)
	}
}

func (a *vmOutputData) decodeString(hexRef *string, trim int) (string, error) {
	hexToDecode := a.handleTrim(hexRef, trim*2)

	bytes, err := hex.DecodeString(hexToDecode)

	return string(bytes), err
}

func (a *vmOutputData) decodeAddress(hexRef *string) (string, error) {
	hexToDecode := (*hexRef)[:AddressHexLen]
	*hexRef = (*hexRef)[AddressHexLen:]

	addressBytes, err := hex.DecodeString(hexToDecode)
	if err != nil {
		return "", fmt.Errorf("error convertig hex address to bytes: %w", err)
	}

	if len(addressBytes) != AddressBytesLen {
		return "", fmt.Errorf("decoding address, expected length %d, received %d",
			AddressBytesLen, len(addressBytes))
	}

	address5Bits, err := bech32.ConvertBits(addressBytes, uint8(Bits8), uint8(Bits5), true)
	if err != nil {
		return "", err
	}

	bech32Address, err := bech32.Encode(HRP, address5Bits)

	return bech32Address, err
}

func (a *vmOutputData) decodeBaseUint(hexRef *string, bitSize int) (*uint64, error) {
	uintValue, err := strconv.ParseUint(*hexRef, BaseHex, bitSize)

	return &uintValue, err
}

func (a *vmOutputData) getDynamicTrim(hexRef *string, trimSize int) string {
	var hexToDecode string

	if len(*hexRef) > trimSize {
		hexToDecode = a.handleTrim(hexRef, trimSize)

		return hexToDecode
	}

	hexToDecode = *hexRef
	*hexRef = ""

	return hexToDecode
}

func (a *vmOutputData) decodeUint(hexRef *string, bitsHexLen int) (uint, error) {
	hexToDecode := a.getDynamicTrim(hexRef, bitsHexLen)

	switch rawHexLen := len(hexToDecode); {
	case rawHexLen <= HexLength8Bits:
		targetValue, err := a.decodeBaseUint(&hexToDecode, Bits8)
		return uint(*targetValue), err
	case rawHexLen <= HexLength16Bits:
		targetValue, err := a.decodeBaseUint(&hexToDecode, Bits16)
		return uint(*targetValue), err
	case rawHexLen <= HexLength32Bits:
		targetValue, err := a.decodeBaseUint(&hexToDecode, Bits32)
		return uint(*targetValue), err
	case rawHexLen <= HexLength64Bits:
		targetValue, err := a.decodeBaseUint(&hexToDecode, Bits64)
		return uint(*targetValue), err
	default:
		return 0, fmt.Errorf("invalid hex string %s to dedode to uint", hexToDecode)
	}
}

func (a *vmOutputData) decodeInt(hexRef *string, bitsHexLen int) (int, error) {
	hexToDecode := a.getDynamicTrim(hexRef, bitsHexLen)

	switch rawHexLen := len(hexToDecode); {
	case rawHexLen <= HexLength8Bits:
		targetValue, err := a.decodeBaseUint(&hexToDecode, Bits8)
		return int(int8(*targetValue)), err // #nosec G115
	case rawHexLen <= HexLength16Bits:
		targetValue, err := a.decodeBaseUint(&hexToDecode, Bits16)
		return int(int16(*targetValue)), err // #nosec G115
	case rawHexLen <= HexLength32Bits:
		targetValue, err := a.decodeBaseUint(&hexToDecode, Bits32)
		return int(int32(*targetValue)), err // #nosec G115
	case rawHexLen <= HexLength64Bits:
		targetValue, err := a.decodeBaseUint(&hexToDecode, Bits64)
		return int(int64(*targetValue)), err // #nosec G115
	default:
		return 0, fmt.Errorf("invalid hex string %s to decode to int", hexToDecode)
	}
}

func (a *vmOutputData) decodeBigUint(hexRef *string, trim int) (*big.Int, error) {
	hexToDecode := a.handleTrim(hexRef, trim*2)

	targetValue, err := a.decodeStringBigNumber(&hexToDecode)
	// if that function suceeds, then it was a string representing a decimal big number
	if err == nil {
		return targetValue, nil
	}

	targetValue, ok := new(big.Int).SetString(hexToDecode, BaseHex)
	if !ok {
		return nil, fmt.Errorf("invalid hex string %s to decode to uint64", hexToDecode)
	}

	return targetValue, nil
}

func (a *vmOutputData) decodeBigInt(hexRef *string, trim int) (*big.Int, error) {
	hexToDecode := a.handleTrim(hexRef, trim*2)

	targetValueFromString, err := a.decodeStringBigNumber(&hexToDecode)
	// if that function suceeds, then it was a string representing a decimal number
	if err == nil {
		return targetValueFromString, nil
	}

	targetValueFromInt128, err := a.handleBigIntTill128(&hexToDecode)
	if err == nil {
		return targetValueFromInt128, nil
	}

	targetValue, err := a.decodeInt(&hexToDecode, len(hexToDecode))
	if err != nil {
		return nil, fmt.Errorf("invelid hex string %s to decode to big int", hexToDecode)
	}

	return big.NewInt(int64(targetValue)), nil
}

func (a *vmOutputData) decodeStringBigNumber(hexRef *string) (*big.Int, error) {
	targetString, err := a.decodeString(hexRef, 0)
	if err != nil {
		return nil, err
	}

	targetValue, ok := new(big.Int).SetString(targetString, BaseDecimal)
	if !ok {
		return nil, fmt.Errorf("invalid hex string %s to decode to BigInt", (*hexRef))
	}

	return targetValue, nil
}

func (a *vmOutputData) handleBigIntTill128(hexRef *string) (*big.Int, error) {
	rawValue, ok := new(big.Int).SetString(*hexRef, BaseHex)
	if !ok {
		return nil, fmt.Errorf("invalid hex string to decode to BigInt: %s", *hexRef)
	}

	valueBits := len(*hexRef) * BitsByHexDigit

	two := big.NewInt(2)
	twoToTheNth := new(big.Int).Exp(two, big.NewInt(int64(valueBits-1)), nil) // 2^valueBits
	one := big.NewInt(1)

	MaxIntNBits := new(big.Int).Sub(twoToTheNth, one)

	if rawValue.Cmp(new(big.Int).SetUint64(math.MaxUint64)) == 1 &&
		rawValue.Cmp(MaxIntNBits) == -1 {
		return rawValue, nil
	}

	if rawValue.Cmp(MaxIntNBits) == 1 {
		decodedValue := rawValue.Sub(rawValue, new(big.Int).Lsh(one, uint(valueBits))) // #nosec G115
		return decodedValue, nil
	}

	return nil, fmt.Errorf("%v range is lower range than 128 bits", rawValue)
}

func (a *vmOutputData) decodeBoolean(hexRef *string) bool {
	const BooleanLength int = 2

	hexToDecode := a.getDynamicTrim(hexRef, BooleanLength)
	return hexToDecode == "01"
}

func (a *vmOutputData) decodeBigFloat(hexRef *string, trim int) (interface{}, error) {
	hexToDecode := a.handleTrim(hexRef, trim*2)

	hexBytes, err := hex.DecodeString(hexToDecode)
	if err != nil {
		return nil, fmt.Errorf("error decoding hex string to bytes: %w", err)
	}

	decodedValue := new(big.Float)

	if err := decodedValue.GobDecode(hexBytes); err != nil {
		return nil, fmt.Errorf("error decoding big float: %w", err)
	}

	return decodedValue, nil
}

func IsDynamicLengthType(t string) bool {
	typesMap := map[string]struct{}{
		ManagedBuffer:   {},
		TokenIdentifier: {},
		Bytes:           {},
		BoxedBytes:      {},
		String:          {},
		StrRef:          {},
		VecU8:           {},
		SliceU8:         {},
		BigInt:          {},
		BigUint:         {},
		BigFloat:        {},
	}

	_, exists := typesMap[t]
	return exists
}

func (a *vmOutputData) decodeOption(hexRef *string, valueType string) (interface{}, error) {
	if *hexRef == "" {
		return nil, nil
	}

	const OptionStringLenght int = 2
	a.getDynamicTrim(hexRef, OptionStringLenght)

	hasListPrefix := strings.HasPrefix(valueType, List)

	var trim int

	if IsDynamicLengthType(valueType) || hasListPrefix {
		calculatedTrim, err := a.getFixedTrim(hexRef)
		if err != nil {
			return nil, fmt.Errorf("error while triming option hex string %w", err)
		}

		trim = calculatedTrim
	}

	return a.doDecode(hexRef, valueType, trim)
}

func (a *vmOutputData) getFixedTrim(hexRef *string) (int, error) {
	trimStringHex := (*hexRef)[:LengthHexSizer]

	trimRef, err := a.decodeBaseUint(&trimStringHex, LengthHexSizer*BitsByHexDigit)

	*hexRef = (*hexRef)[LengthHexSizer:]

	return int(*trimRef), err // #nosec G115
}

func (a *vmOutputData) decodeList(hexRef *string, valueType string) (interface{}, error) {
	var result []interface{}
	for len((*hexRef)) > 0 {
		decoded, err := a.handleList(hexRef, valueType)
		if err != nil {
			return nil, fmt.Errorf("error decoding list item: %w", err)
		}

		result = append(result, decoded)
	}

	return result, nil
}

func (a *vmOutputData) handleList(hexRef *string, valueType string) (interface{}, error) {
	wrapperType, coreType := SplitTypes(valueType)

	if wrapperType == List {
		listTrim, err := a.getFixedTrim(hexRef)
		if err != nil {
			return nil, fmt.Errorf("error getting the list trim: %w", err)
		}

		return a.decodeNestedList(hexRef, coreType, listTrim)
	}

	var valueTrim int

	if IsDynamicLengthType(valueType) {
		calculatedTrim, err := a.getFixedTrim(hexRef)
		if err != nil {
			return nil, fmt.Errorf("error getting the list item trim: %w", err)
		}

		valueTrim = calculatedTrim
	}

	return a.decodeSingleValue(hexRef, coreType, valueTrim)
}

func (a *vmOutputData) decodeNestedList(
	hexRef *string,
	valueType string,
	limit int,
) (interface{}, error) {
	var result []interface{}
	for i := 0; i < limit; i++ {
		decoded, err := a.handleList(hexRef, valueType)
		if err != nil {
			return nil, fmt.Errorf("error decoding nested list: %w", err)
		}

		result = append(result, decoded)
	}

	return result, nil
}

func SplitTupleTypes(tuple string) []string {
	var result []string
	var current int
	depth := 0
	start := 0

	for current < len(tuple) {
		switch tuple[current] {
		case '<':
			depth++
		case '>':
			depth--
		case ',':
			if depth == 0 {
				result = append(result, strings.TrimSpace(tuple[start:current]))
				start = current + 1
			}
		}
		current++
	}

	result = append(result, strings.TrimSpace(tuple[start:]))

	return result
}

func (a *vmOutputData) decodeTuple(hexRef *string, valueType string) ([]interface{}, error) {
	var result []interface{}

	types := SplitTupleTypes(valueType)

	for _, t := range types {
		if strings.HasPrefix(t, List) {
			decodedList, err := a.handleList(hexRef, t)
			if err != nil {
				return nil, err
			}

			result = append(result, decodedList)
			continue
		}

		var valueTrim int

		if IsDynamicLengthType(t) {
			calculatedTrim, err := a.getFixedTrim(hexRef)
			if err != nil {
				return nil, fmt.Errorf("error getting the list item trim: %w", err)
			}

			valueTrim = calculatedTrim
		}

		decodedValue, err := a.doDecode(hexRef, t, valueTrim)
		if err != nil {
			return nil, fmt.Errorf("error decoding tuple value %w", err)
		}

		result = append(result, decodedValue)
	}

	return result, nil
}

func (a *vmOutputData) decodeVariadic(hexRef *string, valueType string) (interface{}, error) {
	decodedValue, err := a.doDecode(hexRef, valueType, 0)
	if err != nil {
		return nil, fmt.Errorf("error trying to decode variadic value: %w", err)
	}

	return decodedValue, nil
}

func (a *vmOutputData) decodeStruct(
	hexRef *string,
	valueType string,
) (map[string]interface{}, error) {
	typeDef, exists := a.Types[valueType]
	if !exists {
		return nil, fmt.Errorf("type %s not found in provided abi", valueType)
	}

	result := make(map[string]interface{})

	for _, field := range typeDef.Fields {
		if strings.HasPrefix(field.Type, List) {
			decodedList, err := a.handleList(hexRef, field.Type)
			if err != nil {
				return nil, fmt.Errorf(
					"error %w decoding list value of key %s of custom type %s",
					err,
					field.Type,
					valueType,
				)
			}

			result[field.Name] = decodedList
			continue
		}

		var trim int

		if IsDynamicLengthType(field.Type) {
			calculatedTrim, err := a.getFixedTrim(hexRef)
			if err != nil {
				return nil, fmt.Errorf("error while triming option hex string %w", err)
			}

			trim = calculatedTrim
		}

		decodedValue, err := a.doDecode(hexRef, field.Type, trim)
		if err != nil {
			return nil, fmt.Errorf(
				"error %w decoding value of key %s of custom type %s",
				err,
				field.Type,
				valueType,
			)
		}

		result[field.Name] = decodedValue
	}

	return result, nil
}

func EncodeInput(args []string) (string, error) {
	encodedArgs := ""
	for _, arg := range args {
		encoded, err := doEncode(arg)
		if err != nil {
			return "", fmt.Errorf("invalid item to encode `%s`", arg)
		}

		encodedArgs += "@" + encoded
	}

	return encodedArgs, nil
}

func doEncode(arg string) (string, error) {
	kv := strings.SplitN(arg, ":", 2)

	isOption := false
	// check if it is an option argument
	if strings.HasPrefix(kv[0], "option") {
		isOption = true
		kv[0] = kv[0][6:]
	}

	if len(kv) < 2 {
		return "", fmt.Errorf("invalid encoding type: %s", arg)
	}

	encoded, err := encodeSingleValue(kv[0], kv[1], isOption)
	if err != nil {
		return "", err
	}

	if isOption {
		encoded = "01" + encoded
	}

	return encoded, nil
}

func encodeSingleValue(t, v string, isNested bool) (string, error) {
	switch t {
	case
		Int8, strings.ToUpper(Int8), // int8
		Int16, strings.ToUpper(Int16), // int16
		Int32, strings.ToUpper(Int32), Isize, strings.ToUpper(Isize), // int32
		Int64, strings.ToUpper(Int64): // int864

		return encodeInt(v, t, isNested)
	case
		Uint8, strings.ToUpper(Uint8), // uint8
		Uint16, strings.ToUpper(Uint16), // uint16
		Uint32, strings.ToUpper(Uint32), Usize, strings.ToUpper(Usize), // uint32
		Uint64, strings.ToUpper(Uint64): // uint864

		return encodeUint(v, t, isNested)
	case BigInt, strings.ToLower(BigInt),
		BigUint, strings.ToLower(BigUint),
		"bi", "n", "BI", "N": // BigInt and BigUint

		return encodeBigInt(v, isNested)
	case Number, strings.ToLower(Number): // Auto handler number

		return encodeNumber(v, isNested)
	case Address, strings.ToLower(Address), "a", "A":
		return encodeAddress(v)
	case BigFloat, strings.ToLower(BigFloat),
		"bf", "BF", "f", "F": // BigFloat:

		return encodeBigFloat(v, isNested)
	case
		ManagedBuffer, strings.ToLower(ManagedBuffer),
		TokenIdentifier, strings.ToLower(TokenIdentifier),
		Bytes,
		BoxedBytes, strings.ToLower(BoxedBytes),
		String, strings.ToLower(String),
		VecU8, strings.ToLower(VecU8),
		StrRef,
		SliceU8: // All types of strings

		return encodeString(v, isNested), nil
	case "bool", "boolean", "b", "B":
		return encodeBoolean(v, isNested)
	case "empty", "0", "e", "E":
		return "", nil
	case "file", "code", "wasm":
		return encodeFile(v)
	case "hex":
		return v, nil
	default:
		return "", fmt.Errorf("invalid encode type `%s`", t)
	}
}

func encodeInt(v, t string, isNested bool) (string, error) {
	if isNested {
		return encodeNestedInt(v, t)
	}

	return encodeTopLevelInt(v)
}

func encodeTopLevelInt(v string) (string, error) {
	rawInt, err := strconv.ParseInt(v, BaseDecimal, Bits64)
	if err != nil {
		return "", fmt.Errorf("invalid string `%s` to convert to signed integer", v)
	}

	var encoded string
	switch {
	// typecast to uint to correspondent bit size to use 2's complement in negative values
	case rawInt >= math.MinInt8 && rawInt <= math.MaxInt8:
		encoded = fmt.Sprintf("%02x", uint8(rawInt)) // #nosec G115 - type specific
	case rawInt >= math.MinInt16 && rawInt <= math.MaxInt16:
		encoded = fmt.Sprintf("%04x", uint16(rawInt)) // #nosec G115 - type specific
	case rawInt >= math.MinInt32 && rawInt <= math.MaxInt32:
		encoded = fmt.Sprintf("%08x", uint32(rawInt)) // #nosec G115 - type specific
	default:
		encoded = fmt.Sprintf("%016x", uint64(rawInt)) // #nosec G115 - type specific
	}

	if len(encoded)%2 != 0 {
		prefix := "0"
		if rawInt < 0 {
			prefix = "f"
		}
		encoded = prefix + encoded
	}

	return encoded, nil
}

func encodeNestedInt(v, t string) (string, error) {
	rawInt, err := strconv.ParseInt(v, BaseDecimal, Bits64)
	if err != nil {
		return "", fmt.Errorf("invalid string `%s` to convert to signed integer", v)
	}
	switch t {
	// typecast to uint to correspondent bit size to use 2's complement in negative values
	case Int8:
		return fmt.Sprintf("%02x", uint8(rawInt)), nil // #nosec G115 - type specific
	case Int16:
		return fmt.Sprintf("%04x", uint16(rawInt)), nil // #nosec G115 - type specific
	case Int32:
		return fmt.Sprintf("%08x", uint32(rawInt)), nil // #nosec G115 - type specific
	default:
		return fmt.Sprintf("%016x", uint64(rawInt)), nil // #nosec G115 - type specific
	}
}

func encodeUint(v, t string, isNested bool) (string, error) {
	if isNested {
		return encodeNestedUint(v, t)
	}

	return encodeTopLevelUint(v)
}

func encodeTopLevelUint(v string) (string, error) {
	rawUint, err := strconv.ParseUint(v, BaseDecimal, Bits64)
	if err != nil {
		return "", fmt.Errorf("invalid string `%s` to convert to signed integer", v)
	}

	var encoded string
	switch {
	case rawUint <= math.MaxUint8:
		encoded = fmt.Sprintf("%x", uint8(rawUint)) // #nosec G115 - type specific
	case rawUint <= math.MaxUint16:
		encoded = fmt.Sprintf("%x", uint16(rawUint)) // #nosec G115 - type specific
	case rawUint <= math.MaxUint32:
		encoded = fmt.Sprintf("%x", uint32(rawUint)) // #nosec G115 - type specific
	default:
		encoded = fmt.Sprintf("%x", rawUint)
	}

	if len(encoded)%2 != 0 {
		encoded = "0" + encoded
	}

	return encoded, nil
}

func encodeNestedUint(v, t string) (string, error) {
	rawInt, err := strconv.ParseUint(v, BaseDecimal, Bits64)
	if err != nil {
		return "", fmt.Errorf("invalid string `%s` to convert to signed integer", v)
	}
	switch t {
	case Int8:
		if rawInt > math.MaxUint8 {
			return "", fmt.Errorf("value `%d` overflows uint8", rawInt)
		}
		return fmt.Sprintf("%02x", uint8(rawInt)), nil // #nosec G115 - type specific
	case Int16:
		if rawInt > math.MaxUint16 {
			return "", fmt.Errorf("value `%d` overflows uint16", rawInt)
		}
		return fmt.Sprintf("%04x", uint16(rawInt)), nil // #nosec G115 - type specific
	case Int32:
		if rawInt > math.MaxUint32 {
			return "", fmt.Errorf("value `%d` overflows uint32", rawInt)
		}
		return fmt.Sprintf("%08x", uint32(rawInt)), nil // #nosec G115 - type specific
	default:
		return fmt.Sprintf("%016x", rawInt), nil
	}
}

func encodeAddress(v string) (string, error) {
	address, err := walletPubKeyConverter.Decode(v)
	if err != nil {
		return "", fmt.Errorf("invalid address: %s", v)
	}

	return hex.EncodeToString(address), nil
}

func encodeString(v string, isNested bool) string {
	hexString := ""

	for i := 0; i < len(v); i++ {
		hexString += fmt.Sprintf("%02x", v[i])
	}

	if isNested {
		hexLen := fmt.Sprintf("%08x", len(hexString)/2)
		hexString = hexLen + hexString
	}

	return hexString
}

func encodeBoolean(v string, isNested bool) (string, error) {
	switch v {
	case "true":
		return "01", nil
	case "false":
		if isNested {
			return "00", nil
		}
		return "", nil
	default:
		return "", fmt.Errorf("invalid boolean to encode, must be `true` or `false`")
	}
}

func encodeFile(v string) (string, error) {
	// v is the path of file

	bytecode, err := os.ReadFile(filepath.Clean(v))
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// check if the file is a .kleversc.json
	if isKleverSC := strings.HasSuffix(v, ".kleversc.json"); isKleverSC {
		kleverJson := struct {
			Code string `json:"code"`
		}{}

		err := json.Unmarshal(bytecode, &kleverJson)
		if err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}

		return kleverJson.Code, nil
	}

	return hex.EncodeToString(bytecode), nil
}

func encodeBigInt(v string, isNested bool) (string, error) {
	bi, ok := new(big.Int).SetString(v, BaseDecimal)
	if !ok {
		return "", fmt.Errorf("invalid string `%s` to convert to big integer", v)
	}

	if bi.BitLen() > Bits128 {
		return fmt.Sprintf("%x", v), nil
	}

	if bi.BitLen() <= Bits128 && bi.Sign() == -1 {
		bigIntTwosComplement(bi)
	}

	hexBi := fmt.Sprintf("%x", bi)

	if len(hexBi)%2 != 0 {
		if strings.HasPrefix(v, "-") {
			hexBi = "f" + hexBi
		} else {
			hexBi = "0" + hexBi
		}
	}

	if isNested {
		hexLen := fmt.Sprintf("%08x", len(hexBi)/2)
		hexBi = hexLen + hexBi
	}

	return hexBi, nil
}

func bigIntTwosComplement(b *big.Int) {
	bitsLen := b.BitLen()
	// Here, even though `bitsLen` isn't a multiple of 4, by
	// adding 4 to it we are ensuring that bitMask created later is large enough
	// to cover all bits of the original value `b` in the Xor operation.
	bitsLen += BitsByHexDigit

	maskHexLen := bitsLen / BitsByHexDigit
	maskStringHex := strings.Repeat("f", maskHexLen)
	bitMask, _ := new(big.Int).SetString(maskStringHex, BaseHex)

	b.Abs(b)

	b.Xor(b, bitMask)

	b.Add(b, big.NewInt(1))
}

func encodeBigFloat(v string, isNested bool) (string, error) {
	bf, ok := new(big.Float).SetString(v)
	if !ok {
		return "", fmt.Errorf(
			"invalid string `%s` representing a big float to encode to hexadecimal",
			v,
		)
	}

	bf.SetPrec(BigFloatVMPrecision)
	bf.SetMode(big.RoundingMode(big.Exact))

	bfBytes, err := bf.GobEncode()
	if err != nil {
		return "", fmt.Errorf(
			"invalid string `%s` representing a big float to encode to hexadecimal",
			v,
		)
	}

	hexBf := hex.EncodeToString(bfBytes)

	if isNested {
		hexLen := fmt.Sprintf("%08x", len(hexBf)/2)
		hexBf = hexLen + hexBf
	}

	return hexBf, nil
}

func encodeNumber(v string, isNested bool) (string, error) {
	if signedInt, err := strconv.ParseInt(v, BaseDecimal, Bits64); err == nil {
		if signedInt < 0 {
			intType := determineIntType(signedInt)
			log.Info(fmt.Sprintf("Auto type of %s", v), "Type", intType)
			return encodeInt(v, intType, isNested)
		}
		uintType := determineUintType(uint64(signedInt))
		log.Info(fmt.Sprintf("Auto type of %s", v), "Type", uintType)
		return encodeUint(v, uintType, isNested)
	}

	return "", fmt.Errorf("invalid number format: %s", v)
}

// determineIntType determines the most appropriate signed integer type based on value
func determineIntType(value int64) string {
	switch {
	case value >= math.MinInt8 && value <= math.MaxInt8:
		return Int8
	case value >= math.MinInt16 && value <= math.MaxInt16:
		return Int16
	case value >= math.MinInt32 && value <= math.MaxInt32:
		return Int32
	default:
		return Int64
	}
}

// determineUintType determines the most appropriate unsigned integer type based on value
func determineUintType(value uint64) string {
	switch {
	case value <= math.MaxUint8:
		return Uint8
	case value <= math.MaxUint16:
		return Uint16
	case value <= math.MaxUint32:
		return Uint32
	default:
		return Uint64
	}
}
