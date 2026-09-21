package accounts

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	integrationMock "github.com/klever-io/klever-go/integrationTest/mock"
	"github.com/klever-io/klever-go/kapps"
	kvmStub "github.com/klever-io/klever-go/kvm/mock/stub"
)

//////////////
// Unfreeze //
//////////////

var (
	testBucketID = []byte("TEST-BUCKET")
	txSender     = []byte("testAddress")
)

func TestUnfreeze(t *testing.T) {
	var (
		errAccNotFound  = errors.New("Account not found")
		errGetKda       = errors.New("Error getting KDA")
		errClaimRewards = errors.New("Error claim asset")
		errGetStaking   = errors.New("Error getting staking")
		errGetUserKda   = errors.New("Error getting KDA")
		errAccUnfreeze  = errors.New("Error unfreeze")
		errUndelegate   = errors.New("Error undelegate")
		errSetUserKda   = errors.New("Error setting user KDA")
		errUpdateUser   = errors.New("Error updating user account")
		errSetStaking   = errors.New("Error setting KDA staking")
		errUpdateKapp   = errors.New("Error updating Kapp")
		errSetKDA       = errors.New("Error updating KDA Kapp")
		errGetProposal  = errors.New("Error getting proposal")

		kdaKappAddrBytes      = []byte("KDAKappAddress")
		proposalKappAddrBytes = []byte("proposalKappAddress")

		gainsMap = map[string]int64{
			"ABC-123": 0,
			"DEF-456": 10,
			"GHI-789": 20,
		}
	)

	cases := []struct {
		title             string
		forkController    core.ForkController
		accountsCacher    state.AccountsCacher
		kappController    kapp.KAppController
		expectedErr       error
		expectedTxResCode transaction.Transaction_TXResultCode
		unfreezeTx        *transaction.UnfreezeContract
	}{
		{
			title:          "Failing to retrieve user account",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, errAccNotFound
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block:          &block.Block{},
					})
				},
			},
			expectedErr:       errAccNotFound,
			expectedTxResCode: transaction.Transaction_LoadAccountError,
			unfreezeTx:        &transaction.UnfreezeContract{},
		},
		{
			title:          "Failing to retrieve KDA data",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.AccountWrapMock{}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, nil, errGetKda
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block:          &block.Block{},
					})
				},
			},
			expectedErr:       errGetKda,
			expectedTxResCode: transaction.Transaction_KAPPError,
			unfreezeTx:        &transaction.UnfreezeContract{},
		},
		{
			title:          "Failing to retrieve KDA data due to is non fungible",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.AccountWrapMock{}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{AssetType: kapps.KDAData_NonFungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							return nil, nil, nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block:          &block.Block{},
					})
				},
			},
			expectedErr:       common.ErrAssetTypeInvalid,
			expectedTxResCode: transaction.Transaction_AssetError,
			unfreezeTx:        &transaction.UnfreezeContract{},
		},
		{
			title:          "Failing to retrieve staking data",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.AccountWrapMock{}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							return nil, nil, errGetStaking
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block:          &block.Block{},
					})
				},
			},
			expectedErr:       errGetStaking,
			expectedTxResCode: transaction.Transaction_AssetError,
			unfreezeTx:        &transaction.UnfreezeContract{},
		},
		{
			title:          "Failing to retrieve user kda",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, errGetUserKda
						},
					}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							return nil, nil, nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block:          &block.Block{},
					})
				},
			},
			expectedErr:       errGetUserKda,
			expectedTxResCode: transaction.Transaction_AssetError,
			unfreezeTx:        &transaction.UnfreezeContract{},
		},
		{
			title:          "Failing to claim rewards",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return nil, errClaimRewards
						},
					}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							return nil, nil, nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			expectedErr:       errClaimRewards,
			expectedTxResCode: transaction.Transaction_ClaimError,
			unfreezeTx:        &transaction.UnfreezeContract{},
		},
		{
			title:          "Account unfreeze fail",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return nil, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 0, errAccUnfreeze
						},
					}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							return nil, nil, nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			expectedErr:       errAccUnfreeze,
			expectedTxResCode: transaction.Transaction_UnfreezeError,
			unfreezeTx:        &transaction.UnfreezeContract{},
		},
		{
			title:          "Validator undelegate fail",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return nil, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return []byte("delegationAddress"), 0, nil
						},
					}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							return nil, nil, nil
						},
					}
				},
				GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
					return &commonMock.ValidatorsKAppStub{
						UndelegateCalled: func(blockEpoch uint32, validator []byte, sender []byte, tc *transaction.UndelegateContract) (transaction.Transaction_TXResultCode, error) {
							return transaction.Transaction_Fail, errUndelegate
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			expectedErr:       errUndelegate,
			expectedTxResCode: transaction.Transaction_Fail,
			unfreezeTx:        &transaction.UnfreezeContract{},
		},
		{
			title: "Undelegate bucket fail",
			forkController: &integrationMock.ForkControllerStub{
				EnableSmartContractsCalled: func() bool { return true },
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return nil, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return []byte("delegationAddress"), 0, nil
						},
						UndelegateCalled: func(bucketID []byte, userKDA *kapps.UserKDA) ([]byte, int64, error) {
							return nil, 0, errUndelegate
						},
					}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							return nil, nil, nil
						},
					}
				},
				GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
					return &commonMock.ValidatorsKAppStub{
						UndelegateCalled: func(blockEpoch uint32, validator []byte, sender []byte, tc *transaction.UndelegateContract) (transaction.Transaction_TXResultCode, error) {
							return transaction.Transaction_Ok, nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			expectedErr:       errUndelegate,
			expectedTxResCode: transaction.Transaction_UndelegateError,
			unfreezeTx: &transaction.UnfreezeContract{
				BucketID: testBucketID,
			},
		},
		{
			title:          "Failing to set user KDA",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return nil, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 0, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return errSetUserKda
						},
					}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							return nil, nil, nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			expectedErr:       errSetUserKda,
			expectedTxResCode: transaction.Transaction_AssetError,
			unfreezeTx:        &transaction.UnfreezeContract{},
		},
		{
			title:          "Failing to set user KDA",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return nil, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 0, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return errUpdateUser
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							return nil, nil, nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			expectedErr:       errUpdateUser,
			expectedTxResCode: transaction.Transaction_SaveAccountError,
			unfreezeTx:        &transaction.UnfreezeContract{},
		},
		{
			title:          "Failing to set staking",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return nil, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 0, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							return nil, nil, nil
						},
						SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
							return errSetStaking
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			expectedErr:       errSetStaking,
			expectedTxResCode: transaction.Transaction_SetStakingErr,
			unfreezeTx:        &transaction.UnfreezeContract{},
		},
		{
			title:          "Failing to update staking kapp",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return nil, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 0, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				UpdateKappCalled: func(account state.AccountHandler) error {
					return errUpdateKapp
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							return nil, nil, nil
						},
						SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
							return nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			expectedErr:       errUpdateKapp,
			expectedTxResCode: transaction.Transaction_SaveAccountError,
			unfreezeTx:        &transaction.UnfreezeContract{},
		},
		{
			title:          "Failing to set kda",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return nil, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 0, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				UpdateKappCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							return nil, nil, nil
						},
						SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
							return nil
						},
						SetKDACalled: func(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
							return errSetKDA
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			expectedErr:       errSetKDA,
			expectedTxResCode: transaction.Transaction_KAPPError,
			unfreezeTx:        &transaction.UnfreezeContract{},
		},
		{
			title:          "Failing to update kda kapp",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return gainsMap, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 100, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return txSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				UpdateKappCalled: func(account state.AccountHandler) error {
					if bytes.Equal(account.AddressBytes(), kdaKappAddrBytes) {
						return errUpdateKapp
					}
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							kdaKapp, _ := state.NewKAppAccount(kdaKappAddrBytes)
							return kdaKapp, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							stakingKapp, _ := state.NewKAppAccount([]byte("stakingKappAddress"))
							return stakingKapp, &kapps.StakingData{
								MinEpochsToClaim: 1,
								InterestType:     kapps.StakingData_APRI,
							}, nil
						},
						SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
							return nil
						},
						SetKDACalled: func(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
							return nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			expectedErr:       errUpdateKapp,
			expectedTxResCode: transaction.Transaction_SaveAccountError,
			unfreezeTx:        &transaction.UnfreezeContract{},
		},
		{
			title:          "Failing to retrieve proposal kapp",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return gainsMap, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 100, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return txSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				UpdateKappCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							kdaKapp, _ := state.NewKAppAccount(kdaKappAddrBytes)
							return kdaKapp, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							stakingKapp, _ := state.NewKAppAccount([]byte("stakingKappAddress"))
							return stakingKapp, &kapps.StakingData{
								MinEpochsToClaim: 1,
								InterestType:     kapps.StakingData_APRI,
							}, nil
						},
						SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
							return nil
						},
						SetKDACalled: func(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
							return nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
				GetProposalKAppCalled: func() kapp.ProposalKapp {
					return &commonMock.ProposalKappStub{
						GetProposalCalled: func(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
							return nil, nil, nil, errGetProposal
						},
					}
				},
			},
			expectedErr:       errGetProposal,
			expectedTxResCode: transaction.Transaction_AccountError,
			unfreezeTx: &transaction.UnfreezeContract{
				AssetID: kdautils.KFIIdentifier,
			},
		},
		{
			title:          "Failing to retrieve user kda during proposals processing",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					callCount := 0
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							callCount++
							if callCount == 1 {
								return &kapps.UserKDA{}, nil
							}
							return nil, errGetUserKda
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return gainsMap, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 100, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return txSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				UpdateKappCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							kdaKapp, _ := state.NewKAppAccount(kdaKappAddrBytes)
							return kdaKapp, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							stakingKapp, _ := state.NewKAppAccount([]byte("stakingKappAddress"))
							return stakingKapp, &kapps.StakingData{
								MinEpochsToClaim: 1,
								InterestType:     kapps.StakingData_APRI,
							}, nil
						},
						SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
							return nil
						},
						SetKDACalled: func(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
							return nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
				GetProposalKAppCalled: func() kapp.ProposalKapp {
					return &commonMock.ProposalKappStub{
						GetProposalCalled: func(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
							return nil, nil, nil, nil
						},
					}
				},
			},
			expectedErr:       errGetUserKda,
			expectedTxResCode: transaction.Transaction_AccountError,
			unfreezeTx: &transaction.UnfreezeContract{
				AssetID: kdautils.KFIIdentifier,
			},
		},
		{
			title:          "Failing to retrieve proposal during its processing",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return gainsMap, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 100, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return txSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				UpdateKappCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							kdaKapp, _ := state.NewKAppAccount(kdaKappAddrBytes)
							return kdaKapp, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							stakingKapp, _ := state.NewKAppAccount([]byte("stakingKappAddress"))
							return stakingKapp, &kapps.StakingData{
								MinEpochsToClaim: 1,
								InterestType:     kapps.StakingData_APRI,
							}, nil
						},
						SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
							return nil
						},
						SetKDACalled: func(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
							return nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
				GetProposalKAppCalled: func() kapp.ProposalKapp {
					return &commonMock.ProposalKappStub{
						GetProposalCalled: func(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
							if proposalID == 0 {
								return nil, nil, &kapps.ProposalController{
									ActiveProposals: map[uint32]*kapps.ActiveProposals{
										1: {
											ProposalIDs: []uint64{1},
										},
									},
								}, nil
							}
							return nil, nil, nil, errGetProposal
						},
					}
				},
			},
			expectedErr:       errGetProposal,
			expectedTxResCode: transaction.Transaction_AccountError,
			unfreezeTx: &transaction.UnfreezeContract{
				AssetID: kdautils.KFIIdentifier,
			},
		},
		{
			title:          "Finishes successful without changing proposal due to user is not unfreeze KFI",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return gainsMap, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 100, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return txSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				UpdateKappCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							kdaKapp, _ := state.NewKAppAccount(kdaKappAddrBytes)
							return kdaKapp, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							stakingKapp, _ := state.NewKAppAccount([]byte("stakingKappAddress"))
							return stakingKapp, &kapps.StakingData{
								MinEpochsToClaim: 1,
								InterestType:     kapps.StakingData_APRI,
							}, nil
						},
						SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
							return nil
						},
						SetKDACalled: func(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
							return nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
				GetProposalKAppCalled: func() kapp.ProposalKapp {
					return &commonMock.ProposalKappStub{
						GetProposalCalled: func(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
							return nil, nil, &kapps.ProposalController{
								ActiveProposals: map[uint32]*kapps.ActiveProposals{
									1: {
										ProposalIDs: []uint64{1},
									},
								},
							}, nil
						},
					}
				},
			},
			expectedErr:       nil,
			expectedTxResCode: transaction.Transaction_Ok,
			unfreezeTx: &transaction.UnfreezeContract{
				AssetID: []byte("TEST_12AB"),
			},
		},
		{
			title:          "Finishes successful without update proposal due to user has not voted",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return gainsMap, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 100, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return txSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				UpdateKappCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							kdaKapp, _ := state.NewKAppAccount(kdaKappAddrBytes)
							return kdaKapp, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							stakingKapp, _ := state.NewKAppAccount([]byte("stakingKappAddress"))
							return stakingKapp, &kapps.StakingData{
								MinEpochsToClaim: 1,
								InterestType:     kapps.StakingData_APRI,
							}, nil
						},
						SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
							return nil
						},
						SetKDACalled: func(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
							return nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
				GetProposalKAppCalled: func() kapp.ProposalKapp {
					return &commonMock.ProposalKappStub{
						GetProposalCalled: func(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
							if proposalID == 0 {
								return nil, nil, &kapps.ProposalController{
									ActiveProposals: map[uint32]*kapps.ActiveProposals{
										1: {
											ProposalIDs: []uint64{1},
										},
									},
								}, nil
							}
							return nil, &kapps.ProposalData{
								Voters: map[string]*kapps.ProposalData_VoteDetail{
									"randomAddress": {},
								},
							}, nil, nil
						},
					}
				},
			},
			expectedErr:       nil,
			expectedTxResCode: transaction.Transaction_Ok,
			unfreezeTx: &transaction.UnfreezeContract{
				AssetID: kdautils.KFIIdentifier,
			},
		},
		{
			title:          "Finishes successful without update proposal due to user KFI frozen balance still higher than vote amount",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return &kapps.UserKDA{FrozenBalance: 100}, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return gainsMap, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 100, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return txSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				UpdateKappCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							kdaKapp, _ := state.NewKAppAccount(kdaKappAddrBytes)
							return kdaKapp, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							stakingKapp, _ := state.NewKAppAccount([]byte("stakingKappAddress"))
							return stakingKapp, &kapps.StakingData{
								MinEpochsToClaim: 1,
								InterestType:     kapps.StakingData_APRI,
							}, nil
						},
						SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
							return nil
						},
						SetKDACalled: func(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
							return nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
				GetProposalKAppCalled: func() kapp.ProposalKapp {
					return &commonMock.ProposalKappStub{
						GetProposalCalled: func(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
							if proposalID == 0 {
								return nil, nil, &kapps.ProposalController{
									ActiveProposals: map[uint32]*kapps.ActiveProposals{
										1: {
											ProposalIDs: []uint64{1},
										},
									},
								}, nil
							}
							return nil, &kapps.ProposalData{
								Voters: map[string]*kapps.ProposalData_VoteDetail{
									hex.EncodeToString(txSender): {Amount: 90},
								},
							}, nil, nil
						},
					}
				},
			},
			expectedErr:       nil,
			expectedTxResCode: transaction.Transaction_Ok,
			unfreezeTx: &transaction.UnfreezeContract{
				AssetID: kdautils.KFIIdentifier,
			},
		},
		{
			title: "On unfreeze KFI failing to update proposal total staked",
			forkController: &integrationMock.ForkControllerStub{
				EnableSmartContractsCalled: func() bool { return true },
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return &kapps.UserKDA{FrozenBalance: 100}, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return gainsMap, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 100, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return txSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				UpdateKappCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					getStakingCallCount := 0
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							kdaKapp, _ := state.NewKAppAccount(kdaKappAddrBytes)
							return kdaKapp, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							getStakingCallCount++
							if getStakingCallCount == 1 {
								stakingKapp, _ := state.NewKAppAccount(kdautils.KFIIdentifier)
								return stakingKapp, &kapps.StakingData{
									MinEpochsToClaim: 1,
									InterestType:     kapps.StakingData_APRI,
								}, nil
							}
							return nil, nil, errGetStaking
						},
						SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
							return nil
						},
						SetKDACalled: func(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
							return nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
				GetProposalKAppCalled: func() kapp.ProposalKapp {
					return &commonMock.ProposalKappStub{
						GetProposalCalled: func(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
							if proposalID == 0 {
								return nil, nil, &kapps.ProposalController{
									ActiveProposals: map[uint32]*kapps.ActiveProposals{
										1: {
											ProposalIDs: []uint64{1},
										},
									},
								}, nil
							}
							return nil, &kapps.ProposalData{
								Voters: map[string]*kapps.ProposalData_VoteDetail{
									hex.EncodeToString(txSender): {
										Amount: 101,
										Type:   kapps.ProposalData_VoteDetail_No,
									},
								},
								Votes: map[int32]int64{
									int32(kapps.ProposalData_VoteDetail_No): 200,
								},
							}, nil, nil
						},
					}
				},
			},
			expectedErr:       nil,
			expectedTxResCode: transaction.Transaction_Ok,
			unfreezeTx: &transaction.UnfreezeContract{
				AssetID: kdautils.KFIIdentifier,
			},
		},
		{
			title:          "Failing on update proposal kapp",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return gainsMap, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 100, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return txSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				UpdateKappCalled: func(account state.AccountHandler) error {
					if bytes.Equal(account.AddressBytes(), proposalKappAddrBytes) {
						return errUpdateKapp
					}
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							kdaKapp, _ := state.NewKAppAccount(kdaKappAddrBytes)
							return kdaKapp, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							stakingKapp, _ := state.NewKAppAccount([]byte("stakingKappAddress"))
							return stakingKapp, &kapps.StakingData{
								MinEpochsToClaim: 1,
								InterestType:     kapps.StakingData_APRI,
							}, nil
						},
						SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
							return nil
						},
						SetKDACalled: func(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
							return nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
				GetProposalKAppCalled: func() kapp.ProposalKapp {
					return &commonMock.ProposalKappStub{
						GetProposalCalled: func(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
							if proposalID == 0 {
								proposalKapp, _ := state.NewKAppAccount(proposalKappAddrBytes)
								return proposalKapp, nil, &kapps.ProposalController{
									ActiveProposals: map[uint32]*kapps.ActiveProposals{
										1: {
											ProposalIDs: []uint64{1},
										},
									},
								}, nil
							}
							return nil, &kapps.ProposalData{
								Voters: map[string]*kapps.ProposalData_VoteDetail{
									"randomAddress": {},
								},
							}, nil, nil
						},
					}
				},
			},
			expectedErr:       errUpdateKapp,
			expectedTxResCode: transaction.Transaction_AccountError,
			unfreezeTx: &transaction.UnfreezeContract{
				AssetID: kdautils.KFIIdentifier,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			accsKapp, _ := NewAccountKApp(&ArgsNewAccountKApp{
				Hasher:         &commonMock.HasherMock{},
				Marshalizer:    &commonMock.MarshalizerMock{},
				PubkeyConv:     commonMock.NewPubkeyConverterMock(4),
				ForkController: c.forkController,
			})
			// These setters do not return an error at the moment, but we include require.NoError
			// to satisfy linting requirements and to make the tests resilient to future changes.
			require.NoError(t, accsKapp.SetKAppController(c.kappController))
			require.NoError(t, accsKapp.SetAccountsCacher(c.accountsCacher))

			txResCode, err := accsKapp.Unfreeze(
				txSender,
				c.unfreezeTx,
			)

			require.Equal(t, txResCode, c.expectedTxResCode)
			require.ErrorIs(t, err, c.expectedErr)
		})
	}
}

func TestUnfreezeKFIFailingOnFinishUpdateProposal(t *testing.T) {
	t.Run("Failing to get KFI total staked on proposal update", func(t *testing.T) {
		accsKapp, _ := NewAccountKApp(&ArgsNewAccountKApp{
			Hasher:      &commonMock.HasherMock{},
			Marshalizer: &commonMock.MarshalizerMock{},
			PubkeyConv:  commonMock.NewPubkeyConverterMock(4),
			ForkController: &integrationMock.ForkControllerStub{
				EnableSmartContractsCalled: func() bool { return true },
			},
		})

		errGetStaking := errors.New("Error getting staking kapp")
		getStakingCalled := 0
		_ = accsKapp.SetKAppController(
			&kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							kdaKapp, _ := state.NewKAppAccount([]byte("kdaKappAddress"))
							return kdaKapp, &kapps.KDAData{
								AssetType: kapps.KDAData_Fungible,
							}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							if getStakingCalled == 0 {
								getStakingCalled++

								stakingKapp, _ := state.NewKAppAccount([]byte("stakingKappAddress"))
								return stakingKapp, &kapps.StakingData{
									MinEpochsToClaim: 1,
									InterestType:     kapps.StakingData_APRI,
								}, nil
							}
							return nil, nil, errGetStaking
						},
						SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
							return nil
						},
						SetKDACalled: func(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
							return nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
				GetProposalKAppCalled: func() kapp.ProposalKapp {
					return &commonMock.ProposalKappStub{
						GetProposalCalled: func(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
							if proposalID == 0 {
								return nil, nil, &kapps.ProposalController{
									ActiveProposals: map[uint32]*kapps.ActiveProposals{
										1: {
											ProposalIDs: []uint64{1},
										},
									},
								}, nil
							}
							return nil, &kapps.ProposalData{
								Voters: map[string]*kapps.ProposalData_VoteDetail{
									hex.EncodeToString(txSender): {
										Amount: 90,
										Type:   kapps.ProposalData_VoteDetail_No,
									},
								},
								Votes: map[int32]int64{
									int32(kapps.ProposalData_VoteDetail_No): 200,
								},
							}, nil, nil
						},
					}
				},
			},
		)
		_ = accsKapp.SetAccountsCacher(
			&commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return &kapps.UserKDA{FrozenBalance: 80}, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							gainsMap := map[string]int64{
								"ABC-123": 0,
								"DEF-456": 10,
								"GHI-789": 20,
							}

							return gainsMap, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 100, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return txSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				UpdateKappCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
		)

		txResCode, err := accsKapp.Unfreeze(
			txSender,
			&transaction.UnfreezeContract{
				AssetID: kdautils.KFIIdentifier,
			},
		)

		require.Equal(t, txResCode, transaction.Transaction_AccountError)
		require.ErrorIs(t, err, errGetStaking)
	})

	t.Run("Failing to set proposal after its update", func(t *testing.T) {
		accsKapp, _ := NewAccountKApp(&ArgsNewAccountKApp{
			Hasher:         &commonMock.HasherMock{},
			Marshalizer:    &commonMock.MarshalizerMock{},
			PubkeyConv:     commonMock.NewPubkeyConverterMock(4),
			ForkController: &integrationMock.ForkControllerStub{},
		})

		errSetingProposal := errors.New("Error setting proposal")
		_ = accsKapp.SetKAppController(
			&kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							kdaKapp, _ := state.NewKAppAccount([]byte("kdaKappAddress"))
							return kdaKapp, &kapps.KDAData{
								AssetType: kapps.KDAData_Fungible,
							}, nil
						},
						GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
							stakingKapp, _ := state.NewKAppAccount([]byte("stakingKappAddress"))
							return stakingKapp, &kapps.StakingData{
								MinEpochsToClaim: 1,
								InterestType:     kapps.StakingData_APRI,
							}, nil
						},
						SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
							return nil
						},
						SetKDACalled: func(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
							return nil
						},
					}
				},
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: txSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_UnfreezeContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
				GetProposalKAppCalled: func() kapp.ProposalKapp {
					return &commonMock.ProposalKappStub{
						GetProposalCalled: func(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
							if proposalID == 0 {
								return nil, nil, &kapps.ProposalController{
									ActiveProposals: map[uint32]*kapps.ActiveProposals{
										1: {
											ProposalIDs: []uint64{1},
										},
									},
								}, nil
							}
							return nil, &kapps.ProposalData{
								Voters: map[string]*kapps.ProposalData_VoteDetail{
									hex.EncodeToString(txSender): {
										Amount: 90,
										Type:   kapps.ProposalData_VoteDetail_No,
									},
								},
								Votes: map[int32]int64{
									int32(kapps.ProposalData_VoteDetail_No): 200,
								},
							}, nil, nil
						},
						SetProposalCalled: func(proposalKapp state.KAppAccountHandler, proposalID uint64, proposal *kapps.ProposalData, controller *kapps.ProposalController) error {
							return errSetingProposal
						},
					}
				},
			},
		)
		_ = accsKapp.SetAccountsCacher(
			&commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return &kapps.UserKDA{FrozenBalance: 80}, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							gainsMap := map[string]int64{
								"ABC-123": 0,
								"DEF-456": 10,
								"GHI-789": 20,
							}

							return gainsMap, nil
						},
						UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
							return nil, 100, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return txSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				UpdateKappCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
		)

		txResCode, err := accsKapp.Unfreeze(
			txSender,
			&transaction.UnfreezeContract{
				AssetID: kdautils.KFIIdentifier,
			},
		)

		require.Equal(t, txResCode, transaction.Transaction_AccountError)
		require.ErrorIs(t, err, errSetingProposal)
	})
}

