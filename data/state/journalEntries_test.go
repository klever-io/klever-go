package state_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestNewJournalEntryAccount_NilAccountShouldErr(t *testing.T) {
	t.Parallel()

	entry, err := state.NewJournalEntryAccount(nil)
	assert.True(t, check.IfNil(entry))
	assert.True(t, errors.Is(err, common.ErrNilAccountHandler))
}

func TestNewJournalEntryAccount_OkParams(t *testing.T) {
	t.Parallel()

	entry, err := state.NewJournalEntryAccount(&mock.AccountWrapMock{})
	assert.Nil(t, err)
	assert.False(t, check.IfNil(entry))
}

func TestJournalEntryAccount_Revert(t *testing.T) {
	t.Parallel()

	expectedAcc := &mock.AccountWrapMock{}
	entry, _ := state.NewJournalEntryAccount(expectedAcc)

	acc, err := entry.Revert()
	assert.Nil(t, err)
	assert.Equal(t, expectedAcc, acc)
}

func TestNewJournalEntryAccountCreation_InvalidAddressShouldErr(t *testing.T) {
	t.Parallel()

	entry, err := state.NewJournalEntryAccountCreation([]byte{}, &mock.TrieStub{})
	assert.True(t, check.IfNil(entry))
	assert.Equal(t, common.ErrInvalidAddressLength, err)
}

func TestNewJournalEntryAccountCreation_NilUpdaterShouldErr(t *testing.T) {
	t.Parallel()

	entry, err := state.NewJournalEntryAccountCreation([]byte("address"), nil)
	assert.True(t, check.IfNil(entry))
	assert.Equal(t, common.ErrNilUpdater, err)
}

func TestNewJournalEntryAccountCreation_OkParams(t *testing.T) {
	t.Parallel()

	entry, err := state.NewJournalEntryAccountCreation([]byte("address"), &mock.TrieStub{})
	assert.Nil(t, err)
	assert.False(t, check.IfNil(entry))
}

func TestJournalEntryAccountCreation_RevertErr(t *testing.T) {
	t.Parallel()

	updateErr := errors.New("update error")
	address := []byte("address")
	ts := &mock.TrieStub{
		UpdateCalled: func(key, value []byte) error {
			return updateErr
		},
	}
	entry, _ := state.NewJournalEntryAccountCreation(address, ts)

	acc, err := entry.Revert()
	assert.Equal(t, updateErr, err)
	assert.Nil(t, acc)
}

func TestJournalEntryAccountCreation_RevertUpdatesTheTrie(t *testing.T) {
	t.Parallel()

	updateCalled := false
	address := []byte("address")
	ts := &mock.TrieStub{
		UpdateCalled: func(key, value []byte) error {
			assert.Equal(t, address, key)
			assert.Nil(t, value)
			updateCalled = true
			return nil
		},
	}
	entry, _ := state.NewJournalEntryAccountCreation(address, ts)

	acc, err := entry.Revert()
	assert.Nil(t, err)
	assert.Nil(t, acc)
	assert.True(t, updateCalled)
}

func TestNewJournalEntryAccountDataTrieUpdates_NilAccountShouldErr(t *testing.T) {
	t.Parallel()

	trieUpdates := make(map[string][]byte)
	trieUpdates["a"] = []byte("b")
	entry, err := state.NewJournalEntryAccountDataTrieUpdates(trieUpdates, nil)

	assert.True(t, check.IfNil(entry))
	assert.True(t, errors.Is(err, common.ErrNilAccountHandler))
}

func TestNewJournalEntryAccountDataTrieUpdates_EmptyTrieUpdatesShouldErr(t *testing.T) {
	t.Parallel()

	trieUpdates := make(map[string][]byte)
	accnt, _ := state.NewUserAccount(make([]byte, 32))
	entry, err := state.NewJournalEntryAccountDataTrieUpdates(trieUpdates, accnt)

	assert.True(t, check.IfNil(entry))
	assert.Equal(t, common.ErrNilOrEmptyDataTrieUpdates, err)
}

