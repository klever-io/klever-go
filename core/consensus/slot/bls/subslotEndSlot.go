package bls

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/display"
)

type subslotEndSlot struct {
	*slot.Subslot
	processingThresholdPercentage int
	displayStatistics             func()
	appStatusHandler              core.AppStatusHandler
	mutProcessingEndSlot          sync.Mutex
}

// SetAppStatusHandler method set appStatusHandler
func (sr *subslotEndSlot) SetAppStatusHandler(ash core.AppStatusHandler) error {
	if ash == nil || ash.IsInterfaceNil() {
		return slot.ErrNilAppStatusHandler
	}

	sr.appStatusHandler = ash
	return nil
}

// NewSubslotEndSlot creates a subslotEndSlot object
func NewSubslotEndSlot(
	baseSubslot *slot.Subslot,
	extend func(subslotId int),
	processingThresholdPercentage int,
	displayStatistics func(),
) (*subslotEndSlot, error) {
	err := checkNewSubslotEndSlotParams(
		baseSubslot,
	)
	if err != nil {
		return nil, err
	}

	srEndSlot := subslotEndSlot{
		baseSubslot,
		processingThresholdPercentage,
		displayStatistics,
		statusHandler.NewNilStatusHandler(),
		sync.Mutex{},
	}
	srEndSlot.Job = srEndSlot.doEndSlotJob
	srEndSlot.Check = srEndSlot.doEndSlotConsensusCheck
	srEndSlot.Extend = extend

	return &srEndSlot, nil
}

func checkNewSubslotEndSlotParams(
	baseSubslot *slot.Subslot,
) error {
	if baseSubslot == nil {
		return slot.ErrNilSubslot
	}
	if baseSubslot.ConsensusState == nil {
		return slot.ErrNilConsensusState
	}

	err := slot.ValidateConsensusCore(baseSubslot.ConsensusCoreHandler)

	return err
}

// receivedBlockHeaderFinalInfo method is called when a block header final info is received
func (sr *subslotEndSlot) receivedBlockHeaderFinalInfo(cnsDta *consensus.Message) bool {
	node := string(cnsDta.PubKey)

	if !sr.IsConsensusDataSet() {
		return false
	}

	if !sr.IsNodeLeaderInCurrentSlot(node) { // is NOT this node leader in current slot?
		sr.PeerHonestyHandler().ChangeScore(
			node,
			common.ConsensusTopic,
			slot.LeaderPeerHonestyDecreaseFactor,
		)

		return false
	}

	if sr.IsSelfLeaderInCurrentSlot() {
		return false
	}

	if !sr.IsConsensusDataEqual(cnsDta.BlockHeaderHash) {
		return false
	}

	if !sr.CanProcessReceivedMessage(cnsDta, sr.SlotManager().Index(), sr.Current()) {
		return false
	}

	if !sr.isBlockHeaderFinalInfoValid(cnsDta) {
		return false
	}

	log.Debug("step 3: block header final info has been received",
		"PubKeysBitmap", cnsDta.PubKeysBitmap,
		"AggregateSignature", cnsDta.AggregateSignature,
		"LeaderSignature", cnsDta.LeaderSignature)

	sr.PeerHonestyHandler().ChangeScore(
		node,
		common.ConsensusTopic,
		slot.LeaderPeerHonestyIncreaseFactor,
	)

	return sr.doEndSlotJobByParticipant(cnsDta)
}

func (sr *subslotEndSlot) isBlockHeaderFinalInfoValid(cnsDta *consensus.Message) bool {
	sr.RLockSlotState()
	currentHeader := sr.Header
	sr.RUnlockSlotState()

	if check.IfNil(currentHeader) {
		return false
	}

	header := currentHeader.Clone()
	header.SetPubKeysBitmap(cnsDta.PubKeysBitmap)
	header.SetSignature(cnsDta.AggregateSignature)
	header.SetProducerSignature(cnsDta.LeaderSignature)

	err := sr.HeaderSigVerifier().VerifyLeaderSignature(header)
	if err != nil {
		log.Debug("isBlockHeaderFinalInfoValid.VerifyLeaderSignature", "error", err.Error())
		return false
	}

	err = sr.HeaderSigVerifier().VerifySignature(header)
	if err != nil {
		log.Debug("isBlockHeaderFinalInfoValid.VerifySignature", "error", err.Error())
		return false
	}

	return true
}

