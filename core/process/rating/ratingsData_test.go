package rating

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	validatorIncreaseRatingStep = int32(2)
	validatorDecreaseRatingStep = int32(-8)
	proposerIncreaseRatingStep  = int32(6)
	proposerDecreaseRatingStep  = int32(-24)

	signedBlocksThreshold          = 0.025
	consecutiveMissedBlocksPenalty = 1.1

	minNodes                 = 6
	consensusSize            = 3
	slotDurationMilliseconds = 6000
)

func createDymmyRatingsData() RatingsDataArg {
	return RatingsDataArg{
		Config:                   config.RatingsConfig{},
		ConsensusSize:            consensusSize,
		MinNodes:                 minNodes,
		SlotDurationMilliseconds: slotDurationMilliseconds,
	}
}

func createDummyRatingsConfig() config.RatingsConfig {
	return config.RatingsConfig{
		General: config.General{
			StartRating:           4000,
			MaxRating:             10000,
			MinRating:             1,
			SignedBlocksThreshold: signedBlocksThreshold,
			SelectionChances: []*config.SelectionChance{
				{MaxThreshold: 0, ChancePercent: 5},
				{MaxThreshold: 25, ChancePercent: 19},
				{MaxThreshold: 75, ChancePercent: 20},
				{MaxThreshold: 100, ChancePercent: 21},
			},
		},

		RatingSteps: config.RatingSteps{
			HoursToMaxRatingFromStartRating: 2,
			ProposerValidatorImportance:     1,
			ProposerDecreaseFactor:          -4,
			ValidatorDecreaseFactor:         -4,
			ConsecutiveMissedBlocksPenalty:  consecutiveMissedBlocksPenalty,
		},
	}
}

func TestRatingsData_RatingsDataMinGreaterMaxShouldErr(t *testing.T) {
	t.Parallel()

	ratingsDataArg := createDymmyRatingsData()
	ratingsConfig := createDummyRatingsConfig()
	ratingsConfig.General.MinRating = 10
	ratingsConfig.General.MaxRating = 8

	ratingsDataArg.Config = ratingsConfig
	ratingsData, err := NewRatingsData(ratingsDataArg)

	assert.Nil(t, ratingsData)
	assert.True(t, errors.Is(err, process.ErrMaxRatingIsSmallerThanMinRating))
}

func TestRatingsData_RatingsDataMinSmallerThanOne(t *testing.T) {
	t.Parallel()

	ratingsDataArg := createDymmyRatingsData()
	ratingsConfig := createDummyRatingsConfig()
	ratingsConfig.General.MinRating = 0
	ratingsConfig.General.MaxRating = 8
	ratingsDataArg.Config = ratingsConfig
	ratingsData, err := NewRatingsData(ratingsDataArg)

	assert.Nil(t, ratingsData)
	assert.Equal(t, process.ErrMinRatingSmallerThanOne, err)
}

func TestRatingsData_RatingsStartGreaterMaxShouldErr(t *testing.T) {
	t.Parallel()

	ratingsDataArg := createDymmyRatingsData()
	ratingsConfig := createDummyRatingsConfig()
	ratingsConfig.General.MinRating = 10
	ratingsConfig.General.MaxRating = 100
	ratingsConfig.General.StartRating = 110
	ratingsDataArg.Config = ratingsConfig
	ratingsData, err := NewRatingsData(ratingsDataArg)

	assert.Nil(t, ratingsData)
	assert.True(t, errors.Is(err, process.ErrStartRatingNotBetweenMinAndMax))
}

func TestRatingsData_RatingsStartLowerMinShouldErr(t *testing.T) {
	t.Parallel()

	ratingsDataArg := createDymmyRatingsData()
	ratingsConfig := createDummyRatingsConfig()
	ratingsConfig.General.MinRating = 10
	ratingsConfig.General.MaxRating = 100
	ratingsConfig.General.StartRating = 5
	ratingsDataArg.Config = ratingsConfig
	ratingsData, err := NewRatingsData(ratingsDataArg)

	assert.Nil(t, ratingsData)
	assert.True(t, errors.Is(err, process.ErrStartRatingNotBetweenMinAndMax))
}

