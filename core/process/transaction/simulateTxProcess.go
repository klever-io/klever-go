package transaction

import (
	"fmt"
	"math/big"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"

	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/txsimulator"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
)

var _ txsimulator.TransactionProcessor = (*simulateTxProcessor)(nil)

// txProcessor implements TransactionProcessor interface and can modify account states according to a transaction
type simulateTxProcessor struct {
	*baseTxProcessor
	vmOutputCacher storage.Cacher
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
	VMOutputCacher  storage.Cacher
}

// NewSimulateTxProcessor creates a new txProcessor engine
func NewSimulateTxProcessor(args ArgsNewSimulateTxProcessor) (*simulateTxProcessor, error) {
	if err := validateArgs(args); err != nil {
		return nil, err
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
		vmOutputCacher:  args.VMOutputCacher,
	}

	return txProc, nil
}

func validateArgs(args ArgsNewSimulateTxProcessor) error {
	if check.IfNil(args.Hasher) {
		return process.ErrNilHasher
	}
	if check.IfNil(args.Marshalizer) {
		return process.ErrNilMarshalizer
	}
	if check.IfNil(args.PubkeyConv) {
		return process.ErrNilPubkeyConverter
	}
	if check.IfNil(args.AccountsCacher) {
		return process.ErrNilAccountsAdapter
	}
	if check.IfNil(args.KAppsController) {
		return process.ErrNilKAppsController
	}
	if check.IfNil(args.ScProcessor) {
		return process.ErrNilSmartContractProcessor
	}
	if check.IfNil(args.EconomicsFee) {
		return process.ErrNilEconomicsFeeHandler
	}
	if check.IfNil(args.ForkController) {
		return common.ErrNilForkController
	}
	if check.IfNil(args.VMOutputCacher) {
		return common.ErrNilCacher
	}

	return nil
}

// ProcessTransaction modifies the account states in respect with the transaction data
func (txProc *simulateTxProcessor) ProcessTransaction(tx *transaction.Transaction) error {
	if err := txProc.validateAndPrepareTransaction(tx); err != nil {
		return err
	}

	ownerAcc, computedHash, err := txProc.prepareAccountAndHash(tx)
	if err != nil {
		return err
	}

	process.DisplayProcessTxDetails(
		"ProcessTransaction: sender account details",
		ownerAcc,
		tx,
		computedHash,
		txProc.pubkeyConv,
	)

	ctx, err := txProc.createKAppContext(tx, computedHash)
	if err != nil {
		return err
	}

	sw := tools.NewStopWatch()
	sw.Start("txProc")

	if err = txProc.processContracts(ctx, ownerAcc, tx, computedHash, sw); err != nil {
		return err
	}

	sw.Stop("txProc")
	duration := sw.GetMeasurement("txProc")
	logSC.Trace("txProc smart contract", "duration", duration)

	return nil
}

func (txProc *simulateTxProcessor) GetVmOutputs(calculatedHash []byte) (*vmcommon.VMOutput, error) {
	vmOutputI, ok := txProc.vmOutputCacher.Get(calculatedHash)
	if !ok {
		return nil, process.ErrNilVMOutput
	}

	vmOutput, ok := vmOutputI.(*vmcommon.VMOutput)
	if !ok {
		return nil, process.ErrInvalidVMOutput
	}

	return vmOutput, nil
}

func (txProc *simulateTxProcessor) GetTotalConsumedGasByContract(calculatedHash []byte) (*big.Int, error) {
	vmOutput, err := txProc.GetVmOutputs(calculatedHash)
	if err != nil {
		return nil, err
	}

	return vmOutput.ComputeTotalGasConsumed(), nil
}

func (txProc *simulateTxProcessor) validateAndPrepareTransaction(tx *transaction.Transaction) error {
	if check.IfNil(tx) {
		return process.ErrNilTransaction
	}

	txProc.accountsCacher.ResetAll(true)
	tx.GasLimit = txProc.economicsFee.MaxGasLimitPerTX()

	return nil
}

func (txProc *simulateTxProcessor) prepareAccountAndHash(tx *transaction.Transaction) (state.UserAccountHandler, []byte, error) {
	ownerAcc, err := txProc.accountsCacher.LoadUser(tx.GetSender())
	if err != nil {
		tx.ResultCode = transaction.Transaction_LoadAccountError
		return nil, nil, err
	}

	ownerAcc.IncreaseNonce(1)

	computedHash, err := tools.CalculateHash(txProc.marshalizer, txProc.hasher, tx.RawData)
	if err != nil {
		tx.ResultCode = transaction.Transaction_Fail
		log.Error("invalid tx hash", "computedHash", computedHash)
		return nil, nil, err
	}

	return ownerAcc, computedHash, nil
}