func (sr *subslotEndSlot) receivedHeader(headerHandler data.HeaderHandler) {
	if sr.ConsensusGroup() == nil || sr.IsSelfLeaderInCurrentSlot() {
		return
	}

	sr.AddReceivedHeader(headerHandler)

	sr.doEndSlotJobByParticipant(nil)
}

// doEndSlotJob method does the job of the subslot EndSlot
func (sr *subslotEndSlot) doEndSlotJob() bool {
	if !sr.IsSelfLeaderInCurrentSlot() {
		if sr.IsNodeInConsensusGroup(sr.SelfPubKey()) {
			err := sr.prepareBroadcastBlockDataForValidator()
			if err != nil {
				log.Warn("validator in consensus group preparing for delayed broadcast",
					"error", err.Error())
			}
		}

		return sr.doEndSlotJobByParticipant(nil)
	}

	return sr.doEndSlotJobByLeader()
}

func (sr *subslotEndSlot) doEndSlotJobByLeader() bool {
	bitmap := sr.GenerateBitmap(SrSignature)
	err := sr.checkSignaturesValidity(bitmap)
	if err != nil {
		log.Debug("doEndSlotJob.checkSignaturesValidity", "error", err.Error())
		return false
	}

	// Aggregate sig and add it to the block
	sig, err := sr.MultiSigner().AggregateSigs(bitmap)
	if err != nil {
		log.Debug("doEndSlotJob.AggregateSigs", "error", err.Error())
		return false
	}

	// Snapshot the consensus header pointer once. The invariant this relies on:
	// from this point until the leader finishes endSlot, the leader's chronology
	// goroutine is the SOLE mutator of the underlying header object — the
	// interceptor path (subslotBlock.receivedBlockHeader) does not write
	// sr.Header for the self-leader case, and ResetConsensusState runs only at
	// the next slot boundary, which is sequenced after this endSlot completes on
	// the same chronology goroutine. We pass this snapshot explicitly to every
	// downstream call (signBlockHeader, createAndBroadcastHeaderFinalInfo,
	// BroadcastBlock, CommitBlock, broadcastBlockDataLeader) so the
	// "leader-owned" guarantee is encoded in the call signatures rather than
	// the implicit field aliasing on sr.Header.
	sr.RLockSlotState()
	header := sr.Header
	sr.RUnlockSlotState()

	header.SetPubKeysBitmap(bitmap)
	header.SetSignature(sig)

	// Header is complete so the leader can sign it
	leaderSignature, err := sr.signBlockHeader(header)
	if err != nil {
		log.Error(err.Error())
		return false
	}
	header.SetProducerSignature(leaderSignature)

	// validate has block signature prior commit
	if len(header.GetSignature()) == 0 ||
		len(header.GetProducerSignature()) == 0 ||
		len(header.GetPubKeysBitmap()) == 0 {
		log.Error("doEndSlotJobByLeader invalid block",
			"signature", header.GetSignature(),
			"producerSignature", header.GetProducerSignature(),
			"pubKeysBitmap", header.GetPubKeysBitmap(),
		)
		return false
	}

	// broadcast section

	spawnSlot := tools.SafeU64ToI64(header.GetSlot())
	releaseProcessingBlock := sr.AcquireProcessingBlock(spawnSlot)
	defer releaseProcessingBlock()

	sr.RLockSlotState()
	extendedCalled := sr.ExtendedCalled
	sr.RUnlockSlotState()

	shouldNotCommitBlock := extendedCalled ||
		header.GetSlot() < tools.SafeI64ToU64(sr.SlotManager().Index())
	if shouldNotCommitBlock {
		log.Debug("canceled slot, extended has been called or slot index has been changed",
			"slot", sr.SlotManager().Index(),
			"subslot", sr.Name(),
			"header slot", header.GetSlot(),
			"extended called", extendedCalled,
		)
		return false
	}

	if sr.isOutOfTime() {
		return false
	}

	// create and broadcast header final info
	sr.createAndBroadcastHeaderFinalInfo(header)

	// broadcast header
	err = sr.BroadcastMessenger().BroadcastBlock(header)
	if err != nil {
		log.Debug("doEndSlotJob.BroadcastHeader", "error", err.Error())
	}

	startTime := time.Now()
	err = sr.BlockProcessor().CommitBlock(header)
	elapsedTime := time.Since(startTime)
	if elapsedTime >= core.CommitMaxTime {
		log.Warn("doEndSlotJobByLeader.CommitBlock", "elapsed time", elapsedTime)
	} else {
		log.Debug("elapsed time to commit block",
			"time [s]", elapsedTime,
		)
	}
	if err != nil {
		log.Debug("doEndSlotJob.CommitBlock", "error", err)
		return false
	}

	sr.SetStatus(sr.Current(), slot.SsFinished)

	sr.displayStatistics()

	log.Debug("step 3: Header have been committed and header has been broadcast")

	err = sr.broadcastBlockDataLeader(header)
	if err != nil {
		log.Debug("doEndRoundJobByLeader.broadcastBlockDataLeader", "error", err.Error())
	}

	msg := fmt.Sprintf("Added proposed block with nonce  %d  in blockchain", header.GetNonce())
	log.Debug(display.Headline(msg, sr.SyncTimer().FormattedCurrentTime(), "+"))

	sr.updateMetricsForLeader()

	return true
}

