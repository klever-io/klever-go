package kdautils

import (
	"bytes"
	"testing"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/stretchr/testify/require"
)

func TestExtractAssetIDAndNonce(t *testing.T) {
	tests := []struct {
		name          string
		input         []byte
		expectedKDA   []byte
		expectedNonce uint64
		expectedType  core.KDAType
		expectError   bool
	}{
		{
			name:          "Valid input with KLV",
			input:         []byte("KLV"),
			expectedKDA:   []byte("KLV"),
			expectedNonce: 0,
			expectedType:  0,
			expectError:   false,
		},
		{
			name:          "Valid input with FUNGIBLE-28EN",
			input:         []byte("FUNGIBLE-28EN"),
			expectedKDA:   []byte("FUNGIBLE-28EN"),
			expectedNonce: 0,
			expectedType:  0,
			expectError:   false,
		},
		{
			name:          "Valid input with NFT-28EN/323",
			input:         []byte("NFT-28EN/323"),
			expectedKDA:   []byte("NFT-28EN"),
			expectedNonce: 323,
			expectedType:  1,
			expectError:   false,
		},
		{
			name:          "Invalid input with wrong base36 FUNGIBLE-36#2",
			input:         []byte("FUNGIBLE-36#2"),
			expectedKDA:   []byte("FUNGIBLE-36#2"),
			expectedNonce: 0,
			expectedType:  0,
			expectError:   true,
		},
		{
			name:          "Invalid input with more than 2 separators",
			input:         []byte("FUNGIBLE-28EN-3"),
			expectedKDA:   []byte("FUNGIBLE-28EN-3"),
			expectedNonce: 0,
			expectedType:  0,
			expectError:   true,
		},
		{
			name:          "Invalid input with invalid length",
			input:         []byte("FUNGIBLEAAAAAAAAA-28EN"),
			expectedKDA:   []byte("FUNGIBLEAAAAAAAAA-28EN"),
			expectedNonce: 0,
			expectedType:  0,
			expectError:   true,
		},
		{
			name:          "Invalid input with invalid random sequence length",
			input:         []byte("FUNGIBLE-BIGGER"),
			expectedKDA:   []byte("FUNGIBLE-BIGGER"),
			expectedNonce: 0,
			expectedType:  0,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kda, nonce, tokenType, err := ExtractAssetIDAndNonce(tt.input)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedKDA, kda)
				require.Equal(t, tt.expectedNonce, nonce)
				require.Equal(t, tt.expectedType, tokenType)
			}
		})
	}
}

