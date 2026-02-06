package state_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/tools/atomic"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func generateAccountDBFromTrie(trie data.Trie) *state.AccountsDB {
	accnt, _ := state.NewAccountsDB(trie, &mock.HasherMock{}, &mock.MarshalizerMock{}, &mock.AccountsFactoryStub{
		CreateAccountCalled: func(address []byte) (state.AccountHandler, error) {
			return mock.NewAccountWrapMock(address), nil
		},
	}, core.Normal)
	return accnt
}

func generateAccount() *mock.AccountWrapMock {
	return mock.NewAccountWrapMock(make([]byte, 32))
}

func generateAddressAccountAccountsDB(trie data.Trie) ([]byte, *mock.AccountWrapMock, *state.AccountsDB) {
	adr := make([]byte, 32)
	account := mock.NewAccountWrapMock(adr)

	adb := generateAccountDBFromTrie(trie)

	return adr, account, adb
}

//------- NewAccountsDB

func TestNewAccountsDB_WithNilTrieShouldErr(t *testing.T) {
	t.Parallel()

	adb, err := state.NewAccountsDB(
		nil,
		&mock.HasherMock{},
		&mock.MarshalizerMock{},
		&mock.AccountsFactoryStub{},
		core.Normal,
	)

	assert.True(t, check.IfNil(adb))
	assert.Equal(t, common.ErrNilTrie, err)
}

func TestNewAccountsDB_WithNilHasherShouldErr(t *testing.T) {
	t.Parallel()

	adb, err := state.NewAccountsDB(
		&mock.TrieStub{},
		nil,
		&mock.MarshalizerMock{},
		&mock.AccountsFactoryStub{},
		core.Normal,
	)

	assert.True(t, check.IfNil(adb))
	assert.Equal(t, common.ErrNilHasher, err)
}

func TestNewAccountsDB_WithNilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	adb, err := state.NewAccountsDB(
		&mock.TrieStub{},
		&mock.HasherMock{},
		nil,
		&mock.AccountsFactoryStub{},
		core.Normal,
	)

	assert.True(t, check.IfNil(adb))
	assert.Equal(t, common.ErrNilMarshalizer, err)
}

func TestNewAccountsDB_WithNilAddressFactoryShouldErr(t *testing.T) {
	t.Parallel()

	adb, err := state.NewAccountsDB(
		&mock.TrieStub{},
		&mock.HasherMock{},
		&mock.MarshalizerMock{},
		nil,
		core.Normal,
	)

	assert.True(t, check.IfNil(adb))
	assert.Equal(t, common.ErrNilAccountFactory, err)
}

func TestNewAccountsDB_OkValsShouldWork(t *testing.T) {
	t.Parallel()

	adb, err := state.NewAccountsDB(
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

func TestNewAccountsDB_SetsNumCheckpoints(t *testing.T) {
	t.Parallel()

	numCheckpointsKey := []byte("state checkpoint")
	numCheckpoints := uint32(121)
	db := mock.NewMemDbMock()

	numCheckpointsVal := make([]byte, 4)
	binary.BigEndian.PutUint32(numCheckpointsVal, numCheckpoints)
	_ = db.Put(numCheckpointsKey, numCheckpointsVal)

	adb, _ := state.NewAccountsDB(
		&mock.TrieStub{
			GetStorageManagerCalled: func() data.StorageManager {
				return &mock.StorageManagerStub{
					DatabaseCalled: func() data.DBWriteCacher {
						return db
					},
				}
			},
		},
		&mock.HasherMock{},
		&mock.MarshalizerMock{},
		&mock.AccountsFactoryStub{},
		core.Normal,
	)

	assert.Equal(t, numCheckpoints, adb.GetNumCheckpoints())
}

func TestAccountsDB_SetStateCheckpointSavesNumCheckpoints(t *testing.T) {
	numCheckpointsKey := []byte("state checkpoint")
	numCheckpoints := 50
	db := mock.NewMemDbMock()
	adb, _ := state.NewAccountsDB(
		&mock.TrieStub{
			GetStorageManagerCalled: func() data.StorageManager {
				return &mock.StorageManagerStub{
					DatabaseCalled: func() data.DBWriteCacher {
						return db
					},
				}
			},
			RecreateCalled: func(root []byte) (data.Trie, error) {
				return &mock.TrieStub{
					GetAllLeavesOnChannelCalled: func(_ []byte) (chan data.KeyValueHolder, error) {
						ch := make(chan data.KeyValueHolder)
						close(ch)

						return ch, nil
					},
				}, nil
			},
		},
		&mock.HasherMock{},
		&mock.MarshalizerMock{},
		&mock.AccountsFactoryStub{},
		core.ImportDb,
	)

	for range numCheckpoints {
		adb.SetStateCheckpoint([]byte("rootHash"), context.Background())
	}

	val, err := db.Get(numCheckpointsKey)
	assert.Nil(t, err)

	numCheckpointsRecovered := binary.BigEndian.Uint32(val)
	assert.Equal(t, uint32(numCheckpoints), numCheckpointsRecovered)
	assert.Equal(t, uint32(numCheckpoints), adb.GetNumCheckpoints())
}

//------- SaveAccount

func TestAccountsDB_SaveAccountNilAccountShouldErr(t *testing.T) {
	t.Parallel()

	adb := generateAccountDBFromTrie(&mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	})

	err := adb.SaveAccount(nil)
	assert.True(t, errors.Is(err, common.ErrNilAccountHandler))
}

func TestAccountsDB_SaveAccountErrWhenGettingOldAccountShouldErr(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("trie get err")
	adb := generateAccountDBFromTrie(&mock.TrieStub{
		GetCalled: func(key []byte) (i []byte, err error) {
			return nil, expectedErr
		},
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	})

	err := adb.SaveAccount(generateAccount())
	assert.Equal(t, expectedErr, err)
}

func TestAccountsDB_SaveAccountNilOldAccount(t *testing.T) {
	t.Parallel()

	adb := generateAccountDBFromTrie(&mock.TrieStub{
		GetCalled: func(key []byte) (i []byte, err error) {
			return nil, nil
		},
		UpdateCalled: func(key, value []byte) error {
			return nil
		},
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	})

	acc, _ := state.NewUserAccount([]byte("someAddress"))
	err := adb.SaveAccount(acc)
	assert.Nil(t, err)
	assert.Equal(t, 1, adb.JournalLen())
}

func TestAccountsDB_SaveAccountExistingOldAccount(t *testing.T) {
	t.Parallel()

	acc, _ := state.NewUserAccount([]byte("someAddress"))

	adb := generateAccountDBFromTrie(&mock.TrieStub{
		GetCalled: func(key []byte) (i []byte, err error) {
			return (&mock.MarshalizerMock{}).Marshal(acc)
		},
		UpdateCalled: func(key, value []byte) error {
			return nil
		},
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	})

	err := adb.SaveAccount(acc)
	assert.Nil(t, err)
	assert.Equal(t, 1, adb.JournalLen())
}

func TestAccountsDB_SaveAccountSavesCodeAndDataTrieForUserAccount(t *testing.T) {
	t.Parallel()

	updateCalled := 0
	trieStub := &mock.TrieStub{
		GetCalled: func(key []byte) (i []byte, err error) {
			return nil, nil
		},
		UpdateCalled: func(key, value []byte) error {
			return nil
		},
		RootCalled: func() (i []byte, err error) {
			return []byte("rootHash"), nil
		},
	}

	adb := generateAccountDBFromTrie(&mock.TrieStub{
		RootCalled: func() (i []byte, err error) {
			return []byte("rootHash"), nil
		},
		GetCalled: func(key []byte) (i []byte, err error) {
			return nil, nil
		},
		UpdateCalled: func(key, value []byte) error {
			updateCalled++
			return nil
		},
		RecreateCalled: func(root []byte) (d data.Trie, err error) {
			return trieStub, nil
		},
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	})

	acc, _ := state.NewUserAccount([]byte("someAddress"))

	err := adb.SaveAccount(acc)
	assert.Nil(t, err)
	assert.Equal(t, 1, updateCalled)

	rootHash, err := adb.RootHash()
	assert.Nil(t, err)

	assert.NotNil(t, rootHash)
}

func TestAccountsDB_SaveAccountMalfunctionMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	account := generateAccount()
	mockTrie := &mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	marshalizer := &mock.MarshalizerMock{}
	adb, _ := state.NewAccountsDB(mockTrie, &mock.HasherMock{}, marshalizer, &mock.AccountsFactoryStub{
		CreateAccountCalled: func(address []byte) (state.AccountHandler, error) {
			return mock.NewAccountWrapMock(address), nil
		},
	},
		core.Normal)

	marshalizer.Fail = true

	//should return error
	err := adb.SaveAccount(account)

	assert.NotNil(t, err)
}

