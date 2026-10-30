package kda_test

import (
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/kda"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// burnContract builds the AssetTriggerContract Burn consumes.
func burnContract(
	triggerType transaction.AssetTriggerContract_EnumTriggerType,
	assetID []byte,
	to []byte,
	amount int64,
) *transaction.AssetTriggerContract {
	return &transaction.AssetTriggerContract{
		TriggerType: triggerType,
		AssetID:     assetID,
		ToAddress:   to,
		Amount:      amount,
	}
}

// withNonce appends an NFT/SFT nonce to an asset id, the form Burn splits apart.
func withNonce(assetID []byte, nonce string) []byte {
	return append(append([]byte{}, assetID...), []byte(kapps.Sp+nonce)...)
}

func TestKDAKapp_BurnRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	t.Run("an unknown asset is not found", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)

		code, err := kc.GetKDAKApp().Burn(defaultSender,
			burnContract(transaction.AssetTriggerContract_Burn, []byte("NOPE-0000"), defaultSender, 1))

		assert.Equal(t, transaction.Transaction_KAPPError, code)
		require.ErrorIs(t, err, common.ErrAssetNotFound)
	})

	t.Run("wipe from a non-owner is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 1000, 0, nil)

		code, err := kc.GetKDAKApp().Burn(defaultOtherSender,
			burnContract(transaction.AssetTriggerContract_Wipe, defaultAssetID, defaultSender, 1))

		assert.Equal(t, transaction.Transaction_AccountNotOwner, code)
		require.ErrorIs(t, err, common.ErrAccNotOwner)
	})

	t.Run("wipe on an asset that cannot be wiped is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 1000, 0,
			&transaction.PropertiesInfo{CanMint: true, CanBurn: true})

		code, err := kc.GetKDAKApp().Burn(defaultSender,
			burnContract(transaction.AssetTriggerContract_Wipe, defaultAssetID, defaultSender, 1))

		assert.Equal(t, transaction.Transaction_AssetCantBeWiped, code)
		require.ErrorIs(t, err, common.ErrAssetTriggerInvalid)
	})

	t.Run("an asset that cannot be burned is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 1000, 0,
			&transaction.PropertiesInfo{CanMint: true, CanWipe: true})

		code, err := kc.GetKDAKApp().Burn(defaultSender,
			burnContract(transaction.AssetTriggerContract_Burn, defaultAssetID, defaultSender, 1))

		assert.Equal(t, transaction.Transaction_AssetCantBeBurned, code)
		require.ErrorIs(t, err, common.ErrAssetTriggerInvalid)
	})

	t.Run("a receiver address of the wrong length is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 1000, 0, nil)

		code, err := kc.GetKDAKApp().Burn(defaultSender,
			burnContract(transaction.AssetTriggerContract_Burn, defaultAssetID, []byte("short"), 1))

		assert.Equal(t, transaction.Transaction_AccountError, code)
		require.ErrorIs(t, err, process.ErrInvalidRcvAddr)
	})

	t.Run("a fungible burn carrying a nonce is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 1000, 0, nil)

		// a fungible asset has no nonce, so appending one is rejected while the
		// asset id is still being parsed
		code, err := kc.GetKDAKApp().Burn(defaultSender,
			burnContract(transaction.AssetTriggerContract_Burn, withNonce(defaultAssetID, "1"), defaultSender, 1))

		assert.Equal(t, transaction.Transaction_ParameterInvalid, code)
		require.ErrorIs(t, err, common.ErrInvalidValue)
	})

	t.Run("a non-positive fungible amount is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 1000, 0, nil)

		code, err := kc.GetKDAKApp().Burn(defaultSender,
			burnContract(transaction.AssetTriggerContract_Burn, defaultAssetID, defaultSender, 0))

		assert.Equal(t, transaction.Transaction_ParameterInvalid, code)
		require.ErrorIs(t, err, process.ErrInvalidArgument)
	})

	t.Run("burning more than the circulating supply is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 1000, 0, nil)

		code, err := kc.GetKDAKApp().Burn(defaultSender,
			burnContract(transaction.AssetTriggerContract_Burn, defaultAssetID, defaultSender, 2000))

		assert.Equal(t, transaction.Transaction_AssetError, code)
		require.ErrorIs(t, err, process.ErrSupplyNotValid)
	})

	t.Run("a non-fungible burn without a nonce is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		createDefaultAsset(t, kc, transaction.CreateAssetContract_NonFungible, 0, 0, 0, nil)

		code, err := kc.GetKDAKApp().Burn(defaultSender,
			burnContract(transaction.AssetTriggerContract_Burn, defaultAssetID, defaultSender, 1))

		assert.Equal(t, transaction.Transaction_AssetTypeInvalid, code)
		require.ErrorIs(t, err, process.ErrInvalidArgument)
	})

	t.Run("a semi-fungible burn without a nonce is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		createDefaultAsset(t, kc, transaction.CreateAssetContract_SemiFungible, 0, 0, 0, nil)

		code, err := kc.GetKDAKApp().Burn(defaultSender,
			burnContract(transaction.AssetTriggerContract_Burn, defaultAssetID, defaultSender, 1))

		assert.Equal(t, transaction.Transaction_AssetTypeInvalid, code)
		require.ErrorIs(t, err, process.ErrInvalidArgument)
	})
}

