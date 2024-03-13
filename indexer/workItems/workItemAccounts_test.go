package workItems_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/indexer/mock"
	"github.com/klever-io/klever-go/indexer/workItems"
	"github.com/stretchr/testify/require"
)

func TestItemAccounts_Save(t *testing.T) {
	called := false
	itemAccounts := workItems.NewItemAccounts(
		&mock.ElasticProcessorStub{
			SaveAccountsCalled: func(_ int64, _ []*data.Account) error {
				called = true
				return nil
			},
		},
		0,
		[]state.UserAccountHandler{},
	)
	require.False(t, itemAccounts.IsInterfaceNil())

	err := itemAccounts.Save()
	require.NoError(t, err)
	require.True(t, called)
}

func TestItemAccounts_SaveAccountsShouldErr(t *testing.T) {
	localErr := errors.New("local err")
	itemAccounts := workItems.NewItemAccounts(
		&mock.ElasticProcessorStub{
			SaveAccountsCalled: func(_ int64, _ []*data.Account) error {
				return localErr
			},
		},
		0,
		[]state.UserAccountHandler{},
	)
	require.False(t, itemAccounts.IsInterfaceNil())

	err := itemAccounts.Save()
	require.Equal(t, localErr, err)
}
