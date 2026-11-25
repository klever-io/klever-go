package state

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/tools/check"
)

// ErrReadOnlyView is returned by every write to a read-only view of committed state.
var ErrReadOnlyView = errors.New("committed state view is read-only")

// CommittedView reads the state at one committed root. The root stays pinned until Release, so
// pruning deletes none of its nodes and the accounts it returns, data tries included, can be read
// until then and never after. Block processing never writes it, so any goroutine may read it. The
// accounts are fresh objects, not tracked for saving.
type CommittedView struct {
	adb     *AccountsDB
	trie    data.Trie
	root    []byte
	storage data.StorageManager
	release sync.Once
}

// OpenLastCommitted returns a view of the last committed root, pinned until Release. Unlike
// GetExistingAccount it never sees what a block still being processed has saved, nor shares a trie
// with block processing, so any goroutine may read it. The accounts it returns, data tries
// included, may be read until Release and never after.
func (adb *AccountsDB) OpenLastCommitted() (*CommittedView, error) {
	return adb.openView(nil, true)
}

// OpenCommittedAt pins rootHash and returns a read-only view of the state at it. It fails with
// ErrStateRootUnavailable when rootHash is not stored or already pruned. A root whose prune was
// already issued can still open and then fail on reads: the pin cannot recall a prune the pruning
// buffer already holds. Open the last committed root, or one RootHash just returned. The caller
// must Release it.
func (adb *AccountsDB) OpenCommittedAt(rootHash []byte) (*CommittedView, error) {
	return adb.openView(rootHash, false)
}

// openView pins root, or the last committed root when atLast, copied under mutOp so that a pin never
// races the prune that PruneTrie runs under it. Recreating the trie then reads the root node, outside
// mutOp, which fails if a prune removed it before the pin. A view of the last committed root shares
// the trie recreated once per commit (see committedTrieAt), which no prune can have touched while it
// is the last; any other root gets its own.
func (adb *AccountsDB) openView(root []byte, atLast bool) (*CommittedView, error) {
	adb.mutOp.RLock()
	mainTrie := adb.mainTrie
	if check.IfNil(mainTrie) {
		adb.mutOp.RUnlock()
		return nil, common.ErrNilTrie
	}
	if atLast {
		root = adb.lastRootHash
	}
	root = cloneBytes(root)
	isLast := bytes.Equal(root, adb.lastRootHash)
	gen := adb.rootGen
	storage := mainTrie.GetStorageManager()
	if !check.IfNil(storage) {
		storage.PinRoot(root)
	}
	adb.mutOp.RUnlock()

	tr, err := adb.committedTrieAt(mainTrie, root, isLast, gen)
	if err != nil {
		unpin(storage, root)
		return nil, err
	}

	return &CommittedView{adb: adb, trie: tr, root: root, storage: storage}, nil
}

// committedTrieAt returns the main trie at root. The last committed root's trie is recreated once
// per commit, by the first view after it, and shared by every view of that root, so the nodes it
// resolves are shared too. On a cache miss, building the empty trie the root is recreated through
// takes the live trie's lock (no I/O) under mutCommitted, so it can wait for the live trie's Commit
// too.
// ponytail: the shared trie keeps every node it resolves until the next commit replaces it, so on a
// chain that stops committing, reads can pull the whole main trie into memory. Recreate it after a
// read budget if halted nodes keep serving REST.
func (adb *AccountsDB) committedTrieAt(mainTrie data.Trie, root []byte, isLast bool, gen uint64) (data.Trie, error) {
	if !isLast {
		return recreateFromMainDb(mainTrie, root)
	}

	adb.mutCommitted.Lock()
	defer adb.mutCommitted.Unlock()

	if !check.IfNil(adb.committedTrie) && adb.committedTrieGen == gen {
		return adb.committedTrie, nil
	}

	committedTrie, err := recreateFromMainDb(mainTrie, root)
	if err != nil {
		return nil, err
	}
	// a view that captured gen before a later commit must not replace the newer root's trie
	if gen >= adb.committedTrieGen {
		adb.committedTrie, adb.committedTrieGen = committedTrie, gen
	}

	return committedTrie, nil
}

