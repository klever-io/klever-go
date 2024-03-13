package peer

type validatorCounters struct {
	leaderIncreaseCount    uint32
	leaderDecreaseCount    uint32
	validatorIncreaseCount uint32
	validatorDecreaseCount uint32
}

type validatorSlotCounters map[string]*validatorCounters

func (vrc *validatorSlotCounters) reset() {
	*vrc = make(validatorSlotCounters)
}

func (vrc validatorSlotCounters) increaseValidator(key []byte) {
	vrc.get(key).validatorIncreaseCount++
}

func (vrc validatorSlotCounters) decreaseValidator(key []byte) {
	vrc.get(key).validatorDecreaseCount++
}

func (vrc validatorSlotCounters) increaseLeader(key []byte) {
	vrc.get(key).leaderIncreaseCount++
}

func (vrc validatorSlotCounters) decreaseLeader(key []byte) {
	vrc.get(key).leaderDecreaseCount++
}

func (vrc validatorSlotCounters) get(key []byte) *validatorCounters {
	vdCounter, ok := vrc[string(key)]
	if !ok {
		vrc[string(key)] = &validatorCounters{}
		return vrc[string(key)]
	}

	return vdCounter
}