func TestKDAKapp_BurnFungibleReducesSupply(t *testing.T) {
	t.Parallel()

	kc, err := createMockControllers()
	require.NoError(t, err)
	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 1000, 0, nil)

	code, err := kc.GetKDAKApp().Burn(defaultSender,
		burnContract(transaction.AssetTriggerContract_Burn, defaultAssetID, defaultSender, 400))

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, code)

	_, kda, err := kc.GetKDAKApp().GetKDA(defaultAssetID)
	require.NoError(t, err)
	assert.Equal(t, int64(400), kda.BurnedValue)
	assert.Equal(t, int64(600), kda.CirculatingSupply)

	// the holder paid for it out of their own balance
	acc, err := kc.GetKDAKApp().GetAccountsCacher().GetExistingUser(defaultSender)
	require.NoError(t, err)
	assert.Equal(t, int64(600), acc.GetBalance(defaultAssetID, true))
}

// A minted NFT's compound ID names no asset: the asset records live in the KDA KApp under the
// collection ID alone, so a market currency named like an NFT is refused wherever it is loaded.
func TestKDAKapp_GetKDADoesNotResolveCompoundNFTID(t *testing.T) {
	t.Parallel()

	kc, err := createMockControllers()
	require.NoError(t, err)
	createDefaultAsset(t, kc, transaction.CreateAssetContract_NonFungible, 0, 0, 0, nil)
	_, err = kc.GetKDAKApp().Mint(defaultSender, &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      1,
	})
	require.NoError(t, err)

	_, _, err = kc.GetKDAKApp().GetKDA(withNonce(defaultAssetID, "1"))

	require.ErrorIs(t, err, common.ErrAssetNotFound)
}

// A spent NFT record (balance 0, kept in the trie by its metadata) holds no unit. From
// FixAuditChangesV7 burning or wiping it is refused and the supply stays; before it, the burn
// lowers the circulating supply with no unit destroyed (KLC-2835).
func TestKDAKapp_BurnSpentNFTRecord(t *testing.T) {
	t.Parallel()

	burnSpent := func(t *testing.T, triggerType transaction.AssetTriggerContract_EnumTriggerType, fixAuditChangesV7 bool) (*kapps.KDAData, transaction.Transaction_TXResultCode, error) {
		t.Helper()

		kc, err := createMockControllersWithFork(mock.NewForkControllerStub().SetFork("FixAuditChangesV7", fixAuditChangesV7))
		require.NoError(t, err)
		createDefaultAsset(t, kc, transaction.CreateAssetContract_NonFungible, 0, 0, 0, nil)
		_, err = kc.GetKDAKApp().Mint(defaultSender, &transaction.AssetTriggerContract{
			TriggerType: transaction.AssetTriggerContract_Mint,
			AssetID:     defaultAssetID,
			ToAddress:   defaultSender,
			Amount:      1,
		})
		require.NoError(t, err)

		acc, err := kc.GetKDAKApp().GetAccountsCacher().GetExistingUser(defaultSender)
		require.NoError(t, err)
		require.NoError(t, acc.DataTrieTracker().SaveKeyValue(kdautils.ToKDAKey(defaultAssetID, []byte("1")), mock.NFTHolderRecord(0)))

		code, err := kc.GetKDAKApp().Burn(defaultSender,
			burnContract(triggerType, withNonce(defaultAssetID, "1"), defaultSender, 1))

		_, kda, errKDA := kc.GetKDAKApp().GetKDA(defaultAssetID)
		require.NoError(t, errKDA)
		return kda, code, err
	}

	for _, triggerType := range []transaction.AssetTriggerContract_EnumTriggerType{
		transaction.AssetTriggerContract_Burn,
		transaction.AssetTriggerContract_Wipe,
	} {
		t.Run(triggerType.String()+" refused from V7", func(t *testing.T) {
			kda, code, err := burnSpent(t, triggerType, true)

			assert.Equal(t, transaction.Transaction_AssetError, code)
			require.ErrorIs(t, err, process.ErrInvalidArgument)
			assert.Equal(t, int64(1), kda.CirculatingSupply)
		})
	}

	t.Run("lowers the supply before V7", func(t *testing.T) {
		kda, code, err := burnSpent(t, transaction.AssetTriggerContract_Burn, false)

		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)
		assert.Equal(t, int64(0), kda.CirculatingSupply)
	})
}

