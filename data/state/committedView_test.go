package state_test

import (
	"context"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/stretchr/testify/require"
)

func viewAddress(b byte) []byte {
	address := make([]byte, 32)
	address[0] = b
	return address
}

// finalize prunes as updateStateStorage does once the block after root is final.
func finalize(adb *state.AccountsDB, root []byte) {
	adb.CancelPrune(root, data.NewRoot)
	adb.PruneTrie(root, data.OldRoot)
}

func TestCommittedView_ReadsItsRootAfterLaterCommitsAndPrunes(t *testing.T) {
	t.Parallel()

	adb := newPruningProofAccountsDB(t, mock.NewMemDbMock(), &mock.ProtobufMarshalizerMock{})
	address := viewAddress(1)
	putStorage(t, adb, address, []byte("key"), []byte("first"))
	putBalance(t, adb, address, 1)
	rootA, err := adb.Commit()
	require.NoError(t, err)

	view, err := adb.OpenCommittedAt(rootA)
	require.NoError(t, err)
	defer view.Release()

	putStorage(t, adb, address, []byte("key"), []byte("second"))
	putBalance(t, adb, address, 4)
	_, err = adb.Commit()
	require.NoError(t, err)
	finalize(adb, rootA)

	acc, err := view.GetExistingAccount(address)
	require.NoError(t, err)
	user := acc.(state.UserAccountHandler)
	require.Equal(t, int64(1), user.GetBalance(nil, false))
	value, err := user.DataTrieTracker().RetrieveValue([]byte("key"))
	require.NoError(t, err)
	require.Equal(t, []byte("first"), value)

	root, err := view.RootHash()
	require.NoError(t, err)
	require.Equal(t, rootA, root)
}

func TestCommittedView_ReleaseLetsTheHeldBackDeleteRun(t *testing.T) {
	t.Parallel()

	db := mock.NewMemDbMock()
	adb := newPruningProofAccountsDB(t, db, &mock.ProtobufMarshalizerMock{})
	address := viewAddress(1)
	putBalance(t, adb, address, 1)
	rootA, err := adb.Commit()
	require.NoError(t, err)

	view, err := adb.OpenCommittedAt(rootA)
	require.NoError(t, err)
	putBalance(t, adb, address, 1)
	_, err = adb.Commit()
	require.NoError(t, err)
	finalize(adb, rootA)

	_, err = db.Get(rootA)
	require.NoError(t, err, "rootA deleted while the view pins it")

	view.Release()
	view.Release()
	_, err = db.Get(rootA)
	require.NoError(t, err, "a release deletes nothing outside the accounts lock")

	adb.CancelPrune([]byte("no such root"), data.NewRoot)
	_, err = db.Get(rootA)
	require.Error(t, err, "the next prune did not run the held-back delete")
	_, err = adb.OpenCommittedAt(rootA)
	require.ErrorIs(t, err, state.ErrStateRootUnavailable)
}

func TestCommittedView_MissingAccounts(t *testing.T) {
	t.Parallel()

	accounts := newPruningProofAccountsDB(t, mock.NewMemDbMock(), &mock.ProtobufMarshalizerMock{})
	missing := viewAddress(9)

	accountsView, err := accounts.OpenCommittedAt(nil)
	require.NoError(t, err)
	defer accountsView.Release()

	_, err = accountsView.GetExistingAccount(missing)
	require.ErrorIs(t, err, common.ErrAccNotFound)
	acc, err := accountsView.LoadAccount(missing)
	require.NoError(t, err)
	require.Equal(t, missing, acc.AddressBytes())
}

func TestCommittedView_GetCodeReadsTheViewsRoot(t *testing.T) {
	t.Parallel()

	adb := newPruningProofAccountsDB(t, mock.NewMemDbMock(), &mock.ProtobufMarshalizerMock{})
	address := viewAddress(1)
	acc, err := adb.LoadAccount(address)
	require.NoError(t, err)
	user := acc.(state.UserAccountHandler)
	user.SetCode([]byte("code"))
	require.NoError(t, adb.SaveAccount(user))
	root, err := adb.Commit()
	require.NoError(t, err)

	view, err := adb.OpenCommittedAt(root)
	require.NoError(t, err)
	defer view.Release()

	saved, err := view.GetExistingAccount(address)
	require.NoError(t, err)
	codeHash := saved.(state.UserAccountHandler).GetCodeHash()
	require.Equal(t, []byte("code"), view.GetCode(codeHash))
	require.Nil(t, view.GetCode(nil))
}