func TestAccountsDB_SaveAccountWithSomeValuesShouldWork(t *testing.T) {
	t.Parallel()

	ts := &mock.TrieStub{
		GetCalled: func(key []byte) (i []byte, err error) {
			return nil, nil
		},
		UpdateCalled: func(key, value []byte) error {
			return nil
		},
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	_, account, adb := generateAddressAccountAccountsDB(ts)

	//should return error
	err := adb.SaveAccount(account)
	assert.Nil(t, err)
}

//------- RemoveAccount

func TestAccountsDB_RemoveAccountShouldWork(t *testing.T) {
	t.Parallel()

	wasCalled := false
	marshalizer := &mock.MarshalizerMock{}
	trieStub := &mock.TrieStub{
		GetCalled: func(key []byte) (i []byte, err error) {
			return marshalizer.Marshal(mock.AccountWrapMock{})
		},
		UpdateCalled: func(key, value []byte) error {
			wasCalled = true
			return nil
		},
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}

	adr := make([]byte, 32)
	adb := generateAccountDBFromTrie(trieStub)

	err := adb.RemoveAccount(adr)
	assert.Nil(t, err)
	assert.True(t, wasCalled)
	assert.Equal(t, 2, adb.JournalLen())
}

//------- LoadAccount

func TestAccountsDB_LoadAccountMalfunctionTrieShouldErr(t *testing.T) {
	t.Parallel()

	trieStub := &mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	adr := make([]byte, 32)
	adb := generateAccountDBFromTrie(trieStub)

	_, err := adb.LoadAccount(adr)
	assert.NotNil(t, err)
}

func TestAccountsDB_LoadAccountNotFoundShouldCreateEmpty(t *testing.T) {
	t.Parallel()

	trieStub := &mock.TrieStub{
		GetCalled: func(key []byte) (i []byte, e error) {
			return nil, nil
		},
		UpdateCalled: func(key, value []byte) error {
			return nil
		},
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}

	adr := make([]byte, 32)
	adb := generateAccountDBFromTrie(trieStub)

	accountExpected := mock.NewAccountWrapMock(adr)
	accountRecovered, err := adb.LoadAccount(adr)

	assert.Equal(t, accountExpected, accountRecovered)
	assert.Nil(t, err)
}

func TestAccountsDB_LoadAccountExistingShouldLoadDataTrie(t *testing.T) {
	t.Parallel()

	acc := generateAccount()
	acc.SetRootHash([]byte("root hash"))
	dataTrie := &mock.TrieStub{}
	marshalizer := &mock.MarshalizerMock{}

	trieStub := &mock.TrieStub{
		GetCalled: func(key []byte) (i []byte, e error) {
			if bytes.Equal(key, acc.AddressBytes()) {
				return marshalizer.Marshal(acc)
			}
			return nil, nil
		},
		RecreateCalled: func(root []byte) (d data.Trie, err error) {
			return dataTrie, nil
		},
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}

	adb := generateAccountDBFromTrie(trieStub)
	retrievedAccount, err := adb.LoadAccount(acc.AddressBytes())
	assert.Nil(t, err)

	account, _ := retrievedAccount.(state.UserAccountHandler)
	assert.Equal(t, dataTrie, account.DataTrie())
}

//------- GetExistingAccount

func TestAccountsDB_GetExistingAccountMalfunctionTrieShouldErr(t *testing.T) {
	t.Parallel()

	trieStub := &mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	adr := make([]byte, 32)
	adb := generateAccountDBFromTrie(trieStub)

	_, err := adb.GetExistingAccount(adr)
	assert.NotNil(t, err)
}

func TestAccountsDB_GetExistingAccountNotFoundShouldRetNil(t *testing.T) {
	t.Parallel()

	trieStub := &mock.TrieStub{
		GetCalled: func(key []byte) (i []byte, e error) {
			return nil, nil
		},
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}

	adr := make([]byte, 32)
	adb := generateAccountDBFromTrie(trieStub)

	account, err := adb.GetExistingAccount(adr)
	assert.Equal(t, common.ErrAccNotFound, err)
	assert.Nil(t, account)
	//no journal entry shall be created
	assert.Equal(t, 0, adb.JournalLen())
}

func TestAccountsDB_GetExistingAccountFoundShouldRetAccount(t *testing.T) {
	t.Parallel()

	acc := generateAccount()
	acc.SetRootHash([]byte("root hash"))
	dataTrie := &mock.TrieStub{}
	marshalizer := &mock.MarshalizerMock{}

	trieStub := &mock.TrieStub{
		GetCalled: func(key []byte) (i []byte, e error) {
			if bytes.Equal(key, acc.AddressBytes()) {
				return marshalizer.Marshal(acc)
			}
			return nil, nil
		},
		RecreateCalled: func(root []byte) (d data.Trie, err error) {
			return dataTrie, nil
		},
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}

	adb := generateAccountDBFromTrie(trieStub)
	retrievedAccount, err := adb.GetExistingAccount(acc.AddressBytes())
	assert.Nil(t, err)

	account, _ := retrievedAccount.(state.UserAccountHandler)
	assert.Equal(t, dataTrie, account.DataTrie())
}

//------- getAccount

func TestAccountsDB_GetAccountAccountNotFound(t *testing.T) {
	t.Parallel()

	trieMock := &mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	adr, _, _ := generateAddressAccountAccountsDB(trieMock)

	//Step 1. Create an account + its DbAccount representation
	testAccount := mock.NewAccountWrapMock(adr)
	testAccount.MockValue = 45

	//Step 2. marshalize the DbAccount
	marshalizer := mock.MarshalizerMock{}
	buff, err := marshalizer.Marshal(testAccount)
	assert.Nil(t, err)

	trieMock.GetCalled = func(key []byte) (bytes []byte, e error) {
		//whatever the key is, return the same marshalized DbAccount
		return buff, nil
	}

	adb, _ := state.NewAccountsDB(trieMock, &mock.HasherMock{}, &marshalizer, &mock.AccountsFactoryStub{
		CreateAccountCalled: func(address []byte) (state.AccountHandler, error) {
			return mock.NewAccountWrapMock(address), nil
		},
	}, core.Normal)

	//Step 3. call get, should return a copy of DbAccount, recover an Account object
	recoveredAccount, err := adb.GetAccount(adr)
	assert.Nil(t, err)

	//Step 4. Let's test
	assert.Equal(t, testAccount.MockValue, recoveredAccount.(*mock.AccountWrapMock).MockValue)
}

//------- RetrieveData

func TestAccountsDB_LoadDataNilRootShouldRetNil(t *testing.T) {
	t.Parallel()

	tr := &mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	_, account, adb := generateAddressAccountAccountsDB(tr)

	//since root is nil, result should be nil and data trie should be nil
	err := adb.LoadDataTrie(account)
	assert.Nil(t, err)
	assert.Nil(t, account.DataTrie())
}

func TestAccountsDB_LoadDataBadLengthShouldErr(t *testing.T) {
	t.Parallel()

	_, account, adb := generateAddressAccountAccountsDB(&mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	})

	account.SetRootHash([]byte("12345"))

	//should return error
	err := adb.LoadDataTrie(account)
	assert.NotNil(t, err)
}

func TestAccountsDB_LoadDataMalfunctionTrieShouldErr(t *testing.T) {
	t.Parallel()

	account := generateAccount()
	account.SetRootHash([]byte("12345"))

	mockTrie := &mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	adb := generateAccountDBFromTrie(mockTrie)

	//should return error
	err := adb.LoadDataTrie(account)
	assert.NotNil(t, err)
}

func TestAccountsDB_LoadDataNotFoundRootShouldReturnErr(t *testing.T) {
	t.Parallel()

	_, account, adb := generateAddressAccountAccountsDB(&mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	})

	rootHash := make([]byte, mock.HasherMock{}.Size())
	rootHash[0] = 1
	account.SetRootHash(rootHash)

	//should return error
	err := adb.LoadDataTrie(account)
	assert.NotNil(t, err)
	fmt.Println(err.Error())
}

func TestAccountsDB_LoadDataWithSomeValuesShouldWork(t *testing.T) {
	t.Parallel()

	rootHash := make([]byte, mock.HasherMock{}.Size())
	rootHash[0] = 1
	keyRequired := []byte{65, 66, 67}
	val := []byte{32, 33, 34}

	trieVal := append(val, keyRequired...)
	trieVal = append(trieVal, []byte("identifier")...)

	dataTrie := &mock.TrieStub{
		GetCalled: func(key []byte) (i []byte, e error) {
			if bytes.Equal(key, keyRequired) {
				return trieVal, nil
			}

			return nil, nil
		},
	}

	account := generateAccount()
	mockTrie := &mock.TrieStub{
		RecreateCalled: func(root []byte) (trie data.Trie, e error) {
			if !bytes.Equal(root, rootHash) {
				return nil, errors.New("bad root hash")
			}

			return dataTrie, nil
		},
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	adb := generateAccountDBFromTrie(mockTrie)

	account.SetRootHash(rootHash)

	//should not return error
	err := adb.LoadDataTrie(account)
	assert.Nil(t, err)

	//verify data
	dataRecov, err := account.DataTrieTracker().RetrieveValue(keyRequired)
	assert.Nil(t, err)
	assert.Equal(t, val, dataRecov)
}

//------- Commit

func TestAccountsDB_CommitShouldCallCommitFromTrie(t *testing.T) {
	t.Parallel()

	commitCalled := 0
	marshalizer := &mock.MarshalizerMock{}
	serializedAccount, _ := marshalizer.Marshal(mock.AccountWrapMock{})
	trieStub := mock.TrieStub{
		CommitCalled: func() error {
			commitCalled++

			return nil
		},
		RootCalled: func() (i []byte, e error) {
			return nil, nil
		},
		GetCalled: func(key []byte) (i []byte, err error) {
			return serializedAccount, nil
		},
		RecreateCalled: func(root []byte) (trie data.Trie, err error) {
			return &mock.TrieStub{
				GetCalled: func(key []byte) (i []byte, err error) {
					return []byte("doge"), nil
				},
				UpdateCalled: func(key, value []byte) error {
					return nil
				},
				CommitCalled: func() error {
					commitCalled++

					return nil
				},
			}, nil
		},
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}

	adb := generateAccountDBFromTrie(&trieStub)

	state2, _ := adb.LoadAccount(make([]byte, 32))
	_ = state2.(state.UserAccountHandler).DataTrieTracker().SaveKeyValue([]byte("dog"), []byte("puppy"))
	_ = adb.SaveAccount(state2)

	_, err := adb.Commit()
	assert.Nil(t, err)
	//one commit for the JournalEntryData and one commit for the main trie
	assert.Equal(t, 2, commitCalled)
}

//------- RecreateTrie

func TestAccountsDB_RecreateTrieMalfunctionTrieShouldErr(t *testing.T) {
	t.Parallel()

	wasCalled := false

	errExpected := errors.New("failure")
	trieStub := &mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	trieStub.RecreateCalled = func(root []byte) (tree data.Trie, e error) {
		wasCalled = true
		return nil, errExpected
	}

	adb := generateAccountDBFromTrie(trieStub)

	err := adb.RecreateTrie(nil)
	assert.Equal(t, errExpected, err)
	assert.True(t, wasCalled)
}

func TestAccountsDB_RecreateTrieOutputsNilTrieShouldErr(t *testing.T) {
	t.Parallel()

	wasCalled := false

	trieStub := mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	trieStub.RecreateCalled = func(root []byte) (tree data.Trie, e error) {
		wasCalled = true
		return nil, nil
	}

	adb := generateAccountDBFromTrie(&trieStub)
	err := adb.RecreateTrie(nil)

	assert.Equal(t, common.ErrNilTrie, err)
	assert.True(t, wasCalled)

}

func TestAccountsDB_RecreateTrieOkValsShouldWork(t *testing.T) {
	t.Parallel()

	wasCalled := false

	trieStub := mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
		RecreateCalled: func(root []byte) (data.Trie, error) {
			wasCalled = true
			return &mock.TrieStub{}, nil
		},
	}

	adb := generateAccountDBFromTrie(&trieStub)
	err := adb.RecreateTrie(nil)

	assert.Nil(t, err)
	assert.True(t, wasCalled)

}

func TestAccountsDB_CancelPrune(t *testing.T) {
	t.Parallel()

	cancelPruneWasCalled := false
	trieStub := &mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				CancelPruneCalled: func(rootHash []byte, identifier data.TriePruningIdentifier) {
					cancelPruneWasCalled = true
				},
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	adb := generateAccountDBFromTrie(trieStub)
	adb.CancelPrune([]byte("roothash"), data.OldRoot)

	assert.True(t, cancelPruneWasCalled)
}

func TestAccountsDB_PruneTrie(t *testing.T) {
	t.Parallel()

	pruneTrieWasCalled := atomic.Flag{}
	trieStub := &mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				PruneCalled: func(rootHash []byte, identifier data.TriePruningIdentifier) {
					pruneTrieWasCalled.Set()
				},
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	adb := generateAccountDBFromTrie(trieStub)
	adb.PruneTrie([]byte("roothash"), data.OldRoot)
	assert.True(t, pruneTrieWasCalled.IsSet())
}

func TestAccountsDB_SnapshotState(t *testing.T) {
	t.Parallel()

	takeSnapshotWasCalled := false
	snapshotMut := sync.Mutex{}
	trieStub := &mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				TakeSnapshotCalled: func(rootHash []byte) {
					snapshotMut.Lock()
					takeSnapshotWasCalled = true
					snapshotMut.Unlock()
				},
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	adb := generateAccountDBFromTrie(trieStub)
	adb.SnapshotState([]byte("roothash"), context.Background())
	time.Sleep(time.Second)

	snapshotMut.Lock()
	assert.True(t, takeSnapshotWasCalled)
	snapshotMut.Unlock()
}