func TestKDAKapp_BurnNonFungible(t *testing.T) {
	t.Parallel()

	newMintedNFT := func(t *testing.T) (kapp.KAppController, []byte) {
		t.Helper()

		kc, err := createMockControllers()
		require.NoError(t, err)
		createDefaultAsset(t, kc, transaction.CreateAssetContract_NonFungible, 0, 0, 0, nil)

		_, err = kc.GetKDAKApp().Mint(defaultSender, &transaction.AssetTriggerContract{
			TriggerType: transaction.AssetTriggerContract_Mint,
			AssetID:     defaultAssetID,
			ToAddress:   defaultSender,
			Amount:      1,
		})
		require.NoError(t, err)

		return kc, withNonce(defaultAssetID, "1")
	}

	t.Run("burning a minted NFT clears it from the holder", func(t *testing.T) {
		kc, nftID := newMintedNFT(t)

		code, err := kc.GetKDAKApp().Burn(defaultSender,
			burnContract(transaction.AssetTriggerContract_Burn, nftID, defaultSender, 1))

		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)

		_, kda, err := kc.GetKDAKApp().GetKDA(defaultAssetID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), kda.BurnedValue)
		assert.Equal(t, int64(0), kda.CirculatingSupply)
	})

	t.Run("burning an NFT the holder does not have is refused", func(t *testing.T) {
		kc, _ := newMintedNFT(t)

		code, err := kc.GetKDAKApp().Burn(defaultSender,
			burnContract(transaction.AssetTriggerContract_Burn, withNonce(defaultAssetID, "9"), defaultSender, 1))

		assert.Equal(t, transaction.Transaction_AssetError, code)
		require.ErrorIs(t, err, process.ErrInvalidArgument)
	})

	t.Run("burning an NFT with an amount other than one is refused", func(t *testing.T) {
		kc, nftID := newMintedNFT(t)

		code, err := kc.GetKDAKApp().Burn(defaultSender,
			burnContract(transaction.AssetTriggerContract_Burn, nftID, defaultSender, 2))

		assert.Equal(t, transaction.Transaction_AssetError, code)
		require.ErrorIs(t, err, process.ErrInvalidArgument)
	})
}

func TestKDAKapp_GetKDAUncached(t *testing.T) {
	t.Parallel()

	kc, err := createMockControllers()
	require.NoError(t, err)
	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 1000, 0, nil)

	t.Run("an unknown asset is not found", func(t *testing.T) {
		_, errGet := kc.GetKDAKApp().GetKDAUncached([]byte("NOPE-0000"))

		require.ErrorIs(t, errGet, common.ErrAssetNotFound)
	})

	t.Run("an asset created in the current block is not visible yet", func(t *testing.T) {
		// GetKDAUncached deliberately reads the last committed state, so an
		// asset created inside the block being processed is not there yet
		_, errGet := kc.GetKDAKApp().GetKDAUncached(defaultAssetID)

		require.ErrorIs(t, errGet, common.ErrAssetNotFound)
	})
}

