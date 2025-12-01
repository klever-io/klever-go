package node

import (
	"testing"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/stretchr/testify/assert"
)

func TestComputeRewardsForAsset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rewards  map[string]int64
		assetId  string
		expected int64
	}{
		{
			name:     "nil rewards map returns zero",
			rewards:  nil,
			assetId:  "KLV",
			expected: 0,
		},
		{
			name:     "empty rewards map returns zero",
			rewards:  map[string]int64{},
			assetId:  "KLV",
			expected: 0,
		},
		{
			name: "KLV asset returns KLV rewards",
			rewards: map[string]int64{
				"KLV": 1000,
				"BTC": 500,
			},
			assetId:  "KLV",
			expected: 1000,
		},
		{
			name: "KFI asset returns KLV rewards (KFI rewards are in KLV stack)",
			rewards: map[string]int64{
				"KLV": 2000,
				"KFI": 100,
			},
			assetId:  string(kdautils.KFIIdentifier),
			expected: 2000,
		},
		{
			name: "other asset returns its own rewards",
			rewards: map[string]int64{
				"KLV":   1000,
				"TOKEN": 500,
			},
			assetId:  "TOKEN",
			expected: 500,
		},
		{
			name: "missing asset returns zero",
			rewards: map[string]int64{
				"KLV": 1000,
			},
			assetId:  "MISSING",
			expected: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := computeRewardsForAsset(tt.rewards, tt.assetId)
			assert.Equal(t, tt.expected, result)
		})
	}
}
