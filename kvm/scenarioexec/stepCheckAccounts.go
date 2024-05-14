package scenarioexec

import (
	"bytes"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/dkda"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	worldmock "github.com/klever-io/klever-go/kvm/mock/world"
	scenexpressionreconstructor "github.com/klever-io/klever-go/kvm/scenarioexec/expression/reconstructor"
	"github.com/klever-io/klever-go/kvm/scenarioexec/kdaconvert"
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

// ExecuteCheckStateStep executes a CheckStateStep defined by the current scenario.
func (ae *VMTestExecutor) ExecuteCheckStateStep(step *scenjsonmodel.CheckStateStep) error {
	if len(step.Comment) > 0 {
		log.Trace("CheckStateStep", "comment", step.Comment)
	}

	baseErrMsg := checkStateBaseErrorMsg(step)
	return ae.checkAccounts(baseErrMsg, step.CheckAccounts)
}

func checkStateBaseErrorMsg(step *scenjsonmodel.CheckStateStep) string {
	if len(step.CheckStateIdent) > 0 {
		return fmt.Sprintf("Check state \"%s\":", step.CheckStateIdent)
	}
	return "Check state:"
}

func (ae *VMTestExecutor) checkAccounts(baseErrMsg string, checkAccounts *scenjsonmodel.CheckAccounts) error {
	cacher, ok := ae.World.AccountsCacher.(*worldmock.WorldAccountsCacher)
	if ok { // only custom cacher has the ability to know exact account addresses
		if !checkAccounts.MoreAccountsAllowed {
			for _, worldAcctAddr := range cacher.GetAddressList() {
				postAcctMatch := scenjsonmodel.FindCheckAccount(checkAccounts.Accounts, worldAcctAddr)
				if postAcctMatch == nil && !bytes.Equal(worldAcctAddr, core.SystemAccountAddress) {
					return fmt.Errorf("%s unexpected account address: %s",
						baseErrMsg,
						ae.exprReconstructor.Reconstruct(
							worldAcctAddr,
							scenexpressionreconstructor.AddressHint))
				}
			}
		}
	}

	for _, expectedAcct := range checkAccounts.Accounts {
		matchingAcct, err := ae.World.AccountsCacher.GetExistingUser(expectedAcct.Address.Value)
		if err != nil {
			return fmt.Errorf("%s account %s expected but not found after running test",
				baseErrMsg,
				expectedAcct.Address.Original)
		}

		if !bytes.Equal(matchingAcct.AddressBytes(), expectedAcct.Address.Value) {
			return fmt.Errorf("%s bad account address %s",
				baseErrMsg,
				ae.exprReconstructor.Reconstruct(
					matchingAcct.AddressBytes(),
					scenexpressionreconstructor.AddressHint))
		}

		if !expectedAcct.Nonce.Check(matchingAcct.GetNonce()) {
			return fmt.Errorf("%s bad account nonce. Account: %s. Want: \"%s\". Have: \"%d\"",
				baseErrMsg,
				expectedAcct.Address.Original,
				expectedAcct.Nonce.Original,
				matchingAcct.GetNonce())
		}

		if !expectedAcct.Balance.Check(big.NewInt(matchingAcct.GetBalance(nil, true))) {
			return fmt.Errorf("%s bad account balance. Account: %s. Want: \"%s\". Have: \"%d\"",
				baseErrMsg,
				expectedAcct.Address.Original,
				expectedAcct.Balance.Original,
				matchingAcct.GetBalance(nil, true))
		}

		if !expectedAcct.Username.Check(matchingAcct.GetName()) {
			return fmt.Errorf("%s bad account username. Account: %s. Want: %s. Have: \"%s\"",
				baseErrMsg,
				expectedAcct.Address.Original,
				orderedjson.JSONString(expectedAcct.Username.Original),
				ae.exprReconstructor.Reconstruct(
					matchingAcct.GetName(),
					scenexpressionreconstructor.StrHint))
		}

		if !expectedAcct.Code.Check(ae.World.GetCode(matchingAcct)) {
			return fmt.Errorf("%s bad account code. Account: %s. Want: %s. Have: \"%s\"",
				baseErrMsg,
				expectedAcct.Address.Original,
				orderedjson.JSONString(expectedAcct.Code.Original),
				ae.exprReconstructor.Reconstruct(
					ae.World.GetCode(matchingAcct),
					scenexpressionreconstructor.CodeHint))
		}

		if !expectedAcct.Owner.IsUnspecified() && !bytes.Equal(matchingAcct.GetOwnerAddress(), expectedAcct.Owner.Value) {
			return fmt.Errorf("%s bad account ownscenexpressionreconstructor. Account: %s. Want: %s. Have: \"%s\"",
				baseErrMsg,
				expectedAcct.Address.Original,
				orderedjson.JSONString(expectedAcct.Owner.Original),
				ae.exprReconstructor.Reconstruct(
					matchingAcct.GetOwnerAddress(),
					scenexpressionreconstructor.AddressHint))
		}

		err = ae.checkAccountStorage(baseErrMsg, expectedAcct, matchingAcct)
		if err != nil {
			return err
		}

		err = ae.checkAccountKDA(baseErrMsg, expectedAcct, matchingAcct)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ae *VMTestExecutor) checkAccountStorage(baseErrMsg string, expectedAcct *scenjsonmodel.CheckAccount, matchingAcct state.UserAccountHandler) error {
	if expectedAcct.IgnoreStorage {
		return nil
	}

	cacher, ok := ae.World.AccountsCacher.(*worldmock.WorldAccountsCacher)
	if !ok {
		return fmt.Errorf("cannot check account storage without custom cacher")
	}

	expectedStorage := make(map[string]scenjsonmodel.JSONCheckBytes)
	for _, stkvp := range expectedAcct.CheckStorage {
		expectedStorage[string(stkvp.Key.Value)] = stkvp.CheckValue
	}

	allKeys := make(map[string]bool)
	for k := range expectedStorage {
		allKeys[k] = true
	}
	for _, k := range cacher.GetUsedStorageKeys(expectedAcct.Address.Value) {
		allKeys[string(k)] = true
	}
	storageError := ""
	for k := range allKeys {
		// ignore all reserved keys
		if strings.HasPrefix(k, kapps.ProtectedKleverKeyPrefix) || strings.HasPrefix(k, kapps.KDAPrefix) {
			continue
		}

		want, specified := expectedStorage[k]
		if !specified {
			if expectedAcct.MoreStorageAllowed {
				// if `"+": ""` was written in the test, any unspecified entries are allowed,
				// which is equivalent to treating them all as "*".
				want = scenjsonmodel.JSONCheckBytesStar()
			} else {
				// otherwise, by default, any unexpected storage key leads to a test failure
				want = scenjsonmodel.JSONCheckBytesUnspecified()
			}
		}
		have, err := matchingAcct.RetrieveValue([]byte(k))
		if err != nil {
			return fmt.Errorf("%s failed to retrieve storage key %s: %s",
				baseErrMsg,
				ae.exprReconstructor.Reconstruct([]byte(k), scenexpressionreconstructor.NoHint),
				err)
		}

		if !want.Check(have) {
			storageError += fmt.Sprintf(
				"\n  for key %s: Want: %s. Have: \"%s\"",
				ae.exprReconstructor.Reconstruct([]byte(k), scenexpressionreconstructor.NoHint),
				orderedjson.JSONString(want.Original),
				ae.exprReconstructor.Reconstruct(have, scenexpressionreconstructor.NoHint))
		}
	}
	if len(storageError) > 0 {
		return fmt.Errorf("%s wrong account storage for account \"%s\":%s",
			baseErrMsg,
			expectedAcct.Address.Original, storageError)
	}
	return nil
}

func (ae *VMTestExecutor) checkAccountKDA(baseErrMsg string, expectedAcct *scenjsonmodel.CheckAccount, matchingAcct state.UserAccountHandler) error {
	if expectedAcct.IgnoreKDA {
		return nil
	}

	accountAddress := expectedAcct.Address.Original
	expectedTokens := getExpectedTokens(expectedAcct)

	allTokenNames := make(map[string]bool)
	for tokenName := range expectedTokens {
		allTokenNames[tokenName] = true
	}
	var errs []error
	for tokenName := range allTokenNames {
		expectedToken := expectedTokens[tokenName]

		tokenData, err := ae.World.GetKDAData(expectedToken.TokenIdentifier.Value, nil)
		if err != nil {
			return err
		}

		if expectedToken == nil {
			expectedToken = &scenjsonmodel.CheckKDAData{
				TokenIdentifier: scenjsonmodel.JSONBytesFromString{
					Value:    []byte(tokenName),
					Original: ae.exprReconstructor.Reconstruct([]byte(tokenName), scenexpressionreconstructor.StrHint),
				},
				Instances: []*scenjsonmodel.CheckKDAInstance{},
				LastNonce: scenjsonmodel.JSONCheckUint64{Value: 0, Original: ""},
				Roles:     []string{},
			}
		}

		userInstances := make([]*dkda.KDigitalToken, 0)
		for _, instance := range expectedToken.Instances {
			nonce := []byte(strconv.FormatUint(instance.Nonce.Value, 10))

			kda, err := matchingAcct.GetUserKDA(expectedToken.TokenIdentifier.Value, nonce, true)
			if err != nil {
				userInstances = append(userInstances, &dkda.KDigitalToken{
					Value: 0,
					TokenMetaData: &dkda.MetaData{
						Nonce: instance.Nonce.Value,
					},
				})
			}
			userInstances = append(userInstances, &dkda.KDigitalToken{
				Value: kda.Balance,
				TokenMetaData: &dkda.MetaData{
					Nonce: instance.Nonce.Value,
				},
			})
		}
		userRoles := make([][]byte, 0)
		for _, role := range tokenData.Roles {
			if bytes.Equal(role.Address, matchingAcct.AddressBytes()) {
				list := kdaconvert.KDARoleToRoleList(role)
				userRoles = append(userRoles, list...)
			}
		}

		accountToken := &kdaconvert.MockKDAData{
			TokenIdentifier: []byte(tokenName),
			Instances:       userInstances,
			LastNonce:       0,
			Roles:           userRoles,
		}

		errs = append(errs, ae.checkTokenState(accountAddress, tokenName, expectedToken, accountToken)...)
	}

	errorString := makeErrorString(errs)
	if len(errorString) > 0 {
		return fmt.Errorf("%s mismatch for account \"%s\":%s", baseErrMsg, accountAddress, errorString)
	}

	return nil
}

func getExpectedTokens(expectedAcct *scenjsonmodel.CheckAccount) map[string]*scenjsonmodel.CheckKDAData {
	expectedTokens := make(map[string]*scenjsonmodel.CheckKDAData)
	for _, expectedTokenData := range expectedAcct.CheckKDAData {
		tokenName := expectedTokenData.TokenIdentifier.Value
		expectedTokens[string(tokenName)] = expectedTokenData
	}

	return expectedTokens
}

func (ae *VMTestExecutor) checkTokenState(
	accountAddress string,
	tokenName string,
	expectedToken *scenjsonmodel.CheckKDAData,
	accountToken *kdaconvert.MockKDAData,
) []error {

	var errors []error

	errors = append(errors, ae.checkTokenInstances(accountAddress, tokenName, expectedToken, accountToken)...)

	if !expectedToken.LastNonce.Check(accountToken.LastNonce) {
		errors = append(errors, fmt.Errorf("bad account KDA last nonce. Account: %s. Token: %s. Want: \"%s\". Have: %d",
			accountAddress,
			tokenName,
			expectedToken.LastNonce.Original,
			accountToken.LastNonce))
	}

	errors = append(errors, checkTokenRoles(accountAddress, tokenName, expectedToken, accountToken)...)

	return errors
}

func (ae *VMTestExecutor) checkTokenInstances(
	_ string,
	tokenName string,
	expectedToken *scenjsonmodel.CheckKDAData,
	accountToken *kdaconvert.MockKDAData,
) []error {

	var errors []error

	allNonces := make(map[uint64]bool)
	expectedInstances := make(map[uint64]*scenjsonmodel.CheckKDAInstance)
	accountInstances := make(map[uint64]*dkda.KDigitalToken)
	for _, expectedInstance := range expectedToken.Instances {
		nonce := expectedInstance.Nonce.Value
		allNonces[nonce] = true
		expectedInstances[nonce] = expectedInstance
	}
	for _, accountInstance := range accountToken.Instances {
		nonce := accountInstance.TokenMetaData.Nonce
		allNonces[nonce] = true
		accountInstances[nonce] = accountInstance
	}

	for nonce := range allNonces {
		expectedInstance := expectedInstances[nonce]
		accountInstance := accountInstances[nonce]

		if expectedInstance == nil {
			expectedInstance = &scenjsonmodel.CheckKDAInstance{
				Nonce:   scenjsonmodel.JSONUint64{Value: nonce, Original: ""},
				Balance: scenjsonmodel.JSONCheckBigInt{Value: big.NewInt(0), Original: ""},
			}
		} else if accountInstance == nil {
			accountInstance = &dkda.KDigitalToken{
				Value: 0,
				TokenMetaData: &dkda.MetaData{
					Name:  []byte(tokenName),
					Nonce: nonce,
				},
			}
		}

		if !expectedInstance.Balance.Check(big.NewInt(accountInstance.Value)) {
			errors = append(errors, fmt.Errorf(
				"for token: %s, nonce: %d: Bad balance. Want: \"%s\". Have: \"%d\"",
				tokenName,
				nonce,
				expectedInstance.Balance.Original,
				accountInstance.Value))
		}
		if !expectedInstance.Creator.IsUnspecified() &&
			!expectedInstance.Creator.Check(accountInstance.TokenMetaData.Creator) {
			errors = append(errors, fmt.Errorf(
				"for token: %s, nonce: %d: Bad creator. Want: %s. Have: \"%s\"",
				tokenName,
				nonce,
				objectStringOrDefault(expectedInstance.Creator.Original),
				ae.exprReconstructor.Reconstruct(
					accountInstance.TokenMetaData.Creator,
					scenexpressionreconstructor.AddressHint)))
		}
		if !expectedInstance.Royalties.IsUnspecified() &&
			!expectedInstance.Royalties.Check(uint64(accountInstance.TokenMetaData.Royalties)) {
			errors = append(errors, fmt.Errorf(
				"for token: %s, nonce: %d: Bad royalties. Want: \"%s\". Have: \"%s\"",
				tokenName,
				nonce,
				expectedInstance.Royalties.Original,
				ae.exprReconstructor.ReconstructFromUint64(
					uint64(accountInstance.TokenMetaData.Royalties))))
		}
		if !expectedInstance.Hash.IsUnspecified() &&
			!expectedInstance.Hash.Check(accountInstance.TokenMetaData.Hash) {
			errors = append(errors, fmt.Errorf(
				"for token: %s, nonce: %d: Bad hash. Want: %s. Have: %s",
				tokenName,
				nonce,
				objectStringOrDefault(expectedInstance.Hash.Original),
				ae.exprReconstructor.Reconstruct(
					accountInstance.TokenMetaData.Hash,
					scenexpressionreconstructor.NoHint)))
		}

		if !expectedInstance.Uris.IsUnspecified() &&
			!expectedInstance.Uris.CheckList(accountInstance.TokenMetaData.URIs) {
			// in this case unspecified is interpreted as *
			errors = append(errors, fmt.Errorf(
				"for token: %s, nonce: %d: Bad URI. Want: %s. Have: %s",
				tokenName,
				nonce,
				checkBytesListPretty(expectedInstance.Uris),
				ae.exprReconstructor.ReconstructList(accountInstance.TokenMetaData.URIs, scenexpressionreconstructor.StrHint)))
		}

		if !expectedInstance.Attributes.IsUnspecified() &&
			!expectedInstance.Attributes.Check(accountInstance.TokenMetaData.Attributes) {
			errors = append(errors, fmt.Errorf(
				"for token: %s, nonce: %d: Bad attributes. Want: %s. Have: \"%s\"",
				tokenName,
				nonce,
				objectStringOrDefault(expectedInstance.Attributes.Original),
				ae.exprReconstructor.Reconstruct(
					accountInstance.TokenMetaData.Attributes,
					scenexpressionreconstructor.StrHint)))
		}

	}

	return errors
}

func checkTokenRoles(
	accountAddress string,
	tokenName string,
	expectedToken *scenjsonmodel.CheckKDAData,
	accountToken *kdaconvert.MockKDAData) []error {

	var errors []error

	allRoles := make(map[string]bool)
	expectedRoles := make(map[string]bool)
	accountRoles := make(map[string]bool)

	for _, expectedRole := range expectedToken.Roles {
		allRoles[expectedRole] = true
		expectedRoles[expectedRole] = true
	}
	for _, accountRole := range accountToken.Roles {
		allRoles[string(accountRole)] = true
		accountRoles[string(accountRole)] = true
	}
	for role := range allRoles {
		if !expectedRoles[role] {
			errors = append(errors, fmt.Errorf("unexpected KDA role. Account: %s. Token: %s. Role: %s",
				accountAddress,
				tokenName,
				role))
		}
		if !accountRoles[role] {
			errors = append(errors, fmt.Errorf("missing KDA role. Account: %s. Token: %s. Role: %s",
				accountAddress,
				tokenName,
				role))
		}
	}

	return errors
}

func makeErrorString(errors []error) string {
	errorString := ""
	for _, err := range errors {
		errorString += "\n  " + err.Error()
	}
	return errorString
}

func objectStringOrDefault(obj orderedjson.OJsonObject) string {
	if obj == nil {
		return ""
	}

	return orderedjson.JSONString(obj)
}
