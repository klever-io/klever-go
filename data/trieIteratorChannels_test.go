package data

import (
	"errors"
	"testing"
	"time"

	"github.com/klever-io/klever-go/core/keyValStorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLeaves(numLeaves int) []KeyValueHolder {
	leaves := make([]KeyValueHolder, 0, numLeaves)
	for i := 0; i < numLeaves; i++ {
		leaves = append(leaves, keyValStorage.NewKeyValStorage([]byte{byte(i)}, []byte("value")))
	}

	return leaves
}

func TestTrieIteratorChannels_ForEachDeliversEveryLeafOfACompleteWalk(t *testing.T) {
	t.Parallel()

	channels := NewCompletedTrieIteratorChannels(testLeaves(3)...)

	visited := 0
	err := channels.ForEach(func(_ KeyValueHolder) error {
		visited++
		return nil
	})

	require.Nil(t, err)
	assert.Equal(t, 3, visited)
}

func TestTrieIteratorChannels_ForEachReportsATruncatedWalk(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("trie read failed")
	channels := NewFailedTrieIteratorChannels(expectedErr, testLeaves(2)...)

	visited := 0
	err := channels.ForEach(func(_ KeyValueHolder) error {
		visited++
		return nil
	})

	require.ErrorIs(t, err, expectedErr)
	assert.Equal(t, 2, visited)
}

func TestTrieIteratorChannels_ForEachSkipsTheLeavesTheCallbackAccepts(t *testing.T) {
	t.Parallel()

	channels := NewCompletedTrieIteratorChannels(testLeaves(3)...)

	kept := 0
	err := channels.ForEach(func(leaf KeyValueHolder) error {
		if leaf.Key()[0] == 0 {
			return nil
		}
		kept++

		return nil
	})

	require.Nil(t, err)
	assert.Equal(t, 2, kept)
}

func TestTrieIteratorChannels_ForEachReleasesTheProducerWhenTheCallbackFails(t *testing.T) {
	t.Parallel()

	channels := NewTrieIteratorChannels(1)
	producerDone := make(chan struct{})

	go func() {
		for _, leaf := range testLeaves(10) {
			channels.LeavesChan <- leaf
		}
		channels.Close(nil)
		close(producerDone)
	}()

	expectedErr := errors.New("consumer gave up")
	err := channels.ForEach(func(_ KeyValueHolder) error {
		return expectedErr
	})
	require.ErrorIs(t, err, expectedErr)

	select {
	case <-producerDone:
	case <-time.After(time.Second):
		require.Fail(t, "producer was left blocked on a full leaves channel")
	}
}

func TestTrieIteratorChannels_ForEachOnNilChannelsReportsAnError(t *testing.T) {
	t.Parallel()

	var channels *TrieIteratorChannels

	err := channels.ForEach(func(_ KeyValueHolder) error {
		return nil
	})

	require.ErrorIs(t, err, ErrNilTrieIteratorChannels)
}

func TestTrieIteratorChannels_ErrReturnsTheSameValueOnEveryCall(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("trie read failed")
	channels := NewFailedTrieIteratorChannels(expectedErr)

	require.ErrorIs(t, channels.Err(), expectedErr)
	require.ErrorIs(t, channels.Err(), expectedErr)
	require.ErrorIs(t, channels.Err(), expectedErr)
}

func TestTrieIteratorChannels_ErrOnNilChannelsIsNotReportedAsACompleteWalk(t *testing.T) {
	t.Parallel()

	var channels *TrieIteratorChannels

	require.ErrorIs(t, channels.Err(), ErrNilTrieIteratorChannels)
}
