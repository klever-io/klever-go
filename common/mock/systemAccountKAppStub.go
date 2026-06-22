package mock

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
)

// SystemAccountKappStub is a stub implementation of the SystemAccountKapp interface
type SystemAccountKappStub struct {
	SetKAppControllerCalled  func(controller kapp.KAppController) error
	SetAccountsCacherCalled  func(cacher state.AccountsCacher) error
	SFTSetMetadataCalled     func(asset, nonce []byte, args [][]byte) error
	SFTAddCirculationCalled  func(asset, nonce []byte, amount int64) error
	SFTCreateMetaCalled      func(asset, nonce []byte, supply int64, hash []byte) error
	SFTGetMetaCalled         func(asset, nonce []byte) (*kapps.MetaV2, error)
	SFTGetMetaUncachedCalled func(asset, nonce []byte) (*kapps.MetaV2, error)
}

// SetKAppController is the mock implementation for SetKAppController
func (saks *SystemAccountKappStub) SetKAppController(controller kapp.KAppController) error {
	if saks.SetKAppControllerCalled != nil {
		return saks.SetKAppControllerCalled(controller)
	}
	return nil
}

// SetAccountsCacher is the mock implementation for SetAccountsCacher
func (saks *SystemAccountKappStub) SetAccountsCacher(cacher state.AccountsCacher) error {
	if saks.SetAccountsCacherCalled != nil {
		return saks.SetAccountsCacherCalled(cacher)
	}
	return nil
}

// SFTSetMetadata is the mock implementation for SFTSetMetadata
func (saks *SystemAccountKappStub) SFTSetMetadata(asset, nonce []byte, args [][]byte) error {
	if saks.SFTSetMetadataCalled != nil {
		return saks.SFTSetMetadataCalled(asset, nonce, args)
	}
	return nil
}

// SFTAddCirculation is the mock implementation for SFTAddCirculation
func (saks *SystemAccountKappStub) SFTAddCirculation(asset, nonce []byte, amount int64) error {
	if saks.SFTAddCirculationCalled != nil {
		return saks.SFTAddCirculationCalled(asset, nonce, amount)
	}
	return nil
}

// SFTCreateMeta is the mock implementation for SFTCreateMeta
func (saks *SystemAccountKappStub) SFTCreateMeta(asset, nonce []byte, supply int64, hash []byte) error {
	if saks.SFTCreateMetaCalled != nil {
		return saks.SFTCreateMetaCalled(asset, nonce, supply, hash)
	}
	return nil
}

// SFTGetMeta is the mock implementation for SFTGetMeta
func (saks *SystemAccountKappStub) SFTGetMeta(asset, nonce []byte) (*kapps.MetaV2, error) {
	if saks.SFTGetMetaCalled != nil {
		return saks.SFTGetMetaCalled(asset, nonce)
	}
	return nil, nil
}

// SFTGetMetaUncached is the mock implementation for SFTGetMetaUncached
func (saks *SystemAccountKappStub) SFTGetMetaUncached(asset, nonce []byte) (*kapps.MetaV2, error) {
	if saks.SFTGetMetaUncachedCalled != nil {
		return saks.SFTGetMetaUncachedCalled(asset, nonce)
	}
	return nil, nil
}

// IsInterfaceNil checks if the interface is nil
func (saks *SystemAccountKappStub) IsInterfaceNil() bool {
	return saks == nil
}
