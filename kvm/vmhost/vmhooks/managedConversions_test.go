package vmhooks

import (
	"encoding/binary"
	"errors"
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/kvm/config"
	mock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/vmcommon"

	"github.com/klever-io/klever-go/kapps"
	hostmock "github.com/klever-io/klever-go/kvm/vmhost/mock"
	"github.com/stretchr/testify/assert"
)

func TestWriteLastClaim(t *testing.T) {
	t.Parallel()

	mockManagedTypes := &hostmock.ManagedTypesContextMock{
		NewBigIntCalled: func(value *big.Int) int32 {
			if value.Int64() == 0 {
				return 0
			}

			return 1
		},
		NewBigIntFromInt64Called: func(value int64) int32 {
			if value == 0 {
				return 0
			}
			return 1
		},
	}

	tests := []struct {
		name      string
		lastClaim *kapps.LastClaim
		expected  []byte
	}{
		{
			name: "Non-nil LastClaim",
			lastClaim: &kapps.LastClaim{
				Timestamp: 1234567890,
				Epoch:     42,
			},
			expected: []byte{0, 0, 0, 1, 0, 0, 0, 42},
		},
		{
			name:      "Nil LastClaim",
			lastClaim: nil,
			expected:  []byte{0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name: "Different values",
			lastClaim: &kapps.LastClaim{
				Timestamp: 987654321,
				Epoch:     255,
			},
			expected: []byte{0, 0, 0, 1, 0, 0, 0, 255},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := writeLastClaim(mockManagedTypes, tt.lastClaim)

			assert.Equal(t, LastClaimLen, len(result), "Result length should be RoyaltiesLen")
			assert.Equal(t, tt.expected, result, "Result should match expected bytes")
		})
	}
}

func TestWriteLastClaimEndianness(t *testing.T) {
	lastClaim := &kapps.LastClaim{
		Timestamp: 1234567890,
		Epoch:     42,
	}

	mockManagedTypes := &hostmock.ManagedTypesContextMock{
		NewBigIntFromInt64Called: func(value int64) int32 {
			return 1
		},
		NewBigIntCalled: func(value *big.Int) int32 {
			return 1
		},
	}

	result := writeLastClaim(mockManagedTypes, lastClaim)

	assert.Equal(t, uint32(1), binary.BigEndian.Uint32(result[0:4]), "Timestamp handle should be in big-endian")
	assert.Equal(t, uint32(42), binary.BigEndian.Uint32(result[4:8]), "Epoch should be in big-endian")
}

func TestReadKDATransfer(t *testing.T) {
	t.Parallel()

	mockManagedTypes := &hostmock.ManagedTypesContextMock{}

	mockError := errors.New("mock error")

	cases := []struct {
		purpose        string
		mock           func()
		data           []byte
		expectedError  error
		expectedResult *vmcommon.KDATransfer
	}{
		{
			purpose: "should return error if GetBytes returns error",
			mock: func() {
				mockManagedTypes = &hostmock.ManagedTypesContextMock{}
				mockManagedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					return []byte(""), mockError
				}
			},
			data:           make([]byte, 16),
			expectedError:  mockError,
			expectedResult: nil,
		},
		{
			purpose: "should return error if GetBigInt returns error",
			mock: func() {
				mockManagedTypes = &hostmock.ManagedTypesContextMock{}
				mockManagedTypes.GetBigIntCalled = func(handle int32) (*big.Int, error) {
					return nil, mockError
				}
			},
			data:           make([]byte, 16),
			expectedError:  mockError,
			expectedResult: nil,
		},
		{
			purpose: "should return error if data len is different than 16",
			mock: func() {
			},
			data:           make([]byte, 12),
			expectedError:  errors.New("invalid KDA transfer object encoding"),
			expectedResult: nil,
		},
		{
			purpose: "should work with valid data and fungible token",
			mock: func() {
				mockManagedTypes = &hostmock.ManagedTypesContextMock{}
				mockManagedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					return []byte("klv-0123"), nil
				}
				mockManagedTypes.GetBigIntCalled = func(handle int32) (*big.Int, error) {
					return big.NewInt(10), nil
				}
			},
			data:          make([]byte, 16),
			expectedError: nil,
			expectedResult: &vmcommon.KDATransfer{
				KDATokenName:  []byte("klv-0123"),
				KDATokenNonce: 0,
				KDATokenType:  0,
				KDAValue:      big.NewInt(10),
			},
		},
		{
			purpose: "should work with valid data and non fungible token",
			mock: func() {
				mockManagedTypes = &hostmock.ManagedTypesContextMock{}
				mockManagedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					return []byte("klv-0123"), nil
				}
				mockManagedTypes.GetBigIntCalled = func(handle int32) (*big.Int, error) {
					return big.NewInt(10), nil
				}
			},
			data:          []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0},
			expectedError: nil,
			expectedResult: &vmcommon.KDATransfer{
				KDATokenName:  []byte("klv-0123"),
				KDATokenNonce: 1,
				KDATokenType:  1,
				KDAValue:      big.NewInt(10),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.purpose, func(t *testing.T) {
			ass := assert.New(t)
			tc.mock()

			res, err := readKDATransfer(mockManagedTypes, tc.data)
			ass.Equal(tc.expectedError, err)
			ass.Equal(tc.expectedResult, res)
		})
	}

}

