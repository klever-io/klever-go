package transaction_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/pubkeyConverter"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var mockSender = []byte("01234567891011121314151617181920")

func createMockNodeHelper() *mock.NodesHelperMock {
	validatorPubKeyConverter, _ := pubkeyConverter.NewHexPubkeyConverter(96)
	walletPubKeyConverter, _ := pubkeyConverter.NewBech32PubkeyConverter(32)

	forkController := mock.NewForkControllerStub()

	return &mock.NodesHelperMock{
		GetAddressPCKCalled: func() core.PubkeyConverter {
			return walletPubKeyConverter
		},
		GetValidatorPCKCalled: func() core.PubkeyConverter {
			return validatorPubKeyConverter
		},
		GetEncodedAddressLengthCalled: func() int {
			return 62
		},
		GetForkControllerCalled: func() core.ForkController {
			return forkController
		},
		GetAssetCalled: func(address string) (*kapps.KDAData, error) {
			return &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferFixed: 100,
					TransferPercentage: []*kapps.RoyaltyData{
						{
							Amount:     1000,
							Percentage: 500, // 5%
						},
					},
				},
			}, nil
		},
	}
}

func TestTransaction_PrepareForProcessing(t *testing.T) {
	t.Parallel()

	tx := &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{
			Contract: []*transaction.TXContract{{
				Type: transaction.TXContract_TransferContractType,
				Parameter: &anypb.Any{
					TypeUrl: "type.googleapis.com/TransferContract",
					Value:   []byte{1, 2, 3, 4},
				},
			}},
			Data: [][]byte{{1, 2, 3, 4}},
		},
		Receipts: []*transaction.Transaction_Receipt{{
			Data: [][]byte{{1, 2, 3, 4}},
		}},
	}

	tx.PrepareForProcessing()

	assert.Empty(t, tx.Receipts)
	assert.NotEmpty(t, tx.RawData.Contract)
	assert.NotEmpty(t, tx.RawData.Data)
}

func TestTransaction_GetDataSize(t *testing.T) {
	t.Parallel()

	tx := &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{
			Data: make([][]byte, 0),
		},
	}

	bump := func() []byte {
		return make([]byte, 1000000000)
	}

	for i := 0; i < 100; i++ {
		tx.RawData.Data = append(tx.RawData.Data, bump())
	}

	size := tx.GetDataSize()
	assert.NotZero(t, size)
}

func TestTransaction_ValidatePermission(t *testing.T) {
	t.Parallel()

	validPermissions := []transaction.TXContract_ContractType{
		transaction.TXContract_TransferContractType,
		transaction.TXContract_UndelegateContractType,
		transaction.TXContract_SetAccountNameContractType,
	}

	tx := &transaction.Transaction{
		RawData: &transaction.Transaction_Raw{
			Contract: make([]*transaction.TXContract, 0),
		},
	}

	for _, ct := range validPermissions {
		tx.RawData.Contract = append(tx.RawData.Contract, &transaction.TXContract{Type: ct})
	}

	err := tx.ValidatePermissionOperation(bytes.Repeat([]byte{0xff}, 33))
	assert.Equal(t, err, common.ErrInvalidPermission)

	err = tx.ValidatePermissionOperation(nil)
	assert.Equal(t, err, common.ErrNoPermission)

	err = tx.ValidatePermissionOperation(bytes.Repeat([]byte{0xff}, 0))
	assert.Equal(t, err, common.ErrNoPermission)

	err = tx.ValidatePermissionOperation(bytes.Repeat([]byte{0xff}, 1))
	assert.Equal(t, err, common.ErrNoPermission)

	err = tx.ValidatePermissionOperation(bytes.Repeat([]byte{0xff}, core.MaxOperationsSize))
	assert.Nil(t, err)
}

