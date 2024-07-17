package transaction

import (
	"fmt"
	"math"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/txsimulator"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
)

var _ txsimulator.TransactionProcessor = (*simulateTxProcessor)(nil)

// txProcessor implements TransactionProcessor interface and can modify account states according to a transaction
type simulateTxProcessor struct {
	*baseTxProcessor
}

// ArgsNewSimulateTxProcessor defines the arguments needed for new meta tx processor
type ArgsNewSimulateTxProcessor struct {
	Hasher          hashing.Hasher
	Marshalizer     marshal.Marshalizer
	AccountsCacher  state.AccountsCacher
	KAppsController kapp.KAppController
	PubkeyConv      core.PubkeyConverter
	ScProcessor     process.SmartContractProcessor
	EconomicsFee    process.EconomicsDataHandler
	ForkController  core.ForkController
}

// NewSimulateTxProcessor creates a new txProcessor engine
func NewSimulateTxProcessor(args ArgsNewSimulateTxProcessor) (*simulateTxProcessor, error) {
	if check.IfNil(args.AccountsCacher) {
		return nil, process.ErrNilAccountsAdapter
	}
	if check.IfNil(args.PubkeyConv) {
		return nil, process.ErrNilPubkeyConverter
	}
	if check.IfNil(args.ScProcessor) {
		return nil, process.ErrNilSmartContractProcessor
	}
	if check.IfNil(args.EconomicsFee) {
		return nil, process.ErrNilEconomicsFeeHandler
	}
	if check.IfNil(args.ForkController) {
		return nil, common.ErrNilForkController
	}

	baseTxProcess := &baseTxProcessor{
		accountsCacher: args.AccountsCacher,
		kApps:          args.KAppsController,
		pubkeyConv:     args.PubkeyConv,
		economicsFee:   args.EconomicsFee,
		hasher:         args.Hasher,
		marshalizer:    args.Marshalizer,
		scProcessor:    args.ScProcessor,
		forkController: args.ForkController,
	}

	txProc := &simulateTxProcessor{
		baseTxProcessor: baseTxProcess,
	}

	return txProc, nil
}

// ProcessTransaction modifies the account states in respect with the transaction data
func (txProc *simulateTxProcessor) ProcessTransaction(tx *transaction.Transaction) error {
	if check.IfNil(tx) {
		return process.ErrNilTransaction
	}
	txProc.accountsCacher.ResetAll(true)

	tx.GasLimit = math.MaxInt64

	ownerAcc, err := txProc.accountsCacher.LoadUser(tx.GetSender())
	if err != nil {
		tx.ResultCode = transaction.Transaction_LoadAccountError
		return err
	}

	ownerAcc.IncreaseNonce(1)

	computedHash, err := tools.CalculateHash(txProc.marshalizer, txProc.hasher, tx.RawData)
	if err != nil {
		tx.ResultCode = transaction.Transaction_Fail
		log.Error("invalid tx hash", "nonce", "computedHash", computedHash)
		return err
	}

	process.DisplayProcessTxDetails(
		"ProcessTransaction: sender account details",
		ownerAcc,
		tx,
		computedHash,
		txProc.pubkeyConv,
	)

	// retrieve last committed block
	headerHandler := txProc.scProcessor.LastBlock()
	blockHeader, ok := headerHandler.(*block.Block)
	if !ok {
		return common.ErrNilBlockChain
	}

	ctx := kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     -1,
		ContractType:   -1,
		Block:          blockHeader,
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	})

	for i := range tx.RawData.Contract {
		ctx.SetContractID(i)
		txProc.kApps.SetCurrentKAppContext(ctx)

		tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
		if err != nil {
			tx.ResultCode = transaction.Transaction_ContractInvalid
			return err
		}

		switch tc.GetType() {
		case transaction.SmartContract_SCDeploy:
			returnCode, err := txProc.scProcessor.DeploySmartContract(ctx, tc)
			if err != nil || returnCode != vmcommon.Ok {
				tx.ResultCode = returnCode.ResultCode()
				if err == nil {
					err = fmt.Errorf("%w: %s", process.ErrSmartContractDeploymentFailed, returnCode.String())
				}
				logSC.Debug("error deploying smart contract", "error", err, "returnCode", returnCode)
				return err
			}

			tx.ResultCode = transaction.Transaction_Ok
		case transaction.SmartContract_SCInvoke:
			sw := tools.NewStopWatch()
			sw.Start("execute")

			destAcc, err := txProc.accountsCacher.GetExistingUser(tc.GetAddress())
			if err != nil {
				tx.ResultCode = transaction.Transaction_AccountError
				return err
			}

			returnCode, err := txProc.scProcessor.ExecuteSmartContractTransaction(ctx, tc, ownerAcc, destAcc)
			if err != nil || returnCode != vmcommon.Ok {
				tx.ResultCode = returnCode.ResultCode()
				if err == nil {
					err = fmt.Errorf("%w: %s", process.ErrSmartContractInvokeFailed, returnCode.String())
				}
				logSC.Debug("error invoke smart contract", "error", err, "returnCode", returnCode)
				return err
			}

			sw.Stop("execute")
			duration := sw.GetMeasurement("execute")
			logSC.Trace("execute smart contract", "duration", duration)

			tx.ResultCode = transaction.Transaction_Ok
		default:
			tx.ResultCode = transaction.Transaction_ParameterInvalid
			return common.ErrSmartContractTypeInvalid
		}
	}

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (txProc *simulateTxProcessor) IsInterfaceNil() bool {
	return txProc == nil
}
