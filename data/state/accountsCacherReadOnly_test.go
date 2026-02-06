package state_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/state"
	"github.com/stretchr/testify/assert"
)

func TestNewReadOnlyAccountsCacher_WithNilAccountsShouldErr(t *testing.T) {
	t.Parallel()

	roAcc, err := state.NewReadOnlyAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: nil,
			Kapps:    &mock.AccountsStub{},
			Peers:    &mock.AccountsStub{},
		},
	)

	assert.Nil(t, roAcc)
	assert.Equal(t, common.ErrNilAccountsAdapter, err)
}

func TestNewReadOnlyAccountsCacher_WithNilKappsShouldErr(t *testing.T) {
	t.Parallel()

	roAcc, err := state.NewReadOnlyAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: &mock.AccountsStub{},
			Kapps:    nil,
			Peers:    &mock.AccountsStub{},
		},
	)

	assert.Nil(t, roAcc)
	assert.Equal(t, common.ErrNilKAppAccountsAdapter, err)
}

func TestNewReadOnlyAccountsCacher_WithNilPeersShouldErr(t *testing.T) {
	t.Parallel()

	roAcc, err := state.NewReadOnlyAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: &mock.AccountsStub{},
			Kapps:    &mock.AccountsStub{},
			Peers:    nil,
		},
	)

	assert.Nil(t, roAcc)
	assert.Equal(t, common.ErrNilPeerAccountsAdapter, err)
}

func TestNewReadOnlyAccountsCacher_ShouldWork(t *testing.T) {
	t.Parallel()

	roAcc, err := state.NewReadOnlyAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: &mock.AccountsStub{},
			Kapps:    &mock.AccountsStub{},
			Peers:    &mock.AccountsStub{},
		},
	)

	assert.NotNil(t, roAcc)
	assert.Nil(t, err)
	assert.False(t, roAcc.IsInterfaceNil())
}

func TestReadOnlyAccountsCacher_DelegatesGetExisting(t *testing.T) {
	t.Parallel()

	testAddress := []byte("test-address")
	userAcc, _ := state.NewUserAccount(testAddress)
	kappAcc, _ := state.NewKAppAccount(testAddress)
	peerAcc, _ := state.NewPeerAccount(testAddress)

	userCalled := false
	kappCalled := false
	peerCalled := false

	roAcc, _ := state.NewReadOnlyAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: &mock.AccountsStub{
				GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
					userCalled = true
					return userAcc, nil
				},
			},
			Kapps: &mock.AccountsStub{
				GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
					kappCalled = true
					return kappAcc, nil
				},
			},
			Peers: &mock.AccountsStub{
				GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
					peerCalled = true
					return peerAcc, nil
				},
			},
		},
	)

	roAcc.ResetAll(true)

	gotUser, err := roAcc.GetExistingUser(testAddress)
	assert.Nil(t, err)
	assert.Equal(t, userAcc, gotUser)
	assert.True(t, userCalled)

	gotKapp, err := roAcc.GetExistingKapp(testAddress)
	assert.Nil(t, err)
	assert.Equal(t, kappAcc, gotKapp)
	assert.True(t, kappCalled)

	gotPeer, err := roAcc.GetExistingPeer(testAddress)
	assert.Nil(t, err)
	assert.Equal(t, peerAcc, gotPeer)
	assert.True(t, peerCalled)
}

func TestReadOnlyAccountsCacher_DelegatesLoad(t *testing.T) {
	t.Parallel()

	testAddress := []byte("test-address")
	userAcc, _ := state.NewUserAccount(testAddress)
	kappAcc, _ := state.NewKAppAccount(testAddress)
	peerAcc, _ := state.NewPeerAccount(testAddress)

	userCalled := false
	kappCalled := false
	peerCalled := false

	roAcc, _ := state.NewReadOnlyAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: &mock.AccountsStub{
				LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
					userCalled = true
					return userAcc, nil
				},
			},
			Kapps: &mock.AccountsStub{
				LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
					kappCalled = true
					return kappAcc, nil
				},
			},
			Peers: &mock.AccountsStub{
				LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
					peerCalled = true
					return peerAcc, nil
				},
			},
		},
	)

	gotUser, err := roAcc.LoadUser(testAddress)
	assert.Nil(t, err)
	assert.Equal(t, testAddress, gotUser.AddressBytes())
	assert.True(t, userCalled)

	gotKapp, err := roAcc.LoadKApp(testAddress)
	assert.Nil(t, err)
	assert.Equal(t, testAddress, gotKapp.AddressBytes())
	assert.True(t, kappCalled)

	gotPeer, err := roAcc.LoadPeer(testAddress)
	assert.Nil(t, err)
	assert.Equal(t, testAddress, gotPeer.AddressBytes())
	assert.True(t, peerCalled)
}

