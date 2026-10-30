package transaction_test

import (
	"math/big"
	"testing"

	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"

	"github.com/klever-io/klever-go/common"
	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	pTX "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	dataTransaction "github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func computeTransactionCost(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
	return &transaction.CostResponse{
		BandwidthFee: 500_000,
		KAppFee:      1_000_000,
	}, nil
}

var (
	nonceCounter = uint64(0)
)

func SetupKappController(t *testing.T, args *pTX.ArgsNewTxProcessor) {
	kappArgs := kappcontroller.ArgsNewKApp{
		Hasher:         args.Hasher,
		Marshalizer:    args.Marshalizer,
		PubkeyConv:     args.PubkeyConv,
		ForkController: args.ForkController,
		AccountsCacher: args.AccountsCacher,
		RatingsData:    args.RatingsData,
	}

	proposalKapp, err := args.AccountsCacher.LoadKApp(kapps.ProposalKAppAddress)
	require.NoError(t, err)
	pc := initProposalKapp(proposalKapp)
	err = args.AccountsCacher.SaveAll()
	require.NoError(t, err)

	args.KAppController, err = kappcontroller.NewKappController(kappArgs)
	require.NoError(t, err)

	InitKapps(args.KAppController, args.AccountsCacher, pc)
}

func TestTxProcessor_ProcessTransferKDAFee(t *testing.T) {
	t.Parallel()

	initialBalance := int64(100_000_000)
	poolDeposit := int64(10_000_000)
	transferAmount := int64(10_000_000)
	kdaBalance := int64(10_000_000_000)

	args := createBaseKAppsProcessingArgs2()
	args.EconomicsFee = &commonMock.FeeHandlerStub{
		CheckValidityTxValuesCalled: func(tx process.TransactionWithFeeHandler) (*dataTransaction.CostResponse, error) {
			cost, _ := computeTransactionCost(tx)
			if tx.GetBandwidthFee() < cost.BandwidthFee ||
				tx.GetKAppFee() < cost.KAppFee {
				return nil, process.ErrInvalidTransactionFees
			}
			return cost, nil
		},
		ComputeTransactionCostCalled: computeTransactionCost,
	}

	kdaKapp, err := args.AccountsCacher.LoadKApp(kapps.KDAKAppAddress)
	require.Nil(t, err)
	stakingKapp, err := args.AccountsCacher.LoadKApp(kapps.StakingKAppAddress)
	require.Nil(t, err)
	initKLVAndKFIintoKapps(kdaKapp, stakingKapp)
	err = args.AccountsCacher.SaveAll()
	require.Nil(t, err)

	SetupKappController(t, &args)

	execTx, err := pTX.NewTxProcessor(args)
	require.Nil(t, err)

	// KDA ID: TEST-2RQM
	kdaID := []byte("TEST-2RQM")
	tx, err := execute(t, execTx, createKDA(kdaBalance), transaction.TXContract_CreateAssetContractType, nil)
	require.Nil(t, err)
	initialBalance -= tx.GetTotalFees()

	tx, err = execute(t, execTx, createKDAPool(kdaID), transaction.TXContract_AssetTriggerContractType, nil)
	require.Nil(t, err)
	initialBalance -= tx.GetTotalFees()

	tx, err = execute(t, execTx, createPoolDeposit(kdaID, []byte("KLV"), poolDeposit), transaction.TXContract_DepositContractType, nil)
	require.Nil(t, err)
	initialBalance -= tx.GetTotalFees()
	initialBalance -= poolDeposit

	tx, err = execute(t, execTx, createTransfer(transferAmount), transaction.TXContract_TransferContractType, &KDAFee{kdaID, 100})
	require.Nil(t, err)
	initialBalance -= transferAmount
	kdaBalance -= tx.RawData.KDAFee.Amount
	poolDeposit -= tx.GetTotalFees()

	kdaAmount := tx.RawData.KDAFee.Amount

	// Validate Balance
	ownerAcc, err := args.AccountsCacher.LoadUser(testOwnerAddress)
	require.Nil(t, err)
	assert.Equal(t, initialBalance, ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, kdaBalance, ownerAcc.GetBalance(kdaID, true))
	assert.Equal(t, nonceCounter, ownerAcc.GetNonce())

	// withdraw KLV from POOL
	tx, err = execute(t, execTx, createPoolWitdraw(kdaID, []byte("KLV"), poolDeposit), transaction.TXContract_WithdrawContractType, nil)
	require.Nil(t, err)
	initialBalance -= tx.GetTotalFees()
	initialBalance += poolDeposit

	// withdraw KDA from POOL
	tx, err = execute(t, execTx, createPoolWitdraw(kdaID, kdaID, kdaAmount), transaction.TXContract_WithdrawContractType, nil)
	require.Nil(t, err)
	initialBalance -= tx.GetTotalFees()
	kdaBalance += kdaAmount

	// Validate Balance
	ownerAcc, err = args.AccountsCacher.LoadUser(testOwnerAddress)
	require.Nil(t, err)
	assert.Equal(t, initialBalance, ownerAcc.GetBalance(kdautils.KLVIdentifier, true))
	assert.Equal(t, kdaBalance, ownerAcc.GetBalance(kdaID, true))
	assert.Equal(t, nonceCounter, ownerAcc.GetNonce())

}

func createKDA(initital int64) proto.Message {
	return &transaction.CreateAssetContract{
		Type:          transaction.CreateAssetContract_Fungible,
		Name:          []byte("KDA"),
		Ticker:        []byte("TEST"),
		OwnerAddress:  testOwnerAddress,
		Precision:     6,
		InitialSupply: initital,
		MaxSupply:     0,
		Properties: &transaction.PropertiesInfo{
			CanMint: true,
		},
	}
}

func createKDAPool(kdaID []byte) proto.Message {
	return &transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateKDAFeePool,
		AssetID:     kdaID,
		KDAPool: &transaction.KDAPoolInfo{
			Active:       true,
			AdminAddress: testOwnerAddress,
			FRatioKLV:    1000,
			FRatioKDA:    100000,
		},
	}
}

