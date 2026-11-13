package accounts

import (
	"encoding/hex"
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

// The legacy path and the batch share planSplitRoyalty and payRoyalty, so comparing them cannot
// show a receipt field changing on both. This pins every receipt of an SFT sent with a percentage
// royalty and a split, field by field: the split receipt carries no nonce, the royalty receiver's
// and the transfer's carry the SFT nonce, and all three carry the SemiFungible asset type. It holds
// before FixAuditChangesV7, which is what blocks already on chain replay, and after it.
func TestTransfer_SemiFungibleRoyaltyReceiptFields(t *testing.T) {
	kda := func() *kapps.KDAData {
		return parityKDA(kapps.KDAData_SemiFungible, &kapps.RoyaltiesData{
			Address:            parityRoyalty,
			TransferPercentage: []*kapps.RoyaltyData{{Amount: math.MaxInt64, Percentage: core.HundredPercent / 10}},
			SplitRoyalties: map[string]*kapps.RoyaltySplitData{
				hex.EncodeToString(paritySplit): {PercentTransferPercentage: core.HundredPercent / 2},
			},
		})()
	}
	fund := func(t *testing.T, acc state.UserAccountHandler) {
		require.NoError(t, acc.AddToBalanceWithNonce(1_000, paritySemiFungible, parityNonce, true))
		require.NoError(t, acc.AddToBalance(1_000, paritySemiFungible, true))
	}
	sftType := []byte{byte(kapps.KDAData_SemiFungible)}
	receipt := func(to []byte, amount int64, internalID []byte) *transaction.Transaction_Receipt {
		return txProcess.NewReceipt(txProcess.Transfer, transferContractID, paritySender, to,
			[]byte(strconv.FormatInt(amount, 10)), paritySemiFungible, internalID, sftType)
	}
	// A 10% royalty on 400 is 40: 20 to the split, 20 to the royalty receiver.
	expected := []*transaction.Transaction_Receipt{
		receipt(paritySplit, 20, nil),
		receipt(parityRoyalty, 20, parityNonce),
		receipt(parityReceiver, 400, parityNonce),
	}

	for _, fixV7 := range []bool{false, true} {
		cfg := config.EnableEpochs{}
		if !fixV7 {
			cfg.FixAuditChangesV7 = 1000
		}
		w := newParityWorld(t, cfg, kda(), fund)
		require.Equal(t, fixV7, w.accKapp.forkController.FixAuditChangesV7())

		code, err := w.accKapp.Transfer(transaction.TXContract_TransferContractType, paritySender, parityTransfer("SEMI-1234/7", 400, 40, 0))
		require.NoError(t, err, "FixAuditChangesV7=%v", fixV7)
		require.Equal(t, transaction.Transaction_Ok, code, "FixAuditChangesV7=%v", fixV7)
		require.Equal(t, expected, w.ctx.Receipts().Get(), "FixAuditChangesV7=%v", fixV7)
	}
}