// createStakingAsset creates a fungible asset whose staking is configured with the
// given interest type, which is what Deposit checks before accepting FPR funds.
func createStakingAsset(t *testing.T, kc kapp.KAppController, interestType transaction.StakingInfo_InterestType) {
	t.Helper()

	tc := &transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          defaultTicker,
		Ticker:        defaultTicker,
		OwnerAddress:  defaultSender,
		InitialSupply: 1000,
		Properties: &transaction.PropertiesInfo{
			CanFreeze: true, CanWipe: true, CanPause: true, CanMint: true,
			CanBurn: true, CanChangeOwner: true, CanAddRoles: true,
		},
		Staking: &transaction.StakingInfo{
			Type:                interestType,
			APR:                 1000,
			MinEpochsToClaim:    1,
			MinEpochsToUnstake:  1,
			MinEpochsToWithdraw: 1,
		},
	}

	_, err := kc.GetKDAKApp().Create(defaultSender, tc)
	require.NoError(t, err)
}

func TestKDAKapp_DepositRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	t.Run("a missing asset id is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)

		code, err := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{Amount: 1})

		assert.Equal(t, transaction.Transaction_AssetError, code)
		require.ErrorIs(t, err, common.ErrInvalidAsset)
	})

	t.Run("a non-positive amount is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)

		code, err := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
			ID:     defaultAssetID,
			Amount: 0,
		})

		assert.Equal(t, transaction.Transaction_AmountInvalid, code)
		require.ErrorIs(t, err, common.ErrInvalidValue)
	})

	t.Run("an unknown asset is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)

		code, err := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
			ID:     []byte("NOPE-0000"),
			Amount: 10,
		})

		assert.Equal(t, transaction.Transaction_KAPPError, code)
		require.ErrorIs(t, err, common.ErrAssetNotFound)
	})

	t.Run("a sender with no role on the asset is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		createStakingAsset(t, kc, transaction.StakingInfo_FPRI)

		code, err := kc.GetKDAKApp().Deposit(defaultOtherSender, &transaction.DepositContract{
			ID:     defaultAssetID,
			Amount: 10,
		})

		assert.Equal(t, transaction.Transaction_AssetError, code)
		require.ErrorIs(t, err, common.ErrRoleNotFound)
	})

	t.Run("an unknown currency is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		createStakingAsset(t, kc, transaction.StakingInfo_FPRI)

		code, err := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
			ID:         defaultAssetID,
			CurrencyID: []byte("NOPE-0000"),
			Amount:     10,
		})

		assert.Equal(t, transaction.Transaction_KAPPError, code)
		require.ErrorIs(t, err, common.ErrAssetNotFound)
	})

	t.Run("an asset without staking is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		// staking records are only written for assets that can freeze
		createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 1000, 0,
			&transaction.PropertiesInfo{CanMint: true, CanBurn: true})

		code, err := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
			ID:         defaultAssetID,
			CurrencyID: defaultAssetID,
			Amount:     10,
		})

		assert.Equal(t, transaction.Transaction_AssetError, code)
		require.Error(t, err)
	})

	t.Run("an APR-staked asset cannot take FPR deposits", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		createStakingAsset(t, kc, transaction.StakingInfo_APRI)

		code, err := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
			ID:         defaultAssetID,
			CurrencyID: defaultAssetID,
			Amount:     10,
		})

		assert.Equal(t, transaction.Transaction_AssetTypeInvalid, code)
		require.ErrorIs(t, err, common.ErrAssetTypeInvalid)
	})

	t.Run("depositing more than the balance is refused", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		createStakingAsset(t, kc, transaction.StakingInfo_FPRI)

		code, err := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
			ID:         defaultAssetID,
			CurrencyID: defaultAssetID,
			Amount:     10_000,
		})

		assert.Equal(t, transaction.Transaction_BalanceError, code)
		require.Error(t, err)
	})
}

// nextEpochFPR returns the FPR bucket a deposit lands in (the epoch after the
// current block), which sits alongside the epoch-0 bucket Create seeds.
func nextEpochFPR(t *testing.T, staking *kapps.StakingData) *kapps.FPRData {
	t.Helper()

	for _, fpr := range staking.FPR {
		if fpr.GetEpoch() == 1 {
			return fpr
		}
	}

	t.Fatalf("no FPR entry for epoch 1 in %v", staking.FPR)
	return nil
}

