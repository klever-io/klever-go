package bls_test

import (
	"errors"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/consensus/slot/bls"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/blockchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initSubslotEndSlotWithContainer(container *mock.ConsensusCoreMock) bls.SubslotEndSlot {
	ch := make(chan bool, 1)
	consensusState := initConsensusState()
	sr, _ := slot.NewSubslot(
		bls.SrSignature,
		bls.SrEndSlot,
		-1,
		int64(85*slotTimeDuration/100),
		int64(95*slotTimeDuration/100),
		"(END_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	srEndSlot, _ := bls.NewSubslotEndSlot(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
		displayStatistics,
	)

	return srEndSlot
}

func initSubslotEndSlot() bls.SubslotEndSlot {
	container := mock.InitConsensusCore()
	return initSubslotEndSlotWithContainer(container)
}

func TestSubslotEndSlot_NewSubslotEndSlotNilSubslotShouldFail(t *testing.T) {
	t.Parallel()
	srEndSlot, err := bls.NewSubslotEndSlot(
		nil,
		extend,
		bls.ProcessingThresholdPercent,
		displayStatistics,
	)

	assert.Nil(t, srEndSlot)
	assert.Equal(t, slot.ErrNilSubslot, err)
}

func TestSubslotEndSlot_NewSubslotEndSlotNilBlockChainShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := slot.NewSubslot(
		bls.SrSignature,
		bls.SrEndSlot,
		-1,
		int64(85*slotTimeDuration/100),
		int64(95*slotTimeDuration/100),
		"(END_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	container.SetBlockchain(nil)
	srEndSlot, err := bls.NewSubslotEndSlot(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
		displayStatistics,
	)

	assert.Nil(t, srEndSlot)
	assert.Equal(t, slot.ErrNilBlockChain, err)
}

func TestSubslotEndSlot_NewSubslotEndSlotNilBlockProcessorShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := slot.NewSubslot(
		bls.SrSignature,
		bls.SrEndSlot,
		-1,
		int64(85*slotTimeDuration/100),
		int64(95*slotTimeDuration/100),
		"(END_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	container.SetBlockProcessor(nil)
	srEndSlot, err := bls.NewSubslotEndSlot(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
		displayStatistics,
	)

	assert.Nil(t, srEndSlot)
	assert.Equal(t, slot.ErrNilBlockProcessor, err)
}

func TestSubslotEndSlot_NewSubslotEndSlotNilConsensusStateShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := slot.NewSubslot(
		bls.SrSignature,
		bls.SrEndSlot,
		-1,
		int64(85*slotTimeDuration/100),
		int64(95*slotTimeDuration/100),
		"(END_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	sr.ConsensusState = nil
	srEndSlot, err := bls.NewSubslotEndSlot(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
		displayStatistics,
	)

	assert.Nil(t, srEndSlot)
	assert.Equal(t, slot.ErrNilConsensusState, err)
}

func TestSubslotEndSlot_NewSubslotEndSlotNilMultisignerShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := slot.NewSubslot(
		bls.SrSignature,
		bls.SrEndSlot,
		-1,
		int64(85*slotTimeDuration/100),
		int64(95*slotTimeDuration/100),
		"(END_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	container.SetMultiSigner(nil)
	srEndSlot, err := bls.NewSubslotEndSlot(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
		displayStatistics,
	)

	assert.Nil(t, srEndSlot)
	assert.Equal(t, slot.ErrNilMultiSigner, err)
}

func TestSubslotEndSlot_NewSubslotEndSlotNilSlotManagerShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := slot.NewSubslot(
		bls.SrSignature,
		bls.SrEndSlot,
		-1,
		int64(85*slotTimeDuration/100),
		int64(95*slotTimeDuration/100),
		"(END_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	container.SetSlotManager(nil)
	srEndSlot, err := bls.NewSubslotEndSlot(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
		displayStatistics,
	)

	assert.Nil(t, srEndSlot)
	assert.Equal(t, slot.ErrNilSlotManager, err)
}

func TestSubslotEndSlot_NewSubslotEndSlotNilSyncTimerShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := slot.NewSubslot(
		bls.SrSignature,
		bls.SrEndSlot,
		-1,
		int64(85*slotTimeDuration/100),
		int64(95*slotTimeDuration/100),
		"(END_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	container.SetSyncTimer(nil)
	srEndSlot, err := bls.NewSubslotEndSlot(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
		displayStatistics,
	)

	assert.Nil(t, srEndSlot)
	assert.Equal(t, slot.ErrNilSyncTimer, err)
}

