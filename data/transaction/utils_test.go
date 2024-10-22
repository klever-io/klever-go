package transaction_test

import (
	"encoding/hex"
	"testing"

	dt "github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/assert"
)

func TestContractPermissions(t *testing.T) {
	tests := []struct {
		name           string
		contractTypes  []dt.TXContract_ContractType
		expectedHex    string
		expectedBytes  []byte
		expectedDecode []dt.TXContract_ContractType
	}{
		{
			name:           "Empty",
			contractTypes:  []dt.TXContract_ContractType{},
			expectedHex:    "",
			expectedBytes:  []byte{},
			expectedDecode: []dt.TXContract_ContractType{},
		},
		{
			name:           "Single permission",
			contractTypes:  []dt.TXContract_ContractType{dt.TXContract_TransferContractType},
			expectedHex:    "01",
			expectedBytes:  []byte{0x01},
			expectedDecode: []dt.TXContract_ContractType{dt.TXContract_TransferContractType},
		},
		{
			name:           "Multiple permissions in single byte",
			contractTypes:  []dt.TXContract_ContractType{dt.TXContract_TransferContractType, dt.TXContract_CreateAssetContractType, dt.TXContract_CreateValidatorContractType},
			expectedHex:    "07",
			expectedBytes:  []byte{0x07},
			expectedDecode: []dt.TXContract_ContractType{dt.TXContract_TransferContractType, dt.TXContract_CreateAssetContractType, dt.TXContract_CreateValidatorContractType},
		},
		{
			name:           "Permissions across multiple bytes",
			contractTypes:  []dt.TXContract_ContractType{dt.TXContract_TransferContractType, dt.TXContract_WithdrawContractType},
			expectedHex:    "0101",
			expectedBytes:  []byte{0x01, 0x01},
			expectedDecode: []dt.TXContract_ContractType{dt.TXContract_TransferContractType, dt.TXContract_WithdrawContractType},
		},
		{
			name:           "High value permission",
			contractTypes:  []dt.TXContract_ContractType{dt.TXContract_ContractType(31)},
			expectedHex:    "00000080",
			expectedBytes:  []byte{0x00, 0x00, 0x00, 0x80},
			expectedDecode: []dt.TXContract_ContractType{dt.TXContract_ContractType(31)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test EncodeContractPermissionsHex
			gotHex := dt.EncodeContractPermissionsHex(tt.contractTypes...)
			assert.Equal(t, tt.expectedHex, gotHex, "EncodeContractPermissionsHex() = %v, want %v", gotHex, tt.expectedHex)

			// Test EncodeContractPermissions
			gotBytes := dt.EncodeContractPermissions(tt.contractTypes...)
			assert.Equal(t, tt.expectedBytes, gotBytes, "EncodeContractPermissions() = %v, want %v", gotBytes, tt.expectedBytes)

			// Test DecodeContractPermissionsHex
			gotDecode, err := dt.DecodeContractPermissionsHex(tt.expectedHex)
			assert.Nil(t, err, "DecodeContractPermissionsHex() error = %v", err)
			assert.Equal(t, tt.expectedDecode, gotDecode, "DecodeContractPermissionsHex() = %v, want %v", gotDecode, tt.expectedDecode)

			// Test DecodeContractPermissions
			gotDecode = dt.DecodeContractPermissions(tt.expectedBytes)
			assert.Equal(t, tt.expectedDecode, gotDecode, "DecodeContractPermissions() = %v, want %v", gotDecode, tt.expectedDecode)
		})
	}
}

func TestCheckPermissionGranted(t *testing.T) {
	permissions := dt.EncodeContractPermissions(dt.TXContract_TransferContractType, dt.TXContract_CreateAssetContractType, dt.TXContract_WithdrawContractType)

	tests := []struct {
		name         string
		contractType dt.TXContract_ContractType
		expected     bool
	}{
		{"Granted permission", dt.TXContract_TransferContractType, true},
		{"Granted permission (high value)", dt.TXContract_WithdrawContractType, true},
		{"Not granted permission", dt.TXContract_ValidatorConfigContractType, false},
		{"Out of range permission", dt.TXContract_ContractType(100), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dt.CheckPermissionGrantedForContract(permissions, tt.contractType)
			assert.Equal(t, tt.expected, got, "CheckPermissionGranted() = %v, want %v", got, tt.expected)
		})
	}
}

func TestCheckPermissionsGranted(t *testing.T) {
	permissions := dt.EncodeContractPermissions(dt.TXContract_TransferContractType, dt.TXContract_CreateAssetContractType, dt.TXContract_WithdrawContractType)

	tests := []struct {
		name          string
		contractTypes []dt.TXContract_ContractType
		expected      bool
	}{
		{"All granted", []dt.TXContract_ContractType{dt.TXContract_TransferContractType, dt.TXContract_CreateAssetContractType}, true},
		{"One not granted", []dt.TXContract_ContractType{dt.TXContract_TransferContractType, dt.TXContract_ValidatorConfigContractType}, false},
		{"Empty list", []dt.TXContract_ContractType{}, true},
		{"Out of range permission", []dt.TXContract_ContractType{dt.TXContract_TransferContractType, dt.TXContract_ContractType(100)}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dt.CheckPermissionGrantedForContracts(permissions, tt.contractTypes...)
			assert.Equal(t, tt.expected, got, "CheckPermissionsGranted() = %v, want %v", got, tt.expected)
		})
	}

	// Test with empty permissions
	for _, c := range dt.TXContract_ContractType_value {
		result := dt.CheckPermissionGrantedForContracts([]byte{}, dt.TXContract_ContractType(c))
		assert.False(t, result, "CheckPermissionsGranted() with empty permissions should return false")
	}
}

