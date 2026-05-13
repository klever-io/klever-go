package slot

import (
	"bytes"
	"sync"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/sharding"
)

var log = logger.GetOrCreate("consensus/slot")

// ConsensusState defines the data needed by slot to do the consensus in each slot.
//
// mutSlotState guards the slot-scoped fields. Fields stay public; callers
// Lock/RLock at handler boundaries. Even chronology must hold the lock —
// interceptor goroutines write Data/Header for followers.
type ConsensusState struct {
	// mutSlotState guards every field in this block.
	mutSlotState sync.RWMutex
	// hold the data on which validators do the consensus (could be for example a hash of the block header
	// proposed by the leader)
	Data                        []byte
	Header                      data.HeaderHandler
	SlotIndex                   int64
	SlotTimestamp               time.Time
	SlotCanceled                bool
	ExtendedCalled              bool
	WaitingAllSignaturesTimeOut bool

	receivedHeaders    []data.HeaderHandler
	mutReceivedHeaders sync.RWMutex

	processingBlock    bool
	mutProcessingBlock sync.RWMutex

	*slotConsensus
	*slotThreshold
	*slotStatus
}

// LockSlotState locks mutSlotState for writing.
func (cns *ConsensusState) LockSlotState() { cns.mutSlotState.Lock() }

// UnlockSlotState unlocks mutSlotState.
func (cns *ConsensusState) UnlockSlotState() { cns.mutSlotState.Unlock() }

// RLockSlotState locks mutSlotState for reading.
func (cns *ConsensusState) RLockSlotState() { cns.mutSlotState.RLock() }

// RUnlockSlotState releases the read lock on mutSlotState.
func (cns *ConsensusState) RUnlockSlotState() { cns.mutSlotState.RUnlock() }

// SetSlotCanceled writes SlotCanceled under mutSlotState.
func (cns *ConsensusState) SetSlotCanceled(canceled bool) {
	cns.mutSlotState.Lock()
	cns.SlotCanceled = canceled
	cns.mutSlotState.Unlock()
}

// SetExtendedCalled writes ExtendedCalled under mutSlotState.
func (cns *ConsensusState) SetExtendedCalled(extended bool) {
	cns.mutSlotState.Lock()
	cns.ExtendedCalled = extended
	cns.mutSlotState.Unlock()
}

// SetWaitingAllSignaturesTimeOut writes WaitingAllSignaturesTimeOut under mutSlotState.
func (cns *ConsensusState) SetWaitingAllSignaturesTimeOut(timedOut bool) {
	cns.mutSlotState.Lock()
	cns.WaitingAllSignaturesTimeOut = timedOut
	cns.mutSlotState.Unlock()
}

// NewConsensusState creates a new ConsensusState object
func NewConsensusState(
	slotConsensus *slotConsensus,
	slotThreshold *slotThreshold,
	slotStatus *slotStatus,
) *ConsensusState {

	cns := ConsensusState{
		slotConsensus: slotConsensus,
		slotThreshold: slotThreshold,
		slotStatus:    slotStatus,
	}

	cns.ResetConsensusState()

	return &cns
}

// resetSlotStateLocked clears slot-scoped consensus fields.
// Caller must hold mutSlotState for writing.
func (cns *ConsensusState) resetSlotStateLocked() {
	cns.Header = nil
	cns.Data = nil
	cns.SlotCanceled = false
	cns.ExtendedCalled = false
	cns.WaitingAllSignaturesTimeOut = false
}

// ResetConsensusState clears slot-scoped state without touching SlotIndex or
// SlotTimestamp. Use at construction only; for slot transitions in the
// chronology, use BeginNewSlot.
func (cns *ConsensusState) ResetConsensusState() {
	cns.mutSlotState.Lock()
	cns.resetSlotStateLocked()
	cns.mutSlotState.Unlock()

	cns.initReceivedHeaders()
	cns.ResetSlotStatus()
	cns.ResetSlotState()
}

