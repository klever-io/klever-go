package rating

import (
	"fmt"
	"math"

	"github.com/klever-io/klever-go/core/process"

	"github.com/klever-io/klever-go/config"
)

var _ process.RatingsInfoHandler = (*RatingsData)(nil)

const millisecondsInHour = 3600 * 1000

type computeRatingStepArg struct {
	minNodes                        uint32
	consensusSize                   uint32
	slotTimeMillis                  uint64
	startRating                     uint32
	maxRating                       uint32
	hoursToMaxRatingFromStartRating uint32
	proposerDecreaseFactor          float32
	validatorDecreaseFactor         float32
	consecutiveMissedBlocksPenalty  float32
	proposerValidatorImportance     float32
}

// RatingsData will store information about ratingsComputation
type RatingsData struct {
	startRating           uint32
	maxRating             uint32
	minRating             uint32
	signedBlocksThreshold float32
	ratingsStepData       process.RatingsStepHandler
	selectionChances      []process.SelectionChance
}

// RatingsDataArg contains information for the creation of the new ratingsData
type RatingsDataArg struct {
	Config                   config.RatingsConfig
	ConsensusSize            uint32
	MinNodes                 uint32
	SlotDurationMilliseconds uint64
}

// NewRatingsData creates a new RatingsData instance
func NewRatingsData(args RatingsDataArg) (*RatingsData, error) {
	ratingsConfig := args.Config
	err := verifyRatingsConfig(ratingsConfig)
	if err != nil {
		return nil, err
	}

	chances := make([]process.SelectionChance, 0)
	for _, chance := range ratingsConfig.General.SelectionChances {
		chances = append(chances, &SelectionChance{
			MaxThreshold:  chance.MaxThreshold,
			ChancePercent: chance.ChancePercent,
		})
	}

	arg := computeRatingStepArg{
		minNodes:                        args.MinNodes,
		consensusSize:                   args.ConsensusSize,
		slotTimeMillis:                  args.SlotDurationMilliseconds,
		startRating:                     ratingsConfig.General.StartRating,
		maxRating:                       ratingsConfig.General.MaxRating,
		hoursToMaxRatingFromStartRating: ratingsConfig.RatingSteps.HoursToMaxRatingFromStartRating,
		proposerDecreaseFactor:          ratingsConfig.RatingSteps.ProposerDecreaseFactor,
		validatorDecreaseFactor:         ratingsConfig.RatingSteps.ValidatorDecreaseFactor,
		consecutiveMissedBlocksPenalty:  ratingsConfig.RatingSteps.ConsecutiveMissedBlocksPenalty,
		proposerValidatorImportance:     ratingsConfig.RatingSteps.ProposerValidatorImportance,
	}
	ratingStep, err := computeRatingStep(arg)
	if err != nil {
		return nil, err
	}

	return &RatingsData{
		startRating:           ratingsConfig.General.StartRating,
		maxRating:             ratingsConfig.General.MaxRating,
		minRating:             ratingsConfig.General.MinRating,
		signedBlocksThreshold: ratingsConfig.General.SignedBlocksThreshold,
		ratingsStepData:       ratingStep,
		selectionChances:      chances,
	}, nil
}

func verifyRatingsConfig(settings config.RatingsConfig) error {
	if settings.General.MinRating < 1 {
		return process.ErrMinRatingSmallerThanOne
	}
	if settings.General.MinRating > settings.General.MaxRating {
		return fmt.Errorf("%w: minRating: %v, maxRating: %v",
			process.ErrMaxRatingIsSmallerThanMinRating,
			settings.General.MinRating,
			settings.General.MaxRating)
	}
	if settings.General.MaxRating < settings.General.StartRating || settings.General.MinRating > settings.General.StartRating {
		return fmt.Errorf("%w: minRating: %v, startRating: %v, maxRating: %v",
			process.ErrStartRatingNotBetweenMinAndMax,
			settings.General.MinRating,
			settings.General.StartRating,
			settings.General.MaxRating)
	}
	if settings.General.SignedBlocksThreshold > 1 || settings.General.SignedBlocksThreshold < 0 {
		return fmt.Errorf("%w signedBlocksThreshold: %v",
			process.ErrSignedBlocksThresholdNotBetweenZeroAndOne,
			settings.General.SignedBlocksThreshold)
	}
	if settings.RatingSteps.HoursToMaxRatingFromStartRating == 0 {
		return fmt.Errorf("%w hoursToMaxRatingFromStartRating: chain",
			process.ErrHoursToMaxRatingFromStartRatingZero)
	}
	if settings.RatingSteps.ConsecutiveMissedBlocksPenalty < 1 {
		return fmt.Errorf("%w: chain consecutiveMissedBlocksPenalty: %v",
			process.ErrConsecutiveMissedBlocksPenaltyLowerThanOne,
			settings.RatingSteps.ConsecutiveMissedBlocksPenalty)
	}
	if settings.RatingSteps.ProposerDecreaseFactor > -1 || settings.RatingSteps.ValidatorDecreaseFactor > -1 {
		return fmt.Errorf("%w: chain decrease steps - proposer: %v, validator: %v",
			process.ErrDecreaseRatingsStepMoreThanMinusOne,
			settings.RatingSteps.ProposerDecreaseFactor,
			settings.RatingSteps.ValidatorDecreaseFactor)
	}
	return nil
}