func TestReadKDATransfers(t *testing.T) {
	t.Parallel()

	mockManagedTypes := &hostmock.ManagedTypesContextMock{}

	mockError := errors.New("mock error")

	counter := 0

	cases := []struct {
		purpose        string
		mock           func()
		expectedError  error
		expectedResult []*vmcommon.KDATransfer
	}{
		{
			purpose: "should return error if GetBytes returns error on managed vector",
			mock: func() {
				mockManagedTypes = &hostmock.ManagedTypesContextMock{}
				mockManagedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					return nil, mockError
				}
			},
			expectedError:  mockError,
			expectedResult: nil,
		},
		{
			purpose: "should return error if GetBytes returns wrong byte size",
			mock: func() {
				mockManagedTypes = &hostmock.ManagedTypesContextMock{}
				mockManagedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					return []byte{0, 0, 0, 0}, nil
				}
			},
			expectedError:  errors.New("invalid managed vector of KDA transfers"),
			expectedResult: nil,
		},
		{
			purpose: "should work with valid data and non fungible token",
			mock: func() {
				mockManagedTypes = &hostmock.ManagedTypesContextMock{}
				counter = 0
				mockManagedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					counter++

					switch counter {
					case 1:
						return []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0}, nil
					case 2:
						return []byte("klv-0123"), nil
					}

					return nil, nil
				}
				mockManagedTypes.GetBigIntCalled = func(handle int32) (*big.Int, error) {
					return big.NewInt(10), nil
				}
			},
			expectedError: nil,
			expectedResult: []*vmcommon.KDATransfer{
				{
					KDATokenName:  []byte("klv-0123"),
					KDATokenNonce: 1,
					KDATokenType:  1,
					KDAValue:      big.NewInt(10),
				},
			},
		},
		{
			purpose: "should return error if readKDATransfer returns error",
			mock: func() {
				mockManagedTypes = &hostmock.ManagedTypesContextMock{}
				counter = 0
				mockManagedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
					counter++

					switch counter {
					case 1:
						return []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0}, nil
					case 2:
						return nil, mockError
					}

					return nil, nil
				}
				mockManagedTypes.GetBigIntCalled = func(handle int32) (*big.Int, error) {
					return big.NewInt(10), nil
				}
			},
			expectedError:  mockError,
			expectedResult: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.purpose, func(t *testing.T) {
			ass := assert.New(t)
			tc.mock()

			res, err := readKDATransfers(mockManagedTypes, 1)
			ass.Equal(tc.expectedError, err)

			for i := range tc.expectedResult {
				ass.Equal(tc.expectedResult[i], res[i])
			}
		})
	}

}

func TestWriteKDATransfers(t *testing.T) {
	t.Parallel()

	tokenName := "klv-0123"

	mockManagedTypes := &hostmock.ManagedTypesContextMock{
		NewManagedBufferFromBytesCalled: func(bytes []byte) int32 {
			if string(bytes) != tokenName {
				return 0
			}

			return 1000
		},
		NewBigIntCalled: func(value *big.Int) int32 {
			if value.Int64() != 10 {
				return 0
			}

			return 1001
		},
	}

	dest := make([]byte, 16)

	writeKDATransfer(mockManagedTypes, &vmcommon.KDATransfer{
		KDATokenName:  []byte(tokenName),
		KDATokenNonce: 0,
		KDAValue:      big.NewInt(10),
	}, dest)

	assert.Equal(t, []byte{0, 0, 3, 232, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3, 233}, dest)
}

