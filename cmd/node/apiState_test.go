package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/data/state"
	stateFactory "github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/factory"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/node"
	"github.com/stretchr/testify/require"
)

// The REST state reads the last committed state only: a block in progress has saved new values,
// and every REST read, controller getter included, still answers the committed ones. Nothing
// reaches the processing cacher, and building it writes nothing to the live accounts.
func TestCreateAPIState_ReadsOnlyTheLastCommittedState(t *testing.T) {
	marshalizer := &mock.MarshalizerMock{}
	hasher := &mock.HasherMock{}
	newAccountsDB := func(accountFactory state.AccountFactory) *state.AccountsDB {
		storageManager, err := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
		require.NoError(t, err)
		tr, err := trie.NewTrie(storageManager, marshalizer, hasher, 5)
		require.NoError(t, err)
		adb, err := state.NewAccountsDB(tr, hasher, marshalizer, accountFactory, core.Normal)
		require.NoError(t, err)
		return adb
	}
	accounts := newAccountsDB(stateFactory.NewAccountCreator())
	kappAccounts := newAccountsDB(stateFactory.NewKAppAccountCreator())
	peers := newAccountsDB(stateFactory.NewPeerAccountCreator())

	address := bytes.Repeat([]byte{1}, 32)
	pendingRewardsKey := append([]byte("PREW"+kapps.Sp), address...)
	setPendingRewards := func(amount uint64) {
		acc, err := kappAccounts.LoadAccount(kapps.ValidatorsKAppAddress)
		require.NoError(t, err)
		value := make([]byte, 8)
		binary.BigEndian.PutUint64(value, amount)
		require.NoError(t, acc.(state.KAppAccountHandler).SetStorage(pendingRewardsKey, value))
		require.NoError(t, kappAccounts.SaveAccount(acc))
	}
	setMaxSupply := func(maxSupply int64) {
		acc, err := kappAccounts.LoadAccount(kapps.KDAKAppAddress)
		require.NoError(t, err)
		value, err := marshalizer.Marshal(&kapps.KDAData{ID: []byte("KFI"), MaxSupply: maxSupply})
		require.NoError(t, err)
		require.NoError(t, acc.(state.KAppAccountHandler).DataTrieTracker().SaveKeyValue(kdautils.ToKDAKey([]byte("KFI"), nil), value))
		require.NoError(t, kappAccounts.SaveAccount(acc))
	}
	addBalance := func(amount int64) {
		acc, err := accounts.LoadAccount(address)
		require.NoError(t, err)
		require.NoError(t, acc.(state.UserAccountHandler).AddToBalance(amount, nil, false))
		require.NoError(t, accounts.SaveAccount(acc))
	}

	setPendingRewards(5)
	setMaxSupply(100)
	addBalance(1)
	for _, adb := range []*state.AccountsDB{accounts, kappAccounts, peers} {
		_, err := adb.Commit()
		require.NoError(t, err)
	}
	// a block in progress
	setPendingRewards(7)
	setMaxSupply(200)
	addBalance(9)
	kappsJournal := kappAccounts.JournalLen()

	var touched []string
	record := func(name string) { touched = append(touched, name) }
	processingCacher := &mock.AccountsCacherStub{
		GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
			record("GetExistingUser")
			return nil, nil
		},
		GetExistingKappCalled: func([]byte) (state.KAppAccountHandler, error) {
			record("GetExistingKapp")
			return nil, nil
		},
		LoadUserCalled: func([]byte) (state.UserAccountHandler, error) {
			record("LoadUser")
			return nil, nil
		},
		LoadKAppCalled: func([]byte) (state.KAppAccountHandler, error) {
			record("LoadKApp")
			return nil, nil
		},
		LoadKAppUncachedCalled: func([]byte) (state.KAppAccountHandler, error) {
			record("LoadKAppUncached")
			return nil, nil
		},
		SaveAllCalled: func() error {
			record("SaveAll")
			return nil
		},
		UpdateKappCalled: func(state.AccountHandler) error {
			record("UpdateKapp")
			return nil
		},
	}

	stateComponents := &factory.StateComponents{
		AccountsAdapter: accounts,
		KAppsAdapter:    kappAccounts,
		PeersAdapter:    peers,
		KAppController:  kappControllerTripwire{},
		KAppArgs: kappcontroller.ArgsNewKApp{
			Hasher:         hasher,
			Marshalizer:    marshalizer,
			PubkeyConv:     mock.NewPubkeyConverterMock(32),
			ForkController: mock.NewForkControllerStub(),
			AccountsCacher: processingCacher,
			RatingsData:    &mock.RatingsInfoMock{},
		},
	}
	api, err := createAPIState(stateComponents)
	require.NoError(t, err)
	require.True(t, api.kappController.IsReadOnly())

	pending, err := api.kappController.GetValidatorsKApp().GetPendingRewards(address)
	require.NoError(t, err)
	require.Equal(t, int64(5), pending, "the controller reads the committed rewards")

	acc, err := api.accounts.GetExistingAccount(address)
	require.NoError(t, err)
	require.Equal(t, int64(1), acc.(state.UserAccountHandler).GetBalance(nil, false), "accounts read the committed balance")

	validators, err := api.kapps.LoadAccount(kapps.ValidatorsKAppAddress)
	require.NoError(t, err)
	require.ErrorIs(t, api.kapps.SaveAccount(validators), state.ErrReadOnlyView)
	require.ErrorIs(t, api.accounts.SaveAccount(acc), state.ErrReadOnlyView)

	require.Empty(t, touched, "the REST state reached the processing cacher")
	require.Equal(t, kappsJournal, kappAccounts.JournalLen(), "building the REST state wrote to the live KApps")

	livePending, err := kappAccounts.GetExistingAccount(kapps.ValidatorsKAppAddress)
	require.NoError(t, err)
	value := livePending.(state.KAppAccountHandler).GetStorage(pendingRewardsKey)
	require.Equal(t, uint64(7), binary.BigEndian.Uint64(value), "the live state still holds the block in progress")

	// a node built from the options createNode uses answers REST reads from the API state
	conv, err := pubkeyConverter.NewBech32PubkeyConverter(len(address))
	require.NoError(t, err)
	n, err := node.NewNode(append(stateOptions(stateComponents, api),
		node.WithAddressPubkeyConverter(conv),
		node.WithForkController(mock.NewForkControllerStub()),
		node.WithInternalMarshalizer(marshalizer),
	)...)
	require.NoError(t, err)
	restAccount, err := n.GetAccount(conv.Encode(address))
	require.NoError(t, err)
	require.Equal(t, int64(1), restAccount.GetBalance(nil, false), "the node reads the committed balance")
	require.Equal(t, int64(5), restAccount.GetAllowance(), "the node adds the committed pending rewards")
	asset, err := n.GetAsset("KFI")
	require.NoError(t, err)
	require.Equal(t, int64(100), asset.GetMaxSupply(), "the node reads the committed KApps")

	// Merkle proofs need block processing's own accounts: a committed reader cannot build them
	_, err = n.GetProof(conv.Encode(address))
	require.NoError(t, err)
}

// kappControllerTripwire stands in for block processing's KApp controller: anything but the nil
// check panics.
type kappControllerTripwire struct{ kapp.KAppController }

// IsInterfaceNil -
func (kappControllerTripwire) IsInterfaceNil() bool { return false }
