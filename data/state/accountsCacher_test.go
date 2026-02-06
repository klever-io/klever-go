package state_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ------------ NewAccountsCacher

func TestAccountsCacher_WithNilAccountsAdapterShouldErr(t *testing.T) {
	t.Parallel()

	acc, err := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			nil,
			&mock.AccountsStub{},
			&mock.AccountsStub{},
		},
	)

	assert.Nil(t, acc)
	assert.Equal(t, common.ErrNilAccountsAdapter, err)
}

func TestAccountsCacher_WithNilKappAdapterShouldErr(t *testing.T) {
	t.Parallel()

	acc, err := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			nil,
			&mock.AccountsStub{},
		},
	)

	assert.Nil(t, acc)
	assert.Equal(t, common.ErrNilKAppAccountsAdapter, err)
}

func TestAccountsCacher_WithNilPeerAdapterShouldErr(t *testing.T) {
	t.Parallel()

	acc, err := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{},
			nil,
		},
	)

	assert.Nil(t, acc)
	assert.Equal(t, common.ErrNilPeerAccountsAdapter, err)
}

//------- SaveUser

func TestAccountsCacher_SaveUserShouldWork(t *testing.T) {
	t.Parallel()

	testAddress := make([]byte, 32)
	existingAccount, _ := state.NewUserAccount(testAddress)

	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{
				GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
					return existingAccount, nil
				},
			},
			&mock.AccountsStub{},
			&mock.AccountsStub{},
		},
	)

	//add in map
	acc.ResetAll(true)
	user, err := acc.GetExistingUser(testAddress)
	require.Nil(t, err)

	err = acc.SaveUser(user)
	assert.True(t, errors.Is(err, nil))
}

//------- GetExistingUser

func TestAccountsCacher_GetExistingUserNotFoundInMapShouldErr(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{
				GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
					return nil, common.ErrNilAccountHandler
				},
			},
			&mock.AccountsStub{},
			&mock.AccountsStub{},
		},
	)

	address := make([]byte, 32)
	_, err := acc.GetExistingUser(address)
	assert.True(t, errors.Is(err, common.ErrNilAccountHandler))
}

func TestAccountsCacher_GetExistingUserNotFoundInMapShouldAddExisting(t *testing.T) {
	t.Parallel()

	testAddress := make([]byte, 32)
	existingAccount, _ := state.NewUserAccount(testAddress)
	getExistingAccountCalled := 0

	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{
				GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
					getExistingAccountCalled++
					return existingAccount, nil
				},
			},
			&mock.AccountsStub{},
			&mock.AccountsStub{},
		},
	)

	acc.ResetAll(true)
	_, err := acc.GetExistingUser(testAddress)
	assert.True(t, errors.Is(err, nil))
	account, _ := acc.GetExistingUser(testAddress)
	assert.Equal(t, account, existingAccount)
	assert.Equal(t, getExistingAccountCalled, 1)
}

//------- GetExistingKapp

func TestAccountsCacher_GetExistingKappNotFoundInMapShouldErr(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{
				GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
					return nil, common.ErrNilAccountHandler
				},
			},
			&mock.AccountsStub{},
		},
	)

	address := make([]byte, 32)
	_, err := acc.GetExistingKapp(address)
	assert.True(t, errors.Is(err, common.ErrNilAccountHandler))
}

func TestAccountsCacher_GetExistingKappNotFoundInMapShouldAddExisting(t *testing.T) {
	t.Parallel()

	testAddress := make([]byte, 32)
	existingAccount, _ := state.NewKAppAccount(testAddress)
	getExistingAccountCalled := 0

	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{
				GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
					getExistingAccountCalled++
					return existingAccount, nil
				},
			},
			&mock.AccountsStub{},
		},
	)

	acc.ResetAll(true)
	_, err := acc.GetExistingKapp(testAddress)
	assert.True(t, errors.Is(err, nil))
	account, _ := acc.GetExistingKapp(testAddress)
	assert.Equal(t, account, existingAccount)
	assert.Equal(t, getExistingAccountCalled, 1)
}