func unpin(storage data.StorageManager, root []byte) {
	if !check.IfNil(storage) {
		storage.UnpinRoot(root)
	}
}

// Release unpins the view's root; the next prune after the last release of a root runs what that
// root held back. Releasing twice is harmless.
func (v *CommittedView) Release() {
	v.release.Do(func() {
		unpin(v.storage, v.root)
	})
}

// GetExistingAccount returns the account at address, or common.ErrAccNotFound
func (v *CommittedView) GetExistingAccount(address []byte) (AccountHandler, error) {
	return v.adb.loadCommittedAccount(v.trie, address)
}

// LoadAccount returns the account at address, or a new empty one when it is missing
func (v *CommittedView) LoadAccount(address []byte) (AccountHandler, error) {
	acc, err := v.GetExistingAccount(address)
	if errors.Is(err, common.ErrAccNotFound) {
		return v.adb.accountFactory.CreateAccount(address)
	}

	return acc, err
}

// GetCode returns the code stored under codeHash, or nil
func (v *CommittedView) GetCode(codeHash []byte) []byte {
	if len(codeHash) == 0 {
		return nil
	}

	val, err := v.trie.Get(codeHash)
	if err != nil {
		return nil
	}

	var codeEntry CodeEntry
	err = v.adb.marshalizer.Unmarshal(&codeEntry, val)
	if err != nil {
		return nil
	}

	return codeEntry.Code
}

// RootHash returns the view's root
func (v *CommittedView) RootHash() ([]byte, error) {
	return cloneBytes(v.root), nil
}

// GetAllLeaves walks the view's root; any other root is refused
func (v *CommittedView) GetAllLeaves(rootHash []byte, ctx context.Context) (*data.TrieIteratorChannels, error) {
	if !bytes.Equal(rootHash, v.root) {
		return nil, fmt.Errorf("%w: a view walks only its own root", ErrReadOnlyView)
	}

	return v.trie.GetAllLeavesOnChannel(rootHash, ctx)
}

// refusesWrites is the write side of a read-only AccountsAdapter: writes fail with ErrReadOnlyView
// and the pruning and snapshot calls do nothing.
type refusesWrites struct{}

// IsPruningEnabled returns false: read-only state never prunes
func (refusesWrites) IsPruningEnabled() bool {
	return false
}

// SaveAccount is refused
func (refusesWrites) SaveAccount(_ AccountHandler) error {
	return ErrReadOnlyView
}

// SaveAccounts is refused
func (refusesWrites) SaveAccounts(_ ...AccountHandler) error {
	return ErrReadOnlyView
}

// RemoveAccountCode is refused
func (refusesWrites) RemoveAccountCode(_ []byte) error {
	return ErrReadOnlyView
}

// RemoveAccount is refused
func (refusesWrites) RemoveAccount(_ []byte) error {
	return ErrReadOnlyView
}

// Commit is refused
func (refusesWrites) Commit() ([]byte, error) {
	return nil, ErrReadOnlyView
}

// JournalLen returns 0: read-only state journals nothing
func (refusesWrites) JournalLen() int {
	return 0
}

// RevertToSnapshot is refused
func (refusesWrites) RevertToSnapshot(_ int) error {
	return ErrReadOnlyView
}

// RecreateTrie is refused
func (refusesWrites) RecreateTrie(_ []byte) error {
	return ErrReadOnlyView
}

// RecreateAllTries is refused
func (refusesWrites) RecreateAllTries(_ []byte, _ context.Context) (map[string]data.Trie, error) {
	return nil, ErrReadOnlyView
}

// PruneTrie does nothing
func (refusesWrites) PruneTrie(_ []byte, _ data.TriePruningIdentifier) {
	// no-op: a read-only view never prunes or snapshots; the processing adapter owns that
}

// CancelPrune does nothing
func (refusesWrites) CancelPrune(_ []byte, _ data.TriePruningIdentifier) {
	// no-op: a read-only view never prunes or snapshots; the processing adapter owns that
}

