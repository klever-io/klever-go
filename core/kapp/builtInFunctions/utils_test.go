package builtInFunctions

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeITOPacks(t *testing.T) {
	createPack := func(name string, items ...transaction.PackItem) []byte {
		buf := new(bytes.Buffer)
		_ = writeString(buf, name)
		_ = writeUint32(buf, uint32(len(items)))
		for i := range items {
			_ = writeInt64AsBiInt(buf, items[i].Amount)
			_ = writeInt64AsBiInt(buf, items[i].Price)
		}
		return buf.Bytes()
	}

	tests := []struct {
		name        string
		input       []byte
		expected    map[string]*transaction.PackInfo
		expectedErr string
	}{
		{
			name: "Success",
			input: append([]byte{0, 0, 0, 1}, createPack("KLV",
				transaction.PackItem{Amount: 100, Price: 10},
				transaction.PackItem{Amount: 200, Price: 20})...),
			expected: map[string]*transaction.PackInfo{
				"KLV": {
					Packs: []*transaction.PackItem{
						{Amount: 100, Price: 10},
						{Amount: 200, Price: 20},
					},
				},
			},
		},
		{
			name:        "Error - Invalid Length",
			input:       []byte{0, 0, 0},
			expectedErr: "unexpected EOF",
		},
		{
			name: "Error - Max Packs exceeded",
			input: []byte{
				0, 0, 0, 11, // Invalid length
				0, 0, 0, 3, 'K', 'L', 'V',
			},
			expectedErr: common.ErrMaxBytesExceeded.Error(),
		},
		{
			name: "Error - Invalid token length",
			input: []byte{
				0, 0, 0, 1,
				0, 0, 0, 31, // Invalid length
			},
			expectedErr: common.ErrMaxBytesExceeded.Error(),
		},
		{
			name: "Error - Invalid item length",
			input: func() []byte {
				items := make([]transaction.PackItem, core.MaxPackItems+1)
				for i := range items {
					items[i] = transaction.PackItem{Amount: int64(100 * i), Price: 10}
				}
				return append([]byte{0, 0, 0, 1}, createPack("KLV", items...)...)
			}(),
			expectedErr: common.ErrMaxBytesExceeded.Error(),
		},
		{
			name: "Error - Extra bytes",
			input: []byte{
				0, 0, 0, 1,
				0, 0, 0, 3, 'K', 'L', 'V',
				0, 0, 0, 0,
				0, 1, // Extra bytes
			},
			expectedErr: "extra bytes found in buffer",
		},
		{
			name: "Error - Invalid pack length",
			input: []byte{
				0, 0, 0, 1,
				0, 0, 0, 3, 'K', 'L', 'V',
				0, 0, 0, // Invalid length
			},
			expectedErr: "unexpected EOF",
		},
		{
			name: "Error - Invalid pack amount read",
			input: []byte{
				0, 0, 0, 1,
				0, 0, 0, 3, 'K', 'L', 'V',
				0, 0, 0, 1,
				0, 0, 0, // Invalid amount length
			},
			expectedErr: "unexpected EOF",
		},
		{
			name: "Error - Invalid pack amount max length",
			input: []byte{
				0, 0, 0, 1,
				0, 0, 0, 3, 'K', 'L', 'V',
				0, 0, 0, 1,
				0, 0, 10, 0, // Invalid amount length
			},
			expectedErr: common.ErrMaxBytesExceeded.Error(),
		},
		{
			name: "Error - Invalid pack price read",
			input: []byte{
				0, 0, 0, 1,
				0, 0, 0, 3, 'K', 'L', 'V',
				0, 0, 0, 1,
				0, 0, 0, 1, 100,
				0, 0, 0, // Invalid price length
			},
			expectedErr: "unexpected EOF",
		},
		{
			name: "Error - Invalid pack price max length",
			input: []byte{
				0, 0, 0, 1,
				0, 0, 0, 3, 'K', 'L', 'V',
				0, 0, 0, 1,
				0, 0, 0, 1, 100,
				0, 0, 10, 0, // Invalid amount length
			},
			expectedErr: common.ErrMaxBytesExceeded.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecodeITOPacks(tt.input)

			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
				for key, pack := range tt.expected {
					assert.Equal(t, len(pack.Packs), len(result[key].Packs))
					for i, item := range pack.Packs {
						assert.Equal(t, item.Amount, result[key].Packs[i].Amount)
						assert.Equal(t, item.Price, result[key].Packs[i].Price)
					}
				}
			}
		})
	}
}

