package fallback_test

import (
	"bytes"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fallback"
	"github.com/klever-io/klever-go/core/fallback/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewFallbackHeaderValidator_ShouldErrNilHeadersDataPool(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerStub{}
	storageService := &mock.ChainStorerStub{}

	fhv, err := fallback.NewFallbackHeaderValidator(nil, marshalizer, storageService)
	assert.Nil(t, fhv)
	assert.Equal(t, common.ErrNilHeadersDataPool, err)
}

func TestNewFallbackHeaderValidator_ShouldErrNilMarshalizer(t *testing.T) {
	t.Parallel()

	headersPool := &mock.HeadersCacherStub{}
	storageService := &mock.ChainStorerStub{}

	fhv, err := fallback.NewFallbackHeaderValidator(headersPool, nil, storageService)
	assert.Nil(t, fhv)
	assert.Equal(t, process.ErrNilMarshalizer, err)
}

func TestNewFallbackHeaderValidator_ShouldErrNilStorage(t *testing.T) {
	t.Parallel()

	headersPool := &mock.HeadersCacherStub{}
	marshalizer := &mock.MarshalizerStub{}

	fhv, err := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, nil)
	assert.Nil(t, fhv)
	assert.Equal(t, process.ErrNilStorage, err)
}

func TestNewFallbackHeaderValidator_ShouldWork(t *testing.T) {
	t.Parallel()

	headersPool := &mock.HeadersCacherStub{}
	marshalizer := &mock.MarshalizerStub{}
	storageService := &mock.ChainStorerStub{}

	fhv, err := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, storageService)
	assert.False(t, check.IfNil(fhv))
	assert.Nil(t, err)
}

func TestShouldApplyFallbackConsensus_ShouldReturnFalseWhenHeaderIsNil(t *testing.T) {
	t.Parallel()

	headersPool := &mock.HeadersCacherStub{}
	marshalizer := &mock.MarshalizerStub{}
	storageService := &mock.ChainStorerStub{}

	fhv, _ := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, storageService)
	assert.False(t, fhv.ShouldApplyFallbackValidation(nil))
}

func TestShouldApplyFallbackConsensus_ShouldReturnFalseWhenIsNotMetachainBlock(t *testing.T) {
	t.Parallel()

	headersPool := &mock.HeadersCacherStub{}
	marshalizer := &mock.MarshalizerStub{}
	storageService := &mock.ChainStorerStub{}
	header := &block.Block{Header: &block.BlockHeader{}}

	fhv, _ := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, storageService)
	assert.False(t, fhv.ShouldApplyFallbackValidation(header))
}

func TestShouldApplyFallbackConsensus_ShouldReturnFalseWhenIsNotStartOfEpochMetachainBlock(t *testing.T) {
	t.Parallel()

	headersPool := &mock.HeadersCacherStub{}
	marshalizer := &mock.MarshalizerStub{}
	storageService := &mock.ChainStorerStub{}
	metaBlock := &block.Block{Header: &block.BlockHeader{}}

	fhv, _ := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, storageService)
	assert.False(t, fhv.ShouldApplyFallbackValidation(metaBlock))
}

func TestShouldApplyFallbackConsensus_ShouldReturnFalseWhenPreviousHeaderIsNotFound(t *testing.T) {
	t.Parallel()

	headersPool := &mock.HeadersCacherStub{}
	marshalizer := &mock.MarshalizerStub{}
	storageService := &mock.ChainStorerStub{}
	metaBlock := &block.Block{Header: &block.BlockHeader{}}

	fhv, _ := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, storageService)
	assert.False(t, fhv.ShouldApplyFallbackValidation(metaBlock))
}

func TestShouldApplyFallbackConsensus_ShouldReturnFalseWhenRoundIsNotTooOld(t *testing.T) {
	t.Parallel()

	prevHash := []byte("prev_hash")
	headersPool := &mock.HeadersCacherStub{
		GetHeaderByHashCalled: func(hash []byte) (data.HeaderHandler, error) {
			if bytes.Equal(hash, prevHash) {
				return &block.Block{Header: &block.BlockHeader{}}, nil
			}
			return nil, errors.New("error")
		},
	}
	marshalizer := &mock.MarshalizerStub{}
	storageService := &mock.ChainStorerStub{}
	metaBlock := &block.Block{Header: &block.BlockHeader{
		Slot:       core.MaxSlotsWithoutCommittedStartInEpochBlock - 1,
		ParentHash: prevHash,
	}}

	fhv, _ := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, storageService)
	assert.False(t, fhv.ShouldApplyFallbackValidation(metaBlock))
}

func TestShouldApplyFallbackConsensus_ShouldReturnTrue(t *testing.T) {
	t.Parallel()

	prevHash := []byte("prev_hash")
	headersPool := &mock.HeadersCacherStub{
		GetHeaderByHashCalled: func(hash []byte) (data.HeaderHandler, error) {
			if bytes.Equal(hash, prevHash) {
				return &block.Block{Header: &block.BlockHeader{}}, nil
			}
			return nil, errors.New("error")
		},
	}
	marshalizer := &mock.MarshalizerStub{}
	storageService := &mock.ChainStorerStub{}
	metaBlock := &block.Block{Header: &block.BlockHeader{
		Slot:         core.MaxSlotsWithoutCommittedStartInEpochBlock,
		ParentHash:   prevHash,
		IsEpochStart: true,
	}}

	fhv, _ := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, storageService)
	assert.True(t, fhv.ShouldApplyFallbackValidation(metaBlock))
}
