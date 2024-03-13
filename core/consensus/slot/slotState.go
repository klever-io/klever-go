package slot

import (
	"sync"
)

// slotState defines the data needed by spos to know the state of each node from the current jobDone group,
// regarding to the consensus validatorSlotStates in each subslot of the current slot
type slotState struct {
	jobDone map[int]bool
	mut     sync.RWMutex
}

// NewSlotState creates a new slotState object
func NewSlotState() *slotState {
	rstate := slotState{}
	rstate.jobDone = make(map[int]bool)
	return &rstate
}

// JobDone returns the consensus validatorSlotStates of the given subslotId
func (rstate *slotState) JobDone(subslotId int) bool {
	rstate.mut.RLock()
	retcode := rstate.jobDone[subslotId]
	rstate.mut.RUnlock()
	return retcode
}

// SetJobDone sets the consensus validatorSlotStates of the given subslotId
func (rstate *slotState) SetJobDone(subslotId int, value bool) {
	rstate.mut.Lock()
	rstate.jobDone[subslotId] = value
	rstate.mut.Unlock()
}

// ResetJobsDone method resets the consensus validatorSlotStates of each subslot
func (rstate *slotState) ResetJobsDone() {
	rstate.mut.Lock()

	for k := range rstate.jobDone {
		rstate.jobDone[k] = false
	}

	rstate.mut.Unlock()
}
