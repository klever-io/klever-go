package chronology

func (chr *chronology) StartSlot() {
	chr.startSlot()
}

func (chr *chronology) CurrentSlot() int64 {
	return chr.currentSlot
}

func (chr *chronology) SubslotID() int {
	return chr.subslotID
}

func (chr *chronology) SetSubslotID(subslotID int) {
	chr.subslotID = subslotID
}

func (chr *chronology) SetCurrentSlot(id int64) {
	chr.currentSlot = id
}

func (chr *chronology) UpdateSlot() {
	chr.updateSlot()
}

func (chr *chronology) InitSlot() {
	chr.initSlot()
}
