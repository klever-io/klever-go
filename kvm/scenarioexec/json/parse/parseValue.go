package scenjsonparse

import (
	"errors"
	"math/big"

	twos "github.com/klever-io/klever-go/kvm/math/twos-complement"
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

type bigIntParseFormat int

const (
	bigIntSignedBytes bigIntParseFormat = iota
	bigIntUnsignedBytes
)

func (p *Parser) processCheckBigInt(obj orderedjson.OJsonObject, format bigIntParseFormat) (scenjsonmodel.JSONCheckBigInt, error) {
	if IsStar(obj) {
		// "*" means any value, skip checking it
		return scenjsonmodel.JSONCheckBigInt{
			Value:    nil,
			IsStar:   true,
			Original: "*"}, nil
	}

	jbi, err := p.processBigInt(obj, format)
	if err != nil {
		return scenjsonmodel.JSONCheckBigInt{}, err
	}
	return scenjsonmodel.JSONCheckBigInt{
		Value:    jbi.Value,
		IsStar:   false,
		Original: jbi.Original,
	}, nil
}

func (p *Parser) processBigInt(obj orderedjson.OJsonObject, format bigIntParseFormat) (scenjsonmodel.JSONBigInt, error) {
	strVal, err := p.parseString(obj)
	if err != nil {
		return scenjsonmodel.JSONBigInt{}, err
	}

	bi, err := p.parseBigInt(strVal, format)
	return scenjsonmodel.JSONBigInt{
		Value:    bi,
		Original: strVal,
	}, err
}

func (p *Parser) parseBigInt(strRaw string, format bigIntParseFormat) (*big.Int, error) {
	bytes, err := p.ExprInterpreter.InterpretString(strRaw)
	if err != nil {
		return nil, err
	}
	switch format {
	case bigIntSignedBytes:
		return twos.FromBytes(bytes), nil
	case bigIntUnsignedBytes:
		return big.NewInt(0).SetBytes(bytes), nil
	default:
		return nil, errors.New("unknown format requested")
	}
}

func (p *Parser) processCheckUint64(obj orderedjson.OJsonObject) (scenjsonmodel.JSONCheckUint64, error) {
	if IsStar(obj) {
		// "*" means any value, skip checking it
		return scenjsonmodel.JSONCheckUint64{
			Value:    0,
			IsStar:   true,
			Original: "*"}, nil
	}

	ju, err := p.processUint64(obj)
	if err != nil {
		return scenjsonmodel.JSONCheckUint64{}, err
	}
	return scenjsonmodel.JSONCheckUint64{
		Value:    ju.Value,
		IsStar:   false,
		Original: ju.Original}, nil

}

func (p *Parser) processUint64(obj orderedjson.OJsonObject) (scenjsonmodel.JSONUint64, error) {
	bi, err := p.processBigInt(obj, bigIntUnsignedBytes)
	if err != nil {
		return scenjsonmodel.JSONUint64{}, err
	}

	if bi.Value == nil || !bi.Value.IsUint64() {
		return scenjsonmodel.JSONUint64{}, errors.New("value is not uint64")
	}

	return scenjsonmodel.JSONUint64{
		Value:    bi.Value.Uint64(),
		Original: bi.Original}, nil
}

func (p *Parser) parseCheckBytes(obj orderedjson.OJsonObject) (scenjsonmodel.JSONCheckBytes, error) {
	if IsStar(obj) {
		// "*" means any value, skip checking it
		return scenjsonmodel.JSONCheckBytesStar(), nil
	}

	jb, err := p.processSubTreeAsByteArray(obj)
	if err != nil {
		return scenjsonmodel.JSONCheckBytes{}, err
	}
	return scenjsonmodel.JSONCheckBytes{
		Value:    jb.Value,
		IsStar:   false,
		Original: jb.Original,
	}, nil
}

func (p *Parser) processStringAsByteArray(obj orderedjson.OJsonObject) (scenjsonmodel.JSONBytesFromString, error) {
	strVal, err := p.parseString(obj)
	if err != nil {
		return scenjsonmodel.JSONBytesFromString{}, err
	}
	result, err := p.ExprInterpreter.InterpretString(strVal)
	return scenjsonmodel.NewJSONBytesFromString(result, strVal), err
}

func (p *Parser) processSubTreeAsByteArray(obj orderedjson.OJsonObject) (scenjsonmodel.JSONBytesFromTree, error) {
	value, err := p.ExprInterpreter.InterpretSubTree(obj)
	return scenjsonmodel.JSONBytesFromTree{
		Value:    value,
		Original: obj,
	}, err
}

func (p *Parser) parseString(obj orderedjson.OJsonObject) (string, error) {
	str, isStr := obj.(*orderedjson.OJsonString)
	if !isStr {
		return "", errors.New("not a string value")
	}
	return str.Value, nil
}

func (p *Parser) parseBool(obj orderedjson.OJsonObject) (bool, error) {
	value, isBool := obj.(*orderedjson.OJsonBool)
	if !isBool {
		return false, errors.New("not a bool value")
	}
	return bool(*value), nil
}

// IsStar returns whether check object is othe form "*".
func IsStar(obj orderedjson.OJsonObject) bool {
	str, isStr := obj.(*orderedjson.OJsonString)
	if !isStr {
		return false
	}
	return str.Value == "*"
}
