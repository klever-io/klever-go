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
	if check.IfNil(sr.Header) {
		return false
	}

	header := sr.Header.Clone()
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

	sr.Header.SetPubKeysBitmap(bitmap)
	sr.Header.SetSignature(sig)

	// Header is complete so the leader can sign it
	leaderSignature, err := sr.signBlockHeader()
	if err != nil {
		log.Error(err.Error())
		return false
	}
	sr.Header.SetProducerSignature(leaderSignature)

	// validate has block signature prior commit
	if len(sr.Header.GetSignature()) == 0 ||
		len(sr.Header.GetProducerSignature()) == 0 ||
		len(sr.Header.GetPubKeysBitmap()) == 0 {
		log.Error("doEndSlotJobByLeader invalid block",
			"signature", sr.Header.GetSignature(),
			"producerSignature", sr.Header.GetProducerSignature(),
			"pubKeysBitmap", sr.Header.GetPubKeysBitmap(),
		)
		return false
	}

	// broadcast section

	defer func() {
		sr.SetProcessingBlock(false)
	}()

	sr.SetProcessingBlock(true)

	shouldNotCommitBlock := sr.ExtendedCalled ||
		sr.Header.GetSlot() < tools.SafeI64ToU64(sr.SlotManager().Index())
	if shouldNotCommitBlock {
		log.Debug("canceled slot, extended has been called or slot index has been changed",
			"slot", sr.SlotManager().Index(),
			"subslot", sr.Name(),
			"header slot", sr.Header.GetSlot(),
			"extended called", sr.ExtendedCalled,
		)
		return false
	}

	if sr.isOutOfTime() {
		return false
	}

	// create and broadcast header final info
	sr.createAndBroadcastHeaderFinalInfo()

	// broadcast header
	err = sr.BroadcastMessenger().BroadcastBlock(sr.Header)
	if err != nil {
		log.Debug("doEndSlotJob.BroadcastHeader", "error", err.Error())
	}

	startTime := time.Now()
	err = sr.BlockProcessor().CommitBlock(sr.Header)
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

	log.Debug("step 3: Body and Header have been committed and header has been broadcast")

	msg := fmt.Sprintf("Added proposed block with nonce  %d  in blockchain", sr.Header.GetNonce())
	log.Debug(display.Headline(msg, sr.SyncTimer().FormattedCurrentTime(), "+"))

	sr.updateMetricsForLeader()

	return true
}

func (sr *subslotEndSlot) createAndBroadcastHeaderFinalInfo() {
	cnsMsg := consensus.NewConsensusMessage(
		sr.GetData(),
		nil,
		nil,
		[]byte(sr.SelfPubKey()),
		nil,
		int(MtBlockHeaderFinalInfo),
		sr.SlotManager().Index(),
		sr.Header.GetNonce(),
		sr.ChainID(),
		sr.Header.GetPubKeysBitmap(),
		sr.Header.GetSignature(),
		sr.Header.GetProducerSignature(),
		sr.CurrentPid(),
	)

	err := sr.BroadcastMessenger().BroadcastConsensusMessage(cnsMsg)
	if err != nil {
		log.Debug("doEndSlotJob.BroadcastConsensusMessage", "error", err.Error())
		return
	}

	log.Debug("step 3: block header final info has been sent",
		"PubKeysBitmap", sr.Header.GetPubKeysBitmap(),
		"AggregateSignature", sr.Header.GetSignature(),
		"LeaderSignature", sr.Header.GetProducerSignature())
}

func (sr *subslotEndSlot) doEndSlotJobByParticipant(cnsDta *consensus.Message) bool {
	sr.mutProcessingEndSlot.Lock()
	defer sr.mutProcessingEndSlot.Unlock()

	if sr.SlotCanceled {
		return false
	}
	if !sr.IsConsensusDataSet() {
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

	defer func() {
		sr.SetProcessingBlock(false)
	}()

	sr.SetProcessingBlock(true)

	shouldNotCommitBlock := sr.ExtendedCalled || header.GetSlot() < tools.SafeI64ToU64(sr.SlotManager().Index())
	if shouldNotCommitBlock {
		log.Debug("canceled slot, extended has been called or slot index has been changed",
			"slot", sr.SlotManager().Index(),
			"subslot", sr.Name(),
			"header slot", header.GetSlot(),
			"extended called", sr.ExtendedCalled,
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

	sr.displayStatistics()

	log.Debug("step 3: Body and Header have been committed")

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

	if check.IfNil(sr.Header) {
		return false, nil
	}

	header := sr.Header.Clone()
	header.SetPubKeysBitmap(cnsDta.PubKeysBitmap)
	header.SetSignature(cnsDta.AggregateSignature)
	header.SetProducerSignature(cnsDta.LeaderSignature)

	return true, header
}

func (sr *subslotEndSlot) isConsensusHeaderReceived() (bool, data.HeaderHandler) {
	if check.IfNil(sr.Header) {
		return false, nil
	}

	consensusHeaderHash, err := tools.CalculateHash(sr.Marshalizer(), sr.Hasher(), sr.Header.GetBlockHeader())
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

func (sr *subslotEndSlot) signBlockHeader() ([]byte, error) {
	marshalizedHdr, err := sr.Marshalizer().Marshal(sr.Header.GetBlockHeader())
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
	if sr.SlotCanceled {
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

		err = sr.MultiSigner().VerifySignatureShare(uint16(i), signature, sr.GetData(), bitmap) // #nosec G115
		if err != nil {
			return err
		}
	}

	return nil
}

func (sr *subslotEndSlot) isOutOfTime() bool {
	startTime := sr.SlotTimestamp
	maxTime := sr.SlotManager().TimeDuration() * time.Duration(sr.processingThresholdPercentage) / 100
	if sr.SlotManager().RemainingTime(startTime, maxTime) < 0 {
		log.Debug("canceled slot, time is out",
			"slot", sr.SyncTimer().FormattedCurrentTime(), sr.SlotManager().Index(),
			"subslot", sr.Name())

		sr.SlotCanceled = true
		return true
	}

	return false
}