func TestRatingsData_RatingsSignedBlocksThresholdNotBetweenZeroAndOneShouldErr(t *testing.T) {
	t.Parallel()

	ratingsDataArg := createDymmyRatingsData()
	ratingsConfig := createDummyRatingsConfig()
	ratingsConfig.General.SignedBlocksThreshold = -0.1
	ratingsDataArg.Config = ratingsConfig
	ratingsData, err := NewRatingsData(ratingsDataArg)

	assert.Nil(t, ratingsData)
	assert.True(t, errors.Is(err, process.ErrSignedBlocksThresholdNotBetweenZeroAndOne))

	ratingsConfig.General.SignedBlocksThreshold = 1.01
	ratingsDataArg.Config = ratingsConfig
	ratingsData, err = NewRatingsData(ratingsDataArg)

	assert.Nil(t, ratingsData)
	assert.True(t, errors.Is(err, process.ErrSignedBlocksThresholdNotBetweenZeroAndOne))
}

func TestRatingsData_RatingsConsecutiveMissedBlocksPenaltyLowerThanOneShouldErr(t *testing.T) {
	t.Parallel()

	ratingsDataArg := createDymmyRatingsData()
	ratingsConfig := createDummyRatingsConfig()
	ratingsConfig.RatingSteps.ConsecutiveMissedBlocksPenalty = 0.9
	ratingsDataArg.Config = ratingsConfig
	ratingsData, err := NewRatingsData(ratingsDataArg)

	require.Nil(t, ratingsData)
	require.True(t, errors.Is(err, process.ErrConsecutiveMissedBlocksPenaltyLowerThanOne))
}

func TestRatingsData_HoursToMaxRatingFromStartRatingZeroErr(t *testing.T) {
	t.Parallel()

	ratingsDataArg := createDymmyRatingsData()
	ratingsConfig := createDummyRatingsConfig()
	ratingsConfig.RatingSteps.HoursToMaxRatingFromStartRating = 0
	ratingsDataArg.Config = ratingsConfig
	ratingsData, err := NewRatingsData(ratingsDataArg)

	require.Nil(t, ratingsData)
	require.True(t, errors.Is(err, process.ErrHoursToMaxRatingFromStartRatingZero))
}

func TestRatingsData_PositiveDecreaseRatingsStepsShouldErr(t *testing.T) {
	t.Parallel()

	ratingsDataArg := createDymmyRatingsData()
	ratingsConfig := createDummyRatingsConfig()
	ratingsConfig.RatingSteps.ProposerDecreaseFactor = -0.5
	ratingsDataArg.Config = ratingsConfig
	ratingsData, err := NewRatingsData(ratingsDataArg)

	require.Nil(t, ratingsData)
	require.True(t, errors.Is(err, process.ErrDecreaseRatingsStepMoreThanMinusOne))
	require.True(t, strings.Contains(err.Error(), "greater than"))
}

func TestRatingsData_UnderflowErr(t *testing.T) {
	t.Parallel()

	ratingsDataArg := createDymmyRatingsData()
	ratingsConfig := createDummyRatingsConfig()
	ratingsConfig.RatingSteps.ProposerDecreaseFactor = math.MinInt32
	ratingsDataArg.Config = ratingsConfig
	ratingsData, err := NewRatingsData(ratingsDataArg)

	require.Nil(t, ratingsData)
	require.True(t, errors.Is(err, process.ErrOverflow))
	require.True(t, strings.Contains(err.Error(), "proposerDecrease"))

	ratingsDataArg = createDymmyRatingsData()
	ratingsConfig = createDummyRatingsConfig()
	ratingsConfig.RatingSteps.ValidatorDecreaseFactor = math.MinInt32
	ratingsDataArg.Config = ratingsConfig
	ratingsData, err = NewRatingsData(ratingsDataArg)

	require.Nil(t, ratingsData)
	require.True(t, errors.Is(err, process.ErrOverflow))
	require.True(t, strings.Contains(err.Error(), "validatorDecrease"))
}

func TestRatingsData_OverflowErr(t *testing.T) {
	t.Parallel()

	ratingsDataArg := createDymmyRatingsData()
	ratingsConfig := createDummyRatingsConfig()
	ratingsDataArg.Config = ratingsConfig
	ratingsDataArg.SlotDurationMilliseconds = 3600 * 1000
	ratingsDataArg.MinNodes = math.MaxUint32
	ratingsData, err := NewRatingsData(ratingsDataArg)

	require.Nil(t, ratingsData)
	require.True(t, errors.Is(err, process.ErrOverflow))
	require.True(t, strings.Contains(err.Error(), "proposerIncrease"))

	ratingsDataArg = createDymmyRatingsData()
	ratingsConfig = createDummyRatingsConfig()
	ratingsDataArg.Config = ratingsConfig
	ratingsDataArg.SlotDurationMilliseconds = 3600 * 1000
	ratingsDataArg.MinNodes = math.MaxUint32
	ratingsDataArg.ConsensusSize = 1
	ratingsDataArg.Config.RatingSteps.ProposerValidatorImportance = float32(1) / math.MaxUint32
	ratingsData, err = NewRatingsData(ratingsDataArg)

	require.Nil(t, ratingsData)
	require.True(t, errors.Is(err, process.ErrOverflow))
	require.True(t, strings.Contains(err.Error(), "validatorIncrease"))
}

