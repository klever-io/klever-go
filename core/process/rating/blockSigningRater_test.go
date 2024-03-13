package rating_test

import (
	"errors"
	"math"
	"testing"

	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/rating"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	validatorIncreaseRatingStep         = int32(1)
	validatorDecreaseRatingStep         = int32(-2)
	proposerIncreaseRatingStep          = int32(2)
	proposerDecreaseRatingStep          = int32(-4)
	minRating                           = uint32(1)
	maxRating                           = uint32(100)
	startRating                         = uint32(50)
	consecutiveMissedBlocksPenaltyMeta  = 1.2
	consecutiveMissedBlocksPenaltyShard = 1.1
)

func createDefaultChances() []process.SelectionChance {
	chances := []process.SelectionChance{
		&rating.SelectionChance{MaxThreshold: 0, ChancePercent: 50},
		&rating.SelectionChance{MaxThreshold: 10, ChancePercent: 0},
		&rating.SelectionChance{MaxThreshold: 25, ChancePercent: 90},
		&rating.SelectionChance{MaxThreshold: 75, ChancePercent: 100},
		&rating.SelectionChance{MaxThreshold: 100, ChancePercent: 110},
	}

	return chances
}

func createDefaultRatingsData() *mock.RatingsInfoMock {
	ratingsData := &mock.RatingsInfoMock{
		StartRatingProperty: startRating,
		MaxRatingProperty:   maxRating,
		MinRatingProperty:   minRating,
		RatingsStepDataProperty: &mock.RatingStepMock{
			ProposerIncreaseRatingStepProperty:     proposerIncreaseRatingStep,
			ProposerDecreaseRatingStepProperty:     proposerDecreaseRatingStep,
			ValidatorIncreaseRatingStepProperty:    validatorIncreaseRatingStep,
			ValidatorDecreaseRatingStepProperty:    validatorDecreaseRatingStep,
			ConsecutiveMissedBlocksPenaltyProperty: consecutiveMissedBlocksPenaltyMeta,
		},
		SelectionChancesProperty: createDefaultChances(),
	}

	return ratingsData
}

func TestBlockSigningRater_UpdateRatingsShouldUpdateRatingWhenProposed(t *testing.T) {
	initialRatingValue := uint32(5)
	rd := createDefaultRatingsData()

	bsr, _ := rating.NewBlockSigningRater(rd)
	computedRating := bsr.ComputeIncreaseProposer(initialRatingValue)

	expectedValue := uint32(int32(initialRatingValue) + proposerIncreaseRatingStep)

	assert.Equal(t, expectedValue, computedRating)
}

func TestBlockSigningRater_UpdateRatingsShouldUpdateRatingWhenValidator(t *testing.T) {
	initialRatingValue := uint32(5)
	rd := createDefaultRatingsData()

	bsr, _ := rating.NewBlockSigningRater(rd)
	computedRating := bsr.ComputeIncreaseValidator(initialRatingValue)

	expectedValue := uint32(int32(initialRatingValue) + validatorIncreaseRatingStep)

	assert.Equal(t, expectedValue, computedRating)
}

func TestBlockSigningRater_UpdateRatingsShouldUpdateRatingWhenValidatorButNotAccepted(t *testing.T) {
	initialRatingValue := uint32(5)
	rd := createDefaultRatingsData()

	bsr, _ := rating.NewBlockSigningRater(rd)
	computedRating := bsr.ComputeDecreaseValidator(initialRatingValue)

	expectedValue := uint32(int32(initialRatingValue) + validatorDecreaseRatingStep)

	assert.Equal(t, expectedValue, computedRating)
}

func TestBlockSigningRater_UpdateRatingsShouldUpdateRatingWhenProposerButNotAccepted(t *testing.T) {
	initialRatingValue := uint32(5)
	rd := createDefaultRatingsData()

	bsr, _ := rating.NewBlockSigningRater(rd)
	computedRating := bsr.ComputeDecreaseProposer(initialRatingValue, 0)

	expectedValue := uint32(int32(initialRatingValue) + proposerDecreaseRatingStep)

	assert.Equal(t, expectedValue, computedRating)
}

