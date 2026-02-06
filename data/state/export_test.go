package state

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
