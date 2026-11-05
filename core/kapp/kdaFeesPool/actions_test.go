package kdafeespool_test

import (
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
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

	t.Run("NewPoolWithoutAssetOwner", func(t *testing.T) {
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

		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		poolID := []byte("pool-id")
		adminAddress := make([]byte, 32)
		copy(adminAddress, []byte("admin-address"))
		info := &transaction.KDAPoolInfo{
			Active:       true,
			AdminAddress: adminAddress,
			FRatioKLV:    1000,
			FRatioKDA:    100,
		}

		for _, sender := range [][]byte{nil, {}} {
			code, err := kapp.UpdatePool(poolID, nil, sender, info)
			require.Equal(t, transaction.Transaction_AccountNotOwner, code)
			require.Equal(t, common.ErrAccNotOwner, err)
		}
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

// TestChangePoolOwnerMovesDefaultAdmin pins KLC-2817: a pool admin left at the former
// owner follows the asset to its new owner, so the former owner can neither withdraw
// from nor reconfigure the pool after the transfer.
func TestChangePoolOwnerMovesDefaultAdmin(t *testing.T) {
	t.Parallel()

	addr := func(s string) []byte {
		b := make([]byte, 32)
		copy(b, s)
		return b
	}
	oldOwner := addr("old-owner")
	newOwner := addr("new-owner")
	thirdAdmin := addr("third-admin")
	assetID := []byte("asset-id")
	errReachedReceiver := fmt.Errorf("withdraw passed the permission check")

	tests := []struct {
		name          string
		fixV7         bool
		admin         []byte
		wantAdmin     []byte
		wantFormerRun bool // whether the former owner still passes the permission checks
	}{
		{name: "DefaultAdminFollowsOwner", fixV7: true, admin: oldOwner, wantAdmin: newOwner, wantFormerRun: false},
		{name: "DeliberateAdminStays", fixV7: true, admin: thirdAdmin, wantAdmin: thirdAdmin, wantFormerRun: false},
		{name: "GateOffKeepsFormerOwnerAsAdmin", fixV7: false, admin: oldOwner, wantAdmin: oldOwner, wantFormerRun: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			args := createMockArgsKDAFeesPool()
			marshalizer := &marshal.JSONMarshalizer{}
			args.Marshalizer = marshalizer
			args.ForkController = mock.NewForkControllerStub().SetFork("FixAuditChangesV7", tt.fixV7)
			kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
			require.NoError(t, err)

			poolBytes, err := marshalizer.Marshal(&kdafeespool.KDAFeesPoolData{
				OwnerAddress: oldOwner,
				KDA:          assetID,
				AdminAddress: tt.admin,
				KLVBalance:   100,
				Active:       true,
				FRatioKLV:    1,
				FRatioKDA:    1,
			})
			require.NoError(t, err)
			storage := map[string][]byte{string(assetID): poolBytes}

			kappAcc := &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					return storage[string(key)]
				},
				SetStorageCalled: func(key []byte, value []byte) error {
					storage[string(key)] = value
					return nil
				},
			}
			cacher := &mock.AccountsCacherStub{
				LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
					return kappAcc, nil
				},
				LoadUserCalled: func(address []byte) (state.UserAccountHandler, error) {
					return nil, errReachedReceiver
				},
			}
			_ = kapp.SetAccountsCacher(cacher)

			receipts := &mock.ReceiptsContextStub{}
			controller := &mock.KappsControllerMock{}
			controller.SetCurrentKAppContext(&mock.KAppContextStub{ReceiptsValue: receipts})
			_ = kapp.SetKAppController(controller)

			code, err := kapp.ChangePoolOwner(assetID, oldOwner, newOwner)
			require.NoError(t, err)
			require.Equal(t, transaction.Transaction_Ok, code)

			saved := &kdafeespool.KDAFeesPoolData{}
			require.NoError(t, marshalizer.Unmarshal(saved, storage[string(assetID)]))
			require.Equal(t, newOwner, saved.OwnerAddress)
			require.Equal(t, tt.wantAdmin, saved.AdminAddress)

			// the receipt is unchanged: the indexer refetches the whole pool from state
			require.Len(t, receipts.Get(), 1)
			require.Equal(t, [][]byte{
				{byte(txProcess.UpdateKDAPool), 0},
				assetID,
			}, receipts.Get()[0].Data)

			// the former owner tries to empty the pool
			_, err = kapp.Withdraw(oldOwner, &transaction.WithdrawContract{
				AssetID:    assetID,
				Amount:     100,
				CurrencyID: kdautils.KLVIdentifier,
			})
			if tt.wantFormerRun {
				require.ErrorIs(t, err, errReachedReceiver)
			} else {
				require.ErrorIs(t, err, common.ErrAssetPoolInvalidAddress)
			}

			// and to rewrite its ratios
			code, err = kapp.UpdatePool(assetID, newOwner, oldOwner, &transaction.KDAPoolInfo{
				Active:    true,
				FRatioKLV: 5,
				FRatioKDA: 7,
			})
			if tt.wantFormerRun {
				require.NoError(t, err)
				require.Equal(t, transaction.Transaction_Ok, code)
			} else {
				require.ErrorIs(t, err, common.ErrAccNotOwner)
				require.Equal(t, transaction.Transaction_AccountNotOwner, code)
			}
		})
	}
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
			LoadKAppUncachedCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return &mock.KAppAccountHandlerStub{}, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		feeHandler := &KDAFeeHandlerStub{
			kda:    []byte("asset-id"),
			amount: 100,
		}

		err = kapp.Validate([]byte("sender"), -1, feeHandler)
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
			LoadKAppUncachedCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		feeHandler := &KDAFeeHandlerStub{
			kda:    []byte("asset-id"),
			amount: 100,
		}

		err = kapp.Validate([]byte("sender"), 1000, feeHandler)
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
			LoadKAppUncachedCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		feeHandler := &KDAFeeHandlerStub{
			kda:    []byte("asset-id"),
			amount: 100,
		}

		err = kapp.Validate([]byte("sender"), 1000, feeHandler) // Requesting more than available
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
			LoadKAppUncachedCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return kappAcc, nil
			},
		}
		_ = kapp.SetAccountsCacher(cacher)

		feeHandler := &KDAFeeHandlerStub{
			kda:    []byte("asset-id"),
			amount: 50, // Less than required
		}

		err = kapp.Validate([]byte("sender"), 1000, feeHandler)
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