func TestAccountsDB_SetStateCheckpoint(t *testing.T) {
	t.Parallel()

	setCheckPointWasCalled := false
	snapshotMut := sync.Mutex{}
	trieStub := &mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				SetCheckpointCalled: func(rootHash []byte) {
					snapshotMut.Lock()
					setCheckPointWasCalled = true
					snapshotMut.Unlock()
				},
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	adb := generateAccountDBFromTrie(trieStub)
	adb.SetStateCheckpoint([]byte("roothash"), context.Background())
	time.Sleep(time.Second)

	snapshotMut.Lock()
	assert.True(t, setCheckPointWasCalled)
	snapshotMut.Unlock()
}

func TestAccountsDB_IsPruningEnabled(t *testing.T) {
	t.Parallel()

	trieStub := &mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				IsPruningEnabledCalled: func() bool {
					return true
				},
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	adb := generateAccountDBFromTrie(trieStub)
	res := adb.IsPruningEnabled()

	assert.Equal(t, true, res)
}

func TestAccountsDB_RevertToSnapshotOutOfBounds(t *testing.T) {
	t.Parallel()

	trieStub := &mock.TrieStub{
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	adb := generateAccountDBFromTrie(trieStub)

	err := adb.RevertToSnapshot(1)
	assert.Equal(t, common.ErrSnapshotValueOutOfBounds, err)
}

func TestAccountsDB_RevertToSnapshotShouldWork(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	hsh := mock.HasherMock{}
	accFactory := factory.NewAccountCreator()
	storageManager, _ := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
	maxTrieLevelInMemory := uint(5)
	tr, _ := trie.NewTrie(storageManager, marshalizer, hsh, maxTrieLevelInMemory)

	adb, _ := state.NewAccountsDB(tr, hsh, marshalizer, accFactory, core.Normal)

	acc, _ := adb.LoadAccount(make([]byte, 32))
	_ = adb.SaveAccount(acc)

	err := adb.RevertToSnapshot(0)
	assert.Nil(t, err)

	expectedRoot := make([]byte, 32)
	root, err := adb.RootHash()
	assert.Nil(t, err)
	assert.Equal(t, expectedRoot, root)
}

func TestAccountsDB_RevertToSnapshotWithoutLastRootHashSet(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	hsh := mock.HasherMock{}
	accFactory := factory.NewAccountCreator()
	storageManager, _ := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
	maxTrieLevelInMemory := uint(5)
	tr, _ := trie.NewTrie(storageManager, marshalizer, hsh, maxTrieLevelInMemory)
	adb, _ := state.NewAccountsDB(tr, hsh, marshalizer, accFactory, core.Normal)

	err := adb.RevertToSnapshot(0)
	assert.Nil(t, err)

	rootHash, err := adb.RootHash()
	assert.Nil(t, err)
	assert.Equal(t, make([]byte, 32), rootHash)
}

func TestAccountsDB_RecreateTrieInvalidatesJournalEntries(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	hsh := mock.HasherMock{}
	accFactory := factory.NewAccountCreator()
	storageManager, _ := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
	maxTrieLevelInMemory := uint(5)
	tr, _ := trie.NewTrie(storageManager, marshalizer, hsh, maxTrieLevelInMemory)
	adb, _ := state.NewAccountsDB(tr, hsh, marshalizer, accFactory, core.Normal)

	address := make([]byte, 32)
	key := []byte("key")
	value := []byte("value")

	acc, _ := adb.LoadAccount(address)
	_ = adb.SaveAccount(acc)
	rootHash, _ := adb.Commit()

	acc, _ = adb.LoadAccount(address)
	_ = acc.(state.UserAccountHandler).DataTrieTracker().SaveKeyValue(key, value)
	_ = adb.SaveAccount(acc)

	assert.Equal(t, 2, adb.JournalLen())
	err := adb.RecreateTrie(rootHash)
	assert.Nil(t, err)
	assert.Equal(t, 0, adb.JournalLen())
}

func TestAccountsDB_RootHash(t *testing.T) {
	t.Parallel()

	rootHashCalled := false
	rootHash := []byte("root hash")
	trieStub := &mock.TrieStub{
		RootCalled: func() (i []byte, err error) {
			rootHashCalled = true
			return rootHash, nil
		},
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}
	adb := generateAccountDBFromTrie(trieStub)
	res, err := adb.RootHash()
	assert.Nil(t, err)
	assert.True(t, rootHashCalled)
	assert.Equal(t, rootHash, res)
}

func TestAccountsDB_GetAllLeaves(t *testing.T) {
	t.Parallel()

	getAllLeavesCalled := false
	trieStub := &mock.TrieStub{
		GetAllLeavesOnChannelCalled: func(rootHash []byte) (chan data.KeyValueHolder, error) {
			getAllLeavesCalled = true

			ch := make(chan data.KeyValueHolder)
			close(ch)

			return ch, nil
		},
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				DatabaseCalled: func() data.DBWriteCacher {
					return mock.NewMemDbMock()
				},
			}
		},
	}

	adb := generateAccountDBFromTrie(trieStub)
	_, err := adb.GetAllLeaves([]byte("root hash"), context.Background())
	assert.Nil(t, err)
	assert.True(t, getAllLeavesCalled)
}

// defaultStorageManager returns a standard GetStorageManagerCalled for TrieStub.
func defaultStorageManager() func() data.StorageManager {
	return func() data.StorageManager {
		return &mock.StorageManagerStub{
			DatabaseCalled: func() data.DBWriteCacher {
				return mock.NewMemDbMock()
			},
		}
	}
}

// newSimpleTrieStub returns a TrieStub with passthrough Get/Update and default storage.
func newSimpleTrieStub() *mock.TrieStub {
	return &mock.TrieStub{
		GetCalled:               func(key []byte) ([]byte, error) { return nil, nil },
		UpdateCalled:            func(key, value []byte) error { return nil },
		GetStorageManagerCalled: defaultStorageManager(),
	}
}

//------- ImportAccount

func TestAccountsDB_ImportAccount(t *testing.T) {
	t.Parallel()

	t.Run("nil account", func(t *testing.T) {
		adb := generateAccountDBFromTrie(&mock.TrieStub{GetStorageManagerCalled: defaultStorageManager()})
		err := adb.ImportAccount(nil)
		assert.True(t, errors.Is(err, common.ErrNilAccountHandler))
	})

	t.Run("valid account saves to trie without journal", func(t *testing.T) {
		updateCalled := false
		account := generateAccount()
		ts := newSimpleTrieStub()
		ts.UpdateCalled = func(key, value []byte) error {
			updateCalled = true
			assert.Equal(t, account.AddressBytes(), key)
			return nil
		}
		adb := generateAccountDBFromTrie(ts)
		err := adb.ImportAccount(account)
		assert.Nil(t, err)
		assert.True(t, updateCalled)
		assert.Equal(t, 0, adb.JournalLen())
	})

	t.Run("trie update error propagates", func(t *testing.T) {
		expectedErr := errors.New("trie update error")
		ts := newSimpleTrieStub()
		ts.UpdateCalled = func(key, value []byte) error { return expectedErr }
		adb := generateAccountDBFromTrie(ts)
		err := adb.ImportAccount(generateAccount())
		assert.Equal(t, expectedErr, err)
	})
}

//------- SaveAccounts

func TestAccountsDB_SaveAccounts(t *testing.T) {
	t.Parallel()

	t.Run("single account", func(t *testing.T) {
		adb := generateAccountDBFromTrie(newSimpleTrieStub())
		err := adb.SaveAccounts(generateAccount())
		assert.Nil(t, err)
		assert.Equal(t, 1, adb.JournalLen())
	})

	t.Run("multiple accounts", func(t *testing.T) {
		adb := generateAccountDBFromTrie(newSimpleTrieStub())
		err := adb.SaveAccounts(
			generateAccount(),
			mock.NewAccountWrapMock([]byte("address2")),
			mock.NewAccountWrapMock([]byte("address3")),
		)
		assert.Nil(t, err)
		assert.Equal(t, 3, adb.JournalLen())
	})

	t.Run("error on second account stops batch", func(t *testing.T) {
		expectedErr := errors.New("save error")
		getCalled := 0
		ts := newSimpleTrieStub()
		ts.GetCalled = func(key []byte) ([]byte, error) {
			getCalled++
			if getCalled == 2 {
				return nil, expectedErr
			}
			return nil, nil
		}
		adb := generateAccountDBFromTrie(ts)
		err := adb.SaveAccounts(generateAccount(), mock.NewAccountWrapMock([]byte("addr2")))
		assert.Equal(t, expectedErr, err)
		assert.Equal(t, 1, adb.JournalLen())
	})
}

//------- GetCode

func TestAccountsDB_GetCode(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	validEntry, _ := marshalizer.Marshal(&state.CodeEntry{Code: []byte("code"), NumReferences: 1})

	tests := []struct {
		name      string
		hash      []byte
		getCalled func([]byte) ([]byte, error)
		expected  []byte
	}{
		{"empty hash returns nil", []byte{}, nil, nil},
		{"trie error returns nil", []byte("h"), func([]byte) ([]byte, error) { return nil, errors.New("err") }, nil},
		{"unmarshal error returns nil", []byte("h"), func([]byte) ([]byte, error) { return []byte("bad"), nil }, nil},
		{"success returns code", []byte("h"), func([]byte) ([]byte, error) { return validEntry, nil }, []byte("code")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ts := &mock.TrieStub{GetCalled: tt.getCalled, GetStorageManagerCalled: defaultStorageManager()}
			adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &mock.AccountsFactoryStub{
				CreateAccountCalled: func(address []byte) (state.AccountHandler, error) {
					return mock.NewAccountWrapMock(address), nil
				},
			}, core.Normal)
			assert.Equal(t, tt.expected, adb.GetCode(tt.hash))
		})
	}
}

//------- RemoveAccountCode

func TestAccountsDB_RemoveAccountCode(t *testing.T) {
	t.Parallel()

	t.Run("nil address", func(t *testing.T) {
		adb := generateAccountDBFromTrie(&mock.TrieStub{GetStorageManagerCalled: defaultStorageManager()})
		assert.True(t, errors.Is(adb.RemoveAccountCode(nil), common.ErrNilAddress))
	})

	t.Run("account not found", func(t *testing.T) {
		adb := generateAccountDBFromTrie(newSimpleTrieStub())
		assert.True(t, errors.Is(adb.RemoveAccountCode([]byte("addr")), common.ErrAccNotFound))
	})

	t.Run("non-user account succeeds without removing code", func(t *testing.T) {
		address := []byte("address")
		marshalizer := &mock.MarshalizerMock{}
		accountBytes, _ := marshalizer.Marshal(mock.NewAccountWrapMock(address))
		ts := newSimpleTrieStub()
		ts.GetCalled = func(key []byte) ([]byte, error) {
			if bytes.Equal(key, address) {
				return accountBytes, nil
			}
			return nil, nil
		}
		adb := generateAccountDBFromTrie(ts)
		assert.Nil(t, adb.RemoveAccountCode(address))
		assert.True(t, adb.JournalLen() >= 1)
	})

	t.Run("user account with code deletes code entry", func(t *testing.T) {
		address := []byte("address")
		codeHash := []byte("codehash")
		userAcc, _ := state.NewUserAccount(address)
		userAcc.SetCodeHash(codeHash)

		marshalizer := &mock.MarshalizerMock{}
		accountBytes, _ := marshalizer.Marshal(userAcc)
		codeEntryBytes, _ := marshalizer.Marshal(&state.CodeEntry{Code: []byte("code"), NumReferences: 1})

		codeDeleted := false
		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				if bytes.Equal(key, address) {
					return accountBytes, nil
				}
				if bytes.Equal(key, codeHash) {
					return codeEntryBytes, nil
				}
				return nil, nil
			},
			UpdateCalled: func(key, value []byte) error {
				if bytes.Equal(key, codeHash) && value == nil {
					codeDeleted = true
				}
				return nil
			},
			GetStorageManagerCalled: defaultStorageManager(),
		}
		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &factory.AccountCreator{}, core.Normal)
		assert.Nil(t, adb.RemoveAccountCode(address))
		assert.True(t, codeDeleted)
		assert.Equal(t, 2, adb.JournalLen())
	})
}

