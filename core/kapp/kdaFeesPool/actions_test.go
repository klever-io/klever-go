package kdafeespool_test

import (
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/require"
)

// createMockArgsKDAFeesPool creates a minimal valid args for testing
func createMockArgsKDAFeesPool() *kdafeespool.ArgsNewKDAFeesPoolKApp {
	return &kdafeespool.ArgsNewKDAFeesPoolKApp{
		Marshalizer: &marshal.JSONMarshalizer{},
		PubkeyConv: &mock.PubkeyConverterStub{
			LenCalled: func() int {
				return 32 // Standard address length
			},
		},
		ForkController: mock.NewForkControllerStub(),
	}
}

func TestNewKDAFeesPoolKApp(t *testing.T) {
	t.Parallel()

	t.Run("NilMarshalizer", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		args.Marshalizer = nil

		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.Equal(t, common.ErrNilMarshalizer, err)
		require.Nil(t, kapp)
	})

	t.Run("NilPubkeyConv", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		args.PubkeyConv = nil

		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.Equal(t, common.ErrNilPubkeyConverter, err)
		require.Nil(t, kapp)
	})

	t.Run("NilForkController", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		args.ForkController = nil

		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.Equal(t, common.ErrNilForkController, err)
		require.Nil(t, kapp)
	})

	t.Run("Success", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()

		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)
		require.NotNil(t, kapp)
		require.False(t, kapp.IsInterfaceNil())
	})
}

func TestSetAccountsCacher(t *testing.T) {
	t.Parallel()

	t.Run("NilAccountsCacher", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		err = kapp.SetAccountsCacher(nil)
		require.Equal(t, common.ErrNilAccountsAdapter, err)
	})

	t.Run("Success", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		cacher := &mock.AccountsCacherStub{}
		err = kapp.SetAccountsCacher(cacher)
		require.NoError(t, err)
		require.Equal(t, cacher, kapp.GetAccountsCacher())
	})
}

func TestUpdatePool(t *testing.T) {
	t.Parallel()

	t.Run("GetKAppError", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return nil, common.ErrNilAccountsAdapter
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		// Create mock KAppController
		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		poolID := []byte("pool-id")
		assetOwner := []byte("owner")
		sender := []byte("sender")
		info := &transaction.KDAPoolInfo{
			Active:    true,
			FRatioKLV: 1000,
			FRatioKDA: 100,
		}

		code, err := kapp.UpdatePool(poolID, assetOwner, sender, info)
		require.Equal(t, transaction.Transaction_KAPPError, code)
		require.Equal(t, common.ErrNilAccountsAdapter, err)
	})

	t.Run("NewPoolNotAssetOwner", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		poolData := make(map[string][]byte)
		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return poolData[string(key)]
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		// Create mock KAppController
		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		poolID := []byte("pool-id")
		assetOwner := []byte("owner-address")
		sender := []byte("different-sender")
		info := &transaction.KDAPoolInfo{
			Active:    true,
			FRatioKLV: 1000,
			FRatioKDA: 100,
		}

		code, err := kapp.UpdatePool(poolID, assetOwner, sender, info)
		require.Equal(t, transaction.Transaction_AccountNotOwner, code)
		require.Equal(t, common.ErrAccNotOwner, err)
	})

	t.Run("InvalidFRatioKLV", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		poolData := make(map[string][]byte)
		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return poolData[string(key)]
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		// Create mock KAppController
		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		poolID := []byte("pool-id")
		assetOwner := make([]byte, 32) // 32 bytes
		copy(assetOwner, []byte("owner-address"))
		sender := assetOwner
		info := &transaction.KDAPoolInfo{
			Active:       true,
			AdminAddress: assetOwner, // Valid length
			FRatioKLV:    0,          // Invalid
			FRatioKDA:    100,
		}

		code, err := kapp.UpdatePool(poolID, assetOwner, sender, info)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
		require.Equal(t, common.ErrAssetPoolInvalidAmount, err)
	})

	t.Run("InvalidFRatioKDA", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		poolData := make(map[string][]byte)
		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return poolData[string(key)]
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		// Create mock KAppController
		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		poolID := []byte("pool-id")
		assetOwner := make([]byte, 32) // 32 bytes
		copy(assetOwner, []byte("owner-address"))
		sender := assetOwner
		info := &transaction.KDAPoolInfo{
			Active:       true,
			AdminAddress: assetOwner, // Valid length
			FRatioKLV:    1000,
			FRatioKDA:    0, // Invalid
		}

		code, err := kapp.UpdatePool(poolID, assetOwner, sender, info)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
		require.Equal(t, common.ErrAssetPoolInvalidAmount, err)
	})
}

