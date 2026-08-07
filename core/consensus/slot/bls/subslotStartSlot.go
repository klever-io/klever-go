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
		resetConsensusMessages,
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
	resetConsensusMessages func(),
) error {
	if baseSubslot == nil {
		return slot.ErrNilSubslot
	}
	if baseSubslot.ConsensusState == nil {
		return slot.ErrNilConsensusState
	}
	// called unconditionally by doStartSlotJob and by EpochStartAction below
	if resetConsensusMessages == nil {
		return slot.ErrNilResetConsensusMessages
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
	sr.BeginNewSlot(sr.SlotManager().Index(), sr.SlotManager().Timestamp())

	sr.GetAntiFloodHandler().ResetForTopic(common.ConsensusTopic)
	sr.resetConsensusMessages()
	return true
}

// doStartSlotConsensusCheck method checks if the consensus is achieved in the subslot StartSlot
func (sr *subslotStartSlot) doStartSlotConsensusCheck() bool {
	sr.RLockSlotState()
	canceled := sr.SlotCanceled
	sr.RUnlockSlotState()
	if canceled {
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

		sr.SetSlotCanceled(true)

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

		sr.SetSlotCanceled(true)

		return false
	}

	msg := "(not my turn)"
	if leader == sr.SelfPubKey() {
		sr.AppStatusHandler().Increment(core.MetricCountLeader)
		sr.AppStatusHandler().SetStringValue(core.MetricConsensusSlotState, "proposed")
		sr.AppStatusHandler().SetStringValue(core.MetricConsensusState, "proposer")
		msg = "(my turn)"
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

		err = sr.MultiSigner().Reset(pubKeys, uint16(selfIndex)) // #nosec G115
		if err != nil {
			log.Debug("initCurrentSlot.Reset", "error", err.Error())

			sr.SetSlotCanceled(true)

			return false
		}
	}

	sr.RLockSlotState()
	startTime := sr.SlotTimestamp
	sr.RUnlockSlotState()
	maxTime := sr.SlotManager().TimeDuration() * time.Duration(sr.processingThresholdPercentage) / 100
	if sr.SlotManager().RemainingTime(startTime, maxTime) < 0 {
		log.Debug("canceled slot, time is out",
			"slot", sr.SyncTimer().FormattedCurrentTime(), sr.SlotManager().Index(),
			"subslot", sr.Name())

		sr.SetSlotCanceled(true)

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

	sr.RLockSlotState()
	currentSlotIndex := sr.SlotIndex
	sr.RUnlockSlotState()
	nextConsensusGroup, err := sr.GetNextConsensusGroup(
		randomSeed,
		tools.SafeI64ToU64(currentSlotIndex),
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
// It refreshes the elected nodes set from the coordinator so that
// IsNodeInConsensusGroup accepts validators from the new epoch even while the
// node is still syncing (initCurrentSlot is gated on NsSynchronized and would
// leave the set stale otherwise).
// The elected-set swap and resetConsensusMessages are not serialized with
// ProcessReceivedMessage, which checks the set and increments the per-key
// message budget in its own critical sections. A message landing between the
// two therefore costs at most one extra accepted message for one key in the
// transition slot; serializing would mean holding a lock across the full
// validity pipeline including BLS verification, on the consensus hot path.
// The action also re-fires when fork recovery reverts through an epoch-start
// block (trigger.RevertStateToBlock calls SetProcessed again). On a live,
// synchronized validator that widens the set and clears the budgets mid-slot;
// this is bounded the same way: initCurrentSlot re-narrows the set at the next
// slot and every state-changing path re-validates against the tight consensus
// group (ConsensusGroupIndex, SetJobDone).
func (sr *subslotStartSlot) EpochStartAction(hdr data.HeaderHandler) {
	log.Debug(fmt.Sprintf("epoch %d start action in consensus", hdr.GetEpoch()))

	whitelisted, err := sr.NodesCoordinator().GetConsensusWhitelistedNodes(hdr.GetEpoch())
	if err != nil {
		// without the refresh a syncing node keeps rejecting new-epoch
		// validators (the exact stall this action exists to prevent), so
		// failing here must be loud
		log.Error("EpochStartAction: could not refresh elected nodes",
			"epoch", hdr.GetEpoch(),
			"error", err.Error())
		return
	}

	sr.RefreshElectedNodes(whitelisted)
	sr.resetConsensusMessages()
}

// NotifyOrder returns the notification order for a start of epoch event
func (sr *subslotStartSlot) NotifyOrder() uint32 {
	return core.ConsensusOrder
}
