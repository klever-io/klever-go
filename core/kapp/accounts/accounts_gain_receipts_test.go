package accounts

import (
	"strconv"
	"testing"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/transaction"
	integrationMock "github.com/klever-io/klever-go/integrationTest/mock"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/require"
)

// TestAddStakingGainReceipts pins the receipts a staking claim records for its gains: a Transfer from the
// pool's claim address and a Claim receipt with the staking claim type per gain, in asset order from
// FixAuditChangesV7, with non-positive gains skipped only when asked (KLC-2770).
func TestAddStakingGainReceipts(t *testing.T) {
	t.Parallel()

	owner := []byte("klv1gainreceiptowner000000000000")
	gains := map[string]int64{"KLV": 30, "KFI": 20, "EMPTY-0000": 0}
	kda := &kapps.KDAData{AssetType: kapps.KDAData_Fungible}
	newKApp := func(v7 bool) *accountsKapp {
		return &accountsKapp{forkController: &integrationMock.ForkControllerStub{
			FixAuditChangesV7Called: func() bool { return v7 },
		}}
	}
	newCtx := func() kapp.KappContext {
		return kapp.NewKappContext(kapp.ArgsNewKAppContext{OriginalSender: owner, ContractID: 4, Block: &block.Block{}})
	}

	type gainReceipts struct {
		from  []byte
		asset string
		gain  int64
		claim string
	}
	collect := func(t *testing.T, ctx kapp.KappContext, assetID []byte) []gainReceipts {
		t.Helper()
		receipts := ctx.Receipts().Get()
		require.Zero(t, len(receipts)%2, "a Transfer and a Claim receipt per gain")

		var out []gainReceipts
		for i := 0; i < len(receipts); i += 2 {
			transfer, claim := receipts[i].Data, receipts[i+1].Data
			require.Equal(t, txProcess.Transfer, txProcess.ReceiptType(transfer[0][0]))
			require.Equal(t, txProcess.Claim, txProcess.ReceiptType(claim[0][0]))
			require.Equal(t, owner, transfer[2])
			require.Equal(t, transfer[3], claim[1], "same value on both receipts")
			require.Equal(t, transfer[4], claim[5], "same gain asset on both receipts")
			require.Equal(t, assetID, claim[4])
			require.Equal(t, []byte{byte(kapps.KDAData_Fungible)}, transfer[6])

			gain, err := strconv.ParseInt(string(transfer[3]), 10, 64)
			require.NoError(t, err)
			out = append(out, gainReceipts{from: transfer[1], asset: string(transfer[4]), gain: gain, claim: string(claim[6])})
		}
		return out
	}
	staking := strconv.Itoa(int(transaction.ClaimContract_StakingClaim))

	t.Run("every gain, in asset order, from an FPR pool", func(t *testing.T) {
		a := newKApp(true)
		for i := 0; i < 20; i++ {
			ctx := newCtx()
			recorded := a.addStakingGainReceipts(ctx, owner, []byte("FPR-1234"), gains, kapps.StakingData_FPRI, kda, false)
			require.Equal(t, 3, recorded)
			require.Equal(t, []gainReceipts{
				{from: kapps.StakingKAppAddress, asset: "EMPTY-0000", gain: 0, claim: staking},
				{from: kapps.StakingKAppAddress, asset: "KFI", gain: 20, claim: staking},
				{from: kapps.StakingKAppAddress, asset: "KLV", gain: 30, claim: staking},
			}, collect(t, ctx, []byte("FPR-1234")), "run %d", i)
		}
	})

	t.Run("non-positive gains skipped, minted from an APR pool", func(t *testing.T) {
		ctx := newCtx()
		recorded := newKApp(true).addStakingGainReceipts(ctx, owner, []byte("APR-1234"), gains, kapps.StakingData_APRI, kda, true)
		require.Equal(t, 2, recorded)
		require.Equal(t, []gainReceipts{
			{from: core.ZeroAddress, asset: "KFI", gain: 20, claim: staking},
			{from: core.ZeroAddress, asset: "KLV", gain: 30, claim: staking},
		}, collect(t, ctx, []byte("APR-1234")))
	})

	t.Run("KLV staking pays from the staking kapp", func(t *testing.T) {
		ctx := newCtx()
		newKApp(true).addStakingGainReceipts(ctx, owner, kdautils.KLVIdentifier, map[string]int64{"KLV": 5}, kapps.StakingData_FPRI, kda, false)
		require.Equal(t, []gainReceipts{
			{from: kapps.StakingKAppAddress, asset: "KLV", gain: 5, claim: staking},
		}, collect(t, ctx, kdautils.KLVIdentifier))
	})

	t.Run("map order before FixAuditChangesV7", func(t *testing.T) {
		a := newKApp(false)
		unsorted := false
		for i := 0; i < 20; i++ {
			ctx := newCtx()
			require.Equal(t, 3, a.addStakingGainReceipts(ctx, owner, []byte("FPR-1234"), gains, kapps.StakingData_FPRI, kda, false))
			unsorted = unsorted || collect(t, ctx, []byte("FPR-1234"))[0].asset != "EMPTY-0000"
		}
		require.True(t, unsorted, "pre-fork order must stay Go's map order")
	})
}

// TestAddGainReceipts_Allowance pins the receipts of an allowance claim's gain: the allowance claim type and
// the asset type the caller passes.
func TestAddGainReceipts_Allowance(t *testing.T) {
	t.Parallel()

	owner := []byte("klv1gainreceiptowner000000000000")
	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{OriginalSender: owner, ContractID: 7, Block: &block.Block{}})
	a := &accountsKapp{forkController: &integrationMock.ForkControllerStub{}}

	a.addGainReceipts(ctx, gainReceipt{
		from:      kapps.StakingKAppAddress,
		owner:     owner,
		assetID:   kdautils.KLVIdentifier,
		asset:     kdautils.KLVIdentifier,
		gain:      42,
		assetType: []byte{byte(kapps.KDAData_Fungible)},
		claimType: transaction.ClaimContract_AllowanceClaim,
	})

	require.Equal(t, []*transaction.Transaction_Receipt{
		txProcess.NewReceipt(txProcess.Transfer, 7, kapps.StakingKAppAddress, owner, []byte("42"),
			kdautils.KLVIdentifier, nil, []byte{byte(kapps.KDAData_Fungible)}),
		txProcess.NewReceipt(txProcess.Claim, 7, []byte("42"), nil, nil,
			kdautils.KLVIdentifier, kdautils.KLVIdentifier, []byte(strconv.Itoa(int(transaction.ClaimContract_AllowanceClaim)))),
	}, ctx.Receipts().Get())
}