func TestSubslotEndSlot_NewSubslotEndSlotShouldWork(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := slot.NewSubslot(
		bls.SrSignature,
		bls.SrEndSlot,
		-1,
		int64(85*slotTimeDuration/100),
		int64(95*slotTimeDuration/100),
		"(END_SLOT)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	srEndSlot, err := bls.NewSubslotEndSlot(
		sr,
		extend,
		bls.ProcessingThresholdPercent,
		displayStatistics,
	)

	assert.NotNil(t, srEndSlot)
	assert.Nil(t, err)
}

func TestSubslotEndSlot_SetAppStatusHandlerNilAshShouldErr(t *testing.T) {
	t.Parallel()

	sr := *initSubslotEndSlot()

	err := sr.SetAppStatusHandler(nil)
	assert.Equal(t, slot.ErrNilAppStatusHandler, err)
}

func TestSubslotEndSlot_SetAppStatusHandlerOkAshShouldWork(t *testing.T) {
	t.Parallel()

	sr := *initSubslotEndSlot()

	err := sr.SetAppStatusHandler(&cMock.AppStatusHandlerStub{})
	assert.Nil(t, err)
}

func TestSubslotEndSlot_DoEndSlotJobErrAggregatingSigShouldFail(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	sr := *initSubslotEndSlotWithContainer(container)
	multiSignerMock := mock.InitMultiSignerMock()
	multiSignerMock.AggregateSigsMock = func(bitmap []byte) ([]byte, error) {
		return nil, crypto.ErrNilHasher
	}

	container.SetMultiSigner(multiSignerMock)
	sr.Header = &block.Block{Header: &block.BlockHeader{}}

	sr.SetSelfPubKey("A")

	assert.True(t, sr.IsSelfLeaderInCurrentSlot())
	r := sr.DoEndSlotJob()
	assert.False(t, r)
}

func TestSubslotEndSlot_DoEndSlotJobErrCommitBlockShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	sr := *initSubslotEndSlotWithContainer(container)
	sr.SetSelfPubKey("A")

	blProcMock := mock.InitBlockProcessorMock()
	blProcMock.CommitBlockCalled = func(
		header data.HeaderHandler,
	) error {
		return blockchain.ErrHeaderUnitNil
	}

	container.SetBlockProcessor(blProcMock)
	sr.Header = &block.Block{Header: &block.BlockHeader{}}

	r := sr.DoEndSlotJob()
	assert.False(t, r)
}

func TestSubslotEndSlot_DoEndSlotJobErrBroadcastBlockOK(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	expectedSignature := []byte("signature")
	singleSigner := &cMock.SingleSignerMock{
		SignStub: func(private crypto.PrivateKey, msg []byte) ([]byte, error) {
			var receivedHdr block.Block
			err := container.Marshalizer().Unmarshal(&receivedHdr, msg)
			if err != nil {
				return nil, err
			}

			return expectedSignature, nil
		},
	}
	container.SetSingleSigner(singleSigner)
	bm := &mock.BroadcastMessengerMock{
		BroadcastBlockCalled: func(handler data.HeaderHandler) error {
			return errors.New("error")
		},
	}
	container.SetBroadcastMessenger(bm)
	sr := *initSubslotEndSlotWithContainer(container)
	sr.SetSelfPubKey("A")

	sr.Header = &block.Block{Header: &block.BlockHeader{}}

	r := sr.DoEndSlotJob()
	assert.True(t, r)
}

func TestSubslotEndSlot_DoEndSlotJobAllOK(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	expectedSignature := []byte("signature")
	singleSigner := &cMock.SingleSignerMock{
		SignStub: func(private crypto.PrivateKey, msg []byte) ([]byte, error) {
			var receivedHdr block.Block
			err := container.Marshalizer().Unmarshal(&receivedHdr, msg)
			if err != nil {
				return nil, err
			}

			return expectedSignature, nil
		},
	}
	container.SetSingleSigner(singleSigner)

	bm := &mock.BroadcastMessengerMock{
		BroadcastBlockCalled: func(handler data.HeaderHandler) error {
			return errors.New("error")
		},
	}
	container.SetBroadcastMessenger(bm)
	sr := *initSubslotEndSlotWithContainer(container)
	sr.SetSelfPubKey("A")

	sr.Header = &block.Block{Header: &block.BlockHeader{}}

	r := sr.DoEndSlotJob()
	assert.True(t, r)
}

