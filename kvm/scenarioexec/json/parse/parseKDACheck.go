package scenjsonparse

import (
	"errors"
	"fmt"

	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

func (p *Parser) processCheckKDAData(
	tokenName scenjsonmodel.JSONBytesFromString,
	kdaDataRaw orderedjson.OJsonObject) (*scenjsonmodel.CheckKDAData, error) {

	switch data := kdaDataRaw.(type) {
	case *orderedjson.OJsonString:
		// simple string representing balance "400,000,000,000"
		kdaData := scenjsonmodel.CheckKDAData{
			TokenIdentifier: tokenName,
		}
		balance, err := p.processCheckBigInt(kdaDataRaw, bigIntUnsignedBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid KDA balance: %w", err)
		}
		kdaData.Instances = []*scenjsonmodel.CheckKDAInstance{
			{
				Nonce:   scenjsonmodel.JSONUint64Zero(),
				Balance: balance,
			},
		}
		return &kdaData, nil
	case *orderedjson.OJsonMap:
		return p.processCheckKDADataMap(tokenName, data)
	default:
		return nil, errors.New("invalid JSON object for KDA")
	}
}

// Map containing KDA fields, e.g.:
//
//	{
//		"instances": [ ... ],
//	 "lastNonce": "5",
//		"frozen": "true"
//	}
func (p *Parser) processCheckKDADataMap(tokenName scenjsonmodel.JSONBytesFromString, kdaDataMap *orderedjson.OJsonMap) (*scenjsonmodel.CheckKDAData, error) {
	kdaData := scenjsonmodel.CheckKDAData{
		TokenIdentifier: tokenName,
	}
	// var err error
	firstInstance := &scenjsonmodel.CheckKDAInstance{
		Nonce:      scenjsonmodel.JSONUint64Zero(),
		Balance:    scenjsonmodel.JSONCheckBigIntUnspecified(),
		Creator:    scenjsonmodel.JSONCheckBytesUnspecified(),
		Royalties:  scenjsonmodel.JSONCheckUint64Unspecified(),
		Hash:       scenjsonmodel.JSONCheckBytesUnspecified(),
		Uris:       scenjsonmodel.JSONCheckValueListUnspecified(),
		Attributes: scenjsonmodel.JSONCheckBytesUnspecified(),
	}
	firstInstanceLoaded := false
	var explicitInstances []*scenjsonmodel.CheckKDAInstance

	for _, kvp := range kdaDataMap.OrderedKV {
		// it is allowed to load the instance directly, fields set to the first instance
		instanceFieldLoaded, err := p.tryProcessCheckKDAInstanceField(kvp, firstInstance)
		if err != nil {
			return nil, fmt.Errorf("invalid account KDA instance field: %w", err)
		}
		if instanceFieldLoaded {
			firstInstanceLoaded = true
		} else {
			switch kvp.Key {
			case "instances":
				explicitInstances, err = p.processCheckKDAInstances(kvp.Value)
				if err != nil {
					return nil, fmt.Errorf("invalid account KDA instances: %w", err)
				}
			case "lastNonce":
				kdaData.LastNonce, err = p.processCheckUint64(kvp.Value)
				if err != nil {
					return nil, fmt.Errorf("invalid account KDA lastNonce: %w", err)
				}
			case "roles":
				kdaData.Roles, err = p.processStringList(kvp.Value)
				if err != nil {
					return nil, fmt.Errorf("invalid account KDA roles: %w", err)
				}
			case "frozen":
				kdaData.Frozen, err = p.processCheckUint64(kvp.Value)
				if err != nil {
					return nil, fmt.Errorf("invalid KDA frozen flag: %w", err)
				}
			default:
				return nil, fmt.Errorf("unknown KDA data field: %s", kvp.Key)
			}
		}
	}

	if firstInstanceLoaded {
		if !p.AllowEsdtLegacyCheckSyntax {
			return nil, fmt.Errorf("wrong KDA check state syntax: instances in root no longer allowed")
		}
		kdaData.Instances = []*scenjsonmodel.CheckKDAInstance{firstInstance}
	}
	kdaData.Instances = append(kdaData.Instances, explicitInstances...)

	return &kdaData, nil
}

func (p *Parser) tryProcessCheckKDAInstanceField(kvp *orderedjson.OJsonKeyValuePair, targetInstance *scenjsonmodel.CheckKDAInstance) (bool, error) {
	var err error
	switch kvp.Key {
	case "nonce":
		targetInstance.Nonce, err = p.processUint64(kvp.Value)
		if err != nil {
			return false, fmt.Errorf("invalid account nonce: %w", err)
		}
	case "balance":
		targetInstance.Balance, err = p.processCheckBigInt(kvp.Value, bigIntUnsignedBytes)
		if err != nil {
			return false, fmt.Errorf("invalid KDA balance: %w", err)
		}
	case "creator":
		targetInstance.Creator, err = p.parseCheckBytes(kvp.Value)
		if err != nil {
			return false, fmt.Errorf("invalid KDA NFT creator address: %w", err)
		}
	case "royalties":
		targetInstance.Royalties, err = p.processCheckUint64(kvp.Value)
		if err != nil {
			return false, fmt.Errorf("invalid KDA NFT royalties: %w", err)
		}
		if targetInstance.Royalties.Value > 10000 {
			return false, errors.New("invalid KDA NFT royalties: value exceeds maximum allowed 10000")
		}
	case "hash":
		targetInstance.Hash, err = p.parseCheckBytes(kvp.Value)
		if err != nil {
			return false, fmt.Errorf("invalid KDA NFT hash: %w", err)
		}
	case "uri":
		targetInstance.Uris, err = p.parseCheckValueList(kvp.Value)
		if err != nil {
			return false, fmt.Errorf("invalid KDA NFT URI: %w", err)
		}
	case "attributes":
		targetInstance.Attributes, err = p.parseCheckBytes(kvp.Value)
		if err != nil {
			return false, fmt.Errorf("invalid KDA NFT attributes: %w", err)
		}
	default:
		return false, nil
	}
	return true, nil
}

func (p *Parser) processCheckKDAInstances(kdaInstancesRaw orderedjson.OJsonObject) ([]*scenjsonmodel.CheckKDAInstance, error) {
	var instancesResult []*scenjsonmodel.CheckKDAInstance
	kdaInstancesList, isList := kdaInstancesRaw.(*orderedjson.OJsonList)
	if !isList {
		return nil, errors.New("kda instances object is not a list")
	}
	for _, instanceItem := range kdaInstancesList.AsList() {
		instanceAsMap, isMap := instanceItem.(*orderedjson.OJsonMap)
		if !isMap {
			return nil, errors.New("JSON map expected as kda instances list item")
		}

		instance := scenjsonmodel.NewCheckKDAInstance()

		for _, kvp := range instanceAsMap.OrderedKV {
			instanceFieldLoaded, err := p.tryProcessCheckKDAInstanceField(kvp, instance)
			if err != nil {
				return nil, fmt.Errorf("invalid account KDA instance field in instances list: %w", err)
			}
			if !instanceFieldLoaded {
				return nil, fmt.Errorf("invalid account KDA instance field in instances list: `%s`", kvp.Key)
			}
		}

		instancesResult = append(instancesResult, instance)

	}

	return instancesResult, nil
}
