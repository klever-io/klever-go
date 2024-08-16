package bls

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/data"
	elasticIndexer "github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

// subslotStartSlot defines the data needed by the subslot StartSlot
type subslotStartSlot struct {
	*slot.Subslot
	processingThresholdPercentage int
	executeStoredMessages         func()
	resetConsensusMessages        func()

	indexer slot.ConsensusDataIndexer
}

// NewSubslotStartSlot creates a subslotStartSlot object
func NewSubslotStartSlot(
	baseSubslot *slot.Subslot,
	extend func(subslotId int),
	processingThresholdPercentage int,
	executeStoredMessages func(),
	resetConsensusMessages func(),
) (*subslotStartSlot, error) {
	err := checkNewSubslotStartSlotParams(
		baseSubslot,
	)
	if err != nil {
		return nil, err
	}

	srStartSlot := subslotStartSlot{
		Subslot:                       baseSubslot,
		processingThresholdPercentage: processingThresholdPercentage,
		executeStoredMessages:         executeStoredMessages,
		resetConsensusMessages:        resetConsensusMessages,
		indexer:                       elasticIndexer.NewNilIndexer(),
	}
	srStartSlot.Job = srStartSlot.doStartSlotJob
	srStartSlot.Check = srStartSlot.doStartSlotConsensusCheck
	srStartSlot.Extend = extend
	baseSubslot.EpochStartRegistrationHandler().RegisterHandler(&srStartSlot)

	return &srStartSlot, nil
}

