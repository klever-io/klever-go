package bls_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/consensus/slot/bls"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/stretchr/testify/assert"
)

func defaultSubslotForSRBlock(consensusState *slot.ConsensusState, ch chan bool,
	container *mock.ConsensusCoreMock) (*slot.Subslot, error) {
	return slot.NewSubslot(
		bls.SrStartSlot,
		bls.SrBlock,
		bls.SrSignature,
		int64(5*slotTimeDuration/100),
		int64(25*slotTimeDuration/100),
		"(BLOCK)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
}

func defaultSubslotBlockFromSubslot(sr *slot.Subslot) (bls.SubslotBlock, error) {
	srBlock, err := bls.NewSubslotBlock(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
	)

	return srBlock, err
}

func defaultSubslotBlockWithoutErrorFromSubslot(sr *slot.Subslot) bls.SubslotBlock {
	srBlock, _ := bls.NewSubslotBlock(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
	)

	return srBlock
}

func initSubslotBlock(blockChain data.ChainHandler, container *mock.ConsensusCoreMock) bls.SubslotBlock {
	if blockChain == nil {
		blockChain = &cMock.BlockChainMock{
			GetCurrentBlockHeaderCalled: func() data.HeaderHandler {
				return &block.Block{Header: &block.BlockHeader{}}
			},
			GetGenesisHeaderCalled: func() data.HeaderHandler {
				return &block.Block{
					Header: &block.BlockHeader{
						Nonce:    uint64(0),
						RandSeed: []byte{0},
					},
					Signature: []byte("genesis signature"),
				}
			},
			GetGenesisHeaderHashCalled: func() []byte {
				return []byte("genesis header hash")
			},
		}
	}

	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	container.SetBlockchain(blockChain)

	sr, _ := defaultSubslotForSRBlock(consensusState, ch, container)
	srBlock, _ := defaultSubslotBlockFromSubslot(sr)
	return srBlock
}

func TestSubslotBlock_NewSubslotBlockNilSubslotShouldFail(t *testing.T) {
	t.Parallel()

	srBlock, err := bls.NewSubslotBlock(
		nil,
		extend,
		bls.ProcessingThresholdPercent,
	)
	assert.Nil(t, srBlock)
	assert.Equal(t, slot.ErrNilSubslot, err)
}

func TestSubslotBlock_NewSubslotBlockNilBlockchainShouldFail(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()

	consensusState := initConsensusState()

	ch := make(chan bool, 1)
	sr, _ := defaultSubslotForSRBlock(consensusState, ch, container)

	container.SetBlockchain(nil)

	srBlock, err := defaultSubslotBlockFromSubslot(sr)
	assert.Nil(t, srBlock)
	assert.Equal(t, slot.ErrNilBlockChain, err)
}

func TestSubslotBlock_NewSubslotBlockNilBlockProcessorShouldFail(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()

	consensusState := initConsensusState()

	ch := make(chan bool, 1)
	sr, _ := defaultSubslotForSRBlock(consensusState, ch, container)

	container.SetBlockProcessor(nil)

	srBlock, err := defaultSubslotBlockFromSubslot(sr)
	assert.Nil(t, srBlock)
	assert.Equal(t, slot.ErrNilBlockProcessor, err)
}

func TestSubslotBlock_NewSubslotBlockNilConsensusStateShouldFail(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	sr, _ := defaultSubslotForSRBlock(consensusState, ch, container)

	sr.ConsensusState = nil

	srBlock, err := defaultSubslotBlockFromSubslot(sr)
	assert.Nil(t, srBlock)
	assert.Equal(t, slot.ErrNilConsensusState, err)
}

func TestSubslotBlock_NewSubslotBlockNilHasherShouldFail(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()

	consensusState := initConsensusState()

	ch := make(chan bool, 1)
	sr, _ := defaultSubslotForSRBlock(consensusState, ch, container)

	container.SetHasher(nil)
	srBlock, err := defaultSubslotBlockFromSubslot(sr)
	assert.Nil(t, srBlock)
	assert.Equal(t, slot.ErrNilHasher, err)
}

func TestSubslotBlock_NewSubslotBlockNilMarshalizerShouldFail(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()

	consensusState := initConsensusState()

	ch := make(chan bool, 1)
	sr, _ := defaultSubslotForSRBlock(consensusState, ch, container)

	container.SetMarshalizer(nil)
	srBlock, err := defaultSubslotBlockFromSubslot(sr)
	assert.Nil(t, srBlock)
	assert.Equal(t, slot.ErrNilMarshalizer, err)
}

func TestSubslotBlock_NewSubslotBlockNilMultisignerShouldFail(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()

	consensusState := initConsensusState()

	ch := make(chan bool, 1)
	sr, _ := defaultSubslotForSRBlock(consensusState, ch, container)

	container.SetMultiSigner(nil)
	srBlock, err := defaultSubslotBlockFromSubslot(sr)
	assert.Nil(t, srBlock)
	assert.Equal(t, slot.ErrNilMultiSigner, err)
}

func TestSubslotBlock_NewSubslotBlockNilSlotManagerShouldFail(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()

	consensusState := initConsensusState()

	ch := make(chan bool, 1)
	sr, _ := defaultSubslotForSRBlock(consensusState, ch, container)

	container.SetSlotManager(nil)
	srBlock, err := defaultSubslotBlockFromSubslot(sr)
	assert.Nil(t, srBlock)
	assert.Equal(t, slot.ErrNilSlotManager, err)
}

func TestSubslotBlock_NewSubslotBlockNilSyncTimerShouldFail(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()

	consensusState := initConsensusState()

	ch := make(chan bool, 1)
	sr, _ := defaultSubslotForSRBlock(consensusState, ch, container)

	container.SetSyncTimer(nil)
	srBlock, err := defaultSubslotBlockFromSubslot(sr)
	assert.Nil(t, srBlock)
	assert.Equal(t, slot.ErrNilSyncTimer, err)
}

func TestSubslotBlock_NewSubslotBlockShouldWork(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)
	sr, _ := defaultSubslotForSRBlock(consensusState, ch, container)
	srBlock, err := defaultSubslotBlockFromSubslot(sr)
	assert.NotNil(t, srBlock)
	assert.Nil(t, err)
}

