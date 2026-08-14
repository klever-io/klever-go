package sync_test

import (
	"bytes"
	"math"
	"testing"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common/mock"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/sync"
	"github.com/klever-io/klever-go/data/block"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// isConsensusStuck derives its slot lag the same way the bootstrapper does, and
// on this path the wrapped value does not merely trigger a wasted header request:
// it produces the ForkInfo that isForcedRollBackOneBlock matches, so the node
// reverts a block it just committed. checkBlockBasicValidity deliberately accepts
// headers one slot ahead of the local index, so a node whose clock trails its
// peers reaches this state without any peer misbehaving.
func TestMetaForkDetector_CheckForkNoForcedRollBackWhenCheckpointIsAheadOfSlotIndex(t *testing.T) {
	t.Parallel()

	// A multiple of process.SlotModulusTrigger, so IsInProperSlot holds and only
	// the underflow guard can keep the forced rollback from firing.
	const laggingSlotIndex = int64(50)
	const checkpointSlot = uint64(100)

	sloterMock := &consensusMock.SlotManagerMock{
		SlotIndex:          int64(checkpointSlot),
		TimeDurationCalled: func() time.Duration { return 0 },
	}
	bfd, err := sync.NewMetaForkDetector(sloterMock, &mock.BlackListHandlerStub{}, 0)
	assert.Nil(t, err)

	hdr := &block.Block{Header: &block.BlockHeader{Nonce: 5, Slot: checkpointSlot}}
	err = bfd.AddHeader(hdr, []byte("hash"), process.BHProcessed, nil, nil)
	assert.Nil(t, err)
	assert.Equal(t, checkpointSlot, bfd.LastCheckpointSlot())

	// The local slot index now trails the committed block's slot.
	sloterMock.SlotIndex = laggingSlotIndex

	forkInfo := bfd.CheckFork()

	assert.False(t, forkInfo.IsDetected)
}

// The counterpart of the guard above: with the checkpoint genuinely behind the
// slot index, the consensus-stuck detection must still fire. Without this the
// guard could disable the mechanism entirely and no test would notice.
func TestMetaForkDetector_CheckForkStillDetectsStuckConsensusWhenCheckpointIsBehind(t *testing.T) {
	t.Parallel()

	// A multiple of process.SlotModulusTrigger, with a lag well past
	// process.MaxSlotsWithoutCommittedBlock.
	const checkpointSlot = uint64(1)
	const laggingSlotIndex = int64(50)

	sloterMock := &consensusMock.SlotManagerMock{
		SlotIndex:          int64(checkpointSlot),
		TimeDurationCalled: func() time.Duration { return 0 },
	}
	bfd, err := sync.NewMetaForkDetector(sloterMock, &mock.BlackListHandlerStub{}, 0)
	assert.Nil(t, err)

	hdr := &block.Block{Header: &block.BlockHeader{Nonce: 1, Slot: checkpointSlot}}
	err = bfd.AddHeader(hdr, []byte("hash"), process.BHProcessed, nil, nil)
	assert.Nil(t, err)

	sloterMock.SlotIndex = laggingSlotIndex

	forkInfo := bfd.CheckFork()

	assert.True(t, forkInfo.IsDetected)
	// The shape isForcedRollBackOneBlock matches.
	assert.Equal(t, uint64(math.MaxUint64), forkInfo.Nonce)
	assert.Nil(t, forkInfo.Hash)
}

type checkpointAheadWarnFormatter struct {
	logger.PlainFormatter
}

func (f *checkpointAheadWarnFormatter) Output(line logger.LogLineHandler) []byte {
	if line.GetMessage() != "last checkpoint is ahead of the local slot index, node clock appears to trail the network" {
		return nil
	}

	return f.PlainFormatter.Output(line)
}

// In the clock-trails-tip state this warning is the node's only signal, but
// CheckFork can run on every 5 ms sync-loop iteration while the node is not
// synchronized, so it must be throttled to once per slot or it becomes hundreds
// of lines per second. Not parallel: it registers a global log observer.
func TestMetaForkDetector_CheckpointAheadWarnsOncePerSlot(t *testing.T) {
	buff := &bytes.Buffer{}
	require.Nil(t, logger.AddLogObserver(buff, &checkpointAheadWarnFormatter{}))
	t.Cleanup(func() {
		require.Nil(t, logger.RemoveLogObserver(buff))
	})

	const checkpointSlot = uint64(100)

	sloterMock := &consensusMock.SlotManagerMock{
		SlotIndex:          int64(checkpointSlot),
		TimeDurationCalled: func() time.Duration { return 0 },
	}
	bfd, err := sync.NewMetaForkDetector(sloterMock, &mock.BlackListHandlerStub{}, 0)
	require.Nil(t, err)

	hdr := &block.Block{Header: &block.BlockHeader{Nonce: 5, Slot: checkpointSlot}}
	require.Nil(t, bfd.AddHeader(hdr, []byte("hash"), process.BHProcessed, nil, nil))

	sloterMock.SlotIndex = 50
	forkInfo := bfd.CheckFork()
	require.False(t, forkInfo.IsDetected)

	firstLen := buff.Len()
	require.Greater(t, firstLen, 0)
	require.Contains(t, buff.String(), "last checkpoint is ahead of the local slot index")

	// Same slot: throttled.
	_ = bfd.CheckFork()
	require.Equal(t, firstLen, buff.Len())

	// Next slot: the state persists, so it warns again.
	sloterMock.SlotIndex = 51
	_ = bfd.CheckFork()
	require.Greater(t, buff.Len(), firstLen)
}
