package kapps_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/assert"
)

func TestKDAData_GetTransferRoyaltyByAmount(t *testing.T) {
	tests := []struct {
		name                       string
		royalties                  []*kapps.RoyaltyData
		amount                     int64
		isKdaFprFork               bool
		isEnableSmartContractsFork bool
		expectedRoyalty            int64
		expectedError              error
	}{
		{
			name:                       "Empty royalties returns error",
			royalties:                  []*kapps.RoyaltyData{},
			amount:                     1000,
			isKdaFprFork:               true,
			isEnableSmartContractsFork: true,
			expectedRoyalty:            0,
			expectedError:              common.ErrInvalidValue,
		},
		{
			name: "Old flow - basic calculation",
			royalties: []*kapps.RoyaltyData{
				{Amount: 1000, Percentage: 500},
				{Amount: 5000, Percentage: 300},
			},
			amount:                     500,
			isKdaFprFork:               false,
			isEnableSmartContractsFork: true,
			expectedRoyalty:            25,
			expectedError:              nil,
		},
		{
			name: "Old flow - uses last percentage",
			royalties: []*kapps.RoyaltyData{
				{Amount: 1000, Percentage: 500},
				{Amount: 5000, Percentage: 300},
			},
			amount:                     15000,
			isKdaFprFork:               false,
			isEnableSmartContractsFork: true,
			expectedRoyalty:            450,
			expectedError:              nil,
		},
		{
			name: "New flow - single royalty",
			royalties: []*kapps.RoyaltyData{
				{Amount: 1000, Percentage: 500},
			},
			amount:                     800,
			isKdaFprFork:               true,
			isEnableSmartContractsFork: true,
			expectedRoyalty:            40,
			expectedError:              nil,
		},
		{
			name: "New flow - multiple royalties",
			royalties: []*kapps.RoyaltyData{
				{Amount: 1000, Percentage: 500},
				{Amount: 2000, Percentage: 300},
			},
			amount:                     2500,
			isKdaFprFork:               true,
			isEnableSmartContractsFork: true,
			expectedRoyalty:            95,
			expectedError:              nil,
		},
		{
			name: "New flow - exceeds all royalties",
			royalties: []*kapps.RoyaltyData{
				{Amount: 1000, Percentage: 200},
			},
			amount:                     3000,
			isKdaFprFork:               true,
			isEnableSmartContractsFork: true,
			expectedRoyalty:            60,
			expectedError:              nil,
		},
		{
			name: "Zero amount",
			royalties: []*kapps.RoyaltyData{
				{Amount: 1000, Percentage: 500},
			},
			amount:                     0,
			isKdaFprFork:               true,
			isEnableSmartContractsFork: true,
			expectedRoyalty:            0,
			expectedError:              nil,
		},
		{
			name: "Math overflow should cause error",
			royalties: []*kapps.RoyaltyData{
				{Amount: 1000, Percentage: 2147483647},
			},
			amount:                     9223372036854775807,
			isKdaFprFork:               true,
			isEnableSmartContractsFork: true,
			expectedRoyalty:            0,
			expectedError:              common.ErrInt64Overflow,
		},
		{
			name: "Old flow - overflow in splitToPay calculation",
			royalties: []*kapps.RoyaltyData{
				{Amount: 9223372036854775807, Percentage: 2147483647}, // both max values
				{Amount: 5000, Percentage: 300},
			},
			amount:                     9223372036854775806, // slightly less than max, should hit old flow
			isKdaFprFork:               false,
			isEnableSmartContractsFork: true,
			expectedRoyalty:            0,
			expectedError:              common.ErrInt64Overflow,
		},
		{
			name: "New flow - overflow in loop calculation with exact royalty amount",
			royalties: []*kapps.RoyaltyData{
				{Amount: 4611686018427387903, Percentage: 2147483647}, // both max values
			},
			amount:                     4611686018427387904, // slightly more than royalty.Amount
			isKdaFprFork:               true,
			isEnableSmartContractsFork: true,
			expectedRoyalty:            0,
			expectedError:              common.ErrInt64Overflow,
		},
		{
			name: "New flow - overflow when amount < royalty.Amount",
			royalties: []*kapps.RoyaltyData{
				{Amount: 9223372036854775807, Percentage: 2147483647}, // max values
			},
			amount:                     4611686018427387903, // amount < royalty.Amount
			isKdaFprFork:               true,
			isEnableSmartContractsFork: true,
			expectedRoyalty:            0,
			expectedError:              common.ErrInt64Overflow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kda := &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferPercentage: tt.royalties,
				},
			}

			result, err := kda.GetTransferRoyaltyByAmount(
				tt.amount,
				tt.isKdaFprFork,
				tt.isEnableSmartContractsFork,
			)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
				assert.Equal(t, tt.expectedRoyalty, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRoyalty, result)
			}
		})
	}
}
