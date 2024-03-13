package mock

import (
	"github.com/klever-io/klever-go/genesis"
)

// AccountsParserStub -
type AccountsParserStub struct {
	InitialAccountsCalled                    func() []genesis.InitialAccountHandler
	GetTotalStakedForDelegationAddressCalled func(delegationAddress string) int64
	GetInitialAccountsForDelegatedCalled     func(addressBytes []byte) []genesis.InitialAccountHandler
}

// GetTotalStakedForDelegationAddress -
func (aps *AccountsParserStub) GetTotalStakedForDelegationAddress(delegationAddress string) int64 {
	if aps.GetTotalStakedForDelegationAddressCalled != nil {
		return aps.GetTotalStakedForDelegationAddressCalled(delegationAddress)
	}

	return 0
}

// GetInitialAccountsForDelegated -
func (aps *AccountsParserStub) GetInitialAccountsForDelegated(addressBytes []byte) []genesis.InitialAccountHandler {
	if aps.GetInitialAccountsForDelegatedCalled != nil {
		return aps.GetInitialAccountsForDelegatedCalled(addressBytes)
	}

	return make([]genesis.InitialAccountHandler, 0)
}

// InitialAccounts -
func (aps *AccountsParserStub) InitialAccounts() []genesis.InitialAccountHandler {
	if aps.InitialAccountsCalled != nil {
		return aps.InitialAccountsCalled()
	}

	return make([]genesis.InitialAccountHandler, 0)
}

// IsInterfaceNil -
func (aps *AccountsParserStub) IsInterfaceNil() bool {
	return aps == nil
}
