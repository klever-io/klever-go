package accounts

import (
	"encoding/hex"
	"errors"
	"math"
	"testing"

	"bytes"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	vmStub "github.com/klever-io/klever-go/kvm/mock/stub"

	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var inactiveFork = uint32(1)
var activeFork = uint32(0)

var validAddress = hex.EncodeToString(makeAddress("valid"))
var validAddressBytes = makeAddress("valid")
var emptyAccCacher = &mock.AccountsCacherStub{}

var mockError = errors.New("mock-error")

func makeAddress(prefix string) []byte {
	addr := make([]byte, 32)
	copy(addr, []byte(prefix))
	return addr
}

func setupAccCacher(accCacher *mock.AccountsCacherStub) *mock.AccountsCacherStub {
	if accCacher != nil {
		return accCacher
	}

	return &mock.AccountsCacherStub{}
}

func setupKappController(kappController *vmStub.KAppControllerStub) *vmStub.KAppControllerStub {
	if kappController != nil {
		return kappController
	}

	return &vmStub.KAppControllerStub{}
}

func setupAccountsKapp(t *testing.T, cfg config.EnableEpochs) *accountsKapp {
	epochNotifier := &mock.EpochNotifierStub{}
	forkController, err := fork.NewForkController(
		cfg,
		epochNotifier,
	)
	require.NoError(t, err)

	accountArgs := ArgsNewAccountKApp{
		Marshalizer:    &mock.ProtoMarshalizerMock{},
		PubkeyConv:     cryptoMock.NewPubkeyConverterMock(32),
		ForkController: forkController,
	}

	accountsKapp, err := NewAccountKApp(&accountArgs)
	require.NoError(t, err)

	return accountsKapp
}

func Test_NewAccountKApp_NilMarshalizer(t *testing.T) {
	epochNotifier := &mock.EpochNotifierStub{}
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
	epochNotifier := &mock.EpochNotifierStub{}
	forkController, err := fork.NewForkController(
		config.EnableEpochs{},
		epochNotifier,
	)
	require.NoError(t, err)

	accountArgs := ArgsNewAccountKApp{
		Marshalizer:    &mock.ProtoMarshalizerMock{},
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
		accCacher   *mock.AccountsCacherStub
	}{{
		description: "should fail to get user",
		expectedErr: mockError,
		accCacher: &mock.AccountsCacherStub{
			GetExistingUserCalled: func(address []byte) (state.UserAccountHandler, error) {
				return nil, mockError
			},
		},
	}, {
		description: "should work",
		expectedErr: nil,
		accCacher: &mock.AccountsCacherStub{
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
		kappController *vmStub.KAppControllerStub
	}{{
		description: "should not find kda",
		kappController: &vmStub.KAppControllerStub{
			GetKDAKAppCalled: func() kapp.KDAKapp {
				return &vmStub.KDAKappStub{
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
			kappController: &vmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &vmStub.KDAKappStub{
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
			kappController: &vmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &vmStub.KDAKappStub{
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
			kappController: &vmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &vmStub.KDAKappStub{
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
		accCacher      *mock.AccountsCacherStub
		kappController *vmStub.KAppControllerStub
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
			accCacher: &mock.AccountsCacherStub{
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
			accCacher: &mock.AccountsCacherStub{
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
			accCacher: &mock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &mock.UserAccountHandlerStub{
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
			accCacher: &mock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &mock.UserAccountHandlerStub{
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
			acc:         &mock.UserAccountHandlerStub{},
			accCacher: &mock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &mock.UserAccountHandlerStub{
						AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
							return nil
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &vmStub.KAppControllerStub{
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
		accCacher           *mock.AccountsCacherStub
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
		}, {
			description: "same account",
			transactionContract: &transaction.TransferContract{
				ToAddress: makeAddress("valid"),
			},
			sender:         makeAddress("valid"),
			expectedErr:    process.ErrSameSenderAndReceiverAddress,
			expectedStatus: transaction.Transaction_SameAccountError,
		}, {
			description: "load source account error",
			transactionContract: &transaction.TransferContract{
				ToAddress: makeAddress("valid-1"),
			},
			sender: validAddressBytes,
			accCacher: &mock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, mockError
				},
			},
			expectedErr:    mockError,
			expectedStatus: transaction.Transaction_LoadAccountError,
		}, {
			description: "load destination account error",
			transactionContract: &transaction.TransferContract{
				ToAddress: makeAddress("valid-1"),
			},
			sender: validAddressBytes,
			accCacher: &mock.AccountsCacherStub{
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
			accCacher: &mock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, nil
				},
			},
			expectedErr:    nil,
			expectedStatus: transaction.Transaction_Ok,
		}}

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
	royaltyOwner := &mock.UserAccountHandlerStub{
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
		accCacher           *mock.AccountsCacherStub
		kappController      *vmStub.KAppControllerStub
		fprFork             uint32
		expectedErr         error
		expectedStatus      transaction.Transaction_TXResultCode
	}{{
		description: "without royalties",
		kda: &kapps.KDAData{
			Royalties: &kapps.RoyaltiesData{},
		},
		expectedErr:    nil,
		expectedStatus: transaction.Transaction_Ok,
	}, {
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
			accSrc: &mock.UserAccountHandlerStub{
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
			accSrc: &mock.UserAccountHandlerStub{
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
			accSrc: &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
			},
			accCacher: &mock.AccountsCacherStub{
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
						"invalidAddress": &kapps.RoyaltySplitData{},
					},
				},
			},
			transactionContract: &transaction.TransferContract{
				KLVRoyalties: balance,
			},
			accSrc: &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
			},
			accCacher: &mock.AccountsCacherStub{
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
			accSrc: &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
			},
			accCacher: &mock.AccountsCacherStub{
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
			accSrc: &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
			},
			accCacher: &mock.AccountsCacherStub{
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
			accSrc: &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
			},
			accCacher: &mock.AccountsCacherStub{
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &mock.UserAccountHandlerStub{
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
			accSrc: &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
			},
			accCacher: &mock.AccountsCacherStub{
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
			accSrc: &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return balance
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
			},
			accCacher: &mock.AccountsCacherStub{
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return royaltyOwner, nil
				},
			},
			kappController: &vmStub.KAppControllerStub{
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

			status, err := accKapp.processFixedRoyaltiesTransfer(transaction.TXContract_TransferContractType, tt.transactionContract,
				tt.accSrc, tt.accDst, tt.kda)
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
		accCacher           *mock.AccountsCacherStub
		kappController      *vmStub.KAppControllerStub
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
			accSrc: &mock.UserAccountHandlerStub{
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
			accSrc: &mock.UserAccountHandlerStub{
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

			_, status, err := accKapp.validatePercentageRoyaltiesTransfer(transaction.TXContract_TransferContractType,
				tt.transactionContract, tt.kda, tt.accSrc, kdautils.KLVIdentifier)
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
		accCacher           *mock.AccountsCacherStub
		kappController      *vmStub.KAppControllerStub
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
			accSrc: &mock.UserAccountHandlerStub{
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
			accSrc: &mock.UserAccountHandlerStub{
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
						"invalidAddress": &kapps.RoyaltySplitData{},
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
			accSrc: &mock.UserAccountHandlerStub{
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
			accSrc: &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
			},
			accCacher: &mock.AccountsCacherStub{
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &mock.UserAccountHandlerStub{
						AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
							return nil
						},
					}, nil
				},
			},
			kappController: &vmStub.KAppControllerStub{
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
			accSrc: &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
			},
			accCacher: &mock.AccountsCacherStub{
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
			accSrc: &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return mockError
				},
			},
			accCacher: &mock.AccountsCacherStub{
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
			accSrc: &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
			},
			accCacher: &mock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &mock.UserAccountHandlerStub{
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
			accSrc: &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
			},
			accCacher: &mock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &mock.UserAccountHandlerStub{
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
			accSrc: &mock.UserAccountHandlerStub{
				GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
					return 100_000
				},
				SubFromBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
					return nil
				},
			},
			accCacher: &mock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return &mock.UserAccountHandlerStub{
						AddToBalanceCalled: func(value int64, assetID []byte, cdd bool, userKDA ...*kapps.UserKDA) error {
							return nil
						},
					}, nil
				},
				UpdateUserCalled: func(account state.AccountHandler) error {
					return nil
				},
			},
			kappController: &vmStub.KAppControllerStub{
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

			status, err := accKapp.processPercentageRoyaltiesTransfer(transaction.TXContract_TransferContractType,
				tt.transactionContract, kdautils.KLVIdentifier, []byte{0}, tt.accSrc, tt.accDst, tt.kda)
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
		accCacher           *mock.AccountsCacherStub
		kappController      *vmStub.KAppControllerStub
		expectedErr         error
		expectedStatus      transaction.Transaction_TXResultCode
	}{{
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
			accCacher: &mock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, nil
				},
			},
			kappController: &vmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &vmStub.KDAKappStub{
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
			accCacher: &mock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, nil
				},
			},
			kappController: &vmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &vmStub.KDAKappStub{
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
		}, {
			description: "should fail in because asset is paused",
			transactionContract: &transaction.TransferContract{
				ToAddress: makeAddress("valid-1"),
				AssetID:   []byte("valid"),
			},
			sender: validAddressBytes,
			accCacher: &mock.AccountsCacherStub{
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, nil
				},
			},
			kappController: &vmStub.KAppControllerStub{
				GetKDAKAppCalled: func() kapp.KDAKapp {
					return &vmStub.KDAKappStub{
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