//------- Code entry management (via SaveAccount with SetCode)

func TestAccountsDB_SaveAccount_CodeEntryManagement(t *testing.T) {
	t.Parallel()

	t.Run("new code creates entry with NumReferences=1", func(t *testing.T) {
		address := []byte("address")
		newCode := []byte("new smart contract code")
		hasher := &mock.HasherMock{}
		newCodeHash := hasher.Compute(string(newCode))
		marshalizer := &mock.MarshalizerMock{}

		codeEntryCreated := false
		ts := &mock.TrieStub{
			GetCalled:               func(key []byte) ([]byte, error) { return nil, nil },
			GetStorageManagerCalled: defaultStorageManager(),
			UpdateCalled: func(key, value []byte) error {
				if bytes.Equal(key, newCodeHash) {
					var entry state.CodeEntry
					_ = marshalizer.Unmarshal(&entry, value)
					assert.Equal(t, newCode, entry.Code)
					assert.Equal(t, uint32(1), entry.NumReferences)
					codeEntryCreated = true
				}
				return nil
			},
		}
		adb, _ := state.NewAccountsDB(ts, hasher, marshalizer, &factory.AccountCreator{}, core.Normal)
		acc, _ := state.NewUserAccount(address)
		acc.SetCode(newCode)
		assert.Nil(t, adb.SaveAccount(acc))
		assert.True(t, codeEntryCreated)
	})

	t.Run("existing code entry increments NumReferences", func(t *testing.T) {
		address := []byte("address")
		code := []byte("existing code")
		hasher := &mock.HasherMock{}
		codeHash := hasher.Compute(string(code))
		marshalizer := &mock.MarshalizerMock{}
		existingBytes, _ := marshalizer.Marshal(&state.CodeEntry{Code: code, NumReferences: 5})

		incremented := false
		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				if bytes.Equal(key, codeHash) {
					return existingBytes, nil
				}
				return nil, nil
			},
			UpdateCalled: func(key, value []byte) error {
				if bytes.Equal(key, codeHash) {
					var entry state.CodeEntry
					_ = marshalizer.Unmarshal(&entry, value)
					assert.Equal(t, uint32(6), entry.NumReferences)
					incremented = true
				}
				return nil
			},
			GetStorageManagerCalled: defaultStorageManager(),
		}
		adb, _ := state.NewAccountsDB(ts, hasher, marshalizer, &factory.AccountCreator{}, core.Normal)
		acc, _ := state.NewUserAccount(address)
		acc.SetCode(code)
		assert.Nil(t, adb.SaveAccount(acc))
		assert.True(t, incremented)
	})

	t.Run("same code hash skips code entry update", func(t *testing.T) {
		address := []byte("address")
		code := []byte("code")
		hasher := &mock.HasherMock{}
		codeHash := hasher.Compute(string(code))
		marshalizer := &mock.MarshalizerMock{}

		oldAcc, _ := state.NewUserAccount(address)
		oldAcc.SetCodeHash(codeHash)
		oldAccBytes, _ := marshalizer.Marshal(oldAcc)

		codeEntryLookedUp := false
		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				if bytes.Equal(key, address) {
					return oldAccBytes, nil
				}
				if bytes.Equal(key, codeHash) {
					codeEntryLookedUp = true
				}
				return nil, nil
			},
			UpdateCalled:            func(key, value []byte) error { return nil },
			GetStorageManagerCalled: defaultStorageManager(),
		}
		adb, _ := state.NewAccountsDB(ts, hasher, marshalizer, &factory.AccountCreator{}, core.Normal)
		newAcc, _ := state.NewUserAccount(address)
		newAcc.SetCode(code)
		assert.Nil(t, adb.SaveAccount(newAcc))
		assert.False(t, codeEntryLookedUp)
	})

	t.Run("replacing code with old refs > 1 decrements old entry", func(t *testing.T) {
		address := []byte("address")
		hasher := &mock.HasherMock{}
		marshalizer := &mock.MarshalizerMock{}
		oldCodeHash := hasher.Compute("old code")

		oldAcc, _ := state.NewUserAccount(address)
		oldAcc.SetCodeHash(oldCodeHash)
		oldAccBytes, _ := marshalizer.Marshal(oldAcc)
		oldCodeBytes, _ := marshalizer.Marshal(&state.CodeEntry{Code: []byte("old code"), NumReferences: 3})

		decremented := false
		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				if bytes.Equal(key, address) {
					return oldAccBytes, nil
				}
				if bytes.Equal(key, oldCodeHash) {
					return oldCodeBytes, nil
				}
				return nil, nil
			},
			UpdateCalled: func(key, value []byte) error {
				if bytes.Equal(key, oldCodeHash) && value != nil {
					var entry state.CodeEntry
					_ = marshalizer.Unmarshal(&entry, value)
					assert.Equal(t, uint32(2), entry.NumReferences)
					decremented = true
				}
				return nil
			},
			GetStorageManagerCalled: defaultStorageManager(),
		}
		adb, _ := state.NewAccountsDB(ts, hasher, marshalizer, &factory.AccountCreator{}, core.Normal)
		newAcc, _ := state.NewUserAccount(address)
		newAcc.SetCode([]byte("new code"))
		assert.Nil(t, adb.SaveAccount(newAcc))
		assert.True(t, decremented)
	})

	t.Run("replacing code with old refs <= 1 deletes old entry", func(t *testing.T) {
		address := []byte("address")
		hasher := &mock.HasherMock{}
		marshalizer := &mock.MarshalizerMock{}
		oldCodeHash := hasher.Compute("old code")

		oldAcc, _ := state.NewUserAccount(address)
		oldAcc.SetCodeHash(oldCodeHash)
		oldAccBytes, _ := marshalizer.Marshal(oldAcc)
		oldCodeBytes, _ := marshalizer.Marshal(&state.CodeEntry{Code: []byte("old code"), NumReferences: 1})

		deleted := false
		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				if bytes.Equal(key, address) {
					return oldAccBytes, nil
				}
				if bytes.Equal(key, oldCodeHash) {
					return oldCodeBytes, nil
				}
				return nil, nil
			},
			UpdateCalled: func(key, value []byte) error {
				if bytes.Equal(key, oldCodeHash) && value == nil {
					deleted = true
				}
				return nil
			},
			GetStorageManagerCalled: defaultStorageManager(),
		}
		adb, _ := state.NewAccountsDB(ts, hasher, marshalizer, &factory.AccountCreator{}, core.Normal)
		newAcc, _ := state.NewUserAccount(address)
		newAcc.SetCode([]byte("new code"))
		assert.Nil(t, adb.SaveAccount(newAcc))
		assert.True(t, deleted)
	})
}

//------- removeDataTrie (via RemoveAccount)

func TestAccountsDB_RemoveDataTrie(t *testing.T) {
	t.Parallel()

	t.Run("empty root hash skips data trie removal", func(t *testing.T) {
		address := []byte("address")
		acc, _ := state.NewUserAccount(address)
		marshalizer := &mock.MarshalizerMock{}
		accBytes, _ := marshalizer.Marshal(acc)

		recreateCalled := false
		ts := newSimpleTrieStub()
		ts.GetCalled = func(key []byte) ([]byte, error) { return accBytes, nil }
		ts.RecreateCalled = func(root []byte) (data.Trie, error) {
			recreateCalled = true
			return nil, nil
		}
		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &factory.AccountCreator{}, core.Normal)
		assert.Nil(t, adb.RemoveAccount(address))
		assert.False(t, recreateCalled)
	})

	t.Run("trie recreate error propagates", func(t *testing.T) {
		address := []byte("address")
		acc, _ := state.NewUserAccount(address)
		acc.SetRootHash([]byte("roothash"))
		marshalizer := &mock.MarshalizerMock{}
		accBytes, _ := marshalizer.Marshal(acc)

		expectedErr := errors.New("recreate error")
		ts := newSimpleTrieStub()
		ts.GetCalled = func(key []byte) ([]byte, error) {
			if bytes.Equal(key, address) {
				return accBytes, nil
			}
			return nil, nil
		}
		ts.RecreateCalled = func(root []byte) (data.Trie, error) { return nil, expectedErr }
		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &factory.AccountCreator{}, core.Normal)
		err := adb.RemoveAccount(address)
		assert.Contains(t, err.Error(), "recreate error")
	})

	t.Run("success collects obsolete hashes", func(t *testing.T) {
		address := []byte("address")
		acc, _ := state.NewUserAccount(address)
		acc.SetRootHash([]byte("roothash"))
		marshalizer := &mock.MarshalizerMock{}
		accBytes, _ := marshalizer.Marshal(acc)

		ts := newSimpleTrieStub()
		ts.GetCalled = func(key []byte) ([]byte, error) { return accBytes, nil }
		ts.RecreateCalled = func(root []byte) (data.Trie, error) {
			return &mock.TrieStub{
				GetAllHashesCalled: func() ([][]byte, error) {
					return [][]byte{[]byte("h1"), []byte("h2")}, nil
				},
			}, nil
		}
		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &factory.AccountCreator{}, core.Normal)
		assert.Nil(t, adb.RemoveAccount(address))
		assert.Equal(t, 3, adb.JournalLen()) // account + code + data trie
	})
}

//------- RecreateAllTries

func TestAccountsDB_RecreateAllTries(t *testing.T) {
	t.Parallel()

	rootHash := []byte("roothash")
	marshalizer := &mock.MarshalizerMock{}

	acc1, _ := state.NewUserAccount([]byte("addr1"))
	acc1.SetRootHash([]byte("datatrie1"))
	acc2, _ := state.NewUserAccount([]byte("addr2"))
	acc2.SetRootHash([]byte("datatrie2"))
	acc1Bytes, _ := marshalizer.Marshal(acc1)
	acc2Bytes, _ := marshalizer.Marshal(acc2)

	ch := make(chan data.KeyValueHolder, 2)
	ch <- &mock.KeyValueHolderStub{ValueCalled: func() []byte { return acc1Bytes }}
	ch <- &mock.KeyValueHolderStub{ValueCalled: func() []byte { return acc2Bytes }}
	close(ch)

	dataTrie1, dataTrie2, mainTrieRecreated := &mock.TrieStub{}, &mock.TrieStub{}, &mock.TrieStub{}
	ts := &mock.TrieStub{
		GetAllLeavesOnChannelCalled: func(hash []byte) (chan data.KeyValueHolder, error) { return ch, nil },
		RecreateCalled: func(root []byte) (data.Trie, error) {
			switch string(root) {
			case string(rootHash):
				return mainTrieRecreated, nil
			case "datatrie1":
				return dataTrie1, nil
			case "datatrie2":
				return dataTrie2, nil
			}
			return nil, errors.New("unexpected")
		},
		GetStorageManagerCalled: defaultStorageManager(),
	}

	adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &factory.AccountCreator{}, core.Normal)
	allTries, err := adb.RecreateAllTries(rootHash, context.Background())

	assert.Nil(t, err)
	assert.Equal(t, 3, len(allTries))
	assert.Equal(t, mainTrieRecreated, allTries[string(rootHash)])
	assert.Equal(t, dataTrie1, allTries["datatrie1"])
	assert.Equal(t, dataTrie2, allTries["datatrie2"])
}