func TestWriteSplitRoyalties(t *testing.T) {
	t.Parallel()

	key := "key"

	mockManagedTypes := &hostmock.ManagedTypesContextMock{
		NewManagedBufferFromBytesCalled: func(bytes []byte) int32 {
			if string(bytes) != key {
				return 0
			}

			return 1
		},
	}

	dest := make([]byte, 28)

	writeSplitRoyalties(mockManagedTypes, key, &kapps.RoyaltySplitData{
		PercentTransferPercentage: 1,
		PercentTransferFixed:      2,
		PercentMarketPercentage:   3,
		PercentMarketFixed:        4,
		PercentITOPercentage:      5,
		PercentITOFixed:           6,
	}, dest)

	assert.Equal(t, dest[4:8], []byte{0, 0, 0, 1})
	assert.Equal(t, dest[8:12], []byte{0, 0, 0, 2})
	assert.Equal(t, dest[12:16], []byte{0, 0, 0, 3})
	assert.Equal(t, dest[16:20], []byte{0, 0, 0, 4})
	assert.Equal(t, dest[20:24], []byte{0, 0, 0, 5})
	assert.Equal(t, dest[24:28], []byte{0, 0, 0, 6})
}

func TestWriteSplitRoyaltiesToBytes(t *testing.T) {
	t.Parallel()

	counter := 0
	mockManagedTypes := &hostmock.ManagedTypesContextMock{
		NewManagedBufferFromBytesCalled: func(bytes []byte) int32 {
			if counter == 0 {
				if string(bytes) != "key1" {
					return 0
				}

				counter++

				return 1
			}

			if string(bytes) != "key2" {
				return 0
			}

			counter++

			return 8
		},
	}

	dest := writeSplitRoyaltiesToBytes(mockManagedTypes, map[string]*kapps.RoyaltySplitData{
		"key1": {
			PercentTransferPercentage: 2,
			PercentTransferFixed:      3,
			PercentMarketPercentage:   4,
			PercentMarketFixed:        5,
			PercentITOPercentage:      6,
			PercentITOFixed:           7,
		},
		"key2": {
			PercentTransferPercentage: 9,
			PercentTransferFixed:      10,
			PercentMarketPercentage:   11,
			PercentMarketFixed:        12,
			PercentITOPercentage:      13,
			PercentITOFixed:           14,
		},
	})

	assert.Equal(t, []byte{0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0, 4, 0, 0, 0, 5, 0, 0, 0, 6, 0, 0, 0, 7, 0, 0, 0, 8, 0, 0, 0, 9, 0, 0, 0, 10, 0, 0, 0, 11, 0, 0, 0, 12, 0, 0, 0, 13, 0, 0, 0, 14}, dest)
}

func TestWriteTransferPercentages(t *testing.T) {
	t.Parallel()

	dest := make([]byte, 8)

	managedTypes := &hostmock.ManagedTypesContextMock{
		NewBigIntCalled: func(value *big.Int) int32 {
			if value.Int64() != 1 {
				return 0
			}

			return 1
		},
	}

	writeTransferPercentages(managedTypes, &kapps.RoyaltyData{
		Amount:     1,
		Percentage: 2,
	}, dest)

	assert.Equal(t, []byte{0, 0, 0, 1, 0, 0, 0, 2}, dest)
}

func TestWriteTransferPercentagesToBytes(t *testing.T) {
	t.Parallel()

	counter := 0

	managedTypes := &hostmock.ManagedTypesContextMock{
		NewBigIntCalled: func(value *big.Int) int32 {
			if counter == 0 {
				if value.Int64() != 1 {
					return 0
				}

				counter++

				return 1
			}

			if value.Int64() != 3 {
				return 0
			}

			return 3
		},
	}

	dest := writeTransferPercentagesToBytes(managedTypes, []*kapps.RoyaltyData{
		{
			Amount:     1,
			Percentage: 2,
		},
		{
			Amount:     3,
			Percentage: 4,
		},
	})

	assert.Equal(t, []byte{0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0, 4}, dest)
}