func TestKDAKapp_DepositAccumulatesFPR(t *testing.T) {
	t.Parallel()

	t.Run("a deposit lands on the next epoch's FPR", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		createStakingAsset(t, kc, transaction.StakingInfo_FPRI)

		code, err := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
			ID:         defaultAssetID,
			CurrencyID: defaultAssetID,
			Amount:     500,
		})

		require.NoError(t, err)
		require.Equal(t, transaction.Transaction_Ok, code)

		_, staking, err := kc.GetKDAKApp().GetStaking(defaultAssetID)
		require.NoError(t, err)
		fpr := nextEpochFPR(t, staking)
		require.Contains(t, fpr.KDAS, string(defaultAssetID))
		assert.Equal(t, int64(500), fpr.KDAS[string(defaultAssetID)].TotalAmount)

		// the depositor paid for it out of their own balance
		acc, err := kc.GetKDAKApp().GetAccountsCacher().GetExistingUser(defaultSender)
		require.NoError(t, err)
		assert.Equal(t, int64(500), acc.GetBalance(defaultAssetID, true))
	})

	t.Run("two deposits accumulate on the same currency bucket", func(t *testing.T) {
		kc, err := createMockControllers()
		require.NoError(t, err)
		createStakingAsset(t, kc, transaction.StakingInfo_FPRI)

		for range 2 {
			code, errDeposit := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
				ID:         defaultAssetID,
				CurrencyID: defaultAssetID,
				Amount:     100,
			})
			require.NoError(t, errDeposit)
			require.Equal(t, transaction.Transaction_Ok, code)
		}

		_, staking, err := kc.GetKDAKApp().GetStaking(defaultAssetID)
		require.NoError(t, err)
		fpr := nextEpochFPR(t, staking)
		assert.Equal(t, int64(200), fpr.KDAS[string(defaultAssetID)].TotalAmount)
	})
}

func TestNewKDAKApp(t *testing.T) {
	t.Parallel()

	validArgs := func() *kda.ArgsNewKDAKApp {
		return &kda.ArgsNewKDAKApp{
			Hasher:         &sha256.Sha256{},
			Marshalizer:    marshal.NewProtoMarshalizer(),
			PubkeyConv:     cryptoMock.NewPubkeyConverterMock(32),
			ForkController: mock.NewForkControllerStub(),
		}
	}

	t.Run("a nil hasher is rejected", func(t *testing.T) {
		args := validArgs()
		args.Hasher = nil

		app, err := kda.NewKDAKApp(args)

		assert.Nil(t, app)
		require.ErrorIs(t, err, common.ErrNilHasher)
	})

	t.Run("a nil marshalizer is rejected", func(t *testing.T) {
		args := validArgs()
		args.Marshalizer = nil

		app, err := kda.NewKDAKApp(args)

		assert.Nil(t, app)
		require.ErrorIs(t, err, common.ErrNilMarshalizer)
	})

	t.Run("a nil pubkey converter is rejected", func(t *testing.T) {
		args := validArgs()
		args.PubkeyConv = nil

		app, err := kda.NewKDAKApp(args)

		assert.Nil(t, app)
		require.ErrorIs(t, err, common.ErrNilPubkeyConverter)
	})

	t.Run("a complete argument set builds the KApp", func(t *testing.T) {
		app, err := kda.NewKDAKApp(validArgs())

		require.NoError(t, err)
		require.NotNil(t, app)
		assert.False(t, app.IsInterfaceNil())
		assert.Nil(t, app.GetAccountsCacher())
	})

	t.Run("a nil accounts cacher is rejected", func(t *testing.T) {
		app, err := kda.NewKDAKApp(validArgs())
		require.NoError(t, err)

		require.ErrorIs(t, app.SetAccountsCacher(nil), common.ErrNilAccountsAdapter)
	})
}

// enableSmartContracts flips the fork the KDA KApp shares with its controller,
// which is what gates SFT support and the relaxed circulating-supply rule.
func enableSmartContracts(t *testing.T, kc kapp.KAppController) {
	t.Helper()

	forkController, ok := kc.GetForkController().(*mock.ForkControllerStub)
	require.True(t, ok)
	forkController.EnableSmartContractsValue = true
}