func TestSubslotEndSlot_CheckIfSignatureIsFilled(t *testing.T) {
	t.Parallel()

	expectedSignature := []byte("signature")
	container := mock.InitConsensusCore()
	singleSigner := &cMock.SingleSignerMock{
		SignStub: func(private crypto.PrivateKey, msg []byte) ([]byte, error) {
			var receivedHdr block.Block
			err := container.Marshalizer().Unmarshal(&receivedHdr, msg)
			if err != nil {
				return nil, err
			}

			return expectedSignature, nil
		},
	}
	container.SetSingleSigner(singleSigner)
	bm := &mock.BroadcastMessengerMock{
		BroadcastBlockCalled: func(handler data.HeaderHandler) error {
			return errors.New("error")
		},
	}
	container.SetBroadcastMessenger(bm)
	sr := *initSubslotEndSlotWithContainer(container)
	sr.SetSelfPubKey("A")

	sr.Header = &block.Block{Header: &block.BlockHeader{Nonce: 5}}

	r := sr.DoEndSlotJob()
	assert.True(t, r)
	assert.Equal(t, expectedSignature, sr.Header.GetProducerSignature())
}

func TestSubslotEndSlot_DoEndSlotConsensusCheckShouldReturnFalseWhenSlotIsCanceled(t *testing.T) {
	t.Parallel()

	sr := *initSubslotEndSlot()
	sr.SlotCanceled = true

	ok := sr.DoEndSlotConsensusCheck()
	assert.False(t, ok)
}

func TestSubslotEndSlot_DoEndSlotConsensusCheckShouldReturnTrueWhenSlotIsFinished(t *testing.T) {
	t.Parallel()

	sr := *initSubslotEndSlot()
	sr.SetStatus(bls.SrEndSlot, slot.SsFinished)

	ok := sr.DoEndSlotConsensusCheck()
	assert.True(t, ok)
}

func TestSubslotEndSlot_DoEndSlotConsensusCheckShouldReturnFalseWhenSlotIsNotFinished(t *testing.T) {
	t.Parallel()

	sr := *initSubslotEndSlot()

	ok := sr.DoEndSlotConsensusCheck()
	assert.False(t, ok)
}

func TestSubslotEndSlot_CheckSignaturesValidityShouldErrNilSignature(t *testing.T) {
	t.Parallel()

	sr := *initSubslotEndSlot()

	err := sr.CheckSignaturesValidity([]byte{2})
	assert.Equal(t, common.ErrNilSignature, err)
}

func TestSubslotEndSlot_CheckSignaturesValidityShouldErrIndexOutOfBounds(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	sr := *initSubslotEndSlotWithContainer(container)
	_, _ = sr.MultiSigner().Create(nil, 0)
	_ = sr.SetJobDone(sr.ConsensusGroup()[0], bls.SrSignature, true)

	multiSignerMock := mock.InitMultiSignerMock()
	multiSignerMock.SignatureShareMock = func(index uint16) ([]byte, error) {
		return nil, crypto.ErrIndexOutOfBounds
	}
	container.SetMultiSigner(multiSignerMock)

	err := sr.CheckSignaturesValidity([]byte{1})
	assert.Equal(t, crypto.ErrIndexOutOfBounds, err)
}

func TestSubslotEndSlot_CheckSignaturesValidityShouldErrInvalidSignatureShare(t *testing.T) {
	t.Parallel()
	container := mock.InitConsensusCore()
	sr := *initSubslotEndSlotWithContainer(container)
	multiSignerMock := mock.InitMultiSignerMock()
	err := errors.New("invalid signature share")
	multiSignerMock.VerifySignatureShareMock = func(index uint16, sig []byte, msg []byte) error {
		return err
	}
	container.SetMultiSigner(multiSignerMock)

	_ = sr.SetJobDone(sr.ConsensusGroup()[0], bls.SrSignature, true)

	err2 := sr.CheckSignaturesValidity([]byte{1})
	assert.Equal(t, err, err2)
}

func TestSubslotEndSlot_CheckSignaturesValidityShouldReturnNil(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	sr := *initSubslotEndSlotWithContainer(container)

	_ = sr.SetJobDone(sr.ConsensusGroup()[0], bls.SrSignature, true)

	err := sr.CheckSignaturesValidity([]byte{1})
	assert.Equal(t, nil, err)
}

