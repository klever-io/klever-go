package scenjsonparse

import (
	"errors"
	"fmt"

	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

func (p *Parser) parseAccountAddress(addrRaw string) (scenjsonmodel.JSONBytesFromString, error) {
	if len(addrRaw) == 0 {
		return scenjsonmodel.JSONBytesFromString{}, errors.New("missing account address")
	}
	addrBytes, err := p.ExprInterpreter.InterpretString(addrRaw)
	if err == nil && len(addrBytes) != 32 {
		return scenjsonmodel.JSONBytesFromString{}, errors.New("account address is not 32 bytes in length")
	}
	return scenjsonmodel.NewJSONBytesFromString(addrBytes, addrRaw), err
}

func (p *Parser) processAccount(acctRaw orderedjson.OJsonObject) (*scenjsonmodel.Account, error) {
	acctMap, isMap := acctRaw.(*orderedjson.OJsonMap)
	if !isMap {
		return nil, errors.New("unmarshalled account object is not a map")
	}

	acct := scenjsonmodel.Account{
		IsSmartContract: false,
		Comment:         "",
		Nonce:           scenjsonmodel.JSONUint64Zero(),
		Balance:         scenjsonmodel.JSONBigIntZero(),
		Username:        scenjsonmodel.JSONBytesEmpty(),
		Storage:         nil,
		Code:            scenjsonmodel.JSONBytesEmpty(),
		CodeMetadata:    scenjsonmodel.JSONBytesEmpty(),
		Owner:           scenjsonmodel.JSONBytesEmpty(),
		KDAData:         nil,
		Update:          false,
	}

	var err error

	for _, kvp := range acctMap.OrderedKV {
		switch kvp.Key {
		case "comment":
			acct.Comment, err = p.parseString(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid account comment: %w", err)
			}
		case "update":
			acct.Update, err = p.parseBool(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid update flag bool: %w", err)
			}
		case "nonce":
			acct.Nonce, err = p.processUint64(kvp.Value)
			if err != nil {
				return nil, errors.New("invalid account nonce")
			}
		case "balance":
			acct.Balance, err = p.processBigInt(kvp.Value, bigIntUnsignedBytes)
			if err != nil {
				return nil, errors.New("invalid account balance")
			}
		case "kda":
			kdaMap, kdaOk := kvp.Value.(*orderedjson.OJsonMap)
			if !kdaOk {
				return nil, errors.New("invalid KDA map")
			}
			for _, kdaKvp := range kdaMap.OrderedKV {
				tokenNameStr, err := p.ExprInterpreter.InterpretString(kdaKvp.Key)
				if err != nil {
					return nil, fmt.Errorf("invalid kda token identifer: %w", err)
				}
				tokenName := scenjsonmodel.NewJSONBytesFromString(tokenNameStr, kdaKvp.Key)
				kdaItem, err := p.processKDAData(tokenName, kdaKvp.Value)
				if err != nil {
					return nil, fmt.Errorf("invalid kda value: %w", err)
				}
				acct.KDAData = append(acct.KDAData, kdaItem)
			}
		case "username":
			acct.Username, err = p.processStringAsByteArray(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid account username: %w", err)
			}
		case "storage":
			storageMap, storageOk := kvp.Value.(*orderedjson.OJsonMap)
			if !storageOk {
				return nil, errors.New("invalid account storage")
			}
			for _, storageKvp := range storageMap.OrderedKV {
				byteKey, err := p.ExprInterpreter.InterpretString(storageKvp.Key)
				if err != nil {
					return nil, fmt.Errorf("invalid account storage key: %w", err)
				}
				byteVal, err := p.processSubTreeAsByteArray(storageKvp.Value)
				if err != nil {
					return nil, fmt.Errorf("invalid account storage value: %w", err)
				}
				stElem := scenjsonmodel.StorageKeyValuePair{
					Key:   scenjsonmodel.NewJSONBytesFromString(byteKey, storageKvp.Key),
					Value: byteVal,
				}
				acct.Storage = append(acct.Storage, &stElem)
			}
		case "code":
			acct.Code, err = p.processStringAsByteArray(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid account code: %w", err)
			}
		case "codeMetadata":
			acct.CodeMetadata, err = p.processStringAsByteArray(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid account codeMetadata: %w", err)
			}
		case "owner":
			acct.Owner, err = p.processStringAsByteArray(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid account owner: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown account field: %s", kvp.Key)
		}
	}

	return &acct, nil
}

func (p *Parser) processAccountMap(acctMapRaw orderedjson.OJsonObject) ([]*scenjsonmodel.Account, error) {
	var accounts []*scenjsonmodel.Account
	preMap, isPreMap := acctMapRaw.(*orderedjson.OJsonMap)
	if !isPreMap {
		return nil, errors.New("unmarshalled account map object is not a map")
	}
	for _, acctKVP := range preMap.OrderedKV {
		acct, acctErr := p.processAccount(acctKVP.Value)
		if acctErr != nil {
			return nil, acctErr
		}
		acctAddr, hexErr := p.parseAccountAddress(acctKVP.Key)
		if hexErr != nil {
			return nil, hexErr
		}
		acct.Address = acctAddr
		accounts = append(accounts, acct)

	}
	return accounts, nil
}
