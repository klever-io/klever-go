package scenjsonwrite

import (
	scenjsonmodel "github.com/klever-io/klever-go/kvm/scenarioexec/model"
	"github.com/klever-io/klever-go/kvm/scenarioexec/orderedjson"
)

// AccountsToOJ converts a scenarios-format account to an ordered JSON representation.
func AccountsToOJ(accounts []*scenjsonmodel.Account) orderedjson.OJsonObject {
	acctsOJ := orderedjson.NewMap()
	for _, account := range accounts {
		acctOJ := orderedjson.NewMap()
		if len(account.Comment) > 0 {
			acctOJ.Put("comment", stringToOJ(account.Comment))
		}
		if account.Update {
			acctOJ.Put("update", boolToOJ(account.Update))
		}
		if len(account.Nonce.Original) > 0 {
			acctOJ.Put("nonce", uint64ToOJ(account.Nonce))
		}
		if len(account.Balance.Original) > 0 {
			acctOJ.Put("balance", bigIntToOJ(account.Balance))
		}
		if len(account.KDAData) > 0 {
			acctOJ.Put("kda", kdaDataToOJ(account.KDAData))
		}
		storageOJ := orderedjson.NewMap()
		for _, st := range account.Storage {
			storageOJ.Put(bytesFromStringToString(st.Key), bytesFromTreeToOJ(st.Value))
		}
		if len(account.Username.Value) > 0 {
			acctOJ.Put("username", bytesFromStringToOJ(account.Username))
		}
		if storageOJ.Size() > 0 {
			acctOJ.Put("storage", storageOJ)
		}
		if len(account.Code.Original) > 0 {
			acctOJ.Put("code", bytesFromStringToOJ(account.Code))
		}
		if len(account.CodeMetadata.Original) > 0 {
			acctOJ.Put("codeMetadata", bytesFromStringToOJ(account.CodeMetadata))
		}
		if len(account.Owner.Value) > 0 {
			acctOJ.Put("owner", bytesFromStringToOJ(account.Owner))
		}

		acctsOJ.Put(bytesFromStringToString(account.Address), acctOJ)
	}

	return acctsOJ
}

func checkAccountsToOJ(checkAccounts *scenjsonmodel.CheckAccounts) orderedjson.OJsonObject {
	acctsOJ := orderedjson.NewMap()
	for _, checkAccount := range checkAccounts.Accounts {
		acctOJ := orderedjson.NewMap()
		if len(checkAccount.Comment) > 0 {
			acctOJ.Put("comment", stringToOJ(checkAccount.Comment))
		}
		if !checkAccount.Nonce.IsUnspecified() {
			acctOJ.Put("nonce", checkUint64ToOJ(checkAccount.Nonce))
		}
		if !checkAccount.Balance.IsUnspecified() {
			acctOJ.Put("balance", checkBigIntToOJ(checkAccount.Balance))
		}
		if checkAccount.IgnoreKDA {
			acctOJ.Put("kda", stringToOJ("*"))
		} else {
			if len(checkAccount.CheckKDAData) > 0 {
				acctOJ.Put("kda", checkKDADataToOJ(
					checkAccount.CheckKDAData, checkAccount.MoreKDATokensAllowed))
			}
		}
		if !checkAccount.Username.IsUnspecified() {
			acctOJ.Put("username", checkBytesToOJ(checkAccount.Username))
		}
		if checkAccount.ExplicitStorage {
			if checkAccount.IgnoreStorage {
				acctOJ.Put("storage", stringToOJ("*"))
			} else {
				storageOJ := orderedjson.NewMap()
				for _, st := range checkAccount.CheckStorage {
					storageOJ.Put(bytesFromStringToString(st.Key), checkBytesToOJ(st.CheckValue))
				}
				if checkAccount.MoreStorageAllowed {
					storageOJ.Put("+", stringToOJ(""))
				}
				acctOJ.Put("storage", storageOJ)
			}
		}
		if !checkAccount.Code.IsUnspecified() {
			acctOJ.Put("code", checkBytesToOJ(checkAccount.Code))
		}
		if !checkAccount.CodeMetadata.IsUnspecified() {
			acctOJ.Put("codeMetadata", checkBytesToOJ(checkAccount.CodeMetadata))
		}
		if !checkAccount.Owner.IsUnspecified() {
			acctOJ.Put("owner", checkBytesToOJ(checkAccount.Owner))
		}

		acctsOJ.Put(bytesFromStringToString(checkAccount.Address), acctOJ)
	}

	if checkAccounts.MoreAccountsAllowed {
		acctsOJ.Put("+", stringToOJ(""))
	}

	return acctsOJ
}