func TestSubslotBlock_DoBlockJob(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	sr := *initSubslotBlock(nil, container)
	r := sr.DoBlockJob()
	assert.False(t, r)

	sr.SetSelfPubKey(sr.ConsensusGroup()[0])
	_ = sr.SetJobDone(sr.SelfPubKey(), bls.SrBlock, true)
	r = sr.DoBlockJob()
	assert.False(t, r)

	_ = sr.SetJobDone(sr.SelfPubKey(), bls.SrBlock, false)
	sr.SetStatus(bls.SrBlock, slot.SsFinished)
	r = sr.DoBlockJob()
	assert.False(t, r)

	sr.SetStatus(bls.SrBlock, slot.SsNotFinished)
	bpm := &mock.BlockProcessorMock{}
	err := errors.New("error")
	bpm.CreateBlockCalled = func(header data.HeaderHandler, remainingTime func() bool) (data.HeaderHandler, error) {
		return header, err
	}
	container.SetBlockProcessor(bpm)
	r = sr.DoBlockJob()
	assert.False(t, r)

	bpm = mock.InitBlockProcessorMock()
	container.SetBlockProcessor(bpm)
	bm := &mock.BroadcastMessengerMock{
		BroadcastConsensusMessageCalled: func(message *consensus.Message) error {
			return nil
		},
	}
	container.SetBroadcastMessenger(bm)
	container.SetSlotManager(&mock.SlotManagerMock{
		SlotIndex: 1,
	})
	r = sr.DoBlockJob()
	assert.True(t, r)
	assert.Equal(t, uint64(0), sr.Header.GetNonce())
}

