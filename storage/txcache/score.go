package txcache

import (
	"math"
)

var _ scoreComputer = (*defaultScoreComputer)(nil)

type senderScoreParams struct {
	count uint64
	// Size is in bytes
	size uint64
	// Fees
	fees uint64
}

type defaultScoreComputer struct {
	ppuDivider uint64
}

func newDefaultScoreComputer() *defaultScoreComputer {
	ppuScoreDivider := uint64(1_000_000)

	return &defaultScoreComputer{
		ppuDivider: ppuScoreDivider,
	}
}

// computeScore computes the score of the sender, as an integer 0-100
func (computer *defaultScoreComputer) computeScore(scoreParams senderScoreParams) uint32 {
	rawScore := computer.computeRawScore(scoreParams)
	truncatedScore := uint32(rawScore)
	return truncatedScore
}

// TODO (optimization): switch to integer operations (as opposed to float operations).
func (computer *defaultScoreComputer) computeRawScore(params senderScoreParams) float64 {
	allParamsDefined := params.fees > 0 && params.size > 0 && params.count > 0
	if !allParamsDefined {
		return 0
	}

	// Compute magnitude accordingly with the size of the fee
	shiftMagnitude := computeShiftMagnitude(params.fees)
	if shiftMagnitude < 100 {
		shiftMagnitude = 100
	}
	ppuScoreAdjusted := math.Log(float64(params.fees)) * float64(shiftMagnitude) / 100_000

	// Apply logaritimic on the tx count
	countPow2 := float64(params.count) * float64(params.count)
	countScore := math.Log(countPow2+1) + 1

	// We use size in ~kB
	const bytesInKB = 1000
	size := float64(params.size) / bytesInKB
	sizePow2 := size * size
	sizeScore := math.Log(sizePow2+1) + 1

	rawScore := (ppuScoreAdjusted * sizeScore) / countScore

	// We apply the logistic function,
	// and then subtract 0.5, since we only deal with positive scores,
	// and then we multiply by 2, to have full [0..1] range.
	asymptoticScore := (1/(1+math.Exp(-rawScore)) - 0.5) * 2
	score := asymptoticScore * float64(numberOfScoreChunks)

	return score
}

// returns the maximum shift magnitude of the number in order to maintain the given binary resolution
func computeShiftMagnitude(x uint64) uint64 {
	m := uint64(0)
	stopCondition := uint64(1) << 12
	shiftStep := uint64(1)
	incrementor := uint64(7)

	for i := x; i > stopCondition; i >>= shiftStep {
		m += incrementor
	}

	return m * m
}