// BeginNewSlot atomically transitions ConsensusState into the next slot,
// updating SlotIndex + SlotTimestamp and clearing slot-scoped flags under a
// single mutSlotState write. The atomicity guards waitAllSignatures: if
// SlotIndex were updated separately from the flag reset, a stale goroutine
// from the prior slot could fire in the gap, observe SlotIndex still equal
// to its spawnSlot, and set WaitingAllSignaturesTimeOut on the freshly-cleared
// state — causing the next slot's leader to commit with minimum quorum
// prematurely.
func (cns *ConsensusState) BeginNewSlot(slotIndex int64, slotTimestamp time.Time) {
	cns.mutSlotState.Lock()
	cns.resetSlotStateLocked()
	cns.SlotIndex = slotIndex
	cns.SlotTimestamp = slotTimestamp
	cns.mutSlotState.Unlock()

	cns.initReceivedHeaders()
	cns.ResetSlotStatus()
	cns.ResetSlotState()
}

func (cns *ConsensusState) initReceivedHeaders() {
	cns.mutReceivedHeaders.Lock()
	cns.receivedHeaders = make([]data.HeaderHandler, 0)
	cns.mutReceivedHeaders.Unlock()
}

// AddReceivedHeader append the provided header to the inner received headers list
func (cns *ConsensusState) AddReceivedHeader(headerHandler data.HeaderHandler) {
	cns.mutReceivedHeaders.Lock()
	cns.receivedHeaders = append(cns.receivedHeaders, headerHandler)
	cns.mutReceivedHeaders.Unlock()
}

// GetReceivedHeaders returns the received headers list
func (cns *ConsensusState) GetReceivedHeaders() []data.HeaderHandler {
	cns.mutReceivedHeaders.RLock()
	receivedHeaders := cns.receivedHeaders
	cns.mutReceivedHeaders.RUnlock()

	return receivedHeaders
}

// IsNodeLeaderInCurrentSlot method checks if the given node is leader in the current slot
func (cns *ConsensusState) IsNodeLeaderInCurrentSlot(node string) bool {
	leader, err := cns.GetLeader()
	if err != nil {
		log.Debug("GetLeader", "error", err.Error())
		return false
	}

	return leader == node
}

// IsSelfLeaderInCurrentSlot method checks if the current node is leader in the current slot
func (cns *ConsensusState) IsSelfLeaderInCurrentSlot() bool {
	return cns.IsNodeLeaderInCurrentSlot(cns.selfPubKey)
}

// GetLeader method gets the leader of the current slot
func (cns *ConsensusState) GetLeader() (string, error) {
	cg := cns.ConsensusGroup()
	if cg == nil {
		return "", ErrNilConsensusGroup
	}

	if len(cg) == 0 {
		return "", ErrEmptyConsensusGroup
	}

	return cg[0], nil
}

// GetNextConsensusGroup gets the new consensus group for the current slot based on current eligible list and a random
// source for the new selection
func (cns *ConsensusState) GetNextConsensusGroup(
	randomSource []byte,
	slot uint64,
	nodesCoordinator sharding.NodesCoordinator,
	epoch uint32,
) ([]string, error) {
	validatorsGroup, err := nodesCoordinator.ComputeConsensusGroup(randomSource, slot, epoch)
	if err != nil {
		log.Debug(
			"compute consensus group",
			"error", err.Error(),
			"randomSource", randomSource,
			"slot", slot,
			"epoch", epoch,
		)
		return nil, err
	}

	consensusSize := len(validatorsGroup)
	newConsensusGroup := make([]string, consensusSize)

	for i := 0; i < consensusSize; i++ {
		newConsensusGroup[i] = string(validatorsGroup[i].PubKey())
	}

	return newConsensusGroup, nil
}

// IsConsensusDataSet method returns true if the consensus data for the current slot is set and false otherwise.
func (cns *ConsensusState) IsConsensusDataSet() bool {
	cns.mutSlotState.RLock()
	isConsensusDataSet := cns.Data != nil
	cns.mutSlotState.RUnlock()

	return isConsensusDataSet
}

// IsConsensusDataEqual method returns true if the consensus data for the current slot is the same with the given
// one and false otherwise.
func (cns *ConsensusState) IsConsensusDataEqual(data []byte) bool {
	cns.mutSlotState.RLock()
	isConsensusDataEqual := bytes.Equal(cns.Data, data)
	cns.mutSlotState.RUnlock()

	return isConsensusDataEqual
}

