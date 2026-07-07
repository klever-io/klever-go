package bitmap_test

import (
	"testing"

	bitmaputil "github.com/klever-io/klever-go/tools/bitmap"
	"github.com/stretchr/testify/require"
)

func TestHasPaddingBitsSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		bitmap       []byte
		numValidBits int
		expected     bool
	}{
		{
			name:         "no bits set",
			bitmap:       []byte{0x00},
			numValidBits: 5,
			expected:     false,
		},
		{
			name:         "only valid bits set",
			bitmap:       []byte{0x1F}, // bits 0..4
			numValidBits: 5,
			expected:     false,
		},
		{
			name:         "padding bit set in last byte",
			bitmap:       []byte{0x20}, // bit 5 set, beyond 5 valid bits
			numValidBits: 5,
			expected:     true,
		},
		{
			name:         "count multiple of 8 has no padding bits",
			bitmap:       []byte{0xFF},
			numValidBits: 8,
			expected:     false,
		},
		{
			name:         "all valid bits set across two bytes",
			bitmap:       []byte{0xFF, 0x03}, // bits 0..9
			numValidBits: 10,
			expected:     false,
		},
		{
			name:         "padding bit set in trailing byte beyond required length",
			bitmap:       []byte{0xFF, 0x00, 0x01}, // bit 16 set, only 8 valid bits
			numValidBits: 8,
			expected:     true,
		},
		{
			name:         "empty bitmap",
			bitmap:       []byte{},
			numValidBits: 0,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.expected, bitmaputil.HasPaddingBitsSet(tt.bitmap, tt.numValidBits))
		})
	}
}
