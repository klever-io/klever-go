package factory

import (
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/marshal"
)

// UserAccountTrie represents the use account identifier
const UserAccountTrie = "userAccount"

// PeerAccountTrie represents the peer account identifier
const PeerAccountTrie = "peerAccount"

// KAppAccountTrie represents the kapp account identifier
const KAppAccountTrie = "kappAccount"

// TrieFactoryArgs holds arguments for creating a trie factory
type TrieFactoryArgs struct {
	EvictionWaitingListCfg   config.EvictionWaitingListConfig
	SnapshotDbCfg            config.DBConfig
	Marshalizer              marshal.Marshalizer
	Hasher                   hashing.Hasher
	PathManager              storage.PathManagerHandler
	TrieStorageManagerConfig config.TrieStorageManagerConfig
}