//------- snapshotUserAccountDataTrie

func TestAccountsDB_SnapshotCheckpointsDataTries(t *testing.T) {
	rootHash := []byte("roothash")
	marshalizer := &mock.MarshalizerMock{}

	acc, _ := state.NewUserAccount([]byte("addr1"))
	acc.SetRootHash([]byte("datatrie1"))
	accBytes, _ := marshalizer.Marshal(acc)

	ch := make(chan data.KeyValueHolder, 1)
	ch <- &mock.KeyValueHolderStub{ValueCalled: func() []byte { return accBytes }}
	close(ch)

	checkpointCalls := 0
	ts := &mock.TrieStub{
		GetAllLeavesOnChannelCalled: func(hash []byte) (chan data.KeyValueHolder, error) { return ch, nil },
		GetStorageManagerCalled: func() data.StorageManager {
			return &mock.StorageManagerStub{
				SetCheckpointCalled: func(hash []byte) { checkpointCalls++ },
				DatabaseCalled:      func() data.DBWriteCacher { return mock.NewMemDbMock() },
			}
		},
	}

	adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &factory.AccountCreator{}, core.ImportDb)
	adb.SetStateCheckpoint(rootHash, context.Background())
	time.Sleep(100 * time.Millisecond)

	assert.True(t, checkpointCalls >= 2) // main trie + data trie
}

//------- RevertToSnapshot

func TestAccountsDB_RevertToSnapshot_NegativeSnapshotShouldErr(t *testing.T) {
	t.Parallel()
	adb := generateAccountDBFromTrie(&mock.TrieStub{GetStorageManagerCalled: defaultStorageManager()})
	assert.Equal(t, common.ErrSnapshotValueOutOfBounds, adb.RevertToSnapshot(-1))
}

func TestAccountsDB_RevertToSnapshot_RevertEntriesShouldWork(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	hsh := mock.HasherMock{}
	storageManager, _ := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
	tr, _ := trie.NewTrie(storageManager, marshalizer, hsh, uint(5))
	adb, _ := state.NewAccountsDB(tr, hsh, marshalizer, factory.NewAccountCreator(), core.Normal)

	acc, _ := adb.LoadAccount(make([]byte, 32))
	_ = adb.SaveAccount(acc)
	assert.True(t, adb.JournalLen() > 0)

	assert.Nil(t, adb.RevertToSnapshot(1))
	assert.Equal(t, 1, adb.JournalLen())
}

func TestAccountsDB_RevertToSnapshot_AboveBoundsShouldErr(t *testing.T) {
	t.Parallel()
	adb := generateAccountDBFromTrie(&mock.TrieStub{GetStorageManagerCalled: defaultStorageManager()})
	assert.Equal(t, common.ErrSnapshotValueOutOfBounds, adb.RevertToSnapshot(999))
}

func TestAccountsDB_RemoveAccount_ErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("nil address returns error", func(t *testing.T) {
		adb := generateAccountDBFromTrie(newSimpleTrieStub())
		err := adb.RemoveAccount(nil)
		assert.True(t, errors.Is(err, common.ErrNilAddress))
	})

	t.Run("getAccount error propagates", func(t *testing.T) {
		expectedErr := errors.New("get error")
		ts := &mock.TrieStub{
			GetCalled:               func(key []byte) ([]byte, error) { return nil, expectedErr },
			GetStorageManagerCalled: defaultStorageManager(),
		}
		adb := generateAccountDBFromTrie(ts)
		err := adb.RemoveAccount(make([]byte, 32))
		assert.Equal(t, expectedErr, err)
	})

	t.Run("account not found returns error", func(t *testing.T) {
		adb := generateAccountDBFromTrie(newSimpleTrieStub())
		err := adb.RemoveAccount(make([]byte, 32))
		assert.True(t, errors.Is(err, common.ErrAccNotFound))
	})
}

func TestAccountsDB_RecreateAllTries_ErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("GetAllLeavesOnChannel error", func(t *testing.T) {
		expectedErr := errors.New("leaves error")
		ts := &mock.TrieStub{
			GetAllLeavesOnChannelCalled: func(hash []byte) (chan data.KeyValueHolder, error) {
				return nil, expectedErr
			},
			GetStorageManagerCalled: defaultStorageManager(),
		}
		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, &mock.MarshalizerMock{}, &factory.AccountCreator{}, core.Normal)
		_, err := adb.RecreateAllTries([]byte("root"), context.Background())
		assert.Equal(t, expectedErr, err)
	})

	t.Run("Recreate error for main trie", func(t *testing.T) {
		expectedErr := errors.New("recreate error")
		ch := make(chan data.KeyValueHolder)
		close(ch)
		ts := &mock.TrieStub{
			GetAllLeavesOnChannelCalled: func(hash []byte) (chan data.KeyValueHolder, error) { return ch, nil },
			RecreateCalled:              func(root []byte) (data.Trie, error) { return nil, expectedErr },
			GetStorageManagerCalled:     defaultStorageManager(),
		}
		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, &mock.MarshalizerMock{}, &factory.AccountCreator{}, core.Normal)
		_, err := adb.RecreateAllTries([]byte("root"), context.Background())
		assert.Equal(t, expectedErr, err)
	})

	t.Run("Recreate error for data trie propagates", func(t *testing.T) {
		marshalizer := &mock.MarshalizerMock{}
		acc, _ := state.NewUserAccount([]byte("addr1"))
		acc.SetRootHash([]byte("datatrie1"))
		accBytes, _ := marshalizer.Marshal(acc)

		ch := make(chan data.KeyValueHolder, 1)
		ch <- &mock.KeyValueHolderStub{ValueCalled: func() []byte { return accBytes }}
		close(ch)

		expectedErr := errors.New("data trie recreate error")
		mainRecreated := &mock.TrieStub{}
		ts := &mock.TrieStub{
			GetAllLeavesOnChannelCalled: func(hash []byte) (chan data.KeyValueHolder, error) { return ch, nil },
			RecreateCalled: func(root []byte) (data.Trie, error) {
				if string(root) == "datatrie1" {
					return nil, expectedErr
				}
				return mainRecreated, nil
			},
			GetStorageManagerCalled: defaultStorageManager(),
		}
		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &factory.AccountCreator{}, core.Normal)
		_, err := adb.RecreateAllTries([]byte("root"), context.Background())
		assert.Equal(t, expectedErr, err)
	})

	t.Run("unmarshal error skips leaf continues", func(t *testing.T) {
		ch := make(chan data.KeyValueHolder, 1)
		ch <- &mock.KeyValueHolderStub{ValueCalled: func() []byte { return []byte("not-valid-proto") }}
		close(ch)

		mainRecreated := &mock.TrieStub{}
		ts := &mock.TrieStub{
			GetAllLeavesOnChannelCalled: func(hash []byte) (chan data.KeyValueHolder, error) { return ch, nil },
			RecreateCalled:              func(root []byte) (data.Trie, error) { return mainRecreated, nil },
			GetStorageManagerCalled:     defaultStorageManager(),
		}
		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, &mock.MarshalizerMock{}, &factory.AccountCreator{}, core.Normal)
		result, err := adb.RecreateAllTries([]byte("root"), context.Background())
		assert.Nil(t, err)
		assert.Equal(t, 1, len(result))
	})
}

func TestAccountsDB_SnapshotUserAccountDataTrie_ErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("GetAllLeavesOnChannel error does not panic", func(t *testing.T) {
		ts := &mock.TrieStub{
			GetAllLeavesOnChannelCalled: func(hash []byte) (chan data.KeyValueHolder, error) {
				return nil, errors.New("leaves error")
			},
			GetStorageManagerCalled: func() data.StorageManager {
				return &mock.StorageManagerStub{
					TakeSnapshotCalled: func(rootHash []byte) {},
					DatabaseCalled:     func() data.DBWriteCacher { return mock.NewMemDbMock() },
				}
			},
		}
		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, &mock.MarshalizerMock{}, &factory.AccountCreator{}, core.ImportDb)
		assert.NotPanics(t, func() {
			adb.SnapshotState([]byte("root"), context.Background())
		})
	})

	t.Run("unmarshal error skips leaf", func(t *testing.T) {
		ch := make(chan data.KeyValueHolder, 1)
		ch <- &mock.KeyValueHolderStub{ValueCalled: func() []byte { return []byte("invalid-proto") }}
		close(ch)

		checkpointCalls := 0
		snapshotMut := sync.Mutex{}
		ts := &mock.TrieStub{
			GetAllLeavesOnChannelCalled: func(hash []byte) (chan data.KeyValueHolder, error) { return ch, nil },
			GetStorageManagerCalled: func() data.StorageManager {
				return &mock.StorageManagerStub{
					TakeSnapshotCalled: func(rootHash []byte) {},
					SetCheckpointCalled: func(hash []byte) {
						snapshotMut.Lock()
						checkpointCalls++
						snapshotMut.Unlock()
					},
					DatabaseCalled: func() data.DBWriteCacher { return mock.NewMemDbMock() },
				}
			},
		}
		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, &mock.MarshalizerMock{}, &factory.AccountCreator{}, core.ImportDb)
		adb.SnapshotState([]byte("root"), context.Background())
		time.Sleep(200 * time.Millisecond)

		snapshotMut.Lock()
		assert.Equal(t, 0, checkpointCalls)
		snapshotMut.Unlock()
	})

	t.Run("leaf with empty RootHash does not trigger checkpoint", func(t *testing.T) {
		marshalizer := &mock.MarshalizerMock{}
		acc, _ := state.NewUserAccount([]byte("addr"))
		accBytes, _ := marshalizer.Marshal(acc)

		ch := make(chan data.KeyValueHolder, 1)
		ch <- &mock.KeyValueHolderStub{ValueCalled: func() []byte { return accBytes }}
		close(ch)

		checkpointCalls := 0
		snapshotMut := sync.Mutex{}
		ts := &mock.TrieStub{
			GetAllLeavesOnChannelCalled: func(hash []byte) (chan data.KeyValueHolder, error) { return ch, nil },
			GetStorageManagerCalled: func() data.StorageManager {
				return &mock.StorageManagerStub{
					TakeSnapshotCalled: func(rootHash []byte) {},
					SetCheckpointCalled: func(hash []byte) {
						snapshotMut.Lock()
						checkpointCalls++
						snapshotMut.Unlock()
					},
					DatabaseCalled: func() data.DBWriteCacher { return mock.NewMemDbMock() },
				}
			},
		}
		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &factory.AccountCreator{}, core.ImportDb)
		adb.SnapshotState([]byte("root"), context.Background())
		time.Sleep(200 * time.Millisecond)

		snapshotMut.Lock()
		assert.Equal(t, 0, checkpointCalls)
		snapshotMut.Unlock()
	})
}