func TestBlockSigningRater_UpdateRatingsShouldNotIncreaseAboveMaxRating(t *testing.T) {
	initialRatingValue := maxRating - 1
	rd := createDefaultRatingsData()

	bsr, _ := rating.NewBlockSigningRater(rd)
	computedRating := bsr.ComputeIncreaseProposer(initialRatingValue)

	expectedValue := maxRating

	assert.Equal(t, expectedValue, computedRating)
}

func TestBlockSigningRater_UpdateRatingsShouldNotDecreaseBelowMinRating(t *testing.T) {
	initialRatingValue := minRating + 1
	rd := createDefaultRatingsData()

	bsr, _ := rating.NewBlockSigningRater(rd)
	computedRating := bsr.ComputeDecreaseProposer(initialRatingValue, 0)

	expectedValue := minRating

	assert.Equal(t, expectedValue, computedRating)
}

func TestBlockSigningRater_UpdateRatingsWithMultiplePeersShouldReturnRatings(t *testing.T) {
	rd := createDefaultRatingsData()
	bsr, _ := rating.NewBlockSigningRater(rd)

	pk1 := "pk1"
	pk2 := "pk2"
	pk3 := "pk3"
	pk4 := "pk4"

	pk1Rating := uint32(4)
	pk2Rating := uint32(5)
	pk3Rating := uint32(6)
	pk4Rating := uint32(7)

	ratingsMap := make(map[string]uint32)
	ratingsMap[pk1] = pk1Rating
	ratingsMap[pk2] = pk2Rating
	ratingsMap[pk3] = pk3Rating
	ratingsMap[pk4] = pk4Rating

	pk1ComputedRating := bsr.ComputeIncreaseProposer(ratingsMap[pk1])
	pk2ComputedRating := bsr.ComputeDecreaseProposer(ratingsMap[pk2], 0)
	pk3ComputedRating := bsr.ComputeIncreaseValidator(ratingsMap[pk3])
	pk4ComputedRating := bsr.ComputeDecreaseValidator(ratingsMap[pk4])

	expectedPk1 := uint32(int32(ratingsMap[pk1]) + proposerIncreaseRatingStep)
	expectedPk2 := uint32(int32(ratingsMap[pk2]) + proposerDecreaseRatingStep)
	expectedPk3 := uint32(int32(ratingsMap[pk3]) + validatorIncreaseRatingStep)
	expectedPk4 := uint32(int32(ratingsMap[pk4]) + validatorDecreaseRatingStep)

	assert.Equal(t, expectedPk1, pk1ComputedRating)
	assert.Equal(t, expectedPk2, pk2ComputedRating)
	assert.Equal(t, expectedPk3, pk3ComputedRating)
	assert.Equal(t, expectedPk4, pk4ComputedRating)
}

func TestBlockSigningRater_UpdateRatingsOnMetaWithMultiplePeersShouldReturnRatings(t *testing.T) {
	rd := createDefaultRatingsData()
	bsr, _ := rating.NewBlockSigningRater(rd)

	pk1 := "pk1"
	pk2 := "pk2"
	pk3 := "pk3"
	pk4 := "pk4"

	pk1Rating := uint32(14)
	pk2Rating := uint32(15)
	pk3Rating := uint32(16)
	pk4Rating := uint32(17)

	ratingsMap := make(map[string]uint32)
	ratingsMap[pk1] = pk1Rating
	ratingsMap[pk2] = pk2Rating
	ratingsMap[pk3] = pk3Rating
	ratingsMap[pk4] = pk4Rating

	pk1ComputedRating := bsr.ComputeIncreaseProposer(ratingsMap[pk1])
	pk2ComputedRating := bsr.ComputeDecreaseProposer(ratingsMap[pk2], 0)
	pk3ComputedRating := bsr.ComputeIncreaseValidator(ratingsMap[pk3])
	pk4ComputedRating := bsr.ComputeDecreaseValidator(ratingsMap[pk4])

	expectedPk1 := uint32(int32(ratingsMap[pk1]) + proposerIncreaseRatingStep)
	expectedPk2 := uint32(int32(ratingsMap[pk2]) + proposerDecreaseRatingStep)
	expectedPk3 := uint32(int32(ratingsMap[pk3]) + validatorIncreaseRatingStep)
	expectedPk4 := uint32(int32(ratingsMap[pk4]) + validatorDecreaseRatingStep)

	assert.Equal(t, expectedPk1, pk1ComputedRating)
	assert.Equal(t, expectedPk2, pk2ComputedRating)
	assert.Equal(t, expectedPk3, pk3ComputedRating)
	assert.Equal(t, expectedPk4, pk4ComputedRating)
}

