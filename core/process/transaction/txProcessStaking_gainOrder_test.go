package transaction_test

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	pTX "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

var (
	stakingClaimType   = strconv.Itoa(int(transaction.ClaimContract_StakingClaim))
	allowanceClaimType = strconv.Itoa(int(transaction.ClaimContract_AllowanceClaim))
	sortedGains        = [][]byte{kdautils.KFIIdentifier, kdautils.KLVIdentifier}
)

// TestStakingTxProcessor_FPRGainReceiptsInAssetOrder pins the order of the gain receipts a staking
// operation records when an FPR claim pays several currencies: from FixAuditChangesV7 they follow the
// sorted asset IDs (KFI before KLV) on every node, instead of Go's random map order (KLC-2770). Every
// staker claims its own gains map, so each one is an independent draw of the map order.
func TestStakingTxProcessor_FPRGainReceiptsInAssetOrder(t *testing.T) {
	const numStakers = 16

	ops := []struct {
		name         string
		prepare      proto.Message // sent by each staker while the claim window is shut, when set
		contract     proto.Message
		contractType transaction.TXContract_ContractType
	}{
		{"claim", nil, &transaction.ClaimContract{ClaimType: transaction.ClaimContract_StakingClaim, ID: FPRIdentifier}, transaction.TXContract_ClaimContractType},
		{"freeze", nil, &transaction.FreezeContract{AssetID: FPRIdentifier, Amount: topUpIncrement}, transaction.TXContract_FreezeContractType},
		{"unfreeze", nil, &transaction.UnfreezeContract{AssetID: FPRIdentifier, BucketID: FPRIdentifier}, transaction.TXContract_UnfreezeContractType},
		{
			"withdraw",
			&transaction.UnfreezeContract{AssetID: FPRIdentifier, BucketID: FPRIdentifier},
			&transaction.WithdrawContract{AssetID: FPRIdentifier, WithdrawType: transaction.WithdrawContract_Staking},
			transaction.TXContract_WithdrawContractType,
		},
	}

	for _, op := range ops {
		for _, v7 := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/v7=%t", op.name, v7), func(t *testing.T) {
				requireGainOrders(t, v7, runFPRGainOps(t, numStakers, v7, op.prepare, op.contract, op.contractType))
			})
		}
	}
}

// TestStakingTxProcessor_KLVGainReceiptsInAssetOrder does the same for Delegate and Undelegate, which claim
// a KLV bucket's gains from the KLV pool, here paying both KLV and KFI.
func TestStakingTxProcessor_KLVGainReceiptsInAssetOrder(t *testing.T) {
	const numStakers = 16

	for _, op := range []string{"delegate", "undelegate"} {
		for _, v7 := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/v7=%t", op, v7), func(t *testing.T) {
				requireGainOrders(t, v7, runKLVGainOps(t, numStakers, v7, op))
			})
		}
	}
}

// requireGainOrders checks each staker's gain order: sorted on every staker from FixAuditChangesV7, and
// Go's map order on at least one staker before.
func requireGainOrders(t *testing.T, v7 bool, orders [][][]byte) {
	t.Helper()

	if v7 {
		for i, order := range orders {
			require.Equal(t, sortedGains, order, "staker %d", i)
		}
		return
	}

	unsorted := false
	for _, order := range orders {
		require.ElementsMatch(t, sortedGains, order)
		unsorted = unsorted || !bytes.Equal(order[0], sortedGains[0])
	}
	require.True(t, unsorted, "pre-fork order must stay Go's map order")
}

func gainEpochs(v7 bool) config.EnableEpochs {
	epochs := config.EnableEpochs{FixAuditChangesV5: 0}
	if !v7 {
		epochs.FixAuditChangesV7 = 1_000
	}
	return epochs
}

