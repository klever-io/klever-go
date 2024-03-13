package mock

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/kapps"
)

// NodesHelperMock defines the behaviour of a node helper
type NodesHelperMock struct {
	GetAssetCalled                func(address string) (*kapps.KDAData, error)
	GetNFTCalled                  func(owner string, address string) (*kapps.UserKDA, *kapps.KDAData, error)
	GetAddressPCKCalled           func() core.PubkeyConverter
	GetValidatorPCKCalled         func() core.PubkeyConverter
	GetEncodedAddressLengthCalled func() int
	GetForkControllerCalled       func() core.ForkController
	IsInterfaceNilCalled          func() bool
}

// NewNodeHelperMock -
func NewNodeHelperMock() *NodesHelperMock {
	return &NodesHelperMock{}
}

// GetAsset -
func (nhm *NodesHelperMock) GetAsset(address string) (*kapps.KDAData, error) {
	if nhm.GetAssetCalled != nil {
		return nhm.GetAssetCalled(address)
	}
	return nil, nil
}

// GetNFT -
func (nhm *NodesHelperMock) GetNFT(owner string, address string) (*kapps.UserKDA, *kapps.KDAData, error) {
	if nhm.GetNFTCalled != nil {
		return nhm.GetNFTCalled(owner, address)
	}
	return nil, nil, nil
}

// GetAddressPCK -
func (nhm *NodesHelperMock) GetAddressPCK() core.PubkeyConverter {
	if nhm.GetAddressPCKCalled != nil {
		return nhm.GetAddressPCKCalled()
	}
	return nil
}

// GetValidatorPCK -
func (nhm *NodesHelperMock) GetValidatorPCK() core.PubkeyConverter {
	if nhm.GetValidatorPCKCalled != nil {
		return nhm.GetValidatorPCKCalled()
	}
	return nil
}

// GetEncodedAddressLength -
func (nhm *NodesHelperMock) GetEncodedAddressLength() int {
	if nhm.GetEncodedAddressLengthCalled != nil {
		return nhm.GetEncodedAddressLengthCalled()
	}
	return 32
}

// GetForkController -
func (nhm *NodesHelperMock) GetForkController() core.ForkController {
	if nhm.GetForkControllerCalled != nil {
		return nhm.GetForkControllerCalled()
	}
	return nil
}

// IsInterfaceNil -
func (nhm *NodesHelperMock) IsInterfaceNil() bool {
	return nhm == nil
}
