package config

// RatingsConfig will hold the configuration data needed for the ratings
type RatingsConfig struct {
	General     General           `yaml:"general"`
	PeerHonesty PeerHonestyConfig `yaml:"peerHonesty"`
	RatingSteps RatingSteps       `yaml:"ratingSteps"`
}

// General will hold ratings settings for the chain
type General struct {
	StartRating           uint32             `yaml:"startRating"`
	MaxRating             uint32             `yaml:"maxRating"`
	MinRating             uint32             `yaml:"minRating"`
	SignedBlocksThreshold float32            `yaml:"signedBlocksThreshold"`
	SelectionChances      []*SelectionChance `yaml:"selectionChances"`
}

// SelectionChance will hold the percentage modifier for up to the specified threshold
type SelectionChance struct {
	MaxThreshold  uint32 `yaml:"maxThreshold"`
	ChancePercent uint32 `yaml:"chancePercent"`
}

// RatingSteps holds the necessary increases and decreases of the rating steps
type RatingSteps struct {
	HoursToMaxRatingFromStartRating uint32  `yaml:"hoursToMaxRatingFromStartRating"`
	ProposerValidatorImportance     float32 `yaml:"proposerValidatorImportance"`
	ProposerDecreaseFactor          float32 `yaml:"proposerDecreaseFactor"`
	ValidatorDecreaseFactor         float32 `yaml:"validatorDecreaseFactor"`
	ConsecutiveMissedBlocksPenalty  float32 `yaml:"consecutiveMissedBlocksPenalty"`
}

// PeerHonestyConfig holds the parameters for the peer honesty handler
type PeerHonestyConfig struct {
	DecayCoefficient             float64 `yaml:"decayCoefficient"`
	DecayUpdateIntervalInSeconds uint32  `yaml:"decayUpdateIntervalInSeconds"`
	MaxScore                     float64 `yaml:"maxScore"`
	MinScore                     float64 `yaml:"minScore"`
	BadPeerThreshold             float64 `yaml:"badPeerThreshold"`
	UnitValue                    float64 `yaml:"unitValue"`
}