func createPoolDeposit(poolID []byte, currency []byte, amount int64) proto.Message {
	return &transaction.DepositContract{
		DepositType: transaction.DepositContract_KDAPool,
		ID:          poolID,
		CurrencyID:  currency,
		Amount:      amount,
	}
}

func createPoolWitdraw(poolID []byte, currency []byte, amount int64) proto.Message {
	return &transaction.WithdrawContract{
		WithdrawType: transaction.WithdrawContract_KDAPool,
		AssetID:      poolID,
		CurrencyID:   currency,
		Amount:       amount,
	}
}

func createTransfer(amount int64) proto.Message {
	return &transaction.TransferContract{
		ToAddress: testToAddress,
		Amount:    amount,
	}
}

type KDAFee struct {
	KDA   []byte
	Ratio float64
}

func execute(
	t *testing.T,
	execTx process.TransactionProcessor,
	contract proto.Message,
	txType transaction.TXContract_ContractType,
	kdaFee *KDAFee,
) (tx *transaction.Transaction, err error) {

	tx, err = createTransactionMock(contract, txType, testOwnerAddress, nonceCounter)
	if err != nil {
		return
	}
	nonceCounter++

	// update TX fee
	cost, _ := computeTransactionCost(tx)
	tx.RawData.BandwidthFee = cost.BandwidthFee
	tx.RawData.KAppFee = cost.KAppFee

	if kdaFee != nil {
		tx.RawData.KDAFee = &transaction.Transaction_KDAFee{
			KDA:    kdaFee.KDA,
			Amount: int64(float64(tx.GetTotalFees()) * kdaFee.Ratio),
		}
	}

	ownerAcc, txHash, err := execTx.PreProcessTransaction(tx)
	if err != nil {
		return
	}

	_, err = execTx.ProcessBandwidthFee(nil, tx, ownerAcc)
	if err != nil {
		return
	}

	err = execTx.ProcessTransaction(createBlockHeader(), txHash, tx)
	if err != nil {
		return
	}

	return
}

const (
	revertTestBandwidthFee = int64(500_000)
	revertTestKAppFee      = int64(1_000_000)
	revertTestPoolDeposit  = int64(50_000_000)
	revertTestRatioKLV     = int64(3)
	revertTestRatioKDA     = int64(7)

	// revertTestTimestamp stands in for the wall clock, so two flows built one after
	// the other persist the same state
	revertTestTimestamp = int64(1_767_225_600)
)

