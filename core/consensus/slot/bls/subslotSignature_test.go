package bls_test

import (
	"testing"

	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/mock"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/core/consensus/slot/bls"
	"github.com/klever-io/klever-go/data"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func initSubslotSignatureWithContainer(container *mock.ConsensusCoreMock) bls.SubslotSignature {
	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := slot.NewSubslot(
		bls.SrBlock,
		bls.SrSignature,
		bls.SrEndSlot,
		int64(70*slotTimeDuration/100),
		int64(85*slotTimeDuration/100),
		"(SIGNATURE)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	srSignature, _ := bls.NewSubslotSignature(
		sr,
		extend,
	)

	return srSignature
}

func initSubslotSignature() bls.SubslotSignature {
	container := mock.InitConsensusCore()
	return initSubslotSignatureWithContainer(container)
}

func TestSubslotSignature_NewSubslotSignatureNilSubslotShouldFail(t *testing.T) {
	t.Parallel()

	srSignature, err := bls.NewSubslotSignature(
		nil,
		extend,
	)

	assert.Nil(t, srSignature)
	assert.Equal(t, slot.ErrNilSubslot, err)
}

func TestSubslotSignature_NewSubslotSignatureNilConsensusStateShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := slot.NewSubslot(
		bls.SrBlock,
		bls.SrSignature,
		bls.SrEndSlot,
		int64(70*slotTimeDuration/100),
		int64(85*slotTimeDuration/100),
		"(SIGNATURE)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	sr.ConsensusState = nil
	srSignature, err := bls.NewSubslotSignature(
		sr,
		extend,
	)

	assert.Nil(t, srSignature)
	assert.Equal(t, slot.ErrNilConsensusState, err)
}

func TestSubslotSignature_NewSubslotSignatureNilHasherShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := slot.NewSubslot(
		bls.SrBlock,
		bls.SrSignature,
		bls.SrEndSlot,
		int64(70*slotTimeDuration/100),
		int64(85*slotTimeDuration/100),
		"(SIGNATURE)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	container.SetHasher(nil)
	srSignature, err := bls.NewSubslotSignature(
		sr,
		extend,
	)

	assert.Nil(t, srSignature)
	assert.Equal(t, slot.ErrNilHasher, err)
}

func TestSubslotSignature_NewSubslotSignatureNilMultisignerShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := slot.NewSubslot(
		bls.SrBlock,
		bls.SrSignature,
		bls.SrEndSlot,
		int64(70*slotTimeDuration/100),
		int64(85*slotTimeDuration/100),
		"(SIGNATURE)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	container.SetMultiSigner(nil)
	srSignature, err := bls.NewSubslotSignature(
		sr,
		extend,
	)

	assert.Nil(t, srSignature)
	assert.Equal(t, slot.ErrNilMultiSigner, err)
}

func TestSubslotSignature_NewSubslotSignatureNilSlotManagerShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := slot.NewSubslot(
		bls.SrBlock,
		bls.SrSignature,
		bls.SrEndSlot,
		int64(70*slotTimeDuration/100),
		int64(85*slotTimeDuration/100),
		"(SIGNATURE)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	container.SetSlotManager(nil)

	srSignature, err := bls.NewSubslotSignature(
		sr,
		extend,
	)

	assert.Nil(t, srSignature)
	assert.Equal(t, slot.ErrNilSlotManager, err)
}

func TestSubslotSignature_NewSubslotSignatureNilSyncTimerShouldFail(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := slot.NewSubslot(
		bls.SrBlock,
		bls.SrSignature,
		bls.SrEndSlot,
		int64(70*slotTimeDuration/100),
		int64(85*slotTimeDuration/100),
		"(SIGNATURE)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)
	container.SetSyncTimer(nil)
	srSignature, err := bls.NewSubslotSignature(
		sr,
		extend,
	)

	assert.Nil(t, srSignature)
	assert.Equal(t, slot.ErrNilSyncTimer, err)
}

func TestSubslotSignature_NewSubslotSignatureShouldWork(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	consensusState := initConsensusState()
	ch := make(chan bool, 1)

	sr, _ := slot.NewSubslot(
		bls.SrBlock,
		bls.SrSignature,
		bls.SrEndSlot,
		int64(70*slotTimeDuration/100),
		int64(85*slotTimeDuration/100),
		"(SIGNATURE)",
		consensusState,
		ch,
		executeStoredMessages,
		container,
		chainID,
		currentPid,
	)

	srSignature, err := bls.NewSubslotSignature(
		sr,
		extend,
	)

	assert.NotNil(t, srSignature)
	assert.Nil(t, err)
}

func TestSubslotSignature_DoSignatureJob(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	sr := *initSubslotSignatureWithContainer(container)

	sr.Data = nil
	r := sr.DoSignatureJob()
	assert.False(t, r)

	sr.Data = []byte("X")

	multiSignerMock := mock.InitMultiSignerMock()

	err := errors.New("create signature share error")
	multiSignerMock.CreateSignatureShareMock = func(msg []byte, bitmap []byte) ([]byte, error) {
		return nil, err
	}

	container.SetMultiSigner(multiSignerMock)

	r = sr.DoSignatureJob()
	assert.False(t, r)

	multiSignerMock = mock.InitMultiSignerMock()

	multiSignerMock.CreateSignatureShareMock = func(msg []byte, bitmap []byte) ([]byte, error) {
		return []byte("SIG"), nil
	}
	container.SetMultiSigner(multiSignerMock)

	sr.Header = &cMock.HeaderHandlerStub{}
	r = sr.DoSignatureJob()
	assert.True(t, r)

	_ = sr.SetJobDone(sr.SelfPubKey(), bls.SrSignature, false)
	sr.SlotCanceled = false
	sr.SetSelfPubKey(sr.ConsensusGroup()[0])
	r = sr.DoSignatureJob()
	assert.True(t, r)
	assert.False(t, sr.SlotCanceled)
}

func TestSubslotSignature_ReceivedSignature(t *testing.T) {
	t.Parallel()

	sr := *initSubslotSignature()
	signature := []byte("signature")
	cnsMsg := consensus.NewConsensusMessage(
		sr.Data,
		signature,
		nil,
		[]byte(sr.ConsensusGroup()[1]),
		[]byte("sig"),
		int(bls.MtSignature),
		0,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)

	sr.Data = nil
	r := sr.ReceivedSignature(cnsMsg)
	assert.False(t, r)

	sr.Data = []byte("Y")
	r = sr.ReceivedSignature(cnsMsg)
	assert.False(t, r)

	sr.Data = []byte("X")
	r = sr.ReceivedSignature(cnsMsg)
	assert.False(t, r)

	sr.SetSelfPubKey(sr.ConsensusGroup()[0])

	cnsMsg.PubKey = []byte("X")
	r = sr.ReceivedSignature(cnsMsg)
	assert.False(t, r)

	cnsMsg.PubKey = []byte(sr.ConsensusGroup()[1])
	maxCount := len(sr.ConsensusGroup()) * 2 / 3
	count := 0
	for i := 0; i < len(sr.ConsensusGroup()); i++ {
		if sr.ConsensusGroup()[i] != string(cnsMsg.PubKey) {
			_ = sr.SetJobDone(sr.ConsensusGroup()[i], bls.SrSignature, true)
			count++
			if count == maxCount {
				break
			}
		}
	}
	r = sr.ReceivedSignature(cnsMsg)
	assert.True(t, r)
}

func TestSubslotSignature_ReceivedSignature_HonestyScore(t *testing.T) {
	t.Parallel()

	var testCases = []struct {
		name              string
		pubKeyIndex       int
		signature         string
		shouldJobBeDone   bool
		shouldBeAccepted  bool
		shouldChangeScore bool
		expectedUnits     int
	}{
		{
			name:              "valid signature",
			pubKeyIndex:       1,
			signature:         "i am valid!",
			shouldBeAccepted:  true,
			shouldJobBeDone:   true,
			shouldChangeScore: true,
			expectedUnits:     1,
		},
		{
			name:              "invalid signature",
			pubKeyIndex:       2,
			signature:         "signature share", // mocked to be invalid
			shouldBeAccepted:  false,
			shouldJobBeDone:   false,
			shouldChangeScore: false,
			expectedUnits:     0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Build a fresh container + subslot per subtest. Sharing one
			// container across t.Parallel() subtests races on the mock's
			// peerHonestyHandler slot via SetPeerHonestyHandler below.
			container := mock.InitConsensusCore()
			sr := *initSubslotSignatureWithContainer(container)
			sr.SetSelfPubKey(sr.ConsensusGroup()[0])
			pubKey := sr.ConsensusGroup()[tc.pubKeyIndex]

			called := false
			container.SetPeerHonestyHandler(&cMock.PeerHonestyHandlerStub{
				ChangeScoreCalled: func(pk string, topic string, units int) {
					called = true
					assert.Equal(t, pubKey, pk)
					assert.Equal(t, "consensus", topic)
					assert.Equal(t, tc.expectedUnits, units)
				},
			})

			cnsMsg := consensus.NewConsensusMessage(
				sr.GetData(),
				[]byte(tc.signature),
				nil,
				[]byte(pubKey),
				[]byte("sig"),
				int(bls.MtSignature),
				0,
				0,
				chainID,
				nil,
				nil,
				nil,
				currentPid,
			)
			accepted := sr.ReceivedSignature(cnsMsg)
			assert.Equal(t, tc.shouldBeAccepted, accepted)
			assert.Equal(t, tc.shouldJobBeDone, sr.IsJobDone(pubKey, bls.SrSignature))
			assert.Equal(t, tc.shouldChangeScore, called)
		})
	}
}

func TestSubslotSignature_SignaturesCollected(t *testing.T) {
	t.Parallel()

	sr := *initSubslotSignature()

	for i := 0; i < len(sr.ConsensusGroup()); i++ {
		_ = sr.SetJobDone(sr.ConsensusGroup()[i], bls.SrBlock, false)
		_ = sr.SetJobDone(sr.ConsensusGroup()[i], bls.SrSignature, false)
	}

	ok, n := sr.AreSignaturesCollected(2)
	assert.False(t, ok)
	assert.Equal(t, 0, n)

	ok, _ = sr.AreSignaturesCollected(2)
	assert.False(t, ok)

	_ = sr.SetJobDone("B", bls.SrSignature, true)
	isJobDone, _ := sr.JobDone("B", bls.SrSignature)
	assert.True(t, isJobDone)

	ok, _ = sr.AreSignaturesCollected(2)
	assert.False(t, ok)

	_ = sr.SetJobDone("C", bls.SrSignature, true)
	ok, _ = sr.AreSignaturesCollected(2)
	assert.True(t, ok)
}

func TestSubslotSignature_DoSignatureConsensusCheckShouldReturnFalseWhenSlotIsCanceled(t *testing.T) {
	t.Parallel()

	sr := *initSubslotSignature()
	sr.SlotCanceled = true
	assert.False(t, sr.DoSignatureConsensusCheck())
}

func TestSubslotSignature_DoSignatureConsensusCheckShouldReturnTrueWhenSubslotIsFinished(t *testing.T) {
	t.Parallel()

	sr := *initSubslotSignature()
	sr.SetStatus(bls.SrSignature, slot.SsFinished)
	assert.True(t, sr.DoSignatureConsensusCheck())
}

func TestSubslotSignature_DoSignatureConsensusCheckShouldReturnTrueWhenSignaturesCollectedReturnTrue(t *testing.T) {
	t.Parallel()

	sr := *initSubslotSignature()

	for i := 0; i < sr.Threshold(bls.SrSignature); i++ {
		_ = sr.SetJobDone(sr.ConsensusGroup()[i], bls.SrSignature, true)
	}

	assert.True(t, sr.DoSignatureConsensusCheck())
}

func TestSubslotSignature_DoSignatureConsensusCheckShouldReturnFalseWhenSignaturesCollectedReturnFalse(t *testing.T) {
	t.Parallel()

	sr := *initSubslotSignature()
	assert.False(t, sr.DoSignatureConsensusCheck())
}

func TestSubslotSignature_DoSignatureConsensusCheckShouldReturnFalseWhenNotAllSignaturesCollectedAndTimeIsNotOut(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	sr := *initSubslotSignatureWithContainer(container)
	sr.WaitingAllSignaturesTimeOut = false

	sr.SetSelfPubKey(sr.ConsensusGroup()[0])

	for i := 0; i < sr.Threshold(bls.SrSignature); i++ {
		_ = sr.SetJobDone(sr.ConsensusGroup()[i], bls.SrSignature, true)
	}

	assert.False(t, sr.DoSignatureConsensusCheck())
}

func TestSubslotSignature_DoSignatureConsensusCheckShouldReturnTrueWhenAllSignaturesCollected(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	sr := *initSubslotSignatureWithContainer(container)
	sr.WaitingAllSignaturesTimeOut = false

	sr.SetSelfPubKey(sr.ConsensusGroup()[0])

	for i := 0; i < sr.ConsensusGroupSize(); i++ {
		_ = sr.SetJobDone(sr.ConsensusGroup()[i], bls.SrSignature, true)
	}

	assert.True(t, sr.DoSignatureConsensusCheck())
}

func TestSubslotSignature_DoSignatureConsensusCheckShouldReturnTrueWhenEnoughButNotAllSignaturesCollectedAndTimeIsOut(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	sr := *initSubslotSignatureWithContainer(container)
	sr.WaitingAllSignaturesTimeOut = true

	sr.SetSelfPubKey(sr.ConsensusGroup()[0])

	for i := 0; i < sr.Threshold(bls.SrSignature); i++ {
		_ = sr.SetJobDone(sr.ConsensusGroup()[i], bls.SrSignature, true)
	}

	assert.True(t, sr.DoSignatureConsensusCheck())
}

func TestSubslotSignature_DoSignatureConsensusCheckShouldReturnFalseWhenFallbackThresholdCouldNotBeApplied(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	container.SetFallbackHeaderValidator(&cMock.FallBackHeaderValidatorStub{
		ShouldApplyFallbackValidationCalled: func(headerHandler data.HeaderHandler) bool {
			return false
		},
	})
	sr := *initSubslotSignatureWithContainer(container)
	sr.WaitingAllSignaturesTimeOut = false

	sr.SetSelfPubKey(sr.ConsensusGroup()[0])

	for i := 0; i < sr.FallbackThreshold(bls.SrSignature); i++ {
		_ = sr.SetJobDone(sr.ConsensusGroup()[i], bls.SrSignature, true)
	}

	assert.False(t, sr.DoSignatureConsensusCheck())
}

func TestSubslotSignature_DoSignatureConsensusCheckShouldReturnTrueWhenFallbackThresholdCouldBeApplied(t *testing.T) {
	t.Parallel()

	container := mock.InitConsensusCore()
	container.SetFallbackHeaderValidator(&cMock.FallBackHeaderValidatorStub{
		ShouldApplyFallbackValidationCalled: func(headerHandler data.HeaderHandler) bool {
			return true
		},
	})
	sr := *initSubslotSignatureWithContainer(container)
	sr.WaitingAllSignaturesTimeOut = true

	sr.SetSelfPubKey(sr.ConsensusGroup()[0])

	for i := 0; i < sr.FallbackThreshold(bls.SrSignature); i++ {
		_ = sr.SetJobDone(sr.ConsensusGroup()[i], bls.SrSignature, true)
	}

	assert.True(t, sr.DoSignatureConsensusCheck())
}

func TestSubslotSignature_ReceivedSignatureReturnFalseWhenConsensusDataIsNotEqual(t *testing.T) {
	t.Parallel()

	sr := *initSubslotSignature()

	cnsMsg := consensus.NewConsensusMessage(
		append(sr.Data, []byte("X")...),
		[]byte("signature"),
		nil,
		[]byte(sr.ConsensusGroup()[0]),
		[]byte("sig"),
		int(bls.MtSignature),
		0,
		0,
		chainID,
		nil,
		nil,
		nil,
		currentPid,
	)

	assert.False(t, sr.ReceivedSignature(cnsMsg))
}
