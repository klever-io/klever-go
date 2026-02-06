package mock

import "github.com/klever-io/klever-go/data/state"

// PeerAccountHandlerMock -
type PeerAccountHandlerMock struct {
	IncreaseLeaderSuccessRateValue               uint32
	DecreaseLeaderSuccessRateValue               uint32
	IncreaseValidatorSuccessRateValue            uint32
	DecreaseValidatorSuccessRateValue            uint32
	IncreaseValidatorIgnoredSignaturesValue      uint32
	IncreaseLeaderSuccessRateCalled              func(uint32)
	DecreaseLeaderSuccessRateCalled              func(uint32)
	IncreaseValidatorSuccessRateCalled           func(uint32)
	DecreaseValidatorSuccessRateCalled           func(uint32)
	IncreaseValidatorIgnoredSignaturesRateCalled func(uint32)
	SetTempRatingCalled                          func(uint32)
	GetTempRatingCalled                          func() uint32
	AddToAccumulatedFeesCalled                   func(int64)
	GetAccumulatedFeesCalled                     func() int64
	GetConsecutiveProposerMissesCalled           func() uint32
	SetConsecutiveProposerMissesCalled           func(uint32)
	SetListAndIndexCalled                        func(list state.List, index uint32)
	GetListCalled                                func() state.List
	CopyFromCalled                               func(state.PeerAccountHandler) error
}

// GetBLSPublicKey -
func (p *PeerAccountHandlerMock) GetBLSPublicKey() []byte { return nil }

// SetBLSPublicKey -
func (p *PeerAccountHandlerMock) SetBLSPublicKey([]byte) error { return nil }

// GetOwnerAddress -
func (p *PeerAccountHandlerMock) GetOwnerAddress() []byte { return nil }

// SetOwnerAddress -
func (p *PeerAccountHandlerMock) SetOwnerAddress([]byte) error { return nil }

// SetRevoked -
func (p *PeerAccountHandlerMock) SetRevoked() {}

// GetRevoked -
func (p *PeerAccountHandlerMock) GetRevoked() bool { return false }

// GetAccumulatedFees -
func (p *PeerAccountHandlerMock) GetAccumulatedFees() int64 {
	if p.GetAccumulatedFeesCalled != nil {
		return p.GetAccumulatedFeesCalled()
	}
	return 0
}

// AddToAccumulatedFees -
func (p *PeerAccountHandlerMock) AddToAccumulatedFees(val int64) {
	if p.AddToAccumulatedFeesCalled != nil {
		p.AddToAccumulatedFeesCalled(val)
	}
}

// GetList -
func (p *PeerAccountHandlerMock) GetList() state.List {
	if p.GetListCalled != nil {
		return p.GetListCalled()
	}
	return state.List_inactive
}

// GetIndex -
func (p *PeerAccountHandlerMock) GetIndex() uint32 { return 0 }

// GetListString -
func (p *PeerAccountHandlerMock) GetListString() string { return "" }

// SetList -
func (p *PeerAccountHandlerMock) SetList(_ state.List) {}

// SetListAndIndex -
func (p *PeerAccountHandlerMock) SetListAndIndex(list state.List, index uint32) {
	if p.SetListAndIndexCalled != nil {
		p.SetListAndIndexCalled(list, index)
	}
}

// GetLeaderSuccessRateSuccess -
func (p *PeerAccountHandlerMock) GetLeaderSuccessRateSuccess() uint32 { return 0 }

// GetTotalLeaderSuccessRateSuccess -
func (p *PeerAccountHandlerMock) GetTotalLeaderSuccessRateSuccess() uint32 { return 0 }

// GetValidatorSuccessRateSuccess -
func (p *PeerAccountHandlerMock) GetValidatorSuccessRateSuccess() uint32 { return 0 }

// GetTotalValidatorSuccessRateSuccess -
func (p *PeerAccountHandlerMock) GetTotalValidatorSuccessRateSuccess() uint32 { return 0 }

// GetLeaderSuccessRateFailure -
func (p *PeerAccountHandlerMock) GetLeaderSuccessRateFailure() uint32 { return 0 }

// GetTotalLeaderSuccessRateFailure -
func (p *PeerAccountHandlerMock) GetTotalLeaderSuccessRateFailure() uint32 { return 0 }

// GetValidatorSuccessRateFailure -
func (p *PeerAccountHandlerMock) GetValidatorSuccessRateFailure() uint32 { return 0 }

// GetTotalValidatorSuccessRateFailure -
func (p *PeerAccountHandlerMock) GetTotalValidatorSuccessRateFailure() uint32 { return 0 }

// IncreaseLeaderSuccessRate -
func (p *PeerAccountHandlerMock) IncreaseLeaderSuccessRate(val uint32) {
	if p.IncreaseLeaderSuccessRateCalled != nil {
		p.IncreaseLeaderSuccessRateCalled(val)
		return
	}
	p.IncreaseLeaderSuccessRateValue += val
}