func TestDecodeITOWhitelist(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		address := bytes.Repeat([]byte{1}, 32)
		data := []byte{
			0, 0, 0, 1, // length of map
		}
		data = append(data, address...)
		data = append(data, 0, 0, 0, 1, 100) // limit 100

		result, err := DecodeITOWhitelist(data)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		key := hex.EncodeToString(address)
		assert.Equal(t, int64(100), result[key].Limit)
	})

	t.Run("Error - Extra bytes", func(t *testing.T) {
		data := []byte{
			0, 0, 0, 0, // empty list
			0, 0, 0, 32, // Extra bytes
		}

		result, err := DecodeITOWhitelist(data)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "extra bytes found in buffer")
	})
}

func TestDecodeURIs(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expected    map[string]string
		expectedErr string
	}{
		{
			name: "Success",
			input: []byte{
				0, 0, 0, 1, // length of map
				0, 0, 0, 3, 'k', 'e', 'y',
				0, 0, 0, 5, 'v', 'a', 'l', 'u', 'e',
			},
			expected: map[string]string{"key": "value"},
		},
		{
			name: "Error - Extra bytes",
			input: []byte{
				0, 0, 0, 1,
				0, 0, 0, 3, 'k', 'e', 'y',
				0, 0, 0, 5, 'v', 'a', 'l', 'u', 'e',
				0, // Extra byte
			},
			expectedErr: "extra bytes found in buffer",
		},
		{
			name: "Error - Invalid Map Size",
			input: []byte{
				0, 0, 0, 11, // Invalid length
				0, 0, 0, 3, 'k', 'e', 'y',
				0, 0, 0, 5, 'v', 'a', 'l', 'u', 'e',
			},
			expectedErr: common.ErrMaxBytesExceeded.Error(),
		},
		{
			name: "Error - Invalid Key Length",
			input: append([]byte{
				0, 0, 0, 1,
				0, 0, 0, 31,
			}, append(bytes.Repeat([]byte{1}, 31), 0, 0, 0, 5, 'v', 'a', 'l', 'u', 'e')...),
			expectedErr: common.ErrMaxBytesExceeded.Error(),
		},
		{
			name: "Error - Invalid Value Length",
			input: append([]byte{
				0, 0, 0, 1,
				0, 0, 0, 3, 'k', 'e', 'y',
				0, 0, 0, 251,
			}, bytes.Repeat([]byte{1}, 251)...),
			expectedErr: common.ErrMaxBytesExceeded.Error(),
		},
		{
			name:        "Error - reading buffer length",
			input:       []byte{0, 0, 0},
			expectedErr: "unexpected EOF",
		},
		{
			name: "Error - reading key length",
			input: []byte{
				0, 0, 0, 1,
				0, 0, 0,
			},
			expectedErr: "unexpected EOF",
		},
		{
			name: "Error - reading key",
			input: []byte{
				0, 0, 0, 1,
				0, 0, 0, 3, 'k', 'e',
			},
			expectedErr: "unexpected EOF",
		},
		{
			name: "Error - reading value length",
			input: []byte{
				0, 0, 0, 1,
				0, 0, 0, 3, 'k', 'e', 'y',
				0, 0, 0,
			},
			expectedErr: "unexpected EOF",
		},
		{
			name: "Error - reading value",
			input: []byte{
				0, 0, 0, 1,
				0, 0, 0, 3, 'k', 'e', 'y',
				0, 0, 0, 5, 'v', 'a',
			},
			expectedErr: "unexpected EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecodeURIs(tt.input)

			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestDecodeRoyaltiesData(t *testing.T) {
	address := bytes.Repeat([]byte{1}, 32)
	data := append(address, []byte{
		0, 0, 0, 1, // TransferPercentage length
		0, 0, 0, 1, 100, // Amount BitInt
		0, 0, 0, 0x0A, // Percentage Uint32
	}...)
	data = appendBigInt(data, big.NewInt(200)) // TransferFixed
	data = append(data, 0, 0, 0, 0x14)         // MarketPercentage
	data = appendBigInt(data, big.NewInt(300)) // MarketFixed
	data = append(data, []byte{
		0, 0, 0, 1, // SplitRoyalties length
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1,
		0, 0, 0, 0x0A, // PercentTransferPercentage
		0, 0, 0, 0x14, // PercentTransferFixed
		0, 0, 0, 0x1E, // PercentMarketPercentage
		0, 0, 0, 0x28, // PercentMarketFixed
		0, 0, 0, 0x32, // PercentITOPercentage
		0, 0, 0, 0x3C, // PercentITOFixed
	}...)
	data = append(data, 0, 0, 0, 0x46)         // ITOPercentage
	data = appendBigInt(data, big.NewInt(400)) // ITOFixed

	t.Run("Success", func(t *testing.T) {
		testData := append([]byte(nil), data...)
		result, err := DecodeRoyaltiesData(testData)
		require.NoError(t, err)
		assert.Equal(t, address, result.Address)
		assert.Len(t, result.TransferPercentage, 1)
		assert.Equal(t, int64(100), result.TransferPercentage[0].Amount)
		assert.Equal(t, uint32(10), result.TransferPercentage[0].Percentage)
		assert.Equal(t, int64(200), result.TransferFixed)
		assert.Equal(t, uint32(20), result.MarketPercentage)
		assert.Equal(t, int64(300), result.MarketFixed)
		assert.Len(t, result.SplitRoyalties, 1)
		splitKey := hex.EncodeToString(bytes.Repeat([]byte{1}, 32))
		assert.Equal(t, uint32(10), result.SplitRoyalties[splitKey].PercentTransferPercentage)
		assert.Equal(t, uint32(20), result.SplitRoyalties[splitKey].PercentTransferFixed)
		assert.Equal(t, uint32(30), result.SplitRoyalties[splitKey].PercentMarketPercentage)
		assert.Equal(t, uint32(40), result.SplitRoyalties[splitKey].PercentMarketFixed)
		assert.Equal(t, uint32(50), result.SplitRoyalties[splitKey].PercentITOPercentage)
		assert.Equal(t, uint32(60), result.SplitRoyalties[splitKey].PercentITOFixed)
		assert.Equal(t, uint32(70), result.ITOPercentage)
		assert.Equal(t, int64(400), result.ITOFixed)
	})

	t.Run("Error - Extra bytes", func(t *testing.T) {
		testData := append([]byte(nil), data...)
		testData = append(testData, 0, 0, 0, 0, 0) // Extra bytes

		result, err := DecodeRoyaltiesData(testData)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "extra bytes found in buffer")
	})
}

// appendBigInt appends a big.Int to a byte slice in the format expected by DecodeRoyaltiesData
func appendBigInt(data []byte, value *big.Int) []byte {
	bytes := value.Bytes()
	length := uint32(len(bytes))
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, length)
	data = append(data, lengthBytes...)
	return append(data, bytes...)
}