func TestCommittedView_GetAllLeavesRefusesAnotherRoot(t *testing.T) {
	t.Parallel()

	adb := newPruningProofAccountsDB(t, mock.NewMemDbMock(), &mock.ProtobufMarshalizerMock{})
	putBalance(t, adb, viewAddress(1), 1)
	root, err := adb.Commit()
	require.NoError(t, err)

	view, err := adb.OpenCommittedAt(root)
	require.NoError(t, err)
	defer view.Release()

	_, err = view.GetAllLeaves([]byte("another root"), context.Background())
	require.ErrorIs(t, err, state.ErrReadOnlyView)
}

func TestCommittedView_GetAllLeavesWalksItsRoot(t *testing.T) {
	t.Parallel()

	adb := newPruningProofAccountsDB(t, mock.NewMemDbMock(), &mock.ProtobufMarshalizerMock{})
	putBalance(t, adb, viewAddress(1), 1)
	putBalance(t, adb, viewAddress(2), 1)
	root, err := adb.Commit()
	require.NoError(t, err)

	view, err := adb.OpenCommittedAt(root)
	require.NoError(t, err)
	defer view.Release()

	leaves, err := view.GetAllLeaves(root, context.Background())
	require.NoError(t, err)
	numLeaves := 0
	require.NoError(t, leaves.ForEach(func(data.KeyValueHolder) error {
		numLeaves++
		return nil
	}))
	require.Equal(t, 2, numLeaves)
}

func TestCommittedView_DoesNotSeeSavedButUncommittedState(t *testing.T) {
	t.Parallel()

	adb := newPruningProofAccountsDB(t, mock.NewMemDbMock(), &mock.ProtobufMarshalizerMock{})
	address := viewAddress(1)
	putStorage(t, adb, address, []byte("key"), []byte("committed"))
	putBalance(t, adb, address, 1)
	_, err := adb.Commit()
	require.NoError(t, err)

	// a block in progress saves, but has not committed yet
	putStorage(t, adb, address, []byte("key"), []byte("in progress"))
	putBalance(t, adb, address, 9)

	view, err := adb.OpenLastCommitted()
	require.NoError(t, err)
	defer view.Release()

	acc, err := view.GetExistingAccount(address)
	require.NoError(t, err)
	user := acc.(state.UserAccountHandler)
	require.Equal(t, int64(1), user.GetBalance(nil, false))
	value, err := user.DataTrieTracker().RetrieveValue([]byte("key"))
	require.NoError(t, err)
	require.Equal(t, []byte("committed"), value)

	live, err := adb.GetExistingAccount(address)
	require.NoError(t, err)
	require.Equal(t, int64(10), live.(state.UserAccountHandler).GetBalance(nil, false), "the live trie does see it")
}

func TestNewLastCommittedAccounts_NilAdapter(t *testing.T) {
	t.Parallel()

	_, err := state.NewLastCommittedAccounts(nil)
	require.ErrorIs(t, err, common.ErrNilAccountsAdapter)
}

func TestLastCommittedAccounts_ReadsTheLastCommittedRootPerCall(t *testing.T) {
	t.Parallel()

	db := mock.NewMemDbMock()
	adb := newPruningProofAccountsDB(t, db, &mock.ProtobufMarshalizerMock{})
	committed, err := state.NewLastCommittedAccounts(adb)
	require.NoError(t, err)

	address := viewAddress(1)
	putStorage(t, adb, address, []byte("key"), []byte("first"))
	putBalance(t, adb, address, 1)
	rootA, err := adb.Commit()
	require.NoError(t, err)

	// saved by a block in progress, not committed
	putBalance(t, adb, address, 2)
	acc, err := committed.GetExistingAccount(address)
	require.NoError(t, err)
	require.Equal(t, int64(1), acc.(state.UserAccountHandler).GetBalance(nil, false))
	root, err := committed.RootHash()
	require.NoError(t, err)
	require.Equal(t, rootA, root)

	rootB, err := adb.Commit()
	require.NoError(t, err)
	acc, err = committed.LoadAccount(address)
	require.NoError(t, err)
	require.Equal(t, int64(3), acc.(state.UserAccountHandler).GetBalance(nil, false), "the next read follows the commit")
	root, err = committed.RootHash()
	require.NoError(t, err)
	require.Equal(t, rootB, root)

	// no pin outlives a call: rootA prunes at once
	finalize(adb, rootA)
	_, err = db.Get(rootA)
	require.Error(t, err)

	missing, err := committed.LoadAccount(viewAddress(2))
	require.NoError(t, err)
	require.Equal(t, viewAddress(2), missing.AddressBytes(), "a missing account loads empty")
	_, err = committed.GetExistingAccount(viewAddress(2))
	require.ErrorIs(t, err, common.ErrAccNotFound)
}