func TestBlockSigningRater_NewBlockSigningRaterWithChancesNilShouldErr(t *testing.T) {
	ratingsData := createDefaultRatingsData()
	ratingsData.SelectionChancesProperty = nil

	bsr, err := rating.NewBlockSigningRater(ratingsData)

	assert.Nil(t, bsr)
	assert.Equal(t, process.ErrNoChancesProvided, err)
}

func TestBlockSigningRater_NewBlockSigningRaterWithDupplicateMaxThresholdShouldErr(t *testing.T) {
	chances := []process.SelectionChance{
		&rating.SelectionChance{MaxThreshold: 0, ChancePercent: 50},
		&rating.SelectionChance{MaxThreshold: 10, ChancePercent: 0},
		&rating.SelectionChance{MaxThreshold: 20, ChancePercent: 90},
		&rating.SelectionChance{MaxThreshold: 20, ChancePercent: 100},
		&rating.SelectionChance{MaxThreshold: 100, ChancePercent: 110},
	}
	ratingsData := createDefaultRatingsData()
	ratingsData.SelectionChancesProperty = chances

	bsr, err := rating.NewBlockSigningRater(ratingsData)

	assert.Nil(t, bsr)
	assert.Equal(t, process.ErrDuplicateThreshold, err)
}

func TestBlockSigningRater_NewBlockSigningRaterWithZeroMinRatingShouldErr(t *testing.T) {
	ratingsData := createDefaultRatingsData()
	ratingsData.MinRatingProperty = 0

	_, err := rating.NewBlockSigningRater(ratingsData)
	assert.Equal(t, process.ErrMinRatingSmallerThanOne, err)
}

func TestBlockSigningRater_NewBlockSigningRaterWithMinGreaterThanMaxShouldErr(t *testing.T) {
	ratingsData := createDefaultRatingsData()
	ratingsData.MinRatingProperty = 100
	ratingsData.MaxRatingProperty = 90

	bsr, err := rating.NewBlockSigningRater(ratingsData)

	assert.Nil(t, bsr)
	assert.True(t, errors.Is(err, process.ErrMaxRatingIsSmallerThanMinRating))
}

func TestBlockSigningRater_NewBlockSigningRaterWithSignedBlocksThresholdNegativeShouldErr(t *testing.T) {
	ratingsData := createDefaultRatingsData()
	ratingsData.SignedBlocksThresholdProperty = -0.01

	bsr, err := rating.NewBlockSigningRater(ratingsData)

	assert.Nil(t, bsr)
	assert.True(t, errors.Is(err, process.ErrSignedBlocksThresholdNotBetweenZeroAndOne))
}

func TestBlockSigningRater_NewBlockSigningRaterWithSignedBlocksThresholdAbove1ShouldErr(t *testing.T) {
	ratingsData := createDefaultRatingsData()
	ratingsData.SignedBlocksThresholdProperty = 1.01

	bsr, err := rating.NewBlockSigningRater(ratingsData)

	assert.Nil(t, bsr)
	assert.True(t, errors.Is(err, process.ErrSignedBlocksThresholdNotBetweenZeroAndOne))
}

