package state_test

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/stretchr/testify/require"
)

// Benchmark comparing the old external-reader path (cached shared instance)
// against the new one (uncached private copy) for KApp storage reads.
func BenchmarkKAppStorageRead(b *testing.B) {
	marshalizer := &mock.MarshalizerMock{}
	hsh := &mock.HasherMock{}
	storageManager, err := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
	require.NoError(b, err)
	tr, err := trie.NewTrie(storageManager, marshalizer, hsh, 5)
	require.NoError(b, err)
	adb, err := state.NewAccountsDB(tr, hsh, marshalizer, factory.NewKAppAccountCreator(), core.Normal)
	require.NoError(b, err)

	addr := make([]byte, 32)
	copy(addr, []byte("validators-kapp-bench-address"))

	// populate the kapp account with 10k storage entries, committed to trie
	acnt, err := adb.LoadAccount(addr)
	require.NoError(b, err)
	app := acnt.(state.KAppAccountHandler)
	val := make([]byte, 8)
	for i := 0; i < 10000; i++ {
		binary.BigEndian.PutUint64(val, uint64(i))
		require.NoError(b, app.SetStorage([]byte(fmt.Sprintf("pendingRewards/addr-%05d", i)), val))
	}
	require.NoError(b, adb.SaveAccount(app))
	_, err = adb.Commit()
	require.NoError(b, err)

	cacher, err := state.NewAccountsCacher(state.ArgsAcccountCacher{
		Accounts: adb,
		Kapps:    adb,
		Peers:    adb,
	})
	require.NoError(b, err)
	cacher.ResetAll(true)

	key := []byte("pendingRewards/addr-05000")

	b.Run("cached shared instance (old path)", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			kapp, errLoad := cacher.LoadKApp(addr)
			if errLoad != nil {
				b.Fatal(errLoad)
			}
			if v := kapp.GetStorage(key); len(v) == 0 {
				b.Fatal("missing value")
			}
		}
	})

	b.Run("uncached private copy (new path)", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			kapp, errLoad := cacher.LoadKAppUncached(addr)
			if errLoad != nil {
				b.Fatal(errLoad)
			}
			if v := kapp.GetStorage(key); len(v) == 0 {
				b.Fatal("missing value")
			}
		}
	})
}
