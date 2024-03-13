package redundancy

// GetMaxSlotsOfInactivityAccepted -
func GetMaxSlotsOfInactivityAccepted() uint64 {
	return maxSlotsOfInactivityAccepted
}

// GetSlotsOfInactivity -
func (nr *nodeRedundancy) GetSlotsOfInactivity() uint64 {
	return nr.slotsOfInactivity
}

// SetSlotsOfInactivity -
func (nr *nodeRedundancy) SetSlotsOfInactivity(slotsOfInactivity uint64) {
	nr.slotsOfInactivity = slotsOfInactivity
}

// GetLastSlotIndexCheck -
func (nr *nodeRedundancy) GetLastSlotIndexCheck() int64 {
	return nr.lastSlotIndexCheck
}

// SetLastSlotIndexCheck -
func (nr *nodeRedundancy) SetLastSlotIndexCheck(lastSlotIndexCheck int64) {
	nr.lastSlotIndexCheck = lastSlotIndexCheck
}
