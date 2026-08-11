package scenarioexec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/klever-io/klever-go/kvm/scenarioexec/kdaconvert"

	"github.com/klever-io/klever-go/kapps"

	"github.com/klever-io/klever-go/data/state"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	scenexpressionreconstructor "github.com/klever-io/klever-go/kvm/scenarioexec/expression/reconstructor"
	scenjsonwrite "github.com/klever-io/klever-go/kvm/scenarioexec/json/write"
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

const includeProtectedStorage = false

func (ae *VMTestExecutor) convertMockAccountToScenarioFormat(account state.UserAccountHandler) (*scenjsonmodel.Account, error) {
	accountStorage, err := ae.World.ExtractAccountStorage(account)
	if err != nil {
		return nil, err
	}

	var storageKeys []string
	var kdaKeys []string
	for storageKey := range accountStorage {
		if strings.HasPrefix(storageKey, kapps.KDAPrefix) {
			kdaKeys = append(kdaKeys, storageKey)
		} else {
			storageKeys = append(storageKeys, storageKey)
		}
	}

	// convert storage keys to scenario format
	sort.Strings(storageKeys)
	var storageKvps []*scenjsonmodel.StorageKeyValuePair
	for _, storageKey := range storageKeys {
		storageValue := accountStorage[storageKey]
		includeKey := includeProtectedStorage || !strings.HasPrefix(storageKey, kapps.ProtectedKleverKeyPrefix)
		if includeKey && len(storageValue) > 0 {
			storageKvps = append(storageKvps, &scenjsonmodel.StorageKeyValuePair{
				Key: scenjsonmodel.JSONBytesFromString{
					Value:    []byte(storageKey),
					Original: ae.exprReconstructor.Reconstruct([]byte(storageKey), scenexpressionreconstructor.NoHint),
				},
				Value: scenjsonmodel.JSONBytesFromTree{
					Value:    storageValue,
					Original: &orderedjson.OJsonString{Value: ae.exprReconstructor.Reconstruct(storageValue, scenexpressionreconstructor.NoHint)},
				},
			})
		}
	}

	// convert kda keys to scenario format
	sort.Strings(kdaKeys)
	var scenKDA []*scenjsonmodel.KDAData
	for _, tokenName := range kdaKeys {
		// 0 index is "KDA" (Protected Key Identifier)
		// 1 index is the token name
		// 2 (if present) is the token nonce
		split := strings.Split(tokenName, kapps.Sp)
		tokenIdentifier := []byte(split[1])
		var nonce []byte
		var uintNonce uint64
		if len(split) > 2 {
			nonce = []byte(split[2])
			uintNonce = binary.BigEndian.Uint64(nonce)
		}
		userInstances := make([]*scenjsonmodel.KDAInstance, 0)

		// load kda data
		kda, err := account.GetUserKDA(tokenIdentifier, nonce, true)
		if err != nil {
			// if for some reason the KDA Key was in storage but the KDA was not found, create a KDAInstance with 0 balance
			userInstances = append(userInstances, &scenjsonmodel.KDAInstance{
				Nonce: scenjsonmodel.JSONUint64{
					Value:    uintNonce,
					Original: ae.exprReconstructor.ReconstructFromUint64(uintNonce),
				},
				Balance: scenjsonmodel.JSONBigInt{
					Value:    big.NewInt(0),
					Original: ae.exprReconstructor.ReconstructFromBigInt(big.NewInt(0)),
				},
			})
		}

		// load token data
		kdaData, err := ae.World.GetKDAData(tokenIdentifier, nil)
		if err != nil {
			return nil, err
		}

		// load token roles
		userRoles := make([]string, 0)
		for _, role := range kdaData.Roles {
			if bytes.Equal(role.Address, account.AddressBytes()) {
				roleListBytes := kdaconvert.KDARoleToRoleList(role)
				for _, roleBytes := range roleListBytes {
					userRoles = append(userRoles, string(roleBytes))
				}
			}
		}

		// perform conversion of KDA to scenario format
		userInstances = append(userInstances, &scenjsonmodel.KDAInstance{
			Nonce: scenjsonmodel.JSONUint64{
				Value:    uintNonce,
				Original: ae.exprReconstructor.ReconstructFromUint64(uintNonce),
			},
			Balance: scenjsonmodel.JSONBigInt{
				Value:    big.NewInt(kda.Balance),
				Original: ae.exprReconstructor.ReconstructFromBigInt(big.NewInt(kda.Balance)),
			},
		})
		scenKDAData := &scenjsonmodel.KDAData{
			TokenIdentifier: scenjsonmodel.JSONBytesFromString{
				Value:    tokenIdentifier,
				Original: ae.exprReconstructor.Reconstruct(tokenIdentifier, scenexpressionreconstructor.NoHint),
			},
			Instances: userInstances,
			Roles:     userRoles,
		}
		scenKDA = append(scenKDA, scenKDAData)
	}

	return &scenjsonmodel.Account{
		Address: scenjsonmodel.JSONBytesFromString{
			Value:    account.AddressBytes(),
			Original: ae.exprReconstructor.Reconstruct(account.AddressBytes(), scenexpressionreconstructor.AddressHint),
		},
		Nonce: scenjsonmodel.JSONUint64{
			Value:    account.GetNonce(),
			Original: ae.exprReconstructor.ReconstructFromUint64(account.GetNonce()),
		},
		Balance: scenjsonmodel.JSONBigInt{
			Value:    big.NewInt(account.GetBalance(nil, true)),
			Original: ae.exprReconstructor.ReconstructFromBigInt(big.NewInt(account.GetBalance(nil, true))),
		},
		Storage: storageKvps,
		KDAData: scenKDA,
		Owner: scenjsonmodel.JSONBytesFromString{
			Value:    account.GetOwnerAddress(),
			Original: ae.exprReconstructor.Reconstruct(account.GetOwnerAddress(), scenexpressionreconstructor.AddressHint),
		},
		Code: scenjsonmodel.JSONBytesFromString{
			Value:    ae.World.GetCode(account),
			Original: ae.exprReconstructor.Reconstruct(ae.World.GetCode(account), scenexpressionreconstructor.NoHint),
		},
	}, nil
}

// DumpWorld prints the state of the MockWorld to stdout.
func (ae *VMTestExecutor) DumpWorld() error {

	worldCacher, ok := ae.World.AccountsCacher.(*worldmock.WorldAccountsCacher)
	if !ok {
		return errors.New("world accounts cacher is not a mock")
	}

	var scenAccounts []*scenjsonmodel.Account
	for _, address := range worldCacher.GetAddressList() {
		account, err := worldCacher.GetExistingUser(address)
		if err != nil {
			continue // deleted account, skip
		}
		scenAccount, err := ae.convertMockAccountToScenarioFormat(account)
		if err != nil {
			return err
		}
		scenAccounts = append(scenAccounts, scenAccount)
	}

	ojAccount := scenjsonwrite.AccountsToOJ(scenAccounts)
	s := orderedjson.JSONString(ojAccount)
	fmt.Println(s)

	return nil
}
