package fallback_test

import (
	"bytes"
	"testing"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
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

const v7Epoch = uint32(100)

func forkWithV7At(epoch uint32) *commonMock.ForkControllerStub {
	return &commonMock.ForkControllerStub{FixAuditChangesV7Epoch: &epoch}
}

func eligibleEpochStartHeader(prevHash []byte, epoch uint32) *block.Block {
	return &block.Block{Header: &block.BlockHeader{
		Slot:         core.MaxSlotsWithoutCommittedStartInEpochBlock,
		ParentHash:   prevHash,
		IsEpochStart: true,
		Epoch:        epoch,
	}}
}

func TestNewFallbackHeaderValidator_ShouldErrNilHeadersDataPool(t *testing.T) {
	t.Parallel()

	marshalizer := &mock.MarshalizerStub{}
	storageService := &mock.ChainStorerStub{}

	fhv, err := fallback.NewFallbackHeaderValidator(nil, marshalizer, storageService, forkWithV7At(v7Epoch))
	assert.Nil(t, fhv)
	assert.Equal(t, common.ErrNilHeadersDataPool, err)
}

func TestNewFallbackHeaderValidator_ShouldErrNilMarshalizer(t *testing.T) {
	t.Parallel()

	headersPool := &mock.HeadersCacherStub{}
	storageService := &mock.ChainStorerStub{}

	fhv, err := fallback.NewFallbackHeaderValidator(headersPool, nil, storageService, forkWithV7At(v7Epoch))
	assert.Nil(t, fhv)
	assert.Equal(t, process.ErrNilMarshalizer, err)
}

func TestNewFallbackHeaderValidator_ShouldErrNilStorage(t *testing.T) {
	t.Parallel()

	headersPool := &mock.HeadersCacherStub{}
	marshalizer := &mock.MarshalizerStub{}

	fhv, err := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, nil, forkWithV7At(v7Epoch))
	assert.Nil(t, fhv)
	assert.Equal(t, process.ErrNilStorage, err)
}

func TestNewFallbackHeaderValidator_ShouldErrNilForkController(t *testing.T) {
	t.Parallel()

	fhv, err := fallback.NewFallbackHeaderValidator(&mock.HeadersCacherStub{}, &mock.MarshalizerStub{}, &mock.ChainStorerStub{}, nil)
	assert.Nil(t, fhv)
	assert.Equal(t, common.ErrNilForkController, err)
}

func TestNewFallbackHeaderValidator_ShouldWork(t *testing.T) {
	t.Parallel()

	headersPool := &mock.HeadersCacherStub{}
	marshalizer := &mock.MarshalizerStub{}
	storageService := &mock.ChainStorerStub{}

	fhv, err := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, storageService, forkWithV7At(v7Epoch))
	assert.False(t, check.IfNil(fhv))
	assert.Nil(t, err)
}

func TestShouldApplyFallbackConsensus_ShouldReturnFalseWhenHeaderIsNil(t *testing.T) {
	t.Parallel()

	headersPool := &mock.HeadersCacherStub{}
	marshalizer := &mock.MarshalizerStub{}
	storageService := &mock.ChainStorerStub{}

	fhv, _ := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, storageService, forkWithV7At(v7Epoch))
	assert.False(t, fhv.ShouldApplyFallbackValidation(nil))
}

func TestShouldApplyFallbackConsensus_ShouldReturnFalseWhenIsNotMetachainBlock(t *testing.T) {
	t.Parallel()

	headersPool := &mock.HeadersCacherStub{}
	marshalizer := &mock.MarshalizerStub{}
	storageService := &mock.ChainStorerStub{}
	header := &block.Block{Header: &block.BlockHeader{}}

	fhv, _ := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, storageService, forkWithV7At(v7Epoch))
	assert.False(t, fhv.ShouldApplyFallbackValidation(header))
}

func TestShouldApplyFallbackConsensus_ShouldReturnFalseWhenIsNotStartOfEpochMetachainBlock(t *testing.T) {
	t.Parallel()

	headersPool := &mock.HeadersCacherStub{}
	marshalizer := &mock.MarshalizerStub{}
	storageService := &mock.ChainStorerStub{}
	metaBlock := &block.Block{Header: &block.BlockHeader{}}

	fhv, _ := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, storageService, forkWithV7At(v7Epoch))
	assert.False(t, fhv.ShouldApplyFallbackValidation(metaBlock))
}

func TestShouldApplyFallbackConsensus_ShouldReturnFalseWhenPreviousHeaderIsNotFound(t *testing.T) {
	t.Parallel()

	headersPool := &mock.HeadersCacherStub{}
	marshalizer := &mock.MarshalizerStub{}
	storageService := &mock.ChainStorerStub{}
	metaBlock := &block.Block{Header: &block.BlockHeader{}}

	fhv, _ := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, storageService, forkWithV7At(v7Epoch))
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

	fhv, _ := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, storageService, forkWithV7At(v7Epoch))
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
	metaBlock := eligibleEpochStartHeader(prevHash, v7Epoch-1)

	fhv, _ := fallback.NewFallbackHeaderValidator(headersPool, marshalizer, storageService, forkWithV7At(v7Epoch))
	assert.True(t, fhv.ShouldApplyFallbackValidation(metaBlock))
}

func TestShouldApplyFallbackConsensus_FixAuditChangesV7Gate(t *testing.T) {
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
	fhv, _ := fallback.NewFallbackHeaderValidator(headersPool, &mock.MarshalizerStub{}, &mock.ChainStorerStub{}, forkWithV7At(v7Epoch))

	// gated on the header's own epoch, so pre-fork history still replays with the fallback quorum
	assert.True(t, fhv.ShouldApplyFallbackValidation(eligibleEpochStartHeader(prevHash, v7Epoch-1)))
	assert.False(t, fhv.ShouldApplyFallbackValidation(eligibleEpochStartHeader(prevHash, v7Epoch)))
	assert.False(t, fhv.ShouldApplyFallbackValidation(eligibleEpochStartHeader(prevHash, v7Epoch+1)))
}