func TestKDAKapp_BurnSemiFungible(t *testing.T) {
	t.Parallel()

	kc, err := createMockControllers()
	require.NoError(t, err)
	enableSmartContracts(t, kc)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_SemiFungible, 0, 0, 0, nil)

	_, err = kc.GetKDAKApp().Mint(defaultSender, &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      10,
	})
	require.NoError(t, err)

	sftID := withNonce(defaultAssetID, "1")

	t.Run("burning part of the holding reduces the circulation", func(t *testing.T) {
		code, errBurn := kc.GetKDAKApp().Burn(defaultSender,
			burnContract(transaction.AssetTriggerContract_Burn, sftID, defaultSender, 4))

		require.NoError(t, errBurn)
		require.Equal(t, transaction.Transaction_Ok, code)

		acc, errAcc := kc.GetKDAKApp().GetAccountsCacher().GetExistingUser(defaultSender)
		require.NoError(t, errAcc)
		assert.Equal(t, int64(6), acc.GetBalance(sftID, true))
	})

	t.Run("burning more than the holding is refused", func(t *testing.T) {
		code, errBurn := kc.GetKDAKApp().Burn(defaultSender,
			burnContract(transaction.AssetTriggerContract_Burn, sftID, defaultSender, 100))

		assert.Equal(t, transaction.Transaction_BalanceError, code)
		require.Error(t, errBurn)
	})
}

func TestKDAKapp_BurnFungibleDownToZeroPostFork(t *testing.T) {
	t.Parallel()

	kc, err := createMockControllers()
	require.NoError(t, err)
	// after the smart-contracts fork a supply of exactly zero is valid, where
	// the older rule required it to stay strictly positive
	enableSmartContracts(t, kc)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 1000, 0, nil)

	code, err := kc.GetKDAKApp().Burn(defaultSender,
		burnContract(transaction.AssetTriggerContract_Burn, defaultAssetID, defaultSender, 1000))

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, code)

	_, kdaData, err := kc.GetKDAKApp().GetKDA(defaultAssetID)
	require.NoError(t, err)
	assert.Zero(t, kdaData.CirculatingSupply)
	assert.Equal(t, int64(1000), kdaData.BurnedValue)
}

// registerKLV writes a KLV entry into the KDA trie; the in-memory harness starts
// without one, and a KLV-denominated deposit needs the asset to resolve.
func registerKLV(t *testing.T, kc kapp.KAppController) {
	t.Helper()

	kdaKapp, _, err := kc.GetKDAKApp().GetKDA(nil)
	require.NoError(t, err)

	err = kc.GetKDAKApp().SetKDA(kdaKapp, kdautils.KLVIdentifier, &kapps.KDAData{
		AssetType: kapps.KDAData_Fungible,
		ID:        kdautils.KLVIdentifier,
		Name:      kdautils.KLVIdentifier,
		Ticker:    kdautils.KLVIdentifier,
		// KLV carries no burn/wipe rights; Burn dereferences Properties directly
		Properties: &kapps.PropertiesData{},
	})
	require.NoError(t, err)
	require.NoError(t, kc.GetKDAKApp().GetAccountsCacher().UpdateKapp(kdaKapp))
}

// fundSenderWithKLV credits the sender's KLV balance through the cacher the
// KApps read from; the harness seeds its funds before the cacher is reset.
func fundSenderWithKLV(t *testing.T, kc kapp.KAppController, amount int64) {
	t.Helper()

	cacher := kc.GetKDAKApp().GetAccountsCacher()
	acc, err := cacher.LoadUser(defaultSender)
	require.NoError(t, err)
	require.NoError(t, acc.AddToBalance(amount, kdautils.KLVIdentifier, true))
	require.NoError(t, cacher.UpdateUser(acc))
}

func TestKDAKapp_DepositInKLV(t *testing.T) {
	t.Parallel()

	kc, err := createMockControllers()
	require.NoError(t, err)
	registerKLV(t, kc)
	fundSenderWithKLV(t, kc, 5_000)
	createStakingAsset(t, kc, transaction.StakingInfo_FPRI)

	t.Run("a KLV deposit accumulates on the FPR total rather than a currency bucket", func(t *testing.T) {
		code, errDeposit := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
			ID:     defaultAssetID,
			Amount: 700,
		})

		require.NoError(t, errDeposit)
		require.Equal(t, transaction.Transaction_Ok, code)

		_, staking, errStaking := kc.GetKDAKApp().GetStaking(defaultAssetID)
		require.NoError(t, errStaking)
		fpr := nextEpochFPR(t, staking)
		assert.Equal(t, int64(700), fpr.TotalAmount)
		assert.Empty(t, fpr.KDAS)
	})

	t.Run("a second KLV deposit adds onto the same total", func(t *testing.T) {
		code, errDeposit := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
			ID:     defaultAssetID,
			Amount: 300,
		})

		require.NoError(t, errDeposit)
		require.Equal(t, transaction.Transaction_Ok, code)

		_, staking, errStaking := kc.GetKDAKApp().GetStaking(defaultAssetID)
		require.NoError(t, errStaking)
		assert.Equal(t, int64(1000), nextEpochFPR(t, staking).TotalAmount)
	})
}

