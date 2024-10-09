package core

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddress_isSmartContractAddress(t *testing.T) {
	t.Parallel()

	// invalid length
	address, _ := hex.DecodeString("12345")
	assert.False(t, IsSmartContractAddress(address))

	// not enough leading zeros
	address, _ = hex.DecodeString("000000000001000000005fed9c659422cd8429ce92f8973bba2a9fb51e0eb3a1")
	assert.False(t, IsSmartContractAddress(address))

	// invalid VM type
	address, _ = hex.DecodeString("000000000000000006005fed9c659422cd8429ce92f8973bba2a9fb51e0eb3a1")
	assert.False(t, IsSmartContractAddress(address))

	// valid smart contract address
	scaddress, _ := hex.DecodeString("000000000000000005005fed9c659422cd8429ce92f8973bba2a9fb51e0eb3a1")
	assert.True(t, IsSmartContractAddress(scaddress))

	// empty address should pass no matter if vm type provided or not
	emAddress, _ := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000000")
	assert.True(t, IsSmartContractAddress(emAddress))
}