func TestNewJournalEntryAccountDataTrieUpdates_OkValsShouldWork(t *testing.T) {
	t.Parallel()

	trieUpdates := make(map[string][]byte)
	trieUpdates["a"] = []byte("b")
	accnt, _ := state.NewUserAccount(make([]byte, 32))
	entry, err := state.NewJournalEntryAccountDataTrieUpdates(trieUpdates, accnt)

	assert.Nil(t, err)
	assert.False(t, check.IfNil(entry))
}

func TestJournalEntryDataTrieUpdates_RevertFailsWhenUpdateFails(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("error")

	trieUpdates := make(map[string][]byte)
	trieUpdates["a"] = []byte("b")
	accnt := mock.NewAccountWrapMock(nil)

	trie := &mock.TrieStub{
		UpdateCalled: func(key, value []byte) error {
			return expectedErr
		},
	}

	accnt.SetDataTrie(trie)
	//accnt, _ := state.NewAccount(mock.NewAddressMock(), &mock.AccountTrackerStub{})
	entry, _ := state.NewJournalEntryAccountDataTrieUpdates(trieUpdates, accnt)

	acc, err := entry.Revert()
	assert.Nil(t, acc)
	assert.Equal(t, expectedErr, err)
}

func TestJournalEntryDataTrieUpdates_RevertFailsWhenAccountRootFails(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("error")

	trieUpdates := make(map[string][]byte)
	trieUpdates["a"] = []byte("b")
	accnt := mock.NewAccountWrapMock(nil)

	trie := &mock.TrieStub{
		UpdateCalled: func(key, value []byte) error {
			return nil
		},
		RootCalled: func() ([]byte, error) {
			return nil, expectedErr
		},
	}

	accnt.SetDataTrie(trie)
	entry, _ := state.NewJournalEntryAccountDataTrieUpdates(trieUpdates, accnt)

	acc, err := entry.Revert()
	assert.Nil(t, acc)
	assert.Equal(t, expectedErr, err)
}

func TestJournalEntryDataTrieUpdates_RevertShouldWork(t *testing.T) {
	t.Parallel()

	updateWasCalled := false
	rootWasCalled := false

	trieUpdates := make(map[string][]byte)
	trieUpdates["a"] = []byte("b")
	accnt := mock.NewAccountWrapMock(nil)

	trie := &mock.TrieStub{
		UpdateCalled: func(key, value []byte) error {
			updateWasCalled = true
			return nil
		},
		RootCalled: func() ([]byte, error) {
			rootWasCalled = true
			return []byte{}, nil
		},
	}

	accnt.SetDataTrie(trie)
	entry, _ := state.NewJournalEntryAccountDataTrieUpdates(trieUpdates, accnt)

	acc, err := entry.Revert()
	assert.NotNil(t, acc)
	assert.Nil(t, err)
	assert.True(t, updateWasCalled)
	assert.True(t, rootWasCalled)
}

func TestJournalEntryAccountDataTrieRemove_Revert(t *testing.T) {
	t.Parallel()

	rootHash := []byte("root_hash")
	obsoleteDataTrieHashes := make(map[string][][]byte)
	obsoleteDataTrieHashes[string(rootHash)] = [][]byte{[]byte("hash1"), []byte("hash2")}
	obsoleteDataTrieHashes["other_root"] = [][]byte{[]byte("hash3")}

	entry, err := state.NewJournalEntryAccountDataTrieRemove(rootHash, obsoleteDataTrieHashes)
	assert.Nil(t, err)
	assert.False(t, check.IfNil(entry))

	acc, err := entry.Revert()
	assert.Nil(t, err)
	assert.Nil(t, acc)

	_, exists := obsoleteDataTrieHashes[string(rootHash)]
	assert.False(t, exists, "rootHash should be deleted from obsoleteDataTrieHashes")

	_, otherExists := obsoleteDataTrieHashes["other_root"]
	assert.True(t, otherExists, "other entries should remain in map")
}

func TestJournalEntryCode_RevertSameHash(t *testing.T) {
	t.Parallel()

	codeHash := []byte("same_hash")
	oldCodeEntry := &state.CodeEntry{
		Code:          []byte("code"),
		NumReferences: 1,
	}

	entry, err := state.NewJournalEntryCode(oldCodeEntry, codeHash, codeHash, &mock.TrieStub{}, &mock.MarshalizerMock{})
	assert.Nil(t, err)

	acc, err := entry.Revert()
	assert.Nil(t, err)
	assert.Nil(t, acc)
}