func TestAccountsDB_RevertToSnapshot_RecreateFails(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	hsh := mock.HasherMock{}
	storageManager, _ := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
	tr, _ := trie.NewTrie(storageManager, marshalizer, hsh, uint(5))
	adb, _ := state.NewAccountsDB(tr, hsh, marshalizer, factory.NewAccountCreator(), core.Normal)

	acc, _ := adb.LoadAccount(make([]byte, 32))
	_ = adb.SaveAccount(acc)
	_, _ = adb.Commit()

	acc2, _ := adb.LoadAccount(bytes.Repeat([]byte{1}, 32))
	_ = adb.SaveAccount(acc2)
	assert.True(t, adb.JournalLen() > 0)

	err := adb.RevertToSnapshot(0)
	assert.Nil(t, err)
}

func TestAccountsDB_Commit_DataTrieErrors(t *testing.T) {
	t.Parallel()

	t.Run("data trie commit error propagates", func(t *testing.T) {
		expectedErr := errors.New("data trie commit error")
		marshalizer := &mock.MarshalizerMock{}
		serializedAccount, _ := marshalizer.Marshal(mock.AccountWrapMock{})

		ts := &mock.TrieStub{
			GetCalled:    func(key []byte) ([]byte, error) { return serializedAccount, nil },
			UpdateCalled: func(key, value []byte) error { return nil },
			RecreateCalled: func(root []byte) (data.Trie, error) {
				return &mock.TrieStub{
					GetCalled:    func(key []byte) ([]byte, error) { return []byte("val"), nil },
					UpdateCalled: func(key, value []byte) error { return nil },
					CommitCalled: func() error { return expectedErr },
				}, nil
			},
			GetStorageManagerCalled: defaultStorageManager(),
		}
		adb := generateAccountDBFromTrie(ts)

		acc, _ := adb.LoadAccount(make([]byte, 32))
		_ = acc.(state.UserAccountHandler).DataTrieTracker().SaveKeyValue([]byte("key"), []byte("val"))
		_ = adb.SaveAccount(acc)

		_, err := adb.Commit()
		assert.Equal(t, expectedErr, err)
	})

	t.Run("main trie commit error propagates", func(t *testing.T) {
		expectedErr := errors.New("main commit error")
		ts := newSimpleTrieStub()
		ts.CommitCalled = func() error { return expectedErr }
		adb := generateAccountDBFromTrie(ts)

		_, err := adb.Commit()
		assert.Equal(t, expectedErr, err)
	})

	t.Run("main trie RootHash error propagates", func(t *testing.T) {
		expectedErr := errors.New("root hash error")
		ts := newSimpleTrieStub()
		ts.CommitCalled = func() error { return nil }
		ts.RootCalled = func() ([]byte, error) { return nil, expectedErr }
		adb := generateAccountDBFromTrie(ts)

		_, err := adb.Commit()
		assert.Equal(t, expectedErr, err)
	})
}

func TestAccountsDB_GetExistingAccount_ErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("nil address returns error", func(t *testing.T) {
		adb := generateAccountDBFromTrie(newSimpleTrieStub())
		_, err := adb.GetExistingAccount(nil)
		assert.True(t, errors.Is(err, common.ErrNilAddress))
	})

	t.Run("getAccount trie error propagates", func(t *testing.T) {
		expectedErr := errors.New("trie error")
		ts := &mock.TrieStub{
			GetCalled:               func(key []byte) ([]byte, error) { return nil, expectedErr },
			GetStorageManagerCalled: defaultStorageManager(),
		}
		adb := generateAccountDBFromTrie(ts)
		_, err := adb.GetExistingAccount(make([]byte, 32))
		assert.Equal(t, expectedErr, err)
	})

	t.Run("account not found returns error", func(t *testing.T) {
		adb := generateAccountDBFromTrie(newSimpleTrieStub())
		_, err := adb.GetExistingAccount(make([]byte, 32))
		assert.Equal(t, common.ErrAccNotFound, err)
	})
}

func TestAccountsDB_LoadAccount_ErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("nil address returns error", func(t *testing.T) {
		adb := generateAccountDBFromTrie(newSimpleTrieStub())
		_, err := adb.LoadAccount(nil)
		assert.True(t, errors.Is(err, common.ErrNilAddress))
	})

	t.Run("factory CreateAccount error propagates", func(t *testing.T) {
		expectedErr := errors.New("factory error")
		ts := newSimpleTrieStub()
		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, &mock.MarshalizerMock{}, &mock.AccountsFactoryStub{
			CreateAccountCalled: func(address []byte) (state.AccountHandler, error) {
				return nil, expectedErr
			},
		}, core.Normal)
		_, err := adb.LoadAccount(make([]byte, 32))
		assert.Equal(t, expectedErr, err)
	})
}

func TestAccountsDB_SaveAccount_ErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("nil account returns error", func(t *testing.T) {
		adb := generateAccountDBFromTrie(newSimpleTrieStub())
		err := adb.SaveAccount(nil)
		assert.True(t, errors.Is(err, common.ErrNilAccountHandler))
	})

	t.Run("getAccount trie error propagates", func(t *testing.T) {
		expectedErr := errors.New("get error")
		ts := &mock.TrieStub{
			GetCalled:               func(key []byte) ([]byte, error) { return nil, expectedErr },
			UpdateCalled:            func(key, value []byte) error { return nil },
			GetStorageManagerCalled: defaultStorageManager(),
		}
		adb := generateAccountDBFromTrie(ts)
		err := adb.SaveAccount(generateAccount())
		assert.Equal(t, expectedErr, err)
	})
}

func TestAccountsDB_SaveAccounts_StopsOnError(t *testing.T) {
	t.Parallel()

	adb := generateAccountDBFromTrie(newSimpleTrieStub())
	err := adb.SaveAccounts(nil)
	assert.True(t, errors.Is(err, common.ErrNilAccountHandler))
}

func TestAccountsDB_RevertToSnapshot_LoopBodyExecuted(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	hsh := mock.HasherMock{}
	storageManager, _ := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
	tr, _ := trie.NewTrie(storageManager, marshalizer, hsh, uint(5))
	adb, _ := state.NewAccountsDB(tr, hsh, marshalizer, factory.NewAccountCreator(), core.Normal)

	addr := make([]byte, 32)

	// Save and commit to put account in trie
	acc, _ := adb.LoadAccount(addr)
	_ = adb.SaveAccount(acc)
	_, _ = adb.Commit()

	// Save a new account (creates journalEntryAccountCreation at index 0)
	addr2 := bytes.Repeat([]byte{1}, 32)
	acc2, _ := adb.LoadAccount(addr2)
	_ = adb.SaveAccount(acc2)

	// Modify existing account addr (creates journalEntryAccount at index 1 with old state)
	acc, _ = adb.LoadAccount(addr)
	acc.IncreaseNonce(1)
	_ = adb.SaveAccount(acc)

	assert.True(t, adb.JournalLen() >= 2)

	// RevertToSnapshot(1) iterates the loop reverting entries[1..last],
	// which includes journalEntryAccount that returns non-nil account from Revert()
	err := adb.RevertToSnapshot(1)
	assert.Nil(t, err)
	assert.Equal(t, 1, adb.JournalLen())
}

func TestAccountsDB_Journalize_NilEntry(t *testing.T) {
	t.Parallel()

	adb := generateAccountDBFromTrie(newSimpleTrieStub())
	initialLen := adb.JournalLen()

	adb.Journalize(nil)
	assert.Equal(t, initialLen, adb.JournalLen())
}

func TestAccountsDB_RemoveCodeAndDataTrie_NonUserAccount(t *testing.T) {
	t.Parallel()

	// Use a PeerAccountHandlerMock which implements AccountHandler but NOT UserAccountHandler
	peerAcc, _ := state.NewPeerAccount(make([]byte, 32))
	adb := generateAccountDBFromTrie(newSimpleTrieStub())
	err := adb.RemoveCodeAndDataTrie(peerAcc)
	assert.Nil(t, err)
}

func TestAccountsDB_RemoveAccount_WithPeerAccountFactory(t *testing.T) {
	t.Parallel()

	address := make([]byte, 32)
	marshalizer := &mock.MarshalizerMock{}
	peerAcc, _ := state.NewPeerAccount(address)
	peerBytes, _ := marshalizer.Marshal(peerAcc)

	ts := &mock.TrieStub{
		GetCalled: func(key []byte) ([]byte, error) {
			if bytes.Equal(key, address) {
				return peerBytes, nil
			}
			return nil, nil
		},
		UpdateCalled:            func(key, value []byte) error { return nil },
		GetStorageManagerCalled: defaultStorageManager(),
	}

	adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &mock.AccountsFactoryStub{
		CreateAccountCalled: func(addr []byte) (state.AccountHandler, error) {
			return state.NewPeerAccount(addr)
		},
	}, core.Normal)

	err := adb.RemoveAccount(address)
	assert.Nil(t, err)
	// removeCodeAndDataTrie returns nil early for non-UserAccountHandler
	assert.Equal(t, 1, adb.JournalLen()) // only the account journal entry, no code entry
}

func TestAccountsDB_Commit_MainTrieRootHashError(t *testing.T) {
	t.Parallel()

	rootErr := errors.New("root hash error after commit")
	ts := newSimpleTrieStub()
	ts.CommitCalled = func() error { return nil }
	ts.RootCalled = func() ([]byte, error) { return nil, rootErr }
	adb := generateAccountDBFromTrie(ts)
	_, err := adb.Commit()
	assert.Equal(t, rootErr, err)
}

func TestAccountsDB_LoadDataTrie_CachedDataTrie(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{}
	hsh := mock.HasherMock{}
	storageManager, _ := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
	tr, _ := trie.NewTrie(storageManager, marshalizer, hsh, uint(5))
	adb, _ := state.NewAccountsDB(tr, hsh, marshalizer, factory.NewAccountCreator(), core.Normal)

	address := make([]byte, 32)

	// Load account, add dirty data to create a data trie
	acc, _ := adb.LoadAccount(address)
	userAcc := acc.(state.UserAccountHandler)
	_ = userAcc.DataTrieTracker().SaveKeyValue([]byte("key"), []byte("val"))
	_ = adb.SaveAccount(acc)

	// Loading the same account again should find the data trie in cache
	acc2, err := adb.LoadAccount(address)
	assert.Nil(t, err)
	assert.False(t, check.IfNil(acc2))
}

func TestAccountsDB_RemoveAccountCode_RemoveCodeError(t *testing.T) {
	t.Parallel()

	address := []byte("address")
	codeHash := []byte("codehash")
	expectedErr := errors.New("get code entry error")

	userAcc, _ := state.NewUserAccount(address)
	userAcc.SetCodeHash(codeHash)
	marshalizer := &mock.MarshalizerMock{}
	accountBytes, _ := marshalizer.Marshal(userAcc)

	ts := &mock.TrieStub{
		GetCalled: func(key []byte) ([]byte, error) {
			if bytes.Equal(key, address) {
				return accountBytes, nil
			}
			if bytes.Equal(key, codeHash) {
				return nil, expectedErr
			}
			return nil, nil
		},
		UpdateCalled:            func(key, value []byte) error { return nil },
		GetStorageManagerCalled: defaultStorageManager(),
	}

	adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &factory.AccountCreator{}, core.Normal)
	err := adb.RemoveAccountCode(address)
	assert.Equal(t, expectedErr, err)
}