// runFPRGainOps stakes numStakers accounts in one FPR pool paid in KLV and KFI. Each one sends prepare, when
// set, while the claim window is shut, then contract once it opens. It returns the gain assets of each
// one's receipts in the order they were recorded.
func runFPRGainOps(
	t *testing.T,
	numStakers int,
	v7 bool,
	prepare proto.Message,
	contract proto.Message,
	contractType transaction.TXContract_ContractType,
) [][][]byte {
	t.Helper()

	c := NewController(t)
	c.UpdateForkController(gainEpochs(v7))

	stakers := gainStakers(numStakers)
	for _, staker := range stakers {
		c.AddUser(staker, topUpInitialKLV, kdautils.KLVIdentifier)
		c.AddUser(staker, topUpInitialFPR, FPRIdentifier)
	}
	c.AddUser(RefAddress, topUpInitialKLV, kdautils.KLVIdentifier)
	c.AddUser(RefAddress, topUpInitialKLV, kdautils.KFIIdentifier)

	blk := c.CreateBlockHeader(0, topUpStakeEpoch, 1)
	updateFPRStakingConfig(t, c, blk, topUpClaimWindow, 1, 0)
	for _, staker := range stakers {
		runStakingContractTx(t, c, blk, &transaction.FreezeContract{
			AssetID: FPRIdentifier, Amount: topUpBaseStake,
		}, transaction.TXContract_FreezeContractType, staker, nil)
	}

	blk.Header.Epoch = topUpStakeEpoch + 1
	for _, currency := range [][]byte{kdautils.KLVIdentifier, kdautils.KFIIdentifier} {
		runStakingContractTx(t, c, blk, &transaction.DepositContract{
			DepositType: transaction.DepositContract_FPRDeposit, ID: FPRIdentifier,
			CurrencyID: currency, Amount: topUpRewardPool,
		}, transaction.TXContract_DepositContractType, RefAddress, nil)
	}

	if prepare != nil {
		blk.Header.Epoch = topUpRewardEpoch
		for _, staker := range stakers {
			tx := runStakingContractTx(t, c, blk, proto.Clone(prepare), gainContractType(prepare), staker, nil)
			require.Empty(t, requireGainReceipts(t, tx, FPRIdentifier, stakingClaimType), "no gain while the claim window is shut")
		}
	}

	blk.Header.Epoch = topUpMatureEpoch
	orders := make([][][]byte, 0, numStakers)
	for _, staker := range stakers {
		tx := runStakingContractTx(t, c, blk, proto.Clone(contract), contractType, staker, nil)
		order := requireGainReceipts(t, tx, FPRIdentifier, stakingClaimType)
		require.Len(t, order, 2, "one gain per reward currency")
		orders = append(orders, order)
	}

	return orders
}

// runKLVGainOps stakes numStakers KLV buckets, adds a KLV pool paying KLV and KFI, then has each staker
// delegate its bucket, or undelegate it after delegating it first. It returns the gain assets of each
// one's receipts in the order they were recorded.
func runKLVGainOps(t *testing.T, numStakers int, v7 bool, op string) [][][]byte {
	t.Helper()

	const (
		initialBalance = int64(1_000_000_000_000)
		freezeAmount   = int64(1_000_000_000)
	)

	c := NewController(t)
	c.UpdateForkController(gainEpochs(v7))
	c.AddUser(validatorA, initialBalance, kdautils.KLVIdentifier)
	c.AddUser(validatorB, initialBalance, kdautils.KLVIdentifier)

	blk := c.CreateBlockHeader(0, 1, 1)
	CreateValidators(c, blk)

	stakers := gainStakers(numStakers)
	buckets := make([][]byte, numStakers)
	for i, staker := range stakers {
		c.AddUser(staker, initialBalance, kdautils.KLVIdentifier)
		buckets[i] = c.RunFreezeTX(c.CreateBlockHeader(0, 1, uint64(i+2)), staker, kdautils.KLVIdentifier, freezeAmount)
		if op == "undelegate" {
			c.RunDelegateTX(blk, staker, buckets[i], validatorA)
		}
	}

	c.AddToStakingPool(kdautils.KLVIdentifier, &kapps.FPRData{
		TotalAmount: 1_000_000,
		TotalStaked: freezeAmount * int64(numStakers),
		Epoch:       2,
		KDAS: map[string]*kapps.KDAFPRData{
			string(kdautils.KFIIdentifier): {TotalAmount: 1_000_000},
		},
	}, nil)

	blk = c.CreateBlockHeader(0, 2, 1)
	orders := make([][][]byte, 0, numStakers)
	for i, staker := range stakers {
		contract, contractType := klvGainContract(op, buckets[i])
		tx := runStakingContractTx(t, c, blk, contract, contractType, staker, nil)
		order := requireGainReceipts(t, tx, kdautils.KLVIdentifier, stakingClaimType)
		require.Len(t, order, 2, "one gain per reward currency")
		orders = append(orders, order)
	}

	return orders
}

