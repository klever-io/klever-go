package data

import (
	"testing"

	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createMockInitialAccount() *InitialAccount {
	return &InitialAccount{
		Address: "address",
		Balance: 2242,
		Delegation: &DelegationData{
			Address: "delegation address",
			Value:   4442,
		},
	}
}

func TestInitialAccount_AddressBytes(t *testing.T) {
	t.Parallel()

	ia := &InitialAccount{}
	addrBytes := []byte("address bytes")
	ia.SetAddressBytes(addrBytes)
	recoverdAddrBytes := ia.AddressBytes()

	assert.Equal(t, addrBytes, recoverdAddrBytes)
}

func TestInitialAccount_Clone(t *testing.T) {
	t.Parallel()

	ia := &InitialAccount{
		Address:      "address",
		Balance:      56,
		addressBytes: []byte("address bytes"),
		Delegation: &DelegationData{
			Address:      "delegation address",
			Value:        910,
			addressBytes: []byte("delegation address bytes"),
		},
	}

	iaCloned := ia.Clone()

	assert.Equal(t, ia, iaCloned)
	assert.False(t, ia == iaCloned) //pointer testing
	assert.True(t, ia.Balance == iaCloned.GetBalanceValue())
	assert.False(t, ia.Delegation == iaCloned.GetDelegationHandler())
}

func TestInitialAccount_Getters(t *testing.T) {
	t.Parallel()

	accountAddr := "account address"
	balance := int64(67)
	dd := &DelegationData{}
	ia := &InitialAccount{
		Address:    accountAddr,
		Balance:    balance,
		Delegation: dd,
	}

	require.False(t, check.IfNil(ia))
	require.False(t, check.IfNil(ia.GetDelegationHandler()))
	assert.Equal(t, accountAddr, ia.GetAddress())
	assert.Equal(t, balance, ia.GetBalanceValue())
	assert.Equal(t, dd, ia.GetDelegationHandler())
}