func TestUnfreezeKFIAndUpdatingProposals(t *testing.T) {
	gainsMap := map[string]int64{
		"ABC-123": 0,
		"DEF-456": 10,
		"GHI-789": 20,
	}

	t.Run(
		"completly removes vote due to unfreeze KFI amount is higher or equal than vote amount",
		func(t *testing.T) {
			const (
				voteAmount     = int64(90)
				noVotes        = int64(200)
				unfreezeAmount = int64(100)
				kfiTotalStaked = int64(1000)
			)
			accsKapp, _ := NewAccountKApp(&ArgsNewAccountKApp{
				Hasher:      &commonMock.HasherMock{},
				Marshalizer: &commonMock.MarshalizerMock{},
				PubkeyConv:  commonMock.NewPubkeyConverterMock(4),
				ForkController: &integrationMock.ForkControllerStub{
					EnableSmartContractsCalled: func() bool {
						return true
					},
				},
			})

			stakingData := &kapps.StakingData{
				MinEpochsToClaim: 1,
				InterestType:     kapps.StakingData_APRI,
				TotalStaked:      kfiTotalStaked,
			}

			proposalData := &kapps.ProposalData{
				Voters: map[string]*kapps.ProposalData_VoteDetail{
					hex.EncodeToString(txSender): {
						Type:   kapps.ProposalData_VoteDetail_No,
						Amount: voteAmount,
					},
				},
				Votes: map[int32]int64{
					int32(kapps.ProposalData_VoteDetail_No): noVotes,
				},
			}

			_ = accsKapp.SetKAppController(
				&kvmStub.KAppControllerStub{
					GetKDAKAppCalled: func() kapp.KDAKapp {
						return &kvmStub.KDAKappStub{
							GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
								kdaKapp, _ := state.NewKAppAccount([]byte("kdaKappAddress"))
								return kdaKapp, &kapps.KDAData{
									AssetType: kapps.KDAData_Fungible,
								}, nil
							},
							GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
								stakingKapp, _ := state.NewKAppAccount([]byte("stakingKappAddress"))
								return stakingKapp, stakingData, nil
							},
							SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
								return nil
							},
							SetKDACalled: func(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
								return nil
							},
						}
					},
					GetCurrentKAppContextCalled: func() kapp.KappContext {
						return kapp.NewKappContext(kapp.ArgsNewKAppContext{
							OriginalSender: txSender,
							ContractID:     0,
							ContractType:   transaction.TXContract_UnfreezeContractType,
							Block: &block.Block{
								Header: &block.BlockHeader{
									Timestamp: 1000,
									Epoch:     1,
								},
							},
						})
					},
					GetProposalKAppCalled: func() kapp.ProposalKapp {
						return &commonMock.ProposalKappStub{
							GetProposalCalled: func(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
								if proposalID == 0 {
									return nil, nil, &kapps.ProposalController{
										ActiveProposals: map[uint32]*kapps.ActiveProposals{
											1: {
												ProposalIDs: []uint64{1},
											},
										},
									}, nil
								}
								return nil, proposalData, nil, nil
							},
						}
					},
				},
			)
			_ = accsKapp.SetAccountsCacher(
				&commonMock.AccountsCacherStub{
					GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
						return &commonMock.UserAccountHandlerStub{
							GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
								return &kapps.UserKDA{FrozenBalance: 80}, nil
							},
							ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
								return gainsMap, nil
							},
							UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
								return nil, unfreezeAmount, nil
							},
							SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
								return nil
							},
							AddressBytesCalled: func() []byte {
								return txSender
							},
						}, nil
					},
					UpdateUserCalled: func(account state.AccountHandler) error {
						return nil
					},
					UpdateKappCalled: func(account state.AccountHandler) error {
						return nil
					},
				},
			)

			txResCode, err := accsKapp.Unfreeze(
				txSender,
				&transaction.UnfreezeContract{
					AssetID: kdautils.KFIIdentifier,
				},
			)

			require.NotContains(t, proposalData.Voters, hex.EncodeToString(txSender))
			require.Equal(
				t,
				proposalData.Votes[int32(kapps.ProposalData_VoteDetail_No)],
				noVotes-voteAmount,
			)
			require.Equal(t, proposalData.TotalStaked, kfiTotalStaked)

			require.Equal(t, txResCode, transaction.Transaction_Ok)
			require.NoError(t, err)
		},
	)

	t.Run(
		"subtracts vote amount from unfreeze KFI amount",
		func(t *testing.T) {
			const (
				voteAmount     = int64(100)
				noVotes        = int64(200)
				unfreezeAmount = int64(90)
				kfiTotalStaked = int64(1000)
			)
			accsKapp, _ := NewAccountKApp(&ArgsNewAccountKApp{
				Hasher:      &commonMock.HasherMock{},
				Marshalizer: &commonMock.MarshalizerMock{},
				PubkeyConv:  commonMock.NewPubkeyConverterMock(4),
				ForkController: &integrationMock.ForkControllerStub{
					EnableSmartContractsCalled: func() bool {
						return true
					},
				},
			})

			stakingData := &kapps.StakingData{
				MinEpochsToClaim: 1,
				InterestType:     kapps.StakingData_APRI,
				TotalStaked:      kfiTotalStaked,
			}

			proposalData := &kapps.ProposalData{
				Voters: map[string]*kapps.ProposalData_VoteDetail{
					hex.EncodeToString(txSender): {
						Type:   kapps.ProposalData_VoteDetail_No,
						Amount: voteAmount,
					},
				},
				Votes: map[int32]int64{
					int32(kapps.ProposalData_VoteDetail_No): noVotes,
				},
			}

			_ = accsKapp.SetKAppController(
				&kvmStub.KAppControllerStub{
					GetKDAKAppCalled: func() kapp.KDAKapp {
						return &kvmStub.KDAKappStub{
							GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
								kdaKapp, _ := state.NewKAppAccount([]byte("kdaKappAddress"))
								return kdaKapp, &kapps.KDAData{
									AssetType: kapps.KDAData_Fungible,
								}, nil
							},
							GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
								stakingKapp, _ := state.NewKAppAccount([]byte("stakingKappAddress"))
								return stakingKapp, stakingData, nil
							},
							SetStakingCalled: func(stakingKapp state.KAppAccountHandler, assetID []byte, staking *kapps.StakingData) error {
								return nil
							},
							SetKDACalled: func(kdaKapp state.KAppAccountHandler, assetID []byte, kda *kapps.KDAData) error {
								return nil
							},
						}
					},
					GetCurrentKAppContextCalled: func() kapp.KappContext {
						return kapp.NewKappContext(kapp.ArgsNewKAppContext{
							OriginalSender: txSender,
							ContractID:     0,
							ContractType:   transaction.TXContract_UnfreezeContractType,
							Block: &block.Block{
								Header: &block.BlockHeader{
									Timestamp: 1000,
									Epoch:     1,
								},
							},
						})
					},
					GetProposalKAppCalled: func() kapp.ProposalKapp {
						return &commonMock.ProposalKappStub{
							GetProposalCalled: func(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
								if proposalID == 0 {
									return nil, nil, &kapps.ProposalController{
										ActiveProposals: map[uint32]*kapps.ActiveProposals{
											1: {
												ProposalIDs: []uint64{1},
											},
										},
									}, nil
								}
								return nil, proposalData, nil, nil
							},
						}
					},
				},
			)
			_ = accsKapp.SetAccountsCacher(
				&commonMock.AccountsCacherStub{
					GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
						return &commonMock.UserAccountHandlerStub{
							GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
								return &kapps.UserKDA{FrozenBalance: 80}, nil
							},
							ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
								return gainsMap, nil
							},
							UnfreezeCalled: func(assetID, bucketID []byte, blockEpoch uint32, staking *kapps.StakingData, userKDA *kapps.UserKDA, newStakingFlow bool) ([]byte, int64, error) {
								return nil, unfreezeAmount, nil
							},
							SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
								return nil
							},
							AddressBytesCalled: func() []byte {
								return txSender
							},
						}, nil
					},
					UpdateUserCalled: func(account state.AccountHandler) error {
						return nil
					},
					UpdateKappCalled: func(account state.AccountHandler) error {
						return nil
					},
				},
			)

			txResCode, err := accsKapp.Unfreeze(
				txSender,
				&transaction.UnfreezeContract{
					AssetID: kdautils.KFIIdentifier,
				},
			)

			require.Equal(
				t,
				proposalData.Voters[hex.EncodeToString(txSender)].Amount,
				voteAmount-unfreezeAmount,
			)
			require.Equal(
				t,
				proposalData.Votes[int32(kapps.ProposalData_VoteDetail_No)],
				noVotes-unfreezeAmount,
			)
			require.Equal(t, proposalData.TotalStaked, kfiTotalStaked)

			require.Equal(t, txResCode, transaction.Transaction_Ok)
			require.NoError(t, err)
		},
	)
}

type proposalVotesConfig struct {
	fixAuditChangesV5    bool
	enableSmartContracts bool
	controller           *kapps.ProposalController
	// indexIDs builds the stored index with each entry's amount taken from the proposal's own
	// record for the voter, or math.MaxInt64 when there is none, so an entry with nothing behind
	// it is always read and pruned. indexEntries overrides that with exact entries.
	indexIDs     []uint64
	indexEntries []kapp.ProposalVoteIndexEntry
	proposals    map[uint64]*kapps.ProposalData
	// idBound, when set, is what the stubbed PreForkVoteIDBound returns: the highest proposal id
	// the pre-fork scan still reads. Unset means unbounded, so every active id is scanned.
	idBound    *uint64
	indexErr   error
	idBoundErr error
	loadErr    error
	pruneErr   error
	saveErr    error
	stakingErr error
}

type proposalVotesHarness struct {
	accountsKapp *accountsKapp
	loaded       []uint64
	// stakingLoads counts KFI staking reads; controllerRewrites counts proposal saves that
	// carried a controller, which the unfreeze path never needs to write.
	stakingLoads       int
	controllerRewrites int
	// pruned is derived: the ids of the stored index that the written index no longer carries.
	pruned       []uint64
	prunedAddrs  []string
	queriedAddrs []string
	indexWrites  int
	written      []kapp.ProposalVoteIndexEntry
	saved        []uint64
}

func (cfg proposalVotesConfig) storedIndex() []kapp.ProposalVoteIndexEntry {
	if cfg.indexEntries != nil {
		return cfg.indexEntries
	}

	entries := make([]kapp.ProposalVoteIndexEntry, 0, len(cfg.indexIDs))
	for _, id := range cfg.indexIDs {
		amount := int64(math.MaxInt64)
		if proposal, ok := cfg.proposals[id]; ok && proposal != nil {
			if voter, voted := proposal.Voters[hex.EncodeToString(txSender)]; voted && voter != nil {
				amount = voter.Amount
			}
		}
		entries = append(entries, kapp.ProposalVoteIndexEntry{ProposalID: id, Amount: amount})
	}

	return entries
}

func newProposalVotesHarness(t *testing.T, cfg proposalVotesConfig) *proposalVotesHarness {
	t.Helper()

	if cfg.controller == nil {
		// The unfreeze drops an entry whose id is in no bucket without reading it, so a harness
		// that gives no controller lists, in one bucket, every active proposal it knows and
		// every indexed id whose record is missing — tests that expect a missing record to be
		// read and pruned keep doing so. The bound is defaulted to 0 alongside, so this bucket
		// feeds the index pass and not the pre-fork scan; tests about the scan set their own.
		bucket := &kapps.ActiveProposals{}
		for id, proposal := range cfg.proposals {
			if proposal != nil && proposal.ProposalStatus == kapps.ProposalData_ActiveProposal {
				bucket.ProposalIDs = append(bucket.ProposalIDs, id)
			}
		}
		for _, entry := range cfg.storedIndex() {
			if _, known := cfg.proposals[entry.ProposalID]; !known {
				bucket.ProposalIDs = append(bucket.ProposalIDs, entry.ProposalID)
			}
		}
		slices.Sort(bucket.ProposalIDs)
		cfg.controller = &kapps.ProposalController{
			ActiveProposals: map[uint32]*kapps.ActiveProposals{500: bucket},
		}
		if cfg.idBound == nil {
			noScan := uint64(0)
			cfg.idBound = &noScan
		}
	}

	h := &proposalVotesHarness{}

	accsKapp, err := NewAccountKApp(&ArgsNewAccountKApp{
		Hasher:      &commonMock.HasherMock{},
		Marshalizer: &commonMock.MarshalizerMock{},
		PubkeyConv:  commonMock.NewPubkeyConverterMock(4),
		ForkController: &integrationMock.ForkControllerStub{
			EnableSmartContractsCalled: func() bool {
				return cfg.enableSmartContracts
			},
			FixAuditChangesV5Called: func() bool {
				return cfg.fixAuditChangesV5
			},
		},
	})
	require.NoError(t, err)

	proposalKapp, err := state.NewKAppAccount([]byte("proposalKappAddress"))
	require.NoError(t, err)

	err = accsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{
				OriginalSender: txSender,
				ContractID:     0,
				ContractType:   transaction.TXContract_UnfreezeContractType,
				Block: &block.Block{
					Header: &block.BlockHeader{
						Timestamp: 1000,
					},
				},
			})
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetStakingCalled: func(_ []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
					h.stakingLoads++

					if cfg.stakingErr != nil {
						return nil, nil, cfg.stakingErr
					}

					// A different total per load, so a memo that should have been rebuilt and
					// was not writes a stale TotalStaked that a test can see.
					return nil, &kapps.StakingData{TotalStaked: int64(1000 * h.stakingLoads)}, nil
				},
			}
		},
		GetProposalKAppCalled: func() kapp.ProposalKapp {
			return &commonMock.ProposalKappStub{
				GetProposalCalled: func(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
					if proposalID == 0 {
						return proposalKapp, nil, cfg.controller, nil
					}

					// GetProposal re-reads and unmarshals the controller on every call. The
					// unfreeze holds the account already and must read proposals through
					// GetProposalData; a revert to this path fails here by name.
					t.Errorf("unfreeze read proposal %d through GetProposal instead of GetProposalData", proposalID)

					return nil, nil, nil, errors.New("proposal read through the wrong path")
				},
				GetProposalDataCalled: func(proposalID uint64) (*kapps.ProposalData, error) {
					h.loaded = append(h.loaded, proposalID)

					if cfg.loadErr != nil {
						return nil, cfg.loadErr
					}

					proposal, ok := cfg.proposals[proposalID]
					if !ok {
						return nil, common.ErrProposalNotFound
					}

					return proposal, nil
				},
				GetAccountProposalVotesCalled: func(encodedAddr string) ([]kapp.ProposalVoteIndexEntry, error) {
					h.queriedAddrs = append(h.queriedAddrs, encodedAddr)

					if cfg.indexErr != nil {
						return nil, cfg.indexErr
					}

					return cfg.storedIndex(), nil
				},
				PreForkVoteIDBoundCalled: func() (uint64, error) {
					if cfg.idBoundErr != nil {
						return 0, cfg.idBoundErr
					}
					if cfg.idBound != nil {
						return *cfg.idBound, nil
					}

					return math.MaxUint64, nil
				},
				SetAccountProposalVotesCalled: func(encodedAddr string, entries []kapp.ProposalVoteIndexEntry) error {
					if cfg.pruneErr != nil {
						return cfg.pruneErr
					}

					h.indexWrites++
					h.written = entries
					h.prunedAddrs = append(h.prunedAddrs, encodedAddr)

					kept := make(map[uint64]struct{}, len(entries))
					for _, entry := range entries {
						kept[entry.ProposalID] = struct{}{}
					}
					for _, stored := range cfg.storedIndex() {
						if _, ok := kept[stored.ProposalID]; !ok {
							h.pruned = append(h.pruned, stored.ProposalID)
						}
					}

					return nil
				},
				SetProposalCalled: func(_ state.KAppAccountHandler, proposalID uint64, _ *kapps.ProposalData, controller *kapps.ProposalController) error {
					if cfg.saveErr != nil {
						return cfg.saveErr
					}

					if controller != nil {
						h.controllerRewrites++
					}
					h.saved = append(h.saved, proposalID)

					return nil
				},
			}
		},
	})
	require.NoError(t, err)

	err = accsKapp.SetAccountsCacher(&commonMock.AccountsCacherStub{
		UpdateKappCalled: func(_ state.AccountHandler) error {
			return nil
		},
	})
	require.NoError(t, err)

	h.accountsKapp = accsKapp

	return h
}

func (h *proposalVotesHarness) unfreezeVotes(frozenBalance, unfrozenAmount int64) (transaction.Transaction_TXResultCode, error) {
	owner := &commonMock.UserAccountHandlerStub{
		GetUserKDACalled: func(_, _ []byte, _ bool) (*kapps.UserKDA, error) {
			return &kapps.UserKDA{FrozenBalance: frozenBalance}, nil
		},
		AddressBytesCalled: func() []byte {
			return txSender
		},
	}

	return h.accountsKapp.handleProposalVotesOnUnfreeze(owner, kdautils.KFIIdentifier, txSender, unfrozenAmount)
}

func newVotedProposal(voteAmount, totalVotes int64) *kapps.ProposalData {
	return &kapps.ProposalData{
		Voters: map[string]*kapps.ProposalData_VoteDetail{
			hex.EncodeToString(txSender): {
				Type:   kapps.ProposalData_VoteDetail_No,
				Amount: voteAmount,
			},
		},
		Votes: map[int32]int64{
			int32(kapps.ProposalData_VoteDetail_No): totalVotes,
		},
	}
}

func voterAmountOf(proposal *kapps.ProposalData) int64 {
	return proposal.Voters[hex.EncodeToString(txSender)].Amount
}

func noVotesOf(proposal *kapps.ProposalData) int64 {
	return proposal.Votes[int32(kapps.ProposalData_VoteDetail_No)]
}

func TestUnfreezeReducesVotesOnlyForIndexedProposals(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(100, 500),
		2: newVotedProposal(100, 500),
		3: newVotedProposal(100, 500),
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		indexIDs:          []uint64{1, 3},
		proposals:         proposals,
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{1, 3}, h.loaded)
	require.Equal(t, []uint64{1, 3}, h.saved)
	require.Empty(t, h.pruned)

	require.Equal(t, int64(60), voterAmountOf(proposals[1]))
	require.Equal(t, int64(460), noVotesOf(proposals[1]))
	require.Equal(t, int64(60), voterAmountOf(proposals[3]))
	require.Equal(t, int64(460), noVotesOf(proposals[3]))

	require.Equal(t, int64(100), voterAmountOf(proposals[2]))
	require.Equal(t, int64(500), noVotesOf(proposals[2]))
}

func TestUnfreezeWritesTheVoteIndexOnceForManyStaleEntries(t *testing.T) {
	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		indexIDs:          []uint64{1, 2, 3, 4, 5},
		proposals:         map[uint64]*kapps.ProposalData{},
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{1, 2, 3, 4, 5}, h.pruned)
	require.Equal(t, 1, h.indexWrites)
}

func TestUnfreezeAppliesARepeatedIndexEntryOnlyOnce(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(100, 500),
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		indexIDs:          []uint64{1, 1},
		proposals:         proposals,
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{1}, h.loaded)
	require.Equal(t, int64(60), voterAmountOf(proposals[1]))
	require.Equal(t, int64(460), noVotesOf(proposals[1]))
	require.Equal(t, []kapp.ProposalVoteIndexEntry{{ProposalID: 1, Amount: 60}}, h.written,
		"the duplicate collapses and the entry carries the vote that remains")
}

func TestUnfreezeKeysTheVoteIndexOnTheSenderAddress(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(100, 500),
		2: newVotedProposal(100, 500),
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		indexIDs:          []uint64{1, 2},
		proposals:         proposals,
	})

	resCode, err := h.unfreezeVotes(10, 150)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	wantAddr := hex.EncodeToString(txSender)
	require.Equal(t, []string{wantAddr}, h.queriedAddrs)
	require.Equal(t, []string{wantAddr}, h.prunedAddrs)
}

func TestUnfreezeRemovesVoterWhenUnfrozenAmountCoversWholeVote(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(100, 500),
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		indexIDs:          []uint64{1},
		proposals:         proposals,
	})

	resCode, err := h.unfreezeVotes(10, 150)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.NotContains(t, proposals[1].Voters, hex.EncodeToString(txSender))
	require.Equal(t, int64(400), noVotesOf(proposals[1]))
	require.Equal(t, []uint64{1}, h.saved)
}

func TestUnfreezePrunesIndexEntriesForMissingProposals(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(100, 500),
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		indexIDs:          []uint64{2, 1},
		proposals:         proposals,
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{2}, h.pruned)
	require.Equal(t, []uint64{1}, h.saved)
	require.Equal(t, int64(60), voterAmountOf(proposals[1]))
}

func TestUnfreezePrunesIndexEntriesWhenAccountIsNotAVoter(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(100, 500),
		2: {
			Voters: map[string]*kapps.ProposalData_VoteDetail{},
			Votes:  map[int32]int64{},
		},
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		indexIDs:          []uint64{2, 1},
		proposals:         proposals,
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{2}, h.pruned)
	require.Equal(t, []uint64{1}, h.saved)
}

func TestUnfreezePrunesIndexEntriesWithANilVoteDetail(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: {
			Voters: map[string]*kapps.ProposalData_VoteDetail{
				hex.EncodeToString(txSender): nil,
			},
			Votes: map[int32]int64{},
		},
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		indexIDs:          []uint64{1},
		proposals:         proposals,
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{1}, h.pruned)
	require.Empty(t, h.saved)
}

