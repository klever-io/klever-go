package mock

import "github.com/klever-io/klever-go/tools"

// RaterMock -
type RaterMock struct {
	StartRating       uint32
	MinRating         uint32
	MaxRating         uint32
	Chance            uint32
	IncreaseProposer  int32
	DecreaseProposer  int32
	IncreaseValidator int32
	DecreaseValidator int32

	GetRatingCalled                func(string) uint32
	GetStartRatingCalled           func() uint32
	GetSignedBlocksThresholdCalled func() float32
	ComputeIncreaseProposerCalled  func(rating uint32) uint32
	ComputeDecreaseProposerCalled  func(rating uint32, consecutiveMissedBlocks uint32) uint32
	RevertIncreaseProposerCalled   func(rating uint32, nrReverts uint32) uint32
	ComputeIncreaseValidatorCalled func(rating uint32) uint32
	ComputeDecreaseValidatorCalled func(rating uint32) uint32
	GetChancesCalled               func(val uint32) uint32
}

// GetNewMockRater -
func GetNewMockRater() *RaterMock {
	raterMock := &RaterMock{}
	raterMock.GetRatingCalled = func(s string) uint32 {
		return raterMock.StartRating
	}
	raterMock.GetStartRatingCalled = func() uint32 {
		return raterMock.StartRating
	}
	raterMock.ComputeIncreaseProposerCalled = func(rating uint32) uint32 {
		ratingStep := raterMock.IncreaseProposer
		return raterMock.computeRating(rating, ratingStep)
	}
	raterMock.RevertIncreaseProposerCalled = func(rating uint32, nrReverts uint32) uint32 {
		ratingStep := raterMock.IncreaseValidator
		computedStep := -ratingStep * tools.SafeU32ToI32(nrReverts)
		return raterMock.computeRating(rating, computedStep)
	}
	raterMock.ComputeDecreaseProposerCalled = func(rating uint32, consecutiveMissedBlocks uint32) uint32 {
		ratingStep := raterMock.DecreaseProposer
		return raterMock.computeRating(rating, ratingStep)
	}
	raterMock.ComputeIncreaseValidatorCalled = func(rating uint32) uint32 {
		ratingStep := raterMock.IncreaseValidator
		return raterMock.computeRating(rating, ratingStep)
	}
	raterMock.ComputeDecreaseValidatorCalled = func(rating uint32) uint32 {
		ratingStep := raterMock.DecreaseValidator
		return raterMock.computeRating(rating, ratingStep)
	}
	raterMock.GetChancesCalled = func(val uint32) uint32 {
		return raterMock.Chance
	}
	return raterMock
}

func (rm *RaterMock) computeRating(rating uint32, ratingStep int32) uint32 {
	newVal := int64(rating) + int64(ratingStep)
	if newVal < int64(rm.MinRating) {
		return rm.MinRating
	}
	if newVal > int64(rm.MaxRating) {
		return rm.MaxRating
	}

	return tools.SafeI64ToU32(newVal)
}

// GetRating -
func (rm *RaterMock) GetRating(pk string) uint32 {
	return rm.GetRatingCalled(pk)
}

// GetStartRating -
func (rm *RaterMock) GetStartRating() uint32 {
	if rm.GetStartRatingCalled != nil {
		return rm.GetStartRatingCalled()
	}
	return 10
}

// GetSignedBlocksThreshold -
func (rm *RaterMock) GetSignedBlocksThreshold() float32 {
	return rm.GetSignedBlocksThresholdCalled()
}

// ComputeIncreaseProposer -
func (rm *RaterMock) ComputeIncreaseProposer(currentRating uint32) uint32 {
	return rm.ComputeIncreaseProposerCalled(currentRating)
}

// ComputeDecreaseProposer -
func (rm *RaterMock) ComputeDecreaseProposer(currentRating uint32, consecutiveMisses uint32) uint32 {
	return rm.ComputeDecreaseProposerCalled(currentRating, consecutiveMisses)
}

// RevertIncreaseValidator -
func (rm *RaterMock) RevertIncreaseValidator(currentRating uint32, nrReverts uint32) uint32 {
	return rm.RevertIncreaseProposerCalled(currentRating, nrReverts)
}

// ComputeIncreaseValidator -
func (rm *RaterMock) ComputeIncreaseValidator(currentRating uint32) uint32 {
	return rm.ComputeIncreaseValidatorCalled(currentRating)
}

// ComputeDecreaseValidator -
func (rm *RaterMock) ComputeDecreaseValidator(currentRating uint32) uint32 {
	return rm.ComputeDecreaseValidatorCalled(currentRating)
}

// GetChance -
func (rm *RaterMock) GetChance(rating uint32) uint32 {
	return rm.GetChancesCalled(rating)
}

// IsInterfaceNil -
func (rm *RaterMock) IsInterfaceNil() bool {
	return rm == nil
}