func TestEncodeOperationDataCheck(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		operations := []byte{1, 2, 3, 4}
		result, err := EncodeOperationDataCheck(operations)
		assert.NoError(t, err)
		assert.Equal(t, []byte{0, 0, 0, 8, 48, 49, 48, 50, 48, 51, 48, 52}, result)
	})

	t.Run("Error - Operations too long", func(t *testing.T) {
		operations := bytes.Repeat([]byte{1}, core.MaxOperationsSize+1)
		result, err := EncodeOperationDataCheck(operations)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, common.ErrInvalidPermission, err)
	})
}

func TestEncodeAccountPermissionData(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		permissions := []*transaction.AccPermission{
			{
				Type:           transaction.AccPermission_Owner,
				PermissionName: "TestPermission",
				Threshold:      100,
				Operations:     []byte{1, 2, 3},
				Signers: []*transaction.AccKey{
					{
						Address: bytes.Repeat([]byte{1}, 32),
						Weight:  50,
					},
				},
			},
		}

		result, err := EncodeAccountPermissionData(permissions)
		assert.NoError(t, err)
		assert.NotNil(t, result)

		// Decode the result to verify
		decodedPermissions, err := DecodeAccountPermissionData(result)
		assert.NoError(t, err)
		assert.Len(t, decodedPermissions, 1)
		assert.Equal(t, permissions[0].Type, decodedPermissions[0].Type)
		assert.Equal(t, permissions[0].PermissionName, decodedPermissions[0].PermissionName)
		assert.Equal(t, permissions[0].Threshold, decodedPermissions[0].Threshold)
		assert.Equal(t, permissions[0].Operations, decodedPermissions[0].Operations)
		assert.Len(t, decodedPermissions[0].Signers, 1)
		assert.Equal(t, permissions[0].Signers[0].Address, decodedPermissions[0].Signers[0].Address)
		assert.Equal(t, permissions[0].Signers[0].Weight, decodedPermissions[0].Signers[0].Weight)
	})

	t.Run("Error - Invalid address length", func(t *testing.T) {
		permissions := []*transaction.AccPermission{
			{
				Type:           transaction.AccPermission_Owner,
				PermissionName: "TestPermission",
				Threshold:      100,
				Operations:     []byte{1, 2, 3},
				Signers: []*transaction.AccKey{
					{
						Address: []byte{1, 2, 3}, // Invalid length
						Weight:  50,
					},
				},
			},
		}

		result, err := EncodeAccountPermissionData(permissions)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid address length")
	})

	t.Run("Error - Max Account Permission reached", func(t *testing.T) {
		permissions := make([]*transaction.AccPermission, core.MaxAccountPermission+1)
		for i := range permissions {
			permissions[i] = &transaction.AccPermission{
				Type:           transaction.AccPermission_Owner,
				PermissionName: "TestPermission",
				Threshold:      100,
				Operations:     []byte{1, 2, 3},
				Signers: []*transaction.AccKey{
					{
						Address: bytes.Repeat([]byte{1}, 32),
						Weight:  50,
					},
				},
			}
		}

		result, err := EncodeAccountPermissionData(permissions)
		assert.Nil(t, result)
		assert.Equal(t, common.ErrInvalidPermission, err)
	})
}

