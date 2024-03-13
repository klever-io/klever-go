package scenjsonparse

import (
	"errors"
	"fmt"

	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

func (p *Parser) processNewAddressMocks(namsRaw orderedjson.OJsonObject) ([]*scenjsonmodel.NewAddressMock, error) {
	namList, isList := namsRaw.(*orderedjson.OJsonList)
	if !isList {
		return nil, errors.New("newAddresses list is not a list")
	}
	var namEntries []*scenjsonmodel.NewAddressMock
	var err error
	for _, namRaw := range namList.AsList() {
		namMap, isMap := namRaw.(*orderedjson.OJsonMap)
		if !isMap {
			return nil, errors.New("new address mock entry is not a map")
		}
		namEntry := scenjsonmodel.NewAddressMock{}
		for _, kvp := range namMap.OrderedKV {
			switch kvp.Key {
			case "creatorAddress":
				caStr, err := p.parseString(kvp.Value)
				if err != nil {
					return nil, fmt.Errorf("creatorAddress is not a json string: %w", err)
				}
				namEntry.CreatorAddress, err = p.parseAccountAddress(caStr)
				if err != nil {
					return nil, err
				}
			case "creatorNonce":
				namEntry.CreatorNonce, err = p.processUint64(kvp.Value)
				if err != nil {
					return nil, errors.New("invalid creatorNonce")
				}
			case "newAddress":
				naStr, err := p.parseString(kvp.Value)
				if err != nil {
					return nil, fmt.Errorf("newAddress is not a json string: %w", err)
				}
				namEntry.NewAddress, err = p.parseAccountAddress(naStr)
				if err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("unknown nam field: %s", kvp.Key)
			}
		}
		namEntries = append(namEntries, &namEntry)
	}

	return namEntries, nil
}
