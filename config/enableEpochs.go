package config

// EnableEpochsConfig-
type EnableEpochsConfig struct {
	// Shuffler
	MaxNodesChangeEnableEpoch []MaxNodesChangeConfig `yaml:"maxNodesChangeEnableEpoch"`
	EnableEpochs              EnableEpochs           `yaml:"enableEpochs"`
	GasSchedule               GasScheduleConfig      `yaml:"gasSchedule"`
}

// GasScheduleConfig represents the versioning config area for the gas schedule toml
type GasScheduleConfig struct {
	GasScheduleByEpochs []GasScheduleByEpochs `yaml:"gasScheduleByEpochs"`
}

// EnableEpochs will hold the configuration for activation epochs
type EnableEpochs struct {
	ClaimKFI                uint32 `yaml:"claimKFI"`
	ProcessorFlowITOPrice   uint32 `yaml:"processorFlowITOPrice"`
	FixStakingBuckets       uint32 `yaml:"fixStakingBuckets"`
	KdaFpr                  uint32 `yaml:"kdaFpr"`
	BigBucketsCompute       uint32 `yaml:"bigBucketsCompute"`
	FPRComputeAndKdaFeeFlow uint32 `yaml:"fprComputeAndKdaFeeFlow"`
	FixDelegationSameEpoch  uint32 `yaml:"fixDelegationSameEpoch"`
	SmartContracts          uint32 `yaml:"smartContracts"`
	FixAuditChanges         uint32 `yaml:"fixAuditChanges"`
	EpochRewardsV2          uint32 `yaml:"epochRewardsV2"`
	FixAuditChangesV2       uint32 `yaml:"fixAuditChangesV2"`
	FixMarketBuyOverflow    uint32 `yaml:"fixMarketBuyOverflow"`
}

// GasScheduleByEpochs represents a gas schedule toml entry that will be applied from the provided epoch
type GasScheduleByEpochs struct {
	StartEpoch uint32 `yaml:"startEpoch"`
	FileName   string `yaml:"fileName"`
}
