package mock

import "github.com/klever-io/klever-go/data/state"

// ValidatorsProviderStub -
type ValidatorsProviderStub struct {
	GetLatestValidatorsCalled func() map[string]*state.ValidatorApiResponse
	GetLatestPeersCalled      func() []state.PeerAccountHandler
}

// GetLatestValidators -
func (vp *ValidatorsProviderStub) GetLatestValidators() map[string]*state.ValidatorApiResponse {
	if vp.GetLatestValidatorsCalled != nil {
		return vp.GetLatestValidatorsCalled()
	}
	return nil
}

// GetLatestPeers -
func (vp *ValidatorsProviderStub) GetLatestPeers() []state.PeerAccountHandler {
	if vp.GetLatestValidatorsCalled != nil {
		return vp.GetLatestPeersCalled()
	}
	return nil
}

// IsInterfaceNil -
func (vp *ValidatorsProviderStub) IsInterfaceNil() bool {
	return vp == nil
}