func (txProc *simulateTxProcessor) createKAppContext(tx *transaction.Transaction, computedHash []byte) (kapp.KappContext, error) {
	// retrieve last committed block
	headerHandler := txProc.scProcessor.LastBlock()
	blockHeader, ok := headerHandler.(*block.Block)
	if !ok {
		tx.ResultCode = transaction.Transaction_Fail
		return nil, common.ErrNilBlockChain
	}

	return kapp.NewKappContext(kapp.ArgsNewKAppContext{
		OriginalSender: tx.GetSender(),
		ContractID:     -1,
		ContractType:   -1,
		Block:          blockHeader,
		TxHash:         computedHash,
		TX:             tx,
		IsScSimulation: true,
	}), nil
}

func (txProc *simulateTxProcessor) processContracts(ctx kapp.KappContext, ownerAcc state.UserAccountHandler, tx *transaction.Transaction, computedHash []byte, sw *tools.StopWatch) error {
	totalConsumedGas := big.NewInt(0)

	// simulator only runs SmartContracts, and only one contract at a time
	// so if contract count is more than 1, we will return an error
	if len(tx.RawData.Contract) > 1 {
		tx.ResultCode = transaction.Transaction_ContractInvalid
		return process.ErrSmartContractFailMaxContracts
	}

	for i := range tx.RawData.Contract {
		ctx.SetContractID(i)
		txProc.kApps.SetCurrentKAppContext(ctx)

		tc, err := tx.RawData.Contract[ctx.ContractID()].GetSmartContract()
		if err != nil {
			tx.ResultCode = transaction.Transaction_ContractInvalid
			return err
		}

		var consumedGas *big.Int
		var processErr error

		switch tc.GetType() {
		case transaction.SmartContract_SCDeploy:
			consumedGas, processErr = txProc.deploySmartContract(ctx, tc, tx, computedHash, sw)
		case transaction.SmartContract_SCInvoke:
			consumedGas, processErr = txProc.executeSmartContract(ctx, ownerAcc, tc, tx, computedHash, sw)
		default:
			tx.ResultCode = transaction.Transaction_ParameterInvalid
			return common.ErrSmartContractTypeInvalid
		}

		if processErr != nil {
			return processErr
		}

		totalConsumedGas = totalConsumedGas.Add(totalConsumedGas, consumedGas)

		// check if the total consumed gas exceeds max gas limit
		maxGasBigInt := big.NewInt(0).SetUint64(txProc.economicsFee.MaxGasLimitPerTX())
		if totalConsumedGas.Cmp(maxGasBigInt) > 0 {
			tx.ResultCode = transaction.Transaction_VMOutOfGas
			return process.ErrInvalidMaxGasLimitPerTx
		}
	}

	return nil
}

func (txProc *simulateTxProcessor) deploySmartContract(ctx kapp.KappContext, tc *transaction.SmartContract, tx *transaction.Transaction, computedHash []byte, sw *tools.StopWatch) (*big.Int, error) {
	sw.Start("deploy")
	defer sw.Stop("deploy")

	returnCode, err := txProc.scProcessor.DeploySmartContract(ctx, tc)
	if err != nil || returnCode != vmcommon.Ok {
		tx.ResultCode = returnCode.ResultCode()
		return nil, txProc.handleSmartContractError(err, returnCode, "deploying", process.ErrSmartContractDeploymentFailed)
	}

	consumedGas, err := txProc.GetTotalConsumedGasByContract(computedHash)
	if err != nil {
		tx.ResultCode = transaction.Transaction_Fail
		return nil, err
	}

	logSC.Trace("deploy smart contract", "duration", sw.GetMeasurement("deploy"))
	return consumedGas, nil
}

func (txProc *simulateTxProcessor) executeSmartContract(ctx kapp.KappContext, ownerAcc state.UserAccountHandler, tc *transaction.SmartContract, tx *transaction.Transaction, computedHash []byte, sw *tools.StopWatch) (*big.Int, error) {
	sw.Start("execute")
	defer sw.Stop("execute")

	destAcc, err := txProc.accountsCacher.GetExistingUser(tc.GetAddress())
	if err != nil {
		tx.ResultCode = transaction.Transaction_AccountError
		return nil, err
	}

	returnCode, err := txProc.scProcessor.ExecuteSmartContractTransaction(ctx, tc, ownerAcc, destAcc)
	if err != nil || returnCode != vmcommon.Ok {
		tx.ResultCode = returnCode.ResultCode()
		return nil, txProc.handleSmartContractError(err, returnCode, "invoking", process.ErrSmartContractInvokeFailed)
	}

	consumedGas, err := txProc.GetTotalConsumedGasByContract(computedHash)
	if err != nil {
		tx.ResultCode = transaction.Transaction_Fail
		return nil, err
	}

	logSC.Trace("execute smart contract", "duration", sw.GetMeasurement("execute"))
	return consumedGas, nil
}

func (txProc *simulateTxProcessor) handleSmartContractError(err error, returnCode vmcommon.ReturnCode, action string, defaultErr error) error {
	if err == nil {
		err = fmt.Errorf("%w: %s", defaultErr, returnCode.String())
	}
	logSC.Debug(fmt.Sprintf("error %s smart contract", action), "error", err, "returnCode", returnCode)
	return err
}

// IsInterfaceNil returns true if there is no value under the interface
func (txProc *simulateTxProcessor) IsInterfaceNil() bool {
	return txProc == nil
}
