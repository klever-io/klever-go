package state_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestNewPeerAccountsDB_WithNilTrieShouldErr(t *testing.T) {
	t.Parallel()

	adb, err := state.NewPeerAccountsDB(
		nil,
		&mock.HasherMock{},
		&mock.MarshalizerMock{},
		&mock.AccountsFactoryStub{},
		core.Normal,
	)

	assert.True(t, check.IfNil(adb))
	assert.Equal(t, common.ErrNilTrie, err)
}

func TestNewPeerAccountsDB_WithNilHasherShouldErr(t *testing.T) {
	t.Parallel()

	adb, err := state.NewPeerAccountsDB(
		&mock.TrieStub{},
		nil,
		&mock.MarshalizerMock{},
		&mock.AccountsFactoryStub{},
		core.Normal,
	)

	assert.True(t, check.IfNil(adb))
	assert.Equal(t, common.ErrNilHasher, err)
}

func TestNewPeerAccountsDB_WithNilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	adb, err := state.NewPeerAccountsDB(
		&mock.TrieStub{},
		&mock.HasherMock{},
		nil,
		&mock.AccountsFactoryStub{},
		core.Normal,
	)

	assert.True(t, check.IfNil(adb))
	assert.Equal(t, common.ErrNilMarshalizer, err)
}

func TestNewPeerAccountsDB_WithNilAddressFactoryShouldErr(t *testing.T) {
	t.Parallel()

	adb, err := state.NewPeerAccountsDB(
		&mock.TrieStub{},
		&mock.HasherMock{},
		&mock.MarshalizerMock{},
		nil,
		core.Normal,
	)

	assert.True(t, check.IfNil(adb))
	assert.Equal(t, common.ErrNilAccountFactory, err)
}

func TestNewPeerAccountsDB_OkValsShouldWork(t *testing.T) {
	t.Parallel()

	adb, err := state.NewPeerAccountsDB(
		&mock.TrieStub{
			GetStorageManagerCalled: func() data.StorageManager {
				return &mock.StorageManagerStub{
					DatabaseCalled: func() data.DBWriteCacher {
						return mock.NewMemDbMock()
					},
				}
			},
		},
		&mock.HasherMock{},
		&mock.MarshalizerMock{},
		&mock.AccountsFactoryStub{},
		core.Normal,
	)

	assert.Nil(t, err)
	assert.False(t, check.IfNil(adb))
}

func TestPeerAccountsDB_SnapshotState(t *testing.T) {
	t.Parallel()

	enterCalled := false
	exitCalled := false
	snapshotCalled := false
	rootHash := []byte("root-hash")

	adb, _ := state.NewPeerAccountsDB(
		&mock.TrieStub{
			GetStorageManagerCalled: func() data.StorageManager {
				return &mock.StorageManagerStub{
					DatabaseCalled: func() data.DBWriteCacher {
						return mock.NewMemDbMock()
					},
					EnterPruningBufferingModeCalled: func() {
						enterCalled = true
					},
					ExitPruningBufferingModeCalled: func() {
						exitCalled = true
					},
					TakeSnapshotCalled: func(hash []byte) {
						snapshotCalled = true
						assert.Equal(t, rootHash, hash)
					},
				}
			},
		},
		&mock.HasherMock{},
		&mock.MarshalizerMock{},
		&mock.AccountsFactoryStub{},
		core.Normal,
	)

	adb.SnapshotState(rootHash, nil)

	assert.True(t, enterCalled)
	assert.True(t, exitCalled)
	assert.True(t, snapshotCalled)
}

func TestPeerAccountsDB_SetStateCheckpoint(t *testing.T) {
	t.Parallel()

	enterCalled := false
	exitCalled := false
	checkpointCalled := false
	rootHash := []byte("checkpoint-root")

	adb, _ := state.NewPeerAccountsDB(
		&mock.TrieStub{
			GetStorageManagerCalled: func() data.StorageManager {
				return &mock.StorageManagerStub{
					DatabaseCalled: func() data.DBWriteCacher {
						return mock.NewMemDbMock()
					},
					EnterPruningBufferingModeCalled: func() {
						enterCalled = true
					},
					ExitPruningBufferingModeCalled: func() {
						exitCalled = true
					},
					SetCheckpointCalled: func(hash []byte) {
						checkpointCalled = true
						assert.Equal(t, rootHash, hash)
					},
				}
			},
		},
		&mock.HasherMock{},
		&mock.MarshalizerMock{},
		&mock.AccountsFactoryStub{},
		core.Normal,
	)

	adb.SetStateCheckpoint(rootHash, nil)

	assert.True(t, enterCalled)
	assert.True(t, exitCalled)
	assert.True(t, checkpointCalled)
}

func TestPeerAccountsDB_RecreateAllTries(t *testing.T) {
	t.Parallel()

	t.Run("successful recreation", func(t *testing.T) {
		t.Parallel()

		rootHash := []byte("trie-root")
		recreatedTrie := &mock.TrieStub{}

		adb, _ := state.NewPeerAccountsDB(
			&mock.TrieStub{
				GetStorageManagerCalled: func() data.StorageManager {
					return &mock.StorageManagerStub{
						DatabaseCalled: func() data.DBWriteCacher {
							return mock.NewMemDbMock()
						},
					}
				},
				RecreateCalled: func(root []byte) (data.Trie, error) {
					assert.Equal(t, rootHash, root)
					return recreatedTrie, nil
				},
			},
			&mock.HasherMock{},
			&mock.MarshalizerMock{},
			&mock.AccountsFactoryStub{},
			core.Normal,
		)

		tries, err := adb.RecreateAllTries(rootHash, nil)
		assert.Nil(t, err)
		assert.NotNil(t, tries)
		assert.Equal(t, 1, len(tries))
		assert.Equal(t, recreatedTrie, tries[string(rootHash)])
	})

	t.Run("recreation error", func(t *testing.T) {
		t.Parallel()

		rootHash := []byte("trie-root")
		expectedErr := common.ErrNilTrie

		adb, _ := state.NewPeerAccountsDB(
			&mock.TrieStub{
				GetStorageManagerCalled: func() data.StorageManager {
					return &mock.StorageManagerStub{
						DatabaseCalled: func() data.DBWriteCacher {
							return mock.NewMemDbMock()
						},
					}
				},
				RecreateCalled: func(root []byte) (data.Trie, error) {
					return nil, expectedErr
				},
			},
			&mock.HasherMock{},
			&mock.MarshalizerMock{},
			&mock.AccountsFactoryStub{},
			core.Normal,
		)

		tries, err := adb.RecreateAllTries(rootHash, nil)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, tries)
	})
}