func TestBlockSigningRater_NewBlockSigningRaterWithNonExistingMaxThresholdZeroShouldErr(t *testing.T) {
	chances := []process.SelectionChance{
		&rating.SelectionChance{MaxThreshold: 10, ChancePercent: 0},
		&rating.SelectionChance{MaxThreshold: 20, ChancePercent: 90},
		&rating.SelectionChance{MaxThreshold: 20, ChancePercent: 100},
		&rating.SelectionChance{MaxThreshold: 100, ChancePercent: 110},
	}

	ratingsData := createDefaultRatingsData()
	ratingsData.SelectionChancesProperty = chances

	bsr, err := rating.NewBlockSigningRater(ratingsData)

	assert.Nil(t, bsr)
	assert.Equal(t, process.ErrNilMinChanceIfZero, err)
}

func TestBlockSigningRater_NewBlockSigningRaterWithNoValueForMaxThresholdShouldErr(t *testing.T) {
	chances := []process.SelectionChance{
		&rating.SelectionChance{MaxThreshold: 0, ChancePercent: 5},
		&rating.SelectionChance{MaxThreshold: 10, ChancePercent: 0},
		&rating.SelectionChance{MaxThreshold: 20, ChancePercent: 90},
	}
	ratingsData := createDefaultRatingsData()
	ratingsData.SelectionChancesProperty = chances

	bsr, err := rating.NewBlockSigningRater(ratingsData)

	assert.Nil(t, bsr)
	assert.Equal(t, process.ErrNoChancesForMaxThreshold, err)
}

func TestBlockSigningRater_NewBlockSigningRaterWitStartRatingSmallerThanMinShouldErr(t *testing.T) {

	ratingsData := createDefaultRatingsData()
	ratingsData.MinRatingProperty = 50
	ratingsData.StartRatingProperty = 40
	ratingsData.MaxRatingProperty = 100

	bsr, err := rating.NewBlockSigningRater(ratingsData)

	assert.Nil(t, bsr)
	assert.True(t, errors.Is(err, process.ErrStartRatingNotBetweenMinAndMax))
}

func TestBlockSigningRater_NewBlockSigningRaterWitStartRatingGreaterThanMaxdShouldErr(t *testing.T) {
	ratingsData := createDefaultRatingsData()
	ratingsData.MinRatingProperty = 50
	ratingsData.StartRatingProperty = 110
	ratingsData.MaxRatingProperty = 100

	bsr, err := rating.NewBlockSigningRater(ratingsData)

	assert.Nil(t, bsr)
	assert.True(t, errors.Is(err, process.ErrStartRatingNotBetweenMinAndMax))
}

func TestBlockSigningRater_RevertIncreaseValidator(t *testing.T) {
	ratingsData := createDefaultRatingsData()
	startRating := ratingsData.StartRating()
	validatorIncrease := ratingsData.ChainRatingsStepHandler().ValidatorIncreaseRatingStep()

	bsr, _ := rating.NewBlockSigningRater(ratingsData)

	zeroReverts := bsr.RevertIncreaseValidator(ratingsData.StartRating(), 0)
	require.Equal(t, startRating, zeroReverts)

	oneRevert := bsr.RevertIncreaseValidator(ratingsData.StartRating(), 1)
	require.Equal(t, uint32(int32(startRating)-validatorIncrease), oneRevert)

	tenReverts := bsr.RevertIncreaseValidator(ratingsData.StartRating(), 10)
	require.Equal(t, uint32(int32(startRating)-10*validatorIncrease), tenReverts)

	hundredReverts := bsr.RevertIncreaseValidator(ratingsData.StartRating(), 100)
	require.Equal(t, ratingsData.MinRating(), hundredReverts)

	ratingBelowMinRating := bsr.RevertIncreaseValidator(ratingsData.MinRating()-1, 0)
	require.Equal(t, ratingsData.MinRating(), ratingBelowMinRating)

	ratingBelowMinRating = bsr.RevertIncreaseValidator(ratingsData.MinRating()-1, 1)
	require.Equal(t, ratingsData.MinRating(), ratingBelowMinRating)
}

