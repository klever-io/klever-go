package trie

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errInjectedPut = errors.New("injected put failure")

// failOncePutDb fails the failAt-th Put (1-based) and stores every other one.
type failOncePutDb struct {
	*mock.MemDbMock
	puts   int
	failAt int
}

func newFailOncePutDb(failAt int) *failOncePutDb {
	return &failOncePutDb{MemDbMock: mock.NewMemDbMock(), failAt: failAt}
}

func (db *failOncePutDb) Put(key, val []byte) error {
	db.puts++
	if db.puts == db.failAt {
		return errInjectedPut
	}
	return db.MemDbMock.Put(key, val)
}

func requireAllInDb(t *testing.T, db data.DBWriteCacher, hashes data.ModifiedHashes) {
	for h := range hashes {
		key, _ := hex.DecodeString(h)
		_, err := db.Get(key)
		assert.Nil(t, err, "node %s missing from db", h)
	}
}

func allNodeHashes(t *testing.T, n node) data.ModifiedHashes {
	require.Nil(t, n.setHash())
	hashes := make(data.ModifiedHashes)
	require.Nil(t, n.getDirtyHashes(hashes))
	return hashes
}

func TestLeafNode_commitRetryAfterFailedPutWritesNode(t *testing.T) {
	t.Parallel()

	db := newFailOncePutDb(1)
	ln := getLn(getTestMarshalizerAndHasher())
	hashes := allNodeHashes(t, ln)

	err := ln.commit(false, 0, 5, db, db)
	assert.Equal(t, errInjectedPut, err)
	assert.True(t, ln.isDirty())

	err = ln.commit(false, 0, 5, db, db)
	assert.Nil(t, err)
	assert.False(t, ln.isDirty())
	assert.Equal(t, 2, db.puts)
	requireAllInDb(t, db, hashes)
}

func TestBranchNode_commitRetryAfterFailedChildPutWritesEveryNode(t *testing.T) {
	t.Parallel()

	// puts run child 2, child 6, child 13, then the branch: fail child 6
	db := newFailOncePutDb(2)
	bn, _ := getBnAndCollapsedBn(getTestMarshalizerAndHasher())
	hashes := allNodeHashes(t, bn)
	require.Len(t, hashes, 4)

	err := bn.commit(false, 0, 5, db, db)
	assert.Equal(t, errInjectedPut, err)
	assert.True(t, bn.isDirty())
	assert.True(t, bn.children[6].isDirty())

	err = bn.commit(false, 0, 5, db, db)
	assert.Nil(t, err)
	assert.False(t, bn.isDirty())
	requireAllInDb(t, db, hashes)
}

func TestBranchNode_commitRetryAfterFailedOwnPutWritesNode(t *testing.T) {
	t.Parallel()

	db := newFailOncePutDb(4)
	bn, _ := getBnAndCollapsedBn(getTestMarshalizerAndHasher())
	hashes := allNodeHashes(t, bn)

	err := bn.commit(false, 0, 5, db, db)
	assert.Equal(t, errInjectedPut, err)
	assert.True(t, bn.isDirty())

	err = bn.commit(false, 0, 5, db, db)
	assert.Nil(t, err)
	assert.False(t, bn.isDirty())
	assert.Equal(t, 5, db.puts)
	requireAllInDb(t, db, hashes)
}

func TestExtensionNode_commitRetryAfterFailedOwnPutWritesNode(t *testing.T) {
	t.Parallel()

	// puts run the three leaves, the branch child, then the extension
	db := newFailOncePutDb(5)
	en := getCanonicalEn()
	hashes := allNodeHashes(t, en)
	require.Len(t, hashes, 5)

	err := en.commit(false, 0, 5, db, db)
	assert.Equal(t, errInjectedPut, err)
	assert.True(t, en.isDirty())

	err = en.commit(false, 0, 5, db, db)
	assert.Nil(t, err)
	assert.False(t, en.isDirty())
	assert.Equal(t, 6, db.puts)
	requireAllInDb(t, db, hashes)
}

func TestExtensionNode_commitRetryAfterFailedChildPutWritesEveryNode(t *testing.T) {
	t.Parallel()

	db := newFailOncePutDb(3)
	en := getCanonicalEn()
	hashes := allNodeHashes(t, en)

	err := en.commit(false, 0, 5, db, db)
	assert.Equal(t, errInjectedPut, err)
	assert.True(t, en.isDirty())

	err = en.commit(false, 0, 5, db, db)
	assert.Nil(t, err)
	assert.False(t, en.isDirty())
	requireAllInDb(t, db, hashes)
}

func TestPatriciaMerkleTrie_CommitRetryAfterFailedPutPersistsEveryNode(t *testing.T) {
	t.Parallel()

	for failAt := 1; failAt <= 4; failAt++ {
		db := newFailOncePutDb(failAt)
		tr := initTrie()
		tr.trieStorage.(*trieStorageManager).db = db
		hashes := allNodeHashes(t, tr.root)
		require.GreaterOrEqual(t, len(hashes), failAt)

		err := tr.Commit()
		assert.Equal(t, errInjectedPut, err, "failAt %d", failAt)
		assert.True(t, tr.root.isDirty(), "failAt %d", failAt)

		err = tr.Commit()
		assert.Nil(t, err, "failAt %d", failAt)
		assert.False(t, tr.root.isDirty(), "failAt %d", failAt)
		requireAllInDb(t, db, hashes)
	}
}