func TestComputeUncached_ConcurrentWithProcessingWrites(t *testing.T) {
	t.Parallel()

	// block processing mutates the cached fees pool KApp as fees accumulate;
	// external readers must not race on its dirty-data map
	sharedApp, err := state.NewKAppAccount([]byte("fees-pool-kapp-addr-............"))
	require.NoError(t, err)

	args := createMockArgsKDAFeesPool()
	kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
	require.NoError(t, err)
	require.NoError(t, kapp.SetAccountsCacher(&mock.AccountsCacherStub{
		LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return sharedApp, nil
		},
		LoadKAppUncachedCalled: func(address []byte) (state.KAppAccountHandler, error) {
			return state.NewKAppAccount(address)
		},
	}))

	feeHandler := &KDAFeeHandlerStub{
		kda:    []byte("asset-id"),
		amount: 100,
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			_ = sharedApp.SetStorage([]byte(fmt.Sprintf("pool-%d", i)), []byte("data"))
		}
	}()

	for i := 0; i < 500; i++ {
		// pool does not exist on the fresh copy; the error is expected —
		// the assertion here is the absence of a data race
		_, _ = kapp.ComputeUncached(1000, feeHandler)
	}
	<-done
}

func TestUpdatePoolNonPositiveRatio(t *testing.T) {
	t.Parallel()

	t.Run("NegativeRatioRejectedWithFork", func(t *testing.T) {
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

		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		poolID := []byte("pool-id")
		assetOwner := make([]byte, 32)
		copy(assetOwner, []byte("owner-address"))
		sender := assetOwner
		info := &transaction.KDAPoolInfo{
			Active:       true,
			AdminAddress: assetOwner,
			FRatioKLV:    -1000000000,
			FRatioKDA:    -1,
		}

		code, err := kapp.UpdatePool(poolID, assetOwner, sender, info)
		require.Equal(t, transaction.Transaction_ParameterInvalid, code)
		require.Equal(t, common.ErrAssetPoolInvalidAmount, err)
	})

	t.Run("NegativeRatioAcceptedWithoutFork", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		args.ForkController = mock.NewForkControllerStub().SetFork("FixAuditChangesV3", false)
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		poolData := make(map[string][]byte)
		var savedPool *kdafeespool.KDAFeesPoolData
		kappAcc := &mock.KAppAccountHandlerStub{
			GetStorageCalled: func(key []byte) []byte {
				return poolData[string(key)]
			},
			SetStorageCalled: func(key []byte, value []byte) error {
				if string(key) == "pool-id" {
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

		receipts := &mock.ReceiptsContextStub{}
		kappContext := &mock.KAppContextStub{
			ReceiptsValue: receipts,
		}
		controller := &mock.KappsControllerMock{}
		controller.SetCurrentKAppContext(kappContext)
		_ = kapp.SetKAppController(controller)

		poolID := []byte("pool-id")
		assetOwner := make([]byte, 32)
		copy(assetOwner, []byte("owner-address"))
		sender := assetOwner
		info := &transaction.KDAPoolInfo{
			Active:       true,
			AdminAddress: assetOwner,
			FRatioKLV:    -1000000000,
			FRatioKDA:    -1,
		}

		code, err := kapp.UpdatePool(poolID, assetOwner, sender, info)
		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
		require.NotNil(t, savedPool)
		require.Equal(t, int64(-1000000000), savedPool.FRatioKLV)
		require.Equal(t, int64(-1), savedPool.FRatioKDA)
	})
}

func TestComputeOutOfRangePrice(t *testing.T) {
	t.Parallel()

	t.Run("OutOfRangeRejectedWithFork", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		// klvFee * FRatioKDA / FRatioKLV = 1e18 * 1e6 / 1 = 1e24, beyond int64 range
		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: []byte("owner"),
			KDA:          []byte("asset-id"),
			Active:       true,
			KLVBalance:   10000,
			FRatioKLV:    1,
			FRatioKDA:    1000000,
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

		_, err = kapp.Compute(1000000000000000000, feeHandler)
		require.Equal(t, common.ErrAssetPoolInvalidAmount, err)
	})

	t.Run("OutOfRangeWrapsWithoutFork", func(t *testing.T) {
		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		args.ForkController = mock.NewForkControllerStub().SetFork("FixAuditChangesV3", false)
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: []byte("owner"),
			KDA:          []byte("asset-id"),
			Active:       true,
			KLVBalance:   10000,
			FRatioKLV:    1,
			FRatioKDA:    1000000,
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

		_, err = kapp.Compute(1000000000000000000, feeHandler)
		require.NoError(t, err)
	})
}

func TestComputeNeutralizesStoredNonPositiveRatio(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, fixActive bool, fRatioKLV, fRatioKDA int64) (int64, error) {
		t.Helper()

		args := createMockArgsKDAFeesPool()
		marshalizer := &marshal.JSONMarshalizer{}
		args.Marshalizer = marshalizer
		args.ForkController = mock.NewForkControllerStub().SetFork("FixAuditChangesV3", fixActive)
		kapp, err := kdafeespool.NewKDAFeesPoolKApp(args)
		require.NoError(t, err)

		poolData := &kdafeespool.KDAFeesPoolData{
			OwnerAddress: []byte("owner"),
			KDA:          []byte("asset-id"),
			Active:       true,
			KLVBalance:   1000000000,
			FRatioKLV:    fRatioKLV,
			FRatioKDA:    fRatioKDA,
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

		return kapp.Compute(1000, feeHandler)
	}

	t.Run("BothNegativeRejectedWithFork", func(t *testing.T) {
		_, err := run(t, true, -1, -1)
		require.Equal(t, common.ErrAssetPoolInvalidAmount, err)
	})

	t.Run("OneNegativeRejectedWithFork", func(t *testing.T) {
		_, err := run(t, true, 1, -1)
		require.Equal(t, common.ErrAssetPoolInvalidAmount, err)
	})

	t.Run("NegativeNotNeutralizedWithoutFork", func(t *testing.T) {
		value, err := run(t, false, -1, -1)
		require.NoError(t, err)
		require.Equal(t, int64(1000), value)
	})

	t.Run("PositiveRatioUnaffectedWithFork", func(t *testing.T) {
		value, err := run(t, true, 1000, 100)
		require.NoError(t, err)
		require.Equal(t, int64(100), value)
	})
}

// newSwapTestKApp wires a fee pool over an in-memory KApp account. kda is required rather than
// optional on purpose: the asset controls in Swap are only as good as what GetKDA answers, so every
// case has to state the asset it is swapping against instead of inheriting a permissive default.
// kdaErr, when set, is what the KDA KApp answers instead of the asset.
//
// The third return reports the asset identifiers the KDA KApp was asked for, so a test can pin the
// identifier the fee leg derives from the fee rather than only its outcome.
func newSwapTestKApp(
	t *testing.T,
	forkStub *mock.ForkControllerStub,
	kda *kapps.KDAData,
	kdaErr error,
) (kapp.KDAFeesPoolKapp, func() *kdafeespool.KDAFeesPoolData, func() [][]byte) {
	t.Helper()

	marshalizer := &marshal.JSONMarshalizer{}
	args := createMockArgsKDAFeesPool()
	args.Marshalizer = marshalizer
	args.ForkController = forkStub

	feesPool, err := kdafeespool.NewKDAFeesPoolKApp(args)
	require.NoError(t, err)

	stored, err := marshalizer.Marshal(&kdafeespool.KDAFeesPoolData{
		OwnerAddress: []byte("owner"),
		KDA:          []byte("asset-id"),
		Active:       true,
		KLVBalance:   swapTestInitialKLVBalance,
		FRatioKLV:    1000,
		FRatioKDA:    100,
	})
	require.NoError(t, err)

	kappAcc := &mock.KAppAccountHandlerStub{
		GetStorageCalled: func(_ []byte) []byte {
			return stored
		},
		SetStorageCalled: func(_ []byte, value []byte) error {
			stored = value
			return nil
		},
	}

	cacher := &mock.AccountsCacherStub{
		LoadKAppCalled: func(_ []byte) (state.KAppAccountHandler, error) {
			return kappAcc, nil
		},
		LoadKAppUncachedCalled: func(_ []byte) (state.KAppAccountHandler, error) {
			return kappAcc, nil
		},
	}
	require.NoError(t, feesPool.SetAccountsCacher(cacher))

	var requested [][]byte
	controller := &mock.KappsControllerMock{
		KDAKapp: &stub.KDAKappStub{
			GetKDACalled: func(assetID []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
				requested = append(requested, assetID)
				return nil, kda, kdaErr
			},
			// Validate reads the asset uncached because it runs on an interceptor goroutine.
			GetKDAUncachedCalled: func(assetID []byte) (*kapps.KDAData, error) {
				requested = append(requested, assetID)
				return kda, kdaErr
			},
		},
	}
	require.NoError(t, feesPool.SetKAppController(controller))

	readPool := func() *kdafeespool.KDAFeesPoolData {
		pool := &kdafeespool.KDAFeesPoolData{}
		require.NoError(t, marshalizer.Unmarshal(pool, stored))

		return pool
	}

	return feesPool, readPool, func() [][]byte { return requested }
}

const (
	swapTestInitialKLVBalance = int64(1_000_000)
	swapTestKLVFee            = int64(1000)
	// FRatioKDA 100 over FRatioKLV 1000, so the quote is a tenth of the KLV fee.
	swapTestQuote = swapTestKLVFee * 100 / 1000
)

func TestSwapAssetControlsAndDebitAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fixV5     bool
		kda       *kapps.KDAData
		kdaErr    error
		feeKDA    []byte
		amount    int64
		wantDebit int64
		wantErr   error
		wantAsset []byte
	}{
		{
			name:      "after fork the whole signed amount including the tip is debited",
			fixV5:     true,
			kda:       &kapps.KDAData{},
			amount:    swapTestQuote * 500,
			wantDebit: swapTestQuote * 500,
		},
		{
			name:      "after fork an exact fee is accepted",
			fixV5:     true,
			kda:       &kapps.KDAData{},
			amount:    swapTestQuote,
			wantDebit: swapTestQuote,
		},
		{
			name:    "after fork underpayment is rejected",
			fixV5:   true,
			kda:     &kapps.KDAData{},
			amount:  swapTestQuote - 1,
			wantErr: common.ErrAssetPoolAmountError,
		},
		{
			name:      "before fork the whole supplied amount is debited",
			fixV5:     false,
			kda:       &kapps.KDAData{},
			amount:    50000,
			wantDebit: 50000,
		},
		{
			name:    "after fork a paused asset cannot pay the fee",
			fixV5:   true,
			kda:     &kapps.KDAData{Attributes: &kapps.AttributesData{IsPaused: true}},
			amount:  50000,
			wantErr: process.ErrAssetIsPaused,
		},
		{
			name:  "after fork a sender without the transfer role cannot pay the fee",
			fixV5: true,
			kda: &kapps.KDAData{
				Properties: &kapps.PropertiesData{LimitTransfer: true},
				Attributes: &kapps.AttributesData{},
			},
			amount:  50000,
			wantErr: process.ErrKDATransferNotAllowed,
		},
		{
			name:      "before fork a paused asset is still accepted",
			fixV5:     false,
			kda:       &kapps.KDAData{Attributes: &kapps.AttributesData{IsPaused: true}},
			amount:    50000,
			wantDebit: 50000,
		},
		// The two cases below are the happy path of the transfer-role control. Without them an
		// implementation that refused every limit-transfer asset would ship green while making
		// legitimate fee payments in those assets impossible.
		{
			name:  "after fork a sender holding the transfer role can pay the fee",
			fixV5: true,
			kda: &kapps.KDAData{
				Properties: &kapps.PropertiesData{LimitTransfer: true},
				Attributes: &kapps.AttributesData{},
				Roles:      []*kapps.RolesData{{Address: []byte("sender"), HasRoleTransfer: true}},
			},
			amount:    swapTestQuote * 500,
			wantDebit: swapTestQuote * 500,
		},
		{
			name:  "after fork the asset owner can pay the fee in a limit-transfer asset",
			fixV5: true,
			kda: &kapps.KDAData{
				Properties:   &kapps.PropertiesData{LimitTransfer: true},
				Attributes:   &kapps.AttributesData{},
				OwnerAddress: []byte("sender"),
			},
			amount:    swapTestQuote * 500,
			wantDebit: swapTestQuote * 500,
		},
		{
			name:      "after fork an unknown asset cannot pay the fee",
			fixV5:     true,
			kda:       &kapps.KDAData{},
			kdaErr:    common.ErrAssetNotFound,
			amount:    50000,
			wantErr:   common.ErrAssetNotFound,
			wantAsset: []byte("asset-id"),
		},
		{
			name:      "after fork the nonce is stripped from a collection fee identifier",
			fixV5:     true,
			kda:       &kapps.KDAData{},
			feeKDA:    []byte("asset-id/7"),
			amount:    swapTestQuote * 500,
			wantDebit: swapTestQuote * 500,
			wantAsset: []byte("asset-id"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			forkStub := mock.NewForkControllerStub()
			forkStub.FixAuditChangesV5Value = test.fixV5

			feesPool, readPool, requestedAssets := newSwapTestKApp(t, forkStub, test.kda, test.kdaErr)

			var debitedFromSender, credited int64
			subCalled := false
			sender := &mock.UserAccountHandlerStub{
				AddressBytesCalled: func() []byte { return []byte("sender") },
				SubFromBalanceCalled: func(value int64, _ []byte, _ bool, _ ...*kapps.UserKDA) error {
					subCalled = true
					debitedFromSender = value
					return nil
				},
				AddToBalanceCalled: func(value int64, _ []byte, _ bool, _ ...*kapps.UserKDA) error {
					credited = value
					return nil
				},
			}

			feeKDA := test.feeKDA
			if feeKDA == nil {
				feeKDA = []byte("asset-id")
			}

			feeHandler := &KDAFeeHandlerStub{kda: feeKDA, amount: test.amount}
			err := feesPool.Swap(sender, swapTestKLVFee, feeHandler)

			if test.wantAsset != nil {
				require.Equal(t, [][]byte{test.wantAsset}, requestedAssets())
			}

			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				require.False(t, subCalled, "a rejected fee payment must not debit the sender")

				pool := readPool()
				require.Equal(t, int64(0), pool.KDABalance)
				require.Equal(t, swapTestInitialKLVBalance, pool.KLVBalance)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.wantDebit, debitedFromSender)

			// The pool must be credited exactly what the sender was debited. Asserting only the
			// debit would let a credit of info.GetAmount() pass as green while inflating the pool.
			pool := readPool()
			require.Equal(t, test.wantDebit, pool.KDABalance)
			require.Equal(t, swapTestInitialKLVBalance-swapTestKLVFee, pool.KLVBalance)
			require.Equal(t, swapTestKLVFee, credited)
		})
	}
}