func TestBlockSigningRater_RevertIncreaseValidatorWithOverFlow(t *testing.T) {
	ratingsData := createDefaultRatingsData()
	ratingsData.StartRatingProperty = math.MaxUint32 / 2
	ratingsData.MaxRatingProperty = math.MaxUint32
	ratingsData.RatingsStepDataProperty = &mock.RatingStepMock{
		ProposerIncreaseRatingStepProperty:     proposerIncreaseRatingStep,
		ProposerDecreaseRatingStepProperty:     proposerDecreaseRatingStep,
		ValidatorIncreaseRatingStepProperty:    math.MaxInt32,
		ValidatorDecreaseRatingStepProperty:    validatorDecreaseRatingStep,
		ConsecutiveMissedBlocksPenaltyProperty: consecutiveMissedBlocksPenaltyMeta,
	}

	ratingsData.SelectionChancesProperty[len(ratingsData.SelectionChancesProperty)-1] = &rating.SelectionChance{
		MaxThreshold:  ratingsData.MaxRating(),
		ChancePercent: 10,
	}

	bsr, _ := rating.NewBlockSigningRater(ratingsData)
	zeroReverts := bsr.RevertIncreaseValidator(ratingsData.StartRating(), 0)
	assert.Equal(t, ratingsData.StartRating(), zeroReverts)

	oneRevert := bsr.RevertIncreaseValidator(ratingsData.StartRating(), 1)
	assert.Equal(t, ratingsData.MinRating(), oneRevert)

	overFlowRevert := bsr.RevertIncreaseValidator(ratingsData.StartRating(), 2)
	assert.Equal(t, ratingsData.MinRating(), overFlowRevert)

	overFlowRevert = bsr.RevertIncreaseValidator(ratingsData.StartRating(), math.MaxUint32)
	assert.Equal(t, ratingsData.MinRating(), overFlowRevert)
}

func TestBlockSigningRater_NewBlockSigningRaterWithCorrectValueShouldWork(t *testing.T) {
	ratingsData := createDefaultRatingsData()
	metaRatingsStepHandler := ratingsData.RatingsStepDataProperty

	bsr, err := rating.NewBlockSigningRater(ratingsData)

	assert.NotNil(t, bsr)
	assert.Nil(t, err)

	testValue := int32(50)
	assert.Equal(t, ratingsData.StartRating(), bsr.GetStartRating())
	assert.Equal(t, ratingsData.SignedBlocksThreshold(), bsr.GetSignedBlocksThreshold())

	assert.Equal(t, ratingsData.StartRating(), bsr.GetStartRating())
	assert.Equal(t, uint32(testValue+metaRatingsStepHandler.ValidatorIncreaseRatingStep()), bsr.ComputeIncreaseValidator(uint32(testValue)))
	assert.Equal(t, uint32(testValue+metaRatingsStepHandler.ValidatorDecreaseRatingStep()), bsr.ComputeDecreaseValidator(uint32(testValue)))
	assert.Equal(t, uint32(testValue+metaRatingsStepHandler.ProposerIncreaseRatingStep()), bsr.ComputeIncreaseProposer(uint32(testValue)))
	assert.Equal(t, uint32(testValue+metaRatingsStepHandler.ProposerDecreaseRatingStep()), bsr.ComputeDecreaseProposer(uint32(testValue), 0))
	assert.Equal(t, uint32(testValue-metaRatingsStepHandler.ValidatorIncreaseRatingStep()), bsr.RevertIncreaseValidator(uint32(testValue), 1))

	assert.Equal(t, ratingsData.SelectionChances()[0].GetChancePercent(), bsr.GetChance(uint32(0)))
	assert.Equal(t, ratingsData.SelectionChances()[1].GetChancePercent(), bsr.GetChance(uint32(9)))
	assert.Equal(t, ratingsData.SelectionChances()[2].GetChancePercent(), bsr.GetChance(uint32(20)))
	assert.Equal(t, ratingsData.SelectionChances()[3].GetChancePercent(), bsr.GetChance(uint32(50)))
	assert.Equal(t, ratingsData.SelectionChances()[4].GetChancePercent(), bsr.GetChance(uint32(100)))

	assert.False(t, bsr.IsInterfaceNil())
}

