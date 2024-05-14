package transfer

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/integrationTest"
	"github.com/klever-io/klever-go/integrationTest/processorNode"
	commonTxTest "github.com/klever-io/klever-go/integrationTest/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransaction_TransactionTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	// Example to Enable Elastic
	// processorNode.WithElastic()
	initialBalance := int64(1_000_000_000_000)
	xidProposerBlock := 0

	numWallets := 5
	nodes, wallets, err := commonTxTest.CreateStandardSetupForTxTests(numWallets)
	require.Nil(t, err)

	time.Sleep(3000 * time.Millisecond)

	defer func() {
		for _, n := range nodes {
			n.Messenger.Close()
		}
	}()

	amount := int64(1000)
	slot := uint64(0)
	nonce := uint64(0)
	slot = integrationTest.IncrementAndPrintSlot(slot)
	nonce++

	tx1, _, err := commonTxTest.CreateAndSendTransaction(
		nodes[0],
		wallets[0],
		transaction.TXContract_TransferContractType,
		struct {
			Receiver string
			Amount   int64
			Asset    string
		}{
			Receiver: processorNode.TestAddressPubkeyConverter.Encode(wallets[1].Address),
			Amount:   amount,
			Asset:    "KLV",
		},
	)
	require.Nil(t, err)

	time.Sleep(1000 * time.Millisecond)

	_, _, nodes, err = integrationTest.ProposeAndSyncOneBlock(t, nodes, xidProposerBlock, slot, nonce)
	require.Nil(t, err)

	//Check accounts balance in second node
	//Sender balance and account nonce
	senderAccount := integrationTest.GetUserAccount(nodes[1], wallets[0].Address)
	require.NotNil(t, senderAccount)
	assert.Equal(t, uint64(1), senderAccount.GetNonce())
	assert.Equal(t, initialBalance-(amount+tx1.GetTotalFees()), senderAccount.GetBalance(kdautils.KLVIdentifier, true), true)

	//Receiver balance
	receiverAccount := integrationTest.GetUserAccount(nodes[1], wallets[1].Address)
	require.NotNil(t, receiverAccount)
	assert.Equal(t, initialBalance+amount, receiverAccount.GetBalance(kdautils.KLVIdentifier, true))

}
