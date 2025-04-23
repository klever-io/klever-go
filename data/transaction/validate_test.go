package transaction_test

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
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
				CallValue: getCallValues(),
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

			result := transaction.IsContractSizeValid(tt.contract, tt.contractType)
			require.Equal(tt.expectedResult, result)
		})
	}

}

func TestIsContractSizeValid_CheckAllContractsExists(t *testing.T) {
	for name, id := range transaction.TXContract_ContractType_value {
		contractType := transaction.TXContract_ContractType(id)
		// dummy contract
		contract := make([]byte, 1)
		require.True(t, transaction.IsContractSizeValid(contract, contractType), name)
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

func createDummyAddress(hexChars int) []byte {
	if hexChars < 1 {
		return []byte{}
	}

	buff := make([]byte, hexChars/2)
	_, _ = rand.Reader.Read(buff)

	return buff
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

	for i := range core.MaxWhitelistSize {
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

func getCallValues() map[string]int64 {
	callValues := make(map[string]int64, core.MaxCallValueSize)

	for i := 0; i < core.MaxCallValueSize; i++ {
		callValues[fmt.Sprintf("%s%d", generateRandomString(core.MaxLengthForAssetTicker), i)] = math.MaxInt64
	}

	return callValues
}

func makeAddress(addr string) []byte {
	result := make([]byte, core.PubKeyLen)
	copy(result, []byte(addr))

	return result
}

func makeMap(size int) map[string]string {
	m := make(map[string]string, size)
	for i := 0; i < size; i++ {
		m[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}
	return m
}

func makePackInfo(sizeInfo int, sizeItems int) map[string]*transaction.PackInfo {
	packInfo := make(map[string]*transaction.PackInfo, sizeInfo)
	for i := 0; i < sizeInfo; i++ {
		packs := make([]*transaction.PackItem, sizeItems)
		for j := 0; j < sizeItems; j++ {
			packs[j] = &transaction.PackItem{
				Amount: math.MaxInt64,
				Price:  math.MaxInt64,
			}
		}

		packInfo[generateRandomString(64)] = &transaction.PackInfo{
			Packs: packs,
		}
	}
	return packInfo
}

func makeSplitRoyalties(size int) map[string]*transaction.RoyaltySplitInfo {
	split := make(map[string]*transaction.RoyaltySplitInfo, size)
	for i := 0; i < size; i++ {
		split[fmt.Sprintf("address%d", i)] = &transaction.RoyaltySplitInfo{
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

func TestTransferContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.TransferContract
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid transfer contract",
			contract: &transaction.TransferContract{
				ToAddress: makeAddress("validAddress"),
			},
			expectError: false,
		},
		{
			name: "invalid zero address",
			contract: &transaction.TransferContract{
				ToAddress: core.ZeroAddress,
			},
			expectError: true,
			errorMsg:    "invalid receiver address",
		},
		{
			name: "nil address",
			contract: &transaction.TransferContract{
				ToAddress: nil,
			},
			expectError: true,
			errorMsg:    "invalid receiver address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(nil)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateAssetContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.CreateAssetContract
		fc          core.ForkController
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid fungible asset",
			contract: &transaction.CreateAssetContract{
				Name:      []byte("ValidName"),
				Ticker:    []byte("VLD"),
				Type:      transaction.CreateAssetContract_Fungible,
				Precision: 6,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "invalid asset name post fork with space",
			contract: &transaction.CreateAssetContract{
				Name:      []byte("Valid Name"),
				Ticker:    []byte("VLD"),
				Type:      transaction.CreateAssetContract_Fungible,
				Precision: 6,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid name",
		},
		{
			name: "valid asset name pre fork with space",
			contract: &transaction.CreateAssetContract{
				Name:      []byte("Valid Name"),
				Ticker:    []byte("VLD"),
				Type:      transaction.CreateAssetContract_Fungible,
				Precision: 6,
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "invalid name",
			contract: &transaction.CreateAssetContract{
				Name:   []byte("Invalid\x00Name"),
				Ticker: []byte("VLD"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid name",
		},
		{
			name: "invalid ticker",
			contract: &transaction.CreateAssetContract{
				Name:   []byte("ValidName"),
				Ticker: []byte("invalid-ticker"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid ticker",
		},
		{
			name: "invalid logo URI size",
			contract: &transaction.CreateAssetContract{
				Name:   []byte("ValidName"),
				Ticker: []byte("VLD"),
				Logo:   string(make([]byte, core.MaxLogoURISize+1)),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid logo",
		},
		{
			name: "invalid precision for fungible",
			contract: &transaction.CreateAssetContract{
				Name:      []byte("ValidName"),
				Ticker:    []byte("VLD"),
				Type:      transaction.CreateAssetContract_Fungible,
				Precision: core.MaxNumberOfDecimals + 1,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid precision",
		},
		{
			name: "invalid URIs map size",
			contract: &transaction.CreateAssetContract{
				Name:   []byte("ValidName"),
				Ticker: []byte("VLD"),
				URIs:   makeMap(core.MaxURIMapSize + 1),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid uri",
		},
		{
			name: "invalid transfer royalties length",
			contract: &transaction.CreateAssetContract{
				Name:   []byte("ValidName"),
				Ticker: []byte("VLD"),
				Royalties: &transaction.RoyaltiesInfo{
					TransferPercentage: make([]*transaction.RoyaltyInfo, core.MaxTransferRoyalties+1),
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid transfer royalties length",
		},
		{
			name: "invalid split royalties length",
			contract: &transaction.CreateAssetContract{
				Name:   []byte("ValidName"),
				Ticker: []byte("VLD"),
				Royalties: &transaction.RoyaltiesInfo{
					SplitRoyalties: makeSplitRoyalties(core.MaxTransferRoyalties + 1),
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid split royalties length",
		},
		{
			name: "invalid roles length",
			contract: &transaction.CreateAssetContract{
				Name:   []byte("ValidName"),
				Ticker: []byte("VLD"),
				Roles:  make([]*transaction.RolesInfo, core.MaxRoles+1),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid roles length",
		},
		{
			name: "invalid URI key size",
			contract: &transaction.CreateAssetContract{
				Name:   []byte("ValidName"),
				Ticker: []byte("VLD"),
				URIs:   map[string]string{generateRandomString(core.MaxURIKeySize + 1): "value"},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid uri",
		},
		{
			name: "invalid URI value size",
			contract: &transaction.CreateAssetContract{
				Name:   []byte("ValidName"),
				Ticker: []byte("VLD"),
				URIs:   map[string]string{generateRandomString(core.MaxURIKeySize): generateRandomString(core.MaxURIValueSize + 1)},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid uri",
		},
		{
			name: "create non-fungible asset with invalid precision",
			contract: &transaction.CreateAssetContract{
				Name:      []byte("ValidName"),
				Ticker:    []byte("VLD"),
				Type:      transaction.CreateAssetContract_NonFungible,
				Precision: 6,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid parameter",
		},
		{
			name: "create non-fungible asset with invalid initial supply",
			contract: &transaction.CreateAssetContract{
				Name:          []byte("ValidName"),
				Ticker:        []byte("VLD"),
				Type:          transaction.CreateAssetContract_NonFungible,
				InitialSupply: 100,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid parameter",
		},
		{
			name: "create semi-fungible asset with invalid precision",
			contract: &transaction.CreateAssetContract{
				Name:      []byte("ValidName"),
				Ticker:    []byte("VLD"),
				Type:      transaction.CreateAssetContract_SemiFungible,
				Precision: core.MaxNumberOfDecimals + 1,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid precision",
		},
		{
			name: "create invalid asset type",
			contract: &transaction.CreateAssetContract{
				Name:   []byte("ValidName"),
				Ticker: []byte("VLD"),
				Type:   999,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid asset type",
		},
		{
			name: "valid owner address pre fork",
			contract: &transaction.CreateAssetContract{
				Name:         []byte("ValidName"),
				Ticker:       []byte("VLD"),
				Type:         transaction.CreateAssetContract_Fungible,
				Precision:    6,
				OwnerAddress: []byte("address"),
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "invalid owner address pos fork",
			contract: &transaction.CreateAssetContract{
				Name:         []byte("ValidName"),
				Ticker:       []byte("VLD"),
				Type:         transaction.CreateAssetContract_Fungible,
				Precision:    6,
				OwnerAddress: []byte("address"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid owner address",
		},
		{
			name: "valid owner address pos fork",
			contract: &transaction.CreateAssetContract{
				Name:         []byte("ValidName"),
				Ticker:       []byte("VLD"),
				Type:         transaction.CreateAssetContract_Fungible,
				Precision:    6,
				OwnerAddress: core.ZeroAddress,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "valid owner address empty pos fork",
			contract: &transaction.CreateAssetContract{
				Name:         []byte("ValidName"),
				Ticker:       []byte("VLD"),
				Type:         transaction.CreateAssetContract_Fungible,
				Precision:    6,
				OwnerAddress: []byte{},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "valid admin address pre fork",
			contract: &transaction.CreateAssetContract{
				Name:         []byte("ValidName"),
				Ticker:       []byte("VLD"),
				Type:         transaction.CreateAssetContract_Fungible,
				Precision:    6,
				AdminAddress: []byte("address"),
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "invalid admin address pos fork",
			contract: &transaction.CreateAssetContract{
				Name:         []byte("ValidName"),
				Ticker:       []byte("VLD"),
				Type:         transaction.CreateAssetContract_Fungible,
				Precision:    6,
				AdminAddress: []byte("address"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid admin address",
		},
		{
			name: "valid admin address pos fork",
			contract: &transaction.CreateAssetContract{
				Name:         []byte("ValidName"),
				Ticker:       []byte("VLD"),
				Type:         transaction.CreateAssetContract_Fungible,
				Precision:    6,
				AdminAddress: core.ZeroAddress,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "valid admin address empty pos fork",
			contract: &transaction.CreateAssetContract{
				Name:         []byte("ValidName"),
				Ticker:       []byte("VLD"),
				Type:         transaction.CreateAssetContract_Fungible,
				Precision:    6,
				AdminAddress: []byte{},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(tt.fc)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAssetTriggerContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.AssetTriggerContract
		expectError bool
		fc          core.ForkController
		errorMsg    string
	}{
		{
			name: "valid pause trigger",
			contract: &transaction.AssetTriggerContract{
				TriggerType: transaction.AssetTriggerContract_Pause,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "invalid trigger type",
			contract: &transaction.AssetTriggerContract{
				TriggerType: 999,
			},
			expectError: true,
			fc:          mock.NewForkControllerStub(),
			errorMsg:    "invalid trigger type",
		},
		{
			name: "mint trigger with invalid fields",
			contract: &transaction.AssetTriggerContract{
				TriggerType: transaction.AssetTriggerContract_Mint,
				Logo:        "invalid_logo_field", // Logo not allowed for Mint
			},
			expectError: true,
			fc:          mock.NewForkControllerStub(),
			errorMsg:    "invalid contract fields",
		},
		{
			name: "update logo with invalid size",
			contract: &transaction.AssetTriggerContract{
				TriggerType: transaction.AssetTriggerContract_UpdateLogo,
				Logo:        string(make([]byte, core.MaxLogoURISize+1)),
			},
			expectError: true,
			fc:          mock.NewForkControllerStub(),
			errorMsg:    "invalid logo",
		},
		{
			name: "update URIs with invalid size",
			contract: &transaction.AssetTriggerContract{
				TriggerType: transaction.AssetTriggerContract_UpdateURIs,
				URIs:        makeMap(core.MaxURIMapSize + 1),
			},
			expectError: true,
			fc:          mock.NewForkControllerStub(),
			errorMsg:    "invalid uri",
		},
		{
			name: "valid to address pre fork",
			contract: &transaction.AssetTriggerContract{
				TriggerType: transaction.AssetTriggerContract_Mint,
				ToAddress:   []byte("address"),
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "invalid to address pos fork",
			contract: &transaction.AssetTriggerContract{
				TriggerType: transaction.AssetTriggerContract_Mint,
				ToAddress:   []byte("address"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid address",
		},
		{
			name: "valid to address pos fork",
			contract: &transaction.AssetTriggerContract{
				TriggerType: transaction.AssetTriggerContract_Mint,
				ToAddress:   core.ZeroAddress,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "valid to address empty pos fork",
			contract: &transaction.AssetTriggerContract{
				TriggerType: transaction.AssetTriggerContract_Mint,
				ToAddress:   []byte{},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "valid amount pre fork",
			contract: &transaction.AssetTriggerContract{
				TriggerType: transaction.AssetTriggerContract_Wipe,
				Amount:      -1,
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "invalid amount pos fork",
			contract: &transaction.AssetTriggerContract{
				TriggerType: transaction.AssetTriggerContract_Wipe,
				Amount:      -1,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid amount",
		},
		{
			name: "valid amount pos fork",
			contract: &transaction.AssetTriggerContract{
				TriggerType: transaction.AssetTriggerContract_Wipe,
				Amount:      10,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "valid amount empty pos fork",
			contract: &transaction.AssetTriggerContract{
				TriggerType: transaction.AssetTriggerContract_Wipe,
				Amount:      0,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(tt.fc)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnfreezeContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.UnfreezeContract
		expectError bool
		fc          core.ForkController
		errorMsg    string
	}{
		{
			name: "valid unfreeze contract",
			contract: &transaction.UnfreezeContract{
				AssetID:  []byte("KLV"),
				BucketID: make([]byte, core.BucketIDSize),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "invalid asset id",
			contract: &transaction.UnfreezeContract{
				AssetID:  []byte("A"),
				BucketID: make([]byte, core.BucketIDSize),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid asset id",
		},
		{
			name: "invalid bucket id",
			contract: &transaction.UnfreezeContract{
				AssetID:  []byte("KLV"),
				BucketID: make([]byte, 0),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid bucket id",
		},
		{
			// Nil assetID should validate as KLV
			name: "invalid bucket id for KLV",
			contract: &transaction.UnfreezeContract{
				BucketID: []byte("ASSET-1234"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid bucket id",
		},
		{
			// Nil assetID should validate as KLV
			name: "valid bucket for KLV",
			contract: &transaction.UnfreezeContract{
				BucketID: make([]byte, core.BucketIDSize),
			},
			fc: mock.NewForkControllerStub(),
		},
		{
			name: "invalid bucket id for KFI",
			contract: &transaction.UnfreezeContract{
				AssetID:  []byte("KFI"),
				BucketID: []byte("ASSET-1234"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid bucket id",
		},
		{
			// Nil assetID should validate as KLV
			name: "valid bucket for KFI",
			contract: &transaction.UnfreezeContract{
				AssetID:  []byte("KFI"),
				BucketID: make([]byte, core.BucketIDSize),
			},
			fc: mock.NewForkControllerStub(),
		},
		{
			name: "valid bucket id pre fork",
			contract: &transaction.UnfreezeContract{
				AssetID:  []byte("KLV"),
				BucketID: make([]byte, 3),
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "invalid bucket pos fork",
			contract: &transaction.UnfreezeContract{
				AssetID:  []byte("KLV"),
				BucketID: make([]byte, 3),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid bucket id",
		},
		{
			name: "invalid bucket pos fork",
			contract: &transaction.UnfreezeContract{
				AssetID:  []byte("ASSET-1234"),
				BucketID: make([]byte, 3),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid bucket id",
		},
		{
			name: "valid bucket pos fork",
			contract: &transaction.UnfreezeContract{
				AssetID:  []byte("KLV"),
				BucketID: make([]byte, core.BucketIDSize),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "valid bucket pos fork",
			contract: &transaction.UnfreezeContract{
				AssetID:  []byte("ASSET-1234"),
				BucketID: []byte("ASSET-1234"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(tt.fc)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUpdateAccountPermissionContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.UpdateAccountPermissionContract
		expectError bool
		fc          core.ForkController
		errorMsg    string
	}{
		{
			name: "valid update account permission",
			contract: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type:      transaction.AccPermission_Owner,
						Threshold: 1,
						Signers: []*transaction.AccKey{
							{
								Address: core.ZeroAddress,
								Weight:  1,
							},
						},
					},
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "invalid signer address",
			contract: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type:      transaction.AccPermission_Owner,
						Threshold: 1,
						Signers: []*transaction.AccKey{
							{
								Address: []byte("1"),
								Weight:  1,
							},
						},
					},
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid signer address",
		},
		{
			name: "valid signer address pre fork",
			contract: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type:      transaction.AccPermission_Owner,
						Threshold: 1,
						Signers: []*transaction.AccKey{
							{
								Address: []byte("1"),
								Weight:  1,
							},
						},
					},
				},
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fork core.ForkController = mock.NewForkControllerStub()
			if tt.fc != nil {
				fork = tt.fc
			}

			err := tt.contract.Validate(fork)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfigMarketplaceContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.ConfigMarketplaceContract
		expectError bool
		fc          core.ForkController
		errorMsg    string
	}{
		{
			name: "valid config marketplace",
			contract: &transaction.ConfigMarketplaceContract{
				ReferralAddress: core.ZeroAddress,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "invalid config marketplace",
			contract: &transaction.ConfigMarketplaceContract{
				ReferralAddress: []byte("invalid"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid referral address",
		},
		{
			name: "valid config marketplace pre fork",
			contract: &transaction.ConfigMarketplaceContract{
				ReferralAddress: []byte("invalid"),
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fork core.ForkController = mock.NewForkControllerStub()
			if tt.fc != nil {
				fork = tt.fc
			}

			err := tt.contract.Validate(fork)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestITOTriggerContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.ITOTriggerContract
		expectError bool
		fc          core.ForkController
		errorMsg    string
	}{
		{
			name: "valid ito config",
			contract: &transaction.ITOTriggerContract{
				TriggerType:     transaction.ITOTriggerContract_UpdateReceiverAddress,
				AssetID:         []byte("KLV"),
				ReceiverAddress: core.ZeroAddress,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "invalid asset ito config",
			contract: &transaction.ITOTriggerContract{
				AssetID:         []byte("I"),
				TriggerType:     transaction.ITOTriggerContract_UpdateReceiverAddress,
				ReceiverAddress: core.ZeroAddress,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid asset id",
		},
		{
			name: "invalid receiver ito config",
			contract: &transaction.ITOTriggerContract{
				TriggerType:     transaction.ITOTriggerContract_UpdateReceiverAddress,
				AssetID:         []byte("KLV"),
				ReceiverAddress: []byte("invalid"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid receiver address",
		},
		{
			name: "valid receiver ito config pre fork",
			contract: &transaction.ITOTriggerContract{
				TriggerType:     transaction.ITOTriggerContract_UpdateReceiverAddress,
				AssetID:         []byte("KLV"),
				ReceiverAddress: []byte("invalid"),
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "invalid whitelist address ito config",
			contract: &transaction.ITOTriggerContract{
				AssetID:     []byte("KLV"),
				TriggerType: transaction.ITOTriggerContract_AddToWhitelist,
				WhitelistInfo: map[string]*transaction.WhitelistInfo{
					"1": {},
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid whitelist address",
		},
		{
			name: "valid whitelist address ito config",
			contract: &transaction.ITOTriggerContract{
				AssetID:     []byte("KLV"),
				TriggerType: transaction.ITOTriggerContract_AddToWhitelist,
				WhitelistInfo: map[string]*transaction.WhitelistInfo{
					hex.EncodeToString(core.ZeroAddress): {},
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "valid whitelist address ito config pre fork",
			contract: &transaction.ITOTriggerContract{
				AssetID:     []byte("KLV"),
				TriggerType: transaction.ITOTriggerContract_AddToWhitelist,
				WhitelistInfo: map[string]*transaction.WhitelistInfo{
					"1": {},
				},
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "valid whitelist size",
			contract: &transaction.ITOTriggerContract{
				AssetID:       []byte("KLV"),
				TriggerType:   transaction.ITOTriggerContract_AddToWhitelist,
				WhitelistInfo: createValidWhitelist(core.MaxWhitelistSize),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "exceeded whitelist size",
			contract: &transaction.ITOTriggerContract{
				AssetID:       []byte("KLV"),
				TriggerType:   transaction.ITOTriggerContract_AddToWhitelist,
				WhitelistInfo: createValidWhitelist(core.MaxWhitelistSize + 1),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid whitelist size",
		},
		{
			name: "whitelist size validation skipped pre-fork",
			contract: &transaction.ITOTriggerContract{
				AssetID:       []byte("KLV"),
				TriggerType:   transaction.ITOTriggerContract_AddToWhitelist,
				WhitelistInfo: createValidWhitelist(core.MaxWhitelistSize + 1),
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fork core.ForkController = mock.NewForkControllerStub()
			if tt.fc != nil {
				fork = tt.fc
			}

			err := tt.contract.Validate(fork)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDelegateContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.DelegateContract
		expectError bool
		fc          core.ForkController
		errorMsg    string
	}{
		{
			name: "valid delegate contract",
			contract: &transaction.DelegateContract{
				ToAddress: core.ZeroAddress,
				BucketID:  make([]byte, 64),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "invalid bucket id",
			contract: &transaction.DelegateContract{
				ToAddress: core.ZeroAddress,
				BucketID:  make([]byte, 0),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid bucket id",
		},
		{
			name: "invalid to address",
			contract: &transaction.DelegateContract{
				ToAddress: []byte("invalid"),
				BucketID:  make([]byte, 1),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid toAddress",
		},
		{
			name: "valid address pre fork",
			contract: &transaction.DelegateContract{
				ToAddress: []byte("invalid"),
				BucketID:  make([]byte, 1),
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(tt.fc)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestProposalContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.ProposalContract
		expectError bool
		fc          core.ForkController
		errorMsg    string
	}{
		{
			name: "valid proposal contract",
			contract: &transaction.ProposalContract{
				Parameters: map[int32][]byte{
					0: []byte{0x01},
				},
				Description:    []byte("description"),
				EpochsDuration: 0,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "invalid proposal contract - description too long",
			contract: &transaction.ProposalContract{
				Parameters: map[int32][]byte{
					0: []byte{0x01},
				},
				Description:    make([]byte, 2000),
				EpochsDuration: 0,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    transaction.ErrInvalidDescription.Error(),
		},
		{
			name: "invalid proposal contract - parameter too long",
			contract: &transaction.ProposalContract{
				Parameters: map[int32][]byte{
					0: []byte{255, 255, 255, 255, 255, 255, 255},
				},
				EpochsDuration: 0,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    transaction.ErrInvalidParameter.Error(),
		},
		{
			name: "invalid proposal contract - too many parameters",
			contract: &transaction.ProposalContract{
				Parameters: map[int32][]byte{
					0:  []byte{0x01},
					1:  []byte{0x01},
					2:  []byte{0x01},
					3:  []byte{0x01},
					4:  []byte{0x01},
					5:  []byte{0x01},
					6:  []byte{0x01},
					7:  []byte{0x01},
					8:  []byte{0x01},
					9:  []byte{0x01},
					10: []byte{0x01},
				},
				Description:    make([]byte, 2000),
				EpochsDuration: 0,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    transaction.ErrInvalidParameter.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(tt.fc)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfigValidatorContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.ValidatorConfigContract
		expectError bool
		fc          core.ForkController
		errorMsg    string
	}{
		{
			name: "valid config contract",
			contract: &transaction.ValidatorConfigContract{
				Config: &transaction.ValidatorConfig{
					BLSPublicKey:        createDummyAddress(96),
					RewardAddress:       createDummyAddress(64),
					CanDelegate:         true,
					Commission:          10,
					MaxDelegationAmount: 100000000,
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "invalid config contract - nil config",
			contract: &transaction.ValidatorConfigContract{
				Config: nil,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    transaction.ErrInvalidConfig.Error(),
		},
		{
			name: "invalid config contract - invalid logo",
			contract: &transaction.ValidatorConfigContract{
				Config: &transaction.ValidatorConfig{
					BLSPublicKey:        createDummyAddress(96),
					RewardAddress:       createDummyAddress(64),
					CanDelegate:         true,
					Commission:          10,
					Logo:                strings.Repeat("a", 1000),
					MaxDelegationAmount: 100000000,
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    transaction.ErrInvalidLogo.Error(),
		},
		{
			name: "invalid config contract - invalid name",
			contract: &transaction.ValidatorConfigContract{
				Config: &transaction.ValidatorConfig{
					BLSPublicKey:        createDummyAddress(96),
					RewardAddress:       createDummyAddress(64),
					CanDelegate:         true,
					Commission:          10,
					Name:                strings.Repeat("a", 1000),
					MaxDelegationAmount: 100000000,
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    transaction.ErrInvalidName.Error(),
		},
		{
			name: "invalid config contract - invalid URI len",
			contract: &transaction.ValidatorConfigContract{
				Config: &transaction.ValidatorConfig{
					BLSPublicKey:  createDummyAddress(96),
					RewardAddress: createDummyAddress(64),
					CanDelegate:   true,
					Commission:    10,
					URIs: map[string]string{
						"0":  "1",
						"1":  "1",
						"2":  "1",
						"3":  "1",
						"4":  "1",
						"5":  "1",
						"6":  "1",
						"7":  "1",
						"8":  "1",
						"9":  "1",
						"10": "1",
					},
					MaxDelegationAmount: 100000000,
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    transaction.ErrInvalidURI.Error(),
		},
		{
			name: "invalid config contract - invalid URI",
			contract: &transaction.ValidatorConfigContract{
				Config: &transaction.ValidatorConfig{
					BLSPublicKey:  createDummyAddress(96),
					RewardAddress: createDummyAddress(64),
					CanDelegate:   true,
					Commission:    10,
					URIs: map[string]string{
						"invalidURI": strings.Repeat("a", 300),
					},
					MaxDelegationAmount: 100000000,
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    transaction.ErrInvalidURI.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(tt.fc)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateValidatorContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.CreateValidatorContract
		expectError bool
		fc          core.ForkController
		errorMsg    string
	}{
		{
			name: "valid create validator contract",
			contract: &transaction.CreateValidatorContract{
				Config: &transaction.ValidatorConfig{
					BLSPublicKey:        createDummyAddress(96),
					RewardAddress:       createDummyAddress(64),
					CanDelegate:         true,
					Commission:          10,
					MaxDelegationAmount: 100000000,
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "invalid create validator contract - nil config",
			contract: &transaction.CreateValidatorContract{
				Config: nil,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    transaction.ErrInvalidConfig.Error(),
		},
		{
			name: "invalid create validator contract - invalid logo",
			contract: &transaction.CreateValidatorContract{
				Config: &transaction.ValidatorConfig{
					BLSPublicKey:        createDummyAddress(96),
					RewardAddress:       createDummyAddress(64),
					CanDelegate:         true,
					Commission:          10,
					Logo:                strings.Repeat("a", 1000),
					MaxDelegationAmount: 100000000,
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    transaction.ErrInvalidLogo.Error(),
		},
		{
			name: "invalid create validator contract - invalid name",
			contract: &transaction.CreateValidatorContract{
				Config: &transaction.ValidatorConfig{
					BLSPublicKey:        createDummyAddress(96),
					RewardAddress:       createDummyAddress(64),
					CanDelegate:         true,
					Commission:          10,
					Name:                strings.Repeat("a", 1000),
					MaxDelegationAmount: 100000000,
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    transaction.ErrInvalidName.Error(),
		},
		{
			name: "invalid create validator contract - invalid URI len",
			contract: &transaction.CreateValidatorContract{
				Config: &transaction.ValidatorConfig{
					BLSPublicKey:  createDummyAddress(96),
					RewardAddress: createDummyAddress(64),
					CanDelegate:   true,
					Commission:    10,
					URIs: map[string]string{
						"0":  "1",
						"1":  "1",
						"2":  "1",
						"3":  "1",
						"4":  "1",
						"5":  "1",
						"6":  "1",
						"7":  "1",
						"8":  "1",
						"9":  "1",
						"10": "1",
					},
					MaxDelegationAmount: 100000000,
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    transaction.ErrInvalidURI.Error(),
		},
		{
			name: "invalid create validator contract - invalid URI",
			contract: &transaction.CreateValidatorContract{
				Config: &transaction.ValidatorConfig{
					BLSPublicKey:  createDummyAddress(96),
					RewardAddress: createDummyAddress(64),
					CanDelegate:   true,
					Commission:    10,
					URIs: map[string]string{
						"invalidURI": strings.Repeat("a", 300),
					},
					MaxDelegationAmount: 100000000,
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    transaction.ErrInvalidURI.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(tt.fc)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWithdrawContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.WithdrawContract
		expectError bool
		fc          *mock.ForkControllerStub
		errorMsg    string
	}{
		{
			name: "valid staking withdraw",
			contract: &transaction.WithdrawContract{
				WithdrawType: transaction.WithdrawContract_Staking,
				AssetID:      []byte("VALID"),
			},
			expectError: false,
		},
		{
			name: "invalid asset ID length",
			contract: &transaction.WithdrawContract{
				WithdrawType: transaction.WithdrawContract_Staking,
				AssetID:      []byte("IN"),
			},
			expectError: true,
			errorMsg:    "invalid asset id",
		},
		{
			name: "invalid asset ID length",
			contract: &transaction.WithdrawContract{
				WithdrawType: transaction.WithdrawContract_Staking,
				AssetID:      []byte("10123456789-1234"),
			},
			expectError: true,
			errorMsg:    "invalid asset id",
		},
		{
			name: "valid asset ID length pre fork",
			contract: &transaction.WithdrawContract{
				WithdrawType: transaction.WithdrawContract_Staking,
				AssetID:      []byte("10123456789-1234"),
			},
			expectError: false,
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
		},
		{
			name: "invalid withdraw type",
			contract: &transaction.WithdrawContract{
				WithdrawType: 999,
			},
			expectError: true,
			errorMsg:    "invalid withdraw type",
		},
		{
			name: "KDAPool withdraw with invalid amount",
			contract: &transaction.WithdrawContract{
				WithdrawType: transaction.WithdrawContract_KDAPool,
				CurrencyID:   []byte("VALID"),
				Amount:       0,
			},
			expectError: true,
			errorMsg:    "invalid amount",
		},
		{
			name: "KDAPool withdraw with invalid currency ID",
			contract: &transaction.WithdrawContract{
				WithdrawType: transaction.WithdrawContract_KDAPool,
				CurrencyID:   []byte("IN"),
				Amount:       100,
			},
			expectError: true,
			errorMsg:    "invalid currency id",
		},
		{
			name: "KDAPool withdraw with invalid currency ID",
			contract: &transaction.WithdrawContract{
				WithdrawType: transaction.WithdrawContract_KDAPool,
				CurrencyID:   []byte("10123456789-1234"),
				Amount:       100,
			},
			expectError: true,
			errorMsg:    "invalid currency id",
		},
		{
			name: "KDAPool withdraw with valid currency ID pre fork",
			contract: &transaction.WithdrawContract{
				WithdrawType: transaction.WithdrawContract_KDAPool,
				CurrencyID:   []byte("10123456789-1234"),
				Amount:       100,
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := mock.NewForkControllerStub()
			if tt.fc != nil {
				fc = tt.fc
			}

			err := tt.contract.Validate(fc)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSmartContractValidate(t *testing.T) {

	makeSCCallMap := func(size int) map[string]*transaction.CallValue {
		m := make(map[string]*transaction.CallValue, size)
		for i := 0; i < size; i++ {
			m[fmt.Sprintf("key%d", i)] = &transaction.CallValue{
				Amount: math.MaxInt64,
			}
		}
		return m
	}

	tests := []struct {
		name        string
		contract    *transaction.SmartContract
		expectError bool
		fc          *mock.ForkControllerStub
		errorMsg    string
	}{
		{
			name: "valid deploy contract",
			contract: &transaction.SmartContract{
				Type:    transaction.SmartContract_SCDeploy,
				Address: nil,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "deploy with invalid address",
			contract: &transaction.SmartContract{
				Type:    transaction.SmartContract_SCDeploy,
				Address: []byte("invalid"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid contract address",
		},
		{
			name: "invalid call value size",
			contract: &transaction.SmartContract{
				Type:      transaction.SmartContract_SCDeploy,
				CallValue: makeSCCallMap(core.MaxCallValueSize + 1),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid call value",
		},
		{
			name: "non-deploy with invalid address length",
			contract: &transaction.SmartContract{
				Type:    transaction.SmartContract_SCInvoke,
				Address: []byte("invalid"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid contract address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(nil)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestVoteContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.VoteContract
		fc          *mock.ForkControllerStub
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid vote with smart contracts enabled",
			contract: &transaction.VoteContract{
				Type:       1,
				ProposalID: 1,
				Amount:     1000,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "invalid proposal ID with smart contracts enabled",
			contract: &transaction.VoteContract{
				Type:       1,
				ProposalID: 0,
				Amount:     1000,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid proposal id",
		},
		{
			name: "proposal ID validation skipped when smart contracts disabled",
			contract: &transaction.VoteContract{
				Type:       1,
				ProposalID: 0,
				Amount:     1000,
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "zero amount",
			contract: &transaction.VoteContract{
				Type:       1,
				ProposalID: 1,
				Amount:     0,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid amount",
		},
		{
			name: "negative amount",
			contract: &transaction.VoteContract{
				Type:       1,
				ProposalID: 1,
				Amount:     -100,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid amount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(tt.fc)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDepositContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.DepositContract
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid FPR deposit",
			contract: &transaction.DepositContract{
				DepositType: transaction.DepositContract_FPRDeposit,
				ID:          []byte("VALID_ID"),
				CurrencyID:  []byte("VALID_CURR"),
				Amount:      1000,
			},
			expectError: false,
		},
		{
			name: "valid KDA pool deposit",
			contract: &transaction.DepositContract{
				DepositType: transaction.DepositContract_KDAPool,
				ID:          []byte("VALID_ID"),
				CurrencyID:  []byte("VALID_CURR"),
				Amount:      1000,
			},
			expectError: false,
		},
		{
			name: "invalid deposit type",
			contract: &transaction.DepositContract{
				DepositType: 999,
				ID:          []byte("VALID_ID"),
				CurrencyID:  []byte("VALID_CURR"),
				Amount:      1000,
			},
			expectError: true,
			errorMsg:    "invalid deposit type",
		},
		{
			name: "invalid ID length",
			contract: &transaction.DepositContract{
				DepositType: transaction.DepositContract_FPRDeposit,
				ID:          []byte("ID"),
				CurrencyID:  []byte("VALID_CURR"),
				Amount:      1000,
			},
			expectError: true,
			errorMsg:    "invalid id",
		},
		{
			name: "invalid currency ID length",
			contract: &transaction.DepositContract{
				DepositType: transaction.DepositContract_KDAPool,
				ID:          []byte("VALID_ID"),
				CurrencyID:  []byte("ID"),
				Amount:      1000,
			},
			expectError: true,
			errorMsg:    "invalid currency id",
		},
		{
			name: "zero amount",
			contract: &transaction.DepositContract{
				DepositType: transaction.DepositContract_FPRDeposit,
				ID:          []byte("VALID_ID"),
				CurrencyID:  []byte("VALID_CURR"),
				Amount:      0,
			},
			expectError: true,
			errorMsg:    "invalid amount",
		},
		{
			name: "negative amount",
			contract: &transaction.DepositContract{
				DepositType: transaction.DepositContract_KDAPool,
				ID:          []byte("VALID_ID"),
				CurrencyID:  []byte("VALID_CURR"),
				Amount:      -100,
			},
			expectError: true,
			errorMsg:    "invalid amount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(mock.NewForkControllerStub())
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSellContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.SellContract
		fc          *mock.ForkControllerStub
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid sell with smart contracts enabled",
			contract: &transaction.SellContract{
				MarketplaceID: []byte("MARKETPLACE"),
				AssetID:       []byte("ASSET"),
				Price:         1000,
				EndTime:       1000,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "missing marketplace ID with smart contracts enabled",
			contract: &transaction.SellContract{
				AssetID: []byte("ASSET"),
				Price:   1000,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid marketplace id",
		},
		{
			name: "missing asset ID - should return default KLV",
			contract: &transaction.SellContract{
				MarketplaceID: []byte("MARKETPLACE"),
				Price:         1000,
			},
			fc: mock.NewForkControllerStub(),
		},
		{
			name: "valid sell with smart contracts disabled",
			contract: &transaction.SellContract{
				Price:   1000,
				EndTime: 1000,
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "negative price",
			contract: &transaction.SellContract{
				MarketplaceID: []byte("MARKETPLACE"),
				AssetID:       []byte("ASSET"),
				Price:         -100,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid price",
		},
		{
			name: "negative reserve price",
			contract: &transaction.SellContract{
				MarketplaceID: []byte("MARKETPLACE"),
				AssetID:       []byte("ASSET"),
				Price:         1000,
				ReservePrice:  -100,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid reserve price",
		},
		{
			name: "negative end time",
			contract: &transaction.SellContract{
				MarketplaceID: []byte("MARKETPLACE"),
				AssetID:       []byte("ASSET"),
				Price:         1000,
				EndTime:       -100,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid end time",
		},
		{
			name: "valid currency id pre fork",
			contract: &transaction.SellContract{
				MarketplaceID: []byte("MARKETPLACE"),
				AssetID:       []byte("ASSET"),
				CurrencyID:    []byte("I"),
				Price:         1000,
				EndTime:       1000,
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "invalid currency id",
			contract: &transaction.SellContract{
				MarketplaceID: []byte("MARKETPLACE"),
				AssetID:       []byte("ASSET"),
				CurrencyID:    []byte("I"),
				Price:         1000,
				EndTime:       1000,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid currency id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(tt.fc)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClaimContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.ClaimContract
		fc          *mock.ForkControllerStub
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid staking claim",
			contract: &transaction.ClaimContract{
				ClaimType: transaction.ClaimContract_StakingClaim,
				ID:        []byte("VALID_ID"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "valid allowance claim with KLV",
			contract: &transaction.ClaimContract{
				ClaimType: transaction.ClaimContract_AllowanceClaim,
				ID:        []byte("KLV"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "invalid allowance claim with non-KLV asset",
			contract: &transaction.ClaimContract{
				ClaimType: transaction.ClaimContract_AllowanceClaim,
				ID:        []byte("INVALID"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid asset id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(tt.fc)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBuyContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.BuyContract
		fc          *mock.ForkControllerStub
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid buy with smart contracts",
			contract: &transaction.BuyContract{
				ID:     []byte("VALID_ID"),
				Amount: 1000,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "invalid currency id",
			contract: &transaction.BuyContract{
				ID:         []byte("VALID_ID"),
				CurrencyID: []byte("I"),
				Amount:     1000,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid currency id",
		},
		{
			name: "valid currency id pre fork",
			contract: &transaction.BuyContract{
				ID:         []byte("VALID_ID"),
				CurrencyID: []byte("I"),
				Amount:     1000,
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "valid currency id",
			contract: &transaction.BuyContract{
				ID:         []byte("VALID_ID"),
				CurrencyID: []byte("KLV"),
				Amount:     1000,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "missing ID with smart contracts",
			contract: &transaction.BuyContract{
				Amount: 1000,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid pack id",
		},
		{
			name: "missing ID with smart contracts disabled",
			contract: &transaction.BuyContract{
				Amount: 1000,
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "negative amount",
			contract: &transaction.BuyContract{
				ID:     []byte("VALID_ID"),
				Amount: -100,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid amount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(tt.fc)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Helper function to create a valid whitelist of specified size
func createValidWhitelist(size int) map[string]*transaction.WhitelistInfo {
	whitelist := make(map[string]*transaction.WhitelistInfo, size)

	for i := 0; i < size; i++ {
		addr := fmt.Sprintf("%064d", i) // Create a valid hex address of correct length
		whitelist[addr] = &transaction.WhitelistInfo{
			Limit: math.MaxInt64,
		}
	}

	return whitelist
}

func TestConfigITOContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.ConfigITOContract
		expectError bool
		fc          core.ForkController
		errorMsg    string
	}{
		{
			name: "valid config with no pack info",
			contract: &transaction.ConfigITOContract{
				Status: 1,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "invalid receiver address",
			contract: &transaction.ConfigITOContract{
				ReceiverAddress: []byte("invalid"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid receiver address",
		},
		{
			name: "valid receiver address",
			contract: &transaction.ConfigITOContract{
				ReceiverAddress: []byte{},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "valid asset id pre fork",
			contract: &transaction.ConfigITOContract{
				ReceiverAddress: []byte("invalid"),
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "invalid asset id",
			contract: &transaction.ConfigITOContract{
				AssetID: []byte("i"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid asset id",
		},
		{
			name: "valid asset id",
			contract: &transaction.ConfigITOContract{
				AssetID: []byte("KLV"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "valid asset id pre fork",
			contract: &transaction.ConfigITOContract{
				AssetID: []byte("i"),
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "valid config with pack info",
			contract: &transaction.ConfigITOContract{
				PackInfo: map[string]*transaction.PackInfo{
					"pack1": {
						Packs: make([]*transaction.PackItem, core.MaxPackItems-1),
					},
				},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "too many packs",
			contract: &transaction.ConfigITOContract{
				PackInfo: makePackInfo(core.MaxPacks+1, 1),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid packs size",
		},
		{
			name: "too many pack items",
			contract: &transaction.ConfigITOContract{
				PackInfo: makePackInfo(1, core.MaxPackItems+1),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid pack items size",
		},
		{
			name: "valid whitelist size",
			contract: &transaction.ConfigITOContract{
				WhitelistInfo: createValidWhitelist(core.MaxWhitelistSize),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "exceeded whitelist size",
			contract: &transaction.ConfigITOContract{
				WhitelistInfo: createValidWhitelist(core.MaxWhitelistSize + 1),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid whitelist size",
		},
		{
			name: "whitelist size validation skipped pre-fork",
			contract: &transaction.ConfigITOContract{
				WhitelistInfo: createValidWhitelist(core.MaxWhitelistSize + 1),
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(tt.fc)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCancelMarketOrderContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.CancelMarketOrderContract
		fc          *mock.ForkControllerStub
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid cancel with order ID",
			contract: &transaction.CancelMarketOrderContract{
				OrderID: []byte("VALID_ORDER_ID"),
			},
			fc:          mock.NewForkControllerStub(),
			expectError: false,
		},
		{
			name: "missing order ID with smart contracts",
			contract: &transaction.CancelMarketOrderContract{
				OrderID: nil,
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid order id",
		},
		{
			name: "empty order ID with smart contracts",
			contract: &transaction.CancelMarketOrderContract{
				OrderID: []byte{},
			},
			fc:          mock.NewForkControllerStub(),
			expectError: true,
			errorMsg:    "invalid order id",
		},
		{
			name: "missing order ID with smart contracts disabled",
			contract: &transaction.CancelMarketOrderContract{
				OrderID: nil,
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(tt.fc)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSetAccountNameContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.SetAccountNameContract
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid name",
			contract: &transaction.SetAccountNameContract{
				Name: []byte("ValidAccountName"),
			},
			expectError: false,
		},
		{
			name: "name too long",
			contract: &transaction.SetAccountNameContract{
				Name: make([]byte, core.MaxNameSize+1),
			},
			expectError: true,
			errorMsg:    "invalid name",
		},
		{
			name: "invalid UTF-8 name",
			contract: &transaction.SetAccountNameContract{
				Name: []byte{0xFF, 0xFF},
			},
			expectError: true,
			errorMsg:    "invalid name",
		},
		{
			name: "empty name",
			contract: &transaction.SetAccountNameContract{
				Name: []byte{},
			},
			expectError: false,
		},
		{
			name: "nil name",
			contract: &transaction.SetAccountNameContract{
				Name: nil,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(mock.NewForkControllerStub())
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateMarketplaceContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.CreateMarketplaceContract
		expectError bool
		fc          core.ForkController
		errorMsg    string
	}{
		{
			name: "valid marketplace",
			contract: &transaction.CreateMarketplaceContract{
				Name:               []byte("ValidMarketplace"),
				ReferralAddress:    core.ZeroAddress,
				ReferralPercentage: 10,
			},
			expectError: false,
		},
		{
			name: "invalid referral address",
			contract: &transaction.CreateMarketplaceContract{
				Name:               []byte("ValidMarketplace"),
				ReferralAddress:    []byte("ValidAddress"),
				ReferralPercentage: 10,
			},
			expectError: true,
			errorMsg:    "invalid referral address",
		},
		{
			name: "valid marketplace pre fork",
			contract: &transaction.CreateMarketplaceContract{
				Name:               []byte("ValidMarketplace"),
				ReferralAddress:    []byte("ValidAddress"),
				ReferralPercentage: 10,
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "nil name",
			contract: &transaction.CreateMarketplaceContract{
				ReferralAddress:    []byte("ValidAddress"),
				ReferralPercentage: 10,
			},
			expectError: true,
			errorMsg:    "invalid name",
		},
		{
			name: "name too long",
			contract: &transaction.CreateMarketplaceContract{
				Name:               make([]byte, core.MaxNameSize+1),
				ReferralAddress:    []byte("ValidAddress"),
				ReferralPercentage: 10,
			},
			expectError: true,
			errorMsg:    "invalid name",
		},
		{
			name: "invalid UTF-8 name",
			contract: &transaction.CreateMarketplaceContract{
				Name:               []byte{0xFF, 0xFF},
				ReferralAddress:    []byte("ValidAddress"),
				ReferralPercentage: 10,
			},
			expectError: true,
			errorMsg:    "invalid name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fork core.ForkController = mock.NewForkControllerStub()
			if tt.fc != nil {
				fork = tt.fc
			}

			err := tt.contract.Validate(fork)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSetITOPricesContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.SetITOPricesContract
		expectError bool
		fc          core.ForkController
		errorMsg    string
	}{
		{
			name: "valid empty pack info",
			contract: &transaction.SetITOPricesContract{
				PackInfo: make(map[string]*transaction.PackInfo),
			},
			expectError: false,
		},
		{
			name: "valid asset pack info",
			contract: &transaction.SetITOPricesContract{
				PackInfo: make(map[string]*transaction.PackInfo),
				AssetID:  []byte("KLV"),
			},
			expectError: false,
		},
		{
			name: "invalid asset pack info",
			contract: &transaction.SetITOPricesContract{
				PackInfo: make(map[string]*transaction.PackInfo),
				AssetID:  []byte("I"),
			},
			expectError: true,
			errorMsg:    "invalid asset id",
		},
		{
			name: "valid asset pack info pre fork",
			contract: &transaction.SetITOPricesContract{
				PackInfo: make(map[string]*transaction.PackInfo),
				AssetID:  []byte("I"),
			},
			fc:          mock.NewForkControllerStub().SetFork("EnableSmartContracts", false),
			expectError: false,
		},
		{
			name: "valid pack info within limits",
			contract: &transaction.SetITOPricesContract{
				PackInfo: makePackInfo(1, core.MaxPackItems-1),
			},
			expectError: false,
		},
		{
			name: "too many packs",
			contract: &transaction.SetITOPricesContract{
				PackInfo: makePackInfo(core.MaxPacks+1, 1),
			},
			expectError: true,
			errorMsg:    "invalid packs size",
		},
		{
			name: "too many pack items",
			contract: &transaction.SetITOPricesContract{
				PackInfo: makePackInfo(1, core.MaxPackItems+1),
			},
			expectError: true,
			errorMsg:    "invalid pack items size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fork core.ForkController = mock.NewForkControllerStub()
			if tt.fc != nil {
				fork = tt.fc
			}

			err := tt.contract.Validate(fork)
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnjailContractValidate(t *testing.T) {
	t.Run("always valid", func(t *testing.T) {
		contract := &transaction.UnjailContract{}
		err := contract.Validate(mock.NewForkControllerStub())
		require.NoError(t, err)
	})
}

func TestTransactionValidate(t *testing.T) {
	tests := []struct {
		name        string
		tx          *transaction.Transaction
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid transaction without contracts",
			tx: &transaction.Transaction{
				RawData: &transaction.Transaction_Raw{
					Contract: make([]*transaction.TXContract, 0),
				},
			},
			expectError: false,
		},
		{
			name: "valid transaction with valid contract",
			tx: &transaction.Transaction{
				RawData: &transaction.Transaction_Raw{
					Contract: []*transaction.TXContract{
						{
							Type: transaction.TXContract_TransferContractType,
							Parameter: marshalAnyPB(&transaction.TransferContract{
								ToAddress: makeAddress("ValidAddress"),
							}),
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid contract type",
			tx: &transaction.Transaction{
				RawData: &transaction.Transaction_Raw{
					Contract: []*transaction.TXContract{
						{
							Type:      999,
							Parameter: marshalAnyPB(&transaction.TransferContract{}),
						},
					},
				},
			},
			expectError: true,
			errorMsg:    common.ErrInvalidTransactionType.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tx.Validate(mock.NewForkControllerStub())
			if tt.expectError {
				require.Error(t, err)
				require.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func marshalAnyPB(pb proto.Message) *anypb.Any {
	result, _ := anypb.New(pb)
	return result
}

func TestTXContractValidate(t *testing.T) {
	tests := []struct {
		name        string
		contract    *transaction.TXContract
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid transfer contract",
			contract: &transaction.TXContract{
				Type: transaction.TXContract_TransferContractType,
				Parameter: marshalAnyPB(&transaction.TransferContract{
					ToAddress: makeAddress("ValidAddress"),
				}),
			},
			expectError: false,
		},
		{
			name: "valid freeze contract",
			contract: &transaction.TXContract{
				Type: transaction.TXContract_FreezeContractType,
				Parameter: marshalAnyPB(&transaction.FreezeContract{
					Amount: 1000,
				}),
			},
			expectError: false,
		},
		{
			name: "invavalid freeze contract",
			contract: &transaction.TXContract{
				Type: transaction.TXContract_FreezeContractType,
				Parameter: marshalAnyPB(&transaction.FreezeContract{
					AssetID: []byte("10123456789-1234"),
					Amount:  1000,
				}),
			},
			expectError: true,
			errorMsg:    "invalid asset id",
		},
		{
			name: "invalid contract type",
			contract: &transaction.TXContract{
				Type: 999,
			},
			expectError: true,
			errorMsg:    common.ErrInvalidTransactionType.Error(),
		},
		{
			name: "nil parameter",
			contract: &transaction.TXContract{
				Type: transaction.TXContract_TransferContractType,
			},
			expectError: true,
			errorMsg:    common.ErrInvalidTransactionType.Error(),
		},
		{
			name: "invalid parameter",
			contract: &transaction.TXContract{
				Type:      transaction.TXContract_TransferContractType,
				Parameter: marshalAnyPB(&transaction.FreezeContract{}),
			},
			expectError: true,
			errorMsg:    "mismatched message type:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.contract.Validate(mock.NewForkControllerStub())
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTypeURLValidator_IsValidTypeURL(t *testing.T) {
	tests := []struct {
		name         string
		typeURL      string
		contractType transaction.TXContract_ContractType
		want         bool
	}{
		{
			name:         "empty URL",
			typeURL:      "",
			contractType: transaction.TXContract_TransferContractType,
			want:         true,
		},
		{
			name:         "valid URL with proto prefix",
			typeURL:      "type.googleapis.com/proto.TransferContract",
			contractType: transaction.TXContract_TransferContractType,
			want:         true,
		},
		{
			name:         "valid URL with type prefix",
			typeURL:      "type.googleapis.com/TransferContract",
			contractType: transaction.TXContract_TransferContractType,
			want:         true,
		},
		{
			name:         "valid URL without prefix",
			typeURL:      "TransferContract",
			contractType: transaction.TXContract_TransferContractType,
			want:         true,
		},
		{
			name:         "invalid URL",
			typeURL:      "InvalidType",
			contractType: transaction.TXContract_TransferContractType,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transaction.IsValidTypeURL(tt.typeURL, tt.contractType); got != tt.want {
				t.Errorf("IsValidTypeURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