// DecreaseLeaderSuccessRate -
func (p *PeerAccountHandlerMock) DecreaseLeaderSuccessRate(val uint32) {
	if p.DecreaseLeaderSuccessRateCalled != nil {
		p.DecreaseLeaderSuccessRateCalled(val)
		return
	}
	p.DecreaseLeaderSuccessRateValue += val
}

// IncreaseValidatorSuccessRate -
func (p *PeerAccountHandlerMock) IncreaseValidatorSuccessRate(val uint32) {
	if p.IncreaseValidatorSuccessRateCalled != nil {
		p.IncreaseValidatorSuccessRateCalled(val)
		return
	}
	p.IncreaseValidatorSuccessRateValue += val
}

// DecreaseValidatorSuccessRate -
func (p *PeerAccountHandlerMock) DecreaseValidatorSuccessRate(val uint32) {
	if p.DecreaseValidatorSuccessRateCalled != nil {
		p.DecreaseValidatorSuccessRateCalled(val)
		return
	}
	p.DecreaseValidatorSuccessRateValue += val
}

// IncreaseValidatorIgnoredSignaturesRate -
func (p *PeerAccountHandlerMock) IncreaseValidatorIgnoredSignaturesRate(val uint32) {
	if p.IncreaseValidatorIgnoredSignaturesRateCalled != nil {
		p.IncreaseValidatorIgnoredSignaturesRateCalled(val)
		return
	}
	p.IncreaseValidatorIgnoredSignaturesValue += val
}

// GetNumSelectedInSuccessBlocks -
func (p *PeerAccountHandlerMock) GetNumSelectedInSuccessBlocks() uint32 { return 0 }

// IncreaseNumSelectedInSuccessBlocks -
func (p *PeerAccountHandlerMock) IncreaseNumSelectedInSuccessBlocks() {}

// GetLeaderSuccessRate -
func (p *PeerAccountHandlerMock) GetLeaderSuccessRate() *state.SignRate { return &state.SignRate{} }

// GetValidatorSuccessRate -
func (p *PeerAccountHandlerMock) GetValidatorSuccessRate() *state.SignRate {
	return &state.SignRate{}
}

// GetValidatorIgnoredSignaturesRate -
func (p *PeerAccountHandlerMock) GetValidatorIgnoredSignaturesRate() uint32 { return 0 }

// GetTotalLeaderSuccessRate -
func (p *PeerAccountHandlerMock) GetTotalLeaderSuccessRate() *state.SignRate {
	return &state.SignRate{}
}

// GetTotalValidatorSuccessRate -
func (p *PeerAccountHandlerMock) GetTotalValidatorSuccessRate() *state.SignRate {
	return &state.SignRate{}
}

// GetTotalValidatorIgnoredSignaturesRate -
func (p *PeerAccountHandlerMock) GetTotalValidatorIgnoredSignaturesRate() uint32 { return 0 }

// GetRating -
func (p *PeerAccountHandlerMock) GetRating() uint32 { return 0 }

// SetRating -
func (p *PeerAccountHandlerMock) SetRating(uint32) {}

// GetTempRating -
func (p *PeerAccountHandlerMock) GetTempRating() uint32 {
	if p.GetTempRatingCalled != nil {
		return p.GetTempRatingCalled()
	}
	return 0
}

// SetTempRating -
func (p *PeerAccountHandlerMock) SetTempRating(val uint32) {
	if p.SetTempRatingCalled != nil {
		p.SetTempRatingCalled(val)
	}
}

// GetConsecutiveProposerMisses -
func (p *PeerAccountHandlerMock) GetConsecutiveProposerMisses() uint32 {
	if p.GetConsecutiveProposerMissesCalled != nil {
		return p.GetConsecutiveProposerMissesCalled()
	}
	return 0
}

// SetConsecutiveProposerMisses -
func (p *PeerAccountHandlerMock) SetConsecutiveProposerMisses(val uint32) {
	if p.SetConsecutiveProposerMissesCalled != nil {
		p.SetConsecutiveProposerMissesCalled(val)
	}
}

// ResetAtNewEpoch -
func (p *PeerAccountHandlerMock) ResetAtNewEpoch() {}

// CopyFrom -
func (p *PeerAccountHandlerMock) CopyFrom(handler state.PeerAccountHandler) error {
	if p.CopyFromCalled != nil {
		return p.CopyFromCalled(handler)
	}
	return nil
}

// AddressBytes -
func (p *PeerAccountHandlerMock) AddressBytes() []byte { return nil }

// IncreaseNonce -
func (p *PeerAccountHandlerMock) IncreaseNonce(_ uint64) {}

// GetNonce -
func (p *PeerAccountHandlerMock) GetNonce() uint64 { return 0 }

// IsInterfaceNil -
func (p *PeerAccountHandlerMock) IsInterfaceNil() bool { return p == nil }
