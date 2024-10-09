package kapps

import (
	reflect "reflect"
	strconv "strconv"
)

type Parser func([]byte) (reflect.Value, error)

var parsers = map[EnumType]Parser{
	EnumType_Bool:       parseBool,
	EnumType_Int8:       parseInt(8),
	EnumType_Int16:      parseInt(16),
	EnumType_Int32:      parseInt(32),
	EnumType_Int64:      parseInt(64),
	EnumType_Uint8:      parseUint(8),
	EnumType_Uint16:     parseUint(16),
	EnumType_Uint32:     parseUint(32),
	EnumType_Uint64:     parseUint(64),
	EnumType_Float32:    parseFloat(32),
	EnumType_Float64:    parseFloat(64),
	EnumType_Complex64:  parseComplex(64),
	EnumType_Complex128: parseComplex(128),
	EnumType_String:     parseString,
	EnumType_Bytes:      parseBytes,
}

func parseBool(value []byte) (reflect.Value, error) {
	v, err := strconv.ParseBool(string(value))
	return reflect.ValueOf(v), err
}

func parseInt(bitSize int) Parser {
	return func(value []byte) (reflect.Value, error) {
		v, err := strconv.ParseInt(string(value), 10, bitSize)
		return reflect.ValueOf(v), err
	}
}

func parseUint(bitSize int) Parser {
	return func(value []byte) (reflect.Value, error) {
		v, err := strconv.ParseUint(string(value), 10, bitSize)
		return reflect.ValueOf(v), err
	}
}

func parseFloat(bitSize int) Parser {
	return func(value []byte) (reflect.Value, error) {
		v, err := strconv.ParseFloat(string(value), bitSize)
		return reflect.ValueOf(v), err
	}
}

func parseComplex(bitSize int) Parser {
	return func(value []byte) (reflect.Value, error) {
		v, err := strconv.ParseComplex(string(value), bitSize)
		return reflect.ValueOf(v), err
	}
}

func parseString(value []byte) (reflect.Value, error) {
	return reflect.ValueOf(string(value)), nil
}

func parseBytes(value []byte) (reflect.Value, error) {
	return reflect.ValueOf(value), nil
}
