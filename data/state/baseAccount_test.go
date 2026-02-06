package state_test

import (
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestBaseAccount_AddressContainer(t *testing.T) {
	t.Parallel()

	address := make([]byte, 32)

	ba := state.NewEmptyBaseAccount(address, nil)
	assert.Equal(t, address, ba.AddressBytes())
}

func TestBaseAccount_DataTrieTracker(t *testing.T) {
	t.Parallel()

	tracker := &mock.DataTrieTrackerStub{}

	ba := state.NewEmptyBaseAccount(nil, tracker)
	assert.Equal(t, tracker, ba.DataTrieTracker())
}

func TestBaseAccount_DataTrie(t *testing.T) {
	t.Parallel()

	tr := &mock.TrieStub{}
	setCalled := false
	getCalled := false

	tracker := &mock.DataTrieTrackerStub{
		SetDataTrieCalled: func(tr data.Trie) {
			setCalled = true
		},
		DataTrieCalled: func() data.Trie {
			getCalled = true
			return tr
		},
	}

	ba := state.NewEmptyBaseAccount(nil, tracker)
	ba.SetDataTrie(tr)

	assert.Equal(t, tr, ba.DataTrie())
	assert.True(t, setCalled)
	assert.True(t, getCalled)
}

func TestBaseAccount_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	ba := state.NewEmptyBaseAccount(nil, nil)
	assert.False(t, check.IfNil(ba))
	ba = nil
	assert.True(t, check.IfNil(ba))
}

func TestBaseAccount_SetCodeAndHasNewCode(t *testing.T) {
	t.Parallel()

	ba := state.NewEmptyBaseAccount(nil, nil)
	assert.False(t, ba.HasNewCode())

	ba.SetCode([]byte("new-code"))
	assert.True(t, ba.HasNewCode())
}

func TestBaseAccount_SaveKeyValue_NilTrackerShouldErr(t *testing.T) {
	t.Parallel()

	ba := state.NewEmptyBaseAccount(nil, nil)
	err := ba.SaveKeyValue([]byte("key"), []byte("value"))
	assert.Equal(t, state.ErrNilTrackableDataTrie, err)
}

func TestBaseAccount_SaveKeyValue_WithTrackerShouldCallThrough(t *testing.T) {
	t.Parallel()

	saveKeyValueCalled := false
	tracker := &mock.DataTrieTrackerStub{
		SaveKeyValueCalled: func(key []byte, value []byte) error {
			saveKeyValueCalled = true
			return nil
		},
	}

	ba := state.NewEmptyBaseAccount(nil, tracker)
	err := ba.SaveKeyValue([]byte("key"), []byte("value"))
	assert.Nil(t, err)
	assert.True(t, saveKeyValueCalled)
}

func TestBaseAccount_RetrieveValue_NilTrackerShouldErr(t *testing.T) {
	t.Parallel()

	ba := state.NewEmptyBaseAccount(nil, nil)
	val, err := ba.RetrieveValue([]byte("key"))
	assert.Nil(t, val)
	assert.Equal(t, state.ErrNilTrackableDataTrie, err)
}

func TestBaseAccount_RetrieveValue_WithTrackerShouldCallThrough(t *testing.T) {
	t.Parallel()

	expectedValue := []byte("retrieved-value")
	retrieveValueCalled := false
	tracker := &mock.DataTrieTrackerStub{
		RetrieveValueCalled: func(key []byte) ([]byte, error) {
			retrieveValueCalled = true
			return expectedValue, nil
		},
	}

	ba := state.NewEmptyBaseAccount(nil, tracker)
	val, err := ba.RetrieveValue([]byte("key"))
	assert.Nil(t, err)
	assert.Equal(t, expectedValue, val)
	assert.True(t, retrieveValueCalled)
}

func TestBaseAccount_AccountDataHandler(t *testing.T) {
	t.Parallel()

	tracker := &mock.DataTrieTrackerStub{}
	ba := state.NewEmptyBaseAccount(nil, tracker)
	assert.Equal(t, tracker, ba.AccountDataHandler())
}

