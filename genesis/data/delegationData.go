package data

// DelegationData specify the delegation address and the balance provided
type DelegationData struct {
	Address      string `json:"address"`
	Value        int64  `json:"value"`
	addressBytes []byte
}

// AddressBytes will return the delegation address as raw bytes
func (dd *DelegationData) AddressBytes() []byte {
	return dd.addressBytes
}

// SetAddressBytes will set the delegation address as raw bytes
func (dd *DelegationData) SetAddressBytes(address []byte) {
	dd.addressBytes = address
}

// Clone will return a new instance of the delegation data holding the same information
func (dd *DelegationData) Clone() *DelegationData {
	newDelegationData := &DelegationData{
		Address:      dd.Address,
		Value:        dd.Value,
		addressBytes: make([]byte, len(dd.addressBytes)),
	}
	copy(newDelegationData.addressBytes, dd.addressBytes)

	return newDelegationData
}

// GetAddress returns the address as string
func (dd *DelegationData) GetAddress() string {
	return dd.Address
}

// GetValue returns the delegated value
func (dd *DelegationData) GetValue() int64 {
	return dd.Value
}

// SetValue returns the delegated value
func (dd *DelegationData) SetValue(value int64) {
	dd.Value = value
}

// IsInterfaceNil returns if underlying object is true
func (dd *DelegationData) IsInterfaceNil() bool {
	return dd == nil
}
