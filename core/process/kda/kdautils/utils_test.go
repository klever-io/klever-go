package kdautils

import (
	"testing"

	"github.com/klever-io/klever-go/core"
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
