package kda_test

import (
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/stretchr/testify/require"
)

var defaultTicker []byte = []byte("KDA")
var defaultAssetID []byte = []byte("KDA-3W0I")

func createDefaultAsset(
	t *testing.T,
	kc kapp.KAppController,
	assetType transaction.CreateAssetContract_EnumAssetType,
	precision uint32,
	initialSupply int64,
	maxSupply int64,
) {
	tc := &transaction.CreateAssetContract{
		Type:          assetType,
		Name:          defaultTicker,
		Ticker:        defaultTicker,
		OwnerAddress:  defaultSender,
		Precision:     precision,
		InitialSupply: initialSupply,
		MaxSupply:     maxSupply,
		Properties: &transaction.PropertiesInfo{
			CanFreeze:      true,
			CanWipe:        true,
			CanPause:       true,
			CanMint:        true,
			CanBurn:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
			LimitTransfer:  false,
		},
	}

	_, err := kc.GetKDAKApp().Create(defaultSender, tc)
	require.NoError(t, err)
}

func TestKDATrigger_MintInvalidAssetShouldFail(t *testing.T) {
	t.Parallel()

	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0)

	tc := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultTicker,
		ToAddress:   defaultSender,
		Amount:      1000,
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, common.ErrAssetNotFound, err)

	tc = &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     append(defaultAssetID, []byte("/1")...),
		ToAddress:   defaultSender,
		Amount:      1000,
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, common.ErrInvalidValue, err)
}

func TestKDATrigger_MintFTShouldWork(t *testing.T) {
	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0)

	tc := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      100,
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.NoError(t, err)

	// check balance
	acc, err := kc.GetAccountsKApp().GetAccountsCacher().GetExistingUser(defaultSender)
	require.NoError(t, err)
	balance := acc.GetBalanceWithNonce(defaultAssetID, nil, true)
	require.Equal(t, int64(100), balance) // NFT balance always 1

}

func TestKDATrigger_MintFTInvalidAmountShouldFail(t *testing.T) {
	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_Fungible, 0, 0, 0)

	tc := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      0, // Not allowed to mint 0 amount
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, process.ErrInvalidArgument, err)

	tc = &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      -10, // Not allowed negative amount
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, process.ErrInvalidArgument, err)
}

func TestKDATrigger_MintNFTShouldWork(t *testing.T) {
	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_NonFungible, 0, 0, 0)

	tc := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      50,
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.NoError(t, err)

	acc, err := kc.GetAccountsKApp().GetAccountsCacher().GetExistingUser(defaultSender)
	require.NoError(t, err)
	// check balance of internal asset
	for i := 1; i <= 50; i++ {
		balance := acc.GetBalanceWithNonce(defaultAssetID, []byte(fmt.Sprintf("%d", i)), true)
		require.Equal(t, int64(1), balance) // NFT balance always 1
	}
}

func TestKDATrigger_MintNFTInvalidAmountShouldFail(t *testing.T) {
	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_NonFungible, 0, 0, 0)

	tc := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      0, // Not allowed to mint 0 amount
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, process.ErrInvalidArgument, err)

	tc = &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      100, // Not allowed to mint > 50 NFT per contract
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, process.ErrInvalidArgument, err)

	tc = &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      -10, // Not allowed negative amount
	}

	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, process.ErrInvalidArgument, err)
}

func TestKDATrigger_MintSNFTShouldWork(t *testing.T) {
	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_SemiFungible, 0, 0, 0)

	tc := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      0,
	}
	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.NoError(t, err)

	// check balance of internal asset, should be zero
	acc, err := kc.GetAccountsKApp().GetAccountsCacher().GetExistingUser(defaultSender)
	require.NoError(t, err)
	balance := acc.GetBalanceWithNonce(defaultAssetID, []byte("1"), true)
	require.Equal(t, int64(0), balance)

	// create another internal SFT with balance 100
	tc = &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      100,
	}
	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.NoError(t, err)

	// check balance of internal asset, should be zero
	acc, err = kc.GetAccountsKApp().GetAccountsCacher().GetExistingUser(defaultSender)
	require.NoError(t, err)
	balance = acc.GetBalanceWithNonce(defaultAssetID, []byte("2"), true)
	require.Equal(t, int64(100), balance)
}

func TestKDATrigger_MintSNFTInvalidAmountShouldFail(t *testing.T) {
	kc, err := createMockControllers()
	require.NoError(t, err)

	createDefaultAsset(t, kc, transaction.CreateAssetContract_SemiFungible, 0, 0, 0)

	tc := &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_Mint,
		AssetID:     defaultAssetID,
		ToAddress:   defaultSender,
		Amount:      -1,
	}
	_, err = kc.GetKDAKApp().Mint(defaultSender, tc)
	require.Equal(t, process.ErrInvalidArgument, err)
}
