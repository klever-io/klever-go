package config

import "fmt"

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
	FixAuditChangesV3       uint32 `yaml:"fixAuditChangesV3"`
	FixAuditChangesV4       uint32 `yaml:"fixAuditChangesV4"`
	FixJailedPromotionOrder uint32 `yaml:"fixJailedPromotionOrder"`
}

// Validate checks that the configured activation epochs are mutually consistent. It runs
// on the operator config at node start-up, so an inconsistent schedule fails loudly
// instead of silently disabling the behaviour it was meant to activate.
func (e EnableEpochs) Validate() error {
	// The account freeze (common.IsAccountFrozen) holds an account only while
	// fixMarketBuyOverflow is active and fixAuditChangesV4 is not, so a thaw epoch that
	// is not strictly after the freeze epoch leaves an empty window and the listed
	// accounts are never immobilised. Epoch 0 is the template placeholder rather than a
	// real schedule, so only a configured freeze epoch is checked.
	if e.FixMarketBuyOverflow != 0 && e.FixAuditChangesV4 <= e.FixMarketBuyOverflow {
		return fmt.Errorf("fixAuditChangesV4 (%d) must be after fixMarketBuyOverflow (%d), "+
			"otherwise the account freeze window is empty",
			e.FixAuditChangesV4, e.FixMarketBuyOverflow)
	}

	return nil
}

// GasScheduleByEpochs represents a gas schedule toml entry that will be applied from the provided epoch
type GasScheduleByEpochs struct {
	StartEpoch uint32 `yaml:"startEpoch"`
	FileName   string `yaml:"fileName"`
}