func TestSubslotBlock_ReceivedBlock(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	sr := *initSubslotBlock(nil, container)
	blockProcessorMock := mock.InitBlockProcessorMock()
	blkBody := &block.Block{Header: &block.BlockHeader{}}
	blkBodyStr, _ := mock.MarshalizerMock{}.Marshal(blkBody)
	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		blkBodyStr,
		[]byte(sr.ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtBlockBody),
		0,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)

	cnsMsg.PubKey = []byte(sr.ConsensusGroup()[0])
	sr.SetStatus(bls.SrBlock, slot.SsFinished)
	r := sr.ReceivedBlockHeader(cnsMsg)
	assert.False(t, r)

	sr.SetStatus(bls.SrBlock, slot.SsNotFinished)
	r = sr.ReceivedBlockHeader(cnsMsg)
	assert.False(t, r)

	hdr := &block.Block{Header: &block.BlockHeader{Nonce: 2}}
	hdrStr, _ := mock.MarshalizerMock{}.Marshal(hdr)
	hdrHash := cMock.HasherMock{}.Compute(string(hdrStr))
	cnsMsg = consensus.NewConsensusMessage(
		hdrHash,
		nil,
		hdrStr,
		[]byte(sr.ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtBlockHeader),
		0,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	r = sr.ReceivedBlockHeader(cnsMsg)
	assert.False(t, r)

	sr.Data = nil
	sr.Header = hdr
	r = sr.ReceivedBlockHeader(cnsMsg)
	assert.False(t, r)

	sr.Header = nil
	cnsMsg.PubKey = []byte(sr.ConsensusGroup()[1])
	r = sr.ReceivedBlockHeader(cnsMsg)
	assert.False(t, r)

	cnsMsg.PubKey = []byte(sr.ConsensusGroup()[0])
	sr.SetStatus(bls.SrBlock, slot.SsFinished)
	r = sr.ReceivedBlockHeader(cnsMsg)
	assert.False(t, r)

	sr.SetStatus(bls.SrBlock, slot.SsNotFinished)
	container.SetBlockProcessor(blockProcessorMock)
	sr.Data = nil
	sr.Header = nil
	hdr = &block.Block{Header: &block.BlockHeader{Nonce: 1}}
	hdrStr, _ = mock.MarshalizerMock{}.Marshal(hdr)
	hdrHash = cMock.HasherMock{}.Compute(string(hdrStr))
	cnsMsg.BlockHeaderHash = hdrHash
	cnsMsg.Header = hdrStr
	r = sr.ReceivedBlockHeader(cnsMsg)
	assert.True(t, r)
}

func TestSubslotBlock_ProcessReceivedBlockShouldReturnFalseWhenBodyAndHeaderAreNotSet(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	sr := *initSubslotBlock(nil, container)
	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		nil,
		[]byte(sr.ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtBlockBodyAndHeader),
		0,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	assert.False(t, sr.ProcessReceivedBlock(cnsMsg))
}

func TestSubslotBlock_ProcessReceivedBlockShouldReturnFalseWhenProcessBlockFails(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	sr := *initSubslotBlock(nil, container)
	blProcMock := mock.InitBlockProcessorMock()
	err := errors.New("error process block")
	blProcMock.ProcessBlockCalled = func(data.HeaderHandler, func() time.Duration) error {
		return err
	}
	container.SetBlockProcessor(blProcMock)
	hdr := &block.Block{Header: &block.BlockHeader{}}
	blkBodyStr, _ := mock.MarshalizerMock{}.Marshal(hdr)
	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		blkBodyStr,
		[]byte(sr.ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtBlockBody),
		0,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	sr.Header = hdr
	assert.False(t, sr.ProcessReceivedBlock(cnsMsg))
}

func TestSubslotBlock_ProcessReceivedBlockShouldReturnFalseWhenProcessBlockReturnsInNextSlot(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	sr := *initSubslotBlock(nil, container)
	hdr := &block.Block{Header: &block.BlockHeader{}}
	blkBodyStr, _ := mock.MarshalizerMock{}.Marshal(hdr)
	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		blkBodyStr,
		[]byte(sr.ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtBlockBody),
		0,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	sr.Header = hdr
	blockProcessorMock := mock.InitBlockProcessorMock()
	blockProcessorMock.ProcessBlockCalled = func(header data.HeaderHandler, haveTime func() time.Duration) error {
		return errors.New("error")
	}
	container.SetBlockProcessor(blockProcessorMock)
	container.SetSlotManager(&mock.SlotManagerMock{SlotIndex: 1})
	assert.False(t, sr.ProcessReceivedBlock(cnsMsg))
}

func TestSubslotBlock_ProcessReceivedBlockShouldReturnTrue(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	sr := *initSubslotBlock(nil, container)
	hdr := &block.Block{Header: &block.BlockHeader{}}
	blkBodyStr, _ := mock.MarshalizerMock{}.Marshal(hdr)
	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		blkBodyStr,
		[]byte(sr.ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtBlockBody),
		0,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	sr.Header = hdr
	assert.True(t, sr.ProcessReceivedBlock(cnsMsg))
}

