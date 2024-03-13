package factory

import (
	"github.com/klever-io/klever-go/data/state"
)

// KAppAccountCreator has a method to create a new peer account
type KAppAccountCreator struct {
}

// NewKAppAccountCreator creates a peer account creator
func NewKAppAccountCreator() state.AccountFactory {
	return &KAppAccountCreator{}
}

// CreateAccount calls the new Account creator and returns the result
func (kac *KAppAccountCreator) CreateAccount(address []byte) (state.AccountHandler, error) {
	return state.NewKAppAccount(address)
}

// IsInterfaceNil returns true if there is no value under the interface
func (kac *KAppAccountCreator) IsInterfaceNil() bool {
	return kac == nil
}
