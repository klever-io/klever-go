package transaction_test

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/stretchr/testify/require"
)

func TestIsContractSizeValid(t *testing.T) {
	tests := []struct {
		description    string
		contractType   transaction.TXContract_ContractType
		contract       []byte
		expectedResult bool
	}{
		{
			description:  "Transfer contract should be valid",
			contractType: transaction.TXContract_TransferContractType,
			contract: convertContract(models.TransferTXRequest{
				Receiver:     createDummyHexAddress(64),
				Amount:       math.MaxInt64,
				KDA:          generateRandomString(core.MaxLengthForAssetTicker),
				KDARoyalties: math.MaxInt64,
				KLVRoyalties: math.MaxInt64,
			}),
			expectedResult: true,
		},
		{
			description:    "Transfer contract size error",
			contractType:   transaction.TXContract_TransferContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_TransferContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Create Asset contract should be valid",
			contractType: transaction.TXContract_CreateAssetContractType,
			contract: convertContract(&models.CreateAssetTXRequest{
				Type:          math.MaxUint32,
				Name:          generateRandomString(core.MaxLengthForAssetName),
				Ticker:        generateRandomString(core.MaxLengthForAssetTicker),
				OwnerAddress:  createDummyHexAddress(64),
				AdminAddress:  createDummyHexAddress(64),
				Logo:          generateRandomString(core.MaxLogoURISize),
				URIs:          getUri(),
				Precision:     math.MaxUint32,
				InitialSupply: math.MaxInt64,
				MaxSupply:     math.MaxInt64,
				Royalties: &models.RoyaltiesInfo{
					Address:        createDummyHexAddress(64),
					SplitRoyalties: getSplitRoyalties(),
				},
				Properties: &models.PropertiesInfo{},
				Attributes: &models.AttributesInfo{},
				Staking:    &models.StakingInfo{},
				Roles:      getRoles(),
			}),
			expectedResult: true,
		}, {
			description:    "Create Asset contract size error",
			contractType:   transaction.TXContract_CreateAssetContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_CreateAssetContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Create Validator contract should be valid",
			contractType: transaction.TXContract_CreateValidatorContractType,
			contract: convertContract(&models.CreateValidatorTXRequest{
				BLSPublicKey:        createDummyHexAddress(96),
				OwnerAddress:        createDummyHexAddress(64),
				RewardAddress:       createDummyHexAddress(64),
				CanDelegate:         true,
				Commission:          math.MaxUint32,
				MaxDelegationAmount: math.MaxInt64,
				Logo:                generateRandomString(core.MaxLogoURISize),
				Name:                generateRandomString(core.MaxNameSize),
				URIs:                getUri(),
			}),
			expectedResult: true,
		},
		{
			description:    "Create Validator contract size error",
			contractType:   transaction.TXContract_CreateValidatorContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_CreateValidatorContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Validator Config contract should be valid",
			contractType: transaction.TXContract_ValidatorConfigContractType,
			contract: convertContract(&models.ValidatorConfigTXRequest{
				BLSPublicKey:        createDummyHexAddress(194),
				RewardAddress:       createDummyHexAddress(64),
				CanDelegate:         true,
				Commission:          math.MaxUint32,
				MaxDelegationAmount: math.MaxInt64,
				Logo:                generateRandomString(core.MaxLogoURISize),
				Name:                generateRandomString(core.MaxNameSize),
				URIs:                getUri(),
			}),
			expectedResult: true,
		},
		{
			description:    "Validator Config contract size error",
			contractType:   transaction.TXContract_ValidatorConfigContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_ValidatorConfigContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Freeze contract should be valid",
			contractType: transaction.TXContract_FreezeContractType,
			contract: convertContract(&models.FreezeTXRequest{
				KDA:    generateRandomString(core.MaxLengthForAssetTicker),
				Amount: math.MaxInt64,
			}),
			expectedResult: true,
		},
		{
			description:    "Freeze contract size error",
			contractType:   transaction.TXContract_FreezeContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_FreezeContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Unfreeze contract should be valid",
			contractType: transaction.TXContract_UnfreezeContractType,
			contract: convertContract(&models.UnfreezeTXRequest{
				KDA:      generateRandomString(core.MaxLengthForAssetTicker),
				BucketID: createDummyHexAddress(66),
			}),
			expectedResult: true,
		},
		{
			description:    "Unfreeze contract size error",
			contractType:   transaction.TXContract_UnfreezeContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_UnfreezeContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Delegate contract should be valid",
			contractType: transaction.TXContract_DelegateContractType,
			contract: convertContract(&models.DelegateTXRequest{
				Receiver: createDummyHexAddress(64),
				BucketID: createDummyHexAddress(66),
			}),
			expectedResult: true,
		},
		{
			description:    "Delegate contract size error",
			contractType:   transaction.TXContract_DelegateContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_DelegateContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Undelegate contract should be valid",
			contractType: transaction.TXContract_UndelegateContractType,
			contract: convertContract(&models.UndelegateTXRequest{
				BucketID: createDummyHexAddress(66),
			}),
			expectedResult: true,
		},
		{
			description:    "Undelegate contract size error",
			contractType:   transaction.TXContract_UndelegateContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_UndelegateContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Withdraw contract should be valid",
			contractType: transaction.TXContract_WithdrawContractType,
			contract: convertContract(&models.WithdrawTXRequest{
				KDA:          generateRandomString(core.MaxLengthForAssetTicker),
				CurrencyID:   generateRandomString(core.MaxLengthForAssetTicker),
				WithdrawType: math.MaxInt32,
				Amount:       math.MaxInt64,
			}),
			expectedResult: true,
		},
		{
			description:    "Withdraw contract size error",
			contractType:   transaction.TXContract_WithdrawContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_WithdrawContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Claim contract should be valid",
			contractType: transaction.TXContract_ClaimContractType,
			contract: convertContract(&models.ClaimTXRequest{
				ClaimType: math.MaxInt32,
				ID:        generateRandomString(core.MaxLengthForAssetTicker),
			}),
			expectedResult: true,
		},
		{
			description:    "Claim contract size error",
			contractType:   transaction.TXContract_ClaimContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_ClaimContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Asset Trigger contract should be valid",
			contractType: transaction.TXContract_AssetTriggerContractType,
			contract: convertContract(&models.AssetTriggerTXRequest{
				TriggerType: math.MaxUint32,
				AssetID:     createDummyHexAddress(core.MaxLengthForAssetTicker),
				Receiver:    createDummyHexAddress(64),
				Amount:      math.MaxInt64,
				MIME:        createDummyHexAddress(256),
				Logo:        generateRandomString(core.MaxLogoURISize),
				URIs:        getUri(),
				Role: &models.RolesInfo{
					Address: createDummyHexAddress(64),
				},
				Staking: &models.StakingInfo{},
				Royalties: &models.RoyaltiesInfo{
					Address:        createDummyHexAddress(64),
					SplitRoyalties: getSplitRoyalties(),
				},
				KDAPool: &models.KDAPoolInfo{
					AdminAddress: createDummyHexAddress(64),
					FRatioKLV:    math.MaxInt64,
					FRatioKDA:    math.MaxInt64,
				},
			}),
			expectedResult: true,
		},
		{
			description:    "Asset Trigger contract size error",
			contractType:   transaction.TXContract_AssetTriggerContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_AssetTriggerContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Set Account Name contract should be valid",
			contractType: transaction.TXContract_SetAccountNameContractType,
			contract: convertContract(&models.SetAccountNameTXRequest{
				Name: generateRandomString(core.MaxNameSize),
			}),
			expectedResult: true,
		},
		{
			description:    "Set Account Name contract size error",
			contractType:   transaction.TXContract_AssetTriggerContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_AssetTriggerContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Proposal contract should be valid",
			contractType: transaction.TXContract_ProposalContractType,
			contract: convertContract(&models.ProposalTXRequest{
				Parameters:     getProposalParameteres(),
				Description:    generateRandomString(core.MaxDescriptionLength),
				EpochsDuration: math.MaxUint32,
			}),
			expectedResult: true,
		},
		{
			description:    "Proposal contract size error",
			contractType:   transaction.TXContract_ProposalContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_ProposalContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Vote contract should be valid",
			contractType: transaction.TXContract_VoteContractType,
			contract: convertContract(&models.VoteTXRequest{
				Type:       math.MaxUint32,
				ProposalID: math.MaxUint64,
				Amount:     math.MaxInt64,
			}),
			expectedResult: true,
		},
		{
			description:    "Vote contract size error",
			contractType:   transaction.TXContract_VoteContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_VoteContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Config ITO contract should be valid",
			contractType: transaction.TXContract_ConfigITOContractType,
			contract: convertContract(&models.ConfigITOTXRequest{
				KDA:                    generateRandomString(core.MaxLengthForAssetTicker),
				ReceiverAddress:        createDummyHexAddress(64),
				Status:                 math.MaxInt32,
				MaxAmount:              math.MaxInt64,
				PackInfo:               getPackInfo(),
				DefaultLimitPerAddress: math.MaxInt64,
				WhitelistStatus:        math.MaxInt32,
				WhitelistInfo:          getWhitelistInfo(),
				WhitelistStartTime:     math.MaxInt64,
				WhitelistEndTime:       math.MaxInt64,
				StartTime:              math.MaxInt64,
				EndTime:                math.MaxInt64,
			}),
			expectedResult: true,
		},
		{
			description:    "Config ITO contract size error",
			contractType:   transaction.TXContract_ConfigITOContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_ConfigITOContractType]+1),
			expectedResult: false,
		},
		{
			description:  "ITO Trigger contract should be valid",
			contractType: transaction.TXContract_ITOTriggerContractType,
			contract: convertContract(&models.ITOTriggerTXRequest{
				KDA:                    generateRandomString(core.MaxLengthForAssetTicker),
				ReceiverAddress:        createDummyHexAddress(64),
				Status:                 math.MaxInt32,
				MaxAmount:              math.MaxInt64,
				PackInfo:               getPackInfo(),
				DefaultLimitPerAddress: math.MaxInt64,
				WhitelistStatus:        math.MaxInt32,
				WhitelistInfo:          getWhitelistInfo(),
				WhitelistStartTime:     math.MaxInt64,
				WhitelistEndTime:       math.MaxInt64,
				StartTime:              math.MaxInt64,
				EndTime:                math.MaxInt64,
			}),
			expectedResult: true,
		},
		{
			description:    "ITO trigger contract size error",
			contractType:   transaction.TXContract_ITOTriggerContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_ITOTriggerContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Buy contract should be valid",
			contractType: transaction.TXContract_BuyContractType,
			contract: convertContract(&models.BuyTXRequest{
				BuyType:        math.MaxInt32,
				ID:             generateRandomString(core.MaxLengthForAssetTicker),
				Amount:         math.MaxInt64,
				CurrencyID:     generateRandomString(core.MaxLengthForAssetTicker),
				CurrencyAmount: math.MaxInt64,
			}),
			expectedResult: true,
		},
		{
			description:    "Buy contract size error",
			contractType:   transaction.TXContract_BuyContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_BuyContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Sell contract should be valid",
			contractType: transaction.TXContract_SellContractType,
			contract: convertContract(&models.SellTXRequest{
				MarketType:    1,
				MarketplaceID: generateRandomString(16),
				AssetID:       generateRandomString(core.MaxLengthForAssetTicker),
				CurrencyID:    generateRandomString(core.MaxLengthForAssetTicker),
				Price:         math.MaxInt64,
				ReservePrice:  math.MaxInt64,
				EndTime:       math.MaxInt64,
			}),
			expectedResult: true,
		},
		{
			description:    "Sell contract size error",
			contractType:   transaction.TXContract_SellContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_SellContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Cancel Market Order contract should be valid",
			contractType: transaction.TXContract_CancelMarketOrderContractType,
			contract: convertContract(&models.CancelMarketOrderTXRequest{
				OrderID: generateRandomString(16),
			}),
			expectedResult: true,
		},
		{
			description:    "Cancel Market Order contract size error",
			contractType:   transaction.TXContract_CancelMarketOrderContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_CancelMarketOrderContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Create Marketplace contract should be valid",
			contractType: transaction.TXContract_CreateMarketplaceContractType,
			contract: convertContract(&models.CreateMarketplaceTXRequest{
				Name:               generateRandomString(core.MaxNameSize),
				ReferralAddress:    createDummyHexAddress(64),
				ReferralPercentage: math.MaxUint32,
			}),
			expectedResult: true,
		},
		{
			description:    "Create Marketplace contract size error",
			contractType:   transaction.TXContract_CreateMarketplaceContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_CreateMarketplaceContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Create Marketplace contract should be valid",
			contractType: transaction.TXContract_ConfigMarketplaceContractType,
			contract: convertContract(&models.ConfigMarketplaceTXRequest{
				MarketplaceID:      generateRandomString(16),
				Name:               generateRandomString(core.MaxNameSize),
				ReferralAddress:    createDummyHexAddress(64),
				ReferralPercentage: math.MaxUint32,
			}),
			expectedResult: true,
		},
		{
			description:    "Config Marketplace contract size error",
			contractType:   transaction.TXContract_ConfigMarketplaceContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_ConfigMarketplaceContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Update Account Permission contract should be valid",
			contractType: transaction.TXContract_UpdateAccountPermissionContractType,
			contract: convertContract(&models.UpdateAccountPermissionTXRequest{
				Permissions: getUpdatePermissions(),
			}),
			expectedResult: true,
		},
		{
			description:    "Update Account Permission contract size error",
			contractType:   transaction.TXContract_UpdateAccountPermissionContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_UpdateAccountPermissionContractType]+1),
			expectedResult: false,
		},
		{
			description:  "Deposit contract should be valid",
			contractType: transaction.TXContract_DelegateContractType,
			contract: convertContract(&models.DepositTXRequest{
				DepositType: math.MaxInt32,
				CurrencyID:  generateRandomString(core.MaxLengthForAssetTicker),
				KDA:         generateRandomString(core.MaxLengthForAssetTicker),
				Amount:      math.MaxInt64,
			}),
			expectedResult: true,
		},
		{
			description:    "Deposit contract size error",
			contractType:   transaction.TXContract_DepositContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_DepositContractType]+1),
			expectedResult: false,
		},
		{
			description:  "SmartContract contract should be valid",
			contractType: transaction.TXContract_SmartContractType,
			contract: convertContract(&models.SmartContractRequest{
				SCType:    math.MaxInt32,
				Address:   createDummyHexAddress(64),
				CallValue: getCallVallues(),
			}),
			expectedResult: true,
		},
		{
			description:    "SmartContract contract size error",
			contractType:   transaction.TXContract_SmartContractType,
			contract:       make([]byte, transaction.ContractMaxSizes[transaction.TXContract_SmartContractType]+1),
			expectedResult: false,
		},
		{
			description:    "invalid type",
			contractType:   99999,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			require := require.New(t)

			result := transaction.IsContractSizeValid(tt.contract, uint32(tt.contractType))
			require.Equal(tt.expectedResult, result)
		})
	}

}