// setBlockEpoch swaps the KApp context for one sitting on the given epoch, which
// is what decides whether an FPR bucket has expired.
func setBlockEpoch(kc kapp.KAppController, epoch uint32) {
	kc.SetCurrentKAppContext(kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: defaultSender,
		ContractID:     0,
		Block:          &block.Block{Header: &block.BlockHeader{Epoch: epoch}},
	}))
}

// overwriteFPR replaces the asset's FPR buckets with the given ones.
func overwriteFPR(t *testing.T, kc kapp.KAppController, fprs []*kapps.FPRData) {
	t.Helper()

	stakingKapp, staking, err := kc.GetKDAKApp().GetStaking(defaultAssetID)
	require.NoError(t, err)

	staking.FPR = fprs
	require.NoError(t, kc.GetKDAKApp().SetStaking(stakingKapp, defaultAssetID, staking))
	require.NoError(t, kc.GetKDAKApp().GetAccountsCacher().UpdateKapp(stakingKapp))
}

func TestKDAKapp_DepositPaysOutExpiredFPR(t *testing.T) {
	t.Parallel()

	kc, err := createMockControllers()
	require.NoError(t, err)
	createStakingAsset(t, kc, transaction.StakingInfo_FPRI)

	// an old bucket with unclaimed KLV and unclaimed KDA, both owed back to the owner
	overwriteFPR(t, kc, []*kapps.FPRData{{
		Epoch:        0,
		TotalAmount:  100,
		TotalClaimed: 40,
		KDAS: map[string]*kapps.KDAFPRData{
			string(defaultAssetID): {TotalAmount: 30, TotalClaimed: 10},
		},
	}})

	cacher := kc.GetKDAKApp().GetAccountsCacher()
	before, err := cacher.GetExistingUser(defaultSender)
	require.NoError(t, err)
	klvBefore := before.GetBalance(nil, true)
	assetBefore := before.GetBalance(defaultAssetID, true)

	// MaxEpochsUnclaimed defaults to 100, so epoch 200 leaves the bucket expired
	setBlockEpoch(kc, 200)

	code, err := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
		ID:         defaultAssetID,
		CurrencyID: defaultAssetID,
		Amount:     10,
	})

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, code)

	after, err := cacher.GetExistingUser(defaultSender)
	require.NoError(t, err)
	// the 60 unclaimed KLV came back
	assert.Equal(t, klvBefore+60, after.GetBalance(nil, true))
	// the 20 unclaimed KDA came back, minus the 10 just deposited
	assert.Equal(t, assetBefore+20-10, after.GetBalance(defaultAssetID, true))

	_, staking, err := kc.GetKDAKApp().GetStaking(defaultAssetID)
	require.NoError(t, err)
	// the expired bucket is gone, replaced by the one for the next epoch
	require.Len(t, staking.FPR, 1)
	assert.Equal(t, uint32(201), staking.FPR[0].GetEpoch())
}

func TestKDAKapp_DepositRejectsTooManyKDAsInOneEpoch(t *testing.T) {
	t.Parallel()

	kc, err := createMockControllers()
	require.NoError(t, err)
	createStakingAsset(t, kc, transaction.StakingInfo_FPRI)

	// fill the next epoch's bucket up to the per-epoch KDA limit
	saturated := make(map[string]*kapps.KDAFPRData, core.MaxDepositKDAs)
	for i := range core.MaxDepositKDAs {
		saturated[fmt.Sprintf("ASSET-%d", i)] = &kapps.KDAFPRData{TotalAmount: 1}
	}
	overwriteFPR(t, kc, []*kapps.FPRData{{Epoch: 1, KDAS: saturated}})

	code, err := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
		ID:         defaultAssetID,
		CurrencyID: defaultAssetID,
		Amount:     10,
	})

	assert.Equal(t, transaction.Transaction_MaxSupplyExceeded, code)
	require.ErrorIs(t, err, common.ErrMaxSupplyExceeded)
}

