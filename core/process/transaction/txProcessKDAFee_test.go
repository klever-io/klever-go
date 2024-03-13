package transaction_test

import (
	"testing"

	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"

	commonMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	pTX "github.com/klever-io/klever-go/core/process/transaction"
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
	assert.Equal(t, initialBalance, ownerAcc.GetBalance(kdautils.KLVIdentifier))
	assert.Equal(t, kdaBalance, ownerAcc.GetBalance(kdaID))
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
	assert.Equal(t, initialBalance, ownerAcc.GetBalance(kdautils.KLVIdentifier))
	assert.Equal(t, kdaBalance, ownerAcc.GetBalance(kdaID))
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
