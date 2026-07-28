package bls

import (
	"strconv"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/tracing"
)

// subslotBlock defines the data needed by the subslot Block
type subslotBlock struct {
	*slot.Subslot

	processingThresholdPercentage int
}

// NewSubslotBlock creates a subslotBlock object
func NewSubslotBlock(
	baseSubslot *slot.Subslot,
	extend func(subslotId int),
	processingThresholdPercentage int,
) (*subslotBlock, error) {
	err := checkNewSubslotBlockParams(baseSubslot)
	if err != nil {
		return nil, err
	}

	srBlock := subslotBlock{
		Subslot:                       baseSubslot,
		processingThresholdPercentage: processingThresholdPercentage,
	}

	srBlock.Job = srBlock.doBlockJob
	srBlock.Check = srBlock.doBlockConsensusCheck
	srBlock.Extend = extend

	return &srBlock, nil
}

func checkNewSubslotBlockParams(
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

// doBlockJob method does the job of the subslot Block
func (sr *subslotBlock) doBlockJob() bool {
	if !sr.IsSelfLeaderInCurrentSlot() { // is NOT self leader in this slot?
		return false
	}

	if sr.SlotManager().Index() <= sr.getSlotInLastCommittedBlock() {
		return false
	}

	if sr.IsSelfJobDone(sr.Current()) {
		return false
	}

	if sr.IsSubslotFinished(sr.Current()) {
		return false
	}

	metricStatTime := time.Now()
	defer sr.computeSubslotProcessingMetric(metricStatTime, core.MetricCreatedProposedBlock)

	// Add context tags
	defer tracing.StartSpan(
		"consensus.block.createAndSend",
		"slot.index", strconv.FormatInt(sr.SlotManager().Index(), 10),
		"node.role", "leader",
	)()

	header, err := sr.createHeader()
	if err != nil {
		log.Debug("doBlockJob.createHeader", "error", err.Error())
		return false
	}

	header, err = sr.createBlock(header)
	if err != nil {
		log.Debug("doBlockJob.createBlock", "error", err.Error())
		return false
	}

	sentWithSuccess := sr.sendBlock(header)
	if !sentWithSuccess {
		return false
	}

	err = sr.SetSelfJobDone(sr.Current(), true)
	if err != nil {
		log.Debug("doBlockJob.SetSelfJobDone", "error", err.Error())
		return false
	}

	return true
}

func (sr *subslotBlock) sendBlock(header data.HeaderHandler) bool {

	blkObj, ok := header.(*block.Block)
	if !ok {
		log.Error("sendBlock.Assertion block", "error", common.ErrWrongTypeAssertion.Error())
		return false
	}

	headerHash, err := tools.CalculateHash(sr.Marshalizer(), sr.Hasher(), blkObj.Header)
	if err != nil {
		log.Error("sendBlock.CalculateHash", "error", err.Error())
		return false
	}

	marshalizedData, err := sr.Marshalizer().Marshal(header)
	if err != nil {
		log.Debug("sendBlock.Marshal: header", "error", err.Error())
		return false
	}

	return sr.sendBlockHeader(header, headerHash, marshalizedData)

}

func (sr *subslotBlock) createBlock(header data.HeaderHandler) (data.HeaderHandler, error) {
	sr.RLockSlotState()
	startTime := sr.SlotTimestamp
	sr.RUnlockSlotState()
	maxTime := time.Duration(sr.EndTime())
	haveTimeInCurrentSubslot := func() bool {
		return sr.SlotManager().RemainingTime(startTime, maxTime) > 0
	}

	finalHeader, err := sr.BlockProcessor().CreateBlock(
		header,
		haveTimeInCurrentSubslot,
	)
	if err != nil {
		return nil, err
	}

	return finalHeader, nil
}

// sendBlockHeader method sends the proposed block header in the subslot Block
func (sr *subslotBlock) sendBlockHeader(
	headerHandler data.HeaderHandler,
	headerHash []byte,
	marshalizedHeader []byte,
) bool {
	cnsMsg := consensus.NewConsensusMessage(
		headerHash,
		nil,
		marshalizedHeader,
		[]byte(sr.SelfPubKey()),
		nil,
		int(MtBlockHeader),
		sr.SlotManager().Index(),
		headerHandler.GetNonce(),
		sr.ChainID(),
		nil,
		nil,
		nil,
		sr.CurrentPid(),
	)

	err := sr.BroadcastMessenger().BroadcastConsensusMessage(cnsMsg)
	if err != nil {
		log.Debug("sendBlockHeader.BroadcastConsensusMessage", "error", err.Error())
		return false
	}

	log.Debug("step 1: block header have been sent",
		"nonce", headerHandler.GetNonce(),
		"hash", headerHash)

	sr.LockSlotState()
	sr.Data = headerHash
	sr.Header = headerHandler
	sr.UnlockSlotState()

	return true
}

func (sr *subslotBlock) createHeader() (data.HeaderHandler, error) {
	var nonce uint64
	var prevHash []byte
	var prevRandSeed []byte

	currentHeader := sr.Blockchain().GetCurrentBlockHeader()
	if check.IfNil(currentHeader) {
		nonce = sr.Blockchain().GetGenesisHeader().GetNonce() + 1
		prevHash = sr.Blockchain().GetGenesisHeaderHash()
		prevRandSeed = sr.Blockchain().GetGenesisHeader().GetRandSeed()
	} else {
		nonce = currentHeader.GetNonce() + 1
		prevHash = sr.Blockchain().GetCurrentBlockHeaderHash()
		prevRandSeed = currentHeader.GetRandSeed()
	}

	slot := tools.SafeI64ToU64(sr.SlotManager().Index())
	hdr := sr.BlockProcessor().CreateNewHeader(slot, nonce)
	hdr.SetParentHash(prevHash)

	randSeed, err := sr.SingleSigner().Sign(sr.PrivateKey(), prevRandSeed)
	if err != nil {
		return nil, err
	}

	hdr.SetTimestamp(sr.SlotManager().Timestamp().Unix())
	hdr.SetPrevRandSeed(prevRandSeed)
	hdr.SetRandSeed(randSeed)
	hdr.SetChainID(sr.ChainID())

	return hdr, nil
}

// receivedBlockHeader method is called when a block header is received
func (sr *subslotBlock) receivedBlockHeader(cnsDta *consensus.Message) bool {
	sw := tools.NewStopWatch()
	sw.Start("receivedBlockHeader")

	defer func() {
		sw.Stop("receivedHeader")
		log.Debug("time measurements of receivedBlockHeader", sw.GetMeasurements()...)
	}()

	node := string(cnsDta.PubKey)

	if sr.IsConsensusDataSet() {
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

	if sr.IsHeaderAlreadyReceived() {
		return false
	}

	if !sr.CanProcessReceivedMessage(cnsDta, sr.SlotManager().Index(), sr.Current()) {
		return false
	}

	headerHash := cnsDta.BlockHeaderHash
	decodedHeader := sr.BlockProcessor().DecodeBlockHeader(cnsDta.Header)

	if headerHash == nil || check.IfNil(decodedHeader) {
		return false
	}

	sr.LockSlotState()
	sr.Data = headerHash
	sr.Header = decodedHeader
	sr.UnlockSlotState()

	log.Debug("step 1: block header have been received",
		"nonce", decodedHeader.GetNonce(),
		"hash", headerHash)

	sw.Start("processReceivedBlock")
	blockProcessedWithSuccess := sr.processReceivedBlock(cnsDta)
	sw.Stop("processReceivedBlock")

	sr.PeerHonestyHandler().ChangeScore(
		node,
		common.ConsensusTopic,
		slot.LeaderPeerHonestyIncreaseFactor,
	)

	return blockProcessedWithSuccess
}

func (sr *subslotBlock) processReceivedBlock(cnsDta *consensus.Message) bool {
	sr.RLockSlotState()
	header := sr.Header
	startTime := sr.SlotTimestamp
	sr.RUnlockSlotState()

	if check.IfNil(header) {
		return false
	}

	spawnSlot := cnsDta.SlotIndex
	releaseProcessingBlock := sr.AcquireProcessingBlock(spawnSlot)
	defer releaseProcessingBlock()

	// Read ExtendedCalled AFTER AcquireProcessingBlock. Worker.Extend sets
	// ExtendedCalled=true and only then waits for ProcessingBlock to clear,
	// so our acquire creates the happens-before that guarantees
	// either (a) Extend sees our ProcessingBlock and waits — in which case our
	// later snapshot of extendedCalled may still be false but it doesn't matter
	// because Extend's revert is blocked until we finish, or (b) Extend already
	// flipped ExtendedCalled before our acquire — in which case the
	// post-snapshot read sees true and we bail. Snapshotting extendedCalled
	// BEFORE the acquire would let Extend slip in between, set the flag,
	// see ProcessingBlock=false, and start reverting state under us.
	sr.RLockSlotState()
	extendedCalled := sr.ExtendedCalled
	sr.RUnlockSlotState()

	shouldNotProcessBlock := extendedCalled || cnsDta.SlotIndex < sr.SlotManager().Index()
	if shouldNotProcessBlock {
		log.Debug("canceled slot, extended has been called or slot index has been changed",
			"slot", sr.SlotManager().Index(),
			"subslot", sr.Name(),
			"cnsDta slot", cnsDta.SlotIndex,
			"extended called", extendedCalled,
		)
		return false
	}

	node := string(cnsDta.PubKey)

	maxTime := sr.SlotManager().TimeDuration() * time.Duration(sr.processingThresholdPercentage) / 100
	remainingTimeInCurrentSlot := func() time.Duration {
		return sr.SlotManager().RemainingTime(startTime, maxTime)
	}

	metricStatTime := time.Now()
	defer sr.computeSubslotProcessingMetric(metricStatTime, core.MetricProcessedProposedBlock)

	err := sr.BlockProcessor().ProcessBlock(
		header,
		remainingTimeInCurrentSlot,
	)

	if cnsDta.SlotIndex < sr.SlotManager().Index() {
		log.Debug("canceled slot, slot index has been changed",
			"slot", sr.SlotManager().Index(),
			"subslot", sr.Name(),
			"cnsDta slot", cnsDta.SlotIndex,
		)
		return false
	}

	if err != nil {
		log.Debug("canceled slot",
			"slot", sr.SlotManager().Index(),
			"subslot", sr.Name(),
			"error", err.Error())

		sr.SetSlotCanceled(true)

		return false
	}

	err = sr.SetJobDone(node, sr.Current(), true)
	if err != nil {
		log.Debug("canceled slot",
			"slot", sr.SlotManager().Index(),
			"subslot", sr.Name(),
			"error", err.Error())
		return false
	}

	return true
}

func (sr *subslotBlock) computeSubslotProcessingMetric(startTime time.Time, metric string) {
	subSlotDuration := sr.EndTime() - sr.StartTime()
	if subSlotDuration == 0 {
		//can not do division by 0
		return
	}

	percent := uint64(time.Since(startTime)) * 100 / tools.SafeI64ToU64(subSlotDuration) // #nosec G115
	sr.AppStatusHandler().SetUInt64Value(metric, percent)
}

// doBlockConsensusCheck method checks if the consensus in the subslot Block is achieved
func (sr *subslotBlock) doBlockConsensusCheck() bool {
	sr.RLockSlotState()
	canceled := sr.SlotCanceled
	sr.RUnlockSlotState()
	if canceled {
		return false
	}

	if sr.IsSubslotFinished(sr.Current()) {
		return true
	}

	threshold := sr.Threshold(sr.Current())
	if sr.isBlockReceived(threshold) {
		log.Debug("step 1: subslot has been finished",
			"subslot", sr.Name())
		sr.SetStatus(sr.Current(), slot.SsFinished)
		return true
	}

	return false
}

// isBlockReceived method checks if the block was received from the leader in the current slot
func (sr *subslotBlock) isBlockReceived(threshold int) bool {
	n := 0

	consensusGroup := sr.ConsensusGroup()

	for i := 0; i < len(consensusGroup); i++ {
		node := consensusGroup[i]
		isJobDone, err := sr.JobDone(node, sr.Current())
		if err != nil {
			log.Debug("isBlockReceived.JobDone",
				"node", node,
				"subslot", sr.Name(),
				"error", err.Error())
			continue
		}

		if isJobDone {
			n++
		}
	}

	return n >= threshold
}

func (sr *subslotBlock) getSlotInLastCommittedBlock() int64 {
	slotInLastCommittedBlock := int64(0)
	currentHeader := sr.Blockchain().GetCurrentBlockHeader()
	if !check.IfNil(currentHeader) {
		slotInLastCommittedBlock = tools.SafeU64ToI64(currentHeader.GetSlot())
	}

	return slotInLastCommittedBlock
}