func TestBlockSigningRater_GetChancesForStartRatingdReturnStartRatingChance(t *testing.T) {
	ratingsData := createDefaultRatingsData()

	bsr, _ := rating.NewBlockSigningRater(ratingsData)

	chance := bsr.GetChance(startRating)

	chanceForStartRating := uint32(100)
	assert.Equal(t, chanceForStartRating, chance)
}

func TestBlockSigningRater_GetChancesForSetRatingShouldReturnCorrectRating(t *testing.T) {
	rd := createDefaultRatingsData()
	rd.SelectionChancesProperty = []process.SelectionChance{
		&rating.SelectionChance{MaxThreshold: 0, ChancePercent: 50},
		&rating.SelectionChance{MaxThreshold: 10, ChancePercent: 0},
		&rating.SelectionChance{MaxThreshold: 25, ChancePercent: 90},
		&rating.SelectionChance{MaxThreshold: 75, ChancePercent: 100},
		&rating.SelectionChance{MaxThreshold: 100, ChancePercent: 110},
	}
	bsr, _ := rating.NewBlockSigningRater(rd)

	ratingValue := uint32(80)
	chances := bsr.GetChance(ratingValue)

	chancesFor80 := uint32(110)
	assert.Equal(t, chancesFor80, chances)
}

func TestBlockSigningRater_GetChancesForSetRatingShouldReturnCorrectRatingForThresholdValue(t *testing.T) {
	rd := createDefaultRatingsData()

	rd.SelectionChancesProperty = []process.SelectionChance{
		&rating.SelectionChance{MaxThreshold: 0, ChancePercent: 50},
		&rating.SelectionChance{MaxThreshold: 10, ChancePercent: 0},
		&rating.SelectionChance{MaxThreshold: 25, ChancePercent: 90},
		&rating.SelectionChance{MaxThreshold: 75, ChancePercent: 100},
		&rating.SelectionChance{MaxThreshold: 100, ChancePercent: 110},
	}

	bsr, _ := rating.NewBlockSigningRater(rd)

	ratingValue := uint32(75)
	chances := bsr.GetChance(ratingValue)

	chancesFor75 := uint32(100)
	assert.Equal(t, chancesFor75, chances)
}

func TestBlockSigningRater_PositiveDecreaseRatingStep(t *testing.T) {
	rd := createDefaultRatingsData()
	ratingStep := createRatingStepMock()
	ratingStep.ProposerDecreaseRatingStepProperty = 0
	rd.RatingsStepDataProperty = ratingStep
	bsr, err := rating.NewBlockSigningRater(rd)
	require.Nil(t, bsr)
	require.True(t, errors.Is(err, process.ErrDecreaseRatingsStepMoreThanMinusOne))

	rd = createDefaultRatingsData()
	ratingStep = createRatingStepMock()
	ratingStep.ValidatorDecreaseRatingStepProperty = 0
	rd.RatingsStepDataProperty = ratingStep
	bsr, err = rating.NewBlockSigningRater(rd)
	require.Nil(t, bsr)
	require.True(t, errors.Is(err, process.ErrDecreaseRatingsStepMoreThanMinusOne))

	rd = createDefaultRatingsData()
	ratingStep = createRatingStepMock()
	ratingStep.ProposerDecreaseRatingStepProperty = 0
	rd.RatingsStepDataProperty = ratingStep
	bsr, err = rating.NewBlockSigningRater(rd)
	require.Nil(t, bsr)
	require.True(t, errors.Is(err, process.ErrDecreaseRatingsStepMoreThanMinusOne))

	rd = createDefaultRatingsData()
	ratingStep = createRatingStepMock()
	ratingStep.ValidatorDecreaseRatingStepProperty = 0
	rd.RatingsStepDataProperty = ratingStep
	bsr, err = rating.NewBlockSigningRater(rd)
	require.Nil(t, bsr)
	require.True(t, errors.Is(err, process.ErrDecreaseRatingsStepMoreThanMinusOne))
}

