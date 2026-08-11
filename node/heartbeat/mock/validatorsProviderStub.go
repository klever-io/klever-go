package mock

import "github.com/klever-io/klever-go/data/state"

// ValidatorsProviderStub -
type ValidatorsProviderStub struct {
	GetLatestValidatorsCalled func() (map[string]*state.ValidatorApiResponse, error)
	GetLatestPeersCalled      func() ([]state.PeerAccountHandler, error)
}

// GetLatestValidators -
func (vp *ValidatorsProviderStub) GetLatestValidators() (map[string]*state.ValidatorApiResponse, error) {
	if vp.GetLatestValidatorsCalled != nil {
		return vp.GetLatestValidatorsCalled()
	}
	return nil, nil
}

// GetLatestPeers -
func (vp *ValidatorsProviderStub) GetLatestPeers() ([]state.PeerAccountHandler, error) {
	if vp.GetLatestPeersCalled != nil {
		return vp.GetLatestPeersCalled()
	}
	return nil, nil
}

// IsInterfaceNil -
func (vp *ValidatorsProviderStub) IsInterfaceNil() bool {
	return vp == nil
}
