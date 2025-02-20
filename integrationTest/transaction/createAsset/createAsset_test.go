package createasset

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/integrationTest"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	commonTxTest "github.com/klever-io/klever-go/integrationTest/transaction"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/stretchr/testify/require"
)

func TestTransaction_CreateAsset_ShouldWork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	xidProposerBlock := 0

	numWallets := 5
	nodes, wallets, err := commonTxTest.CreateStandardSetupForTxTests(numWallets)
	require.Nil(t, err)

	time.Sleep(300 * time.Millisecond)

	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()

	slot := uint64(0)
	nonce := uint64(0)
	slot = integrationTest.IncrementAndPrintSlot(slot)
	nonce++

	createAssetContract := &models.CreateAssetTXRequest{
		Type:          uint32(*transaction.CreateAssetContract_NonFungible.Enum()),
		Name:          "NFTT",
		Ticker:        "NFTT",
		OwnerAddress:  processorNode.TestAddressPubkeyConverter.Encode(wallets[0].Address),
		InitialSupply: 0,
		MaxSupply:     10000,
		Royalties: &models.RoyaltiesInfo{
			MarketFixed:      1000,
			MarketPercentage: 100,
			TransferFixed:    1000,
		},
		Properties: &models.PropertiesInfo{
			CanMint:        true,
			CanBurn:        true,
			CanPause:       true,
			CanWipe:        true,
			CanChangeOwner: true,
			CanAddRoles:    true,
		},
	}

	_, hash, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_CreateAssetContractType,
		createAssetContract,
	)
	require.Nil(t, err)

	time.Sleep(500 * time.Millisecond)

	_, _, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	// Check if asset exists
	tx, err := nodes[1].GetTransaction(hash, true)
	require.Nil(t, err)

	assetId, err := integrationTest.GetAssetId(tx.Receipts)
	require.NoError(t, err)
	require.NotEmpty(t, assetId)

	asset, err := integrationTest.GetAsset(nodes[1], assetId)
	require.Nil(t, err)
	require.NotNil(t, asset)
}
