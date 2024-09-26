package peer

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewListIndexUpdater(t *testing.T) {
	t.Parallel()

	t.Run("should create a new ListIndexUpdater", func(t *testing.T) {
		updateFunc := func(pubKey string, list string, index uint32) error {
			return nil
		}

		liu := &ListIndexUpdater{
			updateListAndIndex: updateFunc,
		}

		assert.NotNil(t, liu)
		assert.NotNil(t, liu.updateListAndIndex)
	})
}

func TestListIndexUpdater_UpdateListAndIndex(t *testing.T) {
	t.Parallel()

	t.Run("should call updateListAndIndex function", func(t *testing.T) {
		wasCalled := false
		updateFunc := func(pubKey string, list string, index uint32) error {
			wasCalled = true
			return nil
		}

		liu := &ListIndexUpdater{
			updateListAndIndex: updateFunc,
		}

		err := liu.UpdateListAndIndex("pubKey1", "eligible", 1)
		assert.Nil(t, err)
		assert.True(t, wasCalled)
	})

	t.Run("should return error when updateListAndIndex function fails", func(t *testing.T) {
		expectedErr := errors.New("update failed")
		updateFunc := func(pubKey string, list string, index uint32) error {
			return expectedErr
		}

		liu := &ListIndexUpdater{
			updateListAndIndex: updateFunc,
		}

		err := liu.UpdateListAndIndex("pubKey1", "eligible", 1)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("should pass correct parameters to updateListAndIndex function", func(t *testing.T) {
		var capturedPubKey, capturedList string
		var capturedIndex uint32

		updateFunc := func(pubKey string, list string, index uint32) error {
			capturedPubKey = pubKey
			capturedList = list
			capturedIndex = index
			return nil
		}

		liu := &ListIndexUpdater{
			updateListAndIndex: updateFunc,
		}

		expectedPubKey := "pubKey1"
		expectedList := "eligible"
		expectedIndex := uint32(5)

		err := liu.UpdateListAndIndex(expectedPubKey, expectedList, expectedIndex)
		assert.Nil(t, err)
		assert.Equal(t, expectedPubKey, capturedPubKey)
		assert.Equal(t, expectedList, capturedList)
		assert.Equal(t, expectedIndex, capturedIndex)
	})
}

func TestListIndexUpdater_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	t.Run("should return false for non-nil ListIndexUpdater", func(t *testing.T) {
		liu := &ListIndexUpdater{
			updateListAndIndex: func(pubKey string, list string, index uint32) error {
				return nil
			},
		}

		assert.False(t, liu.IsInterfaceNil())
	})

	t.Run("should return true for nil ListIndexUpdater", func(t *testing.T) {
		var liu *ListIndexUpdater
		assert.True(t, liu.IsInterfaceNil())
	})
}