func TestLastCommittedAccounts_RefusesWrites(t *testing.T) {
	t.Parallel()

	adb := newPruningProofAccountsDB(t, mock.NewMemDbMock(), &mock.ProtobufMarshalizerMock{})
	committed, err := state.NewLastCommittedAccounts(adb)
	require.NoError(t, err)
	putBalance(t, adb, viewAddress(1), 1)
	_, err = adb.Commit()
	require.NoError(t, err)

	acc, err := committed.LoadAccount(viewAddress(1))
	require.NoError(t, err)
	require.ErrorIs(t, committed.SaveAccount(acc), state.ErrReadOnlyView)
	require.ErrorIs(t, committed.RemoveAccount(viewAddress(1)), state.ErrReadOnlyView)
	_, err = committed.Commit()
	require.ErrorIs(t, err, state.ErrReadOnlyView)
	require.False(t, committed.IsPruningEnabled())
}

func TestLastCommittedAccounts_WalksARootReadBeforeALaterCommit(t *testing.T) {
	t.Parallel()

	adb := newPruningProofAccountsDB(t, mock.NewMemDbMock(), &mock.ProtobufMarshalizerMock{})
	committed, err := state.NewLastCommittedAccounts(adb)
	require.NoError(t, err)
	putBalance(t, adb, viewAddress(1), 1)
	_, err = adb.Commit()
	require.NoError(t, err)

	root, err := committed.RootHash()
	require.NoError(t, err)
	putBalance(t, adb, viewAddress(2), 1)
	_, err = adb.Commit()
	require.NoError(t, err)

	channels, err := committed.GetAllLeaves(root, context.Background())
	require.NoError(t, err)
	numLeaves := 0
	for range channels.LeavesChan {
		numLeaves++
	}
	require.NoError(t, channels.Err())
	require.Equal(t, 1, numLeaves, "the walk reads the root it was given")
}

// A REST read pins the last root while that block is rolled back, and the canonical block commits
// the same root again. The rollback prune held by the pin must not later delete the live root.
func TestCommittedView_RollbackPruneOfAPinnedRootNeverDeletesItsRecommit(t *testing.T) {
	t.Parallel()

	db := mock.NewMemDbMock()
	adb := newPruningProofAccountsDB(t, db, &mock.ProtobufMarshalizerMock{})
	putBalance(t, adb, viewAddress(1), 1)
	putBalance(t, adb, viewAddress(2), 1)
	r1, err := adb.Commit()
	require.NoError(t, err)
	putBalance(t, adb, viewAddress(1), 5)
	r2, err := adb.Commit()
	require.NoError(t, err)

	view, err := adb.OpenLastCommitted()
	require.NoError(t, err)

	// rollback of r2's block
	require.NoError(t, adb.RecreateTrie(r1))
	adb.CancelPrune(r1, data.OldRoot)
	adb.PruneTrie(r2, data.NewRoot)
	view.Release()

	// the canonical block carries the same transactions
	putBalance(t, adb, viewAddress(1), 5)
	again, err := adb.Commit()
	require.NoError(t, err)
	require.Equal(t, r2, again)
	adb.CancelPrune([]byte("an older root"), data.NewRoot)

	_, err = db.Get(r2)
	require.NoError(t, err, "the live root's node was deleted")
}

