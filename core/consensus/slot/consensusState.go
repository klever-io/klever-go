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

// ConsensusState defines the data needed by slot to do the consensus in each slot
type ConsensusState struct {
	// hold the data on which validators do the consensus (could be for example a hash of the block header
	// proposed by the leader)
	Data   []byte
	Header data.HeaderHandler

	receivedHeaders    []data.HeaderHandler
	mutReceivedHeaders sync.RWMutex

	SlotIndex                   int64
	SlotTimestamp               time.Time
	SlotCanceled                bool
	ExtendedCalled              bool
	WaitingAllSignaturesTimeOut bool

	processingBlock    bool
	mutProcessingBlock sync.RWMutex

	*slotConsensus
	*slotThreshold
	*slotStatus
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

// ResetConsensusState method resets all the consensus data
func (cns *ConsensusState) ResetConsensusState() {
	cns.Header = nil
	cns.Data = nil

	cns.initReceivedHeaders()

	cns.SlotCanceled = false
	cns.ExtendedCalled = false
	cns.WaitingAllSignaturesTimeOut = false

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
	if cns.consensusGroup == nil {
		return "", ErrNilConsensusGroup
	}

	if len(cns.consensusGroup) == 0 {
		return "", ErrEmptyConsensusGroup
	}

	return cns.consensusGroup[0], nil
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

// IsConsensusDataSet method returns true if the consensus data for the current slot is set and false otherwise
func (cns *ConsensusState) IsConsensusDataSet() bool {
	isConsensusDataSet := cns.Data != nil

	return isConsensusDataSet
}

// IsConsensusDataEqual method returns true if the consensus data for the current slot is the same with the given
// one and false otherwise
func (cns *ConsensusState) IsConsensusDataEqual(data []byte) bool {
	isConsensusDataEqual := bytes.Equal(cns.Data, data)

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

// IsHeaderAlreadyReceived method returns true if header is already received and false otherwise
func (cns *ConsensusState) IsHeaderAlreadyReceived() bool {
	isHeaderAlreadyReceived := cns.Header != nil

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
	// generate bitmap according to set commitment hashes
	sizeConsensus := len(cns.ConsensusGroup())

	bitmapSize := sizeConsensus / 8
	if sizeConsensus%8 != 0 {
		bitmapSize++
	}
	bitmap := make([]byte, bitmapSize)

	for i := 0; i < sizeConsensus; i++ {
		pubKey := cns.ConsensusGroup()[i]
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

// GetData gets the Data of the consensusState
func (cns *ConsensusState) GetData() []byte {
	return cns.Data
}
