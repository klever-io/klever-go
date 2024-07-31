package state

import (
	"sync"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

// KAppAccountsDB will save and synchronize data from peer processor, plus will synchronize with nodesCoordinator
type KAppAccountsDB struct {
	*AccountsDB
}

// NewKAppAccountsDB creates a new account manager
func NewKAppAccountsDB(
	trie data.Trie,
	hasher hashing.Hasher,
	marshalizer marshal.Marshalizer,
	accountFactory AccountFactory,
	processingMode core.NodeProcessingMode,
) (*KAppAccountsDB, error) {
	if check.IfNil(trie) {
		return nil, common.ErrNilTrie
	}
	if check.IfNil(hasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(accountFactory) {
		return nil, common.ErrNilAccountFactory
	}

	numCheckpoints := getNumAccountsCheckpoints(trie.GetStorageManager())
	return &KAppAccountsDB{
		&AccountsDB{
			mainTrie:               trie,
			hasher:                 hasher,
			marshalizer:            marshalizer,
			accountFactory:         accountFactory,
			entries:                make([]JournalAccountEntry, 0),
			mutOp:                  sync.RWMutex{},
			dataTries:              NewDataTriesHolder(),
			obsoleteDataTrieHashes: make(map[string][][]byte),
			numCheckpoints:         numCheckpoints,
			loadCodeMeasurements: &loadingMeasurements{
				identifier: "load code",
			},
			processingMode: processingMode,
		},
	}, nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (adb *KAppAccountsDB) IsInterfaceNil() bool {
	return adb == nil
}