func TestSubslotBlock_RemainingTimeShouldReturnNegativeValue(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	slotManagerMock := initSlotManagerMock()
	container.SetSlotManager(slotManagerMock)

	sr := *initSubslotBlock(nil, container)
	remainingTimeInThisSlot := func() time.Duration {
		slotStartTime := sr.SlotManager().Timestamp()
		currentTime := sr.SyncTimer().CurrentTime()
		elapsedTime := currentTime.Sub(slotStartTime)
		remainingTime := sr.SlotManager().TimeDuration()*85/100 - elapsedTime

		return remainingTime
	}
	container.SetSyncTimer(&mock.SyncTimerMock{CurrentTimeCalled: func() time.Time {
		return time.Unix(0, 0).Add(slotTimeDuration * 84 / 100)
	}})
	ret := remainingTimeInThisSlot()
	assert.True(t, ret > 0)

	container.SetSyncTimer(&mock.SyncTimerMock{CurrentTimeCalled: func() time.Time {
		return time.Unix(0, 0).Add(slotTimeDuration * 85 / 100)
	}})
	ret = remainingTimeInThisSlot()
	assert.True(t, ret == 0)

	container.SetSyncTimer(&mock.SyncTimerMock{CurrentTimeCalled: func() time.Time {
		return time.Unix(0, 0).Add(slotTimeDuration * 86 / 100)
	}})
	ret = remainingTimeInThisSlot()
	assert.True(t, ret < 0)
}

func TestSubslotBlock_DoBlockConsensusCheckShouldReturnFalseWhenSlotIsCanceled(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	sr := *initSubslotBlock(nil, container)
	sr.SlotCanceled = true
	assert.False(t, sr.DoBlockConsensusCheck())
}

func TestSubslotBlock_DoBlockConsensusCheckShouldReturnTrueWhenSubslotIsFinished(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	sr := *initSubslotBlock(nil, container)
	sr.SetStatus(bls.SrBlock, slot.SsFinished)
	assert.True(t, sr.DoBlockConsensusCheck())
}

func TestSubslotBlock_DoBlockConsensusCheckShouldReturnTrueWhenBlockIsReceivedReturnTrue(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	sr := *initSubslotBlock(nil, container)
	for i := 0; i < sr.Threshold(bls.SrBlock); i++ {
		_ = sr.SetJobDone(sr.ConsensusGroup()[i], bls.SrBlock, true)
	}
	assert.True(t, sr.DoBlockConsensusCheck())
}

func TestSubslotBlock_DoBlockConsensusCheckShouldReturnFalseWhenBlockIsReceivedReturnFalse(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	sr := *initSubslotBlock(nil, container)
	assert.False(t, sr.DoBlockConsensusCheck())
}

func TestSubslotBlock_IsBlockReceived(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	sr := *initSubslotBlock(nil, container)
	for i := 0; i < len(sr.ConsensusGroup()); i++ {
		_ = sr.SetJobDone(sr.ConsensusGroup()[i], bls.SrBlock, false)
		_ = sr.SetJobDone(sr.ConsensusGroup()[i], bls.SrSignature, false)
	}
	ok := sr.IsBlockReceived(1)
	assert.False(t, ok)

	_ = sr.SetJobDone("A", bls.SrBlock, true)
	isJobDone, _ := sr.JobDone("A", bls.SrBlock)
	assert.True(t, isJobDone)

	ok = sr.IsBlockReceived(1)
	assert.True(t, ok)

	ok = sr.IsBlockReceived(2)
	assert.False(t, ok)
}

func TestSubslotBlock_HaveTimeInCurrentSubslotShouldReturnTrue(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	sr := *initSubslotBlock(nil, container)
	haveTimeInCurrentSubound := func() bool {
		slotStartTime := sr.SlotManager().Timestamp()
		currentTime := sr.SyncTimer().CurrentTime()
		elapsedTime := currentTime.Sub(slotStartTime)
		remainingTime := sr.EndTime() - int64(elapsedTime)

		return time.Duration(remainingTime) > 0
	}
	slotManagerMock := &mock.SlotManagerMock{}
	slotManagerMock.TimeDurationCalled = func() time.Duration {
		return 4000 * time.Millisecond
	}
	slotManagerMock.TimestampCalled = func() time.Time {
		return time.Unix(0, 0)
	}
	syncTimerMock := &mock.SyncTimerMock{}
	timeElapsed := sr.EndTime() - 1
	syncTimerMock.CurrentTimeCalled = func() time.Time {
		return time.Unix(0, timeElapsed)
	}
	container.SetSlotManager(slotManagerMock)
	container.SetSyncTimer(syncTimerMock)

	assert.True(t, haveTimeInCurrentSubound())
}

