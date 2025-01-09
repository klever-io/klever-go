package worldmock

import (
	"context"
	"errors"

	"github.com/klever-io/klever-go/data/state"
)

// ErrTrieHandlingNotImplemented indicates that no trie-related operations are
// currently implemented.
var ErrTrieHandlingNotImplemented = errors.New("trie handling not implemented")

// MockAccountsAdapter is an implementation of AccountsAdapter based on
// MockWorld and the accounts within it.
type MockAccountsAdapter struct {
	World     *MockWorld
	Snapshots []AccountMap
}

// GetExistingAccount -
func (m *MockAccountsAdapter) GetExistingAccount(address []byte) (state.AccountHandler, error) {
	return m.World.AccountsCacher.GetExistingUser(address)
}

// LoadAccount -
func (m *MockAccountsAdapter) LoadAccount(address []byte) (state.AccountHandler, error) {
	return m.World.AccountsCacher.LoadUser(address)
}

// SaveAccounts -
func (m *MockAccountsAdapter) SaveAccounts(accounts ...state.AccountHandler) error {
	for _, account := range accounts {
		err := m.World.AccountsCacher.SaveUser(account)
		if err != nil {
			return err
		}
	}
	return nil
}

// SaveAccount -
func (m *MockAccountsAdapter) SaveAccount(account state.AccountHandler) error {
	return m.World.AccountsCacher.SaveUser(account)
}

// RemoveAccountCode -
func (m *MockAccountsAdapter) RemoveAccountCode(address []byte) error {
	return nil
}

// RemoveAccount -
func (m *MockAccountsAdapter) RemoveAccount(address []byte) error {
	return nil
}

// Commit -
func (m *MockAccountsAdapter) Commit() ([]byte, error) {
	m.Snapshots = make([]AccountMap, 0)
	err := m.World.AccountsCacher.SaveAll()
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// JournalLen -
func (m *MockAccountsAdapter) JournalLen() int {
	return len(m.Snapshots) - 1
}

// RevertToSnapshot -
func (m *MockAccountsAdapter) RevertToSnapshot(snapshotIndex int) error {
	m.World.AccountsCacher.ResetAll(true)
	return nil
}

// GetNumCheckpoints -
func (m *MockAccountsAdapter) GetNumCheckpoints() uint32 {
	return uint32(len(m.Snapshots)) // #nosec G115
}

// GetCode -
func (m *MockAccountsAdapter) GetCode(codeHash []byte) []byte {
	return m.World.AccountsCacher.GetCode(codeHash)
}

// RootHash -
func (m *MockAccountsAdapter) RootHash() ([]byte, error) {
	return nil, ErrTrieHandlingNotImplemented
}

// RecreateTrie -
func (m *MockAccountsAdapter) RecreateTrie(_ []byte) error {
	return ErrTrieHandlingNotImplemented
}

// SnapshotState -
func (m *MockAccountsAdapter) SnapshotState(_ []byte, _ context.Context) {
	// snapshot := m.World.AcctMap.Clone()
	// m.Snapshots = append(m.Snapshots, snapshot)
}

// SetStateCheckpoint -
func (m *MockAccountsAdapter) SetStateCheckpoint(_ []byte, _ context.Context) {
}

// IsPruningEnabled -
func (m *MockAccountsAdapter) IsPruningEnabled() bool {
	return false
}

// IsInterfaceNil -
func (m *MockAccountsAdapter) IsInterfaceNil() bool {
	return m == nil
}