func TestRatingsData_IncreaseLowerThanZeroErr(t *testing.T) {
	t.Parallel()

	ratingsDataArg := createDymmyRatingsData()
	ratingsConfig := createDummyRatingsConfig()
	ratingsDataArg.Config = ratingsConfig
	ratingsDataArg.Config.RatingSteps.HoursToMaxRatingFromStartRating = math.MaxUint32
	ratingsData, err := NewRatingsData(ratingsDataArg)

	require.Nil(t, ratingsData)
	require.True(t, errors.Is(err, process.ErrIncreaseStepLowerThanOne))
	require.True(t, strings.Contains(err.Error(), "proposerIncrease"))

	ratingsDataArg = createDymmyRatingsData()
	ratingsConfig = createDummyRatingsConfig()
	ratingsDataArg.Config = ratingsConfig
	ratingsDataArg.Config.RatingSteps.HoursToMaxRatingFromStartRating = 2
	ratingsDataArg.Config.RatingSteps.ProposerValidatorImportance = math.MaxUint32
	ratingsData, err = NewRatingsData(ratingsDataArg)

	require.Nil(t, ratingsData)
	require.True(t, errors.Is(err, process.ErrIncreaseStepLowerThanOne))
	require.True(t, strings.Contains(err.Error(), "validatorIncrease"))
}

func TestRatingsData_RatingsCorrectValues(t *testing.T) {
	t.Parallel()

	ratingsDataArg := createDymmyRatingsData()
	minRating := uint32(1)
	maxRating := uint32(10000)
	startRating := uint32(4000)
	signedBlocksThreshold := float32(0.025)
	metaConsecutivePenalty := float32(1.3)
	hoursToMaxRatingFromStartRating := uint32(5)
	decreaseFactor := float32(-4)

	ratingsConfig := createDummyRatingsConfig()
	ratingsConfig.General.MinRating = minRating
	ratingsConfig.General.MaxRating = maxRating
	ratingsConfig.General.StartRating = startRating
	ratingsConfig.RatingSteps.HoursToMaxRatingFromStartRating = hoursToMaxRatingFromStartRating
	ratingsConfig.General.SignedBlocksThreshold = signedBlocksThreshold
	ratingsConfig.RatingSteps.ConsecutiveMissedBlocksPenalty = metaConsecutivePenalty
	ratingsConfig.RatingSteps.ProposerDecreaseFactor = decreaseFactor
	ratingsConfig.RatingSteps.ValidatorDecreaseFactor = decreaseFactor

	selectionChances := []*config.SelectionChance{
		{MaxThreshold: 0, ChancePercent: 1},
		{MaxThreshold: minRating, ChancePercent: 2},
		{MaxThreshold: maxRating, ChancePercent: 4},
	}

	ratingsConfig.General.SelectionChances = selectionChances

	ratingsDataArg.Config = ratingsConfig
	ratingsData, err := NewRatingsData(ratingsDataArg)

	require.Nil(t, err)
	assert.NotNil(t, ratingsData)
	assert.Equal(t, startRating, ratingsData.StartRating())
	assert.Equal(t, minRating, ratingsData.MinRating())
	assert.Equal(t, maxRating, ratingsData.MaxRating())
	assert.Equal(t, signedBlocksThreshold, ratingsData.SignedBlocksThreshold())
	assert.Equal(t, validatorIncreaseRatingStep, ratingsData.ChainRatingsStepHandler().ValidatorIncreaseRatingStep())
	assert.Equal(t, validatorDecreaseRatingStep, ratingsData.ChainRatingsStepHandler().ValidatorDecreaseRatingStep())
	assert.Equal(t, proposerIncreaseRatingStep, ratingsData.ChainRatingsStepHandler().ProposerIncreaseRatingStep())
	assert.Equal(t, proposerDecreaseRatingStep, ratingsData.ChainRatingsStepHandler().ProposerDecreaseRatingStep())
	assert.Equal(t, metaConsecutivePenalty, ratingsData.ChainRatingsStepHandler().ConsecutiveMissedBlocksPenalty())

	for i := range selectionChances {
		assert.Equal(t, selectionChances[i].MaxThreshold, ratingsData.SelectionChances()[i].GetMaxThreshold())
		assert.Equal(t, selectionChances[i].ChancePercent, ratingsData.SelectionChances()[i].GetChancePercent())
	}
}