func TestChangePoolOwner(t *testing.T) {
	t.Parallel()

	t.Run("SmartContractsNotEnabled", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		args.ForkController = mock.NewForkControllerStub().SetFork("EnableSmartContracts", false)
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		assetID := []byte("asset-id")
		sender := []byte("sender")
		newOwner := []byte("new-owner")

		code, err := kapp.ChangePoolOwner(assetID, sender, newOwner)
		require.Equal(t, transaction.Transaction_Ok, code)
		require.NoError(t, err)
	})

	t.Run("PoolNotFound", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return nil // Pool not found
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		// Create mock KAppController with receipts
		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		assetID := []byte("asset-id")
		sender := []byte("sender")
		newOwner := []byte("new-owner")

		code, err := kapp.ChangePoolOwner(assetID, sender, newOwner)
		require.Equal(t, transaction.Transaction_Ok, code)
		require.NoError(t, err)
	})

	t.Run("NotPoolOwner", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: []byte("owner"),
			KDA:          []byte("asset-id"),
			AdminAddress: []byte("admin"),
		}
		poolBytes, _ := marshalizer.Marshal(poolData)

		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				if string(key) == "asset-id" {
					return poolBytes
				}
				return nil
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		// Create mock KAppController
		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		assetID := []byte("asset-id")
		sender := []byte("different-sender")
		newOwner := []byte("new-owner")

		code, err := kapp.ChangePoolOwner(assetID, sender, newOwner)
		require.Equal(t, transaction.Transaction_AccountNotOwner, code)
		require.Equal(t, common.ErrAccNotOwner, err)
	})

	t.Run("InvalidNewOwnerAddress", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		owner := []byte("owner-address-32bytes-long!!!")
		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: owner,
			KDA:          []byte("asset-id"),
			AdminAddress: []byte("admin"),
		}
		poolBytes, _ := marshalizer.Marshal(poolData)

		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				if string(key) == "asset-id" {
					return poolBytes
				}
				return nil
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		// Create mock KAppController
		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		assetID := []byte("asset-id")
		sender := owner
		newOwner := []byte("short") // Invalid length

		code, err := kapp.ChangePoolOwner(assetID, sender, newOwner)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
		require.Equal(t, common.ErrAssetPoolInvalidAddress, err)
	})

	t.Run("Success", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		oldOwner := make([]byte, 32)
		copy(oldOwner, []byte("old-owner"))
		newOwner := make([]byte, 32)
		copy(newOwner, []byte("new-owner"))

		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: oldOwner,
			KDA:          []byte("asset-id"),
			AdminAddress: []byte("admin"),
		}
		poolBytes, _ := marshalizer.Marshal(poolData)

		var savedPool *kdafeespool.KDAFeesPoolData
		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				if string(key) == "asset-id" {
					return poolBytes
				}
				return nil
			},
			SetStorageCalled: func(key []byte, value []byte) error {
				if string(key) == "asset-id" {
					savedPool = &kdafeespool.KDAFeesPoolData{}
					_ = marshalizer.Unmarshal(savedPool, value)
				}
				return nil
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
			UpdateKappCalled: func(account state.AccountHandler) error {
				return nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		// Create mock KAppController
		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		assetID := []byte("asset-id")
		sender := oldOwner

		code, err := kapp.ChangePoolOwner(assetID, sender, newOwner)
		require.Equal(t, transaction.Transaction_Ok, code)
		require.NoError(t, err)

		// Verify owner was actually changed
		require.NotNil(t, savedPool)
		require.Equal(t, newOwner, savedPool.OwnerAddress)

		// Verify receipt was added
		require.Len(t, receipts.Get(), 1)
	})
}

func TestGetPoolOwner(t *testing.T) {
	t.Parallel()

	t.Run("PoolNotFound", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return nil
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		assetID := []byte("asset-id")
		owner, err := kapp.GetPoolOwner(assetID)
		require.Equal(t, common.ErrAssetPoolNotFound, err)
		require.Nil(t, owner)
	})

	t.Run("Success", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		expectedOwner := []byte("owner-address")
		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: expectedOwner,
			KDA:          []byte("asset-id"),
			AdminAddress: []byte("admin"),
		}
		poolBytes, _ := marshalizer.Marshal(poolData)

		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				if string(key) == "asset-id" {
					return poolBytes
				}
				return nil
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		assetID := []byte("asset-id")
		owner, err := kapp.GetPoolOwner(assetID)
		require.NoError(t, err)
		require.Equal(t, expectedOwner, owner)
	})
}