func klvGainContract(op string, bucketID []byte) (proto.Message, transaction.TXContract_ContractType) {
	if op == "undelegate" {
		return &transaction.UndelegateContract{BucketID: bucketID}, transaction.TXContract_UndelegateContractType
	}
	return &transaction.DelegateContract{BucketID: bucketID, ToAddress: validatorA}, transaction.TXContract_DelegateContractType
}

func gainContractType(contract proto.Message) transaction.TXContract_ContractType {
	switch contract.(type) {
	case *transaction.UnfreezeContract:
		return transaction.TXContract_UnfreezeContractType
	default:
		panic(fmt.Sprintf("no contract type for %T", contract))
	}
}

func gainStakers(n int) [][]byte {
	stakers := make([][]byte, n)
	for i := range stakers {
		stakers[i] = []byte(fmt.Sprintf("klv1gainorderstaker%013d", i))
	}
	return stakers
}

// requireGainReceipts checks every gain's receipts in tx: a Transfer of a KLV or KFI gain from the staking
// kapp, followed by its Claim receipt with the same value and gain asset, the claimed asset and the claim
// type. It returns the gain assets in receipt order.
func requireGainReceipts(t *testing.T, tx *transaction.Transaction, claimedAsset []byte, claimType string) [][]byte {
	t.Helper()

	receipts := tx.GetReceipts()
	assets := make([][]byte, 0, 2)
	for i, receipt := range receipts {
		transfer := receipt.GetData()
		// Transfer receipt data: type, from, to, value, asset, nonce, asset type
		if len(transfer) < 7 || pTX.ReceiptType(transfer[0][0]) != pTX.Transfer ||
			(!bytes.Equal(transfer[4], kdautils.KLVIdentifier) && !bytes.Equal(transfer[4], kdautils.KFIIdentifier)) {
			continue
		}
		require.Equal(t, kapps.StakingKAppAddress, transfer[1], "gain paid from the staking kapp")

		require.Less(t, i+1, len(receipts), "a Claim receipt follows each gain's Transfer")
		claim := receipts[i+1].GetData()
		// Claim receipt data: type, value, nil, nil, claimed asset, gain asset, claim type
		require.Len(t, claim, 7)
		require.Equal(t, pTX.Claim, pTX.ReceiptType(claim[0][0]))
		require.Equal(t, transfer[3], claim[1], "claim value")
		require.Equal(t, claimedAsset, claim[4], "claimed asset")
		require.Equal(t, transfer[4], claim[5], "gain asset")
		require.Equal(t, claimType, string(claim[6]), "claim type")

		assets = append(assets, transfer[4])
	}

	return assets
}

// TestStakingTxProcessor_AllowanceClaimReceipts pins the receipts of a successful allowance claim: a KLV
// Transfer from the staking kapp marked fungible, and a Claim receipt carrying the allowance claim type.
func TestStakingTxProcessor_AllowanceClaimReceipts(t *testing.T) {
	const allowance = int64(10_000)

	c := NewController(t)
	acc := c.AddUser(OwnerAddress, topUpInitialKLV, nil)
	c.AddAllowance(acc, allowance)

	tx := c.RunClaimTX(c.CreateBlockHeader(0, 1, 1), transaction.ClaimContract_AllowanceClaim, OwnerAddress, kdautils.KLVIdentifier, nil)

	require.Equal(t, [][]byte{kdautils.KLVIdentifier}, requireGainReceipts(t, tx, kdautils.KLVIdentifier, allowanceClaimType))
	for _, receipt := range tx.GetReceipts() {
		data := receipt.GetData()
		if pTX.ReceiptType(data[0][0]) != pTX.Transfer {
			continue
		}
		require.Equal(t, OwnerAddress, data[2])
		require.Equal(t, strconv.FormatInt(allowance, 10), string(data[3]))
		require.Equal(t, []byte{byte(kapps.KDAData_Fungible)}, data[6])
	}
}