func computeRatingStep(
	arg computeRatingStepArg,
) (process.RatingsStepHandler, error) {
	blocksProducedInHours := uint64(arg.hoursToMaxRatingFromStartRating*millisecondsInHour) / arg.slotTimeMillis
	ratingDifference := arg.maxRating - arg.startRating

	proposerProbability := float32(blocksProducedInHours) / float32(arg.minNodes)
	validatorProbability := proposerProbability * float32(arg.consensusSize)

	totalImportance := arg.proposerValidatorImportance + 1

	ratingFromProposer := float32(ratingDifference) * (arg.proposerValidatorImportance / totalImportance)
	ratingFromValidator := float32(ratingDifference) * (1 / totalImportance)

	proposerIncrease := ratingFromProposer / proposerProbability
	validatorIncrease := ratingFromValidator / validatorProbability
	proposerDecrease := proposerIncrease * arg.proposerDecreaseFactor
	validatorDecrease := validatorIncrease * arg.validatorDecreaseFactor

	if proposerIncrease > math.MaxInt32 {
		return nil, fmt.Errorf("%w proposerIncrease overflowed %v", process.ErrOverflow, proposerIncrease)
	}
	if validatorIncrease > math.MaxInt32 {
		return nil, fmt.Errorf("%w validatorIncrease overflowed %v", process.ErrOverflow, validatorIncrease)
	}
	if proposerDecrease < math.MinInt32 {
		return nil, fmt.Errorf("%w proposerDecrease overflowed %v", process.ErrOverflow, proposerDecrease)
	}
	if validatorDecrease < math.MinInt32 {
		return nil, fmt.Errorf("%w validatorDecrease overflowed %v", process.ErrOverflow, validatorDecrease)
	}
	if int32(proposerIncrease) < 1 {
		return nil, fmt.Errorf("%w proposerIncrease zero: %v", process.ErrIncreaseStepLowerThanOne, proposerIncrease)
	}
	if int32(validatorIncrease) < 1 {
		return nil, fmt.Errorf("%w validatorIncrease zero: %v", process.ErrIncreaseStepLowerThanOne, validatorIncrease)
	}

	return &RatingStep{
		proposerIncreaseRatingStep:     int32(proposerIncrease),
		proposerDecreaseRatingStep:     int32(proposerDecrease),
		validatorIncreaseRatingStep:    int32(validatorIncrease),
		validatorDecreaseRatingStep:    int32(validatorDecrease),
		consecutiveMissedBlocksPenalty: arg.consecutiveMissedBlocksPenalty}, nil
}

// StartRating will return the start rating
func (rd *RatingsData) StartRating() uint32 {
	return rd.startRating
}

// MaxRating will return the max rating
func (rd *RatingsData) MaxRating() uint32 {
	return rd.maxRating
}

// MinRating will return the min rating
func (rd *RatingsData) MinRating() uint32 {
	return rd.minRating
}

// SignedBlocksThreshold will return the signed blocks threshold
func (rd *RatingsData) SignedBlocksThreshold() float32 {
	return rd.signedBlocksThreshold
}

// SelectionChances will return the array of selectionChances and thresholds
func (rd *RatingsData) SelectionChances() []process.SelectionChance {
	return rd.selectionChances
}

// ChainRatingsStepHandler returns the RatingsStepHandler used for the chain
func (rd *RatingsData) ChainRatingsStepHandler() process.RatingsStepHandler {
	return rd.ratingsStepData
}

// IsInterfaceNil returns true if underlying object is nil
func (rd *RatingsData) IsInterfaceNil() bool {
	return rd == nil
}
