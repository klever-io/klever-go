package trie

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/tools/marshal"
)

type node interface {
	getHash() []byte
	setHash() error
	setGivenHash([]byte)
	setHashConcurrent(wg *sync.WaitGroup, c chan error)
	setRootHash() error
	getCollapsed() (node, error) // a collapsed node is a node that instead of the children holds the children hashes
	isCollapsed() bool
	isPosCollapsed(pos int) bool
	isDirty() bool
	getEncodedNode() ([]byte, error)
	commit(force bool, level byte, maxTrieLevelInMemory uint, originDb data.DBWriteCacher, targetDb data.DBWriteCacher) error
	resolveCollapsed(pos byte, db data.DBWriteCacher) error
	hashNode() ([]byte, error)
	hashChildren() error
	tryGet(key []byte, db data.DBWriteCacher) ([]byte, error)
	getNext(key []byte, db data.DBWriteCacher) (node, []byte, error)
	insert(n *leafNode, db data.DBWriteCacher) (bool, node, [][]byte, error)
	delete(key []byte, db data.DBWriteCacher) (bool, node, [][]byte, error)
	reduceNode(pos int) (node, bool, error)
	isEmptyOrNil() error
	print(writer io.Writer, index int, db data.DBWriteCacher)
	deepClone() node
	getDirtyHashes(data.ModifiedHashes) error
	getChildren(db data.DBWriteCacher) ([]node, error)
	isValid() bool
	setDirty(bool)
	loadChildren(func([]byte) (node, error)) ([][]byte, []node, error)
	getAllLeavesOnChannel(chan data.KeyValueHolder, []byte, data.DBWriteCacher, marshal.Marshalizer, context.Context) error
	getAllHashes(db data.DBWriteCacher) ([][]byte, error)
	getNextHashAndKey([]byte) (bool, []byte, []byte)
	isInterfaceNil() bool

	getMarshalizer() marshal.Marshalizer
	setMarshalizer(marshal.Marshalizer)
	getHasher() hashing.Hasher
	setHasher(hashing.Hasher)
}

type atomicBuffer interface {
	add(rootHash []byte)
	removeAll() [][]byte
	len() int
}

type snapshotNode interface {
	commit(force bool, level byte, maxTrieLevelInMemory uint, originDb data.DBWriteCacher, targetDb data.DBWriteCacher) error
}

// RequestHandler defines the methods through which request to data can be made
type RequestHandler interface {
	RequestTrieNodes(hashes [][]byte, topic string)
	RequestInterval() time.Duration
	IsInterfaceNil() bool
}