func (sr *subslotEndSlot) createAndBroadcastHeaderFinalInfo(header data.HeaderHandler) {
	cnsMsg := consensus.NewConsensusMessage(
		sr.GetData(),
		nil,
		nil,
		[]byte(sr.SelfPubKey()),
		nil,
		int(MtBlockHeaderFinalInfo),
		sr.SlotManager().Index(),
		header.GetNonce(),
		sr.ChainID(),
		header.GetPubKeysBitmap(),
		header.GetSignature(),
		header.GetProducerSignature(),
		sr.CurrentPid(),
	)

	err := sr.BroadcastMessenger().BroadcastConsensusMessage(cnsMsg)
	if err != nil {
		log.Debug("doEndSlotJob.BroadcastConsensusMessage", "error", err.Error())
		return
	}

	log.Debug("step 3: block header final info has been sent",
		"PubKeysBitmap", header.GetPubKeysBitmap(),
		"AggregateSignature", header.GetSignature(),
		"LeaderSignature", header.GetProducerSignature())
}

func (sr *subslotEndSlot) doEndSlotJobByParticipant(cnsDta *consensus.Message) bool {
	sr.mutProcessingEndSlot.Lock()
	defer sr.mutProcessingEndSlot.Unlock()

	sr.RLockSlotState()
	canceled := sr.SlotCanceled
	dataSet := sr.Data != nil
	sr.RUnlockSlotState()

	if canceled {
		return false
	}
	if !dataSet {
		return false
	}
	if !sr.IsSubslotFinished(sr.Previous()) {
		return false
	}
	if sr.IsSubslotFinished(sr.Current()) {
		return false
	}

	haveHeader, header := sr.haveConsensusHeaderWithFullInfo(cnsDta)
	if !haveHeader {
		return false
	}

	spawnSlot := tools.SafeU64ToI64(header.GetSlot())
	releaseProcessingBlock := sr.AcquireProcessingBlock(spawnSlot)
	defer releaseProcessingBlock()

	sr.RLockSlotState()
	extendedCalled := sr.ExtendedCalled
	sr.RUnlockSlotState()

	shouldNotCommitBlock := extendedCalled || header.GetSlot() < tools.SafeI64ToU64(sr.SlotManager().Index())
	if shouldNotCommitBlock {
		log.Debug("canceled slot, extended has been called or slot index has been changed",
			"slot", sr.SlotManager().Index(),
			"subslot", sr.Name(),
			"header slot", header.GetSlot(),
			"extended called", extendedCalled,
		)
		return false
	}

	if sr.isOutOfTime() {
		return false
	}

	startTime := time.Now()
	err := sr.BlockProcessor().CommitBlock(header)
	elapsedTime := time.Since(startTime)
	if elapsedTime >= core.CommitMaxTime {
		log.Warn("doEndSlotJobByParticipant.CommitBlock", "elapsed time", elapsedTime)
	} else {
		log.Debug("elapsed time to commit block",
			"time [s]", elapsedTime,
		)
	}
	if err != nil {
		log.Debug("doEndSlotJobByParticipant.CommitBlock", "error", err.Error())
		return false
	}

	sr.SetStatus(sr.Current(), slot.SsFinished)

	if sr.IsNodeInConsensusGroup(sr.SelfPubKey()) {
		err = sr.setHeaderForValidator(header)
		if err != nil {
			log.Warn("doEndRoundJobByParticipant", "error", err.Error())
		}
	}

	sr.displayStatistics()

	log.Debug("step 3: Header have been committed")

	headerTypeMsg := "received"
	if cnsDta != nil {
		headerTypeMsg = "assembled"
	}

	msg := fmt.Sprintf("Added %s block with nonce  %d  in blockchain", headerTypeMsg, header.GetNonce())
	log.Debug(display.Headline(msg, sr.SyncTimer().FormattedCurrentTime(), "-"))
	return true
}

