package transaction

import (
	"encoding/hex"

	"github.com/klever-io/klever-go/tools"
)

// EncodeContractPermissionsHex encodes contract permissions into a hex string.
func EncodeContractPermissionsHex(contractTypes ...TXContract_ContractType) string {
	return hex.EncodeToString(EncodeContractPermissions(contractTypes...))
}

// EncodeContractPermissions encodes a list of contract types into a byte slice, using bitwise operations to represent permissions.
func EncodeContractPermissions(contractTypes ...TXContract_ContractType) []byte {
	if len(contractTypes) == 0 {
		return make([]byte, 0)
	}

	var value uint64

	for _, contractType := range contractTypes {
		value |= 1 << uint(contractType) // #nosec G115
	}

	return tools.Uint64ToBytesLETruncate(value)
}

// DecodeContractPermissionsHex decodes a hex string into a list of contract types, handling errors if the input is not a valid hex string.
func DecodeContractPermissionsHex(hexPermissions string) ([]TXContract_ContractType, error) {
	permissions, err := hex.DecodeString(hexPermissions)
	if err != nil {
		return nil, err
	}

	return DecodeContractPermissions(permissions), nil
}

// DecodeContractPermissions converts a byte slice representing permissions into a list of contract types by checking each bit.
func DecodeContractPermissions(permissions []byte) []TXContract_ContractType {
	contractTypes := make([]TXContract_ContractType, 0)

	for byteIndex, permByte := range permissions {
		if permByte == 0 {
			continue // Skip bytes that are all zero
		}

		for bitIndex := 0; bitIndex < 8; bitIndex++ {
			if permByte&(1<<bitIndex) != 0 {
				contractType := TXContract_ContractType(byteIndex*8 + bitIndex) // #nosec G115
				contractTypes = append(contractTypes, contractType)
			}
		}
	}

	return contractTypes
}

// getPermissionMask calculates the byte index and bit mask for a given contract type.
func getPermissionMask(contractType uint32) (byteIndex int, bitMask byte) {
	byteIndex = int(contractType / 8)
	bitMask = 1 << (contractType % 8)
	return
}

// CheckPermissionGrantedForContract checks if a specific contract type is granted in the permissions byte slice.
func CheckPermissionGrantedForContract(permissions []byte, contractType TXContract_ContractType) bool {
	byteIndex, bitMask := getPermissionMask(uint32(contractType)) // #nosec G115
	if byteIndex >= len(permissions) {
		return false
	}

	return (permissions[byteIndex] & bitMask) != 0
}

// CheckPermissionGrantedForContracts checks if all specified contract types are granted in the permissions byte slice.
func CheckPermissionGrantedForContracts(permissions []byte, contractTypes ...TXContract_ContractType) bool {
	if len(permissions) == 0 {
		return false
	}

	for _, contractType := range contractTypes {
		if !CheckPermissionGrantedForContract(permissions, contractType) {
			return false
		}
	}

	return true
}

// CheckPermissionGrantedForUint64 checks if all requested operations are permitted by comparing against the permissions byte slice.
func CheckPermissionGrantedForUint64(permissions []byte, ops uint64) bool {
	permissionUint64 := tools.BytesToUint64LE(permissions)

	// Check if all requested operations are permitted
	return (ops & ^permissionUint64) == 0
}
