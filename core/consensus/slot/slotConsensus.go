package slot

import (
	"sync"
)

// slotConsensus defines the data needed by spos to do the consensus in each slot.
//
// Locking layout — three independent mutexes, intentionally separate to keep
// the hot signature-collection path (JobDone / SetJobDone) from contending
// against ConsensusGroup() reads called from every consensus check:
//   - mutElected protects electedNodes
//   - mutConsensusGroup protects consensusGroup
//   - mut           protects validatorSlotStates
//
// SetConsensusGroup installs consensusGroup and validatorSlotStates in two
// separate critical sections (mutConsensusGroup, then mut); the writes are
// intentionally not atomic against combined readers, and a concurrent
// ComputeSize can undercount for one poll cycle before self-healing on the
// next poll. Lock order is fixed: mutConsensusGroup before mut, never the
// reverse, so readers holding mut.RLock can safely call ConsensusGroup().
type slotConsensus struct {
	electedNodes        map[string]struct{}
	mutElected          sync.RWMutex
	consensusGroup      []string
	mutConsensusGroup   sync.RWMutex
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
	rcns.mutConsensusGroup.RLock()
	defer rcns.mutConsensusGroup.RUnlock()
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

// ConsensusGroup returns the consensus group ID's. The slice header is captured
// under mutConsensusGroup.RLock so callers see a stable view even while
// SetConsensusGroup is installing a new group. The returned slice is not
// deep-copied; callers must treat the elements as read-only.
func (rcns *slotConsensus) ConsensusGroup() []string {
	rcns.mutConsensusGroup.RLock()
	cg := rcns.consensusGroup
	rcns.mutConsensusGroup.RUnlock()
	return cg
}

// SetConsensusGroup sets the consensus group ID's
func (rcns *slotConsensus) SetConsensusGroup(consensusGroup []string) {
	// update elected nodes
	rcns.mutElected.Lock()
	rcns.electedNodes = make(map[string]struct{})
	for _, v := range consensusGroup {
		rcns.electedNodes[v] = struct{}{}
	}
	rcns.mutElected.Unlock()

	// Acquire mutConsensusGroup then mut, with each section released before
	// the next is taken. The two writes are NOT atomic against combined
	// readers: a concurrent ComputeSize call can briefly observe the new
	// consensusGroup against the old validatorSlotStates map, which causes
	// JobDone lookups to return ErrInvalidKey and ComputeSize to undercount
	// for one poll cycle. This is benign — the next poll sees consistent
	// state. Holding both locks simultaneously would prevent that window
	// but forces the same lock-order discipline on every reader, which we
	// avoid here so JobDone (hot path) does not contend with ConsensusGroup
	// reads. Lock order is fixed (mutConsensusGroup before mut) so that
	// callers holding mut.RLock can safely call ConsensusGroup() afterward
	// without deadlock; never reverse.
	rcns.mutConsensusGroup.Lock()
	rcns.consensusGroup = consensusGroup
	rcns.mutConsensusGroup.Unlock()

	rcns.mut.Lock()
	rcns.validatorSlotStates = make(map[string]*slotState)
	for i := range consensusGroup {
		rcns.validatorSlotStates[consensusGroup[i]] = NewSlotState()
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

// RefreshElectedNodes replaces the elected nodes map without touching the
// consensus group or validator slot states. Called on epoch start to keep the
// IsNodeInConsensusGroup gate current while the node is still syncing and
// initCurrentSlot (which calls SetConsensusGroup) is not running yet.
// Ownership of elected transfers to the receiver: the caller must not retain or
// write to the map afterwards, since readers access it under mutElected only.
// The sole caller, EpochStartAction, passes the fresh map built by
// GetConsensusWhitelistedNodes.
func (rcns *slotConsensus) RefreshElectedNodes(elected map[string]struct{}) {
	rcns.mutElected.Lock()
	rcns.electedNodes = elected
	rcns.mutElected.Unlock()
}

// IsNodeInConsensusGroup method checks if the node is part of consensus group of the current slot
func (rcns *slotConsensus) IsNodeInConsensusGroup(node string) bool {
	rcns.mutElected.RLock()
	_, ok := rcns.electedNodes[node]
	rcns.mutElected.RUnlock()

	return ok
}

// ComputeSize method returns the number of messages received from the nodes belonging to the current jobDone group
// related to this subslot
func (rcns *slotConsensus) ComputeSize(subslotId int) int {
	n := 0

	cg := rcns.ConsensusGroup()
	for i := range cg {
		isJobDone, err := rcns.JobDone(cg[i], subslotId)
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
	// Snapshot consensusGroup via the getter so we never nest
	// mutConsensusGroup inside mut (which would deadlock against
	// SetConsensusGroup's acquire order).
	cg := rcns.ConsensusGroup()

	rcns.mut.Lock()
	defer rcns.mut.Unlock()

	for _, pk := range cg {
		currentSlotState := rcns.validatorSlotStates[pk]
		if currentSlotState == nil {
			log.Debug("validatorSlotStates", "error", ErrNilSlotState.Error())
			continue
		}

		currentSlotState.ResetJobsDone()
	}
}
