package state_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
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
	)

	assert.Nil(t, err)
	assert.False(t, check.IfNil(adb))
}