func TestValidateRejectsWhateverSwapWouldReject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fixV5   bool
		kda     *kapps.KDAData
		wantErr error
	}{
		{
			name:  "after fork a paused asset is refused at interception",
			fixV5: true,
			kda:   &kapps.KDAData{Attributes: &kapps.AttributesData{IsPaused: true}},

			wantErr: process.ErrAssetIsPaused,
		},
		{
			name:  "after fork a sender without the transfer role is refused at interception",
			fixV5: true,
			kda: &kapps.KDAData{
				Properties: &kapps.PropertiesData{LimitTransfer: true},
				Attributes: &kapps.AttributesData{},
			},
			wantErr: process.ErrKDATransferNotAllowed,
		},
		{
			name:  "before fork interception keeps accepting both",
			fixV5: false,
			kda:   &kapps.KDAData{Attributes: &kapps.AttributesData{IsPaused: true}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			forkStub := mock.NewForkControllerStub()
			forkStub.FixAuditChangesV5Value = test.fixV5

			feesPool, _, _ := newSwapTestKApp(t, forkStub, test.kda, nil)
			feeHandler := &KDAFeeHandlerStub{kda: []byte("asset-id"), amount: 50000}

			err := feesPool.Validate([]byte("sender"), swapTestKLVFee, feeHandler)

			if test.wantErr != nil {
				// A fee payment accepted here but refused by Swap is gossiped network-wide and then
				// dropped without consuming a fee or a nonce, so it can be resubmitted for free.
				require.ErrorIs(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

// swapShareSender records what a share swap debits and credits; balance is what it holds
// of the fee asset.
func swapShareSender(balance int64, debited *int64, credited *int64, subCalled *bool) *mock.UserAccountHandlerStub {
	return &mock.UserAccountHandlerStub{
		AddressBytesCalled: func() []byte { return []byte("sender") },
		GetBalanceCalled:   func(_ []byte, _ bool) int64 { return balance },
		SubFromBalanceCalled: func(value int64, _ []byte, _ bool, _ ...*kapps.UserKDA) error {
			*subCalled = true
			*debited = value
			return nil
		},
		AddToBalanceCalled: func(value int64, _ []byte, _ bool, _ ...*kapps.UserKDA) error {
			*credited = value
			return nil
		},
	}
}

func TestSwapBandwidthFeeSwapsOnlyTheBandwidthShare(t *testing.T) {
	t.Parallel()

	// FRatioKDA 100 over FRatioKLV 1000: the quote is a tenth of the KLV amount
	const (
		bandwidthFee = int64(1000)
		totalFee     = int64(3000)
		overpay      = int64(5)
	)
	signed := totalFee/10 + overpay

	tests := []struct {
		name          string
		bandwidthFee  int64
		totalFee      int64
		amount        int64
		senderBalance int64
		kda           *kapps.KDAData
		wantDebit     int64
		wantErr       error
	}{
		{
			name:          "the bandwidth share is swapped at the pool's ratio",
			bandwidthFee:  bandwidthFee,
			totalFee:      totalFee,
			amount:        signed,
			senderBalance: signed,
			wantDebit:     bandwidthFee / 10,
		},
		{
			name:          "a fractional bandwidth share is rounded up",
			bandwidthFee:  bandwidthFee + 5,
			totalFee:      totalFee,
			amount:        signed,
			senderBalance: signed,
			wantDebit:     bandwidthFee/10 + 1,
		},
		{
			// rounded down it would cost nothing, and a failed transaction would go free
			name:          "a bandwidth share below one KDA unit costs one unit",
			bandwidthFee:  5,
			totalFee:      totalFee,
			amount:        signed,
			senderBalance: signed,
			wantDebit:     1,
		},
		{
			// 1.5 KDA rounds up to 2, but the whole fee of 1.9 KDA was signed as 1
			name:          "the rounded-up share is capped at the signed amount",
			bandwidthFee:  15,
			totalFee:      19,
			amount:        1,
			senderBalance: 1,
			wantDebit:     1,
		},
		{
			name:          "a signed amount below the whole fee is refused",
			bandwidthFee:  bandwidthFee,
			totalFee:      totalFee,
			amount:        totalFee/10 - 1,
			senderBalance: signed,
			wantErr:       common.ErrAssetPoolAmountError,
		},
		{
			name:          "a pool that cannot cover the whole fee is refused",
			bandwidthFee:  bandwidthFee,
			totalFee:      swapTestInitialKLVBalance + 1,
			amount:        swapTestInitialKLVBalance,
			senderBalance: swapTestInitialKLVBalance,
			wantErr:       common.ErrAssetPoolOutOfFunds,
		},
		{
			name:          "a sender holding less than the signed amount is refused",
			bandwidthFee:  bandwidthFee,
			totalFee:      totalFee,
			amount:        signed,
			senderBalance: signed - 1,
			wantErr:       state.ErrInsufficientFunds,
		},
		{
			name:          "a negative bandwidth fee is refused",
			bandwidthFee:  -1,
			totalFee:      totalFee,
			amount:        signed,
			senderBalance: signed,
			wantErr:       common.ErrAssetPoolInvalidAmount,
		},
		{
			name:          "a bandwidth fee above the whole fee is refused",
			bandwidthFee:  totalFee + 1,
			totalFee:      totalFee,
			amount:        signed,
			senderBalance: signed,
			wantErr:       common.ErrAssetPoolInvalidAmount,
		},
		{
			name:          "a paused asset cannot pay the bandwidth share",
			bandwidthFee:  bandwidthFee,
			totalFee:      totalFee,
			amount:        signed,
			senderBalance: signed,
			kda:           &kapps.KDAData{Attributes: &kapps.AttributesData{IsPaused: true}},
			wantErr:       process.ErrAssetIsPaused,
		},
		{
			name:          "a sender without the transfer role cannot pay the bandwidth share",
			bandwidthFee:  bandwidthFee,
			totalFee:      totalFee,
			amount:        signed,
			senderBalance: signed,
			kda: &kapps.KDAData{
				Properties: &kapps.PropertiesData{LimitTransfer: true},
				Attributes: &kapps.AttributesData{},
			},
			wantErr: process.ErrKDATransferNotAllowed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			kda := test.kda
			if kda == nil {
				kda = &kapps.KDAData{}
			}

			forkStub := mock.NewForkControllerStub()
			feesPool, readPool, _ := newSwapTestKApp(t, forkStub, kda, nil)

			var debited, credited int64
			subCalled := false
			sender := swapShareSender(test.senderBalance, &debited, &credited, &subCalled)

			feeHandler := &KDAFeeHandlerStub{kda: []byte("asset-id"), amount: test.amount}
			err := feesPool.SwapBandwidthFee(sender, test.bandwidthFee, test.totalFee, feeHandler)

			pool := readPool()
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				require.False(t, subCalled, "a refused fee payment must not debit the sender")
				require.Equal(t, int64(0), pool.KDABalance)
				require.Equal(t, swapTestInitialKLVBalance, pool.KLVBalance)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.wantDebit, debited)
			require.Equal(t, test.bandwidthFee, credited)
			require.Equal(t, test.wantDebit, pool.KDABalance)
			require.Equal(t, swapTestInitialKLVBalance-test.bandwidthFee, pool.KLVBalance)
		})
	}
}

func TestSwapKAppFeeTakesTheRestOfTheSignedAmount(t *testing.T) {
	t.Parallel()

	const bandwidthFee = int64(1000)
	bandwidthKDA := bandwidthFee / 10

	tests := []struct {
		name         string
		bandwidthFee int64
		kappFee      int64
		amount       int64
		wantDebit    int64
		wantCredit   int64
		wantErr      error
	}{
		{
			name:       "the rest of the signed amount, excess included, pays the KApp fee",
			kappFee:    2000,
			amount:     bandwidthKDA + 200 + 5,
			wantDebit:  200 + 5,
			wantCredit: 2000,
		},
		{
			name:      "an excess with no KApp fee still goes to the pool",
			kappFee:   0,
			amount:    bandwidthKDA + 5,
			wantDebit: 5,
		},
		{
			name:    "nothing left and no KApp fee swaps nothing",
			kappFee: 0,
			amount:  bandwidthKDA,
		},
		{
			// 100.5 KDA of bandwidth rounds up to 101, leaving 198 for a KApp fee quoted
			// at 199.3: the whole fee of 299.8 KDA is covered, so nothing is refused
			name:         "a rest one unit short of the KApp fee's own quote pays it",
			bandwidthFee: bandwidthFee + 5,
			kappFee:      1993,
			amount:       299,
			wantDebit:    198,
			wantCredit:   1993,
		},
		{
			name:    "a signed amount below the whole fee's quote is refused",
			kappFee: 2000,
			amount:  bandwidthKDA + 199,
			wantErr: common.ErrAssetPoolAmountError,
		},
		{
			name:    "a pool that cannot cover the KApp fee is refused",
			kappFee: swapTestInitialKLVBalance + 1,
			amount:  bandwidthKDA + swapTestInitialKLVBalance,
			wantErr: common.ErrAssetPoolOutOfFunds,
		},
		{
			name:    "a negative KApp fee is refused",
			kappFee: -1,
			amount:  bandwidthKDA,
			wantErr: common.ErrAssetPoolInvalidAmount,
		},
		{
			name:    "a signed amount below the bandwidth share is refused",
			kappFee: 2000,
			amount:  bandwidthKDA - 1,
			wantErr: common.ErrAssetPoolAmountError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			forkStub := mock.NewForkControllerStub()
			feesPool, readPool, _ := newSwapTestKApp(t, forkStub, &kapps.KDAData{}, nil)

			var debited, credited int64
			subCalled := false
			sender := swapShareSender(test.amount, &debited, &credited, &subCalled)

			bw := test.bandwidthFee
			if bw == 0 {
				bw = bandwidthFee
			}

			feeHandler := &KDAFeeHandlerStub{kda: []byte("asset-id"), amount: test.amount}
			err := feesPool.SwapKAppFee(sender, bw, test.kappFee, feeHandler)

			pool := readPool()
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				require.False(t, subCalled, "a refused fee payment must not debit the sender")
				require.Equal(t, int64(0), pool.KDABalance)
				require.Equal(t, swapTestInitialKLVBalance, pool.KLVBalance)

				return
			}

			require.NoError(t, err)
			require.Equal(t, test.wantDebit != 0 || test.wantCredit != 0, subCalled)
			require.Equal(t, test.wantDebit, debited)
			require.Equal(t, test.wantCredit, credited)
			require.Equal(t, test.wantDebit, pool.KDABalance)
			require.Equal(t, swapTestInitialKLVBalance-test.wantCredit, pool.KLVBalance)
		})
	}
}

func createAccountsDB(marshalizer marshal.Marshalizer, accountFactory state.AccountFactory) *state.AccountsDB {
	hasher := &sha256.Sha256{}
	trieStorageManager, _ := trie.NewTrieStorageManagerWithoutPruning(mock.NewMemDbMock())
	tr, _ := trie.NewTrie(trieStorageManager, marshalizer, hasher, 5)
	adb, _ := state.NewAccountsDB(tr, hasher, marshalizer, accountFactory, core.Normal)
	return adb
}

// processingCacher is the real accounts cacher, except that it does not save the swap's stub sender.
type processingCacher struct {
	state.AccountsCacher
}

func (processingCacher) SaveUser(_ state.AccountHandler) error {
	return nil
}

// newPoolKApp wires a fee pool over a real accounts cacher, with klvBalance already saved.
func newPoolKApp(t *testing.T, klvBalance int64) (kapp.KDAFeesPoolKapp, state.AccountsCacher, *state.AccountsDB) {
	t.Helper()

	marshalizer := marshal.NewProtoMarshalizer()
	kappsDB := createAccountsDB(marshalizer, factory.NewKAppAccountCreator())
	accCacher, err := state.NewAccountsCacher(state.ArgsAcccountCacher{
		Accounts: createAccountsDB(marshalizer, factory.NewAccountCreator()),
		Kapps:    kappsDB,
		Peers:    createAccountsDB(marshalizer, factory.NewPeerAccountCreator()),
	})
	require.NoError(t, err)
	accCacher.ResetAll(true)
	cacher := processingCacher{AccountsCacher: accCacher}

	args := createMockArgsKDAFeesPool()
	args.Marshalizer = marshalizer
	feesPool, err := kdafeespool.NewKDAFeesPoolKApp(args)
	require.NoError(t, err)
	require.NoError(t, feesPool.SetAccountsCacher(cacher))

	asset := &kapps.KDAData{}
	require.NoError(t, feesPool.SetKAppController(&mock.KappsControllerMock{
		KDAKapp: &stub.KDAKappStub{
			GetKDACalled: func(_ []byte) (state.KAppAccountHandler, *kapps.KDAData, error) {
				return nil, asset, nil
			},
			GetKDAUncachedCalled: func(_ []byte) (*kapps.KDAData, error) {
				return asset, nil
			},
		},
	}))

	pool, err := marshalizer.Marshal(&kdafeespool.KDAFeesPoolData{
		OwnerAddress: []byte("owner"),
		KDA:          []byte("asset-id"),
		Active:       true,
		KLVBalance:   klvBalance,
		FRatioKLV:    1000,
		FRatioKDA:    100,
	})
	require.NoError(t, err)

	app, err := cacher.LoadKApp(kapps.KDAFeesPoolKAppAddress)
	require.NoError(t, err)
	require.NoError(t, app.SetStorage([]byte("asset-id"), pool))
	require.NoError(t, cacher.SaveAll())
	_, err = kappsDB.Commit()
	require.NoError(t, err)

	return feesPool, cacher, kappsDB
}

func TestValidate_ConcurrentWithSwap(t *testing.T) {
	t.Parallel()

	feesPool, cacher, kappsDB := newPoolKApp(t, swapTestInitialKLVBalance)
	feeHandler := &KDAFeeHandlerStub{kda: []byte("asset-id"), amount: swapTestQuote}

	// block processing swaps the pool while admission reads it; the assertion is a clean -race run
	done := make(chan error, 1)
	go func() {
		for i := 0; i < 200; i++ {
			cacher.ResetAll(true)
			err := feesPool.Swap(&mock.UserAccountHandlerStub{}, swapTestKLVFee, feeHandler)
			if err == nil {
				err = cacher.SaveAll()
			}
			if err == nil && i%20 == 0 {
				_, err = kappsDB.Commit()
			}
			if err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()

	for {
		select {
		case err := <-done:
			require.NoError(t, err)
			return
		default:
			require.NoError(t, feesPool.Validate([]byte("sender"), swapTestKLVFee, feeHandler))
		}
	}
}
