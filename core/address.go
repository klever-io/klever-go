package core

import (
	"bytes"

	"github.com/klever-io/klever-go/common"
)

// SystemAccountAddress is the hard-coded address in which we save global settings on all shards
var SystemAccountAddress = bytes.Repeat([]byte{255}, 32)

// NumInitCharactersForScAddress numbers of characters for smart contract address identifier
const NumInitCharactersForScAddress = 10

// VMTypeLen number of characters with VMType identifier in an address, these are the last 2 characters from the
// initial identifier
const VMTypeLen = 2

// ShardIdentiferLen number of characters for shard identifier in an address
const ShardIdentiferLen = 2

const numInitCharactersForSystemAccountAddress = 30

// IsSystemAccountAddress returns true if given address is system account address
func IsSystemAccountAddress(address []byte) bool {
	if len(address) < numInitCharactersForSystemAccountAddress {
		return false
	}
	return bytes.Equal(address[:numInitCharactersForSystemAccountAddress], SystemAccountAddress[:numInitCharactersForSystemAccountAddress])
}

// IsSmartContractAddress verifies if a set address is of type smart contract
func IsSmartContractAddress(rcvAddress []byte) bool {
	if len(rcvAddress) <= NumInitCharactersForScAddress {
		return false
	}

	if IsEmptyAddress(rcvAddress) {
		return true
	}

	numOfZeros := NumInitCharactersForScAddress - VMTypeLen
	isSCAddress := bytes.Equal(rcvAddress[:numOfZeros], make([]byte, numOfZeros))
	isValidVMType := bytes.Equal(rcvAddress[numOfZeros:NumInitCharactersForScAddress], common.WasmVirtualMachine)
	return isSCAddress && isValidVMType
}

// IsEmptyAddress returns whether an address is empty
func IsEmptyAddress(address []byte) bool {
	isEmptyAddress := bytes.Equal(address, make([]byte, len(address)))
	return isEmptyAddress
}
