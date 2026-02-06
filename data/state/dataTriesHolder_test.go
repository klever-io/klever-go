package state_test

import (
	"strconv"
	"sync"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestNewDataTriesHolder(t *testing.T) {
	t.Parallel()

	dth := state.NewDataTriesHolder()
	assert.False(t, check.IfNil(dth))
}

func TestDataTriesHolder_PutAndGet(t *testing.T) {
	t.Parallel()

	tr1 := &mock.TrieStub{}

	dth := state.NewDataTriesHolder()
	dth.Put([]byte("trie1"), tr1)
	tr := dth.Get([]byte("trie1"))

	assert.True(t, tr == tr1)
}

func TestDataTriesHolder_GetAll(t *testing.T) {
	t.Parallel()

	tr1 := &mock.TrieStub{}
	tr2 := &mock.TrieStub{}
	tr3 := &mock.TrieStub{}

	dth := state.NewDataTriesHolder()
	dth.Put([]byte("trie1"), tr1)
	dth.Put([]byte("trie2"), tr2)
	dth.Put([]byte("trie3"), tr3)
	tries := dth.GetAll()

	assert.Equal(t, 3, len(tries))
}

func TestDataTriesHolder_Reset(t *testing.T) {
	t.Parallel()

	tr1 := &mock.TrieStub{}

	dth := state.NewDataTriesHolder()
	dth.Put([]byte("trie1"), tr1)
	dth.Reset()

	tr := dth.Get([]byte("trie1"))
	assert.Nil(t, tr)
}

func TestDataTriesHolder_Concurrency(t *testing.T) {
	t.Parallel()

	dth := state.NewDataTriesHolder()
	numTries := 50

	wg := sync.WaitGroup{}
	wg.Add(numTries)

	for i := range numTries {
		go func() {
			dth.Put([]byte(strconv.Itoa(i)), &mock.TrieStub{})
			wg.Done()
		}()
	}

	wg.Wait()

	tries := dth.GetAll()
	assert.Equal(t, numTries, len(tries))
}

func TestDataTriesHolder_Replace(t *testing.T) {
	t.Parallel()

	tr1 := &mock.TrieStub{}
	tr2 := &mock.TrieStub{}

	dth := state.NewDataTriesHolder()
	dth.Put([]byte("trie1"), tr1)
	assert.True(t, dth.Get([]byte("trie1")) == tr1)

	dth.Replace([]byte("trie1"), tr2)
	assert.True(t, dth.Get([]byte("trie1")) == tr2)
}

func TestDataTriesHolder_GetAllTries(t *testing.T) {
	t.Parallel()

	tr1 := &mock.TrieStub{}
	tr2 := &mock.TrieStub{}

	dth := state.NewDataTriesHolder()
	dth.Put([]byte("key1"), tr1)
	dth.Put([]byte("key2"), tr2)

	triesMap := dth.GetAllTries()
	assert.Equal(t, 2, len(triesMap))
	assert.True(t, triesMap["key1"] == tr1)
	assert.True(t, triesMap["key2"] == tr2)
}