func TestSubslotEndSlot_DoEndSlotJobByParticipant_SlotCanceledShouldReturnFalse(t *testing.T) {
	t.Parallel()

	sr := *initSubslotEndSlot()
	sr.SlotCanceled = true

	cnsData := consensus.Message{}
	res := sr.DoEndSlotJobByParticipant(&cnsData)
	assert.False(t, res)
}

func TestSubslotEndSlot_DoEndSlotJobByParticipant_ConsensusDataNotSetShouldReturnFalse(t *testing.T) {
	t.Parallel()

	sr := *initSubslotEndSlot()
	sr.Data = nil

	cnsData := consensus.Message{}
	res := sr.DoEndSlotJobByParticipant(&cnsData)
	assert.False(t, res)
}

func TestSubslotEndSlot_DoEndSlotJobByParticipant_PreviousSubslotNotFinishedShouldReturnFalse(t *testing.T) {
	t.Parallel()

	sr := *initSubslotEndSlot()
	sr.SetStatus(2, slot.SsNotFinished)
	cnsData := consensus.Message{}
	res := sr.DoEndSlotJobByParticipant(&cnsData)
	assert.False(t, res)
}

func TestSubslotEndSlot_DoEndSlotJobByParticipant_CurrentSubslotFinishedShouldReturnFalse(t *testing.T) {
	t.Parallel()

	sr := *initSubslotEndSlot()

	// set previous as finished
	sr.SetStatus(2, slot.SsFinished)

	// set current as finished
	sr.SetStatus(3, slot.SsFinished)

	cnsData := consensus.Message{}
	res := sr.DoEndSlotJobByParticipant(&cnsData)
	assert.False(t, res)
}

func TestSubslotEndSlot_DoEndSlotJobByParticipant_ConsensusHeaderNotReceivedShouldReturnFalse(t *testing.T) {
	t.Parallel()

	sr := *initSubslotEndSlot()

	// set previous as finished
	sr.SetStatus(2, slot.SsFinished)

	// set current as not finished
	sr.SetStatus(3, slot.SsNotFinished)

	cnsData := consensus.Message{}
	res := sr.DoEndSlotJobByParticipant(&cnsData)
	assert.False(t, res)
}

func TestSubslotEndSlot_DoEndSlotJobByParticipant_ShouldReturnTrue(t *testing.T) {
	t.Parallel()

	hdr := &block.Block{Header: &block.BlockHeader{Nonce: 37}}
	sr := *initSubslotEndSlot()
	sr.Header = hdr
	sr.AddReceivedHeader(hdr)

	// set previous as finished
	sr.SetStatus(2, slot.SsFinished)

	// set current as not finished
	sr.SetStatus(3, slot.SsNotFinished)

	cnsData := consensus.Message{}
	res := sr.DoEndSlotJobByParticipant(&cnsData)
	assert.True(t, res)
}

func TestSubslotEndSlot_Validator_DoEndSlotJobAllOK(t *testing.T) {

	container := mock.InitConsensusCore()
	expectedSignature := []byte("signature")
	singleSigner := &cMock.SingleSignerMock{
		SignStub: func(private crypto.PrivateKey, msg []byte) ([]byte, error) {
			var receivedHdr block.Block
			err := container.Marshalizer().Unmarshal(&receivedHdr, msg)
			if err != nil {
				return nil, err
			}

			return expectedSignature, nil
		},
	}
	container.SetSingleSigner(singleSigner)

	bm := &mock.BroadcastMessengerMock{
		BroadcastBlockCalled: func(handler data.HeaderHandler) error {
			return errors.New("error")
		},
	}
	container.SetBroadcastMessenger(bm)
	sr := *initSubslotEndSlotWithContainer(container)

	hdr := &block.Block{Header: &block.BlockHeader{Nonce: 37}}

	sr.Header = hdr
	sr.AddReceivedHeader(hdr)

	// set previous as finished
	sr.SetStatus(2, slot.SsFinished)

	// set current as not finished
	sr.SetStatus(3, slot.SsNotFinished)

	// set validator in consensus group
	sr.SetSelfPubKey("B")

	r := sr.DoEndSlotJob()
	assert.True(t, r)
}

func TestSubslotEndSlot_IsConsensusHeaderReceived_NoReceivedHeadersShouldReturnFalse(t *testing.T) {
	t.Parallel()

	hdr := &block.Block{Header: &block.BlockHeader{Nonce: 37}}
	sr := *initSubslotEndSlot()
	sr.Header = hdr

	res, retHdr := sr.IsConsensusHeaderReceived()
	assert.False(t, res)
	assert.Nil(t, retHdr)
}

