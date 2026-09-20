package mock

import "github.com/klever-io/klever-go/config"

// ForkControllerStub is a stub implementation of the ForkController for testing purposes
type ForkControllerStub struct {
	ProcessorFlowITOPriceValue   bool
	ClaimKFIValue                bool
	FixStakingBucketsValue       bool
	KdaFprValue                  bool
	BigBucketsComputeValue       bool
	FPRComputeAndKdaFeeFlowValue bool
	FixDelegationSameEpochValue  bool
	EnableSmartContractsValue    bool
	FixAuditChangesValue         bool
	EpochRewardsV2Value          bool
	FixAuditChangesV2Value       bool
	FixMarketBuyOverflowValue    bool
	FixAuditChangesV3Value       bool
	FixAuditChangesV4Value       bool
	FixAuditChangesV5Value       bool
	FixAuditChangesV6Value       bool
	EpochConfirmedCalled         bool
	LastConfirmedEpoch           uint32

	// FixAuditChangesV5Epoch, when non-nil, is the activation epoch answered by
	// FixAuditChangesV5InEpoch. When nil the stub falls back to FixAuditChangesV5Value, so
	// the epoch-aware gate matches the boolean one for tests that only set the latter.
	FixAuditChangesV5Epoch *uint32
	// FixAuditChangesV6Epoch, when non-nil, is the activation epoch answered by
	// FixAuditChangesV6InEpoch. When nil the stub falls back to FixAuditChangesV6Value, so
	// the epoch-aware gate matches the boolean one for tests that only set the latter.
	FixAuditChangesV6Epoch *uint32
}

func NewForkControllerStub() *ForkControllerStub {
	// default values all true
	f := &ForkControllerStub{}
	f.SetAll(true)

	return f
}

func (s *ForkControllerStub) SetFork(forkName string, value bool) *ForkControllerStub {
	switch forkName {
	case "ProcessorFlowITOPrice":
		s.ProcessorFlowITOPriceValue = value
	case "ClaimKFI":
		s.ClaimKFIValue = value
	case "FixStakingBuckets":
		s.FixStakingBucketsValue = value
	case "KdaFpr":
		s.KdaFprValue = value
	case "BigBucketsCompute":
		s.BigBucketsComputeValue = value
	case "FPRComputeAndKdaFeeFlow":
		s.FPRComputeAndKdaFeeFlowValue = value
	case "FixDelegationSameEpoch":
		s.FixDelegationSameEpochValue = value
	case "EnableSmartContracts":
		s.EnableSmartContractsValue = value
	case "FixAuditChanges":
		s.FixAuditChangesValue = value
	case "EpochRewardsV2":
		s.EpochRewardsV2Value = value
	case "FixAuditChangesV2":
		s.FixAuditChangesV2Value = value
	case "FixMarketBuyOverflow":
		s.FixMarketBuyOverflowValue = value
	case "FixAuditChangesV3":
		s.FixAuditChangesV3Value = value
	case "FixAuditChangesV4":
		s.FixAuditChangesV4Value = value
	case "FixAuditChangesV5":
		s.FixAuditChangesV5Value = value
	case "FixAuditChangesV6":
		s.FixAuditChangesV6Value = value
	}

	return s
}

// SetAll sets all values
func (s *ForkControllerStub) SetAll(value bool) {
	s.ProcessorFlowITOPriceValue = value
	s.ClaimKFIValue = value
	s.FixStakingBucketsValue = value
	s.KdaFprValue = value
	s.BigBucketsComputeValue = value
	s.FPRComputeAndKdaFeeFlowValue = value
	s.FixDelegationSameEpochValue = value
	s.EnableSmartContractsValue = value
	s.FixAuditChangesValue = value
	s.EpochRewardsV2Value = value
	s.FixAuditChangesV2Value = value
	s.FixMarketBuyOverflowValue = value
	s.FixAuditChangesV3Value = value
	s.FixAuditChangesV4Value = value
	s.FixAuditChangesV5Value = value
	s.FixAuditChangesV6Value = value
	s.LastConfirmedEpoch = 0
}

