package accounts

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
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
