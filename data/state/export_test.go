package state

import "github.com/klever-io/klever-go/data"

func NewEmptyBaseAccount(address []byte, tracker DataTrieTracker) *baseAccount {
	return &baseAccount{
		address:         address,
		dataTrieTracker: tracker,
	}
}

func (adb *AccountsDB) GetAccount(address []byte) (AccountHandler, error) {
	return adb.getAccount(address)
}

func (adb *AccountsDB) LoadDataTrie(accountHandler baseAccountHandler) error {
	return adb.loadDataTrie(accountHandler)
}

func NewTestSnapshotStatistics(delta int) *snapshotStatistics {
	return newSnapshotStatistics(delta)
}

func (ss *snapshotStatistics) GetNumNodes() uint64 {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()
	return ss.numNodes
}

func (ss *snapshotStatistics) GetTrieSize() uint64 {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()
	return ss.trieSize
}

func (ss *snapshotStatistics) GetNumDataTries() uint64 {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()
	return ss.numDataTries
}

func (adb *AccountsDB) Journalize(entry JournalAccountEntry) {
	adb.journalize(entry)
}

func (adb *AccountsDB) RemoveCodeAndDataTrie(acnt AccountHandler) error {
	return adb.removeCodeAndDataTrie(acnt)
}

// TryLockMutOp reports whether the accounts lock is free, releasing it again if taken.
func (adb *AccountsDB) TryLockMutOp() bool {
	if !adb.mutOp.TryLock() {
		return false
	}
	adb.mutOp.Unlock()
	return true
}

// TryLockMutPrune reports whether PruneTrie could take the prune lock now, releasing it again if taken.
func (adb *AccountsDB) TryLockMutPrune() bool {
	if !adb.mutPrune.TryLock() {
		return false
	}
	adb.mutPrune.Unlock()
	return true
}

// MaxHistoricalProofWalks exports the cap on historical-root proofs in flight.
const MaxHistoricalProofWalks = maxHistoricalProofWalks

// NumTrackedDataTries returns how many data tries the adapter tracks for the next Commit
func (adb *AccountsDB) NumTrackedDataTries() int {
	return len(adb.dataTries.GetAll())
}

// NumCodeLoadsMeasured returns how many code loads the per-commit load-code stats counted so far
func (adb *AccountsDB) NumCodeLoadsMeasured() uint64 {
	adb.loadCodeMeasurements.Lock()
	defer adb.loadCodeMeasurements.Unlock()

	return adb.loadCodeMeasurements.numCalls
}

// LastCommittedTrieAt returns the last committed root's trie as a view that captured gen would.
func (adb *AccountsDB) LastCommittedTrieAt(gen uint64) (data.Trie, error) {
	return adb.committedTrieAt(adb.mainTrie, adb.lastRootHash, true, gen)
}