// kdaFeeBlockFlow drives a tx processor the way the block preprocessor does: the
// bandwidth fee is charged first, then ProcessTransaction runs against a snapshot
// that is reverted when it fails.
type kdaFeeBlockFlow struct {
	t        *testing.T
	execTx   process.TransactionProcessor
	accounts state.AccountsAdapter
	kapps    state.AccountsAdapter
	cacher   state.AccountsCacher
	nonce    uint64
	assetID  []byte
}

func newKDAFeeBlockFlow(t *testing.T, fixAuditChangesV7Epoch uint32) *kdaFeeBlockFlow {
	userDB, kappDB, peerDB, accCacher := createFullArgumentsForKAppsProcessingMemory()
	args := createBaseKAppsProcessingArgsCommon(userDB, kappDB, peerDB, accCacher)
	args.EconomicsFee = &commonMock.FeeHandlerStub{
		CheckValidityTxValuesCalled: func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
			if tx.GetBandwidthFee() < revertTestBandwidthFee || tx.GetKAppFee() < revertTestKAppFee {
				return nil, process.ErrInvalidTransactionFees
			}
			return &transaction.CostResponse{BandwidthFee: revertTestBandwidthFee, KAppFee: revertTestKAppFee}, nil
		},
		ComputeTransactionCostCalled: func(tx process.TransactionWithFeeHandler) (*transaction.CostResponse, error) {
			return &transaction.CostResponse{BandwidthFee: revertTestBandwidthFee, KAppFee: revertTestKAppFee}, nil
		},
	}

	forkController, err := fork.NewForkController(config.EnableEpochs{
		FixAuditChangesV7: fixAuditChangesV7Epoch,
	}, &commonMock.EpochNotifierStub{})
	require.NoError(t, err)
	args.ForkController = forkController

	kdaKapp, err := accCacher.LoadKApp(kapps.KDAKAppAddress)
	require.NoError(t, err)
	stakingKapp, err := accCacher.LoadKApp(kapps.StakingKAppAddress)
	require.NoError(t, err)
	initKLVAndKFIintoKappsWithTime(kdaKapp, stakingKapp, revertTestTimestamp-10*24*60*60)
	require.NoError(t, accCacher.SaveAll())

	SetupKappController(t, &args)

	execTx, err := pTX.NewTxProcessor(args)
	require.NoError(t, err)

	flow := &kdaFeeBlockFlow{
		t:        t,
		execTx:   execTx,
		accounts: userDB,
		kapps:    kappDB,
		cacher:   accCacher,
	}

	tx, err := flow.run(createKDA(10_000_000_000), transaction.TXContract_CreateAssetContractType, revertTestKAppFee, 0)
	require.NoError(t, err)
	flow.assetID = createdAssetID(t, tx)

	_, err = flow.run(&transaction.AssetTriggerContract{
		TriggerType: transaction.AssetTriggerContract_UpdateKDAFeePool,
		AssetID:     flow.assetID,
		KDAPool: &transaction.KDAPoolInfo{
			Active:       true,
			AdminAddress: testOwnerAddress,
			FRatioKLV:    revertTestRatioKLV,
			FRatioKDA:    revertTestRatioKDA,
		},
	}, transaction.TXContract_AssetTriggerContractType, revertTestKAppFee, 0)
	require.NoError(t, err)

	_, err = flow.run(
		createPoolDeposit(flow.assetID, kdautils.KLVIdentifier, revertTestPoolDeposit),
		transaction.TXContract_DepositContractType,
		revertTestKAppFee,
		0,
	)
	require.NoError(t, err)

	return flow
}

func createdAssetID(t *testing.T, tx *transaction.Transaction) []byte {
	for _, r := range tx.GetReceipts() {
		if len(r.GetData()) >= 2 && len(r.GetData()[0]) > 0 && r.GetData()[0][0] == 1 {
			return r.GetData()[1]
		}
	}
	require.Fail(t, "create asset receipt not found")
	return nil
}

