package state

import (
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/tools/check"
)

type baseAccount struct {
	address         []byte
	dataTrieTracker DataTrieTracker
	code            []byte
	hasNewCode      bool
}

// AddressBytes returns the address associated with the account as byte slice
func (ba *baseAccount) AddressBytes() []byte {
	return ba.address
}

// IsInterfaceNil returns true if there is no value under the interface
func (ba *baseAccount) IsInterfaceNil() bool {
	return ba == nil
}

// DataTrie returns the trie that holds the current account's data
func (ba *baseAccount) DataTrie() data.Trie {
	return ba.dataTrieTracker.DataTrie()
}

// SetDataTrie sets the trie that holds the current account's data
func (ba *baseAccount) SetDataTrie(trie data.Trie) {
	ba.dataTrieTracker.SetDataTrie(trie)
}

// DataTrieTracker returns the trie wrapper used in managing the SC data
func (ba *baseAccount) DataTrieTracker() DataTrieTracker {
	return ba.dataTrieTracker
}

// SetCode sets the actual code that needs to be run in the VM
func (ba *baseAccount) SetCode(code []byte) {
	ba.hasNewCode = true
	ba.code = code
}

func (ba *baseAccount) HasNewCode() bool {
	return ba.hasNewCode
}

// SaveKeyValue adds the given key and value to the underlying trackable data trie
func (ba *baseAccount) SaveKeyValue(key []byte, value []byte) error {
	if check.IfNil(ba.dataTrieTracker) {
		return ErrNilTrackableDataTrie
	}

	return ba.dataTrieTracker.SaveKeyValue(key, value)
}

// RetrieveValue fetches the value from a particular key searching the account data store in the data trie tracker
func (ba *baseAccount) RetrieveValue(key []byte) ([]byte, error) {
	if check.IfNil(ba.dataTrieTracker) {
		return nil, ErrNilTrackableDataTrie
	}

	return ba.dataTrieTracker.RetrieveValue(key)
}

// AccountDataHandler returns the account data handler
func (ba *baseAccount) AccountDataHandler() AccountDataHandler {
	return ba.dataTrieTracker
}
