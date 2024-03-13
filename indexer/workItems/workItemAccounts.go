package workItems

import (
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/indexer/data"
)

type itemAccounts struct {
	indexer        saveAccountsIndexer
	blockTimestamp int64
	accounts       []state.UserAccountHandler
}

// NewItemAccounts will create a new instance of itemAccounts
func NewItemAccounts(
	indexer saveAccountsIndexer,
	blockTimestamp int64,
	accounts []state.UserAccountHandler,
) WorkItemHandler {
	return &itemAccounts{
		indexer:        indexer,
		accounts:       accounts,
		blockTimestamp: blockTimestamp,
	}
}

// Save will save information about an account
func (wiv *itemAccounts) Save() error {
	accounts := make([]*data.Account, len(wiv.accounts))
	for idx, account := range wiv.accounts {
		accounts[idx] = &data.Account{
			UserAccount: account,
			IsSender:    false,
		}
	}

	err := wiv.indexer.SaveAccounts(wiv.blockTimestamp, accounts)
	if err != nil {
		log.Warn("itemAccounts.Save",
			"could not index account",
			"error", err.Error())
		return err
	}

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (wiv *itemAccounts) IsInterfaceNil() bool {
	return wiv == nil
}