func TestUnfreezeKeepsVoteWhenFrozenBalanceStillCoversIt(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(50, 500),
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		indexIDs:          []uint64{1},
		proposals:         proposals,
	})

	resCode, err := h.unfreezeVotes(80, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Empty(t, h.loaded, "a vote the remaining frozen balance covers is not even read; the index says so")
	require.Empty(t, h.saved)
	require.Empty(t, h.pruned)
	require.Equal(t, 0, h.indexWrites, "nothing changed, so the index is not rewritten")
	require.Equal(t, int64(50), voterAmountOf(proposals[1]))
	require.Equal(t, int64(500), noVotesOf(proposals[1]))
}

// TestUnfreezeReadsTheStakingTotalAgainOnTheNextUnfreeze: the read is memoised for one unfreeze
// and no longer. The total drops as buckets leave the stake, so a memo that outlived the
// transaction would stamp a stale TotalStaked onto every proposal a later unfreeze touched, on
// every node that had run the earlier one.
func TestUnfreezeReadsTheStakingTotalAgainOnTheNextUnfreeze(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(100, 500),
		2: newVotedProposal(100, 500),
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5:    true,
		enableSmartContracts: true,
		indexIDs:             []uint64{1, 2},
		proposals:            proposals,
	})

	for i := 1; i <= 2; i++ {
		resCode, err := h.unfreezeVotes(10, 40)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, resCode)

		require.Equal(t, i, h.stakingLoads,
			"exactly one staking read per unfreeze: the memo lasts one call, not the life of the kapp")
		require.Equal(t, int64(1000*i), proposals[1].TotalStaked,
			"and the total written is the one this unfreeze read, not a remembered one")
	}

	require.Equal(t, int64(20), voterAmountOf(proposals[1]), "both unfreezes were applied")
}

// TestUnfreezeLeavesTotalStakedAloneWhenSmartContractsAreOff: the refresh is gated, so before
// EnableSmartContracts the shrunk proposal keeps the total it was stored with and the staking
// total is never read. TestUnfreezeLoadsTheStakingTotalOnce covers the enabled side.
func TestUnfreezeLeavesTotalStakedAloneWhenSmartContractsAreOff(t *testing.T) {
	proposal := newVotedProposal(100, 500)
	proposal.TotalStaked = 900
	proposals := map[uint64]*kapps.ProposalData{1: proposal}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		indexIDs:          []uint64{1},
		proposals:         proposals,
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{1}, h.saved)
	require.Equal(t, int64(900), proposals[1].TotalStaked)
	require.Equal(t, 0, h.stakingLoads, "the total is not read while the gate is closed")
}

func TestUnfreezeWithoutVoteIndexScansActiveProposals(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(100, 500),
		2: newVotedProposal(100, 500),
	}

	controller := &kapps.ProposalController{
		ActiveProposals: map[uint32]*kapps.ActiveProposals{
			1: {ProposalIDs: []uint64{1, 2}},
		},
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: false,
		controller:        controller,
		proposals:         proposals,
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{1, 2}, h.saved)
	require.Empty(t, h.pruned)
	require.Equal(t, int64(60), voterAmountOf(proposals[1]))
	require.Equal(t, int64(60), voterAmountOf(proposals[2]))
}

func TestUnfreezeFailsWhenProposalVotesCannotBeResolved(t *testing.T) {
	votedProposal := func() map[uint64]*kapps.ProposalData {
		return map[uint64]*kapps.ProposalData{1: newVotedProposal(100, 500)}
	}
	nonVotedProposal := func() map[uint64]*kapps.ProposalData {
		return map[uint64]*kapps.ProposalData{
			2: {
				Voters: map[string]*kapps.ProposalData_VoteDetail{},
				Votes:  map[int32]int64{},
			},
		}
	}

	tests := []struct {
		name string
		cfg  proposalVotesConfig
	}{
		{
			name: "vote index is unreadable",
			cfg:  proposalVotesConfig{indexErr: mockError},
		},
		{
			name: "the stored scan bound cannot be read",
			cfg:  proposalVotesConfig{idBoundErr: mockError},
		},
		{
			name: "proposal lookup fails for a reason other than a missing proposal",
			cfg:  proposalVotesConfig{indexIDs: []uint64{1}, proposals: votedProposal(), loadErr: mockError},
		},
		{
			name: "pruning a stale index entry fails",
			cfg:  proposalVotesConfig{indexIDs: []uint64{7}, proposals: votedProposal(), pruneErr: mockError},
		},
		{
			name: "pruning an entry the account no longer votes on fails",
			cfg:  proposalVotesConfig{indexIDs: []uint64{2}, proposals: nonVotedProposal(), pruneErr: mockError},
		},
		{
			name: "total staked refresh fails",
			cfg: proposalVotesConfig{
				enableSmartContracts: true,
				indexIDs:             []uint64{1},
				proposals:            votedProposal(),
				stakingErr:           mockError,
			},
		},
		{
			name: "saving the updated proposal fails",
			cfg:  proposalVotesConfig{indexIDs: []uint64{1}, proposals: votedProposal(), saveErr: mockError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.fixAuditChangesV5 = true
			h := newProposalVotesHarness(t, tt.cfg)

			resCode, err := h.unfreezeVotes(10, 40)

			require.ErrorIs(t, err, mockError)
			require.Equal(t, transaction.Transaction_AccountError, resCode)
		})
	}
}

func TestUnfreezeReducesVotesCastBeforeTheVoteIndexExisted(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(100, 500),
	}

	controller := &kapps.ProposalController{
		ActiveProposals: map[uint32]*kapps.ActiveProposals{
			110: {ProposalIDs: []uint64{1}},
		},
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		controller:        controller,
		proposals:         proposals,
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{1}, h.saved)
	require.Equal(t, int64(60), voterAmountOf(proposals[1]))
	require.Equal(t, int64(460), noVotesOf(proposals[1]))
}

func TestUnfreezeCountsAnIndexedProposalOnlyOnceDuringThePreForkWindow(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(100, 500),
	}

	controller := &kapps.ProposalController{
		ActiveProposals: map[uint32]*kapps.ActiveProposals{
			110: {ProposalIDs: []uint64{1}},
		},
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		controller:        controller,
		indexIDs:          []uint64{1},
		proposals:         proposals,
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{1}, h.loaded)
	require.Equal(t, []uint64{1}, h.saved)
	require.Equal(t, int64(60), voterAmountOf(proposals[1]))
}

// TestThePreForkScanReadsNothingOnceItsProposalsHaveSettled: once the ids that predate the fork
// are settled, every id left in the buckets was created after it, so the scan itself reads no
// proposal at all. This is about the scan and not the whole unfreeze: an uncovered vote in the
// caller's own index is still read by the pass above it.
func TestThePreForkScanReadsNothingOnceItsProposalsHaveSettled(t *testing.T) {
	// Voted by someone else, which is the only shape this can take: a post-fork vote by the
	// caller would be in the index, so it could never reach the scan unindexed.
	proposals := map[uint64]*kapps.ProposalData{
		7: {
			Voters: map[string]*kapps.ProposalData_VoteDetail{
				"00112233445566778899aabbccddeeff": {Type: kapps.ProposalData_VoteDetail_No, Amount: 100},
			},
			Votes: map[int32]int64{int32(kapps.ProposalData_VoteDetail_No): 500},
		},
	}

	controller := &kapps.ProposalController{
		ActiveProposals: map[uint32]*kapps.ActiveProposals{
			200: {ProposalIDs: []uint64{7}},
		},
	}

	idBound := uint64(6)
	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		controller:        controller,
		proposals:         proposals,
		idBound:           &idBound,
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Empty(t, h.loaded)
	require.Empty(t, h.saved)
	require.Equal(t, int64(100), proposals[7].Voters["00112233445566778899aabbccddeeff"].Amount)
}

func TestUnfreezeLeavesConcludedProposalsUntouchedAndPrunesTheirIndexEntries(t *testing.T) {
	concluded := newVotedProposal(100, 500)
	concluded.ProposalStatus = kapps.ProposalData_ApprovedProposal
	concluded.TotalStaked = 900

	proposals := map[uint64]*kapps.ProposalData{1: concluded}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5:    true,
		enableSmartContracts: true,
		indexIDs:             []uint64{1},
		proposals:            proposals,
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{1}, h.pruned)
	require.Empty(t, h.saved)
	require.Equal(t, int64(100), voterAmountOf(concluded))
	require.Equal(t, int64(500), noVotesOf(concluded))
	require.Equal(t, int64(900), concluded.TotalStaked)
}

func TestUnfreezePrunesTheIndexEntryWhenTheWholeVoteIsRemoved(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(100, 500),
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		indexIDs:          []uint64{1},
		proposals:         proposals,
	})

	resCode, err := h.unfreezeVotes(10, 150)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{1}, h.saved)
	require.Equal(t, []uint64{1}, h.pruned)
	require.NotContains(t, proposals[1].Voters, hex.EncodeToString(txSender))
}

///////////////
// Royalties //
///////////////

var inactiveFork = uint32(1)

var validAddress = hex.EncodeToString(makeAddress("valid"))

var validAddressBytes = makeAddress("valid")

var emptyAccCacher = &commonMock.AccountsCacherStub{}

var mockError = errors.New("mock-error")

func makeAddress(prefix string) []byte {
	addr := make([]byte, 32)
	copy(addr, []byte(prefix))
	return addr
}

func setupAccCacher(accCacher *commonMock.AccountsCacherStub) *commonMock.AccountsCacherStub {
	if accCacher != nil {
		return accCacher
	}

	return &commonMock.AccountsCacherStub{}
}

func setupKappController(kappController *kvmStub.KAppControllerStub) *kvmStub.KAppControllerStub {
	if kappController != nil {
		// Ensure GetCurrentKAppContextCalled is set if not already provided
		if kappController.GetCurrentKAppContextCalled == nil {
			kappController.GetCurrentKAppContextCalled = func() kapp.KappContext {
				return kapp.NewKappContext(kapp.ArgsNewKAppContext{
					OriginalSender: txSender,
					ContractID:     0,
					Block:          &block.Block{},
				})
			}
		}
		return kappController
	}

	return &kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{
				OriginalSender: txSender,
				ContractID:     0,
				Block:          &block.Block{},
			})
		},
	}
}

func setupAccountsKapp(t *testing.T, cfg config.EnableEpochs) *accountsKapp {
	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, err := fork.NewForkController(
		cfg,
		epochNotifier,
	)
	require.NoError(t, err)

	accountArgs := ArgsNewAccountKApp{
		Marshalizer:    &commonMock.ProtoMarshalizerMock{},
		PubkeyConv:     cryptoMock.NewPubkeyConverterMock(32),
		ForkController: forkController,
	}

	accountsKapp, err := NewAccountKApp(&accountArgs)
	require.NoError(t, err)

	return accountsKapp
}

func Test_NewAccountKApp_NilMarshalizer(t *testing.T) {
	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, err := fork.NewForkController(
		config.EnableEpochs{},
		epochNotifier,
	)
	require.NoError(t, err)

	accountArgs := ArgsNewAccountKApp{
		Marshalizer:    nil,
		PubkeyConv:     cryptoMock.NewPubkeyConverterMock(32),
		ForkController: forkController,
	}

	_, err = NewAccountKApp(&accountArgs)
	require.Error(t, err)
	assert.Equal(t, common.ErrNilMarshalizer, err)
}

func Test_NewAccountKApp_NilPubkeyConverter(t *testing.T) {
	epochNotifier := &commonMock.EpochNotifierStub{}
	forkController, err := fork.NewForkController(
		config.EnableEpochs{},
		epochNotifier,
	)
	require.NoError(t, err)

	accountArgs := ArgsNewAccountKApp{
		Marshalizer:    &commonMock.ProtoMarshalizerMock{},
		PubkeyConv:     nil,
		ForkController: forkController,
	}

	_, err = NewAccountKApp(&accountArgs)
	require.Error(t, err)
	assert.Equal(t, common.ErrNilPubkeyConverter, err)
}

func Test_SetAccountsCacher_NilAccountsAdapter(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	err := accountsKapp.SetAccountsCacher(nil)
	require.Error(t, err)
	assert.Equal(t, common.ErrNilAccountsAdapter, err)
}

func Test_IsInterfaceNil(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})
	isInterfaceNil := accountsKapp.IsInterfaceNil()
	require.False(t, isInterfaceNil)
}

func Test_GetAccountsCacher(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})
	accCacher := accountsKapp.GetAccountsCacher()
	require.Nil(t, accCacher)
}

func Test_GetExistingUserAccount(t *testing.T) {
	tests := []struct {
		description string
		expectedErr error
		accCacher   *commonMock.AccountsCacherStub
	}{{
		description: "should fail to get user",
		expectedErr: mockError,
		accCacher: &commonMock.AccountsCacherStub{
			GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
				return nil, mockError
			},
		},
	}, {
		description: "should work",
		expectedErr: nil,
		accCacher: &commonMock.AccountsCacherStub{
			GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
				return nil, nil
			},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			assert := assert.New(t)

			accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})
			require.NoError(t, accountsKapp.SetAccountsCacher(setupAccCacher(tt.accCacher)))

			_, err := accountsKapp.GetExistingUserAccount([]byte{})
			assert.Equal(tt.expectedErr, err)
		})
	}
}

func Test_LoadKDA(t *testing.T) {
	tests := []struct {
		description    string
		kdaID          []byte
		expectedErr    error
		expectedStatus transaction.Transaction_TXResultCode
		kappController *kvmStub.KAppControllerStub
	}{
		{
			description: "should not find kda",
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, nil, mockError
						},
					}
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_KAPPError,
		},
		{
			description: "should invalid kda id for the kda type",
			kdaID:       []byte("KDA/1"),
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{
								AssetType: kapps.KDAData_Fungible,
							}, nil
						},
					}
				},
			},
			expectedErr:    common.ErrInvalidValue,
			expectedStatus: transaction.Transaction_ParameterInvalid,
		},
		{
			description: "should ok",
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, nil, nil
						},
					}
				},
			},
			expectedErr:    nil,
			expectedStatus: transaction.Transaction_Ok,
		},
		{
			description: "should ok with nft",
			kdaID:       []byte("KDA/1"),
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{
								AssetType: kapps.KDAData_NonFungible,
							}, nil
						},
					}
				},
			},
			expectedErr:    nil,
			expectedStatus: transaction.Transaction_Ok,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			assert := assert.New(t)

			accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})
			require.NoError(t, accountsKapp.SetKAppController(setupKappController(tt.kappController)))

			_, _, _, status, err := accountsKapp.loadKDA(tt.kdaID)
			assert.Equal(tt.expectedErr, err)
			assert.Equal(tt.expectedStatus, status)
		})
	}
}

func Test_ComputeSplitRoyalties(t *testing.T) {
	fixedValue := int64(100)
	fixedPercentage := int64(20_00)
	royaltiesToPay := fixedValue

	tests := []struct {
		description    string
		address        string
		acc            state.UserAccountHandler
		value          int64
		percentage     int64
		scFork         uint32
		accCacher      *commonMock.AccountsCacherStub
		kappController *kvmStub.KAppControllerStub
		expectedErr    error
		expectedStatus transaction.Transaction_TXResultCode
	}{
		{
			description:    "invalid address error",
			address:        "invalid-address",
			accCacher:      emptyAccCacher,
			expectedErr:    hex.InvalidByteError(0x69),
			expectedStatus: transaction.Transaction_LoadAccountError,
		},
		{
			description: "invalid account error",
			address:     validAddress,
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, mockError
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_LoadAccountError,
		},
		{
			description: "invalid royalties values error",
			address:     validAddress,
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, nil
				},
			},
			value:          math.MaxInt64,
			percentage:     math.MaxInt64,
			expectedErr:    common.ErrInt64Overflow,
			expectedStatus: transaction.Transaction_ParameterInvalid,
		},
		{
			description: "add to balance error",
			address:     validAddress,
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
							return mockError
						},
					}, nil
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_BalanceError,
		},
		{
			description: "update user error",
			address:     validAddress,
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
							return nil
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return mockError
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_SaveAccountError,
		},
		{
			description: "ok",
			address:     validAddress,
			acc:         &commonMock.UserAccountHandlerStub{},
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
							return nil
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						ContractID: 0,
					})
				},
			},
			expectedErr:    nil,
			expectedStatus: transaction.Transaction_Ok,
		},
	}

	for _, tt := range tests {
		if tt.value == 0 {
			tt.value = fixedValue
		}
		if tt.percentage == 0 {
			tt.percentage = fixedPercentage
		}

		t.Run(tt.description, func(t *testing.T) {
			assert := assert.New(t)

			accKapp := setupAccountsKapp(t, config.EnableEpochs{
				SmartContracts: tt.scFork,
			})

			_ = accKapp.SetAccountsCacher(tt.accCacher)
			_ = accKapp.SetKAppController(setupKappController(tt.kappController))

			status, err := accKapp.computeSplitRoyalties(tt.address, kdautils.KLVIdentifier,
				kapps.KDAData_Fungible, tt.acc, tt.value, tt.percentage, &royaltiesToPay)
			assert.Equal(tt.expectedErr, err)
			assert.Equal(tt.expectedStatus, status)
		})
	}
}

func Test_ValidateAndLoadAccounts(t *testing.T) {
	tests := []struct {
		description         string
		sender              []byte
		transactionContract *transaction.TransferContract
		accCacher           *commonMock.AccountsCacherStub
		expectedErr         error
		expectedStatus      transaction.Transaction_TXResultCode
	}{
		{
			description: "invalid receiver address",
			transactionContract: &transaction.TransferContract{
				ToAddress: []byte("invalid-receiver-address"),
			},
			expectedErr:    process.ErrInvalidRcvAddr,
			expectedStatus: transaction.Transaction_AccountError,
		},
		{
			description: "same account",
			transactionContract: &transaction.TransferContract{
				ToAddress: makeAddress("valid"),
			},
			sender:         makeAddress("valid"),
			expectedErr:    process.ErrSameSenderAndReceiverAddress,
			expectedStatus: transaction.Transaction_SameAccountError,
		},
		{
			description: "load source account error",
			transactionContract: &transaction.TransferContract{
				ToAddress: makeAddress("valid-1"),
			},
			sender: validAddressBytes,
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, mockError
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_LoadAccountError,
		},
		{
			description: "load destination account error",
			transactionContract: &transaction.TransferContract{
				ToAddress: makeAddress("valid-1"),
			},
			sender: validAddressBytes,
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					if bytes.Equal(address, validAddressBytes) {
						return nil, nil
					}

					return nil, mockError
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_LoadAccountError,
		},
		{
			description: "should work",
			transactionContract: &transaction.TransferContract{
				ToAddress: makeAddress("valid-1"),
			},
			sender: validAddressBytes,
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, nil
				},
			},
			expectedErr:    nil,
			expectedStatus: transaction.Transaction_Ok,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			assert := assert.New(t)

			accKapp := setupAccountsKapp(t, config.EnableEpochs{})

			_ = accKapp.SetAccountsCacher(setupAccCacher(tt.accCacher))
			_ = accKapp.SetKAppController(&kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: tt.sender,
						ContractID:     0,
						ContractType:   transaction.TXContract_TransferContractType,
						Block:          &block.Block{},
					})
				},
			})

			_, _, status, err := accKapp.validateAndLoadAccounts(tt.sender, tt.transactionContract)
			assert.Equal(tt.expectedErr, err)
			assert.Equal(tt.expectedStatus, status)
		})
	}
}

func Test_ProcessFixedRoyaltiesTransfer(t *testing.T) {
	balance := int64(100)
	royaltyOwner := &commonMock.UserAccountHandlerStub{
		GetOwnerAddressCalled: func() []byte { return []byte{1} },
		AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
			return nil
		},
	}

	tests := []struct {
		description         string
		transactionContract *transaction.TransferContract
		accSrc              state.UserAccountHandler
		accDst              state.UserAccountHandler
		kda                 *kapps.KDAData
		accCacher           *commonMock.AccountsCacherStub
		kappController      *kvmStub.KAppControllerStub
		fprFork             uint32
		expectedErr         error
		expectedStatus      transaction.Transaction_TXResultCode
	}{
		{
			description: "without royalties",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{},
			},
			expectedErr:    nil,
			expectedStatus: transaction.Transaction_Ok,
			accSrc: &commonMock.UserAccountHandlerStub{
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
		},
		{
			description: "klv royalties not equal transferFixed royalties",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferFixed: balance + 50,
				},
			},
			transactionContract: &transaction.TransferContract{
				KLVRoyalties: balance,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			expectedErr:    common.ErrInvalidValue,
			expectedStatus: transaction.Transaction_ParameterInvalid,
		},
		{
			description: "insufficient funds",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferFixed: balance,
				},
			},
			transactionContract: &transaction.TransferContract{
				KLVRoyalties: balance,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance - 1
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			expectedErr:    process.ErrInsufficientFunds,
			expectedStatus: transaction.Transaction_OutOfFunds,
		},
		{
			description: "sub from balance error",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferFixed: balance,
				},
			},
			transactionContract: &transaction.TransferContract{
				KLVRoyalties: balance,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return mockError
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_BalanceError,
		},
		{
			description: "update user error",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferFixed: balance,
				},
			},
			transactionContract: &transaction.TransferContract{
				KLVRoyalties: balance,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			accCacher: &commonMock.AccountsCacherStub{
				UpdateUserCalled: func(account state.AccountHandler) error {
					return mockError
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_SaveAccountError,
		},
		{
			description: "invalid split royalties",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferFixed: balance,
					SplitRoyalties: map[string]*kapps.RoyaltySplitData{
						"invalidAddress": {},
					},
				},
			},
			transactionContract: &transaction.TransferContract{
				KLVRoyalties: balance,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			accCacher: &commonMock.AccountsCacherStub{
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			expectedErr:    hex.InvalidByteError(0x69),
			expectedStatus: transaction.Transaction_LoadAccountError,
		},
		{
			description: "error in load royaltyOwner account",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferFixed: balance,
				},
			},
			transactionContract: &transaction.TransferContract{
				KLVRoyalties: balance,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			accCacher: &commonMock.AccountsCacherStub{
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, mockError
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_LoadAccountError,
		},
		{
			description: "error in load royaltyOwner account pre-fpk fork",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferFixed: balance,
				},
			},
			transactionContract: &transaction.TransferContract{
				KLVRoyalties: balance,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			accCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, mockError
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			fprFork:        inactiveFork,
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_LoadAccountError,
		},
		{
			description: "error in add balance of royaltyOwner account",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferFixed: balance,
				},
			},
			transactionContract: &transaction.TransferContract{
				KLVRoyalties: balance,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			accCacher: &commonMock.AccountsCacherStub{
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
							return mockError
						},
					}, nil
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_BalanceError,
		},
		{
			description: "error in update user of royaltyOwner account",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferFixed: balance,
				},
			},
			transactionContract: &transaction.TransferContract{
				KLVRoyalties: balance,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			accCacher: &commonMock.AccountsCacherStub{
				UpdateUserCalled: func(account state.AccountHandler) error {
					if account == royaltyOwner {
						return mockError
					}
					return nil
				},
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return royaltyOwner, nil
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_SaveAccountError,
		},
		{
			description: "should work",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferFixed: balance,
				},
			},
			transactionContract: &transaction.TransferContract{
				KLVRoyalties: balance,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			accCacher: &commonMock.AccountsCacherStub{
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return royaltyOwner, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						ContractID: 0,
					})
				},
			},
			expectedErr:    nil,
			expectedStatus: transaction.Transaction_Ok,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			assert := assert.New(t)

			accKapp := setupAccountsKapp(t, config.EnableEpochs{
				KdaFpr: tt.fprFork,
			})
			_ = accKapp.SetAccountsCacher(setupAccCacher(tt.accCacher))
			_ = accKapp.SetKAppController(setupKappController(tt.kappController))

			status, err := accKapp.processFixedRoyaltiesTransfer(
				tt.transactionContract,
				tt.accSrc,
				tt.accDst,
				tt.kda,
			)
			assert.Equal(tt.expectedErr, err)
			assert.Equal(tt.expectedStatus, status)
		})
	}
}

