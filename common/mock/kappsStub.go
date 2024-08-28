package mock

import (
	"context"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
)

type KappsDBMock struct {
}

func (k KappsDBMock) GetExistingAccount(address []byte) (state.AccountHandler, error) {
	return nil, nil
}

func (k KappsDBMock) LoadAccount(address []byte) (state.AccountHandler, error) {
	return nil, nil
}

func (k KappsDBMock) SaveAccounts(accounts ...state.AccountHandler) error {
	return nil
}

func (k KappsDBMock) SaveAccount(account state.AccountHandler) error {
	return nil
}

func (k KappsDBMock) RemoveAccount(address []byte) error {
	return nil
}

func (k KappsDBMock) Commit() ([]byte, error) {
	return nil, nil
}

func (k KappsDBMock) GetCode(codeHash []byte) []byte {
	return make([]byte, 0)
}

func (k KappsDBMock) JournalLen() int {
	return 0
}

func (k KappsDBMock) RevertToSnapshot(snapshot int) error {
	return nil
}

func (k KappsDBMock) GetNumCheckpoints() uint32 {
	return 0
}

func (k KappsDBMock) RootHash() ([]byte, error) {
	return nil, nil
}

func (k KappsDBMock) RecreateTrie(rootHash []byte) error {
	return nil
}

func (k KappsDBMock) PruneTrie(rootHash []byte, identifier data.TriePruningIdentifier) {
}

func (k KappsDBMock) CancelPrune(rootHash []byte, identifier data.TriePruningIdentifier) {
}

func (k KappsDBMock) SnapshotState(rootHash []byte, ctx context.Context) {
}

func (k KappsDBMock) SetStateCheckpoint(rootHash []byte, ctx context.Context) {
}

func (k KappsDBMock) IsPruningEnabled() bool {
	return false
}

func (k KappsDBMock) GetAllLeaves(rootHash []byte, ctx context.Context) (chan data.KeyValueHolder, error) {
	return nil, nil
}

func (k KappsDBMock) RecreateAllTries(rootHash []byte, ctx context.Context) (map[string]data.Trie, error) {
	return nil, nil
}

func (k KappsDBMock) IsInterfaceNil() bool {
	return false
}

type KappsControllerMock struct {
	kappContext kapp.KappContext
}

func (k *KappsControllerMock) SetCurrentKAppContext(kappContext kapp.KappContext) {
	k.kappContext = kappContext
}
func (k *KappsControllerMock) GetCurrentKAppContext() kapp.KappContext {
	return k.kappContext
}

func (k *KappsControllerMock) InitKApps(_ state.AccountsCacher) error {
	return nil
}

func (k *KappsControllerMock) GetValidatorsKApp() kapp.ValidatorsKapp {
	return nil
}

func (k *KappsControllerMock) GetKDAFeesPoolKApp() kapp.KDAFeesPoolKapp {
	return nil
}

func (k *KappsControllerMock) GetSystemAccountKApp() kapp.SystemAccountKapp {
	return nil
}

// IsInterfaceNil -
func (k *KappsControllerMock) GetAccountsKApp() kapp.AccountsKapp {
	return nil
}

// IsInterfaceNil -
func (k *KappsControllerMock) GetKDAKApp() kapp.KDAKapp {
	return nil
}

// IsInterfaceNil -
func (k *KappsControllerMock) GetITOKApp() kapp.ITOKapp {
	return nil
}

// IsInterfaceNil -
func (k *KappsControllerMock) GetMarketKApp() kapp.MarketKapp {
	return nil
}

// IsInterfaceNil -
func (k *KappsControllerMock) GetProposalKApp() kapp.ProposalKapp {
	return nil
}

// GetProposalController
func (k *KappsControllerMock) GetProposalController() kapps.ActiveProposalController {
	return nil
}

// SetProposalController
func (k *KappsControllerMock) SetProposalController(_ kapps.ActiveProposalController) error {
	return nil
}

func (k *KappsControllerMock) GetForkController() core.ForkController {
	return nil
}

func (k *KappsControllerMock) IsInterfaceNil() bool {
	return false
}