func TestBlockSigningRater_ConsecutiveBlocksPenaltyLessThanOne(t *testing.T) {
	rd := createDefaultRatingsData()
	ratingStep := createRatingStepMock()
	ratingStep.ConsecutiveMissedBlocksPenaltyProperty = 0.5
	rd.RatingsStepDataProperty = ratingStep
	bsr, err := rating.NewBlockSigningRater(rd)
	require.Nil(t, bsr)
	require.True(t, errors.Is(err, process.ErrConsecutiveMissedBlocksPenaltyLowerThanOne))

	rd = createDefaultRatingsData()
	ratingStep = createRatingStepMock()
	ratingStep.ConsecutiveMissedBlocksPenaltyProperty = 0.5
	rd.RatingsStepDataProperty = ratingStep
	bsr, err = rating.NewBlockSigningRater(rd)
	require.Nil(t, bsr)
	require.True(t, errors.Is(err, process.ErrConsecutiveMissedBlocksPenaltyLowerThanOne))
}

func TestBlockSigningRater_ComputeDecreaseProposer(t *testing.T) {
	ratingsData := &mock.RatingsInfoMock{
		StartRatingProperty: startRating * 100,
		MaxRatingProperty:   maxRating * 100,
		MinRatingProperty:   minRating * 100,
		RatingsStepDataProperty: &mock.RatingStepMock{
			ProposerIncreaseRatingStepProperty:     proposerIncreaseRatingStep,
			ProposerDecreaseRatingStepProperty:     proposerDecreaseRatingStep * 100,
			ValidatorIncreaseRatingStepProperty:    validatorIncreaseRatingStep,
			ValidatorDecreaseRatingStepProperty:    validatorDecreaseRatingStep,
			ConsecutiveMissedBlocksPenaltyProperty: consecutiveMissedBlocksPenaltyMeta,
		},
		SelectionChancesProperty: createDefaultChances(),
	}

	ratingsData.SelectionChancesProperty[len(ratingsData.SelectionChancesProperty)-1] = &rating.SelectionChance{
		MaxThreshold:  ratingsData.MaxRating(),
		ChancePercent: 10,
	}

	startRating := ratingsData.StartRating()
	proposerDecrease := make(map[uint32]int32)
	proposerDecrease[0] = ratingsData.ChainRatingsStepHandler().ProposerDecreaseRatingStep()
	proposerDecrease[0] = ratingsData.ChainRatingsStepHandler().ProposerDecreaseRatingStep()

	penalty := make(map[uint32]float32)
	penalty[0] = ratingsData.ChainRatingsStepHandler().ConsecutiveMissedBlocksPenalty()
	penalty[0] = ratingsData.ChainRatingsStepHandler().ConsecutiveMissedBlocksPenalty()

	bsr, _ := rating.NewBlockSigningRater(ratingsData)

	var consecutiveMisses uint32
	for shardId := range proposerDecrease {
		consecutiveMisses = 0
		zeroConsecutive := bsr.ComputeDecreaseProposer(ratingsData.StartRating(), consecutiveMisses)
		decreaseStep := float64(proposerDecrease[shardId]) * math.Pow(float64(penalty[shardId]), float64(consecutiveMisses))
		require.Equal(t, uint32(int32(startRating)+int32(decreaseStep)), zeroConsecutive)

		consecutiveMisses = 1
		oneConsecutive := bsr.ComputeDecreaseProposer(ratingsData.StartRating(), consecutiveMisses)
		decreaseStep = float64(proposerDecrease[shardId]) * math.Pow(float64(penalty[shardId]), float64(consecutiveMisses))
		require.Equal(t, uint32(int32(startRating)+int32(decreaseStep)), oneConsecutive)

		consecutiveMisses = 5
		fiveConsecutive := bsr.ComputeDecreaseProposer(ratingsData.StartRating(), consecutiveMisses)
		decreaseStep = float64(proposerDecrease[shardId]) * math.Pow(float64(penalty[shardId]), float64(consecutiveMisses))
		require.Equal(t, uint32(int32(startRating)+int32(decreaseStep)), fiveConsecutive)

		consecutiveMisses = 0
		ratingBelowMinRating := bsr.ComputeDecreaseProposer(ratingsData.MinRating()-1, consecutiveMisses)
		require.Equal(t, ratingsData.MinRating(), ratingBelowMinRating)

		consecutiveMisses = 1
		ratingBelowMinRating = bsr.ComputeDecreaseProposer(ratingsData.MinRating()-1, consecutiveMisses)
		require.Equal(t, ratingsData.MinRating(), ratingBelowMinRating)
	}
}

