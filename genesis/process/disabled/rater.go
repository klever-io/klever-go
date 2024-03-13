package disabled

// Rater -
type Rater struct {
	StartRating       uint32
	MinRating         uint32
	MaxRating         uint32
	Chance            uint32
	IncreaseProposer  int32
	DecreaseProposer  int32
	IncreaseValidator int32
	DecreaseValidator int32
}

// GetRating -
func (rm *Rater) GetRating(pk string) uint32 {
	return 0
}

// GetStartRating -
func (rm *Rater) GetStartRating() uint32 {
	return 0
}

// GetSignedBlocksThreshold -
func (rm *Rater) GetSignedBlocksThreshold() float32 {
	return 0
}

// ComputeIncreaseProposer -
func (rm *Rater) ComputeIncreaseProposer(currentRating uint32) uint32 {
	return 0
}

// ComputeDecreaseProposer -
func (rm *Rater) ComputeDecreaseProposer(currentRating uint32, consecutiveMisses uint32) uint32 {
	return 0
}

// RevertIncreaseValidator -
func (rm *Rater) RevertIncreaseValidator(currentRating uint32, nrReverts uint32) uint32 {
	return 0
}

// ComputeIncreaseValidator -
func (rm *Rater) ComputeIncreaseValidator(currentRating uint32) uint32 {
	return 0
}

// ComputeDecreaseValidator -
func (rm *Rater) ComputeDecreaseValidator(currentRating uint32) uint32 {
	return 0
}

// GetChance -
func (rm *Rater) GetChance(rating uint32) uint32 {
	return 0
}

// IsInterfaceNil -
func (rm *Rater) IsInterfaceNil() bool {
	return rm == nil
}