// run sends contract with the given KApp fee; kdaAmount > 0 pays the fees in the
// pool's KDA with that signed amount.
func (f *kdaFeeBlockFlow) run(contract proto.Message, txType transaction.TXContract_ContractType, kappFee int64, kdaAmount int64) (*transaction.Transaction, error) {
	tx, err := createTransactionMock(contract, txType, testOwnerAddress, f.nonce)
	require.NoError(f.t, err)

	tx.RawData.BandwidthFee = revertTestBandwidthFee
	tx.RawData.KAppFee = kappFee
	if kdaAmount > 0 {
		tx.RawData.KDAFee = &transaction.Transaction_KDAFee{KDA: f.assetID, Amount: kdaAmount}
	}

	ownerAcc, txHash, err := f.execTx.PreProcessTransaction(tx)
	require.NoError(f.t, err)

	_, err = f.execTx.ProcessBandwidthFee(txHash, tx, ownerAcc)
	require.NoError(f.t, err)
	f.nonce++

	accsSnapshot := f.accounts.JournalLen()
	kappsSnapshot := f.kapps.JournalLen()

	hdr := createBlockHeader()
	hdr.Header.Timestamp = revertTestTimestamp
	err = f.execTx.ProcessTransaction(hdr, txHash, tx)
	if err != nil {
		require.NoError(f.t, f.accounts.RevertToSnapshot(accsSnapshot))
		require.NoError(f.t, f.kapps.RevertToSnapshot(kappsSnapshot))
	}

	return tx, err
}

type kdaFeeBalances struct {
	ownerKLV int64
	ownerKDA int64
	poolKLV  int64
	poolKDA  int64
}

func (f *kdaFeeBlockFlow) balances() kdaFeeBalances {
	f.cacher.ResetAll(true)

	ownerAcc, err := f.cacher.LoadUser(testOwnerAddress)
	require.NoError(f.t, err)

	poolApp, err := f.cacher.LoadKApp(kapps.KDAFeesPoolKAppAddress)
	require.NoError(f.t, err)
	pool := &kdafeespool.KDAFeesPoolData{}
	require.NoError(f.t, (&commonMock.ProtoMarshalizerMock{}).Unmarshal(pool, poolApp.GetStorage(f.assetID)))

	return kdaFeeBalances{
		ownerKLV: ownerAcc.GetBalance(kdautils.KLVIdentifier, true),
		ownerKDA: ownerAcc.GetBalance(f.assetID, true),
		poolKLV:  pool.KLVBalance,
		poolKDA:  pool.KDABalance,
	}
}

// kdaAt is the pool's floor-rounded KDA price for klvAmount
func kdaAt(klvAmount int64) int64 {
	v := big.NewInt(klvAmount)
	v.Mul(v, big.NewInt(revertTestRatioKDA))
	v.Quo(v, big.NewInt(revertTestRatioKLV))
	return v.Int64()
}

// kdaUpAt is kdaAt rounded up, as the bandwidth share is priced
func kdaUpAt(klvAmount int64) int64 {
	v := big.NewInt(klvAmount)
	v.Mul(v, big.NewInt(revertTestRatioKDA))
	v.Add(v, big.NewInt(revertTestRatioKLV-1))
	v.Quo(v, big.NewInt(revertTestRatioKLV))
	return v.Int64()
}

