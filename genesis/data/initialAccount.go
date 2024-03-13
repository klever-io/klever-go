package data

import (
	"github.com/klever-io/klever-go/genesis"
)

// InitialAccount provides information about one entry in the genesis file
type InitialAccount struct {
	Address      string          `json:"address"`
	Balance      int64           `json:"balance"`
	KFIBalance   int64           `json:"kfiBalance"`
	Delegation   *DelegationData `json:"delegation"`
	Permissions  *PermissionData `json:"permissions"`
	addressBytes []byte
}

// AddressBytes will return the address as raw bytes
func (ia *InitialAccount) AddressBytes() []byte {
	return ia.addressBytes
}

// SetAddressBytes will set the address as raw bytes
func (ia *InitialAccount) SetAddressBytes(address []byte) {
	ia.addressBytes = address
}

// Clone will return a new instance of the initial account holding the same information
func (ia *InitialAccount) Clone() genesis.InitialAccountHandler {
	newInitialAccount := &InitialAccount{
		Address:      ia.Address,
		Balance:      ia.Balance,
		KFIBalance:   ia.KFIBalance,
		Delegation:   ia.Delegation.Clone(),
		addressBytes: make([]byte, len(ia.addressBytes)),
	}

	copy(newInitialAccount.addressBytes, ia.addressBytes)

	return newInitialAccount
}

// GetAddress returns the address of the initial account
func (ia *InitialAccount) GetAddress() string {
	return ia.Address
}

// GetBalanceValue returns the initial balance value
func (ia *InitialAccount) GetBalanceValue() int64 {
	return ia.Balance
}

// SetBalanceValue returns the initial balance value
func (ia *InitialAccount) SetBalanceValue(value int64) {
	ia.Balance = value
}

// GetKFIBalanceValue returns the initial balance value
func (ia *InitialAccount) GetKFIBalanceValue() int64 {
	return ia.KFIBalance
}

// SetKFIBalanceValue returns the initial balance value
func (ia *InitialAccount) SetKFIBalanceValue(value int64) {
	ia.KFIBalance = value
}

// GetDelegationHandler returns the delegation handler
func (ia *InitialAccount) GetDelegationHandler() genesis.DelegationDataHandler {
	return ia.Delegation
}

// GetPermissionsHandler returns the permission handler
func (ia *InitialAccount) GetPermissionsHandler() genesis.PermissionsDataHandler {
	return ia.Permissions
}

// IsInterfaceNil returns if underlying object is true
func (ia *InitialAccount) IsInterfaceNil() bool {
	return ia == nil
}