func TestDeposit(t *testing.T) {
	t.Parallel()

	t.Run("NilAssetID", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		// Create mock KAppController
		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		sender := []byte("sender")
		contract := &transaction.DepositContract{
			ID:     nil, // Invalid
			Amount: 1000,
		}

		code, err := kapp.Deposit(sender, contract)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
		require.Equal(t, common.ErrInvalidAsset, err)
	})

	t.Run("InvalidAmount", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		// Create mock KAppController
		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		sender := []byte("sender")
		contract := &transaction.DepositContract{
			ID:     []byte("asset-id"),
			Amount: 0, // Invalid
		}

		code, err := kapp.Deposit(sender, contract)
		require.Equal(t, transaction.Transaction_AmountInvalid, code)
		require.Equal(t, common.ErrInvalidValue, err)
	})

	t.Run("PoolNotFound", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return nil // Pool not found
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		// Create mock KAppController
		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		sender := []byte("sender")
		contract := &transaction.DepositContract{
			ID:     []byte("asset-id"),
			Amount: 1000,
		}

		code, err := kapp.Deposit(sender, contract)
		require.Equal(t, transaction.Transaction_KAPPError, code)
		require.Equal(t, common.ErrAssetPoolNotFound, err)
	})

	t.Run("InsufficientBalance", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		sender := []byte("sender-address")
		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: []byte("owner"),
			KDA:          []byte("asset-id"),
			AdminAddress: sender, // Sender is admin
			Active:       true,
		}
		poolBytes, _ := marshalizer.Marshal(poolData)

		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				if string(key) == "asset-id" {
					return poolBytes
				}
				return nil
			},
		}

		userAcc := &mock.UserAccountHandlerStub{
			GetBalanceCalled: func(assetID []byte, cdd bool) int64 {
				return 500 // Insufficient
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
			LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
				return userAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		// Create mock KAppController
		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		contract := &transaction.DepositContract{
			ID:         []byte("asset-id"),
			Amount:     1000,
			CurrencyID: kdautils.KLVIdentifier,
		}

		code, err := kapp.Deposit(sender, contract)
		require.Equal(t, transaction.Transaction_OutOfFunds, code)
		require.Equal(t, common.ErrBalance, err)
	})
}

func TestWithdraw(t *testing.T) {
	t.Parallel()

	t.Run("InvalidAmount", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer

		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: []byte("owner"),
			KDA:          []byte("asset-id"),
			AdminAddress: []byte("admin"),
		}
		poolBytes, _ := marshalizer.Marshal(poolData)

		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return poolBytes
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		// Create mock KAppController
		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		sender := []byte("owner")
		contract := &transaction.WithdrawContract{
			AssetID:    []byte("asset-id"),
			Amount:     0, // Invalid
			CurrencyID: kdautils.KLVIdentifier,
		}

		code, err := kapp.Withdraw(sender, contract)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
		require.Equal(t, common.ErrAssetPoolAmountError, err)
	})

	t.Run("NotAuthorized", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: []byte("owner"),
			KDA:          []byte("asset-id"),
			AdminAddress: []byte("admin"),
		}
		poolBytes, _ := marshalizer.Marshal(poolData)

		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return poolBytes
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		// Create mock KAppController
		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		sender := []byte("unauthorized")
		contract := &transaction.WithdrawContract{
			AssetID:    []byte("asset-id"),
			Amount:     1000,
			CurrencyID: kdautils.KLVIdentifier,
		}

		code, err := kapp.Withdraw(sender, contract)
		require.Equal(t, transaction.Transaction_KAPPError, code)
		require.Equal(t, common.ErrAssetPoolInvalidAddress, err)
	})

	t.Run("InsufficientPoolBalance", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		owner := []byte("owner-address")
		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: owner,
			KDA:          []byte("asset-id"),
			AdminAddress: []byte("admin"),
			KLVBalance:   500, // Insufficient
		}
		poolBytes, _ := marshalizer.Marshal(poolData)

		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return poolBytes
			},
		}

		userAcc := &mock.UserAccountHandlerStub{}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
			LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
				return userAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		// Create mock KAppController
		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		sender := owner
		contract := &transaction.WithdrawContract{
			AssetID:    []byte("asset-id"),
			Amount:     1000, // More than pool balance
			CurrencyID: kdautils.KLVIdentifier,
		}

		code, err := kapp.Withdraw(sender, contract)
		require.Equal(t, transaction.Transaction_OutOfFunds, code)
		require.Equal(t, common.ErrAssetPoolOutOfFunds, err)
	})
}