func TestBlockSigningRater_ComputeDecreaseProposerWithOverFlow(t *testing.T) {
	ratingsData := createDefaultRatingsData()
	ratingsData.StartRatingProperty = math.MaxUint32 / 2
	ratingsData.MaxRatingProperty = math.MaxUint32
	ratingsData.RatingsStepDataProperty = &mock.RatingStepMock{
		ProposerIncreaseRatingStepProperty:     proposerIncreaseRatingStep,
		ProposerDecreaseRatingStepProperty:     -math.MaxUint32 / 10,
		ValidatorIncreaseRatingStepProperty:    validatorIncreaseRatingStep,
		ValidatorDecreaseRatingStepProperty:    validatorDecreaseRatingStep,
		ConsecutiveMissedBlocksPenaltyProperty: 2,
	}
	ratingsData.SelectionChancesProperty[len(ratingsData.SelectionChancesProperty)-1] = &rating.SelectionChance{
		MaxThreshold:  ratingsData.MaxRating(),
		ChancePercent: 10,
	}

	proposerDecreaseRatingStep := ratingsData.ChainRatingsStepHandler().ProposerDecreaseRatingStep()
	consecutiveMissedBlocksPenalty := ratingsData.ChainRatingsStepHandler().ConsecutiveMissedBlocksPenalty()
	startRating := ratingsData.StartRating()

	bsr, _ := rating.NewBlockSigningRater(ratingsData)
	var consecutiveMisses uint32

	consecutiveMisses = 0
	zeroMisses := bsr.ComputeDecreaseProposer(ratingsData.StartRating(), consecutiveMisses)
	decreaseStep := float64(proposerDecreaseRatingStep) * math.Pow(float64(consecutiveMissedBlocksPenalty), float64(consecutiveMisses))
	assert.Equal(t, uint32(int32(startRating)+int32(decreaseStep)), zeroMisses)

	consecutiveMisses = 1
	oneMisses := bsr.ComputeDecreaseProposer(ratingsData.StartRating(), consecutiveMisses)
	decreaseStep = float64(proposerDecreaseRatingStep) * math.Pow(float64(consecutiveMissedBlocksPenalty), float64(consecutiveMisses))
	assert.Equal(t, uint32(int32(startRating)+int32(decreaseStep)), oneMisses)

	consecutiveMisses = 2
	twoMisses := bsr.ComputeDecreaseProposer(ratingsData.StartRating(), consecutiveMisses)
	decreaseStep = float64(proposerDecreaseRatingStep) * math.Pow(float64(consecutiveMissedBlocksPenalty), float64(consecutiveMisses))
	assert.Equal(t, uint32(int32(startRating)+int32(decreaseStep)), twoMisses)

	consecutiveMisses = 10
	tenMisses := bsr.ComputeDecreaseProposer(ratingsData.StartRating(), consecutiveMisses)
	assert.Equal(t, ratingsData.MinRating(), tenMisses)

	consecutiveMisses = 100
	maxMisses := bsr.ComputeDecreaseProposer(ratingsData.StartRating(), consecutiveMisses)
	assert.Equal(t, ratingsData.MinRating(), maxMisses)
}

func createRatingStepMock() *mock.RatingStepMock {
	return &mock.RatingStepMock{
		ProposerIncreaseRatingStepProperty:     proposerIncreaseRatingStep,
		ProposerDecreaseRatingStepProperty:     proposerDecreaseRatingStep,
		ValidatorIncreaseRatingStepProperty:    validatorIncreaseRatingStep,
		ValidatorDecreaseRatingStepProperty:    validatorDecreaseRatingStep,
		ConsecutiveMissedBlocksPenaltyProperty: consecutiveMissedBlocksPenaltyMeta,
	}
}