func TestWriteUserBuckets(t *testing.T) {
	t.Parallel()

	counter := int32(0)

	mockManagedTypes := &hostmock.ManagedTypesContextMock{
		NewManagedBufferFromBytesCalled: func(bytes []byte) int32 {
			counter++
			switch counter {
			case 1:
				return 1
			case 2:
				return 6
			case 3:
				return 7
			case 4:
				return 12
			}

			return 0
		},
		NewBigIntCalled: func(value *big.Int) int32 {
			return int32(value.Int64()) // #nosec G115
		},
		NewBigIntFromInt64Called: func(value int64) int32 {
			return int32(value) // #nosec G115
		},
	}

	dest := writeUserBuckets(mockManagedTypes, map[string]*kapps.UserBucket{
		"key1": {
			StakedAt:      2,
			StakedEpoch:   3,
			UnstakedEpoch: 4,
			Value:         5,
			Delegation:    []byte("6"),
		},
		"key2": {
			StakedAt:      8,
			StakedEpoch:   9,
			UnstakedEpoch: 10,
			Value:         11,
			Delegation:    []byte("12"),
		},
	})

	assert.Equal(t, []byte{0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0, 4, 0, 0, 0, 5, 0, 0, 0, 6, 0, 0, 0, 7, 0, 0, 0, 8, 0, 0, 0, 9, 0, 0, 0, 10, 0, 0, 0, 11, 0, 0, 0, 12}, dest)
}

func TestWriteRoyaltiesToBytes(t *testing.T) {
	t.Parallel()

	counter := int32(0)

	mockManagedTypes := &hostmock.ManagedTypesContextMock{
		NewManagedBufferFromBytesCalled: func(bytes []byte) int32 {
			counter++

			switch counter {
			case 1:
				return 1
			case 2:
				return 2
			case 4:
				return 6
			default:
				return 0
			}
		},
		NewBigIntFromInt64Called: func(value int64) int32 {
			return int32(value) // #nosec G115
		},
		NewBigIntCalled: func(value *big.Int) int32 {
			return int32(value.Int64()) // #nosec G115
		},
	}

	dest := writeRoyaltiesToBytes(mockManagedTypes, &kapps.RoyaltiesData{
		Address: []byte("1"),
		TransferPercentage: []*kapps.RoyaltyData{{
			Amount: 2,
		}},
		TransferFixed:    3,
		MarketPercentage: 4,
		MarketFixed:      5,
		SplitRoyalties: map[string]*kapps.RoyaltySplitData{
			"6": {PercentTransferFixed: 6},
		},
		ITOFixed:      7,
		ITOPercentage: 8,
	})

	assert.Equal(t, []byte{0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0, 4, 0, 0, 0, 5, 0, 0, 0, 6, 0, 0, 0, 7, 0, 0, 0, 8}, dest)
}

func TestWriteRoyaltiesToBytes_NilRoyalties(t *testing.T) {
	t.Parallel()

	mockManagedTypes := &hostmock.ManagedTypesContextMock{}

	dest := writeRoyaltiesToBytes(mockManagedTypes, nil)

	assert.Equal(t, []byte{}, dest)
}

func TestEncodeBool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value bool
		index int
		want  uint32
	}{
		{
			name:  "Should encode true",
			value: true,
			index: 0,
			want:  1,
		},
		{
			name:  "Should encode false",
			value: false,
			index: 0,
			want:  0,
		},
		{
			name:  "Should encode with index 1 and true",
			value: true,
			index: 1,
			want:  2,
		},
		{
			name:  "Should encode with index 1 and false",
			value: false,
			index: 1,
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			as := assert.New(t)

			got := encodeBool(tt.value, tt.index)
			as.Equal(tt.want, got)
		})
	}
}

func TestGetPropertiesValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		prop      *kapps.PropertiesData
		tokenType int32
		want      uint32
	}{
		{
			name:      "Should return all true - fungible",
			tokenType: int32(kapps.KDAData_Fungible),
			prop: &kapps.PropertiesData{
				CanFreeze:      true,
				CanWipe:        true,
				CanPause:       true,
				CanMint:        true,
				CanBurn:        true,
				CanChangeOwner: true,
				CanAddRoles:    true,
				LimitTransfer:  true,
			},
			want: 255,
		},
		{
			name:      "Should return only one false - fungible",
			tokenType: int32(kapps.KDAData_Fungible),
			prop: &kapps.PropertiesData{
				CanFreeze:      false,
				CanWipe:        true,
				CanPause:       true,
				CanMint:        true,
				CanBurn:        true,
				CanChangeOwner: true,
				CanAddRoles:    true,
				LimitTransfer:  true,
			},
			want: 254,
		},
		{
			name:      "Should return all false - fungible",
			tokenType: int32(kapps.KDAData_Fungible),
			prop: &kapps.PropertiesData{
				CanFreeze:      false,
				CanWipe:        false,
				CanPause:       false,
				CanMint:        false,
				CanBurn:        false,
				CanChangeOwner: false,
				CanAddRoles:    false,
				LimitTransfer:  false,
			},
			want: 0,
		},
		{
			name:      "Should return all true - non_fungible",
			tokenType: int32(kapps.KDAData_NonFungible),
			prop: &kapps.PropertiesData{
				CanFreeze:      true,
				CanWipe:        true,
				CanPause:       true,
				CanMint:        true,
				CanBurn:        true,
				CanChangeOwner: true,
				CanAddRoles:    true,
				LimitTransfer:  true,
			},
			want: 1073741824 + 255,
		},
		{
			name:      "Should return only one false - non_fungible",
			tokenType: int32(kapps.KDAData_NonFungible),
			prop: &kapps.PropertiesData{
				CanFreeze:      false,
				CanWipe:        true,
				CanPause:       true,
				CanMint:        true,
				CanBurn:        true,
				CanChangeOwner: true,
				CanAddRoles:    true,
				LimitTransfer:  true,
			},
			want: 1073741824 + 254,
		},
		{
			name:      "Should return all false - non_fungible",
			tokenType: int32(kapps.KDAData_NonFungible),
			prop: &kapps.PropertiesData{
				CanFreeze:      false,
				CanWipe:        false,
				CanPause:       false,
				CanMint:        false,
				CanBurn:        false,
				CanChangeOwner: false,
				CanAddRoles:    false,
				LimitTransfer:  false,
			},
			want: 1073741824,
		},
		{
			name:      "Should return all true - semi_fungible",
			tokenType: int32(kapps.KDAData_SemiFungible),
			prop: &kapps.PropertiesData{
				CanFreeze:      true,
				CanWipe:        true,
				CanPause:       true,
				CanMint:        true,
				CanBurn:        true,
				CanChangeOwner: true,
				CanAddRoles:    true,
				LimitTransfer:  true,
			},
			want: 2147483648 + 255,
		},
		{
			name:      "Should return only one false - semi_fungible",
			tokenType: int32(kapps.KDAData_SemiFungible),
			prop: &kapps.PropertiesData{
				CanFreeze:      false,
				CanWipe:        true,
				CanPause:       true,
				CanMint:        true,
				CanBurn:        true,
				CanChangeOwner: true,
				CanAddRoles:    true,
				LimitTransfer:  true,
			},
			want: 2147483648 + 254,
		},
		{
			name:      "Should return all false - semi_fungible",
			tokenType: int32(kapps.KDAData_SemiFungible),
			prop: &kapps.PropertiesData{
				CanFreeze:      false,
				CanWipe:        false,
				CanPause:       false,
				CanMint:        false,
				CanBurn:        false,
				CanChangeOwner: false,
				CanAddRoles:    false,
				LimitTransfer:  false,
			},
			want: 2147483648,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			as := assert.New(t)
			got := getPropertiesValue(tt.prop, tt.tokenType)
			as.Equal(tt.want, got)
		})
	}
}

func TestGetAttributesValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		prop *kapps.AttributesData
		want uint32
	}{
		{
			name: "Should return all true",
			prop: &kapps.AttributesData{
				IsPaused:                   true,
				IsNFTMintStopped:           true,
				IsRoyaltiesChangeStopped:   true,
				IsNFTMetadataChangeStopped: true,
			},
			want: 15,
		},
		{
			name: "Should return only one false",
			prop: &kapps.AttributesData{
				IsPaused:                   true,
				IsNFTMintStopped:           false,
				IsRoyaltiesChangeStopped:   true,
				IsNFTMetadataChangeStopped: true,
			},
			want: 13,
		},
		{
			name: "Should return all false",
			prop: &kapps.AttributesData{
				IsPaused:                   false,
				IsNFTMintStopped:           false,
				IsRoyaltiesChangeStopped:   false,
				IsNFTMetadataChangeStopped: false,
			},
			want: 0,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			as := assert.New(t)
			got := getAttributesValue(tt.prop)
			as.Equal(tt.want, got)
		})
	}
}