func Test_ValidatePercentageRoyaltiesTransfer(t *testing.T) {
	percentage := int64(50_00)
	amount := int64(100)

	tests := []struct {
		description         string
		transactionContract *transaction.TransferContract
		accSrc              state.UserAccountHandler
		accDst              state.UserAccountHandler
		kda                 *kapps.KDAData
		accCacher           *commonMock.AccountsCacherStub
		kappController      *kvmStub.KAppControllerStub
		expectedErr         error
		expectedStatus      transaction.Transaction_TXResultCode
	}{
		{
			description: "without royalties",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{},
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			expectedErr:    nil,
			expectedStatus: transaction.Transaction_Ok,
		},
		{
			description: "invalid amount",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferPercentage: []*kapps.RoyaltyData{
						{
							Amount:     amount,
							Percentage: uint32(percentage),
						},
					},
				},
			},
			transactionContract: &transaction.TransferContract{
				Amount: -1,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			expectedErr:    common.ErrInvalidValue,
			expectedStatus: transaction.Transaction_ContractInvalid,
		},
		{
			description: "invalid get balance",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferPercentage: []*kapps.RoyaltyData{
						{
							Amount:     10000,
							Percentage: uint32(percentage),
						},
					},
				},
			},
			transactionContract: &transaction.TransferContract{
				Amount: amount,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return amount
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			expectedErr:    process.ErrInsufficientFunds,
			expectedStatus: transaction.Transaction_OutOfFunds,
		},

		{
			description: "should ok",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferPercentage: []*kapps.RoyaltyData{
						{
							Amount:     amount,
							Percentage: uint32(percentage),
						},
					},
				},
			},
			transactionContract: &transaction.TransferContract{
				Amount: amount,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			expectedErr:    nil,
			expectedStatus: transaction.Transaction_Ok,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			assert := assert.New(t)

			accKapp := setupAccountsKapp(t, config.EnableEpochs{})
			_ = accKapp.SetAccountsCacher(setupAccCacher(tt.accCacher))
			_ = accKapp.SetKAppController(setupKappController(tt.kappController))

			_, status, err := accKapp.validatePercentageRoyaltiesTransfer(
				tt.transactionContract,
				tt.kda,
				tt.accSrc,
				kdautils.KLVIdentifier,
			)
			assert.Equal(tt.expectedErr, err)
			assert.Equal(tt.expectedStatus, status)
		})
	}
}

func Test_ProcessPercentageRoyaltiesTransfer(t *testing.T) {
	percentage := 50_00
	amount := int64(100)

	tests := []struct {
		description         string
		transactionContract *transaction.TransferContract
		accSrc              state.UserAccountHandler
		accDst              state.UserAccountHandler
		kda                 *kapps.KDAData
		accCacher           *commonMock.AccountsCacherStub
		kappController      *kvmStub.KAppControllerStub
		expectedErr         error
		expectedStatus      transaction.Transaction_TXResultCode
	}{
		{
			description: "invalid percent royalties amount",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferPercentage: []*kapps.RoyaltyData{
						{
							Amount:     amount,
							Percentage: uint32(percentage),
						},
					},
				},
			},
			transactionContract: &transaction.TransferContract{
				Amount: -1,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			expectedErr:    common.ErrInvalidValue,
			expectedStatus: transaction.Transaction_ContractInvalid,
		},
		{
			description: "should ok because don't have royaltyAmount",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferPercentage: []*kapps.RoyaltyData{
						{
							Amount:     0,
							Percentage: 0,
						},
					},
				},
			},
			transactionContract: &transaction.TransferContract{
				Amount: amount,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			expectedErr:    nil,
			expectedStatus: transaction.Transaction_Ok,
		},
		{
			description: "kda royalties doesn't match royaltyAmount",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferPercentage: []*kapps.RoyaltyData{
						{
							Amount:     0,
							Percentage: uint32(percentage),
						},
					},
				},
			},
			transactionContract: &transaction.TransferContract{
				Amount:       amount,
				KDARoyalties: 0,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			expectedErr:    common.ErrInvalidValue,
			expectedStatus: transaction.Transaction_ParameterInvalid,
		},
		{
			description: "invalid split royalties",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					SplitRoyalties: map[string]*kapps.RoyaltySplitData{
						"invalidAddress": {},
					},
					TransferPercentage: []*kapps.RoyaltyData{
						{
							Amount:     amount,
							Percentage: uint32(percentage),
						},
					},
				},
			},
			transactionContract: &transaction.TransferContract{
				Amount:       amount,
				KDARoyalties: amount / 2,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			expectedErr:    hex.InvalidByteError(0x69),
			expectedStatus: transaction.Transaction_LoadAccountError,
		},
		{
			description: "valid split royalties but royaltiesToPay all spended in royalties",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					SplitRoyalties: map[string]*kapps.RoyaltySplitData{
						hex.EncodeToString(makeAddress("valid")): {
							PercentTransferPercentage: 100_00,
						},
					},
					TransferPercentage: []*kapps.RoyaltyData{
						{
							Amount:     amount,
							Percentage: uint32(percentage),
						},
					},
				},
			},
			transactionContract: &transaction.TransferContract{
				Amount:       amount,
				KDARoyalties: amount / 2,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			accCacher: &commonMock.AccountsCacherStub{
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
							return nil
						},
					}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						ContractID: 0,
					})
				},
			},
			expectedErr:    nil,
			expectedStatus: transaction.Transaction_Ok,
		},
		{
			description: "can't load royaltyReceiver",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferPercentage: []*kapps.RoyaltyData{
						{
							Amount:     amount,
							Percentage: uint32(percentage),
						},
					},
				},
			},
			transactionContract: &transaction.TransferContract{
				Amount:       amount,
				KDARoyalties: amount / 2,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, mockError
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_LoadAccountError,
		},
		{
			description: "royaltyReceiver sub from balance error",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferPercentage: []*kapps.RoyaltyData{
						{
							Amount:     amount,
							Percentage: uint32(percentage),
						},
					},
				},
			},
			transactionContract: &transaction.TransferContract{
				Amount:       amount,
				KDARoyalties: amount / 2,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return mockError
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, nil
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_BalanceError,
		},
		{
			description: "royaltyReceiver add to balance error",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferPercentage: []*kapps.RoyaltyData{
						{
							Amount:     amount,
							Percentage: uint32(percentage),
						},
					},
				},
			},
			transactionContract: &transaction.TransferContract{
				Amount:       amount,
				KDARoyalties: amount / 2,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
							return mockError
						},
					}, nil
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_BalanceError,
		},
		{
			description: "update user royaltyReceiver error",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferPercentage: []*kapps.RoyaltyData{
						{
							Amount:     amount,
							Percentage: uint32(percentage),
						},
					},
				},
			},
			transactionContract: &transaction.TransferContract{
				Amount:       amount,
				KDARoyalties: amount / 2,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
							return nil
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return mockError
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_SaveAccountError,
		},
		{
			description: "should ok",
			kda: &kapps.KDAData{
				Royalties: &kapps.RoyaltiesData{
					TransferPercentage: []*kapps.RoyaltyData{
						{
							Amount:     amount,
							Percentage: uint32(percentage),
						},
					},
				},
			},
			transactionContract: &transaction.TransferContract{
				Amount:       amount,
				KDARoyalties: amount / 2,
			},
			accSrc: &commonMock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
				AddressBytesCalled: func() []byte {
					return []byte{1}
				},
			},
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
							return nil
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						ContractID: 0,
					})
				},
			},
			expectedErr:    nil,
			expectedStatus: transaction.Transaction_Ok,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			assert := assert.New(t)

			accKapp := setupAccountsKapp(t, config.EnableEpochs{})
			_ = accKapp.SetAccountsCacher(setupAccCacher(tt.accCacher))
			_ = accKapp.SetKAppController(setupKappController(tt.kappController))

			status, err := accKapp.processPercentageRoyaltiesTransfer(
				tt.transactionContract,
				kdautils.KLVIdentifier,
				[]byte{0},
				tt.accSrc,
				tt.accDst,
				tt.kda,
			)
			assert.Equal(tt.expectedErr, err)
			assert.Equal(tt.expectedStatus, status)
		})
	}
}

func TestFix_PercentRoyalty100SplitDebitsSenderAndConserves(t *testing.T) {
	const (
		assetIDStr    = "FUNGI-1234"
		transferValue = int64(800)
		royaltyRate   = uint32(500)
		royaltyAmount = int64(40)
	)
	assetID := []byte(assetIDStr)

	senderAddr := bytes.Repeat([]byte{0x11}, 32)
	recipientAddr := bytes.Repeat([]byte{0x22}, 32)
	recipientKey := hex.EncodeToString(recipientAddr)
	royaltyReceiverAddr := bytes.Repeat([]byte{0x33}, 32)

	buildKDA := func(splitPercent uint32) *kapps.KDAData {
		return &kapps.KDAData{
			AssetType:    kapps.KDAData_Fungible,
			OwnerAddress: senderAddr,
			Royalties: &kapps.RoyaltiesData{
				Address: royaltyReceiverAddr,
				TransferPercentage: []*kapps.RoyaltyData{
					{Amount: 1000, Percentage: royaltyRate},
				},
				SplitRoyalties: map[string]*kapps.RoyaltySplitData{
					recipientKey: {PercentTransferPercentage: splitPercent},
				},
			},
		}
	}

	type runResult struct {
		subFromCalls   int
		subFromAmount  int64
		addToRecipient int64
		addToOwnerRem  int64
		resCode        transaction.Transaction_TXResultCode
		err            error
	}

	run := func(t *testing.T, splitPercent uint32, fixActive bool) runResult {
		t.Helper()
		res := runResult{}

		acntSrc := &commonMock.UserAccountHandlerStub{
			AddressBytesCalled: func() []byte { return senderAddr },
			GetBalanceCalled:   func(_ []byte, _ bool) int64 { return 1_000_000 },
			SubFromBalanceCalled: func(value int64, _ []byte, _ bool, _ ...*kapps.UserKDA) error {
				res.subFromCalls++
				res.subFromAmount += value
				return nil
			},
		}
		acntDst := &commonMock.UserAccountHandlerStub{
			AddressBytesCalled: func() []byte { return royaltyReceiverAddr },
		}
		splitRecipient := &commonMock.UserAccountHandlerStub{
			AddressBytesCalled: func() []byte { return recipientAddr },
			AddToBalanceCalled: func(value int64, _ []byte, _ bool, _ ...*kapps.UserKDA) error {
				res.addToRecipient += value
				return nil
			},
		}
		royaltyReceiver := &commonMock.UserAccountHandlerStub{
			AddressBytesCalled: func() []byte { return royaltyReceiverAddr },
			AddToBalanceCalled: func(value int64, _ []byte, _ bool, _ ...*kapps.UserKDA) error {
				res.addToOwnerRem += value
				return nil
			},
		}
		cacher := &commonMock.AccountsCacherStub{
			LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
				if bytes.Equal(address, recipientAddr) {
					return splitRecipient, nil
				}
				if bytes.Equal(address, royaltyReceiverAddr) {
					return royaltyReceiver, nil
				}
				return acntSrc, nil
			},
			GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
				return royaltyReceiver, nil
			},
			UpdateUserCalled: func(_ state.AccountHandler) error { return nil },
		}
		fc := &integrationMock.ForkControllerStub{
			KdaFprCalled:               func() bool { return true },
			EnableSmartContractsCalled: func() bool { return true },
			FixMarketBuyOverflowCalled: func() bool { return fixActive },
		}
		kappController := &kvmStub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext {
				return kapp.NewKappContext(kapp.ArgsNewKAppContext{
					OriginalSender: senderAddr,
					ContractID:     0,
					ContractType:   transaction.TXContract_TransferContractType,
					Block:          &block.Block{},
				})
			},
		}
		a := &accountsKapp{
			accountsCacher: cacher,
			forkController: fc,
			KAppController: kappController,
		}
		tc := &transaction.TransferContract{
			Amount:       transferValue,
			KDARoyalties: royaltyAmount,
		}
		res.resCode, res.err = a.processPercentageRoyaltiesTransfer(tc, assetID, nil, acntSrc, acntDst, buildKDA(splitPercent))
		return res
	}

	t.Run("fix_on_100pct_conserves", func(t *testing.T) {
		r := run(t, core.HundredPercent, true)

		require.NoError(t, r.err)
		require.Equal(t, transaction.Transaction_Ok, r.resCode)
		require.Equal(t, 1, r.subFromCalls,
			"sender royalty pool must be debited exactly once, before the split distribution")
		require.Equal(t, royaltyAmount, r.subFromAmount,
			"sender must be debited the full royalty pool")
		require.Equal(t, royaltyAmount, r.addToRecipient,
			"split recipient receives the full royalty pool")
		require.Equal(t, int64(0), r.addToOwnerRem,
			"no owner remainder at a 100 percent split")
		require.Equal(t, r.subFromAmount, r.addToRecipient+r.addToOwnerRem,
			"value conserved: total credited equals the sender debit (no mint)")
	})

	t.Run("fix_off_100pct_legacy_preserved", func(t *testing.T) {
		r := run(t, core.HundredPercent, false)

		require.NoError(t, r.err)
		require.Equal(t, transaction.Transaction_Ok, r.resCode)
		require.Equal(t, 0, r.subFromCalls,
			"pre-fork behavior is unchanged: the debit is still skipped at a 100 percent split")
	})

	t.Run("fix_on_50pct_conserves", func(t *testing.T) {
		r := run(t, core.HundredPercent/2, true)

		require.NoError(t, r.err)
		require.Equal(t, transaction.Transaction_Ok, r.resCode)
		require.Equal(t, 1, r.subFromCalls)
		require.Equal(t, royaltyAmount, r.subFromAmount)
		require.Equal(t, royaltyAmount/2, r.addToRecipient)
		require.Equal(t, royaltyAmount/2, r.addToOwnerRem)
		require.Equal(t, r.subFromAmount, r.addToRecipient+r.addToOwnerRem,
			"50 percent split conserves")
	})
}

func Test_Transfer_ShouldFail(t *testing.T) {
	scaddress, _ := hex.DecodeString("000000000000000005005fed9c659422cd8429ce92f8973bba2a9fb51e0eb3a1")
	tests := []struct {
		description         string
		sender              []byte
		transactionContract *transaction.TransferContract
		contractType        transaction.TXContract_ContractType
		accCacher           *commonMock.AccountsCacherStub
		kappController      *kvmStub.KAppControllerStub
		expectedErr         error
		expectedStatus      transaction.Transaction_TXResultCode
	}{
		{
			description: "should fail in validateAndLoadAccounts",
			transactionContract: &transaction.TransferContract{
				ToAddress: []byte("invalid-address"),
			},
			expectedErr:    process.ErrInvalidRcvAddr,
			expectedStatus: transaction.Transaction_AccountError,
		},
		{
			description: "should fail in loadKDA",
			transactionContract: &transaction.TransferContract{
				ToAddress: makeAddress("valid-1"),
				AssetID:   []byte("invalid"),
			},
			sender: validAddressBytes,
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, nil, mockError
						},
					}
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_KAPPError,
		},
		{
			description: "should fail in because asset is paused",
			transactionContract: &transaction.TransferContract{
				ToAddress: makeAddress("valid-1"),
				AssetID:   []byte("valid"),
			},
			sender: validAddressBytes,
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{
								AssetType: kapps.KDAData_Fungible,
								Attributes: &kapps.AttributesData{
									IsPaused: true,
								},
							}, nil
						},
					}
				},
			},
			expectedErr:    process.ErrAssetIsPaused,
			expectedStatus: transaction.Transaction_AssetPaused,
		},
		{
			description: "should fail in because asset is paused",
			transactionContract: &transaction.TransferContract{
				ToAddress: makeAddress("valid-1"),
				AssetID:   []byte("valid"),
			},
			sender: validAddressBytes,
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{
								AssetType:  kapps.KDAData_Fungible,
								Attributes: &kapps.AttributesData{},
								Properties: &kapps.PropertiesData{
									LimitTransfer: true,
								},
							}, nil
						},
					}
				},
			},
			expectedErr:    process.ErrKDATransferNotAllowed,
			expectedStatus: transaction.Transaction_KDATransferNotAllowed,
		},
		{
			description:  "should fail when transferring to uninitialized contract address (non-SmartContractType)",
			contractType: transaction.TXContract_TransferContractType,
			transactionContract: &transaction.TransferContract{
				ToAddress: scaddress,
				AssetID:   []byte("valid"),
			},
			sender: validAddressBytes,
			accCacher: &commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					// Simulate uninitialized contract - account doesn't exist
					return nil, errors.New("account does not exist")
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetForkControllerCalled: func() core.ForkController {
					return &integrationMock.ForkControllerStub{
						EnableSmartContractsCalled: func() bool {
							return true
						},
					}
				},
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							return nil, &kapps.KDAData{
								AssetType:  kapps.KDAData_Fungible,
								Attributes: &kapps.AttributesData{},
								Properties: &kapps.PropertiesData{},
							}, nil
						},
					}
				},
			},
			expectedErr:    process.ErrContractAccountNotAllowed,
			expectedStatus: transaction.Transaction_AccountError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			assert := assert.New(t)

			accKapp := setupAccountsKapp(t, config.EnableEpochs{})

			_ = accKapp.SetAccountsCacher(setupAccCacher(tt.accCacher))
			_ = accKapp.SetKAppController(setupKappController(tt.kappController))

			status, err := accKapp.Transfer(tt.contractType, tt.sender, tt.transactionContract)
			assert.Equal(tt.expectedErr, err)
			assert.Equal(tt.expectedStatus, status)
		})
	}
}

func TestUpdatePermission(t *testing.T) {
	tests := []struct {
		name           string
		sender         []byte
		tc             *transaction.UpdateAccountPermissionContract
		accountsCacher *commonMock.AccountsCacherStub
		forkController core.ForkController
		expectedCode   transaction.Transaction_TXResultCode
		expectedError  error
	}{
		{
			name:   "Too many permissions",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: make([]*transaction.AccPermission, core.MaxAccountPermission+1),
			},
			accountsCacher: &commonMock.AccountsCacherStub{},
			expectedCode:   transaction.Transaction_ParameterInvalid,
			expectedError:  common.ErrInvalidParameter,
		},
		{
			name:   "Invalid account",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{},
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return nil, errors.New("account error")
				},
			},
			expectedCode:  transaction.Transaction_LoadAccountError,
			expectedError: errors.New("account error"),
		},
		{
			name:   "Empty signers list",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type:      transaction.AccPermission_Owner,
						Signers:   []*transaction.AccKey{},
						Threshold: 1,
					},
				},
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidParameter,
		},
		{
			name:   "Invalid signer address length",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_Owner,
						Signers: []*transaction.AccKey{
							{Address: []byte("short")},
						},
						Threshold: 1,
					},
				},
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidParameter,
		},
		{
			name:   "Duplicate signers",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_Owner,
						Signers: []*transaction.AccKey{
							{Address: bytes.Repeat([]byte{1}, 32), Weight: 1},
							{Address: bytes.Repeat([]byte{1}, 32), Weight: 1},
						},
						Threshold: 1,
					},
				},
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidParameter,
		},
		{
			name:   "Invalid threshold",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_Owner,
						Signers: []*transaction.AccKey{
							{Address: bytes.Repeat([]byte{1}, 32), Weight: 1},
						},
						Threshold: 2,
					},
				},
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidParameter,
		},
		{
			name:   "Zero threshold rejected",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_Owner,
						Signers: []*transaction.AccKey{
							{Address: bytes.Repeat([]byte{1}, 32), Weight: 1},
						},
						Threshold: 0,
					},
				},
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: transaction.ErrInvalidPermissionThreshold,
		},
		{
			name:   "Negative threshold rejected",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_Owner,
						Signers: []*transaction.AccKey{
							{Address: bytes.Repeat([]byte{1}, 32), Weight: 1},
						},
						Threshold: -1,
					},
				},
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: transaction.ErrInvalidPermissionThreshold,
		},
		{
			name:   "Zero signer weight rejected",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_Owner,
						Signers: []*transaction.AccKey{
							{Address: bytes.Repeat([]byte{1}, 32), Weight: 0},
						},
						Threshold: 1,
					},
				},
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: transaction.ErrInvalidSignerWeight,
		},
		{
			name:   "Negative signer weight rejected",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_Owner,
						Signers: []*transaction.AccKey{
							{Address: bytes.Repeat([]byte{1}, 32), Weight: -1},
						},
						Threshold: 1,
					},
				},
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: transaction.ErrInvalidSignerWeight,
		},
		{
			name:   "Signer weight sum overflow rejected",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_Owner,
						Signers: []*transaction.AccKey{
							{Address: bytes.Repeat([]byte{1}, 32), Weight: math.MaxInt64},
							{Address: bytes.Repeat([]byte{2}, 32), Weight: 1},
						},
						Threshold: 1,
					},
				},
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInt64Overflow,
		},
		{
			name:   "Zero threshold accepted before fork",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_Owner,
						Signers: []*transaction.AccKey{
							{Address: bytes.Repeat([]byte{1}, 32), Weight: 1},
						},
						Threshold: 0,
					},
				},
			},
			forkController: commonMock.NewForkControllerStub().SetFork("FixAuditChangesV3", false),
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
				UpdateUserCalled: func(state.AccountHandler) error {
					return nil
				},
			},
			expectedCode: transaction.Transaction_Ok,
		},
		{
			name:   "Zero signer weight accepted before fork",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_Owner,
						Signers: []*transaction.AccKey{
							{Address: bytes.Repeat([]byte{1}, 32), Weight: 0},
						},
						Threshold: 0,
					},
				},
			},
			forkController: commonMock.NewForkControllerStub().SetFork("FixAuditChangesV3", false),
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
				UpdateUserCalled: func(state.AccountHandler) error {
					return nil
				},
			},
			expectedCode: transaction.Transaction_Ok,
		},
		{
			name:   "Signer weight sum uses legacy wrap before fork",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_Owner,
						Signers: []*transaction.AccKey{
							{Address: bytes.Repeat([]byte{1}, 32), Weight: math.MaxInt64},
							{Address: bytes.Repeat([]byte{2}, 32), Weight: 1},
						},
						Threshold: 1,
					},
				},
			},
			forkController: commonMock.NewForkControllerStub().SetFork("FixAuditChangesV3", false),
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidParameter,
		},
		{
			name:   "Invalid permission name with KdaFpr enabled",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_Owner,
						Signers: []*transaction.AccKey{
							{Address: bytes.Repeat([]byte{1}, 32), Weight: 1},
						},
						Threshold:      1,
						PermissionName: string([]byte{0xFF}), // Invalid UTF-8
					},
				},
			},
			forkController: &integrationMock.ForkControllerStub{
				KdaFprCalled: func() bool { return true },
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidParameter,
		},
		{
			name:   "Update account error",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_Owner,
						Signers: []*transaction.AccKey{
							{Address: bytes.Repeat([]byte{1}, 32), Weight: 1},
						},
						Threshold: 1,
					},
				},
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
				UpdateUserCalled: func(state.AccountHandler) error {
					return errors.New("update error")
				},
			},
			expectedCode:  transaction.Transaction_SaveAccountError,
			expectedError: errors.New("update error"),
		},
		{
			name:   "Successful update with user permission",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_User,
						Signers: []*transaction.AccKey{
							{Address: bytes.Repeat([]byte{1}, 32), Weight: 1},
						},
						Threshold: 1,
					},
				},
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
				UpdateUserCalled: func(state.AccountHandler) error {
					return nil
				},
			},
			expectedCode: transaction.Transaction_Ok,
		},
		{
			name:   "Successful update with owner permission",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_Owner,
						Signers: []*transaction.AccKey{
							{Address: bytes.Repeat([]byte{1}, 32), Weight: 1},
						},
						Threshold: 1,
					},
				},
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
				UpdateUserCalled: func(state.AccountHandler) error {
					return nil
				},
			},
			expectedCode: transaction.Transaction_Ok,
		},
		{
			name:   "Error update invalid permission type",
			sender: []byte("sender"),
			tc: &transaction.UpdateAccountPermissionContract{
				Permissions: []*transaction.AccPermission{
					{
						Type: transaction.AccPermission_AccPermissionType(0xFF),
						Signers: []*transaction.AccKey{
							{Address: bytes.Repeat([]byte{1}, 32), Weight: 1},
						},
						Threshold: 1,
					},
				},
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
				UpdateUserCalled: func(state.AccountHandler) error {
					return nil
				},
			},
			expectedCode:  transaction.Transaction_ParameterInvalid,
			expectedError: common.ErrInvalidParameter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup accountsKapp
			accKapp := setupAccountsKapp(t, config.EnableEpochs{})
			if tt.forkController != nil {
				accKapp.forkController = tt.forkController
			}
			_ = accKapp.SetAccountsCacher(tt.accountsCacher)
			_ = accKapp.SetKAppController(&kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{})
				},
			})

			// Execute test
			code, err := accKapp.UpdatePermission(tt.sender, tt.sender, tt.tc)

			// Assert results
			assert.Equal(t, tt.expectedCode, code)
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuthorizerCanUpdatePermission(t *testing.T) {
	t.Run("NoValidPermission", func(t *testing.T) {
		permissions := []*state.Permission{
			{
				Signers:    []*state.Key{{Address: []byte("other"), Weight: 1}},
				Threshold:  1,
				Operations: transaction.EncodeContractPermissions(transaction.TXContract_UpdateAccountPermissionContractType),
			},
		}
		assert.False(t, authorizerCanUpdatePermission(permissions, []byte("recipient")))
	})

	t.Run("InsufficientWeight", func(t *testing.T) {
		permissions := []*state.Permission{
			{
				Signers:    []*state.Key{{Address: []byte("recipient"), Weight: 1}},
				Threshold:  2,
				Operations: transaction.EncodeContractPermissions(transaction.TXContract_UpdateAccountPermissionContractType),
			},
		}
		assert.False(t, authorizerCanUpdatePermission(permissions, []byte("recipient")))
	})

	t.Run("WrongOperation", func(t *testing.T) {
		permissions := []*state.Permission{
			{
				Signers:    []*state.Key{{Address: []byte("recipient"), Weight: 1}},
				Threshold:  1,
				Type:       state.Permission_User,
				Operations: []byte{0},
			},
		}
		assert.False(t, authorizerCanUpdatePermission(permissions, []byte("recipient")))
	})

	t.Run("ValidPermissionOwner", func(t *testing.T) {
		permissions := []*state.Permission{
			{
				Signers:   []*state.Key{{Address: []byte("recipient"), Weight: 1}},
				Threshold: 1,
			},
		}
		assert.True(t, authorizerCanUpdatePermission(permissions, []byte("recipient")))
	})

	t.Run("ValidPermissionUser", func(t *testing.T) {
		permissions := []*state.Permission{
			{
				Signers:    []*state.Key{{Address: []byte("recipient"), Weight: 1}},
				Threshold:  1,
				Type:       state.Permission_User,
				Operations: transaction.EncodeContractPermissions(transaction.TXContract_UpdateAccountPermissionContractType),
			},
		}
		assert.True(t, authorizerCanUpdatePermission(permissions, []byte("recipient")))
	})
}