func (sr *subslotEndSlot) haveConsensusHeaderWithFullInfo(cnsDta *consensus.Message) (bool, data.HeaderHandler) {
	if cnsDta == nil {
		return sr.isConsensusHeaderReceived()
	}

	sr.RLockSlotState()
	currentHeader := sr.Header
	sr.RUnlockSlotState()

	if check.IfNil(currentHeader) {
		return false, nil
	}

	header := currentHeader.Clone()
	header.SetPubKeysBitmap(cnsDta.PubKeysBitmap)
	header.SetSignature(cnsDta.AggregateSignature)
	header.SetProducerSignature(cnsDta.LeaderSignature)

	return true, header
}

func (sr *subslotEndSlot) isConsensusHeaderReceived() (bool, data.HeaderHandler) {
	sr.RLockSlotState()
	currentHeader := sr.Header
	sr.RUnlockSlotState()

	if check.IfNil(currentHeader) {
		return false, nil
	}

	consensusHeaderHash, err := tools.CalculateHash(sr.Marshalizer(), sr.Hasher(), currentHeader.GetBlockHeader())
	if err != nil {
		log.Debug("isConsensusHeaderReceived: calculate consensus header hash", "error", err.Error())
		return false, nil
	}

	receivedHeaders := sr.GetReceivedHeaders()

	var receivedHeaderHash []byte
	for index := range receivedHeaders {
		receivedHeader := receivedHeaders[index].GetBlockHeader()

		receivedHeaderHash, err = tools.CalculateHash(sr.Marshalizer(), sr.Hasher(), receivedHeader)
		if err != nil {
			log.Debug("isConsensusHeaderReceived: calculate received header hash", "error", err.Error())
			return false, nil
		}

		if bytes.Equal(receivedHeaderHash, consensusHeaderHash) {
			return true, receivedHeaders[index]
		}
	}

	return false, nil
}

func (sr *subslotEndSlot) signBlockHeader(header data.HeaderHandler) ([]byte, error) {
	marshalizedHdr, err := sr.Marshalizer().Marshal(header.GetBlockHeader())
	if err != nil {
		return nil, err
	}
	return sr.SingleSigner().Sign(sr.PrivateKey(), marshalizedHdr)
}

func (sr *subslotEndSlot) updateMetricsForLeader() {
	sr.appStatusHandler.Increment(core.MetricCountAcceptedBlocks)
	sr.appStatusHandler.SetStringValue(core.MetricConsensusSlotState,
		fmt.Sprintf("valid block produced in %f sec", time.Since(sr.SlotManager().Timestamp()).Seconds()))
}