func TestAccountsDB_ImportAccount_MarshalError(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerMock{Fail: true}
	ts := newSimpleTrieStub()
	adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &mock.AccountsFactoryStub{
		CreateAccountCalled: func(address []byte) (state.AccountHandler, error) {
			return mock.NewAccountWrapMock(address), nil
		},
	}, core.Normal)

	err := adb.ImportAccount(generateAccount())
	assert.NotNil(t, err)
}

func TestAccountsDB_GetAccount_UnmarshalError(t *testing.T) {
	t.Parallel()

	ts := &mock.TrieStub{
		GetCalled: func(key []byte) ([]byte, error) {
			return []byte("invalid-proto-data"), nil
		},
		GetStorageManagerCalled: defaultStorageManager(),
	}

	adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, &mock.MarshalizerMock{}, &mock.AccountsFactoryStub{
		CreateAccountCalled: func(address []byte) (state.AccountHandler, error) {
			return mock.NewAccountWrapMock(address), nil
		},
	}, core.Normal)

	_, err := adb.GetAccount(make([]byte, 32))
	assert.NotNil(t, err)
}

func TestAccountsDB_GetAccount_FactoryError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("factory error")
	ts := &mock.TrieStub{
		GetCalled: func(key []byte) ([]byte, error) {
			return []byte("some-data"), nil
		},
		GetStorageManagerCalled: defaultStorageManager(),
	}

	adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, &mock.MarshalizerMock{}, &mock.AccountsFactoryStub{
		CreateAccountCalled: func(address []byte) (state.AccountHandler, error) {
			return nil, expectedErr
		},
	}, core.Normal)

	_, err := adb.GetAccount(make([]byte, 32))
	assert.Equal(t, expectedErr, err)
}

func TestAccountsDB_GetNumAccountsCheckpoints_ShortValue(t *testing.T) {
	t.Parallel()

	numCheckpointsKey := []byte("state checkpoint")
	db := mock.NewMemDbMock()
	// Store only 2 bytes — less than the required 4 for uint32
	_ = db.Put(numCheckpointsKey, []byte{1, 2})

	adb, _ := state.NewAccountsDB(
		&mock.TrieStub{
			GetStorageManagerCalled: func() data.StorageManager {
				return &mock.StorageManagerStub{
					DatabaseCalled: func() data.DBWriteCacher {
						return db
					},
				}
			},
		},
		&mock.HasherMock{},
		&mock.MarshalizerMock{},
		&mock.AccountsFactoryStub{},
		core.Normal,
	)

	assert.Equal(t, uint32(0), adb.GetNumCheckpoints())
}

func TestAccountsDB_SaveAccount_CodeEntryErrors(t *testing.T) {
	t.Parallel()

	address := []byte("address")
	newCode := []byte("new code")
	hasher := &mock.HasherMock{}
	newCodeHash := hasher.Compute(string(newCode))

	t.Run("getCodeEntry trie get error propagates through SaveAccount", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("trie get error")

		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				if bytes.Equal(key, newCodeHash) {
					return nil, expectedErr
				}
				return nil, nil
			},
			UpdateCalled:            func(key, value []byte) error { return nil },
			GetStorageManagerCalled: defaultStorageManager(),
		}

		adb, _ := state.NewAccountsDB(ts, hasher, &mock.MarshalizerMock{}, &factory.AccountCreator{}, core.Normal)
		acc, _ := state.NewUserAccount(address)
		acc.SetCode(newCode)

		err := adb.SaveAccount(acc)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("saveCodeEntry trie update error propagates through SaveAccount", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("trie update error")

		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) { return nil, nil },
			UpdateCalled: func(key, value []byte) error {
				if bytes.Equal(key, newCodeHash) {
					return expectedErr
				}
				return nil
			},
			GetStorageManagerCalled: defaultStorageManager(),
		}

		adb, _ := state.NewAccountsDB(ts, hasher, &mock.MarshalizerMock{}, &factory.AccountCreator{}, core.Normal)
		acc, _ := state.NewUserAccount(address)
		acc.SetCode(newCode)

		err := adb.SaveAccount(acc)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("CodeEntry marshal error propagates through SaveAccount", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("marshal error")

		marshalizer := &mock.MarshalizerStub{
			MarshalCalled: func(obj interface{}) ([]byte, error) {
				if _, ok := obj.(*state.CodeEntry); ok {
					return nil, expectedErr
				}
				return (&mock.MarshalizerMock{}).Marshal(obj)
			},
			UnmarshalCalled: func(obj interface{}, buff []byte) error {
				return (&mock.MarshalizerMock{}).Unmarshal(obj, buff)
			},
		}

		ts := &mock.TrieStub{
			GetCalled:               func(key []byte) ([]byte, error) { return nil, nil },
			UpdateCalled:            func(key, value []byte) error { return nil },
			GetStorageManagerCalled: defaultStorageManager(),
		}

		adb, _ := state.NewAccountsDB(ts, hasher, marshalizer, &factory.AccountCreator{}, core.Normal)
		acc, _ := state.NewUserAccount(address)
		acc.SetCode(newCode)

		err := adb.SaveAccount(acc)
		assert.Equal(t, expectedErr, err)
	})
}

func TestAccountsDB_SaveAccount_CodeTypeAssertion(t *testing.T) {
	t.Parallel()

	// UserAccountHandlerStub implements UserAccountHandler but is NOT *userAccount,
	// so saveCode's type assertion to *userAccount fails → ErrWrongTypeAssertion
	address := []byte("address")
	customAcc := &mock.UserAccountHandlerStub{
		AddressBytesCalled: func() []byte { return address },
		GetNonceCalled:     func() uint64 { return 0 },
		HasNewCodeCalled:   func() bool { return true },
		GetCodeHashCalled:  func() []byte { return nil },
		DataTrieTrackerCalled: func() state.DataTrieTracker {
			return &mock.DataTrieTrackerStub{
				DirtyDataCalled: func() map[string][]byte {
					return map[string][]byte{}
				},
			}
		},
	}

	ts := &mock.TrieStub{
		GetCalled:               func(key []byte) ([]byte, error) { return nil, nil },
		UpdateCalled:            func(key, value []byte) error { return nil },
		GetStorageManagerCalled: defaultStorageManager(),
	}

	adb := generateAccountDBFromTrie(ts)
	err := adb.SaveAccount(customAcc)
	assert.Equal(t, common.ErrWrongTypeAssertion, err)
}

func TestAccountsDB_SaveAccount_DataTrieErrors(t *testing.T) {
	t.Parallel()

	address := []byte("address")

	t.Run("dataTrie.Get error stops dirty data flush", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("data trie get error")
		dataTrie := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) { return nil, expectedErr },
		}

		ts := &mock.TrieStub{
			GetCalled:               func(key []byte) ([]byte, error) { return nil, nil },
			RecreateCalled:          func(root []byte) (data.Trie, error) { return dataTrie, nil },
			UpdateCalled:            func(key, value []byte) error { return nil },
			GetStorageManagerCalled: defaultStorageManager(),
		}

		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, &mock.MarshalizerMock{}, &factory.AccountCreator{}, core.Normal)
		acc, _ := state.NewUserAccount(address)
		_ = acc.DataTrieTracker().SaveKeyValue([]byte("key"), []byte("value"))

		err := adb.SaveAccount(acc)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("dataTrie.Update error stops dirty data flush", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("data trie update error")
		dataTrie := &mock.TrieStub{
			GetCalled:    func(key []byte) ([]byte, error) { return []byte("old"), nil },
			UpdateCalled: func(key, value []byte) error { return expectedErr },
		}

		ts := &mock.TrieStub{
			GetCalled:               func(key []byte) ([]byte, error) { return nil, nil },
			RecreateCalled:          func(root []byte) (data.Trie, error) { return dataTrie, nil },
			UpdateCalled:            func(key, value []byte) error { return nil },
			GetStorageManagerCalled: defaultStorageManager(),
		}

		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, &mock.MarshalizerMock{}, &factory.AccountCreator{}, core.Normal)
		acc, _ := state.NewUserAccount(address)
		_ = acc.DataTrieTracker().SaveKeyValue([]byte("key"), []byte("value"))

		err := adb.SaveAccount(acc)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("dataTrie.RootHash error after successful flush", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("root hash error")
		dataTrie := &mock.TrieStub{
			GetCalled:    func(key []byte) ([]byte, error) { return nil, nil },
			UpdateCalled: func(key, value []byte) error { return nil },
			RootCalled:   func() ([]byte, error) { return nil, expectedErr },
		}

		ts := &mock.TrieStub{
			GetCalled:               func(key []byte) ([]byte, error) { return nil, nil },
			RecreateCalled:          func(root []byte) (data.Trie, error) { return dataTrie, nil },
			UpdateCalled:            func(key, value []byte) error { return nil },
			GetStorageManagerCalled: defaultStorageManager(),
		}

		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, &mock.MarshalizerMock{}, &factory.AccountCreator{}, core.Normal)
		acc, _ := state.NewUserAccount(address)
		_ = acc.DataTrieTracker().SaveKeyValue([]byte("key"), []byte("value"))

		err := adb.SaveAccount(acc)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("nil data trie triggers Recreate and propagates its error", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("recreate error")

		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) { return nil, nil },
			RecreateCalled: func(root []byte) (data.Trie, error) {
				if len(root) == 0 {
					return nil, expectedErr
				}
				return &mock.TrieStub{}, nil
			},
			UpdateCalled:            func(key, value []byte) error { return nil },
			GetStorageManagerCalled: defaultStorageManager(),
		}

		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, &mock.MarshalizerMock{}, &factory.AccountCreator{}, core.Normal)
		acc, _ := state.NewUserAccount(address)
		_ = acc.DataTrieTracker().SaveKeyValue([]byte("key"), []byte("value"))

		err := adb.SaveAccount(acc)
		assert.Equal(t, expectedErr, err)
	})
}

func TestAccountsDB_SaveAccount_PeerAccountSkipsCodeAndDataTrie(t *testing.T) {
	t.Parallel()

	// PeerAccount implements AccountHandler but NOT baseAccountHandler,
	// so saveCodeAndDataTrie returns early — only the account-save journal entry is created
	address := []byte("address")
	peerAcc, _ := state.NewPeerAccount(address)

	marshalizer := &mock.MarshalizerMock{}
	accBytes, _ := marshalizer.Marshal(peerAcc)

	ts := &mock.TrieStub{
		GetCalled: func(key []byte) ([]byte, error) {
			if bytes.Equal(key, address) {
				return accBytes, nil
			}
			return nil, nil
		},
		UpdateCalled:            func(key, value []byte) error { return nil },
		GetStorageManagerCalled: defaultStorageManager(),
	}

	adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &mock.AccountsFactoryStub{
		CreateAccountCalled: func(addr []byte) (state.AccountHandler, error) {
			return state.NewPeerAccount(addr)
		},
	}, core.Normal)

	err := adb.SaveAccount(peerAcc)
	assert.Nil(t, err)
	assert.Equal(t, 1, adb.JournalLen())
}