// A view holds a root across several commits while a node of that root comes back in a later one.
// The held prune must run before the cancels that keep the recreated node, as it would unpinned.
func TestCommittedView_HeldPruneRunsBeforeTheCancelsThatFollowIt(t *testing.T) {
	t.Parallel()

	adb := newPruningProofAccountsDB(t, mock.NewMemDbMock(), &mock.ProtobufMarshalizerMock{})
	a1, a2, a3 := viewAddress(1), viewAddress(2), viewAddress(3)
	putBalance(t, adb, a1, 1)
	putBalance(t, adb, a2, 1)
	putBalance(t, adb, a3, 1)
	putStorage(t, adb, a1, []byte("k"), []byte("v1"))
	rk, err := adb.Commit()
	require.NoError(t, err)

	view, err := adb.OpenLastCommitted()
	require.NoError(t, err)

	putStorage(t, adb, a1, []byte("k"), []byte("v2"))
	rk1, err := adb.Commit()
	require.NoError(t, err)
	putStorage(t, adb, a1, []byte("k"), []byte("v1")) // a1's nodes from rk come back
	putBalance(t, adb, a2, 1)
	rk2, err := adb.Commit()
	require.NoError(t, err)
	finalize(adb, rk) // held by the view

	putBalance(t, adb, a3, 1)
	_, err = adb.Commit()
	require.NoError(t, err)
	finalize(adb, rk1)

	putBalance(t, adb, a3, 1)
	_, err = adb.Commit()
	require.NoError(t, err)
	adb.CancelPrune(rk2, data.NewRoot)
	view.Release()
	adb.PruneTrie(rk2, data.OldRoot)

	live, err := adb.OpenLastCommitted()
	require.NoError(t, err)
	defer live.Release()
	acc, err := live.GetExistingAccount(a1)
	require.NoError(t, err, "a1 is unreadable in the live root")
	value, err := acc.(state.UserAccountHandler).DataTrieTracker().RetrieveValue([]byte("k"))
	require.NoError(t, err)
	require.Equal(t, []byte("v1"), value)
}

func TestLastCommittedAccounts_GetCodeReadsTheLastCommittedRoot(t *testing.T) {
	t.Parallel()

	adb := newPruningProofAccountsDB(t, mock.NewMemDbMock(), &mock.ProtobufMarshalizerMock{})
	committed, err := state.NewLastCommittedAccounts(adb)
	require.NoError(t, err)
	acc, err := adb.LoadAccount(viewAddress(1))
	require.NoError(t, err)
	user := acc.(state.UserAccountHandler)
	user.SetCode([]byte("code"))
	require.NoError(t, adb.SaveAccount(user))
	_, err = adb.Commit()
	require.NoError(t, err)

	saved, err := committed.GetExistingAccount(viewAddress(1))
	require.NoError(t, err)
	require.Equal(t, []byte("code"), committed.GetCode(saved.(state.UserAccountHandler).GetCodeHash()))
}

// After a rollback, the last committed root is the one rolled back to: the trie cached for the
// abandoned root must not answer for it.
func TestCommittedView_LastCommittedFollowsARollback(t *testing.T) {
	t.Parallel()

	adb := newPruningProofAccountsDB(t, mock.NewMemDbMock(), &mock.ProtobufMarshalizerMock{})
	putBalance(t, adb, viewAddress(1), 1)
	rootA, err := adb.Commit()
	require.NoError(t, err)
	putBalance(t, adb, viewAddress(1), 2)
	_, err = adb.Commit()
	require.NoError(t, err)

	view, err := adb.OpenLastCommitted() // caches the trie of the second root
	require.NoError(t, err)
	view.Release()

	require.NoError(t, adb.RecreateTrie(rootA))
	view, err = adb.OpenLastCommitted()
	require.NoError(t, err)
	defer view.Release()

	root, err := view.RootHash()
	require.NoError(t, err)
	require.Equal(t, rootA, root)
	acc, err := view.GetExistingAccount(viewAddress(1))
	require.NoError(t, err)
	require.Equal(t, int64(1), acc.(state.UserAccountHandler).GetBalance(nil, false))
}

// A view that captured an older commit and recreates its trie late must not evict the newer one.
func TestCommittedView_StaleViewKeepsTheNewerCachedTrie(t *testing.T) {
	t.Parallel()

	adb := newPruningProofAccountsDB(t, mock.NewMemDbMock(), &mock.ProtobufMarshalizerMock{})
	putBalance(t, adb, viewAddress(1), 1)
	_, err := adb.Commit()
	require.NoError(t, err)

	newer, err := adb.LastCommittedTrieAt(2)
	require.NoError(t, err)
	_, err = adb.LastCommittedTrieAt(1)
	require.NoError(t, err)
	cached, err := adb.LastCommittedTrieAt(2)
	require.NoError(t, err)
	require.Same(t, newer, cached)
}