//------- GetExistingPeer

func TestAccountsCacher_GetExistingPeerNotFoundInMapShouldErr(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{},
			&mock.AccountsStub{
				GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
					return nil, common.ErrNilAccountHandler
				},
			},
		},
	)

	address := make([]byte, 32)
	_, err := acc.GetExistingPeer(address)
	assert.True(t, errors.Is(err, common.ErrNilAccountHandler))
}

func TestAccountsCacher_GetExistingPeerNotFoundInMapShouldAddExisting(t *testing.T) {
	t.Parallel()

	testAddress := make([]byte, 32)
	existingAccount, _ := state.NewPeerAccount(testAddress)
	getExistingAccountCalled := 0

	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{},
			&mock.AccountsStub{
				GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
					getExistingAccountCalled++
					return existingAccount, nil
				},
			},
		},
	)

	acc.ResetAll(true)
	_, err := acc.GetExistingPeer(testAddress)
	assert.True(t, errors.Is(err, nil))
	account, _ := acc.GetExistingPeer(testAddress)
	assert.Equal(t, account, existingAccount)
	assert.Equal(t, getExistingAccountCalled, 1)
}

//------- LoadUser

func TestAccountsCacher_LoadUserNotFoundInMapShouldCreateEmpty(t *testing.T) {
	t.Parallel()

	newAccount := state.NewEmptyUserAccount()

	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{
				LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
					newAccount.Address = address
					return newAccount, nil
				},
			},
			&mock.AccountsStub{},
			&mock.AccountsStub{},
		},
	)

	address := make([]byte, 32)
	_, err := acc.LoadUser(address)

	assert.True(t, errors.Is(err, nil))
	assert.Equal(t, newAccount.Address, address)
}

//------- LoadKapp

func TestAccountsCacher_LoadKappNotFoundInMapShouldCreateEmpty(t *testing.T) {
	t.Parallel()

	newAccount := state.NewEmptyKAppAccount()

	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{
				LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
					newAccount.Address = address
					return newAccount, nil
				},
			},
			&mock.AccountsStub{},
		},
	)

	address := make([]byte, 32)
	_, err := acc.LoadKApp(address)

	assert.True(t, errors.Is(err, nil))
	assert.Equal(t, newAccount.Address, address)
}

//------- LoadPeer

func TestAccountsCacher_LoadPeerNotFoundInMapShouldCreateEmpty(t *testing.T) {
	t.Parallel()

	newAccount := state.NewEmptyPeerAccount()

	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{},
			&mock.AccountsStub{
				LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
					newAccount.OwnerAddress = address
					return newAccount, nil
				},
			},
		},
	)

	address := make([]byte, 32)
	_, err := acc.LoadPeer(address)

	assert.True(t, errors.Is(err, nil))
	assert.Equal(t, newAccount.OwnerAddress, address)
}

//------- ResetAll

func TestAccountsCacher_ResetAllShouldWork(t *testing.T) {
	t.Parallel()

	newUserAccount := state.NewEmptyUserAccount()
	newKappAccount := state.NewEmptyKAppAccount()
	newPeerAccount := state.NewEmptyPeerAccount()
	getUserExistingAccountCalled := 0
	getKappExistingAccountCalled := 0
	getPeerExistingAccountCalled := 0

	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{
				LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
					newUserAccount.Address = address
					return newUserAccount, nil
				},
				GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
					newUserAccount.Address = address
					getUserExistingAccountCalled++
					return newUserAccount, nil
				},
			},
			&mock.AccountsStub{
				LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
					newKappAccount.Address = address
					return newKappAccount, nil
				},
				GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
					newKappAccount.Address = address
					getKappExistingAccountCalled++
					return newKappAccount, nil
				},
			},
			&mock.AccountsStub{
				LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
					newPeerAccount.OwnerAddress = address
					return newPeerAccount, nil
				},
				GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
					newPeerAccount.OwnerAddress = address
					getPeerExistingAccountCalled++
					return newPeerAccount, nil
				},
			},
		},
	)

	userAddress := make([]byte, 32)
	_, _ = acc.LoadUser(userAddress)

	kappAddress := make([]byte, 32)
	_, _ = acc.LoadKApp(kappAddress)

	peerAddress := make([]byte, 32)
	_, _ = acc.LoadPeer(peerAddress)

	acc.ResetAll(true)

	_, _ = acc.GetExistingUser(userAddress)
	_, _ = acc.GetExistingKapp(kappAddress)
	_, _ = acc.GetExistingPeer(peerAddress)

	assert.Equal(t, getUserExistingAccountCalled, 1)
	assert.Equal(t, getKappExistingAccountCalled, 1)
	assert.Equal(t, getPeerExistingAccountCalled, 1)
}