func TestIsTickerValid(t *testing.T) {
	tests := []struct {
		name     string
		ticker   []byte
		expected bool
	}{
		{"Valid ticker", bytes.Repeat([]byte("A"), core.MinLengthForAssetTicker), true},
		{"Valid ticker with numbers", []byte("ABC123"), true},
		{"Valid ticker with max length", bytes.Repeat([]byte("A"), core.MaxLengthForAssetTicker), true},
		{"Too short", bytes.Repeat([]byte("A"), core.MinLengthForAssetTicker-1), false},
		{"Too long", bytes.Repeat([]byte("A"), core.MaxLengthForAssetTicker+1), false},
		{"Invalid character", []byte("ABC-123"), false},
		{"Reserved KLV", []byte("KLV"), false},
		{"Reserved KFI", []byte("KFI"), false},
		{"Lowercase invalid", []byte("abc"), false},
		{"Special chars invalid", []byte("ABC$"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTickerValid(tt.ticker)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsAssetNameHumanReadable(t *testing.T) {
	tests := []struct {
		name     string
		asset    []byte
		expected bool
	}{
		{"Valid name", []byte("ValidName123"), true},
		{"Mixed case", []byte("MixedCaseValid"), true},
		{"Short valid", []byte("a"), true},
		{"Empty", []byte(""), false},
		{"Too long", bytes.Repeat([]byte("a"), core.MaxLengthForAssetName+1), false},
		{"Special chars invalid", []byte("Name@123"), false},
		{"Numbers valid", []byte("Name123"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAssetNameHumanReadable(tt.asset)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsAssetNameHumanReadableOld(t *testing.T) {
	tests := []struct {
		name     string
		asset    []byte
		expected bool
	}{
		{"Valid name", []byte("Valid Name 123"), true},
		{"Mixed case with space", []byte("Mixed Case Valid"), true},
		{"Short valid", []byte("a"), true},
		{"Empty", []byte(""), false},
		{"Too long", bytes.Repeat([]byte("a"), core.MaxLengthForAssetName+1), false},
		{"Special chars invalid", []byte("Name@123"), false},
		{"Numbers and spaces valid", []byte("Name 123"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAssetNameHumanReadableOld(tt.asset)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSupplyValid(t *testing.T) {
	tests := []struct {
		name       string
		mintable   bool
		init       int64
		circ       int64
		max        int64
		isExpected bool
	}{
		{"Non-mintable equal supplies", false, 1000, 1000, 1000, true},
		{"Non-mintable unequal supplies", false, 1000, 2000, 1000, false},
		{"Mintable infinite max supply", true, 1000, 1000, 0, true},
		{"Mintable valid max supply", true, 1000, 1000, 2000, true},
		{"Mintable invalid max supply", true, 2000, 2000, 1000, false},
		{"Negative initial supply", false, -1000, 1000, 1000, false},
		{"Negative circulating supply", false, 1000, -1000, 1000, false},
		{"Negative max supply", false, 1000, 1000, -1000, false},
		{"Unequal init and circ supply", true, 1000, 2000, 3000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSupplyValid(tt.mintable, tt.init, tt.circ, tt.max)
			require.Equal(t, tt.isExpected, result)
		})
	}
}

func TestIsNFTContractValid(t *testing.T) {
	tests := []struct {
		name      string
		init      int64
		precision uint32
		expected  bool
	}{
		{"Valid NFT", 0, 0, true},
		{"Invalid initial supply", 1, 0, false},
		{"Invalid precision", 0, 1, false},
		{"Both invalid", 1, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNFTContractValid(tt.init, tt.precision)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestToBucketID(t *testing.T) {
	hasher := sha256.Sha256{}
	randSeed := []byte("seed")
	sender := []byte("sender")
	assetID := []byte("asset")
	var nonce uint64 = 1
	contractIndex := 0
	amount := int64(100)

	result := ToBucketID(hasher, randSeed, sender, assetID, nonce, contractIndex, amount)
	require.NotNil(t, result)
	require.Greater(t, len(result), 0)

	// Test deterministic output
	result2 := ToBucketID(hasher, randSeed, sender, assetID, nonce, contractIndex, amount)
	require.Equal(t, result, result2)
}

func TestToMarketID(t *testing.T) {
	hasher := sha256.Sha256{}
	randSeed := []byte("seed")
	sender := []byte("sender")
	var nonce uint64 = 1
	contractIndex := 0
	marketKeyLength := 8

	result := ToMarketID(hasher, randSeed, sender, nonce, contractIndex, marketKeyLength)
	require.NotNil(t, result)
	require.Equal(t, marketKeyLength, len(result))

	// Test deterministic output
	result2 := ToMarketID(hasher, randSeed, sender, nonce, contractIndex, marketKeyLength)
	require.Equal(t, result, result2)
}

func TestToKDAKey(t *testing.T) {
	tests := []struct {
		name     string
		kdaID    []byte
		nonce    []byte
		expected string
	}{
		{"Basic KDA", []byte("ABC"), nil, "KDA/ABC"},
		{"KDA with nonce", []byte("ABC"), []byte("123"), "KDA/ABC/123"},
		{"Zero nonce", []byte("ABC"), []byte("0"), "KDA/ABC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToKDAKey(tt.kdaID, tt.nonce)
			require.Equal(t, []byte(tt.expected), result)
		})
	}
}

func TestToKDAKeyWithoutPrefix(t *testing.T) {
	tests := []struct {
		name     string
		kdaID    []byte
		nonce    []byte
		expected string
	}{
		{"Basic KDA", []byte("ABC"), nil, "ABC"},
		{"KDA with nonce", []byte("ABC"), []byte("123"), "ABC/123"},
		{"Zero nonce", []byte("ABC"), []byte("0"), "ABC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToKDAKeyWithouPrefix(tt.kdaID, tt.nonce)
			require.Equal(t, []byte(tt.expected), result)
		})
	}
}

func TestToProposalKey(t *testing.T) {
	tests := []struct {
		name       string
		proposalID uint64
		expected   string
	}{
		{"Zero ID", 0, "PROP/0"},
		{"Basic proposal", 123, "PROP/123"},
		{"Large ID", 999999, "PROP/999999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToProposalKey(tt.proposalID)
			require.Equal(t, []byte(tt.expected), result)
		})
	}
}

func TestToITOKey(t *testing.T) {
	tests := []struct {
		name     string
		kdaID    []byte
		expected string
	}{
		{"Basic ITO", []byte("ABC"), "ITO/ABC"},
		{"Empty ID", []byte{}, "ITO/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToITOKey(tt.kdaID)
			require.Equal(t, []byte(tt.expected), result)
		})
	}
}

func TestToITOWhitelistKey(t *testing.T) {
	tests := []struct {
		name       string
		kdaID      []byte
		hexAddress string
		expected   string
	}{
		{"Basic whitelist", []byte("ABC"), "0x123", "ITOWL/ABC/0x123"},
		{"Empty address", []byte("ABC"), "", "ITOWL/ABC/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToITOWhitelistKey(tt.kdaID, tt.hexAddress)
			require.Equal(t, []byte(tt.expected), result)
		})
	}
}

func TestToMarketOrderKey(t *testing.T) {
	tests := []struct {
		name     string
		orderID  []byte
		expected string
	}{
		{"Basic order", []byte("ORDER1"), "MKT/ORDER1"},
		{"Empty ID", []byte{}, "MKT/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToMarketOrderKey(tt.orderID)
			require.Equal(t, []byte(tt.expected), result)
		})
	}
}

func TestToMarketplaceKey(t *testing.T) {
	tests := []struct {
		name          string
		marketplaceID []byte
		expected      string
	}{
		{"Basic marketplace", []byte("MARKET1"), "MKTPLACE/MARKET1"},
		{"Empty ID", []byte{}, "MKTPLACE/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToMarketplaceKey(tt.marketplaceID)
			require.Equal(t, []byte(tt.expected), result)
		})
	}
}