// doEndSlotConsensusCheck method checks if the consensus is achieved
func (sr *subslotEndSlot) doEndSlotConsensusCheck() bool {
	sr.RLockSlotState()
	canceled := sr.SlotCanceled
	sr.RUnlockSlotState()

	if canceled {
		return false
	}

	if sr.IsSubslotFinished(sr.Current()) {
		return true
	}

	return false
}

func (sr *subslotEndSlot) checkSignaturesValidity(bitmap []byte) error {
	nbBitsBitmap := len(bitmap) * 8
	consensusGroup := sr.ConsensusGroup()
	consensusGroupSize := len(consensusGroup)
	size := consensusGroupSize

	if consensusGroupSize > nbBitsBitmap {
		size = nbBitsBitmap
	}

	for i := 0; i < size; i++ {
		indexRequired := (bitmap[i/8] & (1 << uint16(i%8))) > 0 // #nosec G115
		if !indexRequired {
			continue
		}

		pubKey := consensusGroup[i]
		isSigJobDone, err := sr.JobDone(pubKey, SrSignature)
		if err != nil {
			return err
		}

		if !isSigJobDone {
			return common.ErrNilSignature
		}

		signature, err := sr.MultiSigner().SignatureShare(uint16(i)) // #nosec G115
		if err != nil {
			return err
		}

		err = sr.MultiSigner().VerifySignatureShare(uint16(i), signature, sr.GetData()) // #nosec G115
		if err != nil {
			return err
		}
	}

	return nil
}

func (sr *subslotEndSlot) isOutOfTime() bool {
	sr.RLockSlotState()
	startTime := sr.SlotTimestamp
	sr.RUnlockSlotState()
	maxTime := sr.SlotManager().TimeDuration() * time.Duration(sr.processingThresholdPercentage) / 100
	if sr.SlotManager().RemainingTime(startTime, maxTime) < 0 {
		log.Debug("canceled slot, time is out",
			"slot", sr.SyncTimer().FormattedCurrentTime(), sr.SlotManager().Index(),
			"subslot", sr.Name())

		sr.SetSlotCanceled(true)
		return true
	}

	return false
}

func (sr *subslotEndSlot) getIndexPkAndDataToBroadcast() (int, []byte, []byte, [][]byte, error) {
	index, err := sr.SelfConsensusGroupIndex()
	if err != nil {
		return -1, nil, nil, nil, err
	}

	sr.RLockSlotState()
	header := sr.Header
	sr.RUnlockSlotState()

	blockBuffer, transactions, err := sr.BlockProcessor().MarshalizedDataToBroadcast(header)
	if err != nil {
		return -1, nil, nil, nil, err
	}

	consensusGroup := sr.ConsensusGroup()
	pk := []byte(consensusGroup[index])

	return index, pk, blockBuffer, transactions, nil

}

func (sr *subslotEndSlot) broadcastBlockDataLeader(header data.HeaderHandler) error {
	data, transactions, err := sr.BlockProcessor().MarshalizedDataToBroadcast(header)
	if err != nil {
		return err
	}

	return sr.BroadcastMessenger().BroadcastBlockDataLeader(header, data, transactions)
}

func (sr *subslotEndSlot) setHeaderForValidator(header data.HeaderHandler) error {
	index, pk, _, transactions, err := sr.getIndexPkAndDataToBroadcast()
	if err != nil {
		return err
	}

	go sr.BroadcastMessenger().PrepareBroadcastHeaderValidator(header, transactions, index, pk)

	return nil
}

func (sr *subslotEndSlot) prepareBroadcastBlockDataForValidator() error {
	index, pk, _, transactions, err := sr.getIndexPkAndDataToBroadcast()
	if err != nil {
		return err
	}

	sr.RLockSlotState()
	header := sr.Header
	sr.RUnlockSlotState()

	go sr.BroadcastMessenger().PrepareBroadcastBlockDataValidator(header, transactions, index, pk)

	return nil
}