func TestAddTransaction(t *testing.T) {
	tests := []struct {
		name          string
		txArgs        transaction.TXArgs
		expectedError error
	}{
		{
			name: "Valid Transfer Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_TransferContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"receiver": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap",
					"amount": 1000,
					"KDA": "KLV"
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Invalid Transfer - Empty Receiver",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_TransferContractType),
				Sender: []byte("klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap"),
				Contract: json.RawMessage(`{
					"receiver": "",
					"amount": 1000,
					"KDA": "KLV"
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: transaction.ErrInvalidReceiverAddress,
		},
		{
			name: "Valid Create Asset Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_CreateAssetContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"type": 0,
					"name": "TestAsset",
					"ticker": "TEST",
					"ownerAddress": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap",
					"precision": 6,
					"initialSupply": 1000000,
					"maxSupply": 10000000,
					"properties": {
						"canFreeze": true,
						"canWipe": true,
						"canPause": true,
						"canMint": true,
						"canBurn": true
					}
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Invalid Create Asset - Invalid Name",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_CreateAssetContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"type": 0,
					"name": "@InvalidName",
					"ticker": "TEST",
					"ownerAddress": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap"
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: common.ErrTokenNameNotHumanReadable,
		},
		{
			name: "Valid Create Validator",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_CreateValidatorContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"BLSPublicKey": "1d8cb37e902525bf8bda62b635ca240ac7c3a713250295381b3e661cb32a7cdeb64cd8f17144ca7ad2520c92dfe5330f610d18bf9b503dda86a1ba5d7071cdeb0e510bcc28e32ca8c033c493f61abf43448ea39e3215cec49e4f4ae796c13b08",
					"commission": 1000,
					"maxDelegationAmount": 1000000,
					"name": "validator1"
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Config Validator",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_ValidatorConfigContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"BLSPublicKey": "1d8cb37e902525bf8bda62b635ca240ac7c3a713250295381b3e661cb32a7cdeb64cd8f17144ca7ad2520c92dfe5330f610d18bf9b503dda86a1ba5d7071cdeb0e510bcc28e32ca8c033c493f61abf43448ea39e3215cec49e4f4ae796c13b08",
					"commission": 1000,
					"maxDelegationAmount": 1000000,
					"name": "validator1"
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Invalid Create Validator - Commission Over 100%",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_CreateValidatorContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"BLSPublicKey": "1d8cb37e902525bf8bda62b635ca240ac7c3a713250295381b3e661cb32a7cdeb64cd8f17144ca7ad2520c92dfe5330f610d18bf9b503dda86a1ba5d7071cdeb0e510bcc28e32ca8c033c493f61abf43448ea39e3215cec49e4f4ae796c13b08",
					"commission": 10001,
					"maxDelegationAmount": 1000000
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: common.ErrInvalidValue,
		},
		{
			name: "Valid Config ITO Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_ConfigITOContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"KDA": "TEST",
					"receiverAddress": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap",
					"status": 1,
					"maxAmount": 1000000,
					"defaultLimitPerAddress": 1000,
					"packInfo": {
						"pack1": {
							"packs": [
								{"amount": 100, "price": 10}
							]
						}
					},
					"whitelistStatus": 1,
					"whitelistStartTime": 1635724800,
					"whitelistEndTime": 1635811200,
					"startTime": 1635897600,
					"endTime": 1635984000
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Invalid Config ITO - Invalid Times",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_ConfigITOContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"KDA": "TEST",
					"receiverAddress": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap",
					"status": 1,
					"maxAmount": 1000000,
					"startTime": 1635984000,
					"endTime": 1635897600
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: common.ErrInvalidValue,
		},
		{
			name: "Valid Sell Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_SellContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"marketType": 1,
					"marketplaceID": "0123456789abcdef",
					"assetID": "TEST",
					"currencyID": "KLV",
					"price": 1000,
					"endTime": 1635984000
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Smart Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_SmartContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"SCType": 1,
					"address": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap",
					"callValue": {
						"KLV": 1000
					}
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Update Account Permission",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_UpdateAccountPermissionContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"permissions": [{
						"type": 1,
						"permissionName": "default",
						"threshold": 1,
						"operations": "ff",
						"signers": [{
							"address": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap",
							"weight": 1
						}]
					}]
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Invalid Update Account Permission - Invalid Operations",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_UpdateAccountPermissionContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"permissions": [{
						"type": 1,
						"permissionName": "default",
						"threshold": 1,
						"operations": "xy",
						"signers": [{
							"address": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap",
							"weight": 1
						}]
					}]
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: common.ErrInvalidValue,
		},
		{
			name: "Valid Create Asset Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_CreateAssetContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"type": 0,
					"name": "TestAsset",
					"ticker": "TEST",
					"ownerAddress": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap",
					"precision": 6,
					"initialSupply": 1000000,
					"maxSupply": 10000000,
					"properties": {
						"canFreeze": true,
						"canWipe": true,
						"canPause": true,
						"canMint": true,
						"canBurn": true
					}
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Invalid Create Asset Contract With Roles And Royalties",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_CreateAssetContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"type": 0,
					"name": "TestAsset",
					"ticker": "TEST",
					"ownerAddress": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap",
					"precision": 6,
					"initialSupply": 1000000,
					"maxSupply": 10000000,
					"properties": {
						"canFreeze": true,
						"canWipe": true,
						"canPause": true,
						"canMint": true,
						"canBurn": true
					},
					"roles": [
						{
							"address": "klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5",
							"hasRoleMint": true
						}
					],
                    "royalties": {
					  "address": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap",
					  "transferPercentage": [{
						 "amount": 10,
						 "percentage": 10
					  }],
					  "splitRoyalties": {
						"klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5": {
						  "percentTransferPercentage": 50
						}
					  }
					}
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := transaction.NewBaseTransaction(tt.txArgs.Sender, 0, nil, 0, 0)
			err := tx.AddTransaction(tt.txArgs)

			if tt.expectedError != nil {
				assert.Error(t, err)
				if tt.expectedError != common.ErrInvalidValue {
					assert.Equal(t, tt.expectedError, err)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tx.GetRawData().Contract)
				assert.Len(t, tx.GetRawData().Contract, 1)
			}
		})
	}
}

func TestAddTransactionWithInvalidJSON(t *testing.T) {
	tests := []struct {
		name          string
		contractType  uint32
		invalidJSON   string
		expectedError error
	}{
		{
			name:         "Invalid JSON Structure",
			contractType: uint32(transaction.TXContract_TransferContractType),
			invalidJSON: `{
				invalid_json
			}`,
			expectedError: common.ErrInvalidValue,
		},
		{
			name:          "Empty JSON",
			contractType:  uint32(transaction.TXContract_TransferContractType),
			invalidJSON:   ``,
			expectedError: common.ErrInvalidValue,
		},
		{
			name:         "Malformed JSON",
			contractType: uint32(transaction.TXContract_TransferContractType),
			invalidJSON: `{
				"receiver": "address",
				"amount": "not_a_number",
			}`,
			expectedError: common.ErrInvalidValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txArgs := transaction.TXArgs{
				Type:       tt.contractType,
				Sender:     mockSender,
				Contract:   json.RawMessage(tt.invalidJSON),
				NodeHelper: createMockNodeHelper(),
			}

			tx := transaction.NewBaseTransaction(txArgs.Sender, 0, nil, 0, 0)
			err := tx.AddTransaction(txArgs)
			assert.Error(t, err)
		})
	}
}

func TestAddTransactionAdditionalCases(t *testing.T) {
	tests := []struct {
		name          string
		txArgs        transaction.TXArgs
		expectedError error
	}{
		{
			name: "Valid Freeze Contract",
			txArgs: transaction.TXArgs{
				ActiveParameters: map[int32]*kapps.Parameter{
					31: {
						Value: []byte("1000000000"),
					},
				},
				Type:   uint32(transaction.TXContract_FreezeContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"Amount": 1000000000,
					"KDA": "KLV"
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Invalid Freeze Contract",
			txArgs: transaction.TXArgs{
				ActiveParameters: map[int32]*kapps.Parameter{
					31: {
						Value: []byte("1000000000"),
					},
				},
				Type:   uint32(transaction.TXContract_FreezeContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"Amount": 10000,
					"KDA": "KLV"
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: common.ErrInvalidValue,
		},
		{
			name: "Valid Unfreeze Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_UnfreezeContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"bucketID": "123456",
					"KDA": "KLV"
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Invalid Unfreeze - Empty BucketID",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_UnfreezeContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"bucketID": "",
					"KDA": "KLV"
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: errors.New("invalid bucket id"),
		},
		{
			name: "Valid Delegate Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_DelegateContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"bucketID": "123456",
					"receiver": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap"
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Invalid Delegate - Missing BucketID",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_DelegateContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"receiver": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap"
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: errors.New("invalid bucket id"),
		},
		{
			name: "Valid Undelegate Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_UndelegateContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"bucketID": "123456"
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Withdraw Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_WithdrawContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"withdrawType": 1,
					"KDA": "TEST",
					"currencyID": "KLV",
					"amount": 1000
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Claim Contract - Staking",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_ClaimContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"claimType": 1,
					"ID": "KLV"
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Claim Contract - Market",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_ClaimContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"claimType": 2,
					"ID": "0123456789abcdef"
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Unjail Contract",
			txArgs: transaction.TXArgs{
				Type:       uint32(transaction.TXContract_UnjailContractType),
				Sender:     mockSender,
				Contract:   json.RawMessage(`{}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Set Account Name Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_SetAccountNameContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"name": "testaccount"
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Proposal Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_ProposalContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"parameters": {
						"1": "100"
					},
					"description": "Test proposal",
					"epochsDuration": 10
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Vote Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_VoteContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"type": 1,
					"proposalID": 1,
					"amount": 1000
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Create Marketplace Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_CreateMarketplaceContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"name": "TestMarket",
					"referralAddress": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap",
					"referralPercentage": 500
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Config Marketplace Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_ConfigMarketplaceContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"marketplaceID": "0123456789abcdef",
					"name": "UpdatedMarket",
					"referralAddress": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap",
					"referralPercentage": 500
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid ITO Trigger Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_ITOTriggerContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"triggerType": 1,
					"KDA": "TEST",
					"status": 1,
					"whitelistInfo": {
						"klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap": {
							"limit": 1000
						}
					},
					"whitelistStartTime": 1635724800,
					"whitelistEndTime": 1635811200,
					"startTime": 1635897600,
					"endTime": 1635984000
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Asset Trigger Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_AssetTriggerContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"triggerType": 1,
					"assetID": "TEST",
					"amount": 1000,
					"value": 1
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Asset Trigger Contract - Update Royalties",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_AssetTriggerContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"triggerType": 14,
					"royalties": {
					  "address": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap",
					  "transferPercentage": [{
						 "amount": 10,
						 "percentage": 10
					  }],
					  "splitRoyalties": {
						"klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5": {
						  "percentTransferPercentage": 50
						}
					  }
					}
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Asset Trigger Contract - Add Role",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_AssetTriggerContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"triggerType": 6,
					"role": {
						"address": "klv1fpwjz234gy8aaae3gx0e8q9f52vymzzn3z5q0s5h60pvktzx0n0qwvtux5",
						"hasRoleMint": true
					}
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Invalid Asset Trigger - Negative Amount",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_AssetTriggerContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"triggerType": 1,
					"assetID": "TEST",
					"amount": -1000
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: common.ErrInvalidValue,
		},
		{
			name: "Valid Buy Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_BuyContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"buyType": 1,
					"ID": "0123456789abcdef",
					"currencyID": "KLV",
					"amount": 1000
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Valid Deposit Contract",
			txArgs: transaction.TXArgs{
				Type:   uint32(transaction.TXContract_DepositContractType),
				Sender: mockSender,
				Contract: json.RawMessage(`{
					"depositType": 1,
					"KDA": "TEST",
					"currencyID": "KLV",
					"amount": 1000
				}`),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: nil,
		},
		{
			name: "Invalid Contract Size",
			txArgs: transaction.TXArgs{
				Type:       uint32(transaction.TXContract_TransferContractType),
				Sender:     mockSender,
				Contract:   json.RawMessage(makeHugeJSON()),
				NodeHelper: createMockNodeHelper(),
			},
			expectedError: common.ErrInvalidContractSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := transaction.NewBaseTransaction(tt.txArgs.Sender, 0, nil, 0, 0)
			err := tx.AddTransaction(tt.txArgs)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tx.GetRawData().Contract)
				assert.Len(t, tx.GetRawData().Contract, 1)
			}
		})
	}
}