func TestTxProcessor_KDAFeeFailedTransactionKeepsKAppShareInKDA(t *testing.T) {
	t.Parallel()

	// a KApp fee above its cost
	kappFee := int64(20_000_000)
	totalFee := revertTestBandwidthFee + kappFee
	overpay := int64(11)
	kdaAmount := kdaAt(totalFee) + overpay
	failingTransfer := createTransfer(1_000_000_000_000_000)
	okTransfer := createTransfer(1_000)

	t.Run("failed transaction pays only the bandwidth share", func(t *testing.T) {
		t.Parallel()

		flow := newKDAFeeBlockFlow(t, 0)
		before := flow.balances()

		tx, err := flow.run(failingTransfer, transaction.TXContract_TransferContractType, kappFee, kdaAmount)
		require.Error(t, err)
		require.NotEqual(t, transaction.Transaction_Ok, tx.ResultCode)

		after := flow.balances()
		assert.Equal(t, before.ownerKLV, after.ownerKLV, "sender must not keep KLV swapped out of the pool")
		assert.Equal(t, before.ownerKDA-kdaUpAt(revertTestBandwidthFee), after.ownerKDA)
		assert.Equal(t, before.poolKLV-revertTestBandwidthFee, after.poolKLV)
		assert.Equal(t, before.poolKDA+kdaUpAt(revertTestBandwidthFee), after.poolKDA)
	})

	t.Run("successful transaction swaps the whole signed amount", func(t *testing.T) {
		t.Parallel()

		flow := newKDAFeeBlockFlow(t, 0)
		before := flow.balances()

		_, err := flow.run(okTransfer, transaction.TXContract_TransferContractType, kappFee, kdaAmount)
		require.NoError(t, err)

		after := flow.balances()
		assert.Equal(t, before.ownerKLV-1_000, after.ownerKLV)
		assert.Equal(t, before.ownerKDA-kdaAmount, after.ownerKDA)
		assert.Equal(t, before.poolKLV-totalFee, after.poolKLV)
		assert.Equal(t, before.poolKDA+kdaAmount, after.poolKDA)
	})

	t.Run("successful transaction matches the legacy state", func(t *testing.T) {
		t.Parallel()

		fixed := newKDAFeeBlockFlow(t, 0)
		legacy := newKDAFeeBlockFlow(t, 1)

		_, err := fixed.run(okTransfer, transaction.TXContract_TransferContractType, kappFee, kdaAmount)
		require.NoError(t, err)
		_, err = legacy.run(okTransfer, transaction.TXContract_TransferContractType, kappFee, kdaAmount)
		require.NoError(t, err)

		assert.Equal(t, legacy.balances(), fixed.balances())

		fixedAccountsRoot, err := fixed.accounts.RootHash()
		require.NoError(t, err)
		legacyAccountsRoot, err := legacy.accounts.RootHash()
		require.NoError(t, err)
		assert.Equal(t, legacyAccountsRoot, fixedAccountsRoot)

		fixedKAppsRoot, err := fixed.kapps.RootHash()
		require.NoError(t, err)
		legacyKAppsRoot, err := legacy.kapps.RootHash()
		require.NoError(t, err)
		assert.Equal(t, legacyKAppsRoot, fixedKAppsRoot)
	})

	t.Run("before FixAuditChangesV7 a failed transaction keeps the legacy swap", func(t *testing.T) {
		t.Parallel()

		flow := newKDAFeeBlockFlow(t, 1)
		before := flow.balances()

		_, err := flow.run(failingTransfer, transaction.TXContract_TransferContractType, kappFee, kdaAmount)
		require.Error(t, err)

		after := flow.balances()
		assert.Equal(t, before.ownerKLV+kappFee, after.ownerKLV)
		assert.Equal(t, before.ownerKDA-kdaAmount, after.ownerKDA)
		assert.Equal(t, before.poolKLV-totalFee, after.poolKLV)
		assert.Equal(t, before.poolKDA+kdaAmount, after.poolKDA)
	})

	t.Run("pool that cannot cover the whole fee still refuses the bandwidth swap", func(t *testing.T) {
		t.Parallel()

		flow := newKDAFeeBlockFlow(t, 0)
		before := flow.balances()

		hugeKAppFee := before.poolKLV
		tx, err := createTransactionMock(okTransfer, transaction.TXContract_TransferContractType, testOwnerAddress, flow.nonce)
		require.NoError(t, err)
		tx.RawData.BandwidthFee = revertTestBandwidthFee
		tx.RawData.KAppFee = hugeKAppFee
		tx.RawData.KDAFee = &transaction.Transaction_KDAFee{
			KDA:    flow.assetID,
			Amount: kdaAt(revertTestBandwidthFee + hugeKAppFee),
		}

		ownerAcc, txHash, err := flow.execTx.PreProcessTransaction(tx)
		require.NoError(t, err)

		_, err = flow.execTx.ProcessBandwidthFee(txHash, tx, ownerAcc)
		require.ErrorIs(t, err, common.ErrAssetPoolOutOfFunds)

		assert.Equal(t, before, flow.balances())
	})
}