// IsJobDone method returns true if the node job for the current subslot is done and false otherwise
func (cns *ConsensusState) IsJobDone(node string, currentSubslotId int) bool {
	jobDone, err := cns.JobDone(node, currentSubslotId)
	if err != nil {
		log.Debug("JobDone", "error", err.Error())
		return false
	}

	return jobDone
}

// IsSelfJobDone method returns true if self job for the current subslot is done and false otherwise
func (cns *ConsensusState) IsSelfJobDone(currentSubslotId int) bool {
	return cns.IsJobDone(cns.selfPubKey, currentSubslotId)
}

// IsSubslotFinished method returns true if the current subslot is finished and false otherwise
func (cns *ConsensusState) IsSubslotFinished(subslotID int) bool {
	isSubslotFinished := cns.Status(subslotID) == SsFinished

	return isSubslotFinished
}

// IsNodeSelf method returns true if the message is received from itself and false otherwise
func (cns *ConsensusState) IsNodeSelf(node string) bool {
	isNodeSelf := node == cns.SelfPubKey()

	return isNodeSelf
}

// IsHeaderAlreadyReceived method returns true if header is already received and false otherwise.
func (cns *ConsensusState) IsHeaderAlreadyReceived() bool {
	cns.mutSlotState.RLock()
	isHeaderAlreadyReceived := cns.Header != nil
	cns.mutSlotState.RUnlock()

	return isHeaderAlreadyReceived
}

// CanDoSubslotJob method returns true if the job of the subslot can be done and false otherwise
func (cns *ConsensusState) CanDoSubslotJob(currentSubslotId int) bool {
	if !cns.IsConsensusDataSet() {
		return false
	}

	if cns.IsSelfJobDone(currentSubslotId) {
		return false
	}

	if cns.IsSubslotFinished(currentSubslotId) {
		return false
	}

	return true
}

// CanProcessReceivedMessage method returns true if the message received can be processed and false otherwise
func (cns *ConsensusState) CanProcessReceivedMessage(cnsDta *consensus.Message, currentSlotIndex int64,
	currentSubslotId int) bool {
	if cns.IsNodeSelf(string(cnsDta.PubKey)) {
		return false
	}

	if currentSlotIndex != cnsDta.SlotIndex {
		return false
	}

	if cns.IsJobDone(string(cnsDta.PubKey), currentSubslotId) {
		return false
	}

	if cns.IsSubslotFinished(currentSubslotId) {
		return false
	}

	return true
}

// GenerateBitmap method generates a bitmap, for a given subslot, in which each node will be marked with 1
// if its job has been done
func (cns *ConsensusState) GenerateBitmap(subslotId int) []byte {
	consensusGroup := cns.ConsensusGroup()
	// generate bitmap according to set commitment hashes
	sizeConsensus := len(consensusGroup)

	bitmapSize := sizeConsensus / 8
	if sizeConsensus%8 != 0 {
		bitmapSize++
	}
	bitmap := make([]byte, bitmapSize)

	for i := 0; i < sizeConsensus; i++ {
		pubKey := consensusGroup[i]
		isJobDone, err := cns.JobDone(pubKey, subslotId)
		if err != nil {
			log.Debug("JobDone", "error", err.Error())
			continue
		}

		if isJobDone {
			bitmap[i/8] |= 1 << (uint16(i) % 8) // #nosec G115
		}
	}

	return bitmap
}

// ProcessingBlock gets the state of block processing
func (cns *ConsensusState) ProcessingBlock() bool {
	cns.mutProcessingBlock.RLock()
	processingBlock := cns.processingBlock
	cns.mutProcessingBlock.RUnlock()
	return processingBlock
}

// SetProcessingBlock sets the state of block processing
func (cns *ConsensusState) SetProcessingBlock(processingBlock bool) {
	cns.mutProcessingBlock.Lock()
	cns.processingBlock = processingBlock
	cns.mutProcessingBlock.Unlock()
}

// GetData returns the consensus data slice header. The slice contents must not be mutated by callers.
func (cns *ConsensusState) GetData() []byte {
	cns.mutSlotState.RLock()
	data := cns.Data
	cns.mutSlotState.RUnlock()
	return data
}
