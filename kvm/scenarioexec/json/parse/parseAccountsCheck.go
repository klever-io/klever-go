package scenjsonparse

import (
	"errors"
	"fmt"

	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

func (p *Parser) processCheckAccount(acctRaw orderedjson.OJsonObject) (*scenjsonmodel.CheckAccount, error) {
	acctMap, isMap := acctRaw.(*orderedjson.OJsonMap)
	if !isMap {
		return nil, errors.New("unmarshalled account object is not a map")
	}

	acct := scenjsonmodel.CheckAccount{
		Comment:              "",
		Nonce:                scenjsonmodel.JSONCheckUint64Unspecified(),
		Balance:              scenjsonmodel.JSONCheckBigIntUnspecified(),
		Username:             scenjsonmodel.JSONCheckBytesUnspecified(),
		ExplicitStorage:      false,
		IgnoreStorage:        true,
		MoreStorageAllowed:   false,
		CheckStorage:         nil,
		Code:                 scenjsonmodel.JSONCheckBytesUnspecified(),
		CodeMetadata:         scenjsonmodel.JSONCheckBytesUnspecified(),
		Owner:                scenjsonmodel.JSONCheckBytesUnspecified(),
		AsyncCallData:        scenjsonmodel.JSONCheckBytesUnspecified(),
		IgnoreKDA:            false,
		MoreKDATokensAllowed: false,
		CheckKDAData:         nil,
		DeveloperReward:      scenjsonmodel.JSONCheckBigIntUnspecified(),
	}
	var err error

	for _, kvp := range acctMap.OrderedKV {
		switch kvp.Key {
		case "comment":
			acct.Comment, err = p.parseString(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid check account comment: %w", err)
			}
		case "nonce":
			acct.Nonce, err = p.processCheckUint64(kvp.Value)
			if err != nil {
				return nil, errors.New("invalid account nonce")
			}
		case "balance":
			acct.Balance, err = p.processCheckBigInt(kvp.Value, bigIntUnsignedBytes)
			if err != nil {
				return nil, errors.New("invalid account balance")
			}
		case "kda":
			acct.IgnoreKDA = IsStar(kvp.Value)
			if !acct.IgnoreKDA {
				kdaMap, kdaOk := kvp.Value.(*orderedjson.OJsonMap)
				if !kdaOk {
					return nil, errors.New("invalid KDA map")
				}
				for _, kdaKvp := range kdaMap.OrderedKV {
					if kdaKvp.Key == "+" {
						acct.MoreKDATokensAllowed = true
					} else {
						tokenNameStr, err := p.ExprInterpreter.InterpretString(kdaKvp.Key)
						if err != nil {
							return nil, fmt.Errorf("invalid kda token identifer: %w", err)
						}
						tokenName := scenjsonmodel.NewJSONBytesFromString(tokenNameStr, kdaKvp.Key)
						kdaItem, err := p.processCheckKDAData(tokenName, kdaKvp.Value)
						if err != nil {
							return nil, fmt.Errorf("invalid kda value: %w", err)
						}
						acct.CheckKDAData = append(acct.CheckKDAData, kdaItem)
					}
				}
			}
		case "username":
			acct.Username, err = p.parseCheckBytes(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid account username: %w", err)
			}
		case "storage":
			acct.ExplicitStorage = true
			acct.IgnoreStorage = IsStar(kvp.Value)
			if !acct.IgnoreStorage {
				storageMap, storageOk := kvp.Value.(*orderedjson.OJsonMap)
				if !storageOk {
					return nil, errors.New("invalid account storage")
				}
				for _, storageKvp := range storageMap.OrderedKV {
					if storageKvp.Key == "+" {
						acct.MoreStorageAllowed = true
					} else {
						byteKey, err := p.ExprInterpreter.InterpretString(storageKvp.Key)
						if err != nil {
							return nil, fmt.Errorf("invalid account storage key: %w", err)
						}
						byteVal, err := p.parseCheckBytes(storageKvp.Value)
						if err != nil {
							return nil, fmt.Errorf("invalid account storage value: %w", err)
						}
						stElem := scenjsonmodel.CheckStorageKeyValuePair{
							Key:        scenjsonmodel.NewJSONBytesFromString(byteKey, storageKvp.Key),
							CheckValue: byteVal,
						}
						acct.CheckStorage = append(acct.CheckStorage, &stElem)
					}
				}
			}
		case "code":
			acct.Code, err = p.parseCheckBytes(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid account code: %w", err)
			}
		case "codeMetadata":
			acct.CodeMetadata, err = p.parseCheckBytes(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid account codeMetadata: %w", err)
			}
		case "owner":
			acct.Owner, err = p.parseCheckBytes(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid account owner: %w", err)
			}
		case "asyncCallData":
			acct.AsyncCallData, err = p.parseCheckBytes(kvp.Value)
			if err != nil {
				return nil, fmt.Errorf("invalid asyncCallData: %w", err)
			}
		case "developerRewards":
			acct.DeveloperReward, err = p.processCheckBigInt(kvp.Value, bigIntUnsignedBytes)
			if err != nil {
				return nil, fmt.Errorf("invalid developerRewards: %w", err)
			}

		default:
			return nil, fmt.Errorf("unknown account field: %s", kvp.Key)
		}
	}

	return &acct, nil
}

func (p *Parser) processCheckAccountMap(acctMapRaw orderedjson.OJsonObject) (*scenjsonmodel.CheckAccounts, error) {
	var checkAccounts = &scenjsonmodel.CheckAccounts{
		Accounts:            nil,
		MoreAccountsAllowed: false,
	}

	preMap, isPreMap := acctMapRaw.(*orderedjson.OJsonMap)
	if !isPreMap {
		return nil, errors.New("unmarshalled check account map object is not a map")
	}
	for _, acctKVP := range preMap.OrderedKV {
		if acctKVP.Key == "+" {
			checkAccounts.MoreAccountsAllowed = true
		} else {
			acct, acctErr := p.processCheckAccount(acctKVP.Value)
			if acctErr != nil {
				return nil, acctErr
			}
			acctAddr, hexErr := p.parseAccountAddress(acctKVP.Key)
			if hexErr != nil {
				return nil, hexErr
			}
			acct.Address = acctAddr
			checkAccounts.Accounts = append(checkAccounts.Accounts, acct)
		}
	}
	return checkAccounts, nil
}