// SnapshotState does nothing
func (refusesWrites) SnapshotState(_ []byte, _ context.Context) {
	// no-op: a read-only view never prunes or snapshots; the processing adapter owns that
}

// SetStateCheckpoint does nothing
func (refusesWrites) SetStateCheckpoint(_ []byte, _ context.Context) {
	// no-op: a read-only view never prunes or snapshots; the processing adapter owns that
}

// GetNumCheckpoints returns 0: a read-only view takes no checkpoints
func (refusesWrites) GetNumCheckpoints() uint32 {
	return 0
}

// lastCommittedAccounts is a read-only AccountsAdapter over the last committed root of accounts.
// Every read opens a view of the root committed at that moment and releases it before returning, so
// it never sees what a block in progress has saved, never hands out a trie block processing writes,
// and holds no pin between calls.
// ponytail: a pin covers the load, not the lazy data-trie reads the caller makes afterwards. If the
// root is pruned in between, one block after the next commit, those reads hit a missing node and
// never return other data; callers that swallow the error (kappAccount.GetStorage, Node.GetAsset)
// answer nil or not found. Leaf walks enter pruning buffering mode themselves. A caller that needs
// a whole response from one root opens a CommittedView instead.
// ponytail: every read recreates the account's data trie (BenchmarkKAppStorageRead: ~60 µs against
// ~5 µs for a live uncached read); cache data tries per committed root if REST load needs it.
type lastCommittedAccounts struct {
	refusesWrites
	accounts CommittedOpener
}

var _ AccountsAdapter = (*lastCommittedAccounts)(nil)
var _ CommittedOpener = (*AccountsDB)(nil)

// NewLastCommittedAccounts returns a read-only AccountsAdapter that reads accounts at its last
// committed root
func NewLastCommittedAccounts(accounts CommittedOpener) (*lastCommittedAccounts, error) {
	if check.IfNil(accounts) {
		return nil, common.ErrNilAccountsAdapter
	}

	return &lastCommittedAccounts{accounts: accounts}, nil
}

// readLast runs fn on a view of the last committed root and releases the view before returning
func readLast[T any](l *lastCommittedAccounts, fn func(view *CommittedView) (T, error)) (T, error) {
	view, err := l.accounts.OpenLastCommitted()
	if err != nil {
		var zero T
		return zero, err
	}
	defer view.Release()

	return fn(view)
}

// GetExistingAccount returns the account at address in the last committed root, or common.ErrAccNotFound
func (l *lastCommittedAccounts) GetExistingAccount(address []byte) (AccountHandler, error) {
	return readLast(l, func(view *CommittedView) (AccountHandler, error) { return view.GetExistingAccount(address) })
}

// LoadAccount returns the account at address in the last committed root, or a new empty one
func (l *lastCommittedAccounts) LoadAccount(address []byte) (AccountHandler, error) {
	return readLast(l, func(view *CommittedView) (AccountHandler, error) { return view.LoadAccount(address) })
}

// GetCode returns the code stored under codeHash in the last committed root, or nil
func (l *lastCommittedAccounts) GetCode(codeHash []byte) []byte {
	code, _ := readLast(l, func(view *CommittedView) ([]byte, error) { return view.GetCode(codeHash), nil })
	return code
}

// RootHash returns the last committed root
func (l *lastCommittedAccounts) RootHash() ([]byte, error) {
	return readLast(l, func(view *CommittedView) ([]byte, error) { return view.RootHash() })
}

// GetAllLeaves walks rootHash, a committed root such as one RootHash returned before a later commit.
// The walk keeps pruning off until it ends.
func (l *lastCommittedAccounts) GetAllLeaves(rootHash []byte, ctx context.Context) (*data.TrieIteratorChannels, error) {
	view, err := l.accounts.OpenCommittedAt(rootHash)
	if err != nil {
		return nil, err
	}
	defer view.Release()

	return view.GetAllLeaves(rootHash, ctx)
}

// IsInterfaceNil returns true if there is no value under the interface
func (l *lastCommittedAccounts) IsInterfaceNil() bool {
	return l == nil
}
