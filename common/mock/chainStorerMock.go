package mock

import (
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/storage"
	"github.com/pkg/errors"
)

// ChainStorerMock is a mock implementation of the ChainStorer interface
type ChainStorerMock struct {
	AddStorerCalled     func(key retriever.UnitType, s storage.Storer)
	GetStorerCalled     func(unitType retriever.UnitType) storage.Storer
	HasCalled           func(unitType retriever.UnitType, key []byte) error
	GetCalled           func(unitType retriever.UnitType, key []byte) ([]byte, error)
	PutCalled           func(unitType retriever.UnitType, key []byte, value []byte) error
	GetAllCalled        func(unitType retriever.UnitType, keys [][]byte) (map[string][]byte, error)
	GetAllStorersCalled func() map[retriever.UnitType]storage.Storer
	DestroyCalled       func() error
	CloseAllCalled      func() error
}

// CloseAll -
func (bc *ChainStorerMock) CloseAll() error {
	if bc.CloseAllCalled != nil {
		return bc.CloseAllCalled()
	}
	return nil
}

// AddStorer will add a new storer to the chain map
func (bc *ChainStorerMock) AddStorer(key retriever.UnitType, s storage.Storer) {
	if bc.AddStorerCalled != nil {
		bc.AddStorerCalled(key, s)
	}
}

// GetStorer returns the storer from the chain map or nil if the storer was not found
func (bc *ChainStorerMock) GetStorer(unitType retriever.UnitType) storage.Storer {
	if bc.GetStorerCalled != nil {
		return bc.GetStorerCalled(unitType)
	}
	return nil
}

// GetAllStorers -
func (bc *ChainStorerMock) GetAllStorers() map[retriever.UnitType]storage.Storer {
	if bc.GetAllStorersCalled != nil {
		return bc.GetAllStorersCalled()
	}

	return nil
}

// Has returns true if the key is found in the selected Unit or false otherwise
// It can return an error if the provided unit type is not supported or if the
// underlying implementation of the storage unit reports an error.
func (bc *ChainStorerMock) Has(unitType retriever.UnitType, key []byte) error {
	if bc.HasCalled != nil {
		return bc.HasCalled(unitType, key)
	}
	return errors.New("Key not found")
}

// Get returns the value for the given key if found in the selected storage unit,
// nil otherwise. It can return an error if the provided unit type is not supported
// or if the storage unit underlying implementation reports an error
func (bc *ChainStorerMock) Get(unitType retriever.UnitType, key []byte) ([]byte, error) {
	if bc.GetCalled != nil {
		return bc.GetCalled(unitType, key)
	}
	return nil, nil
}

// Put stores the key, value pair in the selected storage unit
// It can return an error if the provided unit type is not supported
// or if the storage unit underlying implementation reports an error
func (bc *ChainStorerMock) Put(unitType retriever.UnitType, key []byte, value []byte) error {
	if bc.PutCalled != nil {
		return bc.PutCalled(unitType, key, value)
	}
	return nil
}

// GetAll gets all the elements with keys in the keys array, from the selected storage unit
// It can report an error if the provided unit type is not supported, if there is a missing
// key in the unit, or if the underlying implementation of the storage unit reports an error.
func (bc *ChainStorerMock) GetAll(unitType retriever.UnitType, keys [][]byte) (map[string][]byte, error) {
	if bc.GetAllCalled != nil {
		return bc.GetAllCalled(unitType, keys)
	}
	return nil, nil
}

// SetEpochForPutOperation won't do anything
func (bc *ChainStorerMock) SetEpochForPutOperation(epoch uint32) {
}

// Destroy removes the underlying files/resources used by the storage service
func (bc *ChainStorerMock) Destroy() error {
	if bc.DestroyCalled != nil {
		return bc.DestroyCalled()
	}
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (bc *ChainStorerMock) IsInterfaceNil() bool {
	return bc == nil
}