func checkNewSubslotStartSlotParams(
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

// SetIndexer method set indexer
func (sr *subslotStartSlot) SetIndexer(indexer slot.ConsensusDataIndexer) {
	sr.indexer = indexer
}

// doStartSlotJob method does the job of the subslot StartSlot
func (sr *subslotStartSlot) doStartSlotJob() bool {
	sr.ResetConsensusState()
	sr.SlotIndex = sr.SlotManager().Index()
	sr.SlotTimestamp = sr.SlotManager().Timestamp()
	sr.GetAntiFloodHandler().ResetForTopic(common.ConsensusTopic)
	sr.resetConsensusMessages()
	return true
}

// doStartSlotConsensusCheck method checks if the consensus is achieved in the subslot StartSlot
func (sr *subslotStartSlot) doStartSlotConsensusCheck() bool {
	if sr.SlotCanceled {
		return false
	}

	if sr.IsSubslotFinished(sr.Current()) {
		return true
	}

	return sr.initCurrentSlot()
}

func (sr *subslotStartSlot) initCurrentSlot() bool {
	nodeState := sr.BootStrapper().GetNodeState()
	if nodeState != core.NsSynchronized { // if node is not synchronized yet, it has to continue the bootstrapping mechanism
		return false
	}

	sr.AppStatusHandler().SetStringValue(core.MetricConsensusSlotState, "")

	err := sr.generateNextConsensusGroup(sr.SlotManager().Index())
	if err != nil {
		log.Debug("initCurrentSlot.generateNextConsensusGroup",
			"slot index", sr.SlotManager().Index(),
			"error", err.Error())

		sr.SlotCanceled = true

		return false
	}

	if sr.NodeRedundancyHandler().IsRedundancyNode() {
		sr.NodeRedundancyHandler().AdjustInactivityIfNeeded(
			sr.SelfPubKey(),
			sr.ConsensusGroup(),
			sr.SlotManager().Index(),
		)
		if sr.NodeRedundancyHandler().IsMainMachineActive() {
			return false
		}
	}

	leader, err := sr.GetLeader()
	if err != nil {
		log.Debug("initCurrentSlot.GetLeader", "error", err.Error())

		sr.SlotCanceled = true

		return false
	}

	msg := ""
	if leader == sr.SelfPubKey() {
		sr.AppStatusHandler().Increment(core.MetricCountLeader)
		sr.AppStatusHandler().SetStringValue(core.MetricConsensusSlotState, "proposed")
		sr.AppStatusHandler().SetStringValue(core.MetricConsensusState, "proposer")
		msg = " (my turn)"
	}

	log.Debug("step 0: preparing the slot",
		"leader", tools.GetTrimmedPk(hex.EncodeToString([]byte(leader))),
		"messsage", msg)

	pubKeys := sr.ConsensusGroup()

	selfIndex, err := sr.SelfConsensusGroupIndex()
	if err != nil {
		log.Debug("not in consensus group")
		sr.AppStatusHandler().SetStringValue(core.MetricConsensusState, "not in consensus group")
	} else {
		if leader != sr.SelfPubKey() {
			sr.AppStatusHandler().Increment(core.MetricCountConsensus)
		}
		sr.AppStatusHandler().SetStringValue(core.MetricConsensusState, "participant")

		err = sr.MultiSigner().Reset(pubKeys, uint16(selfIndex))
		if err != nil {
			log.Debug("initCurrentSlot.Reset", "error", err.Error())

			sr.SlotCanceled = true

			return false
		}
	}

	startTime := sr.SlotTimestamp
	maxTime := sr.SlotManager().TimeDuration() * time.Duration(sr.processingThresholdPercentage) / 100
	if sr.SlotManager().RemainingTime(startTime, maxTime) < 0 {
		log.Debug("canceled slot, time is out",
			"slot", sr.SyncTimer().FormattedCurrentTime(), sr.SlotManager().Index(),
			"subslot", sr.Name())

		sr.SlotCanceled = true

		return false
	}

	sr.SetStatus(sr.Current(), slot.SsFinished)

	// execute stored messages which were received in this new slot but before this initialisation
	go sr.executeStoredMessages()

	return true
}

func (sr *subslotStartSlot) generateNextConsensusGroup(slotIndex int64) error {
	currentHeader := sr.Blockchain().GetCurrentBlockHeader()
	if check.IfNil(currentHeader) {
		currentHeader = sr.Blockchain().GetGenesisHeader()
		if check.IfNil(currentHeader) {
			return common.ErrNilHeader
		}
	}

	randomSeed := currentHeader.GetRandSeed()

	log.Debug("random source for the next consensus group",
		"rand", randomSeed)

	nextConsensusGroup, err := sr.GetNextConsensusGroup(
		randomSeed,
		uint64(sr.SlotIndex),
		sr.NodesCoordinator(),
		currentHeader.GetEpoch(),
	)
	if err != nil {
		return err
	}

	log.Trace("consensus group is formed by next validators:",
		"slot", slotIndex)

	for i := 0; i < len(nextConsensusGroup); i++ {
		log.Trace(tools.GetTrimmedPk(hex.EncodeToString([]byte(nextConsensusGroup[i]))), "index", i)
	}

	sr.SetConsensusGroup(nextConsensusGroup)

	return nil
}

// EpochStartPrepare wis called when an epoch start event is observed, but not yet confirmed/committed.
// Some components may need to do initialisation on this event
func (sr *subslotStartSlot) EpochStartPrepare(metaHdr data.HeaderHandler) {
	log.Trace(fmt.Sprintf("epoch %d start prepare in consensus", metaHdr.GetEpoch()))
}

// EpochStartAction is called upon a start of epoch event.
func (sr *subslotStartSlot) EpochStartAction(hdr data.HeaderHandler) {
	log.Trace(fmt.Sprintf("epoch %d start action in consensus", hdr.GetEpoch()))

	sr.changeEpoch(hdr.GetEpoch())
}

func (sr *subslotStartSlot) changeEpoch(currentEpoch uint32) {
	epochNodes, err := sr.NodesCoordinator().GetConsensusWhitelistedNodes(currentEpoch)
	if err != nil {
		panic(fmt.Sprintf("consensus changing epoch failed with error %s", err.Error()))
	}

	sr.SetElectedList(epochNodes)
}

// NotifyOrder returns the notification order for a start of epoch event
func (sr *subslotStartSlot) NotifyOrder() uint32 {
	return core.ConsensusOrder
}
