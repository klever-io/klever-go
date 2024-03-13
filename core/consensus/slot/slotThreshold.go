package slot

import (
	"sync"
)

// slotThreshold defines the minimum agreements needed for each subslot to consider the subslot finished.
// (Ex: PBFT threshold has 2 / 3 + 1 agreements)
type slotThreshold struct {
	threshold         map[int]int
	fallbackThreshold map[int]int
	mut               sync.RWMutex
}

// NewSlotThreshold creates a new slotThreshold object
func NewSlotThreshold() *slotThreshold {
	rthr := slotThreshold{}
	rthr.threshold = make(map[int]int)
	rthr.fallbackThreshold = make(map[int]int)
	return &rthr
}

// Threshold returns the threshold of agreements needed in the given subslot id
func (rthr *slotThreshold) Threshold(subslotId int) int {
	rthr.mut.RLock()
	retcode := rthr.threshold[subslotId]
	rthr.mut.RUnlock()
	return retcode
}

// SetThreshold sets the threshold of agreements needed in the given subslot id
func (rthr *slotThreshold) SetThreshold(subslotId int, threshold int) {
	rthr.mut.Lock()
	rthr.threshold[subslotId] = threshold
	rthr.mut.Unlock()
}

// FallbackThreshold returns the fallback threshold of agreements needed in the given subslot id
func (rthr *slotThreshold) FallbackThreshold(subslotId int) int {
	rthr.mut.RLock()
	retcode := rthr.fallbackThreshold[subslotId]
	rthr.mut.RUnlock()
	return retcode
}

// SetFallbackThreshold sets the fallback threshold of agreements needed in the given subslot id
func (rthr *slotThreshold) SetFallbackThreshold(subslotId int, threshold int) {
	rthr.mut.Lock()
	rthr.fallbackThreshold[subslotId] = threshold
	rthr.mut.Unlock()
}
