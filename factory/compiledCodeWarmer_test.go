package factory_test

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/data/state"
	stateFactory "github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/trie"
	notifierMock "github.com/klever-io/klever-go/eventNotifier/mock"
	"github.com/klever-io/klever-go/factory"
	kvmConfig "github.com/klever-io/klever-go/kvm/config"
	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/require"
)

func createCompiledCodeWarmerWith(t *testing.T, accounts state.AccountsAdapter) (interface{ Close() error }, bool, error) {
	t.Helper()

	forkController, err := fork.NewForkController(config.EnableEpochs{}, &mock.EpochNotifierStub{})
	require.NoError(t, err)

	warmer, err := factory.CreateCompiledCodeWarmerForTest(
		accounts,
		mock.NewPoolsHolderMock(),
		&contextmock.BlockchainHookStub{},
		notifierMock.NewGasScheduleNotifierMock(kvmConfig.MakeGasMapForTests()),
		forkController,
		&mock.HasherMock{},
		statusHandler.NewNilStatusHandler(),
	)
	return warmer, check.IfNil(warmer), err
}

func TestCreateCompiledCodeWarmer_AccountsDBGetsAWarmer(t *testing.T) {
	t.Parallel()

	storageManager, err := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
	require.NoError(t, err)
	tr, err := trie.NewTrie(storageManager, &mock.MarshalizerMock{}, &mock.HasherMock{}, 5)
	require.NoError(t, err)
	adb, err := state.NewAccountsDB(tr, &mock.HasherMock{}, &mock.MarshalizerMock{}, stateFactory.NewAccountCreator(), core.Normal)
	require.NoError(t, err)

	warmer, isNil, err := createCompiledCodeWarmerWith(t, adb)
	require.NoError(t, err)
	require.False(t, isNil)
	require.NoError(t, warmer.Close())
}

func TestCreateCompiledCodeWarmer_AdapterWithoutCodeHashReadGetsNoWarmer(t *testing.T) {
	t.Parallel()

	_, isNil, err := createCompiledCodeWarmerWith(t, &mock.AccountsStub{})
	require.NoError(t, err)
	require.True(t, isNil, "an adapter that can only read through GetExistingAccount must not get a warmer")
}
