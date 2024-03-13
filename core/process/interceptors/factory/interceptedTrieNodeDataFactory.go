package factory

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ process.InterceptedDataFactory = (*interceptedTrieNodeDataFactory)(nil)

type interceptedTrieNodeDataFactory struct {
	marshalizer marshal.Marshalizer
	hasher      hashing.Hasher
}

// NewInterceptedTrieNodeDataFactory creates an instance of interceptedTrieNodeDataFactory
func NewInterceptedTrieNodeDataFactory(
	argument *ArgInterceptedDataFactory,
) (*interceptedTrieNodeDataFactory, error) {

	if argument == nil {
		return nil, process.ErrNilArgumentStruct
	}
	if check.IfNil(argument.ProtoMarshalizer) {
		return nil, process.ErrNilMarshalizer
	}
	if check.IfNil(argument.Hasher) {
		return nil, common.ErrNilHasher
	}

	return &interceptedTrieNodeDataFactory{
		marshalizer: argument.ProtoMarshalizer,
		hasher:      argument.Hasher,
	}, nil
}

// Create creates instances of InterceptedData by unmarshalling provided buffer
func (sidf *interceptedTrieNodeDataFactory) Create(buff []byte) (process.InterceptedData, error) {
	return trie.NewInterceptedTrieNode(buff, sidf.marshalizer, sidf.hasher)
}

// IsInterfaceNil returns true if there is no value under the interface
func (sidf *interceptedTrieNodeDataFactory) IsInterfaceNil() bool {
	return sidf == nil
}