func TestWriteRoles(t *testing.T) {
	dest := make([]byte, 8)
	managedTypeMock := &hostmock.ManagedTypesContextMock{
		NewManagedBufferFromBytesCalled: func(bytes []byte) int32 {
			return 1
		},
	}

	writeRoles(managedTypeMock, &kapps.RolesData{
		Address:             []byte("addr"),
		HasRoleMint:         true,
		HasRoleSetITOPrices: true,
		HasRoleDeposit:      true,
		HasRoleTransfer:     false,
	}, dest)

	assert.Equal(t, []byte{0, 0, 0, 1, 0, 0, 0, 7}, dest)
}

func TestWriteRolesToBytes(t *testing.T) {
	t.Parallel()

	counter := int32(0)

	mockManagedTypes := &hostmock.ManagedTypesContextMock{
		NewManagedBufferFromBytesCalled: func(bytes []byte) int32 {
			counter++
			return counter
		},
	}

	dest := writeRolesToBytes(mockManagedTypes, []*kapps.RolesData{
		{
			Address:             []byte("addr1"),
			HasRoleMint:         true,
			HasRoleSetITOPrices: true,
			HasRoleDeposit:      true,
			HasRoleTransfer:     true,
		},
		{
			Address:             []byte("addr2"),
			HasRoleMint:         false,
			HasRoleSetITOPrices: false,
			HasRoleDeposit:      false,
			HasRoleTransfer:     false,
		},
	})
	assert.Equal(t, []byte{0, 0, 0, 1, 0, 0, 0, 15, 0, 0, 0, 2, 0, 0, 0, 0}, dest)
}

func TestWriteURIs(t *testing.T) {
	t.Parallel()

	counter := int32(0)
	dest := make([]byte, 8)
	managedTypeMock := &hostmock.ManagedTypesContextMock{
		NewManagedBufferFromBytesCalled: func(bytes []byte) int32 {
			counter++
			return counter
		},
	}

	writeURIs(managedTypeMock, "key", "value", dest)

	assert.Equal(t, []byte{0, 0, 0, 1, 0, 0, 0, 2}, dest)
}

func TestWriteURIsToBytes(t *testing.T) {
	t.Parallel()

	counter := int32(0)
	managedTypeMock := &hostmock.ManagedTypesContextMock{
		NewManagedBufferFromBytesCalled: func(bytes []byte) int32 {
			counter++
			return counter
		},
	}

	dest := writeURIsToBytes(managedTypeMock, map[string]string{
		"key":  "value",
		"key2": "value2",
	})

	assert.Equal(t, []byte{0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0, 4}, dest)
}

func TestWriteSFTMeta(t *testing.T) {
	t.Parallel()

	managedTypeMock := &hostmock.ManagedTypesContextMock{
		NewManagedBufferFromBytesCalled: func(bytes []byte) int32 {
			return 3
		},
		NewBigIntFromInt64Called: func(value int64) int32 {
			return int32(value) // #nosec G115
		},
		NewBigIntCalled: func(value *big.Int) int32 {
			return int32(value.Int64()) // #nosec G115
		},
	}

	dest := writeSFTMeta(managedTypeMock, &kapps.MetaV2{
		MaxSupply:   1,
		Circulation: 2,
		Metadata: &kapps.MetaV2Data{
			Hash:       []byte("hash"),
			Attributes: []byte("attr"),
			Name:       []byte("name"),
		},
	})

	assert.Equal(t, []byte{0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3}, dest)
}

func TestWriteSFTMetadata(t *testing.T) {
	t.Parallel()

	counter := int32(0)

	managedTypeMock := &hostmock.ManagedTypesContextMock{
		NewManagedBufferFromBytesCalled: func(bytes []byte) int32 {
			counter++
			return counter
		},
	}

	dest := writeSFTMetadata(managedTypeMock, &kapps.MetaV2Data{
		Hash:       []byte("hash"),
		Attributes: []byte("attr"),
		Name:       []byte("name"),
	})

	assert.Equal(t, []byte{0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3}, dest)
}

