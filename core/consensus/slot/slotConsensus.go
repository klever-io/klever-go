package slot

import (
	"sync"
)

// slotConsensus defines the data needed by spos to do the consensus in each slot
type slotConsensus struct {
	electedNodes        map[string]struct{}
	mutElected          sync.RWMutex
	consensusGroup      []string
	consensusGroupSize  int
	selfPubKey          string
	validatorSlotStates map[string]*slotState
	mut                 sync.RWMutex
}

// NewSlotConsensus creates a new slotConsensus object
func NewSlotConsensus(
	electedNodes map[string]struct{},
	consensusGroupSize int,
	selfId string,
) *slotConsensus {

	rcns := slotConsensus{
		electedNodes:       electedNodes,
		consensusGroupSize: consensusGroupSize,
		selfPubKey:         selfId,
		mutElected:         sync.RWMutex{},
	}

	rcns.validatorSlotStates = make(map[string]*slotState)

	return &rcns
}

// ConsensusGroupIndex returns the index of given public key in the current consensus group
func (rcns *slotConsensus) ConsensusGroupIndex(pubKey string) (int, error) {
	for i, pk := range rcns.consensusGroup {
		if pk == pubKey {
			return i, nil
		}
	}
	return 0, ErrNotFoundInConsensus
}

// SelfConsensusGroupIndex returns the index of self public key in current consensus group
func (rcns *slotConsensus) SelfConsensusGroupIndex() (int, error) {
	return rcns.ConsensusGroupIndex(rcns.selfPubKey)
}

// SetElectedList sets the elected list ID's
func (rcns *slotConsensus) SetElectedList(electedList map[string]struct{}) {
	rcns.mutElected.Lock()
	rcns.electedNodes = electedList
	rcns.mutElected.Unlock()
}

// ConsensusGroup returns the consensus group ID's
func (rcns *slotConsensus) ConsensusGroup() []string {
	return rcns.consensusGroup
}

// SetConsensusGroup sets the consensus group ID's
func (rcns *slotConsensus) SetConsensusGroup(consensusGroup []string) {
	rcns.consensusGroup = consensusGroup

	rcns.mut.Lock()

	rcns.validatorSlotStates = make(map[string]*slotState)

	for i := 0; i < len(consensusGroup); i++ {
		rcns.validatorSlotStates[rcns.consensusGroup[i]] = NewSlotState()
	}

	rcns.mut.Unlock()
}

// ConsensusGroupSize returns the consensus group size
func (rcns *slotConsensus) ConsensusGroupSize() int {
	return rcns.consensusGroupSize
}

// SetConsensusGroupSize sets the consensus group size
func (rcns *slotConsensus) SetConsensusGroupSize(consensusGroudpSize int) {
	rcns.consensusGroupSize = consensusGroudpSize
}

// SelfPubKey returns selfPubKey ID
func (rcns *slotConsensus) SelfPubKey() string {
	return rcns.selfPubKey
}

// SetSelfPubKey sets selfPubKey ID
func (rcns *slotConsensus) SetSelfPubKey(selfPubKey string) {
	rcns.selfPubKey = selfPubKey
}

// JobDone returns the state of the action done, by the node represented by the key parameter,
// in subslot given by the subslotId parameter
func (rcns *slotConsensus) JobDone(key string, subslotId int) (bool, error) {
	rcns.mut.RLock()
	currentSlotState := rcns.validatorSlotStates[key]

	if currentSlotState == nil {
		rcns.mut.RUnlock()
		return false, ErrInvalidKey
	}

	retcode := currentSlotState.JobDone(subslotId)
	rcns.mut.RUnlock()

	return retcode, nil
}

// SetJobDone set the state of the action done, by the node represented by the key parameter,
// in subslot given by the subslotId parameter
func (rcns *slotConsensus) SetJobDone(key string, subslotId int, value bool) error {
	rcns.mut.Lock()

	currentSlotState := rcns.validatorSlotStates[key]

	if currentSlotState == nil {
		rcns.mut.Unlock()
		return ErrInvalidKey
	}

	currentSlotState.SetJobDone(subslotId, value)
	rcns.mut.Unlock()

	return nil
}

// SelfJobDone returns the self state of the action done in subslot given by the subslotId parameter
func (rcns *slotConsensus) SelfJobDone(subslotId int) (bool, error) {
	return rcns.JobDone(rcns.selfPubKey, subslotId)
}

// SetSelfJobDone set the self state of the action done in subslot given by the subslotId parameter
func (rcns *slotConsensus) SetSelfJobDone(subslotId int, value bool) error {
	return rcns.SetJobDone(rcns.selfPubKey, subslotId, value)
}

// IsNodeInConsensusGroup method checks if the node is part of consensus group of the current slot
func (rcns *slotConsensus) IsNodeInConsensusGroup(node string) bool {
	for i := 0; i < len(rcns.consensusGroup); i++ {
		if rcns.consensusGroup[i] == node {
			return true
		}
	}

	return false
}

// IsNodeInElectedList method checks if the node is part of the elected list
func (rcns *slotConsensus) IsNodeInElectedList(node string) bool {
	rcns.mutElected.RLock()
	_, ok := rcns.electedNodes[node]
	rcns.mutElected.RUnlock()

	return ok
}

// ComputeSize method returns the number of messages received from the nodes belonging to the current jobDone group
// related to this subslot
func (rcns *slotConsensus) ComputeSize(subslotId int) int {
	n := 0

	for i := 0; i < len(rcns.consensusGroup); i++ {
		isJobDone, err := rcns.JobDone(rcns.consensusGroup[i], subslotId)
		if err != nil {
			log.Debug("JobDone", "error", err.Error())
			continue
		}

		if isJobDone {
			n++
		}
	}

	return n
}

// ResetSlotState method resets the state of each node from the current jobDone group, regarding to the
// consensus validatorSlotStates
func (rcns *slotConsensus) ResetSlotState() {
	rcns.mut.Lock()

	var currentSlotState *slotState
	for i := 0; i < len(rcns.consensusGroup); i++ {
		currentSlotState = rcns.validatorSlotStates[rcns.consensusGroup[i]]
		if currentSlotState == nil {
			log.Debug("validatorSlotStates", "error", ErrNilSlotState.Error())
			continue
		}

		currentSlotState.ResetJobsDone()
	}

	rcns.mut.Unlock()
}
