package state_test

import (
	"sync"
	"testing"

	"github.com/klever-io/klever-go/data/state"
	"github.com/stretchr/testify/assert"
)

func TestSnapshotStatistics_NewSnapshotStatistics(t *testing.T) {
	t.Parallel()

	ss := state.NewTestSnapshotStatistics(2)
	assert.NotNil(t, ss)
	assert.Equal(t, uint64(0), ss.GetNumNodes())
	assert.Equal(t, uint64(0), ss.GetTrieSize())
	assert.Equal(t, uint64(0), ss.GetNumDataTries())
}

func TestSnapshotStatistics_AddSize(t *testing.T) {
	t.Parallel()

	ss := state.NewTestSnapshotStatistics(1)

	ss.AddSize(100)
	assert.Equal(t, uint64(1), ss.GetNumNodes())
	assert.Equal(t, uint64(100), ss.GetTrieSize())

	ss.AddSize(50)
	assert.Equal(t, uint64(2), ss.GetNumNodes())
	assert.Equal(t, uint64(150), ss.GetTrieSize())
}

func TestSnapshotStatistics_AddSizeConcurrent(t *testing.T) {
	t.Parallel()

	ss := state.NewTestSnapshotStatistics(100)
	iterations := 100

	var wg sync.WaitGroup
	wg.Add(iterations)

	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			ss.AddSize(1)
		}()
	}

	wg.Wait()

	assert.Equal(t, uint64(iterations), ss.GetNumNodes())
	assert.Equal(t, uint64(iterations), ss.GetTrieSize())
}

func TestSnapshotStatistics_NewDataTrie(t *testing.T) {
	t.Parallel()

	ss := state.NewTestSnapshotStatistics(1)

	ss.NewDataTrie()
	assert.Equal(t, uint64(1), ss.GetNumDataTries())

	ss.NewDataTrie()
	assert.Equal(t, uint64(2), ss.GetNumDataTries())
}

func TestSnapshotStatistics_NewDataTrieConcurrent(t *testing.T) {
	t.Parallel()

	ss := state.NewTestSnapshotStatistics(50)
	iterations := 50

	var wg sync.WaitGroup
	wg.Add(iterations)

	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			ss.NewDataTrie()
		}()
	}

	wg.Wait()

	assert.Equal(t, uint64(iterations), ss.GetNumDataTries())
}

func TestSnapshotStatistics_NewSnapshotStarted(t *testing.T) {
	t.Parallel()

	ss := state.NewTestSnapshotStatistics(1)

	done := make(chan bool)
	go func() {
		ss.WaitForSnapshotsToFinish()
		done <- true
	}()

	ss.SnapshotFinished()
	ss.NewSnapshotStarted()
	ss.SnapshotFinished()

	<-done
}

func TestSnapshotStatistics_WaitForSnapshotsToFinish(t *testing.T) {
	t.Parallel()

	ss := state.NewTestSnapshotStatistics(3)

	done := make(chan bool)
	go func() {
		ss.WaitForSnapshotsToFinish()
		done <- true
	}()

	ss.SnapshotFinished()
	ss.SnapshotFinished()
	ss.SnapshotFinished()

	<-done
}