func TestDecodeAccountPermissionData(t *testing.T) {
	t.Run("Error - Invalid data", func(t *testing.T) {
		data := []byte{0, 0, 0, 1} // Only length, no actual data
		result, err := DecodeAccountPermissionData(data)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestReadBigUint(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		data := []byte{0, 0, 0, 2, 1, 44} // 300 in big-endian
		buf := bytes.NewReader(data)
		result, err := readBigUint(buf)
		assert.NoError(t, err)
		assert.Equal(t, big.NewInt(300), result)
	})

	t.Run("Error - Not enough data", func(t *testing.T) {
		data := []byte{0, 0, 0, 2, 1} // Missing one byte
		buf := bytes.NewReader(data)
		result, err := readBigUint(buf)
		assert.Nil(t, result)
		assert.ErrorContains(t, err, "unexpected EOF")
	})
}

func TestWriteAndReadHelpers(t *testing.T) {
	t.Run("writeUint32 and readUint32", func(t *testing.T) {
		buf := new(bytes.Buffer)
		err := writeUint32(buf, 12345)
		require.NoError(t, err)

		reader := bytes.NewReader(buf.Bytes())
		value, err := readUint32(reader)
		require.NoError(t, err)
		assert.Equal(t, uint32(12345), value)
	})

	t.Run("writeUint8 and readUint8", func(t *testing.T) {
		buf := new(bytes.Buffer)
		err := writeUint8(buf, 123)
		require.NoError(t, err)

		reader := bytes.NewReader(buf.Bytes())
		value, err := readUint8(reader)
		require.NoError(t, err)
		assert.Equal(t, uint8(123), value)
	})

	t.Run("writeString and readString", func(t *testing.T) {
		buf := new(bytes.Buffer)
		testString := "Hello, World!"
		err := writeString(buf, testString)
		require.NoError(t, err)

		reader := bytes.NewReader(buf.Bytes())
		value, err := readString(reader)
		require.NoError(t, err)
		assert.Equal(t, testString, value)
	})

	t.Run("writeString and readString - Empty string", func(t *testing.T) {
		buf := new(bytes.Buffer)
		testString := ""
		err := writeString(buf, testString)
		require.NoError(t, err)

		reader := bytes.NewReader(buf.Bytes())
		value, err := readString(reader)
		require.NoError(t, err)
		assert.Equal(t, testString, value)
	})

	t.Run("readString - Not enough data", func(t *testing.T) {
		buf := []byte{0, 0, 0, 5, 'H', 'e', 'l'} // Missing two bytes
		reader := bytes.NewReader(buf)
		value, err := readString(reader)
		require.Error(t, err)
		assert.Equal(t, "", value)
	})
}

func TestDecodeAccountPermissionData_Success(t *testing.T) {
	data, err := hex.DecodeString("00000001010000000a5065726d697373696f6e0000000000000001000000043066666600000001f64e21227e8df59be638d00acfafdeb70d6a678d6eee4d929cbb143bb1edc3e60000000000000001")
	assert.NoError(t, err)

	permissions, err := DecodeAccountPermissionData(data)
	assert.NoError(t, err)

	var addressConverter, _ = pubkeyConverter.NewBech32PubkeyConverter(32)

	addr, err := addressConverter.Decode("klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap")
	assert.NoError(t, err)

	expected := []*transaction.AccPermission{
		{
			Type:           1,
			PermissionName: "Permission",
			Threshold:      1,
			Operations:     []byte{15, 255},
			Signers:        []*transaction.AccKey{{Address: addr, Weight: 0}},
		},
	}

	for i, p := range permissions {
		assert.Equal(t, expected[i].Type, p.Type)
		assert.Equal(t, expected[i].PermissionName, p.PermissionName)
		assert.Equal(t, expected[i].Threshold, p.Threshold)
		assert.Equal(t, expected[i].Operations, p.Operations)
		assert.Equal(t, expected[i].Signers[0].Address, p.Signers[0].Address)
	}
}

func TestDecodeAccountPermissionData_OperationErr(t *testing.T) {
	data, err := hex.DecodeString("00000001010000000a5065726d697373696f6e00000000000000010000000430667a6600000001f64e21227e8df59be638d00acfafdeb70d6a678d6eee4d929cbb143bb1edc3e60000000000000001")
	assert.NoError(t, err)

	permissions, err := DecodeAccountPermissionData(data)
	assert.Equal(t, errors.New("error decoding operations"), err)
	assert.Nil(t, permissions)

	// operations len > core.MaxOperationsSize
	operations := bytes.Repeat([]byte{0xFF}, core.MaxOperationsSize+1)
	// Convert to hexadecimal string
	operationsASCII := encodeOperationData(operations)

	data2, err := hex.DecodeString("00000001010000000a5065726d697373696f6e0000000000000001" + operationsASCII + "00000001f64e21227e8df59be638d00acfafdeb70d6a678d6eee4d929cbb143bb1edc3e60000000000000001")
	assert.NoError(t, err)

	permissions2, err := DecodeAccountPermissionData(data2)
	assert.Equal(t, common.ErrInvalidPermission, err)
	assert.Nil(t, permissions2)
}

func createParameters(parameters map[int32][]byte) []byte {
	buf := new(bytes.Buffer)

	_ = writeUint32(buf, uint32(len(parameters)))
	for i, k := range parameters {
		_ = writeInt32(buf, i)
		_ = writeUint32(buf, uint32(len(k)))
		_, _ = buf.Write(k)
	}
	return buf.Bytes()
}

func TestDecodeProposalContract(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expected    map[int32][]byte
		expectedErr error
	}{
		{
			name: "Success",
			input: createParameters(map[int32][]byte{
				1: []byte("2000000"),
				3: []byte("4000000"),
				5: []byte("6000000"),
			}),
			expected: map[int32][]byte{
				1: []byte("2000000"),
				3: []byte("4000000"),
				5: []byte("6000000"),
			},
			expectedErr: nil,
		},
		{
			name: "Invalid parameters len",
			input: []byte{
				0, 0, 0,
			},
			expectedErr: io.ErrUnexpectedEOF,
		},
		{
			name: "max proposals len error",
			input: []byte{
				0, 0, 0, 0xB,
			},
			expectedErr: common.ErrMaxBytesExceeded,
		},
		{
			name: "invalid parameter key lenght",
			input: []byte{
				0, 0, 0, 4,
				0, 0, 0,
			},
			expectedErr: io.ErrUnexpectedEOF,
		},
		{
			name: "invalid parameter value lenght",
			input: append([]byte{
				0, 0, 0, 1,
				0, 0, 0, 3,
			}, bytes.Repeat([]byte{1}, (2*core.MaxProposalParamLength))...),
			expectedErr: common.ErrMaxBytesExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)

			result, err := DecodeParameters(tt.input)
			if tt.expectedErr != nil {
				assert.Error(err)
				assert.Equal(tt.expectedErr, err)
				assert.Nil(result)
			} else {
				assert.NoError(err)
				assert.Equal(tt.expected, result)
			}
		})
	}

}
