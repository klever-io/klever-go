package txcostestimator

import (
	"fmt"
	"math"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/txsimulator"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools/check"
)

const dummySignature = "01010101"

type transactionCostEstimator struct {
	accountsCacher state.AccountsCacher
	feeHandler     process.EconomicsDataHandler
	txSimulator    txsimulator.TransactionSimulatorProcessor
	forkController core.ForkController
}

// NewTransactionCostEstimator will create a new transaction cost estimator
func NewTransactionCostEstimator(
	accountsCacher state.AccountsCacher,
	feeHandler process.EconomicsDataHandler,
	txSimulator txsimulator.TransactionSimulatorProcessor,
	forkController core.ForkController,
) (*transactionCostEstimator, error) {
	if check.IfNil(accountsCacher) {
		return nil, process.ErrNilAccountsAdapter
	}
	if check.IfNil(feeHandler) {
		return nil, process.ErrNilEconomicsFeeHandler
	}
	if check.IfNil(txSimulator) {
		return nil, common.ErrNilTxSimulatorProcessor
	}
	if check.IfNil(forkController) {
		return nil, process.ErrNilEnableEpochsHandler
	}

	tce := &transactionCostEstimator{
		feeHandler:     feeHandler,
		txSimulator:    txSimulator,
		accountsCacher: accountsCacher,
		forkController: forkController,
	}

	return tce, nil
}

// EstimateTransactionGas will calculate how many gas units a transaction will consume
func (tce *transactionCostEstimator) EstimateTransactionGas(tx *transaction.Transaction) (*transaction.CostResponse, error) {
	tce.addMissingFieldsIfNeeded(tx)

	res, err := tce.txSimulator.ProcessTx(tx)
	if err != nil {
		return &transaction.CostResponse{
			KAppFee:       0,
			BandwidthFee:  0,
			GasEstimated:  0,
			GasMultiplier: 0,
			RetMessage:    err.Error(),
		}, nil
	}

	if res.FailReason != "" {
		return &transaction.CostResponse{
			KAppFee:       0,
			BandwidthFee:  0,
			GasEstimated:  0,
			GasMultiplier: 0,
			RetMessage:    res.FailReason,
		}, nil
	}

	if res.VMOutput == nil {
		return &transaction.CostResponse{
			KAppFee:       0,
			BandwidthFee:  0,
			GasEstimated:  0,
			GasMultiplier: 0,
			RetMessage:    process.ErrNilVMOutput.Error(),
		}, nil
	}

	totalGasConsumed := res.VMOutput.ComputeTotalGasConsumed()

	return &transaction.CostResponse{
		KAppFee:       tx.GetKAppFee(),
		BandwidthFee:  tx.GetBandwidthFee(),
		GasMultiplier: tx.GetGasMultiplier(),
		GasEstimated:  totalGasConsumed.Uint64(),
		RetMessage:    fmt.Sprintf("%s %s", res.VMOutput.ReturnCode.String(), res.VMOutput.ReturnMessage),
	}, nil
}

func (tce *transactionCostEstimator) addMissingFieldsIfNeeded(tx *transaction.Transaction) {
	if len(tx.Signature) == 0 {
		tx.Signature = [][]byte{[]byte(dummySignature)}
	}
	if tx.GasLimit == 0 {
		//add max gas limit for estimated calculation
		tx.GasLimit = math.MaxInt64
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (tce *transactionCostEstimator) IsInterfaceNil() bool {
	return tce == nil
}
