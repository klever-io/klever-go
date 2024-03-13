package mock

const uint32Size = 4
const numUint32 = 2

// ValidatorMock -
type ValidatorMock struct {
	ownerAddress []byte
	pubKey       []byte
	chances      uint32
	index        uint32

	OwnerAddressCalled func() []byte
	PubKeyCalled       func() []byte
}

// NewValidatorMock -
func NewValidatorMock(ownerAddress []byte, pubKey []byte, chances uint32, index uint32) *ValidatorMock {
	return &ValidatorMock{ownerAddress: ownerAddress, index: index, chances: chances}
}

// OwnerAddress -
func (vm *ValidatorMock) OwnerAddress() []byte {
	if vm.OwnerAddressCalled != nil {
		return vm.OwnerAddressCalled()
	}
	return vm.ownerAddress
}

// PubKey -
func (vm *ValidatorMock) PubKey() []byte {
	if vm.PubKeyCalled != nil {
		return vm.PubKeyCalled()
	}
	return vm.pubKey
}

// Chances -
func (vm *ValidatorMock) Chances() uint32 {
	return vm.chances
}

// Size -
func (vm *ValidatorMock) Size() int {
	return len(vm.pubKey) + numUint32*uint32Size
}

// Index -
func (vm *ValidatorMock) Index() uint32 {
	return vm.index
}
