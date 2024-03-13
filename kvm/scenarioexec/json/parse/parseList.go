package scenjsonparse

import (
	"errors"

	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

func (p *Parser) processStringList(obj interface{}) ([]string, error) {
	listRaw, listOk := obj.(*orderedjson.OJsonList)
	if !listOk {
		return nil, errors.New("not a JSON list")
	}
	var result []string
	for _, elemRaw := range listRaw.AsList() {
		strVal, err := p.parseString(elemRaw)
		if err != nil {
			return nil, err
		}
		result = append(result, strVal)
	}
	return result, nil
}

func (p *Parser) parseValueList(obj interface{}) (scenjsonmodel.JSONValueList, error) {
	listRaw, listOk := obj.(*orderedjson.OJsonList)
	if !listOk {
		return scenjsonmodel.JSONValueList{}, errors.New("not a JSON list")
	}
	var result []scenjsonmodel.JSONBytesFromString
	for _, elemRaw := range listRaw.AsList() {
		ba, err := p.processStringAsByteArray(elemRaw)
		if err != nil {
			return scenjsonmodel.JSONValueList{}, err
		}
		result = append(result, ba)
	}
	return scenjsonmodel.JSONValueList{
		Values: result,
	}, nil
}

func (p *Parser) parseSubTreeList(obj interface{}) ([]scenjsonmodel.JSONBytesFromTree, error) {
	listRaw, listOk := obj.(*orderedjson.OJsonList)
	if !listOk {
		return nil, errors.New("not a JSON list")
	}
	var result []scenjsonmodel.JSONBytesFromTree
	for _, elemRaw := range listRaw.AsList() {
		ba, err := p.processSubTreeAsByteArray(elemRaw)
		if err != nil {
			return nil, err
		}
		result = append(result, ba)
	}
	return result, nil
}

func (p *Parser) parseCheckValueList(obj orderedjson.OJsonObject) (scenjsonmodel.JSONCheckValueList, error) {
	if IsStar(obj) {
		return scenjsonmodel.JSONCheckValueListStar(), nil
	}

	listRaw, listOk := obj.(*orderedjson.OJsonList)
	if listOk {
		return p.parseCheckValueJSONList(listRaw)
	}

	if !p.AllowSingleValueInCheckValueList {
		return scenjsonmodel.JSONCheckValueList{}, errors.New("not a JSON list")
	}

	singleValue, err := p.parseCheckBytes(obj)
	if err != nil {
		return scenjsonmodel.JSONCheckValueList{}, err
	}

	if singleValue.OriginalEmpty() {
		// "" becomes [] instead of [""]
		return scenjsonmodel.JSONCheckValueList{
			Values: []scenjsonmodel.JSONCheckBytes{},
		}, nil
	}

	return scenjsonmodel.JSONCheckValueList{
		Values: []scenjsonmodel.JSONCheckBytes{singleValue},
	}, nil
}

func (p *Parser) parseCheckValueJSONList(listRaw *orderedjson.OJsonList) (scenjsonmodel.JSONCheckValueList, error) {
	var values []scenjsonmodel.JSONCheckBytes
	for _, elemRaw := range listRaw.AsList() {
		checkBytes, err := p.parseCheckBytes(elemRaw)
		if err != nil {
			return scenjsonmodel.JSONCheckValueList{}, err
		}
		values = append(values, checkBytes)
	}
	return scenjsonmodel.JSONCheckValueList{
		Values: values,
	}, nil
}
