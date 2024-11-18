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
	EpochConfirmedCalled         bool
	LastConfirmedEpoch           uint32
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

// EpochConfirmed records that the method was called and stores the epoch
func (s *ForkControllerStub) EpochConfirmed(epoch uint32) {
	s.EpochConfirmedCalled = true
	s.LastConfirmedEpoch = epoch
}

// IsInterfaceNil returns false as this is a stub implementation
func (s *ForkControllerStub) IsInterfaceNil() bool {
	return false
}
