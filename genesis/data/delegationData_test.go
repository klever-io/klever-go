package data

import (
	"testing"

	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDelegationData_AddressBytes(t *testing.T) {
	t.Parallel()

	dd := &DelegationData{}
	addrBytes := []byte("address bytes")
	dd.SetAddressBytes(addrBytes)
	recoverdAddrBytes := dd.AddressBytes()

	assert.Equal(t, addrBytes, recoverdAddrBytes)
}

func TestDelegationData_Clone(t *testing.T) {
	t.Parallel()

	dd := &DelegationData{
		Address:      "address",
		Value:        45,
		addressBytes: []byte("address bytes"),
	}

	ddCloned := dd.Clone()

	assert.Equal(t, dd, ddCloned)
	assert.False(t, dd == ddCloned) //pointer testing
	assert.True(t, dd.Value == ddCloned.Value)
}

func TestDelegationData_Getters(t *testing.T) {
	t.Parallel()

	adr := "address"
	val := int64(45)
	dd := &DelegationData{
		Address: adr,
		Value:   val,
	}

	require.False(t, check.IfNil(dd))
	assert.Equal(t, adr, dd.GetAddress())
	assert.Equal(t, val, dd.GetValue())
}