func TestSubslotEndSlot_IsConsensusHeaderReceived_HeaderNotReceivedShouldReturnFalse(t *testing.T) {
	t.Parallel()

	hdr := &block.Block{Header: &block.BlockHeader{Nonce: 37}}
	hdrToSearchFor := &block.Block{Header: &block.BlockHeader{Nonce: 38}}
	sr := *initSubslotEndSlot()
	sr.AddReceivedHeader(hdr)
	sr.Header = hdrToSearchFor

	res, retHdr := sr.IsConsensusHeaderReceived()
	assert.False(t, res)
	assert.Nil(t, retHdr)
}

func TestSubslotEndSlot_IsConsensusHeaderReceivedShouldReturnTrue(t *testing.T) {
	t.Parallel()

	hdr := &block.Block{Header: &block.BlockHeader{Nonce: 37}}
	sr := *initSubslotEndSlot()
	sr.Header = hdr
	sr.AddReceivedHeader(hdr)

	res, retHdr := sr.IsConsensusHeaderReceived()
	assert.True(t, res)
	assert.Equal(t, hdr, retHdr)
}

func TestSubslotEndSlot_HaveConsensusHeaderWithFullInfoNilHdrShouldNotWork(t *testing.T) {
	t.Parallel()

	sr := *initSubslotEndSlot()

	cnsData := consensus.Message{}

	haveHdr, hdr := sr.HaveConsensusHeaderWithFullInfo(&cnsData)
	assert.False(t, haveHdr)
	assert.Nil(t, hdr)
}

func TestSubslotEndSlot_HaveConsensusHeaderWithFullInfoShouldWork(t *testing.T) {
	t.Parallel()

	originalPubKeyBitMap := []byte{0, 1, 2}
	newPubKeyBitMap := []byte{3, 4, 5}
	originalLeaderSig := []byte{6, 7, 8}
	newLeaderSig := []byte{9, 10, 11}
	originalSig := []byte{12, 13, 14}
	newSig := []byte{15, 16, 17}
	hdr := block.Block{Header: &block.BlockHeader{},
		PubKeysBitmap:     originalPubKeyBitMap,
		Signature:         originalSig,
		ProducerSignature: originalLeaderSig,
	}
	sr := *initSubslotEndSlot()
	sr.Header = &hdr

	cnsData := consensus.Message{
		PubKeysBitmap:      newPubKeyBitMap,
		LeaderSignature:    newLeaderSig,
		AggregateSignature: newSig,
	}
	haveHdr, newHdr := sr.HaveConsensusHeaderWithFullInfo(&cnsData)
	assert.True(t, haveHdr)
	require.NotNil(t, newHdr)
	assert.Equal(t, newPubKeyBitMap, newHdr.GetPubKeysBitmap())
	assert.Equal(t, newLeaderSig, newHdr.GetProducerSignature())
	assert.Equal(t, newSig, newHdr.GetSignature())
}

func TestSubslotEndSlot_CreateAndBroadcastHeaderFinalInfoBroadcastShouldBeCalled(t *testing.T) {
	t.Parallel()

	chanRcv := make(chan bool, 1)
	leaderSigInHdr := []byte("leader sig")
	container := mock.InitConsensusCore()
	messenger := &mock.BroadcastMessengerMock{
		BroadcastConsensusMessageCalled: func(message *consensus.Message) error {
			chanRcv <- true
			assert.Equal(t, message.LeaderSignature, leaderSigInHdr)
			return nil
		},
	}
	container.SetBroadcastMessenger(messenger)
	sr := *initSubslotEndSlotWithContainer(container)
	sr.Header = &block.Block{ProducerSignature: leaderSigInHdr}

	sr.CreateAndBroadcastHeaderFinalInfo()

	select {
	case <-chanRcv:
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "broadcast not called")
	}
}

func TestSubslotEndSlot_ReceivedBlockHeaderFinalInfoShouldWork(t *testing.T) {
	t.Parallel()

	hdr := &block.Block{Header: &block.BlockHeader{Nonce: 37}}
	sr := *initSubslotEndSlot()
	sr.Header = hdr
	sr.AddReceivedHeader(hdr)

	sr.SetStatus(2, slot.SsFinished)
	sr.SetStatus(3, slot.SsNotFinished)

	cnsData := consensus.Message{
		// apply the data which is mocked in consensus state so the checks will pass
		BlockHeaderHash: []byte("X"),
		PubKey:          []byte("A"),
	}
	res := sr.ReceivedBlockHeaderFinalInfo(&cnsData)
	assert.True(t, res)
}