func TestJournalEntryCode_RevertOldCodeEntryError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("trie update error")
	oldCodeEntry := &state.CodeEntry{
		Code:          []byte("old_code"),
		NumReferences: 1,
	}
	oldCodeHash := []byte("old_hash")
	newCodeHash := []byte("new_hash")

	trie := &mock.TrieStub{
		UpdateCalled: func(key, value []byte) error {
			return expectedErr
		},
	}

	entry, _ := state.NewJournalEntryCode(oldCodeEntry, oldCodeHash, newCodeHash, trie, &mock.MarshalizerMock{})

	acc, err := entry.Revert()
	assert.Nil(t, acc)
	assert.Equal(t, expectedErr, err)
}

func TestJournalEntryCode_RevertNewCodeEntryError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("get error")
	oldCodeEntry := &state.CodeEntry{
		Code:          []byte("old_code"),
		NumReferences: 1,
	}
	oldCodeHash := []byte("old_hash")
	newCodeHash := []byte("new_hash")

	updateCalled := false
	trie := &mock.TrieStub{
		UpdateCalled: func(key, value []byte) error {
			updateCalled = true
			return nil
		},
		GetCalled: func(key []byte) ([]byte, error) {
			return nil, expectedErr
		},
	}

	entry, _ := state.NewJournalEntryCode(oldCodeEntry, oldCodeHash, newCodeHash, trie, &mock.MarshalizerMock{})

	acc, err := entry.Revert()
	assert.Nil(t, acc)
	assert.Equal(t, expectedErr, err)
	assert.True(t, updateCalled, "should have saved old code entry before encountering error")
}

func TestJournalEntryCode_RevertSuccess(t *testing.T) {
	t.Parallel()

	t.Run("empty old code hash", func(t *testing.T) {
		t.Parallel()

		newCodeEntry := &state.CodeEntry{
			Code:          []byte("new_code"),
			NumReferences: 2,
		}
		oldCodeHash := []byte{}
		newCodeHash := []byte("new_hash")

		marshalizer := &mock.MarshalizerMock{}
		getCalled := false
		updateCalled := false

		trie := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				getCalled = true
				data, _ := marshalizer.Marshal(newCodeEntry)
				return data, nil
			},
			UpdateCalled: func(key, value []byte) error {
				updateCalled = true
				return nil
			},
		}

		entry, _ := state.NewJournalEntryCode(nil, oldCodeHash, newCodeHash, trie, marshalizer)

		acc, err := entry.Revert()
		assert.Nil(t, err)
		assert.Nil(t, acc)
		assert.True(t, getCalled, "should have called Get to retrieve new code entry")
		assert.True(t, updateCalled, "should have called Update to save decremented new code entry")
	})

	t.Run("new code entry decrements references", func(t *testing.T) {
		t.Parallel()

		oldCodeEntry := &state.CodeEntry{
			Code:          []byte("old_code"),
			NumReferences: 3,
		}
		newCodeEntry := &state.CodeEntry{
			Code:          []byte("new_code"),
			NumReferences: 5,
		}
		oldCodeHash := []byte("old_hash")
		newCodeHash := []byte("new_hash")

		marshalizer := &mock.MarshalizerMock{}
		updateCount := 0
		var lastUpdateValue []byte

		trie := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				data, _ := marshalizer.Marshal(newCodeEntry)
				return data, nil
			},
			UpdateCalled: func(key, value []byte) error {
				updateCount++
				lastUpdateValue = value
				return nil
			},
		}

		entry, _ := state.NewJournalEntryCode(oldCodeEntry, oldCodeHash, newCodeHash, trie, marshalizer)

		acc, err := entry.Revert()
		assert.Nil(t, err)
		assert.Nil(t, acc)
		assert.Equal(t, 2, updateCount, "should update twice: save old entry + save decremented new entry")

		var savedEntry state.CodeEntry
		err = marshalizer.Unmarshal(&savedEntry, lastUpdateValue)
		assert.Nil(t, err)
		assert.Equal(t, uint32(4), savedEntry.NumReferences, "new code entry references should be decremented")
	})

	t.Run("new code entry deletes when references is 1", func(t *testing.T) {
		t.Parallel()

		oldCodeEntry := &state.CodeEntry{
			Code:          []byte("old_code"),
			NumReferences: 1,
		}
		newCodeEntry := &state.CodeEntry{
			Code:          []byte("new_code"),
			NumReferences: 1,
		}
		oldCodeHash := []byte("old_hash")
		newCodeHash := []byte("new_hash")

		marshalizer := &mock.MarshalizerMock{}
		var deletedKey []byte

		trie := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				data, _ := marshalizer.Marshal(newCodeEntry)
				return data, nil
			},
			UpdateCalled: func(key, value []byte) error {
				if value == nil {
					deletedKey = key
				}
				return nil
			},
		}

		entry, _ := state.NewJournalEntryCode(oldCodeEntry, oldCodeHash, newCodeHash, trie, marshalizer)

		acc, err := entry.Revert()
		assert.Nil(t, err)
		assert.Nil(t, acc)
		assert.Equal(t, newCodeHash, deletedKey, "should delete new code entry when references is 1")
	})

	t.Run("nil new code entry", func(t *testing.T) {
		t.Parallel()

		oldCodeEntry := &state.CodeEntry{
			Code:          []byte("old_code"),
			NumReferences: 1,
		}
		oldCodeHash := []byte("old_hash")
		newCodeHash := []byte("new_hash")

		marshalizer := &mock.MarshalizerMock{}

		trie := &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) {
				return nil, nil
			},
			UpdateCalled: func(key, value []byte) error {
				return nil
			},
		}

		entry, _ := state.NewJournalEntryCode(oldCodeEntry, oldCodeHash, newCodeHash, trie, marshalizer)

		acc, err := entry.Revert()
		assert.Nil(t, err)
		assert.Nil(t, acc)
	})
}

