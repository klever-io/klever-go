package mock

type ForkControllerStub struct {
	ProcessorFlowITOPriceCalled   func() bool
	ClaimKFICalled                func() bool
	FixStakingBucketsCalled       func() bool
	KdaFprCalled                  func() bool
	BigBucketsComputeCalled       func() bool
	FPRComputeAndKdaFeeFlowCalled func() bool
	FixDelegationSameEpochCalled  func() bool
	EnableSmartContractsCalled    func() bool
}

// ProcessorFlowITOPrice -
func (fc *ForkControllerStub) ProcessorFlowITOPrice() bool {
	if fc.ProcessorFlowITOPriceCalled != nil {
		return fc.ProcessorFlowITOPriceCalled()
	}

	return false
}

// ClaimKFI -
func (fc *ForkControllerStub) ClaimKFI() bool {
	if fc.ClaimKFICalled != nil {
		return fc.ClaimKFICalled()
	}

	return false
}

// FixStakingBuckets -
func (fc *ForkControllerStub) KdaFpr() bool {
	if fc.KdaFprCalled != nil {
		return fc.KdaFprCalled()
	}

	return false
}

// FixStakingBuckets -
func (fc *ForkControllerStub) FixStakingBuckets() bool {
	if fc.FixStakingBucketsCalled != nil {
		return fc.FixStakingBucketsCalled()
	}

	return false
}

func (fc *ForkControllerStub) BigBucketsCompute() bool {
	if fc.BigBucketsComputeCalled != nil {
		return fc.BigBucketsComputeCalled()
	}

	return false
}

func (fc *ForkControllerStub) FPRComputeAndKdaFeeFlow() bool {
	if fc.BigBucketsComputeCalled != nil {
		return fc.FPRComputeAndKdaFeeFlowCalled()
	}

	return false
}

func (fc *ForkControllerStub) FixDelegationSameEpoch() bool {
	if fc.FixDelegationSameEpochCalled != nil {
		return fc.FixDelegationSameEpochCalled()
	}

	return false
}

func (fc *ForkControllerStub) EnableSmartContracts() bool {
	if fc.EnableSmartContractsCalled != nil {
		return fc.EnableSmartContractsCalled()
	}

	return false
}

// IsInterfaceNil -
func (fc *ForkControllerStub) IsInterfaceNil() bool {
	return fc == nil
}