func TestSubslotBlock_HaveTimeInCurrentSuboundShouldReturnFalse(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	sr := *initSubslotBlock(nil, container)
	haveTimeInCurrentSubound := func() bool {
		slotStartTime := sr.SlotManager().Timestamp()
		currentTime := sr.SyncTimer().CurrentTime()
		elapsedTime := currentTime.Sub(slotStartTime)
		remainingTime := sr.EndTime() - int64(elapsedTime)

		return time.Duration(remainingTime) > 0
	}
	slotManagerMock := &mock.SlotManagerMock{}
	slotManagerMock.TimeDurationCalled = func() time.Duration {
		return 4000 * time.Millisecond
	}
	slotManagerMock.TimestampCalled = func() time.Time {
		return time.Unix(0, 0)
	}
	syncTimerMock := &mock.SyncTimerMock{}
	timeElapsed := sr.EndTime() + 1
	syncTimerMock.CurrentTimeCalled = func() time.Time {
		return time.Unix(0, timeElapsed)
	}
	container.SetSlotManager(slotManagerMock)
	container.SetSyncTimer(syncTimerMock)

	assert.False(t, haveTimeInCurrentSubound())
}

func TestSubslotBlock_CreateHeaderNilCurrentHeader(t *testing.T) {
	blockChain := &cMock.BlockChainMock{
		GetCurrentBlockHeaderCalled: func() data.HeaderHandler {
			return nil
		},
		GetGenesisHeaderCalled: func() data.HeaderHandler {
			return &block.Block{Header: &block.BlockHeader{
				Nonce:    uint64(0),
				RandSeed: []byte{0},
			},
				Signature: []byte("genesis signature"),
			}
		},
		GetGenesisHeaderHashCalled: func() []byte {
			return []byte("genesis header hash")
		},
	}
	container := mock.InitConsensusCore()
	sr := *initSubslotBlock(blockChain, container)
	_ = sr.BlockChain().SetCurrentBlockHeader(nil)
	_ = sr.SetAppStatusHandler(&cMock.AppStatusHandlerStub{})
	header, _ := sr.CreateHeader()
	body, _ := sr.CreateBlock(header)
	marshalizedHeader, _ := sr.Marshalizer().Marshal(body)
	_ = sr.SendBlockHeader(body, make([]byte, 32), marshalizedHeader)

	oldRand := sr.BlockChain().GetGenesisHeader().GetRandSeed()
	newRand, _ := sr.SingleSigner().Sign(sr.PrivateKey(), oldRand)
	expectedHeader := &block.Block{
		Header: &block.BlockHeader{
			Slot:               uint64(sr.SlotManager().Index()),
			Timestamp:          sr.SlotManager().Timestamp().Unix(),
			Nonce:              uint64(1),
			ParentHash:         sr.BlockChain().GetGenesisHeaderHash(),
			PrevRandSeed:       sr.BlockChain().GetGenesisHeader().GetRandSeed(),
			RandSeed:           newRand,
			ChainID:            chainID,
			TxRootHash:         []byte(nil),
			TrieRoot:           []byte(nil),
			ValidatorsTrieRoot: []byte(nil),
			KAppsTrieRoot:      []byte(nil),
		},
	}

	assert.Equal(t, expectedHeader, header)
}