//------- SaveAll

func TestAccountsCacher_SaveAllShouldWork(t *testing.T) {
	t.Parallel()

	newUserAccount := state.NewEmptyUserAccount()
	newKappAccount := state.NewEmptyKAppAccount()
	newPeerAccount := state.NewEmptyPeerAccount()
	saveUserAccountCalled := 0
	saveKappAccountCalled := 0
	savePeerAccountCalled := 0

	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{
				LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
					newUserAccount.Address = address
					return newUserAccount, nil
				},
				SaveAccountCalled: func(account state.AccountHandler) error {
					saveUserAccountCalled++
					return nil
				},
			},
			&mock.AccountsStub{
				LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
					newKappAccount.Address = address
					return newKappAccount, nil
				},
				SaveAccountCalled: func(account state.AccountHandler) error {
					saveKappAccountCalled++
					return nil
				},
			},
			&mock.AccountsStub{
				LoadAccountCalled: func(address []byte) (state.AccountHandler, error) {
					newPeerAccount.OwnerAddress = address
					return newPeerAccount, nil
				},
				SaveAccountCalled: func(account state.AccountHandler) error {
					savePeerAccountCalled++
					return nil
				},
			},
		},
	)

	acc.ResetAll(true)

	userAddress := make([]byte, 32)
	_, _ = acc.LoadUser(userAddress)

	kappAddress := make([]byte, 32)
	_, _ = acc.LoadKApp(kappAddress)

	peerAddress := make([]byte, 32)
	_, _ = acc.LoadPeer(peerAddress)

	err := acc.SaveAll()
	assert.True(t, errors.Is(err, nil))
	assert.Equal(t, saveUserAccountCalled, 1)
	assert.Equal(t, saveKappAccountCalled, 1)
	assert.Equal(t, savePeerAccountCalled, 1)
}

//------- UpdateUser

func TestAccountsCacher_UpdateUserCacheEnabledReturnsNil(t *testing.T) {
	t.Parallel()

	saveCalled := false
	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{
				SaveAccountCalled: func(account state.AccountHandler) error {
					saveCalled = true
					return nil
				},
			},
			&mock.AccountsStub{},
			&mock.AccountsStub{},
		},
	)

	acc.ResetAll(true)
	userAccount := state.NewEmptyUserAccount()
	err := acc.UpdateUser(userAccount)
	assert.Nil(t, err)
	assert.False(t, saveCalled)
}

func TestAccountsCacher_UpdateUserCacheDisabledCallsSaveAccount(t *testing.T) {
	t.Parallel()

	saveCalled := false
	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{
				SaveAccountCalled: func(account state.AccountHandler) error {
					saveCalled = true
					return nil
				},
			},
			&mock.AccountsStub{},
			&mock.AccountsStub{},
		},
	)

	acc.ResetAll(false)
	userAccount := state.NewEmptyUserAccount()
	err := acc.UpdateUser(userAccount)
	assert.Nil(t, err)
	assert.True(t, saveCalled)
}

//------- UpdateKapp

