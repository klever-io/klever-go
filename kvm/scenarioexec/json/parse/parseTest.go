package scenjsonparse

import (
	"errors"
	"fmt"

	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

// ParseTestFile converts json string to object representation
func (p *Parser) ParseTestFile(jsonString []byte) ([]*scenjsonmodel.Test, error) {

	jobj, err := orderedjson.ParseOrderedJSON(jsonString)
	if err != nil {
		return nil, err
	}

	topMap, isMap := jobj.(*orderedjson.OJsonMap)
	if !isMap {
		return nil, errors.New("unmarshalled test top level object is not a map")
	}

	var top []*scenjsonmodel.Test
	for _, kvp := range topMap.OrderedKV {
		t, tErr := p.processTest(kvp.Value)
		if tErr != nil {
			return nil, tErr
		}
		t.TestName = kvp.Key
		top = append(top, t)
	}
	return top, nil
}

func (p *Parser) processTest(testObj orderedjson.OJsonObject) (*scenjsonmodel.Test, error) {
	testMap, isTestMap := testObj.(*orderedjson.OJsonMap)
	if !isTestMap {
		return nil, errors.New("unmarshalled test object is not a map")
	}
	test := scenjsonmodel.Test{CheckGas: true}

	var err error
	for _, kvp := range testMap.OrderedKV {
		switch kvp.Key {
		case "checkGas":
			checkGasOJ, isBool := kvp.Value.(*orderedjson.OJsonBool)
			if !isBool {
				return nil, errors.New("unmarshalled test checkGas flag is not boolean")
			}
			test.CheckGas = bool(*checkGasOJ)
		case "pre":
			test.Pre, err = p.processAccountMap(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("cannot parse pre: %w", err)
			}
		case "blocks":
			blocksRaw, blocksOk := kvp.Value.(*orderedjson.OJsonList)
			if !blocksOk {
				return nil, errors.New("unmarshalled blocks object is not a list")
			}
			for _, blRaw := range blocksRaw.AsList() {
				bl, blErr := p.processBlock(blRaw)
				if blErr != nil {
					return nil, blErr
				}
				test.Blocks = append(test.Blocks, bl)
			}
		case "network":
			test.Network, err = p.parseString(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("test network value not a string: %w", err)
			}

		case "blockHashes":
			test.BlockHashes, err = p.parseValueList(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("unmarshalled blockHashes object is not a list: %w", err)
			}
		case "postState":
			test.PostState, err = p.processCheckAccountMap(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("cannot parse postState: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown test: %s", kvp.Key)
		}
	}

	return &test, nil
}