func TestSubslotBlock_CreateHeaderNotNilCurrentHeader(t *testing.T) {
	container := mock.InitConsensusCore()
	sr := *initSubslotBlock(nil, container)
	_ = sr.BlockChain().SetCurrentBlockHeader(&block.Block{Header: &block.BlockHeader{Nonce: 1}})

	header, _ := sr.CreateHeader()
	body, _ := sr.CreateBlock(header)
	marshalizedHeader, _ := sr.Marshalizer().Marshal(body)
	_ = sr.SendBlockHeader(body, make([]byte, 32), marshalizedHeader)

	oldRand := sr.BlockChain().GetGenesisHeader().GetRandSeed()
	newRand, _ := sr.SingleSigner().Sign(sr.PrivateKey(), oldRand)

	expectedHeader := &block.Block{
		Header: &block.BlockHeader{
			Slot:               uint64(sr.SlotManager().Index()),
			Timestamp:          sr.SlotManager().Timestamp().Unix(),
			Nonce:              sr.BlockChain().GetCurrentBlockHeader().GetNonce() + 1,
			ParentHash:         sr.BlockChain().GetCurrentBlockHeaderHash(),
			RandSeed:           newRand,
			ChainID:            chainID,
			TxRootHash:         []byte(nil),
			TrieRoot:           []byte(nil),
			ValidatorsTrieRoot: []byte(nil),
			KAppsTrieRoot:      []byte(nil),
		},
	}

	assert.Equal(t, expectedHeader, header)
}

func TestSubslotBlock_CallFuncRemainingTimeWithStructShouldWork(t *testing.T) {
	slotStartTime := time.Now()
	maxTime := 100 * time.Millisecond
	newSlotStartTime := slotStartTime
	remainingTimeInCurrentSlot := func() time.Duration {
		return RemainingTimeWithStruct(newSlotStartTime, maxTime)
	}
	assert.True(t, remainingTimeInCurrentSlot() > 0)

	time.Sleep(200 * time.Millisecond)
	assert.True(t, remainingTimeInCurrentSlot() < 0)
}

func TestSubslotBlock_CallFuncRemainingTimeWithStructShouldNotWork(t *testing.T) {
	slotStartTime := time.Now()
	maxTime := 100 * time.Millisecond
	remainingTimeInCurrentSlot := func() time.Duration {
		return RemainingTimeWithStruct(slotStartTime, maxTime)
	}
	assert.True(t, remainingTimeInCurrentSlot() > 0)

	time.Sleep(200 * time.Millisecond)
	assert.True(t, remainingTimeInCurrentSlot() < 0)

	slotStartTime = slotStartTime.Add(500 * time.Millisecond)
	assert.False(t, remainingTimeInCurrentSlot() < 0)
}

func RemainingTimeWithStruct(startTime time.Time, maxTime time.Duration) time.Duration {
	currentTime := time.Now()
	elapsedTime := currentTime.Sub(startTime)
	remainingTime := maxTime - elapsedTime
	return remainingTime
}

func TestSubslotBlock_ReceivedBlockComputeProcessDuration(t *testing.T) {
	t.Parallel()

	srStartTime := int64(5 * slotTimeDuration / 100)
	srEndTime := int64(25 * slotTimeDuration / 100)
	srDuration := srEndTime - srStartTime
	delay := srDuration * 430 / 1000

	container := mock.InitConsensusCore()
	container.SetBlockProcessor(&mock.BlockProcessorMock{
		ProcessBlockCalled: func(_ data.HeaderHandler, _ func() time.Duration) error {
			time.Sleep(time.Duration(delay))
			return nil
		},
	})
	sr := *initSubslotBlock(nil, container)
	hdr := &block.Block{Header: &block.BlockHeader{}}
	blkBodyStr, _ := mock.MarshalizerMock{}.Marshal(hdr)

	cnsMsg := consensus.NewConsensusMessage(
		nil,
		nil,
		blkBodyStr,
		[]byte(sr.ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtBlockBody),
		0,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)
	sr.Header = hdr
	receivedValue := uint64(0)
	_ = sr.SetAppStatusHandler(&cMock.AppStatusHandlerStub{
		SetUInt64ValueHandler: func(key string, value uint64) {
			receivedValue = value
		},
	})

	minimumExpectedValue := uint64(delay * 100 / srDuration)
	_ = sr.ProcessReceivedBlock(cnsMsg)

	assert.True(t,
		receivedValue >= minimumExpectedValue,
		fmt.Sprintf("minimum expected was %d, got %d", minimumExpectedValue, receivedValue),
	)
}

func TestSubslotBlock_ReceivedBlockComputeProcessDurationWithZeroDurationShouldNotPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		r := recover()
		if r != nil {
			assert.Fail(t, "should not have paniced", r)
		}
	}()

	container := mock.InitConsensusCore()

	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := defaultSubslotForSRBlock(consensusState, ch, container)
	srBlock := *defaultSubslotBlockWithoutErrorFromSubslot(sr)

	srBlock.ComputeSubslotProcessingMetric(time.Now(), "dummy")
}