func TestReadOnlyAccountsCacher_ResetAllDelegates(t *testing.T) {
	t.Parallel()

	resetCalled := false

	roAcc, _ := state.NewReadOnlyAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: &mock.AccountsStub{
				LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
					acc := state.NewEmptyUserAccount()
					acc.Address = address
					return acc, nil
				},
			},
			Kapps: &mock.AccountsStub{},
			Peers: &mock.AccountsStub{},
		},
	)

	roAcc.ResetAll(true)
	_, _ = roAcc.LoadUser([]byte("addr1"))

	roAcc.ResetAll(false)
	resetCalled = true

	assert.True(t, resetCalled)
}

func TestReadOnlyAccountsCacher_WriteOperationsReturnNil(t *testing.T) {
	t.Parallel()

	saveCalled := false
	updateCalled := false
	removeCodeCalled := false
	removeAccountCalled := false

	roAcc, _ := state.NewReadOnlyAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: &mock.AccountsStub{
				SaveAccountCalled: func(account state.AccountHandler) error {
					saveCalled = true
					return errors.New("should not be called")
				},
				RemoveAccountCodeCalled: func(address []byte) error {
					removeCodeCalled = true
					return errors.New("should not be called")
				},
				RemoveAccountCalled: func(address []byte) error {
					removeAccountCalled = true
					return errors.New("should not be called")
				},
			},
			Kapps: &mock.AccountsStub{
				SaveAccountCalled: func(account state.AccountHandler) error {
					updateCalled = true
					return errors.New("should not be called")
				},
			},
			Peers: &mock.AccountsStub{
				SaveAccountCalled: func(account state.AccountHandler) error {
					updateCalled = true
					return errors.New("should not be called")
				},
			},
		},
	)

	userAcc := state.NewEmptyUserAccount()
	kappAcc := state.NewEmptyKAppAccount()
	peerAcc := state.NewEmptyPeerAccount()

	tests := []struct {
		name      string
		operation func() error
	}{
		{
			name:      "SaveAll",
			operation: func() error { return roAcc.SaveAll() },
		},
		{
			name:      "UpdateUser",
			operation: func() error { return roAcc.UpdateUser(userAcc) },
		},
		{
			name:      "UpdateKapp",
			operation: func() error { return roAcc.UpdateKapp(kappAcc) },
		},
		{
			name:      "UpdatePeer",
			operation: func() error { return roAcc.UpdatePeer(peerAcc) },
		},
		{
			name:      "SaveUser",
			operation: func() error { return roAcc.SaveUser(userAcc) },
		},
		{
			name:      "RemoveAccount",
			operation: func() error { return roAcc.RemoveAccount([]byte("addr")) },
		},
		{
			name:      "RemoveCode",
			operation: func() error { return roAcc.RemoveCode([]byte("addr")) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()
			assert.Nil(t, err)
		})
	}

	assert.False(t, saveCalled)
	assert.False(t, updateCalled)
	assert.False(t, removeCodeCalled)
	assert.False(t, removeAccountCalled)
}

func TestReadOnlyAccountsCacher_GetCodeDelegates(t *testing.T) {
	t.Parallel()

	expectedCode := []byte("contract-code")
	codeHash := []byte("code-hash")
	getCalled := false

	roAcc, _ := state.NewReadOnlyAccountsCacher(
		state.ArgsAcccountCacher{
			Accounts: &mock.AccountsStub{
				GetCodeCalled: func(hash []byte) []byte {
					getCalled = true
					assert.Equal(t, codeHash, hash)
					return expectedCode
				},
			},
			Kapps: &mock.AccountsStub{},
			Peers: &mock.AccountsStub{},
		},
	)

	code := roAcc.GetCode(codeHash)
	assert.Equal(t, expectedCode, code)
	assert.True(t, getCalled)
}