func TestAccountsCacher_UpdateKappCacheEnabledReturnsNil(t *testing.T) {
	t.Parallel()

	saveCalled := false
	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{
				SaveAccountCalled: func(account state.AccountHandler) error {
					saveCalled = true
					return nil
				},
			},
			&mock.AccountsStub{},
		},
	)

	acc.ResetAll(true)
	kappAccount := state.NewEmptyKAppAccount()
	err := acc.UpdateKapp(kappAccount)
	assert.Nil(t, err)
	assert.False(t, saveCalled)
}

func TestAccountsCacher_UpdateKappCacheDisabledCallsSaveAccount(t *testing.T) {
	t.Parallel()

	saveCalled := false
	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{
				SaveAccountCalled: func(account state.AccountHandler) error {
					saveCalled = true
					return nil
				},
			},
			&mock.AccountsStub{},
		},
	)

	acc.ResetAll(false)
	kappAccount := state.NewEmptyKAppAccount()
	err := acc.UpdateKapp(kappAccount)
	assert.Nil(t, err)
	assert.True(t, saveCalled)
}

//------- UpdatePeer

func TestAccountsCacher_UpdatePeerCacheEnabledReturnsNil(t *testing.T) {
	t.Parallel()

	saveCalled := false
	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{},
			&mock.AccountsStub{
				SaveAccountCalled: func(account state.AccountHandler) error {
					saveCalled = true
					return nil
				},
			},
		},
	)

	acc.ResetAll(true)
	peerAccount := state.NewEmptyPeerAccount()
	err := acc.UpdatePeer(peerAccount)
	assert.Nil(t, err)
	assert.False(t, saveCalled)
}

func TestAccountsCacher_UpdatePeerCacheDisabledCallsSaveAccount(t *testing.T) {
	t.Parallel()

	saveCalled := false
	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{},
			&mock.AccountsStub{
				SaveAccountCalled: func(account state.AccountHandler) error {
					saveCalled = true
					return nil
				},
			},
		},
	)

	acc.ResetAll(false)
	peerAccount := state.NewEmptyPeerAccount()
	err := acc.UpdatePeer(peerAccount)
	assert.Nil(t, err)
	assert.True(t, saveCalled)
}

//------- IsInterfaceNil

func TestAccountsCacher_IsInterfaceNilReturnsFalseForValidInstance(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewAccountsCacher(
		state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{},
			&mock.AccountsStub{},
		},
	)

	assert.False(t, acc.IsInterfaceNil())
}

func TestAccountsCacher_RemoveCode(t *testing.T) {
	t.Parallel()

	t.Run("GetExistingUser error", func(t *testing.T) {
		t.Parallel()

		acc, _ := state.NewAccountsCacher(
			state.ArgsAcccountCacher{
				&mock.AccountsStub{
					GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
						return nil, common.ErrNilAccountHandler
					},
				},
				&mock.AccountsStub{},
				&mock.AccountsStub{},
			},
		)

		address := []byte("test-address")
		err := acc.RemoveCode(address)
		assert.True(t, errors.Is(err, common.ErrNilAccountHandler))
	})

	t.Run("RemoveAccountCode error", func(t *testing.T) {
		t.Parallel()

		testAddress := []byte("test-address")
		existingAccount, _ := state.NewUserAccount(testAddress)
		expectedErr := common.ErrInvalidValue

		acc, _ := state.NewAccountsCacher(
			state.ArgsAcccountCacher{
				&mock.AccountsStub{
					GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
						return existingAccount, nil
					},
					RemoveAccountCodeCalled: func(address []byte) error {
						return expectedErr
					},
				},
				&mock.AccountsStub{},
				&mock.AccountsStub{},
			},
		)

		err := acc.RemoveCode(testAddress)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		testAddress := []byte("test-address")
		existingAccount, _ := state.NewUserAccount(testAddress)
		existingAccount.SetCode([]byte("contract-code"))
		existingAccount.SetCodeHash([]byte("code-hash"))

		removeCodeCalled := false

		acc, _ := state.NewAccountsCacher(
			state.ArgsAcccountCacher{
				&mock.AccountsStub{
					GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
						return existingAccount, nil
					},
					RemoveAccountCodeCalled: func(address []byte) error {
						removeCodeCalled = true
						assert.Equal(t, testAddress, address)
						return nil
					},
				},
				&mock.AccountsStub{},
				&mock.AccountsStub{},
			},
		)

		acc.ResetAll(true)
		err := acc.RemoveCode(testAddress)
		assert.Nil(t, err)
		assert.True(t, removeCodeCalled)
		assert.Equal(t, 0, len(existingAccount.GetCodeHash()))
	})
}