func TestSubslotEndSlot_ReceivedBlockHeaderFinalInfoShouldReturnFalseWhenFinalInfoIsNotValid(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()

	headerSigVerifier := &mock.HeaderSigVerifierStub{
		VerifyLeaderSignatureCalled: func(header data.HeaderHandler) error {
			return errors.New("error")
		},
		VerifySignatureCalled: func(header data.HeaderHandler) error {
			return errors.New("error")
		},
	}

	container.SetHeaderSigVerifier(headerSigVerifier)
	sr := *initSubslotEndSlotWithContainer(container)
	cnsData := consensus.Message{
		BlockHeaderHash: []byte("X"),
		PubKey:          []byte("A"),
	}
	sr.Header = &block.Block{Header: &block.BlockHeader{}}
	res := sr.ReceivedBlockHeaderFinalInfo(&cnsData)
	assert.False(t, res)
}

func TestSubslotEndSlot_IsOutOfTimeShouldReturnFalse(t *testing.T) {
	t.Parallel()

	sr := *initSubslotEndSlot()

	res := sr.IsOutOfTime()
	assert.False(t, res)
}

func TestSubslotEndSlot_IsOutOfTimeShouldReturnTrue(t *testing.T) {
	t.Parallel()

	// update sloter's mock so it will calculate for real the duration
	container := mock.InitConsensusCore()
	slotManager := mock.SlotManagerMock{RemainingTimeCalled: func(startTime time.Time, maxTime time.Duration) time.Duration {
		currentTime := time.Now()
		elapsedTime := currentTime.Sub(startTime)
		remainingTime := maxTime - elapsedTime

		return remainingTime
	}}
	container.SetSlotManager(&slotManager)
	sr := *initSubslotEndSlotWithContainer(container)

	sr.SlotTimestamp = time.Now().AddDate(0, 0, -1)

	res := sr.IsOutOfTime()
	assert.True(t, res)
}

func TestSubslotEndSlot_IsBlockHeaderFinalInfoValidShouldReturnFalseWhenVerifyLeaderSignatureFails(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()

	headerSigVerifier := &mock.HeaderSigVerifierStub{
		VerifyLeaderSignatureCalled: func(header data.HeaderHandler) error {
			return errors.New("error")
		},
		VerifySignatureCalled: func(header data.HeaderHandler) error {
			return nil
		},
	}

	container.SetHeaderSigVerifier(headerSigVerifier)
	sr := *initSubslotEndSlotWithContainer(container)
	cnsDta := &consensus.Message{}
	sr.Header = &block.Block{Header: &block.BlockHeader{}}
	isValid := sr.IsBlockHeaderFinalInfoValid(cnsDta)
	assert.False(t, isValid)
}

func TestSubslotEndSlot_IsBlockHeaderFinalInfoValidShouldReturnFalseWhenVerifySignatureFails(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()

	headerSigVerifier := &mock.HeaderSigVerifierStub{
		VerifyLeaderSignatureCalled: func(header data.HeaderHandler) error {
			return nil
		},
		VerifySignatureCalled: func(header data.HeaderHandler) error {
			return errors.New("error")
		},
	}

	container.SetHeaderSigVerifier(headerSigVerifier)
	sr := *initSubslotEndSlotWithContainer(container)
	cnsDta := &consensus.Message{}
	sr.Header = &block.Block{Header: &block.BlockHeader{}}
	isValid := sr.IsBlockHeaderFinalInfoValid(cnsDta)
	assert.False(t, isValid)
}

func TestSubslotEndSlot_IsBlockHeaderFinalInfoValidShouldReturnTrue(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()

	headerSigVerifier := &mock.HeaderSigVerifierStub{
		VerifyLeaderSignatureCalled: func(header data.HeaderHandler) error {
			return nil
		},
		VerifySignatureCalled: func(header data.HeaderHandler) error {
			return nil
		},
	}

	container.SetHeaderSigVerifier(headerSigVerifier)
	sr := *initSubslotEndSlotWithContainer(container)
	cnsDta := &consensus.Message{}
	sr.Header = &block.Block{Header: &block.BlockHeader{}}
	isValid := sr.IsBlockHeaderFinalInfoValid(cnsDta)
	assert.True(t, isValid)
}
