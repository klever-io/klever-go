package mock

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
	return &ForkControllerStub{}
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
