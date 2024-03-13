package factory

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/block/interceptedBlocks"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
)

func TestNewInterceptedBlockDataFactory_NilArgumentsShouldErr(t *testing.T) {
	t.Parallel()

	imh, err := NewInterceptedBlockDataFactory(nil)

	assert.Nil(t, imh)
	assert.Equal(t, process.ErrNilArgumentStruct, err)
}

func TestNewInterceptedBlockDataFactory_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()
	arg.ProtoMarshalizer = nil

	imdf, err := NewInterceptedBlockDataFactory(arg)
	assert.True(t, check.IfNil(imdf))
	assert.Equal(t, process.ErrNilMarshalizer, err)
}

func TestNewInterceptedBlockDataFactory_NilHasherShouldErr(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()
	arg.Hasher = nil

	imdf, err := NewInterceptedBlockDataFactory(arg)
	assert.True(t, check.IfNil(imdf))
	assert.Equal(t, common.ErrNilHasher, err)
}

func TestInterceptedBlockDataFactory_ShouldWorkAndCreate(t *testing.T) {
	t.Parallel()

	arg := createMockArgument()

	imdf, err := NewInterceptedBlockDataFactory(arg)
	assert.False(t, check.IfNil(imdf))
	assert.Nil(t, err)

	marshalizer := &mock.MarshalizerMock{}
	emptyBlockBody := &block.Block{}
	emptyBlockBodyBuff, _ := marshalizer.Marshal(emptyBlockBody)
	interceptedData, err := imdf.Create(emptyBlockBodyBuff)
	assert.Nil(t, err)

	_, ok := interceptedData.(*interceptedBlocks.InterceptedBlock)
	assert.True(t, ok)
}
