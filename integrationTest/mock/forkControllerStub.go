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
	FixAuditChangesCalled         func() bool
	EpochRewardsV2Called          func() bool
	FixAuditChangesV2Called       func() bool
	EnableRWAPermissionsCalled    func() bool
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
	if fc.FPRComputeAndKdaFeeFlowCalled != nil {
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

func (fc *ForkControllerStub) FixAuditChanges() bool {
	if fc.FixAuditChangesCalled != nil {
		return fc.FixAuditChangesCalled()
	}

	return false
}

func (fc *ForkControllerStub) EpochRewardsV2() bool {
	if fc.EpochRewardsV2Called != nil {
		return fc.EpochRewardsV2Called()
	}

	return false
}

func (fc *ForkControllerStub) FixAuditChangesV2() bool {
	if fc.FixAuditChangesV2Called != nil {
		return fc.FixAuditChangesV2Called()
	}

	return false
}

func (fc *ForkControllerStub) EnableRWAPermissions() bool {
	if fc.EnableRWAPermissionsCalled != nil {
		return fc.EnableRWAPermissionsCalled()
	}

	return false
}

// IsInterfaceNil -
func (fc *ForkControllerStub) IsInterfaceNil() bool {
	return fc == nil
}
