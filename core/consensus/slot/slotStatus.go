package slot

import (
	"sync"
)

// SubslotStatus defines the type used to refer the state of the current subslot
type SubslotStatus int

const (
	// SsNotFinished defines the un-finished state of the subslot
	SsNotFinished SubslotStatus = iota
	// SsFinished defines the finished state of the subslot
	SsFinished
)

// slotStatus defines the data needed by spos to know the state of each subslot in the current slot
type slotStatus struct {
	status map[int]SubslotStatus
	mut    sync.RWMutex
}

// NewSlotStatus creates a new slotStatus object
func NewSlotStatus() *slotStatus {
	rstatus := slotStatus{}
	rstatus.status = make(map[int]SubslotStatus)
	return &rstatus
}

// Status returns the status of the given subslot id
func (rstatus *slotStatus) Status(subslotId int) SubslotStatus {
	rstatus.mut.RLock()
	retcode := rstatus.status[subslotId]
	rstatus.mut.RUnlock()
	return retcode
}

// SetStatus sets the status of the given subslot id
func (rstatus *slotStatus) SetStatus(subslotId int, subslotStatus SubslotStatus) {
	rstatus.mut.Lock()
	rstatus.status[subslotId] = subslotStatus
	rstatus.mut.Unlock()
}

// ResetSlotStatus method resets the state of each subslot
func (rstatus *slotStatus) ResetSlotStatus() {
	rstatus.mut.Lock()

	for k := range rstatus.status {
		rstatus.status[k] = SsNotFinished
	}

	rstatus.mut.Unlock()
}