// helpers

func createDummyHexAddress(hexChars int) string {
	if hexChars < 1 {
		return ""
	}

	buff := make([]byte, hexChars/2)
	_, _ = rand.Reader.Read(buff)

	return hex.EncodeToString(buff)
}

func convertContract(contract interface{}) []byte {
	bytes, _ := json.Marshal(contract)
	return bytes
}

func generateRandomString(size int) string {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

func getSplitRoyalties() map[string]*models.RoyaltySplitInfo {
	split := make(map[string]*models.RoyaltySplitInfo, 20)
	for i := 0; i < 20; i++ {
		split[fmt.Sprintf("%s%d", createDummyHexAddress(64), i)] = &models.RoyaltySplitInfo{
			PercentTransferPercentage: math.MaxUint32,
			PercentTransferFixed:      math.MaxUint32,
			PercentMarketPercentage:   math.MaxUint32,
			PercentMarketFixed:        math.MaxUint32,
			PercentITOPercentage:      math.MaxUint32,
			PercentITOFixed:           math.MaxUint32,
		}
	}
	return split
}

func getRoles() []*models.RolesInfo {
	roles := make([]*models.RolesInfo, core.MaxRoles)
	for i := 0; i < core.MaxRoles; i++ {
		roles = append(roles, &models.RolesInfo{
			Address: createDummyHexAddress(64),
		})
	}
	return roles
}

func getUri() map[string]string {
	uris := make(map[string]string, core.MaxURIMapSize)
	for i := 0; i < core.MaxURIMapSize; i++ {
		uris[generateRandomString(core.MaxURIKeySize)] = generateRandomString(core.MaxURIValueSize)
	}
	return uris
}

func getProposalParameteres() map[int32]string {
	proposals := make(map[int32]string, core.MaxProposalsLength)
	for i := 0; i < core.MaxProposalsLength; i++ {
		proposals[int32(i)] = generateRandomString(core.MaxProposalParamLength)
	}
	return proposals
}

func getWhitelistInfo() map[string]models.WhitelistInfoRequest {
	whitelist := make(map[string]models.WhitelistInfoRequest, core.MaxWhitelistSize)

	for i := 0; i < core.MaxWhitelistSize; i++ {
		whitelist[fmt.Sprintf("%d%s", i, createDummyHexAddress(64))] = models.WhitelistInfoRequest{
			Limit: math.MaxInt64,
		}
	}
	return whitelist
}

func getPackInfo() map[string]models.PackInfoRequest {
	packInfoMap := make(map[string]models.PackInfoRequest, core.MaxPacks)

	for i := 0; i < core.MaxPacks; i++ {
		packs := make([]models.PackItemRequest, core.MaxPackItems)
		for j := 0; j < core.MaxPackItems; j++ {
			packs = append(packs, models.PackItemRequest{
				Amount: math.MaxInt64,
				Price:  math.MaxInt64,
			})
		}

		packInfoMap[generateRandomString(64)] = models.PackInfoRequest{
			Packs: packs,
		}
	}
	return packInfoMap
}

func getUpdatePermissions() []models.PermissionTXRequest {
	permissions := make([]models.PermissionTXRequest, core.MaxAccountPermission)

	for i := 0; i < core.MaxAccountPermission; i++ {
		signers := make([]models.SignerTXRequest, core.MaxPermissionSigners)

		for j := 0; j < core.MaxPermissionSigners; j++ {
			signers = append(signers, models.SignerTXRequest{
				Address: createDummyHexAddress(64),
				Weight:  math.MaxInt64,
			})

		}

		permissions = append(permissions, models.PermissionTXRequest{
			Type:           math.MaxInt32,
			PermissionName: generateRandomString(core.MaxNameSize),
			Threshold:      math.MaxInt64,
			Operations:     generateRandomString(core.MaxOperationsSize),
			Signers:        signers,
		})
	}
	return permissions
}

func getCallVallues() map[string]int64 {
	callValues := make(map[string]int64, core.MaxCallValueSize)

	for i := 0; i < core.MaxCallValueSize; i++ {
		callValues[fmt.Sprintf("%s%d", generateRandomString(core.MaxLengthForAssetTicker), i)] = math.MaxInt64
	}

	return callValues
}