// SetByConfig sets values based in the EnableEpochs config
func (s *ForkControllerStub) SetByConfig(config config.EnableEpochs) {
	s.ProcessorFlowITOPriceValue = config.ProcessorFlowITOPrice == 0
	s.ClaimKFIValue = config.ClaimKFI == 0
	s.FixStakingBucketsValue = config.FixStakingBuckets == 0
	s.KdaFprValue = config.KdaFpr == 0
	s.BigBucketsComputeValue = config.BigBucketsCompute == 0
	s.FPRComputeAndKdaFeeFlowValue = config.FPRComputeAndKdaFeeFlow == 0
	s.FixDelegationSameEpochValue = config.FixDelegationSameEpoch == 0
	s.EnableSmartContractsValue = config.SmartContracts == 0
	s.FixAuditChangesValue = config.FixAuditChanges == 0
	s.EpochRewardsV2Value = config.EpochRewardsV2 == 0
	s.FixAuditChangesV2Value = config.FixAuditChangesV2 == 0
	s.FixMarketBuyOverflowValue = config.FixMarketBuyOverflow == 0
	s.FixAuditChangesV3Value = config.FixAuditChangesV3 == 0
	s.FixAuditChangesV4Value = config.FixAuditChangesV4 == 0
	s.FixAuditChangesV5Value = config.FixAuditChangesV5 == 0
	s.FixAuditChangesV5Epoch = &config.FixAuditChangesV5
	s.FixAuditChangesV6Value = config.FixAuditChangesV6 == 0
	s.FixAuditChangesV6Epoch = &config.FixAuditChangesV6
	s.LastConfirmedEpoch = 0
}

// ProcessorFlowITOPrice returns the stubbed value
func (s *ForkControllerStub) ProcessorFlowITOPrice() bool {
	return s.ProcessorFlowITOPriceValue
}

// ClaimKFI returns the stubbed value
func (s *ForkControllerStub) ClaimKFI() bool {
	return s.ClaimKFIValue
}

// FixStakingBuckets returns the stubbed value
func (s *ForkControllerStub) FixStakingBuckets() bool {
	return s.FixStakingBucketsValue
}

// KdaFpr returns the stubbed value
func (s *ForkControllerStub) KdaFpr() bool {
	return s.KdaFprValue
}

// BigBucketsCompute returns the stubbed value
func (s *ForkControllerStub) BigBucketsCompute() bool {
	return s.BigBucketsComputeValue
}

// FPRComputeAndKdaFeeFlow returns the stubbed value
func (s *ForkControllerStub) FPRComputeAndKdaFeeFlow() bool {
	return s.FPRComputeAndKdaFeeFlowValue
}

// FixDelegationSameEpoch returns the stubbed value
func (s *ForkControllerStub) FixDelegationSameEpoch() bool {
	return s.FixDelegationSameEpochValue
}

// EnableSmartContracts returns the stubbed value
func (s *ForkControllerStub) EnableSmartContracts() bool {
	return s.EnableSmartContractsValue
}

// FixAuditChanges returns the stubbed value
func (s *ForkControllerStub) FixAuditChanges() bool {
	return s.FixAuditChangesValue
}

// EpochRewardsV2 returns the stubbed value
func (s *ForkControllerStub) EpochRewardsV2() bool {
	return s.EpochRewardsV2Value
}

// FixAuditChangesV2 returns the stubbed value
func (s *ForkControllerStub) FixAuditChangesV2() bool {
	return s.FixAuditChangesV2Value
}

// FixMarketBuyOverflow returns the stubbed value
func (s *ForkControllerStub) FixMarketBuyOverflow() bool {
	return s.FixMarketBuyOverflowValue
}

// FixAuditChangesV3 returns the stubbed value
func (s *ForkControllerStub) FixAuditChangesV3() bool {
	return s.FixAuditChangesV3Value
}

// FixAuditChangesV4 returns the stubbed value
func (s *ForkControllerStub) FixAuditChangesV4() bool {
	return s.FixAuditChangesV4Value
}

// FixAuditChangesV5 returns the stubbed value
func (s *ForkControllerStub) FixAuditChangesV5() bool {
	return s.FixAuditChangesV5Value
}

// FixAuditChangesV5InEpoch returns the stubbed value for an explicit epoch, using the
// configured activation epoch when one was set and the boolean value otherwise
func (s *ForkControllerStub) FixAuditChangesV5InEpoch(epoch uint32) bool {
	if s.FixAuditChangesV5Epoch == nil {
		return s.FixAuditChangesV5Value
	}

	return epoch >= *s.FixAuditChangesV5Epoch
}

// FixAuditChangesV6 returns the stubbed value
func (s *ForkControllerStub) FixAuditChangesV6() bool {
	return s.FixAuditChangesV6Value
}

// FixAuditChangesV6InEpoch returns the stubbed value for an explicit epoch, using the
// configured activation epoch when one was set and the boolean value otherwise
func (s *ForkControllerStub) FixAuditChangesV6InEpoch(epoch uint32) bool {
	if s.FixAuditChangesV6Epoch == nil {
		return s.FixAuditChangesV6Value
	}

	return epoch >= *s.FixAuditChangesV6Epoch
}

// EpochConfirmed records that the method was called and stores the epoch
func (s *ForkControllerStub) EpochConfirmed(epoch uint32) {
	s.EpochConfirmedCalled = true
	s.LastConfirmedEpoch = epoch
}

// IsInterfaceNil returns false as this is a stub implementation
func (s *ForkControllerStub) IsInterfaceNil() bool {
	return false
}
