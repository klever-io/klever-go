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

func createCompiledCodeWarmerWith(
	t *testing.T,
	accounts state.AccountsAdapter,
	warmUpConfig config.CompiledCodeWarmUpConfig,
) (interface{ Close() error }, bool, error) {
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
		warmUpConfig,
	)
	return warmer, check.IfNil(warmer), err
}

func newTestAccountsDB(t *testing.T) *state.AccountsDB {
	t.Helper()

	storageManager, err := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
	require.NoError(t, err)
	tr, err := trie.NewTrie(storageManager, &mock.MarshalizerMock{}, &mock.HasherMock{}, 5)
	require.NoError(t, err)
	adb, err := state.NewAccountsDB(tr, &mock.HasherMock{}, &mock.MarshalizerMock{}, stateFactory.NewAccountCreator(), core.Normal)
	require.NoError(t, err)
	return adb
}

func TestCreateCompiledCodeWarmer_AccountsDBGetsAWarmer(t *testing.T) {
	t.Parallel()

	enabled := true
	for _, warmUpConfig := range []config.CompiledCodeWarmUpConfig{{}, {Enabled: &enabled}} {
		warmer, isNil, err := createCompiledCodeWarmerWith(t, newTestAccountsDB(t), warmUpConfig)
		require.NoError(t, err)
		require.False(t, isNil, "an absent or true setting enables the warmer")
		require.NoError(t, warmer.Close())
	}
}

func TestCreateCompiledCodeWarmer_DisabledInConfigGetsNoWarmer(t *testing.T) {
	t.Parallel()

	disabled := false
	_, isNil, err := createCompiledCodeWarmerWith(t, newTestAccountsDB(t), config.CompiledCodeWarmUpConfig{Enabled: &disabled})
	require.NoError(t, err)
	require.True(t, isNil)
}

func TestCreateCompiledCodeWarmer_AdapterWithoutCodeReadGetsNoWarmer(t *testing.T) {
	t.Parallel()

	_, isNil, err := createCompiledCodeWarmerWith(t, &mock.AccountsStub{}, config.CompiledCodeWarmUpConfig{})
	require.NoError(t, err)
	require.True(t, isNil, "an adapter that can only read through GetExistingAccount must not get a warmer")
}
