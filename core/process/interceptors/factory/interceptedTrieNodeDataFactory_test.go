package factory

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/stretchr/testify/assert"
)

func TestNewInterceptedTrieNodeDataFactory_NilArgumentsShouldErr(t *testing.T) {
	t.Parallel()

	itn, err := NewInterceptedTrieNodeDataFactory(nil)

	assert.Nil(t, itn)
	assert.Equal(t, process.ErrNilArgumentStruct, err)
}

func TestNewInterceptedTrieNodeDataFactory_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()
	arg.ProtoMarshalizer = nil

	itn, err := NewInterceptedTrieNodeDataFactory(arg)
	assert.Nil(t, itn)
	assert.Equal(t, process.ErrNilMarshalizer, err)
}

func TestNewInterceptedTrieNodeDataFactory_NilHasherShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()
	arg.Hasher = nil

	itn, err := NewInterceptedTrieNodeDataFactory(arg)
	assert.Nil(t, itn)
	assert.Equal(t, common.ErrNilHasher, err)
}

func TestNewInterceptedTrieNodeDataFactory_OkValsShouldWork(t *testing.T) {
	t.Parallel()

	itn, err := NewInterceptedTrieNodeDataFactory(createMockArgument())
	assert.NotNil(t, itn)
	assert.Nil(t, err)
	assert.False(t, itn.IsInterfaceNil())
}