func TestDecodeContractPermissionsHexError(t *testing.T) {
	_, err := dt.DecodeContractPermissionsHex("invalid hex")
	assert.NotNil(t, err, "DecodeContractPermissionsHex() with invalid hex should return an error")
}

func TestDocsExample(t *testing.T) {
	examplePermissions := []dt.TXContract_ContractType{
		dt.TXContract_TransferContractType,
		dt.TXContract_CreateAssetContractType,
		dt.TXContract_CreateValidatorContractType,
		dt.TXContract_ValidatorConfigContractType,
		dt.TXContract_UnfreezeContractType,
		dt.TXContract_UndelegateContractType,
		dt.TXContract_WithdrawContractType,
	}

	value := dt.EncodeContractPermissions(examplePermissions...)
	assert.Equal(t, []byte{0xaf, 0x01}, value)

	valueHex := dt.EncodeContractPermissionsHex(examplePermissions...)
	assert.Equal(t, "af01", valueHex)

	tx := &dt.Transaction{
		RawData: &dt.Transaction_Raw{
			Contract: make([]*dt.TXContract, 0),
		},
	}

	for _, ct := range examplePermissions {
		tx.RawData.Contract = append(tx.RawData.Contract, &dt.TXContract{Type: ct})
	}

	opByte, _ := hex.DecodeString("af01")
	err := tx.ValidatePermissionOperation(opByte)
	assert.Nil(t, err)
}

func TestCheckPermissionGrantedForUint64(t *testing.T) {
	tests := []struct {
		name        string
		permissions []byte
		ops         uint64
		expected    bool
	}{
		{
			name:        "All permissions granted",
			permissions: []byte{255, 255, 255, 255, 255, 255, 255, 255}, // All bits set
			ops:         0xFFFFFFFFFFFFFFFF,                             // All operations requested
			expected:    true,
		},
		{
			name:        "No permissions granted",
			permissions: []byte{0, 0, 0, 0, 0, 0, 0, 0}, // No bits set
			ops:         0xFFFFFFFFFFFFFFFF,             // All operations requested
			expected:    false,
		},
		{
			name:        "Some permissions granted",
			permissions: []byte{255, 0, 0, 0, 0, 0, 0, 0}, // First byte all set
			ops:         0xFF,                             // First byte operations requested
			expected:    true,
		},
		{
			name:        "Partial permissions",
			permissions: []byte{170, 170, 170, 170, 170, 170, 170, 170}, // Alternating bits
			ops:         0xAAAAAAAAAAAAAAAA,                             // Matching alternating bits
			expected:    true,
		},
		{
			name:        "Partial permissions - fail",
			permissions: []byte{170, 170, 170, 170, 170, 170, 170, 170}, // Alternating bits
			ops:         0xFFFFFFFFFFFFFFFF,                             // All operations requested
			expected:    false,
		},
		{
			name:        "Empty permissions",
			permissions: []byte{},
			ops:         1,
			expected:    false,
		},
		{
			name:        "No operations requested",
			permissions: []byte{255, 255, 255, 255, 255, 255, 255, 255},
			ops:         0,
			expected:    true,
		},
		{
			name:        "Single bit permission",
			permissions: []byte{1, 0, 0, 0, 0, 0, 0, 0},
			ops:         1,
			expected:    true,
		},
		{
			name:        "Single bit permission - fail",
			permissions: []byte{1, 0, 0, 0, 0, 0, 0, 0},
			ops:         2,
			expected:    false,
		},
		{
			name:        "High bit permission",
			permissions: []byte{0, 0, 0, 0, 0, 0, 0, 128},
			ops:         1 << 63,
			expected:    true,
		},
		{
			name:        "Mixed permissions",
			permissions: []byte{15, 0, 240, 0, 15, 0, 240, 0},
			ops:         0xF0000F00F0000F,
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dt.CheckPermissionGrantedForUint64(tt.permissions, tt.ops)
			assert.Equal(t, tt.expected, result, "CheckPermissionGrantedForUint64() = %v, want %v", result, tt.expected)
		})
	}
}

func TestCheckPermissionGrantedForUint64_DifferentLengths(t *testing.T) {
	tests := []struct {
		name        string
		permissions []byte
		ops         uint64
		expected    bool
	}{
		{
			name:        "Permissions shorter than 8 bytes",
			permissions: []byte{255, 255, 255, 255},
			ops:         0xFFFFFFFF,
			expected:    true,
		},
		{
			name:        "Permissions shorter than 8 bytes - fail high bits",
			permissions: []byte{255, 255, 255, 255},
			ops:         0xFFFFFFFF00000000,
			expected:    false,
		},
		{
			name:        "Permissions longer than 8 bytes",
			permissions: []byte{255, 255, 255, 255, 255, 255, 255, 255, 255},
			ops:         0xFFFFFFFFFFFFFFFF,
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dt.CheckPermissionGrantedForUint64(tt.permissions, tt.ops)
			assert.Equal(t, tt.expected, result)
		})
	}
}