func TestUpdatePermission_AuthorizerAuthorization(t *testing.T) {
	targetAddr := bytes.Repeat([]byte{2}, 32)
	authorizerAddr := bytes.Repeat([]byte{3}, 32)
	newSigner := bytes.Repeat([]byte{4}, 32)

	tc := &transaction.UpdateAccountPermissionContract{
		Permissions: []*transaction.AccPermission{
			{
				Type: transaction.AccPermission_Owner,
				Signers: []*transaction.AccKey{
					{Address: newSigner, Weight: 1},
				},
				Threshold: 1,
			},
		},
	}

	newAccount := func(permissions []*state.Permission) *commonMock.UserAccountHandlerStub {
		return &commonMock.UserAccountHandlerStub{
			AddressBytesCalled: func() []byte {
				return targetAddr
			},
			GetPermissionsCalled: func() []*state.Permission {
				return permissions
			},
			SetPermissionsCalled: func([]*state.Permission) {},
		}
	}

	setupKapp := func(account state.UserAccountHandler) *accountsKapp {
		accKapp := setupAccountsKapp(t, config.EnableEpochs{})
		accCacher := &commonMock.AccountsCacherStub{
			GetExistingUserCalled: func([]byte) (state.UserAccountHandler, error) {
				return account, nil
			},
			UpdateUserCalled: func(state.AccountHandler) error {
				return nil
			},
		}
		_ = accKapp.SetAccountsCacher(accCacher)
		_ = accKapp.SetKAppController(&kvmStub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext {
				return kapp.NewKappContext(kapp.ArgsNewKAppContext{})
			},
		})
		return accKapp
	}

	t.Run("RejectsUnauthorizedAuthorizer", func(t *testing.T) {
		account := newAccount([]*state.Permission{
			{
				Signers:    []*state.Key{{Address: targetAddr, Weight: 1}},
				Threshold:  1,
				Operations: transaction.EncodeContractPermissions(transaction.TXContract_UpdateAccountPermissionContractType),
			},
		})
		accKapp := setupKapp(account)

		code, err := accKapp.UpdatePermission(authorizerAddr, targetAddr, tc)

		assert.Equal(t, transaction.Transaction_ParameterInvalid, code)
		assert.Equal(t, common.ErrNoPermission, err)
	})

	t.Run("AcceptsAuthorizedSigner", func(t *testing.T) {
		account := newAccount([]*state.Permission{
			{
				Signers:    []*state.Key{{Address: authorizerAddr, Weight: 1}},
				Threshold:  1,
				Type:       state.Permission_User,
				Operations: transaction.EncodeContractPermissions(transaction.TXContract_UpdateAccountPermissionContractType),
			},
		})
		accKapp := setupKapp(account)

		code, err := accKapp.UpdatePermission(authorizerAddr, targetAddr, tc)

		assert.Equal(t, transaction.Transaction_Ok, code)
		assert.NoError(t, err)
	})
}

func TestUpdatePermission_NoOwnerProvided(t *testing.T) {
	// Setup accountsKapp
	accKapp := setupAccountsKapp(t, config.EnableEpochs{})

	senderAddr := []byte("senderAddress")
	var capturedPermissions []*state.Permission

	// Mock account stub that captures the permissions set
	account := &commonMock.UserAccountHandlerStub{
		AddressBytesCalled: func() []byte {
			return senderAddr
		},
		SetPermissionsCalled: func(permissions []*state.Permission) {
			capturedPermissions = permissions
		},
	}

	accCacher := &commonMock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return account, nil
		},
		UpdateUserCalled: func(account state.AccountHandler) error {
			return nil
		},
	}
	_ = accKapp.SetAccountsCacher(accCacher)

	_ = accKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{})
		},
	})

	// Create a test case with a user permission but no owner permission
	tc := &transaction.UpdateAccountPermissionContract{
		Permissions: []*transaction.AccPermission{
			{
				Type: transaction.AccPermission_User,
				Signers: []*transaction.AccKey{
					{
						Address: bytes.Repeat([]byte{1}, 32),
						Weight:  1,
					},
				},
				Threshold: 1,
			},
		},
	}

	// Execute the update
	code, err := accKapp.UpdatePermission(senderAddr, senderAddr, tc)

	// Verifications
	require.Equal(t, transaction.Transaction_Ok, code)
	require.NoError(t, err)

	// Verify that we have both permissions (user + auto-added owner)
	require.Len(t, capturedPermissions, 2)

	// Verify the user permission
	require.Equal(t, int32(0), capturedPermissions[0].ID)
	require.Equal(t, state.Permission_User, capturedPermissions[0].Type)

	// Verify the auto-added owner permission
	ownerPerm := capturedPermissions[1]
	require.Equal(t, int32(1), ownerPerm.ID)
	require.Equal(t, state.Permission_Owner, ownerPerm.Type)
	require.Equal(t, int64(1), ownerPerm.Threshold)
	require.Empty(t, ownerPerm.Operations)
	require.Len(t, ownerPerm.Signers, 1)
	require.Equal(t, senderAddr, ownerPerm.Signers[0].Address)
	require.Equal(t, int64(1), ownerPerm.Signers[0].Weight)

	// Additional test case: empty permissions list
	tc = &transaction.UpdateAccountPermissionContract{
		Permissions: []*transaction.AccPermission{},
	}

	code, err = accKapp.UpdatePermission(senderAddr, senderAddr, tc)

	require.Equal(t, transaction.Transaction_Ok, code)
	require.NoError(t, err)
	require.Len(t, capturedPermissions, 1)

	// Verify the single auto-added owner permission
	ownerPerm = capturedPermissions[0]
	require.Equal(t, int32(0), ownerPerm.ID)
	require.Equal(t, state.Permission_Owner, ownerPerm.Type)
	require.Equal(t, int64(1), ownerPerm.Threshold)
	require.Empty(t, ownerPerm.Operations)
	require.Len(t, ownerPerm.Signers, 1)
	require.Equal(t, senderAddr, ownerPerm.Signers[0].Address)
	require.Equal(t, int64(1), ownerPerm.Signers[0].Weight)
}

func TestUpdatePermission_WithNamePriorAndAfterFPRFork(t *testing.T) {
	// Setup accountsKapp
	accKapp := setupAccountsKapp(t, config.EnableEpochs{})
	accKapp.forkController = &integrationMock.ForkControllerStub{
		KdaFprCalled: func() bool {
			return false
		},
	}

	senderAddr := []byte("senderAddress")
	var capturedPermissions []*state.Permission

	// Mock account stub that captures the permissions set
	account := &commonMock.UserAccountHandlerStub{
		AddressBytesCalled: func() []byte {
			return senderAddr
		},
		SetPermissionsCalled: func(permissions []*state.Permission) {
			capturedPermissions = permissions
		},
	}

	accCacher := &commonMock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return account, nil
		},
		UpdateUserCalled: func(account state.AccountHandler) error {
			return nil
		},
	}
	_ = accKapp.SetAccountsCacher(accCacher)

	_ = accKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{})
		},
	})

	tc := &transaction.UpdateAccountPermissionContract{
		Permissions: []*transaction.AccPermission{
			{
				Type: transaction.AccPermission_Owner,
				Signers: []*transaction.AccKey{
					{
						Address: bytes.Repeat([]byte{1}, 32),
						Weight:  1,
					},
				},
				PermissionName: "owner",
				Threshold:      1,
			},
		},
	}

	// Execute the update
	code, err := accKapp.UpdatePermission(senderAddr, senderAddr, tc)

	// Verifications
	require.Equal(t, transaction.Transaction_Ok, code)
	require.NoError(t, err)

	// Verify that we have both permissions (user + auto-added owner)
	require.Len(t, capturedPermissions, 1)

	// Verify the user permission
	require.Equal(t, int32(0), capturedPermissions[0].ID)
	require.Equal(t, state.Permission_Owner, capturedPermissions[0].Type)
	require.Equal(t, "", capturedPermissions[0].PermissionName)

	accKapp.forkController = &integrationMock.ForkControllerStub{
		KdaFprCalled: func() bool {
			return true
		},
	}

	// Execute the update.
	// Execute the update
	code, err = accKapp.UpdatePermission(senderAddr, senderAddr, tc)

	// Verifications
	require.Equal(t, transaction.Transaction_Ok, code)
	require.NoError(t, err)

	// Verify that we have both permissions (user + auto-added owner)
	require.Len(t, capturedPermissions, 1)

	// Verify the user permission
	require.Equal(t, int32(0), capturedPermissions[0].ID)
	require.Equal(t, state.Permission_Owner, capturedPermissions[0].Type)
	require.Equal(t, "owner", capturedPermissions[0].PermissionName)
}

func TestIsUninitializedContractAddress(t *testing.T) {
	scaddress, _ := hex.DecodeString("000000000000000005005fed9c659422cd8429ce92f8973bba2a9fb51e0eb3a1")
	// Table test cases
	tests := []struct {
		name                  string
		address               []byte
		accountExists         bool
		accountError          error
		codeHash              []byte
		codeMeta              []byte
		codeLen               int
		expectedUninitialized bool
	}{
		{
			name:                  "Non smart contract address",
			address:               []byte("regular_address"),
			accountExists:         true,
			accountError:          nil,
			codeHash:              []byte("hash"),
			codeMeta:              []byte("meta"),
			codeLen:               100,
			expectedUninitialized: false,
		},
		{
			name:                  "Smart contract address not existing",
			address:               scaddress,
			accountExists:         false,
			accountError:          errors.New("account not found"),
			codeHash:              nil,
			codeMeta:              nil,
			codeLen:               0,
			expectedUninitialized: true,
		},
		{
			name:                  "Smart contract address with empty code hash",
			address:               scaddress,
			accountExists:         true,
			accountError:          nil,
			codeHash:              []byte{},
			codeMeta:              []byte("meta"),
			codeLen:               100,
			expectedUninitialized: false,
		},
		{
			name:                  "Smart contract address with empty code meta",
			address:               scaddress,
			accountExists:         true,
			accountError:          nil,
			codeHash:              []byte("hash"),
			codeMeta:              []byte{},
			codeLen:               100,
			expectedUninitialized: false,
		},
		{
			name:                  "Smart contract address with empty code",
			address:               scaddress,
			accountExists:         true,
			accountError:          nil,
			codeHash:              []byte("hash"),
			codeMeta:              []byte("meta"),
			codeLen:               0,
			expectedUninitialized: false,
		},
		{
			name:                  "Initialized smart contract address",
			address:               scaddress,
			accountExists:         true,
			accountError:          nil,
			codeHash:              []byte("hash"),
			codeMeta:              []byte("meta"),
			codeLen:               100,
			expectedUninitialized: false,
		},
		{
			name:                  "Deleted contract with empty code hash and meta",
			address:               scaddress,
			accountExists:         true,
			accountError:          nil,
			codeHash:              nil,
			codeMeta:              []byte("meta"),
			codeLen:               0,
			expectedUninitialized: false,
		},
		{
			name:                  "Contract with code hash but no actual code",
			address:               scaddress,
			accountExists:         true,
			accountError:          nil,
			codeHash:              []byte("existingHash"),
			codeMeta:              []byte("existingMeta"),
			codeLen:               0,
			expectedUninitialized: false,
		},
	}

	// Execute tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock user account
			mockUserAccount := &commonMock.UserAccountHandlerStub{
				GetCodeHashCalled: func() []byte {
					return tt.codeHash
				},
				GetCodeMetadataCalled: func() []byte {
					return tt.codeMeta
				},
			}

			// Create mock accounts cacher
			mockAccountsCacher := &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					if tt.accountExists {
						return mockUserAccount, tt.accountError
					}
					return nil, tt.accountError
				},
				GetCodeCalled: func(codeHash []byte) []byte {
					if tt.codeLen > 0 {
						return make([]byte, tt.codeLen)
					}
					return []byte{}
				},
			}

			// Create accounts kapp instance
			args := &ArgsNewAccountKApp{
				Marshalizer:    &commonMock.MarshalizerStub{},
				PubkeyConv:     &commonMock.PubkeyConverterStub{},
				ForkController: &integrationMock.ForkControllerStub{},
			}

			accKapp, err := NewAccountKApp(args)
			require.NoError(t, err)

			err = accKapp.SetAccountsCacher(mockAccountsCacher)
			require.NoError(t, err)

			// Test the method
			result := accKapp.isUninitializedContractAddress(tt.address)

			// Verify result
			assert.Equal(t, tt.expectedUninitialized, result,
				"Test case '%s' failed: expected uninitialized=%v, got=%v",
				tt.name, tt.expectedUninitialized, result)
		})
	}
}

////////////////////
// ClaimAllowance //
////////////////////

func TestClaimAllowance(t *testing.T) {
	var (
		errAccNotFound     = errors.New("account not found")
		errGetUserKDA      = errors.New("error getting user KDA")
		errClaimPending    = errors.New("error claiming pending rewards")
		errAddToAllowance  = errors.New("error adding to allowance")
		errClaimBalance    = errors.New("error claiming balance")
		errSetUserKDA      = errors.New("error setting user KDA")
		errUpdateUser      = errors.New("error updating user")
		testSender         = []byte("testSenderAddress")
		testAllowanceGains = map[string]int64{"KLV": 100}
		testPendingRewards = int64(500)
	)

	cases := []struct {
		title             string
		forkController    core.ForkController
		accountsCacher    state.AccountsCacher
		kappController    kapp.KAppController
		claimContract     *transaction.ClaimContract
		expectedErr       error
		expectedTxResCode transaction.Transaction_TXResultCode
	}{
		{
			title:          "Failing to retrieve user account",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, errAccNotFound
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: testSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_ClaimContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			claimContract:     &transaction.ClaimContract{ClaimType: transaction.ClaimContract_AllowanceClaim},
			expectedErr:       errAccNotFound,
			expectedTxResCode: transaction.Transaction_LoadAccountError,
		},
		{
			title:          "Invalid asset ID (non-KLV)",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: testSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_ClaimContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			claimContract: &transaction.ClaimContract{
				ClaimType: transaction.ClaimContract_AllowanceClaim,
				ID:        []byte("INVALID-ASSET"),
			},
			expectedErr:       common.ErrAssetIDInvalid,
			expectedTxResCode: transaction.Transaction_AssetIDInvalid,
		},
		{
			title:          "Failing to get user KDA",
			forkController: &integrationMock.ForkControllerStub{},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return nil, errGetUserKDA
						},
					}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: testSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_ClaimContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			claimContract:     &transaction.ClaimContract{ClaimType: transaction.ClaimContract_AllowanceClaim},
			expectedErr:       errGetUserKDA,
			expectedTxResCode: transaction.Transaction_AccountError,
		},
		{
			title: "V2: Failing to claim pending rewards",
			forkController: &integrationMock.ForkControllerStub{
				EpochRewardsV2Called: func() bool { return true },
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return &kapps.UserKDA{}, nil
						},
					}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: testSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_ClaimContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
				GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
					return &commonMock.ValidatorsKAppStub{
						ClaimPendingRewardsCalled: func(address []byte) (int64, error) {
							return 0, errClaimPending
						},
					}
				},
			},
			claimContract:     &transaction.ClaimContract{ClaimType: transaction.ClaimContract_AllowanceClaim},
			expectedErr:       errClaimPending,
			expectedTxResCode: transaction.Transaction_ClaimError,
		},
		{
			title: "V2: Failing to add pending rewards to allowance",
			forkController: &integrationMock.ForkControllerStub{
				EpochRewardsV2Called: func() bool { return true },
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return &kapps.UserKDA{}, nil
						},
						AddToAllowanceCalled: func(value int64) error {
							return errAddToAllowance
						},
					}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: testSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_ClaimContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
				GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
					return &commonMock.ValidatorsKAppStub{
						ClaimPendingRewardsCalled: func(address []byte) (int64, error) {
							return testPendingRewards, nil
						},
					}
				},
			},
			claimContract:     &transaction.ClaimContract{ClaimType: transaction.ClaimContract_AllowanceClaim},
			expectedErr:       errAddToAllowance,
			expectedTxResCode: transaction.Transaction_ClaimError,
		},
		{
			title: "Failing to claim balance",
			forkController: &integrationMock.ForkControllerStub{
				EpochRewardsV2Called: func() bool { return false },
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return &kapps.UserKDA{}, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return nil, errClaimBalance
						},
					}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: testSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_ClaimContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			claimContract:     &transaction.ClaimContract{ClaimType: transaction.ClaimContract_AllowanceClaim},
			expectedErr:       errClaimBalance,
			expectedTxResCode: transaction.Transaction_ClaimError,
		},
		{
			title: "MaxSupplyExceeded error on claim balance",
			forkController: &integrationMock.ForkControllerStub{
				EpochRewardsV2Called: func() bool { return false },
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return &kapps.UserKDA{}, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return nil, common.ErrMaxSupplyExceeded
						},
					}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: testSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_ClaimContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			claimContract:     &transaction.ClaimContract{ClaimType: transaction.ClaimContract_AllowanceClaim},
			expectedErr:       common.ErrMaxSupplyExceeded,
			expectedTxResCode: transaction.Transaction_MaxSupplyExceeded,
		},
		{
			title: "Failing to set user KDA",
			forkController: &integrationMock.ForkControllerStub{
				EpochRewardsV2Called: func() bool { return false },
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return &kapps.UserKDA{}, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return testAllowanceGains, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return errSetUserKDA
						},
						AddressBytesCalled: func() []byte {
							return testSender
						},
					}, nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: testSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_ClaimContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			claimContract:     &transaction.ClaimContract{ClaimType: transaction.ClaimContract_AllowanceClaim},
			expectedErr:       errSetUserKDA,
			expectedTxResCode: transaction.Transaction_AssetError,
		},
		{
			title: "Failing to update user account",
			forkController: &integrationMock.ForkControllerStub{
				EpochRewardsV2Called: func() bool { return false },
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return &kapps.UserKDA{}, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return testAllowanceGains, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return testSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return errUpdateUser
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: testSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_ClaimContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			claimContract:     &transaction.ClaimContract{ClaimType: transaction.ClaimContract_AllowanceClaim},
			expectedErr:       errUpdateUser,
			expectedTxResCode: transaction.Transaction_SaveAccountError,
		},
		{
			title: "Success without V2 epoch rewards",
			forkController: &integrationMock.ForkControllerStub{
				EpochRewardsV2Called: func() bool { return false },
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return &kapps.UserKDA{}, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return testAllowanceGains, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return testSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: testSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_ClaimContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			claimContract:     &transaction.ClaimContract{ClaimType: transaction.ClaimContract_AllowanceClaim},
			expectedErr:       nil,
			expectedTxResCode: transaction.Transaction_Ok,
		},
		{
			title: "Success with V2 epoch rewards and pending rewards",
			forkController: &integrationMock.ForkControllerStub{
				EpochRewardsV2Called: func() bool { return true },
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return &kapps.UserKDA{}, nil
						},
						AddToAllowanceCalled: func(value int64) error {
							assert.Equal(t, testPendingRewards, value)
							return nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return testAllowanceGains, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return testSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: testSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_ClaimContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
				GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
					return &commonMock.ValidatorsKAppStub{
						ClaimPendingRewardsCalled: func(address []byte) (int64, error) {
							return testPendingRewards, nil
						},
					}
				},
			},
			claimContract:     &transaction.ClaimContract{ClaimType: transaction.ClaimContract_AllowanceClaim},
			expectedErr:       nil,
			expectedTxResCode: transaction.Transaction_Ok,
		},
		{
			title: "Success with V2 but zero pending rewards (no AddToAllowance call)",
			forkController: &integrationMock.ForkControllerStub{
				EpochRewardsV2Called: func() bool { return true },
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							return &kapps.UserKDA{}, nil
						},
						AddToAllowanceCalled: func(value int64) error {
							t.Error("AddToAllowance should not be called when pending rewards is 0")
							return nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return testAllowanceGains, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return testSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: testSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_ClaimContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
				GetValidatorsKAppCalled: func() kapp.ValidatorsKapp {
					return &commonMock.ValidatorsKAppStub{
						ClaimPendingRewardsCalled: func(address []byte) (int64, error) {
							return 0, nil // Zero pending rewards
						},
					}
				},
			},
			claimContract:     &transaction.ClaimContract{ClaimType: transaction.ClaimContract_AllowanceClaim},
			expectedErr:       nil,
			expectedTxResCode: transaction.Transaction_Ok,
		},
		{
			title: "Success with nil asset ID defaults to KLV",
			forkController: &integrationMock.ForkControllerStub{
				EpochRewardsV2Called: func() bool { return false },
			},
			accountsCacher: &commonMock.AccountsCacherStub{
				GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &commonMock.UserAccountHandlerStub{
						GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
							assert.Equal(t, kdautils.KLVIdentifier, assetID)
							return &kapps.UserKDA{}, nil
						},
						ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
							return testAllowanceGains, nil
						},
						SetUserKDACalled: func(assetID []byte, nonce []byte, userKDA *kapps.UserKDA) error {
							return nil
						},
						AddressBytesCalled: func() []byte {
							return testSender
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: testSender,
						ContractID:     0,
						ContractType:   transaction.TXContract_ClaimContractType,
						Block: &block.Block{
							Header: &block.BlockHeader{
								Timestamp: 1000,
								Epoch:     1,
							},
						},
					})
				},
			},
			claimContract: &transaction.ClaimContract{
				ClaimType: transaction.ClaimContract_AllowanceClaim,
				ID:        nil, // nil should default to KLV
			},
			expectedErr:       nil,
			expectedTxResCode: transaction.Transaction_Ok,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			accsKapp, _ := NewAccountKApp(&ArgsNewAccountKApp{
				Hasher:         &commonMock.HasherMock{},
				Marshalizer:    &commonMock.MarshalizerMock{},
				PubkeyConv:     commonMock.NewPubkeyConverterMock(4),
				ForkController: c.forkController,
			})
			require.NoError(t, accsKapp.SetKAppController(c.kappController))
			require.NoError(t, accsKapp.SetAccountsCacher(c.accountsCacher))

			txResCode, err := accsKapp.ClaimAllowance(testSender, c.claimContract)

			assert.Equal(t, c.expectedTxResCode, txResCode)
			if c.expectedErr != nil {
				assert.ErrorIs(t, err, c.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Tests for ComputeRoyalties

func Test_ComputeRoyalties_LoadKDAError(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, nil, common.ErrAssetNotFound
				},
			}
		},
	})

	klvRoyalties, assetRoyalties, err := accountsKapp.ComputeRoyalties([]byte("INVALID-ASSET"), 1000)
	require.Error(t, err)
	assert.Equal(t, int64(0), klvRoyalties)
	assert.Equal(t, int64(0), assetRoyalties)
}