// Helper function to create a huge JSON for testing contract size limits
func makeHugeJSON() string {
	hugePadding := bytes.Repeat([]byte("x"), 1024*1024) // 1MB of data
	return `{"receiver": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap", "kda": "` + string(hugePadding) + `"}`
}

func TestValidatePermission(t *testing.T) {
	tx := transaction.NewBaseTransaction(
		mockSender,
		0, nil, 0, 0,
	)

	// Add a valid contract
	err := tx.AddTransaction(transaction.TXArgs{
		Type:   uint32(transaction.TXContract_TransferContractType),
		Sender: mockSender,
		Contract: json.RawMessage(`{
			"receiver": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap",
			"amount": 1000,
			"KDA": "KLV"
		}`),
		NodeHelper: createMockNodeHelper(),
	})
	assert.NoError(t, err)

	tests := []struct {
		name          string
		permission    []byte
		expectedError error
	}{
		{
			name:          "Valid Permission",
			permission:    []byte{0xFF},
			expectedError: nil,
		},
		{
			name:          "Empty Permission",
			permission:    []byte{},
			expectedError: common.ErrNoPermission,
		},
		{
			name:          "Permission Too Long",
			permission:    bytes.Repeat([]byte{0xFF}, core.MaxOperationsSize+1),
			expectedError: common.ErrInvalidPermission,
		},
		{
			name:          "No Permission for Contract Type",
			permission:    []byte{0x00},
			expectedError: common.ErrNoPermission,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tx.ValidatePermissionOperation(tt.permission)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestAddTransaction_ShouldFail_NilNodeHelper(t *testing.T) {
	addr := makeAddress("testAddr")
	tx := transaction.NewBaseTransaction(addr, 0, nil, 0, 0)

	txArgs := transaction.TXArgs{
		Contract: json.RawMessage(`{
			"receiver": "klv17e8zzgn73h6ehe3c6q9vlt77kuxk5euddmhymy5uhv2rhv0dc0nqlfp0ap",
			"amount": 1000,
			"kda": "KLV"
		}`),
	}

	err := tx.AddTransaction(txArgs)
	require.Equal(t, common.ErrNilNodeHelper, err)
}

func TestAddTransaction_FreezeContract(t *testing.T) {
	addr := makeAddress("testAddr")
	txType := transaction.TXContract_FreezeContractType
	minKLVBucketAmount := "1000000000" // 1_000_000_000 = 1000 KLV
	assetID := "KLV"

	t.Run("should fail with nil active parameters", func(t *testing.T) {
		t.Parallel()

		tx := transaction.NewBaseTransaction(addr, 0, nil, 0, 0)

		contract := fmt.Sprintf("{\"kda\": \"%s\"}", assetID)
		txArgs := transaction.TXArgs{
			Type:       uint32(txType),
			Contract:   json.RawMessage(contract),
			NodeHelper: createMockNodeHelper(),
		}

		err := tx.AddTransaction(txArgs)
		require.Equal(t, transaction.ErrNilActiveParameters, err)
	})

	t.Run("should fail with min KLV bucket amount not found", func(t *testing.T) {
		t.Parallel()

		tx := transaction.NewBaseTransaction(addr, 0, nil, 0, 0)

		contract := fmt.Sprintf("{\"kda\": \"%s\"}", assetID)

		activeParameters := map[int32]*kapps.Parameter{}
		txArgs := transaction.TXArgs{
			Type:             uint32(txType),
			Contract:         json.RawMessage(contract),
			ActiveParameters: activeParameters,
			NodeHelper:       createMockNodeHelper(),
		}

		err := tx.AddTransaction(txArgs)
		require.Equal(t, transaction.ErrMinKLVBucketAmountNotFound, err)
	})

	t.Run("should fail with invalid amount", func(t *testing.T) {
		t.Parallel()

		tx := transaction.NewBaseTransaction(addr, 0, nil, 0, 0)

		activeParameters := map[int32]*kapps.Parameter{
			int32(kapps.EnumParameter_MinKLVBucketAmount): {
				Type:  kapps.EnumType_Int64,
				Value: []byte(minKLVBucketAmount),
			},
		}

		contract := fmt.Sprintf("{\"amount\": %d, \"kda\": \"%s\"}", 0, assetID)
		txArgs := transaction.TXArgs{
			Type:             uint32(txType),
			Contract:         json.RawMessage(contract),
			ActiveParameters: activeParameters,
			NodeHelper:       createMockNodeHelper(),
		}

		err := tx.AddTransaction(txArgs)
		require.Equal(t, common.ErrInvalidValue, err)
	})

	t.Run("should work", func(t *testing.T) {
		t.Parallel()

		tx := transaction.NewBaseTransaction(addr, 0, nil, 0, 0)

		activeParameters := map[int32]*kapps.Parameter{
			int32(kapps.EnumParameter_MinKLVBucketAmount): {
				Type:  kapps.EnumType_Int64,
				Value: []byte(minKLVBucketAmount),
			},
		}

		contract := fmt.Sprintf("{\"amount\": %s, \"kda\": \"%s\"}", minKLVBucketAmount, assetID)
		txArgs := transaction.TXArgs{
			Type:             uint32(txType),
			Contract:         json.RawMessage(contract),
			ActiveParameters: activeParameters,
			NodeHelper:       createMockNodeHelper(),
		}

		err := tx.AddTransaction(txArgs)
		require.Nil(t, err)

		// get last contract added, which should be the freeze contract
		require.Len(t, tx.GetContracts(), 1)
		freezeContract := tx.GetContracts()[len(tx.GetContracts())-1]

		require.Equal(t, txType, freezeContract.Type)

		parsedContract := &transaction.FreezeContract{}
		err = anypb.UnmarshalTo(freezeContract.Parameter, parsedContract, proto.UnmarshalOptions{})
		require.Nil(t, err)

		require.Equal(t, []byte(assetID), parsedContract.AssetID)

		expectedBucketAmount, err := strconv.ParseInt(minKLVBucketAmount, 10, 64)
		require.Nil(t, err)

		require.EqualValues(t, expectedBucketAmount, parsedContract.Amount)
	})
}