func TestWriteKDATransfersToBytes(t *testing.T) {
	t.Parallel()

	counter := int32(0)
	managedTypeMock := &hostmock.ManagedTypesContextMock{
		NewManagedBufferFromBytesCalled: func(bytes []byte) int32 {
			counter++
			return counter
		},
		NewBigIntCalled: func(value *big.Int) int32 {
			return int32(value.Int64()) // #nosec G115
		},
	}

	dest := writeKDATransfersToBytes(managedTypeMock, []*vmcommon.KDATransfer{
		{
			KDATokenName:  []byte("token"),
			KDATokenNonce: 2,
			KDAValue:      big.NewInt(3),
		},
		{
			KDATokenName:  []byte("token2"),
			KDATokenNonce: 4,
			KDAValue:      big.NewInt(5),
		},
	})

	assert.Equal(t, []byte{0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0, 0, 5}, dest)
}

func TestReadDestinationArguments(t *testing.T) {
	t.Parallel()

	mockManagedTypes := &hostmock.ManagedTypesContextMock{
		GetBytesCalled: func(handle int32) ([]byte, error) {
			return []byte("destination"), nil
		},
		ReadManagedVecOfManagedBuffersCalled: func(managedVecHandle int32) ([][]byte, uint64, error) {
			return [][]byte{[]byte("arguments")}, 1, nil
		},
	}
	host := &mock.VMHostMock{
		ManagedTypesContext: mockManagedTypes,
		MeteringContext: &mock.MeteringContextMock{
			GasCost: &config.GasCost{},
		},
	}

	input, err := readDestinationArguments(host, 1, 2)
	assert.Nil(t, err)
	assert.Equal(t, []byte("destination"), input.destination)
	assert.Equal(t, [][]byte{[]byte("arguments")}, input.arguments)

	mockError := errors.New("mock error")

	mockManagedTypes.ReadManagedVecOfManagedBuffersCalled = func(managedVecHandle int32) ([][]byte, uint64, error) {
		return nil, 0, mockError
	}

	input, err = readDestinationArguments(host, 1, 2)
	assert.Equal(t, mockError, err)
	assert.Nil(t, input)

	mockManagedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
		return nil, mockError
	}

	input, err = readDestinationArguments(host, 1, 2)
	assert.Equal(t, mockError, err)
	assert.Nil(t, input)

}

func TestReadDestinationFunctionArguments(t *testing.T) {
	t.Parallel()
	mockManagedTypes := &hostmock.ManagedTypesContextMock{
		GetBytesCalled: func(handle int32) ([]byte, error) {
			if handle == 1 {
				return []byte("destination"), nil
			}

			return []byte("function"), nil
		},
		ReadManagedVecOfManagedBuffersCalled: func(managedVecHandle int32) ([][]byte, uint64, error) {
			return [][]byte{[]byte("arguments")}, 1, nil
		},
	}
	host := &mock.VMHostMock{
		ManagedTypesContext: mockManagedTypes,
		MeteringContext: &mock.MeteringContextMock{
			GasCost: &config.GasCost{},
		},
	}

	input, err := readDestinationFunctionArguments(host, 1, 2, 3)
	assert.Nil(t, err)
	assert.Equal(t, []byte("destination"), input.destination)
	assert.Equal(t, [][]byte{[]byte("arguments")}, input.arguments)
	assert.Equal(t, "function", input.function)

	mockError := errors.New("mock error")

	mockManagedTypes.ReadManagedVecOfManagedBuffersCalled = func(managedVecHandle int32) ([][]byte, uint64, error) {
		return nil, 0, mockError
	}

	input, err = readDestinationFunctionArguments(host, 1, 2, 3)
	assert.Equal(t, mockError, err)
	assert.Nil(t, input)

	mockManagedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
		if handle == 1 {
			return []byte("destination"), nil
		}

		return nil, mockError
	}
	mockManagedTypes.ReadManagedVecOfManagedBuffersCalled = func(managedVecHandle int32) ([][]byte, uint64, error) {
		return [][]byte{[]byte("arguments")}, 1, nil
	}

	input, err = readDestinationFunctionArguments(host, 1, 2, 3)
	assert.Equal(t, mockError, err)
	assert.Nil(t, input)
}