func Test_ComputeRoyalties_NilRoyalties(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						Royalties: nil,
					}, nil
				},
			}
		},
	})

	klvRoyalties, assetRoyalties, err := accountsKapp.ComputeRoyalties([]byte("TEST-ASSET"), 1000)
	require.NoError(t, err)
	assert.Equal(t, int64(0), klvRoyalties)
	assert.Equal(t, int64(0), assetRoyalties)
}

func Test_ComputeRoyalties_WithFixedRoyalty(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						Royalties: &kapps.RoyaltiesData{
							TransferFixed: 50,
						},
					}, nil
				},
			}
		},
	})

	klvRoyalties, assetRoyalties, err := accountsKapp.ComputeRoyalties([]byte("TEST-ASSET"), 1000)
	require.NoError(t, err)
	assert.Equal(t, int64(50), klvRoyalties)
	assert.Equal(t, int64(0), assetRoyalties)
}

func Test_ComputeRoyalties_WithPercentageRoyalty(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						Royalties: &kapps.RoyaltiesData{
							TransferPercentage: []*kapps.RoyaltyData{
								{
									Amount:     0,
									Percentage: 1000, // 10%
								},
							},
						},
					}, nil
				},
			}
		},
	})

	klvRoyalties, assetRoyalties, err := accountsKapp.ComputeRoyalties([]byte("TEST-ASSET"), 10000)
	require.NoError(t, err)
	assert.Equal(t, int64(0), klvRoyalties)
	assert.Equal(t, int64(1000), assetRoyalties) // 10% of 10000
}

func Test_ComputeRoyalties_WithBothRoyalties(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						Royalties: &kapps.RoyaltiesData{
							TransferFixed: 100,
							TransferPercentage: []*kapps.RoyaltyData{
								{
									Amount:     0,
									Percentage: 500, // 5%
								},
							},
						},
					}, nil
				},
			}
		},
	})

	klvRoyalties, assetRoyalties, err := accountsKapp.ComputeRoyalties([]byte("TEST-ASSET"), 10000)
	require.NoError(t, err)
	assert.Equal(t, int64(100), klvRoyalties)
	assert.Equal(t, int64(500), assetRoyalties) // 5% of 10000
}

// Tests for Freeze

func Test_Freeze_InvalidAmount(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetProposalControllerCalled: func() kapps.ActiveProposalController {
			return &commonMock.ProposalControllerStub{
				GetParameterIntCalled: func(p kapps.EnumParameter) int64 {
					if p == kapps.EnumParameter_MinKLVBucketAmount {
						return 100
					}
					return 10
				},
			}
		},
	})

	tc := &transaction.FreezeContract{
		Amount:  0,
		AssetID: kdautils.KLVIdentifier,
	}

	code, err := accountsKapp.Freeze(txSender, tc)
	require.Error(t, err)
	assert.Equal(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ValueInvalid, code)
}

func Test_Freeze_AmountBelowMinimumForKLV(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetProposalControllerCalled: func() kapps.ActiveProposalController {
			return &commonMock.ProposalControllerStub{
				GetParameterIntCalled: func(p kapps.EnumParameter) int64 {
					if p == kapps.EnumParameter_MinKLVBucketAmount {
						return 1000
					}
					return 10
				},
			}
		},
	})

	tc := &transaction.FreezeContract{
		Amount:  500, // Below minimum of 1000
		AssetID: nil, // nil defaults to KLV
	}

	code, err := accountsKapp.Freeze(txSender, tc)
	require.Error(t, err)
	assert.Equal(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ValueInvalid, code)
}

func Test_Freeze_LoadAccountError(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetProposalControllerCalled: func() kapps.ActiveProposalController {
			return &commonMock.ProposalControllerStub{
				GetParameterIntCalled: func(p kapps.EnumParameter) int64 {
					return 100
				},
			}
		},
	})

	_ = accountsKapp.SetAccountsCacher(&commonMock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return nil, errors.New("account not found")
		},
	})

	tc := &transaction.FreezeContract{
		Amount:  500,
		AssetID: kdautils.KLVIdentifier,
	}

	code, err := accountsKapp.Freeze(txSender, tc)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_LoadAccountError, code)
}

func Test_Freeze_AssetNotFound(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetProposalControllerCalled: func() kapps.ActiveProposalController {
			return &commonMock.ProposalControllerStub{
				GetParameterIntCalled: func(p kapps.EnumParameter) int64 {
					if p == kapps.EnumParameter_MinKLVBucketAmount {
						return 100
					}
					if p == kapps.EnumParameter_MaxBucketSize {
						return 100
					}
					return 10
				},
			}
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, nil, common.ErrAssetNotFound
				},
			}
		},
	})

	_ = accountsKapp.SetAccountsCacher(&commonMock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return &commonMock.AccountWrapMock{}, nil
		},
	})

	tc := &transaction.FreezeContract{
		Amount:  500,
		AssetID: []byte("INVALID-ASSET"),
	}

	code, err := accountsKapp.Freeze(txSender, tc)
	require.Error(t, err)
	assert.Equal(t, common.ErrAssetNotFound, err)
	assert.Equal(t, transaction.Transaction_KAPPError, code)
}

func Test_ProcessNonFungibleTransfer_NonCanonicalAmount(t *testing.T) {
	sender := bytes.Repeat([]byte{0x11}, 32)
	recipient := bytes.Repeat([]byte{0x22}, 32)
	assetID := []byte("NFT-1234")
	internalID := []byte("7")
	const inflated = int64(1_000_000)

	tests := []struct {
		description  string
		enableEpochs config.EnableEpochs
		// forkActive reports whether FixAuditChangesV3 is active at the current
		// epoch (0). An unset epoch field defaults to 0, so it is active; a field
		// set to 1000 is not yet reached at epoch 0.
		forkActive bool
	}{
		{
			description:  "fork off: legacy behaviour accepts non-canonical amount",
			enableEpochs: config.EnableEpochs{FixAuditChangesV3: 1000},
			forkActive:   false,
		},
		{
			description:  "fork on: non-canonical amount is rejected",
			enableEpochs: config.EnableEpochs{},
			forkActive:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			var subInternalCalled, addInternalCalled bool

			src := &commonMock.UserAccountHandlerStub{
				AddressBytesCalled: func() []byte { return sender },
				SubInternalKDACalled: func(gotAssetID, gotInternalID []byte) ([]byte, error) {
					assert.Equal(t, assetID, gotAssetID)
					assert.Equal(t, internalID, gotInternalID)
					subInternalCalled = true
					return []byte("nft-data"), nil
				},
				SubFromBalanceWithNonceCalled: func(value int64, _, _ []byte, _ bool, _ ...*kapps.UserKDA) error {
					t.Fatalf("balance path called for true NFT with value %d", value)
					return nil
				},
			}
			dst := &commonMock.UserAccountHandlerStub{
				AddressBytesCalled: func() []byte { return recipient },
				AddInternalKDACalled: func(gotAssetID, gotInternalID, data []byte) error {
					assert.Equal(t, assetID, gotAssetID)
					assert.Equal(t, internalID, gotInternalID)
					assert.Equal(t, []byte("nft-data"), data)
					addInternalCalled = true
					return nil
				},
				AddToBalanceWithNonceCalled: func(value int64, _, _ []byte, _ bool, _ ...*kapps.UserKDA) error {
					t.Fatalf("balance path called for true NFT with value %d", value)
					return nil
				},
			}

			accountsKapp := setupAccountsKapp(t, tt.enableEpochs)
			require.NoError(t, accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
				GetCurrentKAppContextCalled: func() kapp.KappContext {
					return kapp.NewKappContext(kapp.ArgsNewKAppContext{
						OriginalSender: sender,
						ContractID:     0,
						Block:          &block.Block{},
					})
				},
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &kvmStub.KDAKappStub{
						GetKDACalled: func(gotAssetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
							assert.Equal(t, assetID, gotAssetID)
							return nil, &kapps.KDAData{
								AssetType:  kapps.KDAData_NonFungible,
								Attributes: &kapps.AttributesData{},
								Properties: &kapps.PropertiesData{},
								Royalties:  &kapps.RoyaltiesData{},
							}, nil
						},
					}
				},
			}))
			require.NoError(t, accountsKapp.SetAccountsCacher(&commonMock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					switch {
					case bytes.Equal(address, sender):
						return src, nil
					case bytes.Equal(address, recipient):
						return dst, nil
					default:
						return nil, fmt.Errorf("unexpected account load %x", address)
					}
				},
				UpdateUserCalled: func(state.AccountHandler) error { return nil },
			}))

			tc := &transaction.TransferContract{
				ToAddress: recipient,
				Amount:    inflated,
				AssetID:   []byte("NFT-1234/7"),
			}

			code, err := accountsKapp.Transfer(transaction.TXContract_SmartContractType, sender, tc)

			if tt.forkActive {
				// Post-fork: a non-canonical NFT amount is rejected before the
				// internal KDA move happens, and the amount is left untouched.
				require.ErrorIs(t, err, common.ErrInvalidValue)
				require.Equal(t, transaction.Transaction_ContractInvalid, code)
				require.False(t, subInternalCalled, "internal KDA path (sub) must not run on rejection")
				require.False(t, addInternalCalled, "internal KDA path (add) must not run on rejection")
				assert.Equal(t, inflated, tc.Amount)
				return
			}

			// Pre-fork: legacy behaviour moves the NFT and leaves the amount untouched.
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, code)
			require.True(t, subInternalCalled, "true NFT was not moved through the internal KDA path (sub)")
			require.True(t, addInternalCalled, "true NFT was not moved through the internal KDA path (add)")
			assert.Equal(t, inflated, tc.Amount)
		})
	}
}

func Test_ProcessNonFungibleTransfer_CanonicalAmountPostFork(t *testing.T) {
	sender := bytes.Repeat([]byte{0x11}, 32)
	recipient := bytes.Repeat([]byte{0x22}, 32)
	assetID := []byte("NFT-1234")
	internalID := []byte("7")

	var subInternalCalled, addInternalCalled bool

	src := &commonMock.UserAccountHandlerStub{
		AddressBytesCalled: func() []byte { return sender },
		SubInternalKDACalled: func(gotAssetID, gotInternalID []byte) ([]byte, error) {
			assert.Equal(t, assetID, gotAssetID)
			assert.Equal(t, internalID, gotInternalID)
			subInternalCalled = true
			return []byte("nft-data"), nil
		},
	}
	dst := &commonMock.UserAccountHandlerStub{
		AddressBytesCalled: func() []byte { return recipient },
		AddInternalKDACalled: func(gotAssetID, gotInternalID, data []byte) error {
			assert.Equal(t, assetID, gotAssetID)
			assert.Equal(t, internalID, gotInternalID)
			assert.Equal(t, []byte("nft-data"), data)
			addInternalCalled = true
			return nil
		},
	}

	// Fork active (FixAuditChangesV3 unset -> epoch 0).
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})
	require.NoError(t, accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext {
			return kapp.NewKappContext(kapp.ArgsNewKAppContext{
				OriginalSender: sender,
				ContractID:     0,
				Block:          &block.Block{},
			})
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func(gotAssetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					assert.Equal(t, assetID, gotAssetID)
					return nil, &kapps.KDAData{
						AssetType:  kapps.KDAData_NonFungible,
						Attributes: &kapps.AttributesData{},
						Properties: &kapps.PropertiesData{},
						Royalties:  &kapps.RoyaltiesData{},
					}, nil
				},
			}
		},
	}))
	require.NoError(t, accountsKapp.SetAccountsCacher(&commonMock.AccountsCacherStub{
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			switch {
			case bytes.Equal(address, sender):
				return src, nil
			case bytes.Equal(address, recipient):
				return dst, nil
			default:
				return nil, fmt.Errorf("unexpected account load %x", address)
			}
		},
		UpdateUserCalled: func(state.AccountHandler) error { return nil },
	}))

	tc := &transaction.TransferContract{
		ToAddress: recipient,
		Amount:    1,
		AssetID:   []byte("NFT-1234/7"),
	}

	code, err := accountsKapp.Transfer(transaction.TXContract_SmartContractType, sender, tc)
	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, code)
	require.True(t, subInternalCalled, "true NFT was not moved through the internal KDA path (sub)")
	require.True(t, addInternalCalled, "true NFT was not moved through the internal KDA path (add)")
	assert.Equal(t, int64(1), tc.Amount)
}

func Test_Freeze_NonFungibleAsset(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetProposalControllerCalled: func() kapps.ActiveProposalController {
			return &commonMock.ProposalControllerStub{
				GetParameterIntCalled: func(p kapps.EnumParameter) int64 {
					if p == kapps.EnumParameter_MinKLVBucketAmount {
						return 100
					}
					if p == kapps.EnumParameter_MaxBucketSize {
						return 100
					}
					return 10
				},
			}
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						AssetType: kapps.KDAData_NonFungible,
					}, nil
				},
			}
		},
	})

	_ = accountsKapp.SetAccountsCacher(&commonMock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return &commonMock.AccountWrapMock{}, nil
		},
	})

	tc := &transaction.FreezeContract{
		Amount:  500,
		AssetID: []byte("NFT-ASSET"),
	}

	code, err := accountsKapp.Freeze(txSender, tc)
	require.Error(t, err)
	assert.Equal(t, common.ErrAssetTypeInvalid, err)
	assert.Equal(t, transaction.Transaction_AssetTypeInvalid, code)
}

func Test_Freeze_StakingNotFound(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetProposalControllerCalled: func() kapps.ActiveProposalController {
			return &commonMock.ProposalControllerStub{
				GetParameterIntCalled: func(p kapps.EnumParameter) int64 {
					if p == kapps.EnumParameter_MinKLVBucketAmount {
						return 100
					}
					if p == kapps.EnumParameter_MaxBucketSize {
						return 100
					}
					return 10
				},
			}
		},
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						AssetType: kapps.KDAData_Fungible,
					}, nil
				},
				GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
					return nil, nil, errors.New("staking not found")
				},
			}
		},
	})

	_ = accountsKapp.SetAccountsCacher(&commonMock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return &commonMock.AccountWrapMock{}, nil
		},
	})

	tc := &transaction.FreezeContract{
		Amount:  500,
		AssetID: []byte("FUNGIBLE-ASSET"),
	}

	code, err := accountsKapp.Freeze(txSender, tc)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_KAPPError, code)
}

// Tests for Delegate

func Test_Delegate_InvalidToAddress(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
	})

	tc := &transaction.DelegateContract{
		ToAddress: []byte("invalid"), // Invalid length
		BucketID:  []byte("bucket1"),
	}

	code, err := accountsKapp.Delegate(txSender, tc)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_AccountError, code)
}

func Test_Delegate_EmptyBucketID(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
	})

	tc := &transaction.DelegateContract{
		ToAddress: make([]byte, 32), // Valid length address (pubkeyConv.Len() = 32)
		BucketID:  nil,              // Empty bucket ID
	}

	code, err := accountsKapp.Delegate(txSender, tc)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_BucketIDInvalid, code)
}

// Tests for Undelegate

func Test_Undelegate_EmptyBucketID(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
	})

	tc := &transaction.UndelegateContract{
		BucketID: nil, // Empty bucket ID
	}

	code, err := accountsKapp.Undelegate(txSender, tc)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_BucketIDInvalid, code)
}

func Test_Undelegate_LoadAccountError(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
	})

	_ = accountsKapp.SetAccountsCacher(&commonMock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return nil, errors.New("account not found")
		},
	})

	tc := &transaction.UndelegateContract{
		BucketID: []byte("bucket1"),
	}

	code, err := accountsKapp.Undelegate(txSender, tc)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_LoadAccountError, code)
}

// Tests for Withdraw

func Test_Withdraw_LoadAccountError(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
	})

	_ = accountsKapp.SetAccountsCacher(&commonMock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return nil, errors.New("account not found")
		},
	})

	tc := &transaction.WithdrawContract{
		AssetID: kdautils.KLVIdentifier,
	}

	code, err := accountsKapp.Withdraw(txSender, tc)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_LoadAccountError, code)
}

// Tests for SetAccountName

func Test_SetAccountName_InvalidUTF8Name(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
	})

	tc := &transaction.SetAccountNameContract{
		Name: []byte{0xff, 0xfe}, // Invalid UTF-8
	}

	code, err := accountsKapp.SetAccountName(txSender, tc)
	require.Error(t, err)
	assert.Equal(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, code)
}

func Test_SetAccountName_NameTooLong(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
	})

	// Create a name longer than MaxNameSize (100)
	longName := make([]byte, 101)
	for i := range longName {
		longName[i] = 'a'
	}

	tc := &transaction.SetAccountNameContract{
		Name: longName,
	}

	code, err := accountsKapp.SetAccountName(txSender, tc)
	require.Error(t, err)
	assert.Equal(t, common.ErrInvalidValue, err)
	assert.Equal(t, transaction.Transaction_ParameterInvalid, code)
}

func Test_SetAccountName_LoadAccountError(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
	})

	_ = accountsKapp.SetAccountsCacher(&commonMock.AccountsCacherStub{
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return nil, errors.New("account not found")
		},
	})

	tc := &transaction.SetAccountNameContract{
		Name: []byte("valid_name"),
	}

	code, err := accountsKapp.SetAccountName(txSender, tc)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_LoadAccountError, code)
}

// Tests for ClaimStaking

func Test_ClaimStaking_LoadAccountError(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled: func() kapp.ReceiptsContext {
			return receiptsCtx
		},
		ContractIDCalled: func() int { return 1 },
	}

	_ = accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
	})

	_ = accountsKapp.SetAccountsCacher(&commonMock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return nil, errors.New("account not found")
		},
	})

	tc := &transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim,
		ID:        kdautils.KLVIdentifier,
	}

	code, err := accountsKapp.ClaimStaking(txSender, tc)
	require.Error(t, err)
	assert.Equal(t, transaction.Transaction_LoadAccountError, code)
}

func Test_ClaimStaking_MaxSupplyExceeded(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsCtx },
		ContractIDCalled: func() int { return 1 },
		BlockCalled: func() *block.Block {
			return &block.Block{Header: &block.BlockHeader{Timestamp: 1000, Epoch: 1}}
		},
	}

	require.NoError(t, accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
					return nil, &kapps.StakingData{}, nil
				},
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
				},
			}
		},
	}))

	require.NoError(t, accountsKapp.SetAccountsCacher(&commonMock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return &commonMock.UserAccountHandlerStub{
				GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
					return &kapps.UserKDA{}, nil
				},
				ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
					return nil, common.ErrMaxSupplyExceeded
				},
			}, nil
		},
	}))

	tc := &transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim,
		ID:        kdautils.KLVIdentifier,
	}

	code, err := accountsKapp.ClaimStaking(txSender, tc)
	require.ErrorIs(t, err, common.ErrMaxSupplyExceeded)
	assert.Equal(t, transaction.Transaction_MaxSupplyExceeded, code)
}

func Test_ClaimStaking_MaxSupplyExceeded_Wrapped(t *testing.T) {
	accountsKapp := setupAccountsKapp(t, config.EnableEpochs{})
	wrapped := fmt.Errorf("ctx: %w", common.ErrMaxSupplyExceeded)

	receiptsCtx := &commonMock.ReceiptsContextStub{}
	ctx := &commonMock.KAppContextStub{
		ReceiptsCalled:   func() kapp.ReceiptsContext { return receiptsCtx },
		ContractIDCalled: func() int { return 1 },
		BlockCalled: func() *block.Block {
			return &block.Block{Header: &block.BlockHeader{Timestamp: 1000, Epoch: 1}}
		},
	}

	require.NoError(t, accountsKapp.SetKAppController(&kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetStakingCalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.StakingData, error) {
					return nil, &kapps.StakingData{}, nil
				},
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{AssetType: kapps.KDAData_Fungible}, nil
				},
			}
		},
	}))

	require.NoError(t, accountsKapp.SetAccountsCacher(&commonMock.AccountsCacherStub{
		GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			return &commonMock.UserAccountHandlerStub{
				GetUserKDACalled: func(assetID, nonce []byte, checkDirtData bool) (*kapps.UserKDA, error) {
					return &kapps.UserKDA{}, nil
				},
				ClaimCalled: func(claimType transaction.ClaimContract_EnumClaimType, assetID []byte, epoch uint32, blockTime int64, staking *kapps.StakingData, kda *kapps.KDAData, userKDA *kapps.UserKDA, forkController core.ForkController) (map[string]int64, error) {
					return nil, wrapped
				},
			}, nil
		},
	}))

	tc := &transaction.ClaimContract{
		ClaimType: transaction.ClaimContract_StakingClaim,
		ID:        kdautils.KLVIdentifier,
	}

	code, err := accountsKapp.ClaimStaking(txSender, tc)
	require.ErrorIs(t, err, common.ErrMaxSupplyExceeded)
	assert.Equal(t, transaction.Transaction_MaxSupplyExceeded, code)
}

