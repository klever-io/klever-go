package sync_test

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/common/mock"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/sync"
	"github.com/klever-io/klever-go/data/block"
	"github.com/stretchr/testify/assert"
)

func TestNewMetaForkDetector_NilSlotManagerShouldErr(t *testing.T) {
	t.Parallel()

	sfd, err := sync.NewMetaForkDetector(
		nil,
		&mock.BlackListHandlerStub{},
		0,
	)
	assert.Nil(t, sfd)
	assert.Equal(t, process.ErrNilSlotManager, err)
}

func TestNewMetaForkDetector_NilBlackListShouldErr(t *testing.T) {
	t.Parallel()

	sfd, err := sync.NewMetaForkDetector(
		&consensusMock.SlotManagerMock{},
		nil,
		0,
	)
	assert.Nil(t, sfd)
	assert.Equal(t, process.ErrNilBlackListCacher, err)
}

func TestNewMetaForkDetector_OkParamsShouldWork(t *testing.T) {
	t.Parallel()

	sfd, err := sync.NewMetaForkDetector(
		&consensusMock.SlotManagerMock{SlotIndex: 1},
		&mock.BlackListHandlerStub{},
		0,
	)
	assert.Nil(t, err)
	assert.NotNil(t, sfd)

	assert.Equal(t, uint64(0), sfd.LastCheckpointNonce())
	assert.Equal(t, uint64(0), sfd.LastCheckpointSlot())
	assert.Equal(t, uint64(0), sfd.FinalCheckpointNonce())
	assert.Equal(t, uint64(0), sfd.FinalCheckpointSlot())
}

func TestMetaForkDetector_AddHeaderNilHeaderShouldErr(t *testing.T) {
	t.Parallel()

	sloterMock := &consensusMock.SlotManagerMock{SlotIndex: 100}
	bfd, _ := sync.NewMetaForkDetector(sloterMock, &mock.BlackListHandlerStub{}, 0)
	err := bfd.AddHeader(nil, make([]byte, 0), process.BHProcessed, nil, nil)
	assert.Equal(t, sync.ErrNilHeader, err)
}

func TestMetaForkDetector_AddHeaderNilHashShouldErr(t *testing.T) {
	t.Parallel()

	sloterMock := &consensusMock.SlotManagerMock{SlotIndex: 100}
	bfd, _ := sync.NewMetaForkDetector(sloterMock, &mock.BlackListHandlerStub{}, 0)
	err := bfd.AddHeader(&block.Block{Header: &block.BlockHeader{}}, nil, process.BHProcessed, nil, nil)
	assert.Equal(t, sync.ErrNilHash, err)
}

func TestMetaForkDetector_AddHeaderNotPresentShouldWork(t *testing.T) {
	t.Parallel()

	hdr := &block.BlockHeader{Nonce: 1, Slot: 1}
	hash := make([]byte, 0)
	sloterMock := &consensusMock.SlotManagerMock{SlotIndex: 1, TimeDurationCalled: func() time.Duration { return 0 }}
	bfd, _ := sync.NewMetaForkDetector(sloterMock, &mock.BlackListHandlerStub{}, 0)

	err := bfd.AddHeader(&block.Block{Header: hdr}, hash, process.BHProcessed, nil, nil)
	assert.Nil(t, err)

	hInfos := bfd.GetHeaders(1)
	assert.Equal(t, 1, len(hInfos))
	assert.Equal(t, hash, hInfos[0].Hash())
}

func TestMetaForkDetector_AddHeaderPresentShouldAppend(t *testing.T) {
	t.Parallel()

	hdr1 := &block.BlockHeader{Nonce: 1, Slot: 1}
	hash1 := []byte("hash1")
	hdr2 := &block.BlockHeader{Nonce: 1, Slot: 1}
	hash2 := []byte("hash2")
	sloterMock := &consensusMock.SlotManagerMock{SlotIndex: 1, TimeDurationCalled: func() time.Duration { return 0 }}
	bfd, _ := sync.NewMetaForkDetector(sloterMock, &mock.BlackListHandlerStub{}, 0)

	err := bfd.AddHeader(&block.Block{Header: hdr1}, hash1, process.BHProcessed, nil, nil)
	assert.Nil(t, err)
	err = bfd.AddHeader(&block.Block{Header: hdr2}, hash2, process.BHProcessed, nil, nil)
	assert.Nil(t, err)

	hInfos := bfd.GetHeaders(1)
	assert.Equal(t, 2, len(hInfos))
	assert.Equal(t, hash1, hInfos[0].Hash())
	assert.Equal(t, hash2, hInfos[1].Hash())
}

func TestMetaForkDetector_AddHeaderWithProcessedBlockShouldSetCheckpoint(t *testing.T) {
	t.Parallel()

	hdr1 := &block.BlockHeader{Nonce: 69, Slot: 72}
	hash1 := []byte("hash1")
	sloterMock := &consensusMock.SlotManagerMock{SlotIndex: 73, TimeDurationCalled: func() time.Duration { return 0 }}
	bfd, _ := sync.NewMetaForkDetector(sloterMock, &mock.BlackListHandlerStub{}, 0)
	_ = bfd.AddHeader(&block.Block{Header: hdr1}, hash1, process.BHProcessed, nil, nil)
	assert.Equal(t, hdr1.Nonce, bfd.LastCheckpointNonce())
}

func TestMetaForkDetector_AddHeaderPresentShouldNotRewriteState(t *testing.T) {
	t.Parallel()

	hdr1 := &block.BlockHeader{Nonce: 1, Slot: 1}
	hash := []byte("hash1")
	hdr2 := &block.BlockHeader{Nonce: 1, Slot: 1}
	sloterMock := &consensusMock.SlotManagerMock{SlotIndex: 1, TimeDurationCalled: func() time.Duration { return 0 }}
	bfd, _ := sync.NewMetaForkDetector(sloterMock, &mock.BlackListHandlerStub{}, 0)

	_ = bfd.AddHeader(&block.Block{Header: hdr1}, hash, process.BHReceived, nil, nil)
	err := bfd.AddHeader(&block.Block{Header: hdr2}, hash, process.BHProcessed, nil, nil)
	assert.Nil(t, err)

	hInfos := bfd.GetHeaders(1)
	assert.Equal(t, 2, len(hInfos))
	assert.Equal(t, hash, hInfos[0].Hash())
	assert.Equal(t, process.BHReceived, hInfos[0].GetBlockHeaderState())
	assert.Equal(t, process.BHProcessed, hInfos[1].GetBlockHeaderState())
}

func TestMetaForkDetector_AddHeaderHigherNonceThanSlotShouldErr(t *testing.T) {
	t.Parallel()

	sloterMock := &consensusMock.SlotManagerMock{SlotIndex: 100}
	bfd, _ := sync.NewMetaForkDetector(sloterMock, &mock.BlackListHandlerStub{}, 0)
	err := bfd.AddHeader(
		&block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: 0}},
		[]byte("hash1"),
		process.BHProcessed,
		nil,
		nil,
	)
	assert.Equal(t, sync.ErrHigherNonceInBlock, err)
}