func TestReadDestinationValueArguments(t *testing.T) {
	t.Parallel()
	mockManagedTypes := &hostmock.ManagedTypesContextMock{
		GetBytesCalled: func(handle int32) ([]byte, error) {
			return []byte("destination"), nil
		},
		GetBigIntCalled: func(handle int32) (*big.Int, error) {
			return big.NewInt(10), nil
		},
		ReadManagedVecOfManagedBuffersCalled: func(managedVecHandle int32) ([][]byte, uint64, error) {
			return [][]byte{[]byte("arguments")}, 1, nil
		},
	}

	host := &mock.VMHostMock{
		ManagedTypesContext: mockManagedTypes,
		MeteringContext: &mock.MeteringContextMock{
			GasCost: &config.GasCost{},
		},
	}

	input, err := readDestinationValueArguments(host, 1, 2, 3)
	assert.Nil(t, err)
	assert.Equal(t, []byte("destination"), input.destination)
	assert.Equal(t, [][]byte{[]byte("arguments")}, input.arguments)
	assert.Equal(t, int64(10), input.value.Int64())

	mockError := errors.New("mock error")

	mockManagedTypes.ReadManagedVecOfManagedBuffersCalled = func(managedVecHandle int32) ([][]byte, uint64, error) {
		return nil, 0, mockError
	}

	input, err = readDestinationValueArguments(host, 1, 2, 3)
	assert.Equal(t, mockError, err)
	assert.Nil(t, input)

	mockManagedTypes.GetBigIntCalled = func(handle int32) (*big.Int, error) {
		return big.NewInt(0), mockError
	}
	mockManagedTypes.ReadManagedVecOfManagedBuffersCalled = func(managedVecHandle int32) ([][]byte, uint64, error) {
		return [][]byte{[]byte("arguments")}, 1, nil
	}

	input, err = readDestinationValueArguments(host, 1, 2, 3)
	assert.Equal(t, mockError, err)
	assert.Nil(t, input)
}

func TestReadDestinationValueFunctionArguments(t *testing.T) {
	t.Parallel()
	mockManagedTypes := &hostmock.ManagedTypesContextMock{
		GetBytesCalled: func(handle int32) ([]byte, error) {
			if handle == 1 {
				return []byte("destination"), nil
			}

			return []byte("function"), nil
		},
		GetBigIntCalled: func(handle int32) (*big.Int, error) {
			return big.NewInt(10), nil
		},
		ReadManagedVecOfManagedBuffersCalled: func(managedVecHandle int32) ([][]byte, uint64, error) {
			return [][]byte{[]byte("arguments")}, 1, nil
		},
	}
	host := &mock.VMHostMock{
		ManagedTypesContext: mockManagedTypes,
		MeteringContext: &mock.MeteringContextMock{
			GasCost: &config.GasCost{},
		},
	}

	input, err := readDestinationValueFunctionArguments(host, 1, 2, 3, 4)
	assert.Nil(t, err)
	assert.Equal(t, []byte("destination"), input.destination)
	assert.Equal(t, [][]byte{[]byte("arguments")}, input.arguments)
	assert.Equal(t, "function", input.function)
	assert.Equal(t, int64(10), input.value.Int64())

	mockError := errors.New("mock error")

	mockManagedTypes.ReadManagedVecOfManagedBuffersCalled = func(managedVecHandle int32) ([][]byte, uint64, error) {
		return nil, 0, mockError
	}

	input, err = readDestinationValueFunctionArguments(host, 1, 2, 3, 4)
	assert.Equal(t, mockError, err)
	assert.Nil(t, input)

	mockManagedTypes.GetBytesCalled = func(handle int32) ([]byte, error) {
		if handle == 1 {
			return []byte("destination"), nil
		}

		return nil, mockError
	}
	mockManagedTypes.ReadManagedVecOfManagedBuffersCalled = func(managedVecHandle int32) ([][]byte, uint64, error) {
		return [][]byte{[]byte("arguments")}, 1, nil
	}

	input, err = readDestinationValueFunctionArguments(host, 1, 2, 3, 4)
	assert.Equal(t, mockError, err)
	assert.Nil(t, input)
}