func Test_claimErrorResultCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		fallback transaction.Transaction_TXResultCode
		want     transaction.Transaction_TXResultCode
	}{
		{
			name:     "direct ErrMaxSupplyExceeded -> MaxSupplyExceeded",
			err:      common.ErrMaxSupplyExceeded,
			fallback: transaction.Transaction_ClaimError,
			want:     transaction.Transaction_MaxSupplyExceeded,
		},
		{
			name:     "wrapped ErrMaxSupplyExceeded -> MaxSupplyExceeded",
			err:      errWrap("ctx", common.ErrMaxSupplyExceeded),
			fallback: transaction.Transaction_ClaimError,
			want:     transaction.Transaction_MaxSupplyExceeded,
		},
		{
			name:     "unrelated error -> fallback",
			err:      errors.New("something else"),
			fallback: transaction.Transaction_ClaimError,
			want:     transaction.Transaction_ClaimError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, claimErrorResultCode(tt.err, tt.fallback))
		})
	}
}

func errWrap(msg string, err error) error {
	return fmt.Errorf("%s: %w", msg, err)
}

func Test_ComputeSplitRoyalties_OverflowGuard(t *testing.T) {
	const pool = int64(1000000)
	const overflowPct = int64(0x80000000)

	run := func(t *testing.T, fixActive bool) (transaction.Transaction_TXResultCode, error, int64, int64) {
		cfg := config.EnableEpochs{SmartContracts: 0}
		if !fixActive {
			cfg.FixMarketBuyOverflow = 1
		}
		accKapp := setupAccountsKapp(t, cfg)

		var credited int64
		_ = accKapp.SetAccountsCacher(&commonMock.AccountsCacherStub{
			LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
				return &commonMock.UserAccountHandlerStub{
					AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
						credited += value
						return nil
					},
				}, nil
			},
			UpdateUserCalled: func(account state.AccountHandler) error { return nil },
		})
		_ = accKapp.SetKAppController(setupKappController(&kvmStub.KAppControllerStub{
			GetCurrentKAppContextCalled: func() kapp.KappContext {
				return kapp.NewKappContext(kapp.ArgsNewKAppContext{ContractID: 0})
			},
		}))

		royaltiesToPay := pool
		status, err := accKapp.computeSplitRoyalties(validAddress, kdautils.KLVIdentifier,
			kapps.KDAData_Fungible, &commonMock.UserAccountHandlerStub{}, pool, overflowPct, &royaltiesToPay)
		return status, err, credited, royaltiesToPay
	}

	t.Run("PreFork_StillMintsKLV", func(t *testing.T) {
		status, err, credited, rtp := run(t, false)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.Greater(t, credited, pool, "pre-fork: split recipient over-paid (mint)")
		require.Less(t, rtp, int64(0), "pre-fork: remainder went negative")
	})

	t.Run("PostFork_Rejected", func(t *testing.T) {
		status, err, credited, rtp := run(t, true)
		require.Error(t, err)
		require.Equal(t, transaction.Transaction_ParameterInvalid, status)
		require.Equal(t, int64(0), credited, "post-fork: nothing credited")
		require.Equal(t, pool, rtp, "post-fork: pool untouched")
	})
}

/////////////////////////////
// Read-only query guard   //
/////////////////////////////

const (
	readOnlySenderStart   = int64(1000)
	readOnlyTransferValue = int64(100)
)

// fungibleKDAController returns a stub KAppController that resolves any asset to
// an active, transferable fungible KDA and supplies a KApp context with a
// receipts collector, so accountsKapp.Transfer can run end to end.
func fungibleKDAController() *kvmStub.KAppControllerStub {
	return setupKappController(&kvmStub.KAppControllerStub{
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						AssetType:  kapps.KDAData_Fungible,
						Attributes: &kapps.AttributesData{},
						Properties: &kapps.PropertiesData{},
					}, nil
				},
			}
		},
	})
}

// fundedUserAccountsAdapter models the trie-backed accounts adapter: every
// LoadAccount/GetExistingAccount returns a FRESH user account instance
// (deserialized copy, as the real trie does), and SaveAccount persists the KLV
// balance back into the shared backing store. `store` is the "committed" state.
func fundedUserAccountsAdapter(store map[string]int64) *commonMock.AccountsStub {
	build := func(address []byte) (state.AccountHandler, error) {
		acc, err := state.NewUserAccount(address)
		if err != nil {
			return nil, err
		}
		if bal := store[string(address)]; bal > 0 {
			if err := acc.AddToBalance(bal, kdautils.KLVIdentifier, false); err != nil {
				return nil, err
			}
		}
		return acc, nil
	}

	return &commonMock.AccountsStub{
		LoadAccountCalled: build,
		GetExistingAccountCalled: func(address []byte) (state.AccountHandler, error) {
			if _, ok := store[string(address)]; !ok {
				return nil, errors.New("account does not exist")
			}
			return build(address)
		},
		SaveAccountCalled: func(account state.AccountHandler) error {
			userAcc, ok := account.(state.UserAccountHandler)
			if !ok {
				return errors.New("unexpected account type")
			}
			store[string(account.AddressBytes())] = userAcc.GetBalance(kdautils.KLVIdentifier, false)
			return nil
		},
	}
}

func readOnlyCacherArgs(adapter *commonMock.AccountsStub) state.ArgsAcccountCacher {
	return state.ArgsAcccountCacher{
		Accounts: adapter,
		Kapps:    &commonMock.AccountsStub{},
		Peers:    &commonMock.AccountsStub{},
	}
}

func readOnlyTransferContract(receiver []byte) *transaction.TransferContract {
	return &transaction.TransferContract{
		ToAddress: receiver,
		AssetID:   kdautils.KLVIdentifier,
		Amount:    readOnlyTransferValue,
	}
}

// A read-only controller refuses the transfer up front even against a fully
// writable cacher, so the guard does not depend on the cacher wiring being right.
func TestTransfer_ReadOnlyController_FailsClosedBeforeAnyMutation(t *testing.T) {
	sender := makeAddress("readOnly-sender")
	receiver := makeAddress("readOnly-receiver")

	store := map[string]int64{string(sender): readOnlySenderStart}
	adapter := fundedUserAccountsAdapter(store)

	// Deliberately a writable production cacher: only the read-only MODE must stop it.
	prodCacher, err := state.NewAccountsCacher(readOnlyCacherArgs(adapter))
	require.NoError(t, err)
	prodCacher.ResetAll(false)

	controller := fungibleKDAController()
	controller.IsReadOnlyCalled = func() bool { return true }

	accKapp := setupAccountsKapp(t, config.EnableEpochs{})
	require.NoError(t, accKapp.SetAccountsCacher(prodCacher))
	require.NoError(t, accKapp.SetKAppController(controller))

	status, err := accKapp.Transfer(transaction.TXContract_TransferContractType, sender, readOnlyTransferContract(receiver))
	require.ErrorIs(t, err, process.ErrReadOnlyKAppMutation)
	require.Equal(t, transaction.Transaction_KAPPError, status)

	require.Equal(t, readOnlySenderStart, store[string(sender)],
		"read-only mode must refuse the transfer before debiting the sender")
	_, receiverExists := store[string(receiver)]
	require.False(t, receiverExists,
		"read-only mode must refuse the transfer before crediting the receiver")
}

// A nil controller is an unknown execution context and must be refused too,
// rather than defaulting to writable.
func TestTransfer_NilController_FailsClosed(t *testing.T) {
	sender := makeAddress("readOnly-sender")
	receiver := makeAddress("readOnly-receiver")

	store := map[string]int64{string(sender): readOnlySenderStart}
	adapter := fundedUserAccountsAdapter(store)

	prodCacher, err := state.NewAccountsCacher(readOnlyCacherArgs(adapter))
	require.NoError(t, err)
	prodCacher.ResetAll(false)

	accKapp := setupAccountsKapp(t, config.EnableEpochs{})
	require.NoError(t, accKapp.SetAccountsCacher(prodCacher))

	status, err := accKapp.Transfer(transaction.TXContract_TransferContractType, sender, readOnlyTransferContract(receiver))
	require.ErrorIs(t, err, process.ErrReadOnlyKAppMutation)
	require.Equal(t, transaction.Transaction_KAPPError, status)

	require.Equal(t, readOnlySenderStart, store[string(sender)],
		"a nil controller must refuse the transfer before debiting the sender")
}

// Bound to a read-only cacher the transfer still reports Ok, but nothing reaches
// committed state and a following SaveAll is a no-op - with cache on or off.
func TestTransfer_QueryBoundToReadOnlyCacher_NeverMutatesCommittedState(t *testing.T) {
	for _, cacheEnabled := range []bool{false, true} {
		name := "cacheDisabled"
		if cacheEnabled {
			name = "cacheEnabled"
		}

		t.Run(name, func(t *testing.T) {
			sender := makeAddress("readOnly-sender")
			receiver := makeAddress("readOnly-receiver")

			store := map[string]int64{string(sender): readOnlySenderStart}
			adapter := fundedUserAccountsAdapter(store)

			queryCacher, err := state.NewReadOnlyAccountsCacher(readOnlyCacherArgs(adapter))
			require.NoError(t, err)
			queryCacher.ResetAll(cacheEnabled)

			accKapp := setupAccountsKapp(t, config.EnableEpochs{})
			require.NoError(t, accKapp.SetAccountsCacher(queryCacher))
			require.NoError(t, accKapp.SetKAppController(fungibleKDAController()))

			status, err := accKapp.Transfer(transaction.TXContract_TransferContractType, sender, readOnlyTransferContract(receiver))
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, status)

			// A read-only SaveAll must never reach the backing store.
			require.NoError(t, queryCacher.SaveAll())

			require.Equal(t, readOnlySenderStart, store[string(sender)],
				"read-only query-path transfer must not debit the sender in committed state")
			_, receiverExists := store[string(receiver)]
			require.False(t, receiverExists,
				"read-only query-path transfer must not credit/create the receiver in committed state")
		})
	}
}

// tripwireCacher fails every accessor and records that it was reached at all, so a
// guard that fires late (or not at all) is visible instead of silently passing.
func tripwireCacher(touched *bool) *commonMock.AccountsCacherStub {
	trip := func() { *touched = true }

	return &commonMock.AccountsCacherStub{
		GetExistingUserCalled: func(_ []byte) (state.UserAccountHandler, error) {
			trip()
			return nil, mockError
		},
		GetExistingKappCalled: func(_ []byte) (state.KAppAccountHandler, error) {
			trip()
			return nil, mockError
		},
		GetExistingPeerCalled: func(_ []byte) (state.PeerAccountHandler, error) {
			trip()
			return nil, mockError
		},
		LoadUserCalled: func(_ []byte) (state.UserAccountHandler, error) {
			trip()
			return nil, mockError
		},
		LoadKAppCalled: func(_ []byte) (state.KAppAccountHandler, error) {
			trip()
			return nil, mockError
		},
		LoadKAppUncachedCalled: func(_ []byte) (state.KAppAccountHandler, error) {
			trip()
			return nil, mockError
		},
		LoadPeerCalled: func(_ []byte) (state.PeerAccountHandler, error) {
			trip()
			return nil, mockError
		},
		SaveUserCalled: func(_ state.AccountHandler) error {
			trip()
			return mockError
		},
		SaveAllCalled: func() error {
			trip()
			return mockError
		},
	}
}

// readOnlyGuardedOps is every accountsKapp entry point that calls checkReadOnly.
// Keep in sync with accounts.go: a new mutating entry point that forgets the guard
// should show up here as a missing case.
func readOnlyGuardedOps() []struct {
	name string
	call func(*accountsKapp) (transaction.Transaction_TXResultCode, error)
} {
	sender := makeAddress("readOnly-sender")
	receiver := makeAddress("readOnly-receiver")

	return []struct {
		name string
		call func(*accountsKapp) (transaction.Transaction_TXResultCode, error)
	}{
		{
			name: "Transfer",
			call: func(a *accountsKapp) (transaction.Transaction_TXResultCode, error) {
				return a.Transfer(transaction.TXContract_TransferContractType, sender, readOnlyTransferContract(receiver))
			},
		},
		{
			name: "Freeze",
			call: func(a *accountsKapp) (transaction.Transaction_TXResultCode, error) {
				return a.Freeze(sender, &transaction.FreezeContract{
					AssetID: kdautils.KLVIdentifier,
					Amount:  readOnlyTransferValue,
				})
			},
		},
		{
			name: "Unfreeze",
			call: func(a *accountsKapp) (transaction.Transaction_TXResultCode, error) {
				return a.Unfreeze(sender, &transaction.UnfreezeContract{
					AssetID:  kdautils.KLVIdentifier,
					BucketID: []byte("bucket"),
				})
			},
		},
		{
			name: "Delegate",
			call: func(a *accountsKapp) (transaction.Transaction_TXResultCode, error) {
				return a.Delegate(sender, &transaction.DelegateContract{
					ToAddress: receiver,
					BucketID:  []byte("bucket"),
				})
			},
		},
		{
			name: "Undelegate",
			call: func(a *accountsKapp) (transaction.Transaction_TXResultCode, error) {
				return a.Undelegate(sender, &transaction.UndelegateContract{
					BucketID: []byte("bucket"),
				})
			},
		},
		{
			name: "Withdraw",
			call: func(a *accountsKapp) (transaction.Transaction_TXResultCode, error) {
				return a.Withdraw(sender, &transaction.WithdrawContract{
					AssetID: kdautils.KLVIdentifier,
				})
			},
		},
		{
			name: "ClaimStaking",
			call: func(a *accountsKapp) (transaction.Transaction_TXResultCode, error) {
				return a.ClaimStaking(sender, &transaction.ClaimContract{
					ID: kdautils.KLVIdentifier,
				})
			},
		},
		{
			name: "ClaimAllowance",
			call: func(a *accountsKapp) (transaction.Transaction_TXResultCode, error) {
				return a.ClaimAllowance(sender, &transaction.ClaimContract{
					ID: kdautils.KLVIdentifier,
				})
			},
		},
		{
			name: "SetAccountName",
			call: func(a *accountsKapp) (transaction.Transaction_TXResultCode, error) {
				return a.SetAccountName(sender, &transaction.SetAccountNameContract{
					Name: []byte("new-name"),
				})
			},
		},
		{
			name: "UpdatePermission",
			call: func(a *accountsKapp) (transaction.Transaction_TXResultCode, error) {
				return a.UpdatePermission(sender, receiver, &transaction.UpdateAccountPermissionContract{})
			},
		},
	}
}

// Every guarded entry point - not just Transfer - must fail closed under a
// read-only controller, before it reads or writes a single account. Asserting on
// the sentinel error is what distinguishes a working guard from an operation that
// merely happens to fail its own validation.
func TestAccountsKapp_AllMutatingOps_ReadOnlyController_FailClosed(t *testing.T) {
	t.Parallel()

	for _, op := range readOnlyGuardedOps() {
		t.Run(op.name, func(t *testing.T) {
			t.Parallel()

			touched := false
			controller := fungibleKDAController()
			controller.IsReadOnlyCalled = func() bool { return true }

			accKapp := setupAccountsKapp(t, config.EnableEpochs{})
			require.NoError(t, accKapp.SetAccountsCacher(tripwireCacher(&touched)))
			require.NoError(t, accKapp.SetKAppController(controller))

			status, err := op.call(accKapp)
			require.ErrorIs(t, err, process.ErrReadOnlyKAppMutation,
				"%s must refuse a read-only execution context", op.name)
			require.Equal(t, transaction.Transaction_KAPPError, status)
			require.False(t, touched,
				"%s must be refused before touching the accounts cacher", op.name)
		})
	}
}

// A nil controller is an unknown execution context: it must be refused rather than
// assumed writable, and must not nil-deref on the GetCurrentKAppContext call that
// follows the guard in most of these operations.
func TestAccountsKapp_AllMutatingOps_NilController_FailClosedNoPanic(t *testing.T) {
	t.Parallel()

	for _, op := range readOnlyGuardedOps() {
		t.Run(op.name, func(t *testing.T) {
			t.Parallel()

			touched := false
			accKapp := setupAccountsKapp(t, config.EnableEpochs{})
			require.NoError(t, accKapp.SetAccountsCacher(tripwireCacher(&touched)))

			require.NotPanics(t, func() {
				status, err := op.call(accKapp)
				require.ErrorIs(t, err, process.ErrReadOnlyKAppMutation,
					"%s must refuse a nil controller", op.name)
				require.Equal(t, transaction.Transaction_KAPPError, status)
			})
			require.False(t, touched,
				"%s must be refused before touching the accounts cacher", op.name)
		})
	}
}

// The guard keys off IsReadOnly only: a writable controller must be let through to
// the operation's own logic, so production behaviour is unchanged. Reaching the
// cacher (and failing there) is the proof that the guard did not short-circuit.
func TestAccountsKapp_AllMutatingOps_WritableController_NotRefused(t *testing.T) {
	t.Parallel()

	for _, op := range readOnlyGuardedOps() {
		t.Run(op.name, func(t *testing.T) {
			t.Parallel()

			touched := false
			controller := fungibleKDAController()
			controller.IsReadOnlyCalled = func() bool { return false }
			controller.GetProposalControllerCalled = func() kapps.ActiveProposalController {
				return &commonMock.ProposalControllerStub{
					GetParameterIntCalled: func(_ kapps.EnumParameter) int64 { return 1 },
				}
			}

			accKapp := setupAccountsKapp(t, config.EnableEpochs{})
			require.NoError(t, accKapp.SetAccountsCacher(tripwireCacher(&touched)))
			require.NoError(t, accKapp.SetKAppController(controller))

			_, err := op.call(accKapp)
			require.NotErrorIs(t, err, process.ErrReadOnlyKAppMutation,
				"%s must not be refused when the controller is writable", op.name)
		})
	}
}

// TestUnfreezeSkipsProposalsTheRemainingFrozenBalanceStillCovers is the closing of KLR-26 rather
// than its narrowing: an account that voted a token amount on every proposal and keeps a large
// bucket frozen used to make every later unfreeze read every one of those proposals. The amount
// now travels in the index, so a covered vote costs nothing and only the vote that has to shrink
// is read, shrunk, and written back with its new amount.
func TestUnfreezeSkipsProposalsTheRemainingFrozenBalanceStillCovers(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(50, 500),
		2: newVotedProposal(120, 500),
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		indexIDs:          []uint64{1, 2},
		proposals:         proposals,
	})

	// 120 frozen before, 40 unfrozen: 80 remain, which still covers the vote of 50 but not 120.
	resCode, err := h.unfreezeVotes(80, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{2}, h.loaded, "only the vote the remaining balance no longer covers is read")
	require.Equal(t, []uint64{2}, h.saved)
	require.Equal(t, int64(50), voterAmountOf(proposals[1]))
	require.Equal(t, int64(80), voterAmountOf(proposals[2]))
	require.Equal(t, 1, h.indexWrites)
	require.Equal(t, []kapp.ProposalVoteIndexEntry{{ProposalID: 1, Amount: 50}, {ProposalID: 2, Amount: 80}}, h.written,
		"the shrunk vote is written back with its new amount so the next unfreeze can skip it too")
}

// TestUnfreezeDropsEntriesForSettledProposalsWithoutReadingThem: an id in none of the controller's
// buckets is a settled proposal, so its entry is dropped on the strength of the controller the
// unfreeze already holds. Without this the index kept every proposal ever voted on while the
// vote stayed covered, and the first deep unfreeze read all of them.
func TestUnfreezeDropsEntriesForSettledProposalsWithoutReadingThem(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(100, 500),
		2: newVotedProposal(100, 500),
	}
	noScan := uint64(0)

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		indexIDs:          []uint64{1, 2},
		proposals:         proposals,
		controller: &kapps.ProposalController{
			ActiveProposals: map[uint32]*kapps.ActiveProposals{500: {ProposalIDs: []uint64{1}}},
		},
		idBound: &noScan,
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{1}, h.loaded, "the settled proposal is never read")
	require.Equal(t, []uint64{2}, h.pruned)
	require.Equal(t, []kapp.ProposalVoteIndexEntry{{ProposalID: 1, Amount: 60}}, h.written)
	require.Equal(t, int64(100), voterAmountOf(proposals[2]), "a settled proposal's record is left as it is")
}

// TestUnfreezeLoadsTheStakingTotalOnce: the KFI staking total cannot change within one unfreeze,
// so it is read once, not once per proposal touched. The saves it makes carry no controller:
// nothing on the unfreeze path changes it.
func TestUnfreezeLoadsTheStakingTotalOnce(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(100, 500),
		2: newVotedProposal(100, 500),
		3: newVotedProposal(100, 500),
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5:    true,
		enableSmartContracts: true,
		indexIDs:             []uint64{1, 2, 3},
		proposals:            proposals,
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{1, 2, 3}, h.saved)
	require.Equal(t, 1, h.stakingLoads, "one staking read per unfreeze, however many proposals it touches")
	require.Equal(t, 0, h.controllerRewrites, "nothing on the unfreeze path changes the controller, so no save carries it")
	for id := uint64(1); id <= 3; id++ {
		require.Equal(t, int64(1000), proposals[id].TotalStaked)
	}
}

// TestUnfreezeRefreshesAnEntryTheIndexOverstated: the proposal is the record. If the index says
// more than the proposal does, the proposal wins, nothing is subtracted, and the entry is
// corrected so the overstatement does not cost a read on every later unfreeze.
func TestUnfreezeRefreshesAnEntryTheIndexOverstated(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(50, 500),
	}

	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		indexEntries:      []kapp.ProposalVoteIndexEntry{{ProposalID: 1, Amount: 100}},
		proposals:         proposals,
	})

	resCode, err := h.unfreezeVotes(80, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{1}, h.loaded)
	require.Empty(t, h.saved, "a covered vote is not subtracted from, whatever the index claimed")
	require.Equal(t, int64(50), voterAmountOf(proposals[1]))
	require.Equal(t, []kapp.ProposalVoteIndexEntry{{ProposalID: 1, Amount: 50}}, h.written)
}

// TestUnfreezeScansActiveProposalsInEpochOrderAndOnlyUpToTheBound covers the pre-fork walk that
// runs while the recorded bound is open. The controller keys its buckets by EpochEnd in a Go map,
// so the walk sorts them: the receipts it emits then come out the same on every node. An id above
// the bound was created after the fork, so its votes are all indexed and it is never read.
// Eight ordered buckets make an unsorted walk fail with overwhelming probability; two would pass
// half the time.
func TestUnfreezeScansActiveProposalsInEpochOrderAndOnlyUpToTheBound(t *testing.T) {
	proposals := map[uint64]*kapps.ProposalData{}
	controller := &kapps.ProposalController{ActiveProposals: map[uint32]*kapps.ActiveProposals{}}
	for id := uint64(1); id <= 8; id++ {
		proposals[id] = newVotedProposal(100, 500)
		controller.ActiveProposals[uint32(100+id)] = &kapps.ActiveProposals{ProposalIDs: []uint64{id}}
	}
	for _, late := range []uint64{50, 60} {
		proposals[late] = newVotedProposal(100, 500)
		controller.ActiveProposals[uint32(100+late)] = &kapps.ActiveProposals{ProposalIDs: []uint64{late}}
	}

	idBound := uint64(8)
	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		controller:        controller,
		proposals:         proposals,
		idBound:           &idBound,
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{1, 2, 3, 4, 5, 6, 7, 8}, h.loaded,
		"buckets are walked in EpochEnd order and the two ids above the bound are never read")
}