func TestAccountsCacher_TypeAssertionErrors(t *testing.T) {
	t.Parallel()

	// Use concrete types that satisfy AccountHandler but not the target handler interface:
	// PeerAccount is not UserAccountHandler or KAppAccountHandler
	// UserAccount is not PeerAccountHandler or KAppAccountHandler
	peerAcc := state.NewEmptyPeerAccount()
	userAcc := state.NewEmptyUserAccount()

	t.Run("GetExistingUser wrong type", func(t *testing.T) {
		t.Parallel()
		acc, _ := state.NewAccountsCacher(state.ArgsAcccountCacher{
			&mock.AccountsStub{GetExistingAccountCalled: func([]byte) (state.AccountHandler, error) { return peerAcc, nil }},
			&mock.AccountsStub{},
			&mock.AccountsStub{},
		})
		_, err := acc.GetExistingUser([]byte("addr"))
		assert.Equal(t, common.ErrWrongTypeAssertion, err)
	})

	t.Run("GetExistingKapp wrong type", func(t *testing.T) {
		t.Parallel()
		acc, _ := state.NewAccountsCacher(state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{GetExistingAccountCalled: func([]byte) (state.AccountHandler, error) { return userAcc, nil }},
			&mock.AccountsStub{},
		})
		_, err := acc.GetExistingKapp([]byte("addr"))
		assert.Equal(t, common.ErrWrongTypeAssertion, err)
	})

	t.Run("GetExistingPeer wrong type", func(t *testing.T) {
		t.Parallel()
		acc, _ := state.NewAccountsCacher(state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{},
			&mock.AccountsStub{GetExistingAccountCalled: func([]byte) (state.AccountHandler, error) { return userAcc, nil }},
		})
		_, err := acc.GetExistingPeer([]byte("addr"))
		assert.Equal(t, common.ErrWrongTypeAssertion, err)
	})

	t.Run("LoadUser wrong type", func(t *testing.T) {
		t.Parallel()
		acc, _ := state.NewAccountsCacher(state.ArgsAcccountCacher{
			&mock.AccountsStub{LoadAccountCalled: func([]byte) (state.AccountHandler, error) { return peerAcc, nil }},
			&mock.AccountsStub{},
			&mock.AccountsStub{},
		})
		_, err := acc.LoadUser([]byte("addr"))
		assert.Equal(t, common.ErrWrongTypeAssertion, err)
	})

	t.Run("LoadKApp wrong type", func(t *testing.T) {
		t.Parallel()
		acc, _ := state.NewAccountsCacher(state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{LoadAccountCalled: func([]byte) (state.AccountHandler, error) { return userAcc, nil }},
			&mock.AccountsStub{},
		})
		_, err := acc.LoadKApp([]byte("addr"))
		assert.Equal(t, common.ErrWrongTypeAssertion, err)
	})

	t.Run("LoadPeer wrong type", func(t *testing.T) {
		t.Parallel()
		acc, _ := state.NewAccountsCacher(state.ArgsAcccountCacher{
			&mock.AccountsStub{},
			&mock.AccountsStub{},
			&mock.AccountsStub{LoadAccountCalled: func([]byte) (state.AccountHandler, error) { return userAcc, nil }},
		})
		_, err := acc.LoadPeer([]byte("addr"))
		assert.Equal(t, common.ErrWrongTypeAssertion, err)
	})
}
