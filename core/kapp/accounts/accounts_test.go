package accounts

import (
	"bytes"
	"encoding/hex"
	"errors"
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
			accsKapp.SetKAppController(c.kappController)
			accsKapp.SetAccountsCacher(c.accountsCacher)

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
		accsKapp.SetKAppController(
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
		accsKapp.SetAccountsCacher(
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
		accsKapp.SetKAppController(
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
		accsKapp.SetAccountsCacher(
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

			accsKapp.SetKAppController(
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
			accsKapp.SetAccountsCacher(
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

			accsKapp.SetKAppController(
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
			accsKapp.SetAccountsCacher(
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

var activeFork = uint32(0)

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
		return kappController
	}

	return &kvmStub.KAppControllerStub{}
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
			accountsKapp.SetAccountsCacher(setupAccCacher(tt.accCacher))

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
			accountsKapp.SetKAppController(setupKappController(tt.kappController))

			_, _, _, status, err := accountsKapp.loadKDA(tt.kdaID)
			assert.Equal(tt.expectedErr, err)
			assert.Equal(tt.expectedStatus, status)
		})
	}
}

func Test_ComputeSplitRoyalties(t *testing.T) {
	fixedValue := int64(100)
	fixedPercentage := int64(20_00)
	royaltiesToPay := int64(0)

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
				transaction.TXContract_TransferContractType,
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
				transaction.TXContract_TransferContractType,
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
				transaction.TXContract_TransferContractType,
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

func Test_Transfer_ShouldFail(t *testing.T) {
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