func TestKDAKapp_DepositRejectsARoleWithoutTheDepositRight(t *testing.T) {
	t.Parallel()

	kc, err := createMockControllers()
	require.NoError(t, err)

	// defaultOtherSender is on the asset, but only to mint
	tc := &transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          defaultTicker,
		Ticker:        defaultTicker,
		OwnerAddress:  defaultSender,
		InitialSupply: 1000,
		Properties: &transaction.PropertiesInfo{
			CanFreeze: true, CanMint: true, CanBurn: true, CanAddRoles: true,
		},
		Staking: &transaction.StakingInfo{Type: transaction.StakingInfo_FPRI},
		Roles: []*transaction.RolesInfo{
			{Address: defaultOtherSender, HasRoleMint: true},
		},
	}
	_, err = kc.GetKDAKApp().Create(defaultSender, tc)
	require.NoError(t, err)

	code, err := kc.GetKDAKApp().Deposit(defaultOtherSender, &transaction.DepositContract{
		ID:         defaultAssetID,
		CurrencyID: defaultAssetID,
		Amount:     10,
	})

	assert.Equal(t, transaction.Transaction_AccountError, code)
	require.ErrorIs(t, err, common.ErrInvalidValue)
}

func TestKDAKapp_GetKDAUncachedSeesCommittedAssets(t *testing.T) {
	t.Parallel()

	kc, err := createMockControllers()
	require.NoError(t, err)
	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 1000, 0, nil)

	// once the block's accounts are written out, the uncached read finds the asset
	require.NoError(t, kc.GetKDAKApp().GetAccountsCacher().SaveAll())

	kdaData, err := kc.GetKDAKApp().GetKDAUncached(defaultAssetID)

	require.NoError(t, err)
	require.NotNil(t, kdaData)
	assert.Equal(t, string(defaultAssetID), string(kdaData.ID))
	assert.Equal(t, int64(1000), kdaData.InitialSupply)
}

func TestKDAKapp_BurnWithoutAnAssetIDFallsBackToKLV(t *testing.T) {
	t.Parallel()

	kc, err := createMockControllers()
	require.NoError(t, err)
	registerKLV(t, kc)

	// an empty asset id means KLV, and the KLV record carries no burn right
	code, err := kc.GetKDAKApp().Burn(defaultSender,
		burnContract(transaction.AssetTriggerContract_Burn, nil, defaultSender, 1))

	assert.Equal(t, transaction.Transaction_AssetCantBeBurned, code)
	require.ErrorIs(t, err, common.ErrAssetTriggerInvalid)
}

func TestKDAKapp_BurnFungibleDownToZeroPreFork(t *testing.T) {
	t.Parallel()

	kc, err := createMockControllers()
	require.NoError(t, err)

	forkController, ok := kc.GetForkController().(*mock.ForkControllerStub)
	require.True(t, ok)
	forkController.EnableSmartContractsValue = false

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 1000, 0, nil)

	// before the smart-contracts fork the supply had to stay strictly positive,
	// so burning the whole supply is rejected
	code, err := kc.GetKDAKApp().Burn(defaultSender,
		burnContract(transaction.AssetTriggerContract_Burn, defaultAssetID, defaultSender, 1000))

	assert.Equal(t, transaction.Transaction_AssetError, code)
	require.ErrorIs(t, err, process.ErrSupplyNotValid)
}

func TestKDAKapp_DepositAddsANewCurrencyToAnExistingBucket(t *testing.T) {
	t.Parallel()

	kc, err := createMockControllers()
	require.NoError(t, err)
	createStakingAsset(t, kc, transaction.StakingInfo_FPRI)

	// the next epoch's bucket already tracks another currency
	overwriteFPR(t, kc, []*kapps.FPRData{{
		Epoch: 1,
		KDAS:  map[string]*kapps.KDAFPRData{"OTHER-0000": {TotalAmount: 5}},
	}})

	code, err := kc.GetKDAKApp().Deposit(defaultSender, &transaction.DepositContract{
		ID:         defaultAssetID,
		CurrencyID: defaultAssetID,
		Amount:     40,
	})

	require.NoError(t, err)
	require.Equal(t, transaction.Transaction_Ok, code)

	_, staking, err := kc.GetKDAKApp().GetStaking(defaultAssetID)
	require.NoError(t, err)
	fpr := nextEpochFPR(t, staking)
	// the existing currency is untouched and the new one is added alongside it
	assert.Equal(t, int64(5), fpr.KDAS["OTHER-0000"].TotalAmount)
	assert.Equal(t, int64(40), fpr.KDAS[string(defaultAssetID)].TotalAmount)
}