// TestUnfreezeToleratesANilBucket: the buckets come back from protobuf, which can hold a nil map
// value, and both paths through the scan walk them. kdautils.ActiveProposalIDs already reads them
// nil-safely and has a test for it; if the scan does not, a KFI unfreeze panics the node instead
// of failing the transaction.
func TestUnfreezeToleratesANilBucket(t *testing.T) {
	for _, fixV5 := range []bool{false, true} {
		t.Run(fmt.Sprintf("fixAuditChangesV5=%v", fixV5), func(t *testing.T) {
			proposals := map[uint64]*kapps.ProposalData{
				1: newVotedProposal(100, 500),
			}

			controller := &kapps.ProposalController{
				ActiveProposals: map[uint32]*kapps.ActiveProposals{
					10: nil,
					20: {ProposalIDs: []uint64{1}},
				},
			}

			idBound := uint64(1)
			h := newProposalVotesHarness(t, proposalVotesConfig{
				fixAuditChangesV5: fixV5,
				controller:        controller,
				proposals:         proposals,
				idBound:           &idBound,
			})

			resCode, err := h.unfreezeVotes(10, 40)

			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, resCode)
			require.Equal(t, []uint64{1}, h.loaded)
			require.Equal(t, int64(60), voterAmountOf(proposals[1]))
		})
	}
}

// TestUnfreezeDoesNotReadAProposalCreatedAfterTheFork is the guard on the transition window, and
// it is written as the attack: proposal 2 belongs to someone else, the unfreezing account has
// never voted on it, and reading it only to discover that is the unmetered work this PR removes.
// It sits in a *lower* bucket than the pre-fork proposal, which is what Create allows and what an
// epoch bound cannot exclude — a bound recorded as 60 here would read it on every unfreeze, for
// as long as the attacker keeps making them. Its id is the thing that gives it away.
func TestUnfreezeDoesNotReadAProposalCreatedAfterTheFork(t *testing.T) {
	const otherVoter = "00112233445566778899aabbccddeeff"

	postFork := &kapps.ProposalData{
		Voters: map[string]*kapps.ProposalData_VoteDetail{
			otherVoter: {Type: kapps.ProposalData_VoteDetail_No, Amount: 100},
		},
		Votes: map[int32]int64{int32(kapps.ProposalData_VoteDetail_No): 500},
	}

	proposals := map[uint64]*kapps.ProposalData{
		1: newVotedProposal(100, 500),
		2: postFork,
	}

	controller := &kapps.ProposalController{
		ActiveProposals: map[uint32]*kapps.ActiveProposals{
			60: {ProposalIDs: []uint64{1}},
			50: {ProposalIDs: []uint64{2}},
		},
	}

	idBound := uint64(1)
	h := newProposalVotesHarness(t, proposalVotesConfig{
		fixAuditChangesV5: true,
		controller:        controller,
		proposals:         proposals,
		idBound:           &idBound,
	})

	resCode, err := h.unfreezeVotes(10, 40)

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, resCode)

	require.Equal(t, []uint64{1}, h.loaded,
		"the proposal created after the fork is never read, though its bucket sorts first")
	require.Equal(t, []uint64{1}, h.saved)
	require.Equal(t, int64(60), voterAmountOf(proposals[1]), "the pre-fork vote is still shrunk")
	require.Equal(t, int64(100), postFork.Voters[otherVoter].Amount, "and the other voter is untouched")
}

const (
	splitRoyaltyAssetID      = "ROY-1234"
	splitRoyaltyMaxSupply    = int64(150)
	splitRoyaltyRate         = uint32(5000) // 50%
	splitRoyaltyTransferSize = int64(100)
	splitRoyaltyPartialSplit = uint32(6000) // 60%
)

// splitRoyaltyAccountsAdapter models the trie-backed accounts adapter for a single
// KDA: every load returns a fresh account copy and SaveAccount writes the
// asset balance back into `store`, the committed state.
func splitRoyaltyAccountsAdapter(store map[string]int64) *commonMock.AccountsStub {
	assetID := []byte(splitRoyaltyAssetID)
	build := func(address []byte) (state.AccountHandler, error) {
		acc, err := state.NewUserAccount(address)
		if err != nil {
			return nil, err
		}
		if bal := store[string(address)]; bal > 0 {
			if err := acc.AddToBalance(bal, assetID, true); err != nil {
				return nil, err
			}
		}
		return acc, nil
	}

	return &commonMock.AccountsStub{
		LoadAccountCalled: build,
		SaveAccountCalled: func(account state.AccountHandler) error {
			userAcc, ok := account.(state.UserAccountHandler)
			if !ok {
				return errors.New("unexpected account type")
			}
			store[string(account.AddressBytes())] = userAcc.GetBalance(assetID, true)
			return nil
		},
	}
}

// splitRoyaltyKDAController resolves the asset to a fungible KDA with InitialSupply ==
// MaxSupply, a 50% transfer royalty paid to royaltyAddr and splitPercent of it
// split to splitAddr
func splitRoyaltyKDAController(royaltyAddr, splitAddr []byte, splitPercent uint32) *kvmStub.KAppControllerStub {
	return setupKappController(&kvmStub.KAppControllerStub{
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, &kapps.KDAData{
						ID:                []byte(splitRoyaltyAssetID),
						AssetType:         kapps.KDAData_Fungible,
						InitialSupply:     splitRoyaltyMaxSupply,
						MaxSupply:         splitRoyaltyMaxSupply,
						CirculatingSupply: splitRoyaltyMaxSupply,
						Attributes:        &kapps.AttributesData{},
						Properties:        &kapps.PropertiesData{},
						Royalties: &kapps.RoyaltiesData{
							Address: royaltyAddr,
							TransferPercentage: []*kapps.RoyaltyData{
								{Amount: 1_000_000, Percentage: splitRoyaltyRate},
							},
							SplitRoyalties: map[string]*kapps.RoyaltySplitData{
								hex.EncodeToString(splitAddr): {PercentTransferPercentage: splitPercent},
							},
						},
					}, nil
				},
			}
		},
	})
}

func splitRoyaltyTotalSupplyHeld(store map[string]int64) int64 {
	total := int64(0)
	for _, bal := range store {
		total += bal
	}
	return total
}

// TestFullySplitPercentageRoyalty_CannotMintPastSupply runs the split percentage royalty
// scenario end to end through Transfer with the production accounts cacher and
// checks the committed spendable balances against the asset supply.
func TestFullySplitPercentageRoyalty_CannotMintPastSupply(t *testing.T) {
	sender := makeAddress("splitRoyalty-sender")
	receiver := makeAddress("splitRoyalty-receiver")
	owner := makeAddress("splitRoyalty-owner")
	thirdParty := makeAddress("splitRoyalty-third-party")

	fixOn := config.EnableEpochs{}
	// A valid schedule with FixMarketBuyOverflow not yet active at epoch 0.
	fixOff := config.EnableEpochs{
		FixMarketBuyOverflow: 1,
		FixAuditChangesV3:    2,
		FixAuditChangesV4:    3,
		FixAuditChangesV5:    4,
	}
	require.NoError(t, fixOff.Validate())

	transfer := func(t *testing.T, cfg config.EnableEpochs, store map[string]int64, from []byte, amount int64, kdaController *kvmStub.KAppControllerStub) {
		t.Helper()

		cacher, err := state.NewAccountsCacher(state.ArgsAcccountCacher{
			Accounts: splitRoyaltyAccountsAdapter(store),
			Kapps:    &commonMock.AccountsStub{},
			Peers:    &commonMock.AccountsStub{},
		})
		require.NoError(t, err)
		// Cache on, as in production since ProcessorFlowITOPrice: one account
		// instance per address for the whole transaction.
		cacher.ResetAll(true)

		accKapp := setupAccountsKapp(t, cfg)
		require.NoError(t, accKapp.SetAccountsCacher(cacher))
		require.NoError(t, accKapp.SetKAppController(kdaController))

		royalty := amount * int64(splitRoyaltyRate) / int64(core.HundredPercent)
		status, err := accKapp.Transfer(transaction.TXContract_TransferContractType, from, &transaction.TransferContract{
			ToAddress:    receiver,
			AssetID:      []byte(splitRoyaltyAssetID),
			Amount:       amount,
			KDARoyalties: royalty,
		})
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, status)
		require.NoError(t, cacher.SaveAll())
	}

	t.Run("fix_on_split_to_sender_conserves_supply", func(t *testing.T) {
		store := map[string]int64{string(sender): splitRoyaltyMaxSupply}
		kdaController := splitRoyaltyKDAController(owner, sender, core.HundredPercent)

		transfer(t, fixOn, store, sender, splitRoyaltyTransferSize, kdaController)

		require.Equal(t, int64(50), store[string(sender)],
			"sender pays 100 + 50 royalty and gets the 50 split back")
		require.Equal(t, splitRoyaltyTransferSize, store[string(receiver)])
		require.Equal(t, int64(0), store[string(owner)], "a 100% split leaves no owner remainder")
		require.Equal(t, splitRoyaltyMaxSupply, splitRoyaltyTotalSupplyHeld(store),
			"spendable balances must equal the supply")

		// A second transfer cannot compound a mint either.
		transfer(t, fixOn, store, sender, 30, kdaController)

		require.Equal(t, int64(20), store[string(sender)])
		require.Equal(t, int64(130), store[string(receiver)])
		require.Equal(t, splitRoyaltyMaxSupply, splitRoyaltyTotalSupplyHeld(store),
			"spendable balances must still equal the supply")
	})

	t.Run("fix_on_split_to_third_party_conserves_supply", func(t *testing.T) {
		store := map[string]int64{string(sender): splitRoyaltyMaxSupply}

		transfer(t, fixOn, store, sender, splitRoyaltyTransferSize,
			splitRoyaltyKDAController(owner, thirdParty, core.HundredPercent))

		require.Equal(t, int64(0), store[string(sender)])
		require.Equal(t, splitRoyaltyTransferSize, store[string(receiver)])
		require.Equal(t, int64(50), store[string(thirdParty)])
		require.Equal(t, int64(0), store[string(owner)], "a 100% split leaves no owner remainder")
		require.Equal(t, splitRoyaltyMaxSupply, splitRoyaltyTotalSupplyHeld(store),
			"spendable balances must equal the supply")
	})

	t.Run("fix_on_partial_split_remainder_to_owner_conserves_supply", func(t *testing.T) {
		store := map[string]int64{string(sender): splitRoyaltyMaxSupply}

		transfer(t, fixOn, store, sender, splitRoyaltyTransferSize,
			splitRoyaltyKDAController(owner, thirdParty, splitRoyaltyPartialSplit))

		require.Equal(t, int64(0), store[string(sender)])
		require.Equal(t, splitRoyaltyTransferSize, store[string(receiver)])
		require.Equal(t, int64(30), store[string(thirdParty)], "60% of the 50 royalty")
		require.Equal(t, int64(20), store[string(owner)], "the 40% remainder goes to the royalty address")
		require.Equal(t, splitRoyaltyMaxSupply, splitRoyaltyTotalSupplyHeld(store),
			"spendable balances must equal the supply")
	})

	t.Run("fix_on_partial_split_remainder_to_sender_conserves_supply", func(t *testing.T) {
		store := map[string]int64{string(sender): splitRoyaltyMaxSupply}

		transfer(t, fixOn, store, sender, splitRoyaltyTransferSize,
			splitRoyaltyKDAController(sender, thirdParty, splitRoyaltyPartialSplit))

		require.Equal(t, int64(20), store[string(sender)],
			"sender pays 100 + 50 royalty and gets the 20 remainder back")
		require.Equal(t, splitRoyaltyTransferSize, store[string(receiver)])
		require.Equal(t, int64(30), store[string(thirdParty)])
		require.Equal(t, splitRoyaltyMaxSupply, splitRoyaltyTotalSupplyHeld(store),
			"spendable balances must equal the supply")
	})

	t.Run("fix_off_split_to_sender_legacy_mint_preserved", func(t *testing.T) {
		store := map[string]int64{string(sender): splitRoyaltyMaxSupply}

		transfer(t, fixOff, store, sender, splitRoyaltyTransferSize,
			splitRoyaltyKDAController(owner, sender, core.HundredPercent))

		require.Equal(t, int64(100), store[string(sender)],
			"sender pays only the 100 transfer and still gets the 50 split")
		require.Equal(t, splitRoyaltyTransferSize, store[string(receiver)])
		require.Equal(t, int64(0), store[string(owner)])
		require.Equal(t, int64(200), splitRoyaltyTotalSupplyHeld(store),
			"pre-fork replay keeps the mint: the royalty pool is never debited")
	})
}

const (
	transferSenderStart   = int64(1_000)
	transferReceiverStart = int64(250)
	transferValue         = int64(400)
)

// transferFixture wires an accountsKapp to two live user accounts through a
// cacher that hands back the same instance on every load (the production cacher
// with the cache on) and counts UpdateUser calls per address in `saved`. The
// context is pinned so receipts can be read back after the call.
type transferFixture struct {
	accKapp  *accountsKapp
	src, dst state.UserAccountHandler
	ctx      kapp.KappContext
	kda      *kapps.KDAData
	saved    map[string]int
}

func newTransferFixture(t *testing.T, assetType kapps.KDAData_EnumAssetType, sender, receiver []byte) transferFixture {
	t.Helper()

	src, err := state.NewUserAccount(sender)
	require.NoError(t, err)
	dst, err := state.NewUserAccount(receiver)
	require.NoError(t, err)

	accounts := map[string]state.UserAccountHandler{string(sender): src, string(receiver): dst}
	saved := make(map[string]int)
	cacher := &commonMock.AccountsCacherStub{
		LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
			acc, ok := accounts[string(address)]
			if !ok {
				return nil, fmt.Errorf("unexpected account load %x", address)
			}
			return acc, nil
		},
		UpdateUserCalled: func(account state.AccountHandler) error {
			saved[string(account.AddressBytes())]++
			return nil
		},
	}

	kda := &kapps.KDAData{
		AssetType:  assetType,
		Attributes: &kapps.AttributesData{},
		Properties: &kapps.PropertiesData{},
	}
	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: sender,
		ContractType:   transaction.TXContract_TransferContractType,
		Block:          &block.Block{},
	})
	controller := &kvmStub.KAppControllerStub{
		GetCurrentKAppContextCalled: func() kapp.KappContext { return ctx },
		GetKDAKAppCalled: func() kapp.KDAKapp {
			return &kvmStub.KDAKappStub{
				GetKDACalled: func(_ []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
					return nil, kda, nil
				},
			}
		},
	}

	accKapp := setupAccountsKapp(t, config.EnableEpochs{})
	require.NoError(t, accKapp.SetAccountsCacher(cacher))
	require.NoError(t, accKapp.SetKAppController(controller))

	return transferFixture{accKapp: accKapp, src: src, dst: dst, ctx: ctx, kda: kda, saved: saved}
}

func requireTransferReceipt(
	t *testing.T,
	receipt *transaction.Transaction_Receipt,
	from, to []byte,
	amount int64,
	assetID, internalID []byte,
	assetType kapps.KDAData_EnumAssetType,
) {
	t.Helper()

	require.Len(t, receipt.Data, 7)
	require.Equal(t, byte(txProcess.Transfer), receipt.Data[0][0])
	require.Equal(t, from, receipt.Data[1])
	require.Equal(t, to, receipt.Data[2])
	require.Equal(t, []byte(strconv.FormatInt(amount, 10)), receipt.Data[3])
	require.Equal(t, assetID, receipt.Data[4])
	require.Equal(t, internalID, receipt.Data[5])
	require.Equal(t, byte(assetType), receipt.Data[6][0])
}

func Test_ProcessFungibleTransfer_DebitsSenderAndCreditsReceiver(t *testing.T) {
	assetID := []byte("FUNGI-1234")
	senderAddr := makeAddress("fungible-sender")
	receiverAddr := makeAddress("fungible-receiver")

	f := newTransferFixture(t, kapps.KDAData_Fungible, senderAddr, receiverAddr)
	require.NoError(t, f.src.AddToBalance(transferSenderStart, assetID, true))
	require.NoError(t, f.dst.AddToBalance(transferReceiverStart, assetID, true))

	tc := &transaction.TransferContract{ToAddress: receiverAddr, AssetID: assetID, Amount: transferValue}

	status, err := f.accKapp.processFungibleTransfer(tc, assetID, f.src, f.dst, f.kda)
	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, status)

	require.Equal(t, transferSenderStart-transferValue, f.src.GetBalance(assetID, true),
		"sender must be debited exactly the transferred amount")
	require.Equal(t, transferReceiverStart+transferValue, f.dst.GetBalance(assetID, true),
		"receiver must be credited exactly the transferred amount")

	require.Equal(t, 1, f.saved[string(senderAddr)], "debited sender must be handed to the cacher")
	require.Equal(t, 1, f.saved[string(receiverAddr)], "credited receiver must be handed to the cacher")

	receipts := f.ctx.Receipts().Get()
	require.Len(t, receipts, 1)
	requireTransferReceipt(t, receipts[0], senderAddr, receiverAddr, transferValue, assetID, nil, kapps.KDAData_Fungible)
}

func Test_ProcessNonFungibleTransfer_MovesUnitToReceiver(t *testing.T) {
	assetID := []byte("NONFUNGI-1234")
	internalID := []byte("7")
	nftPayload := []byte("nft-metadata")
	senderAddr := makeAddress("nonFungible-sender")
	receiverAddr := makeAddress("nonFungible-receiver")

	f := newTransferFixture(t, kapps.KDAData_NonFungible, senderAddr, receiverAddr)
	require.NoError(t, f.src.AddInternalKDA(assetID, internalID, nftPayload))

	status, err := f.accKapp.processNonFungibleTransfer(assetID, internalID, f.src, f.dst)
	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, status)

	require.Equal(t, 1, f.saved[string(senderAddr)], "debited sender must be handed to the cacher")
	require.Equal(t, 1, f.saved[string(receiverAddr)], "credited receiver must be handed to the cacher")

	receipts := f.ctx.Receipts().Get()
	require.Len(t, receipts, 1)
	requireTransferReceipt(t, receipts[0], senderAddr, receiverAddr, 1, assetID, internalID, kapps.KDAData_NonFungible)

	status, err = f.accKapp.processNonFungibleTransfer(assetID, internalID, f.src, f.dst)
	require.Error(t, err, "the sender no longer owns the unit, so it cannot be sent twice")
	require.Equal(t, transaction.Transaction_BalanceError, status)

	moved, err := f.dst.SubInternalKDA(assetID, internalID)
	require.NoError(t, err, "receiver must own the unit after the transfer")
	require.Equal(t, nftPayload, moved, "the unit payload must survive the transfer unchanged")
}

// Transfer routes a SemiFungible asset straight into processSemiFungibleTransfer
// (no royalties configured), so this covers the helper and the account-to-account
// path in one go.
func Test_Transfer_SemiFungibleAccountToAccountMovesBalance(t *testing.T) {
	assetID := []byte("SEMI-1234")
	internalID := []byte("1")
	senderAddr := makeAddress("semiFungible-sender")
	receiverAddr := makeAddress("semiFungible-receiver")

	f := newTransferFixture(t, kapps.KDAData_SemiFungible, senderAddr, receiverAddr)
	require.NoError(t, f.src.AddToBalanceWithNonce(transferSenderStart, assetID, internalID, true))
	require.NoError(t, f.dst.AddToBalanceWithNonce(transferReceiverStart, assetID, internalID, true))

	tc := &transaction.TransferContract{ToAddress: receiverAddr, AssetID: []byte("SEMI-1234/1"), Amount: transferValue}

	status, err := f.accKapp.Transfer(transaction.TXContract_TransferContractType, senderAddr, tc)
	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, status)

	require.Equal(t, transferSenderStart-transferValue, f.src.GetBalanceWithNonce(assetID, internalID, true),
		"sender must be debited exactly the transferred quantity of the nonce")
	require.Equal(t, transferReceiverStart+transferValue, f.dst.GetBalanceWithNonce(assetID, internalID, true),
		"receiver must be credited exactly the transferred quantity of the nonce")
	require.Equal(t, int64(0), f.src.GetBalance(assetID, true),
		"a nonce-scoped transfer must not touch the nonce-less balance of the same asset")
	require.Equal(t, int64(0), f.dst.GetBalance(assetID, true),
		"a nonce-scoped transfer must not touch the nonce-less balance of the same asset")

	// Unlike the fungible and non-fungible helpers, processSemiFungibleTransfer
	// never calls UpdateUser. That is by design, not a gap: SFT transfers are
	// gated behind EnableSmartContracts, which on every production config
	// activates after ProcessorFlowITOPrice has turned the accounts cache on,
	// and UpdateUser is a no-op with the cache on. Pinned so the asymmetry is
	// deliberate in code and any change to it is a conscious one.
	require.Equal(t, 0, f.saved[string(senderAddr)],
		"processSemiFungibleTransfer relies on the accounts cache, not UpdateUser, to persist the sender")
	require.Equal(t, 0, f.saved[string(receiverAddr)],
		"processSemiFungibleTransfer relies on the accounts cache, not UpdateUser, to persist the receiver")

	receipts := f.ctx.Receipts().Get()
	require.Len(t, receipts, 1, "a successful transfer emits exactly one transfer receipt and no error receipt")
	requireTransferReceipt(t, receipts[0], senderAddr, receiverAddr, transferValue, assetID, internalID, kapps.KDAData_SemiFungible)
}

func TestTransfer_FungibleWritableCacher_CommitsBothSides(t *testing.T) {
	for _, cacheEnabled := range []bool{false, true} {
		name := "cacheDisabled"
		if cacheEnabled {
			name = "cacheEnabled"
		}

		t.Run(name, func(t *testing.T) {
			sender := makeAddress("writable-sender")
			receiver := makeAddress("writable-receiver")

			store := map[string]int64{string(sender): readOnlySenderStart}
			adapter := fundedUserAccountsAdapter(store)

			prodCacher, err := state.NewAccountsCacher(readOnlyCacherArgs(adapter))
			require.NoError(t, err)
			prodCacher.ResetAll(cacheEnabled)

			accKapp := setupAccountsKapp(t, config.EnableEpochs{})
			require.NoError(t, accKapp.SetAccountsCacher(prodCacher))
			require.NoError(t, accKapp.SetKAppController(fungibleKDAController()))

			status, err := accKapp.Transfer(transaction.TXContract_TransferContractType, sender, readOnlyTransferContract(receiver))
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, status)

			require.NoError(t, prodCacher.SaveAll())

			require.Equal(t, readOnlySenderStart-readOnlyTransferValue, store[string(sender)],
				"a writable transfer must debit the sender in committed state")
			require.Equal(t, readOnlyTransferValue, store[string(receiver)],
				"a writable transfer must credit the receiver in committed state")
		})
	}
}
