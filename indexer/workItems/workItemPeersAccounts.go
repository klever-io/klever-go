package workItems

import "github.com/klever-io/klever-go/core/kapp"

type itemPeersAccounts struct {
	indexer       saveAccountsIndexer
	peersAccounts []kapp.ValidatorAccountInfoHandler
}

// NewItemPeersAccounts will create a new instance of itemPeersAccounts
func NewItemPeersAccounts(
	indexer saveAccountsIndexer,
	peersAccounts []kapp.ValidatorAccountInfoHandler,
) WorkItemHandler {
	return &itemPeersAccounts{
		indexer:       indexer,
		peersAccounts: peersAccounts,
	}
}

// Save will save information about a peer account
func (wpa *itemPeersAccounts) Save() error {
	err := wpa.indexer.SavePeersAccounts(wpa.peersAccounts)
	if err != nil {
		log.Warn("itemPeerAccount.Save",
			"could not index peer account",
			"error", err.Error())
		return err
	}

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (wpa *itemPeersAccounts) IsInterfaceNil() bool {
	return wpa == nil
}