func TestValidate(t *testing.T) {
	t.Parallel()

	t.Run("NegativeKLVFee", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return &mock.KAppAccountHandlerStub{}, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		feeHandler := &KDAFeeHandlerStub{
			kda:    []byte("asset-id"),
			amount: 100,
		}

		err = kapp.Validate(-1, feeHandler)
		require.Equal(t, common.ErrAssetPoolInvalidAmount, err)
	})

	t.Run("PoolNotActive", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: []byte("owner"),
			KDA:          []byte("asset-id"),
			Active:       false, // Not active
			FRatioKLV:    1000,
			FRatioKDA:    100,
		}
		poolBytes, _ := marshalizer.Marshal(poolData)

		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return poolBytes
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		feeHandler := &KDAFeeHandlerStub{
			kda:    []byte("asset-id"),
			amount: 100,
		}

		err = kapp.Validate(1000, feeHandler)
		require.Equal(t, common.ErrAssetPoolNotActive, err)
	})

	t.Run("InsufficientKLVBalance", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: []byte("owner"),
			KDA:          []byte("asset-id"),
			Active:       true,
			KLVBalance:   500, // Insufficient
			FRatioKLV:    1000,
			FRatioKDA:    100,
		}
		poolBytes, _ := marshalizer.Marshal(poolData)

		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return poolBytes
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		feeHandler := &KDAFeeHandlerStub{
			kda:    []byte("asset-id"),
			amount: 100,
		}

		err = kapp.Validate(1000, feeHandler) // Requesting more than available
		require.Equal(t, common.ErrAssetPoolOutOfFunds, err)
	})

	t.Run("InsufficientKDAAmount", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: []byte("owner"),
			KDA:          []byte("asset-id"),
			Active:       true,
			KLVBalance:   10000,
			FRatioKLV:    1000,
			FRatioKDA:    100,
		}
		poolBytes, _ := marshalizer.Marshal(poolData)

		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return poolBytes
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		feeHandler := &KDAFeeHandlerStub{
			kda:    []byte("asset-id"),
			amount: 50, // Less than required
		}

		err = kapp.Validate(1000, feeHandler)
		require.ErrorContains(t, err, common.ErrAssetPoolAmountError.Error())
	})
}

func TestCompute(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		// Setup pool with ratio 1000:100 (10:1)
		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: []byte("owner"),
			KDA:          []byte("asset-id"),
			Active:       true,
			KLVBalance:   10000,
			FRatioKLV:    1000,
			FRatioKDA:    100,
		}
		poolBytes, _ := marshalizer.Marshal(poolData)

		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return poolBytes
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		feeHandler := &KDAFeeHandlerStub{
			kda:    []byte("asset-id"),
			amount: 100,
		}

		// For 1000 KLV with ratio 1000:100, should require 100 KDA
		value, err := kapp.Compute(1000, feeHandler)
		require.NoError(t, err)
		require.Equal(t, int64(100), value)
	})

	t.Run("PoolNotActive", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: []byte("owner"),
			KDA:          []byte("asset-id"),
			Active:       false, // Not active
			FRatioKLV:    1000,
			FRatioKDA:    100,
		}
		poolBytes, _ := marshalizer.Marshal(poolData)

		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return poolBytes
			},
		}

		cacher := &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		feeHandler := &KDAFeeHandlerStub{
			kda:    []byte("asset-id"),
			amount: 100,
		}

		value, err := kapp.Compute(1000, feeHandler)
		require.Equal(t, common.ErrAssetPoolNotActive, err)
		require.Equal(t, int64(0), value)
	})
}

// KDAFeeHandlerStub is a simple stub for testing KDAFeeHandler interface
type KDAFeeHandlerStub struct {
	kda    []byte
	amount int64
}

func (k *KDAFeeHandlerStub) GetKDA() []byte {
	return k.kda
}

func (k *KDAFeeHandlerStub) GetAmount() int64 {
	return k.amount
}

func (k *KDAFeeHandlerStub) IsInterfaceNil() bool {
	return k == nil
}