func TestRevertOldCodeEntry(t *testing.T) {
	t.Parallel()

	t.Run("empty old code hash returns nil", func(t *testing.T) {
		t.Parallel()

		entry, _ := state.NewJournalEntryCode(nil, []byte{}, []byte("new"), &mock.TrieStub{
			GetCalled: func(key []byte) ([]byte, error) { return nil, nil },
		}, &mock.MarshalizerMock{})

		acc, err := entry.Revert()
		assert.Nil(t, err)
		assert.Nil(t, acc)
	})

	t.Run("trie update error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("trie update error")
		oldCodeEntry := &state.CodeEntry{
			Code:          []byte("old_code"),
			NumReferences: 1,
		}
		oldCodeHash := []byte("old_hash")

		marshalizer := &mock.MarshalizerMock{}
		trie := &mock.TrieStub{
			UpdateCalled: func(key, value []byte) error {
				return expectedErr
			},
		}

		entry, _ := state.NewJournalEntryCode(oldCodeEntry, oldCodeHash, []byte("new_hash"), trie, marshalizer)

		acc, err := entry.Revert()
		assert.Nil(t, acc)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("successful save", func(t *testing.T) {
		t.Parallel()

		oldCodeEntry := &state.CodeEntry{
			Code:          []byte("old_code"),
			NumReferences: 2,
		}
		oldCodeHash := []byte("old_hash")
		newCodeHash := []byte("new_hash")

		marshalizer := &mock.MarshalizerMock{}
		var savedKey []byte
		var savedValue []byte

		trie := &mock.TrieStub{
			UpdateCalled: func(key, value []byte) error {
				savedKey = key
				savedValue = value
				return nil
			},
			GetCalled: func(key []byte) ([]byte, error) {
				return nil, nil
			},
		}

		entry, _ := state.NewJournalEntryCode(oldCodeEntry, oldCodeHash, newCodeHash, trie, marshalizer)

		acc, err := entry.Revert()
		assert.Nil(t, err)
		assert.Nil(t, acc)
		assert.Equal(t, oldCodeHash, savedKey)

		var savedEntry state.CodeEntry
		err = marshalizer.Unmarshal(&savedEntry, savedValue)
		assert.Nil(t, err)
		assert.Equal(t, oldCodeEntry.Code, savedEntry.Code)
		assert.Equal(t, oldCodeEntry.NumReferences, savedEntry.NumReferences)
	})
}

func TestRevertNewCodeEntry(t *testing.T) {
	t.Parallel()

	t.Run("get error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("get error")
		oldCodeEntry := &state.CodeEntry{
			Code:          []byte("old_code"),
			NumReferences: 1,
		}

		trie := &mock.TrieStub{
			UpdateCalled: func(key, value []byte) error {
				return nil
			},
			GetCalled: func(key []byte) ([]byte, error) {
				return nil, expectedErr
			},
		}

		entry, _ := state.NewJournalEntryCode(oldCodeEntry, []byte("old_hash"), []byte("new_hash"), trie, &mock.MarshalizerMock{})

		acc, err := entry.Revert()
		assert.Nil(t, acc)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("delete error when references is 1", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("delete error")
		oldCodeEntry := &state.CodeEntry{
			Code:          []byte("old_code"),
			NumReferences: 1,
		}
		newCodeEntry := &state.CodeEntry{
			Code:          []byte("new_code"),
			NumReferences: 1,
		}

		marshalizer := &mock.MarshalizerMock{}
		updateCallCount := 0

		trie := &mock.TrieStub{
			UpdateCalled: func(key, value []byte) error {
				updateCallCount++
				if value == nil && updateCallCount > 1 {
					return expectedErr
				}
				return nil
			},
			GetCalled: func(key []byte) ([]byte, error) {
				data, _ := marshalizer.Marshal(newCodeEntry)
				return data, nil
			},
		}

		entry, _ := state.NewJournalEntryCode(oldCodeEntry, []byte("old_hash"), []byte("new_hash"), trie, marshalizer)

		acc, err := entry.Revert()
		assert.Nil(t, acc)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("save error when decrementing references", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("save error")
		oldCodeEntry := &state.CodeEntry{
			Code:          []byte("old_code"),
			NumReferences: 1,
		}
		newCodeEntry := &state.CodeEntry{
			Code:          []byte("new_code"),
			NumReferences: 3,
		}

		marshalizer := &mock.MarshalizerMock{}
		updateCallCount := 0

		trie := &mock.TrieStub{
			UpdateCalled: func(key, value []byte) error {
				updateCallCount++
				if updateCallCount > 1 {
					return expectedErr
				}
				return nil
			},
			GetCalled: func(key []byte) ([]byte, error) {
				data, _ := marshalizer.Marshal(newCodeEntry)
				return data, nil
			},
		}

		entry, _ := state.NewJournalEntryCode(oldCodeEntry, []byte("old_hash"), []byte("new_hash"), trie, marshalizer)

		acc, err := entry.Revert()
		assert.Nil(t, acc)
		assert.Equal(t, expectedErr, err)
	})
}

func TestNewJournalEntryAccountDataTrieRemove_Validation(t *testing.T) {
	t.Parallel()

	t.Run("nil map returns error", func(t *testing.T) {
		t.Parallel()
		entry, err := state.NewJournalEntryAccountDataTrieRemove([]byte("root"), nil)
		assert.Nil(t, entry)
		assert.True(t, errors.Is(err, common.ErrNilMapOfHashes))
	})

	t.Run("empty root hash returns error", func(t *testing.T) {
		t.Parallel()
		entry, err := state.NewJournalEntryAccountDataTrieRemove([]byte{}, make(map[string][][]byte))
		assert.Nil(t, entry)
		assert.Equal(t, common.ErrInvalidRootHash, err)
	})
}

func TestNewJournalEntryCode_Validation(t *testing.T) {
	t.Parallel()

	t.Run("nil trie returns error", func(t *testing.T) {
		t.Parallel()
		entry, err := state.NewJournalEntryCode(nil, nil, nil, nil, &mock.MarshalizerMock{})
		assert.Nil(t, entry)
		assert.Equal(t, common.ErrNilUpdater, err)
	})

	t.Run("nil marshalizer returns error", func(t *testing.T) {
		t.Parallel()
		entry, err := state.NewJournalEntryCode(nil, nil, nil, &mock.TrieStub{}, nil)
		assert.Nil(t, entry)
		assert.Equal(t, common.ErrNilMarshalizer, err)
	})
}