func TestAccountsDB_RemoveAccount_DataTrieErrors(t *testing.T) {
	t.Parallel()

	address := []byte("address")
	rootHash := []byte("roothash")

	marshalizer := &mock.MarshalizerMock{}
	acc, _ := state.NewUserAccount(address)
	acc.SetRootHash(rootHash)
	accBytes, _ := marshalizer.Marshal(acc)

	t.Run("Recreate error propagates through RemoveAccount", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("recreate error")

		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				if bytes.Equal(key, address) {
					return accBytes, nil
				}
				return nil, nil
			},
			RecreateCalled: func(root []byte) (data.Trie, error) {
				if bytes.Equal(root, rootHash) {
					return nil, expectedErr
				}
				return &mock.TrieStub{}, nil
			},
			UpdateCalled:            func(key, value []byte) error { return nil },
			GetStorageManagerCalled: defaultStorageManager(),
		}

		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &factory.AccountCreator{}, core.Normal)
		err := adb.RemoveAccount(address)
		assert.ErrorContains(t, err, "recreate error")
	})

	t.Run("GetAllHashes error propagates through RemoveAccount", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("get all hashes error")

		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				if bytes.Equal(key, address) {
					return accBytes, nil
				}
				return nil, nil
			},
			RecreateCalled: func(root []byte) (data.Trie, error) {
				return &mock.TrieStub{
					GetAllHashesCalled: func() ([][]byte, error) {
						return nil, expectedErr
					},
				}, nil
			},
			UpdateCalled:            func(key, value []byte) error { return nil },
			GetStorageManagerCalled: defaultStorageManager(),
		}

		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &factory.AccountCreator{}, core.Normal)
		err := adb.RemoveAccount(address)
		assert.Equal(t, expectedErr, err)
	})
}

func TestAccountsDB_RemoveCodeAndDataTrie_ErrorPropagation(t *testing.T) {
	t.Parallel()

	t.Run("removeCode error when trie.Get fails for code hash", func(t *testing.T) {
		t.Parallel()
		codeHash := []byte("codehash")
		expectedErr := errors.New("get code entry error")

		acc, _ := state.NewUserAccount([]byte("address"))
		acc.SetCodeHash(codeHash)

		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				if bytes.Equal(key, codeHash) {
					return nil, expectedErr
				}
				return nil, nil
			},
			UpdateCalled:            func(key, value []byte) error { return nil },
			GetStorageManagerCalled: defaultStorageManager(),
		}

		adb := generateAccountDBFromTrie(ts)
		err := adb.RemoveCodeAndDataTrie(acc)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("removeDataTrie error when Recreate fails for root hash", func(t *testing.T) {
		t.Parallel()
		rootHash := []byte("roothash")
		expectedErr := errors.New("recreate data trie error")

		acc, _ := state.NewUserAccount([]byte("address"))
		acc.SetRootHash(rootHash)

		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) { return nil, nil },
			RecreateCalled: func(root []byte) (data.Trie, error) {
				if bytes.Equal(root, rootHash) {
					return nil, expectedErr
				}
				return &mock.TrieStub{}, nil
			},
			UpdateCalled:            func(key, value []byte) error { return nil },
			GetStorageManagerCalled: defaultStorageManager(),
		}

		adb := generateAccountDBFromTrie(ts)
		err := adb.RemoveCodeAndDataTrie(acc)
		assert.ErrorContains(t, err, "recreate data trie error")
	})
}

func TestAccountsDB_RemoveAccountCode_Scenarios(t *testing.T) {
	t.Parallel()

	t.Run("nil address returns error", func(t *testing.T) {
		t.Parallel()

		adb := generateAccountDBFromTrie(&mock.TrieStub{GetStorageManagerCalled: defaultStorageManager()})
		err := adb.RemoveAccountCode(nil)
		assert.True(t, errors.Is(err, common.ErrNilAddress))
	})

	t.Run("account not found returns error", func(t *testing.T) {
		t.Parallel()

		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				return nil, nil
			},
			GetStorageManagerCalled: defaultStorageManager(),
		}
		adb := generateAccountDBFromTrie(ts)
		err := adb.RemoveAccountCode([]byte("nonexistent"))
		assert.True(t, errors.Is(err, common.ErrAccNotFound))
	})

	t.Run("non-UserAccountHandler skips code removal", func(t *testing.T) {
		t.Parallel()

		address := []byte("peer-address")
		peerAcc, _ := state.NewPeerAccount(address)
		marshalizer := &mock.MarshalizerMock{}
		accountBytes, _ := marshalizer.Marshal(peerAcc)

		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				if bytes.Equal(key, address) {
					return accountBytes, nil
				}
				return nil, nil
			},
			UpdateCalled:            func(key, value []byte) error { return nil },
			GetStorageManagerCalled: defaultStorageManager(),
		}

		adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &factory.PeerAccountCreator{}, core.Normal)
		err := adb.RemoveAccountCode(address)
		assert.NoError(t, err)
		assert.True(t, adb.JournalLen() >= 1)
	})
}

func TestAccountsDB_UpdateOldCodeEntry_ErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("saveCodeEntry marshal error when NumReferences > 1", func(t *testing.T) {
		t.Parallel()

		address := []byte("user-address")
		hasher := &mock.HasherMock{}
		oldCodeHash := hasher.Compute("old-code")

		normalMarshalizer := &mock.MarshalizerMock{}
		oldAcc, _ := state.NewUserAccount(address)
		oldAcc.SetCodeHash(oldCodeHash)
		oldAccBytes, _ := normalMarshalizer.Marshal(oldAcc)

		oldCodeEntry := &state.CodeEntry{Code: []byte("old-code"), NumReferences: 3}
		oldCodeEntryBytes, _ := normalMarshalizer.Marshal(oldCodeEntry)

		marshalCounter := 0
		marshalStub := &mock.MarshalizerStub{
			MarshalCalled: func(obj interface{}) ([]byte, error) {
				marshalCounter++
				// First Marshal call is saveCodeEntry inside updateOldCodeEntry
				if marshalCounter == 1 {
					return nil, fmt.Errorf("marshal error on save")
				}
				return normalMarshalizer.Marshal(obj)
			},
			UnmarshalCalled: func(obj interface{}, buff []byte) error {
				return normalMarshalizer.Unmarshal(obj, buff)
			},
		}

		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				if bytes.Equal(key, address) {
					return oldAccBytes, nil
				}
				if bytes.Equal(key, oldCodeHash) {
					return oldCodeEntryBytes, nil
				}
				return nil, nil
			},
			UpdateCalled:            func(key, value []byte) error { return nil },
			GetStorageManagerCalled: defaultStorageManager(),
		}

		adb, _ := state.NewAccountsDB(ts, hasher, marshalStub, &factory.AccountCreator{}, core.Normal)

		newAcc, _ := state.NewUserAccount(address)
		newAcc.SetCode([]byte("new-code"))

		err := adb.SaveAccount(newAcc)
		assert.ErrorContains(t, err, "marshal error on save")
	})

	t.Run("trie Update error when NumReferences <= 1", func(t *testing.T) {
		t.Parallel()

		address := []byte("user-address")
		hasher := &mock.HasherMock{}
		oldCodeHash := hasher.Compute("old-code")

		normalMarshalizer := &mock.MarshalizerMock{}
		oldAcc, _ := state.NewUserAccount(address)
		oldAcc.SetCodeHash(oldCodeHash)
		oldAccBytes, _ := normalMarshalizer.Marshal(oldAcc)

		oldCodeEntry := &state.CodeEntry{Code: []byte("old-code"), NumReferences: 1}
		oldCodeEntryBytes, _ := normalMarshalizer.Marshal(oldCodeEntry)

		expectedErr := fmt.Errorf("trie update delete error")
		ts := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				if bytes.Equal(key, address) {
					return oldAccBytes, nil
				}
				if bytes.Equal(key, oldCodeHash) {
					return oldCodeEntryBytes, nil
				}
				return nil, nil
			},
			UpdateCalled: func(key, value []byte) error {
				// Fail when deleting old code entry (value is nil)
				if bytes.Equal(key, oldCodeHash) && value == nil {
					return expectedErr
				}
				return nil
			},
			GetStorageManagerCalled: defaultStorageManager(),
		}

		adb, _ := state.NewAccountsDB(ts, hasher, normalMarshalizer, &factory.AccountCreator{}, core.Normal)

		newAcc, _ := state.NewUserAccount(address)
		newAcc.SetCode([]byte("new-code"))

		err := adb.SaveAccount(newAcc)
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestAccountsDB_Commit_MainTrieErrors(t *testing.T) {
	t.Parallel()

	t.Run("main trie Commit error", func(t *testing.T) {
		t.Parallel()

		expectedErr := fmt.Errorf("main trie commit error")

		ts := &mock.TrieStub{
			ResetOldHashesCalled: func() [][]byte {
				return nil
			},
			AppendToOldHashesCalled: func(hashes [][]byte) {},
			CommitCalled: func() error {
				return expectedErr
			},
			GetStorageManagerCalled: defaultStorageManager(),
		}

		adb := generateAccountDBFromTrie(ts)

		_, err := adb.Commit()
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestAccountsDB_RevertToSnapshot_SaveAccountToTrieError(t *testing.T) {
	t.Parallel()

	address := make([]byte, 32)
	marshalizer := &mock.MarshalizerMock{}
	hsh := &mock.HasherMock{}

	// Track trie storage so GetCalled returns previously saved values
	store := make(map[string][]byte)
	revertPhase := false

	trieStub := &mock.TrieStub{
		GetCalled: func(key []byte) ([]byte, error) {
			val := store[string(key)]
			return val, nil
		},
		UpdateCalled: func(key, value []byte) error {
			if revertPhase {
				return fmt.Errorf("trie update error in revert")
			}
			store[string(key)] = value
			return nil
		},
		RootCalled: func() ([]byte, error) {
			return make([]byte, 32), nil
		},
		GetStorageManagerCalled: defaultStorageManager(),
	}

	adb, _ := state.NewAccountsDB(trieStub, hsh, marshalizer, factory.NewAccountCreator(), core.Normal)

	// First save creates journal entry with nil old account
	acc, _ := adb.LoadAccount(address)
	_ = adb.SaveAccount(acc)

	snapshot := adb.JournalLen()

	// Second save creates journal entry with the first account as old value
	acc2, _ := adb.LoadAccount(address)
	acc2.IncreaseNonce(1)
	err := adb.SaveAccount(acc2)
	assert.NoError(t, err)

	// Now revert — the journal entry will return the old account (non-nil),
	// and saveAccountToTrie will try trie.Update which fails
	revertPhase = true
	err = adb.RevertToSnapshot(snapshot)
	assert.ErrorContains(t, err, "trie update error in revert")
}

func TestAccountsDB_RemoveCode_ErrorPropagation(t *testing.T) {
	t.Parallel()

	address := []byte("user-address")
	codeHash := []byte("code-hash")
	marshalizer := &mock.MarshalizerMock{}

	userAcc, _ := state.NewUserAccount(address)
	userAcc.SetCodeHash(codeHash)
	accountBytes, _ := marshalizer.Marshal(userAcc)

	expectedErr := fmt.Errorf("trie get error")
	ts := &mock.TrieStub{
		GetCalled: func(key []byte) ([]byte, error) {
			if bytes.Equal(key, address) {
				return accountBytes, nil
			}
			if bytes.Equal(key, codeHash) {
				return nil, expectedErr
			}
			return nil, nil
		},
		UpdateCalled:            func(key, value []byte) error { return nil },
		GetStorageManagerCalled: defaultStorageManager(),
	}

	adb, _ := state.NewAccountsDB(ts, &mock.HasherMock{}, marshalizer, &factory.AccountCreator{}, core.Normal)

	err := adb.RemoveAccountCode(address)
	assert.ErrorIs(t, err, expectedErr)
}
