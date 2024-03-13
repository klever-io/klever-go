package process

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/tools/marshal"
)

const maxTrieLevelInMemory = uint(5)

func createAccountAdapter(
	marshalizer marshal.Marshalizer,
	hasher hashing.Hasher,
	accountFactory state.AccountFactory,
	trieStorage data.StorageManager,
) (state.AccountsAdapter, error) {
	tr, err := trie.NewTrie(trieStorage, marshalizer, hasher, maxTrieLevelInMemory)
	if err != nil {
		return nil, err
	}

	adb, err := state.NewAccountsDB(tr, hasher, marshalizer, accountFactory, core.Normal)
	if err != nil {
		return nil, err
	}

	return adb, nil
}
