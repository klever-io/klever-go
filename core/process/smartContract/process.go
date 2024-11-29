package smartContract

import (
	"bytes"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/types"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"

	logger "github.com/klever-io/klever-go-logger"
)

var _ process.SmartContractProcessor = (*scProcessor)(nil)

var log = logger.GetOrCreate("process/smartcontract")
var logCounters = logger.GetOrCreate("process/smartcontract.blockchainHookCounters")

const (
	executeDurationAlarmThreshold = time.Duration(100) * time.Millisecond

	upgradeFunctionName      = "upgradeContract"
	transferValueGasSchedule = "Transfer"
)

type scProcessor struct {
	// accounts           state.AccountsAdapter
	blockChainHook     process.BlockChainHookHandler
	pubkeyConv         core.PubkeyConverter
	hasher             hashing.Hasher
	marshalizer        marshal.Marshalizer
	vmContainer        process.VirtualMachinesContainer
	argsParser         process.ArgumentsParser
	kdaTransferParser  vmcommon.KDATransferParser
	builtInFunctions   vmcommon.BuiltInFunctionContainer
	accountsCacher     state.AccountsCacher
	wasmVMChangeLocker common.Locker

	forkController core.ForkController
	txFeeHandler   process.TransactionFeeHandler
	economicsFee   process.EconomicsDataHandler

	builtInGasCosts     map[string]uint64
	persistPerByte      uint64
	storePerByte        uint64
	mutGasLock          sync.RWMutex
	txLogsProcessor     process.TransactionLogProcessor
	vmOutputCacher      storage.Cacher
	isGenesisProcessing bool
}

// ArgsNewSmartContractProcessor defines the arguments needed for new smart contract processor
type ArgsNewSmartContractProcessor struct {
	VmContainer         process.VirtualMachinesContainer
	ArgsParser          process.ArgumentsParser
	Hasher              hashing.Hasher
	Marshalizer         marshal.Marshalizer
	BlockChainHook      process.BlockChainHookHandler
	BuiltInFunctions    vmcommon.BuiltInFunctionContainer
	PubkeyConv          core.PubkeyConverter
	TxFeeHandler        process.TransactionFeeHandler
	EconomicsFee        process.EconomicsDataHandler
	GasSchedule         core.GasScheduleNotifier
	TxLogsProcessor     process.TransactionLogProcessor
	ForkController      core.ForkController
	VMOutputCacher      storage.Cacher
	WasmVMChangeLocker  common.Locker
	AccountsCacher      state.AccountsCacher
	IsGenesisProcessing bool
}

// NewSmartContractProcessor creates a smart contract processor that creates and interprets VM data
func NewSmartContractProcessor(args ArgsNewSmartContractProcessor) (*scProcessor, error) {
	if check.IfNil(args.VmContainer) {
		return nil, process.ErrNoVM
	}
	if check.IfNil(args.ArgsParser) {
		return nil, process.ErrNilArgumentParser
	}
	if check.IfNil(args.Hasher) {
		return nil, process.ErrNilHasher
	}
	if check.IfNil(args.Marshalizer) {
		return nil, process.ErrNilMarshalizer
	}
	if check.IfNil(args.BlockChainHook) {
		return nil, process.ErrNilTemporaryAccountsHandler
	}
	if check.IfNil(args.PubkeyConv) {
		return nil, process.ErrNilPubkeyConverter
	}
	if check.IfNil(args.TxFeeHandler) {
		return nil, process.ErrNilUnsignedTxHandler
	}
	if check.IfNil(args.EconomicsFee) {
		return nil, process.ErrNilEconomicsFeeHandler
	}
	if check.IfNil(args.GasSchedule) || args.GasSchedule.LatestGasSchedule() == nil {
		return nil, process.ErrNilGasSchedule
	}
	if check.IfNil(args.TxLogsProcessor) {
		return nil, process.ErrNilTxLogsProcessor
	}
	if check.IfNil(args.ForkController) {
		return nil, process.ErrNilEnableEpochsHandler
	}
	if check.IfNilReflect(args.WasmVMChangeLocker) {
		return nil, process.ErrNilLocker
	}
	if check.IfNil(args.VMOutputCacher) {
		return nil, process.ErrNilCacher
	}
	if check.IfNil(args.BuiltInFunctions) {
		return nil, process.ErrNilBuiltInFunction
	}
	if check.IfNil(args.AccountsCacher) {
		return nil, process.ErrNilCacher
	}

	builtInFuncCost := args.GasSchedule.LatestGasSchedule()[common.BuiltInCost]
	baseOperationCost := args.GasSchedule.LatestGasSchedule()[common.BaseOperationCost]
	sc := &scProcessor{
		vmContainer:         args.VmContainer,
		argsParser:          args.ArgsParser,
		hasher:              args.Hasher,
		marshalizer:         args.Marshalizer,
		blockChainHook:      args.BlockChainHook,
		pubkeyConv:          args.PubkeyConv,
		txFeeHandler:        args.TxFeeHandler,
		economicsFee:        args.EconomicsFee,
		builtInGasCosts:     builtInFuncCost,
		txLogsProcessor:     args.TxLogsProcessor,
		forkController:      args.ForkController,
		builtInFunctions:    args.BuiltInFunctions,
		isGenesisProcessing: args.IsGenesisProcessing,
		wasmVMChangeLocker:  args.WasmVMChangeLocker,
		vmOutputCacher:      args.VMOutputCacher,
		accountsCacher:      args.AccountsCacher,
		storePerByte:        baseOperationCost["StorePerByte"],
		persistPerByte:      baseOperationCost["PersistPerByte"],
	}

	var err error
	sc.kdaTransferParser, err = parsers.NewKDATransferParser(args.Marshalizer)
	if err != nil {
		return nil, err
	}

	args.GasSchedule.RegisterNotifyHandler(sc)

	return sc, nil
}

// GasScheduleChange sets the new gas schedule where it is needed
// Warning: do not use flags in this function as it will raise backward compatibility issues because the GasScheduleChange
// is not called on each epoch change
func (sc *scProcessor) GasScheduleChange(gasSchedule map[string]map[string]uint64) {
	sc.mutGasLock.Lock()
	defer sc.mutGasLock.Unlock()

	builtInFuncCost := gasSchedule[common.BuiltInCost]
	if builtInFuncCost == nil {
		return
	}

	sc.builtInGasCosts = builtInFuncCost
	sc.storePerByte = gasSchedule[common.BaseOperationCost]["StorePerByte"]
	sc.persistPerByte = gasSchedule[common.BaseOperationCost]["PersistPerByte"]
}

func (sc *scProcessor) checkTxValidity(tc data.SmartContractHandler) error {
	if check.IfNil(tc) {
		return process.ErrNilTransaction
	}

	// if deploying, new contract, address should be zero
	var recvAddressIsInvalid bool
	switch tc.GetType() {
	case transaction.SmartContract_SCDeploy:
		recvAddressIsInvalid = len(tc.GetAddress()) != 0
	default:
		recvAddressIsInvalid = sc.pubkeyConv.Len() != len(tc.GetAddress())
	}

	if recvAddressIsInvalid {
		return process.ErrInvalidRcvAddr
	}

	return nil
}

// ExecuteSmartContractTransaction processes the transaction, call the VM and processes the SC call output
func (sc *scProcessor) ExecuteSmartContractTransaction(
	ctx kapp.KappContext,
	tc data.SmartContractHandler,
	acntSnd, acntDst state.UserAccountHandler,
) (vmcommon.ReturnCode, error) {
	if !sc.forkController.EnableSmartContracts() {
		return vmcommon.VMContractInvalid, common.ErrInvalidContract
	}

	err := sc.checkTxValidity(tc)
	if err != nil {
		log.Debug("invalid transaction", "error", err.Error())
		return vmcommon.VMContractInvalid, err
	}

	sw := tools.NewStopWatch()
	sw.Start("execute")
	returnCode, err := sc.doExecuteSmartContractTransaction(ctx, tc, acntSnd, acntDst)
	sw.Stop("execute")
	duration := sw.GetMeasurement("execute")

	if duration > executeDurationAlarmThreshold {
		log.Debug(fmt.Sprintf(
			"scProcessor.ExecuteSmartContractTransaction(): execution took > %s", executeDurationAlarmThreshold),
			"tx hash",
			ctx.TxHash(),
			"sc",
			tc.GetAddress(),
			"duration",
			duration,
			"returnCode",
			returnCode,
			"err", err,
			"data",
			string(ctx.GetExecData()),
		)
	} else {
		log.Trace(
			"scProcessor.ExecuteSmartContractTransaction()",
			"sc",
			tc.GetAddress(),
			"duration",
			duration,
			"returnCode",
			returnCode,
			"err",
			err,
			"data",
			string(ctx.GetExecData()),
		)
	}

	return returnCode, err
}

func (sc *scProcessor) prepareExecution(
	ctx kapp.KappContext,
	tc data.SmartContractHandler,
	acntSnd, acntDst state.UserAccountHandler,
) (vmcommon.ReturnCode, *vmcommon.ContractCallInput, error) {
	if check.IfNil(acntDst) {
		return vmcommon.VMUserError, nil, process.ErrNilSCDestAccount
	}

	gasLimit := ctx.GetGasLimit()
	// transaction process transfer funds cost
	gasLimit, err := sc.consumeKDATransferGas(gasLimit, tc)
	if err != nil {
		return vmcommon.VMUserError, nil, process.ErrNotEnoughGas
	}

	err = sc.processSCPayment(tc, acntSnd)
	if err != nil {
		log.Debug("process sc payment error", "error", err.Error())
		return vmcommon.VMContractInvalid, nil, err
	}

	var vmInput *vmcommon.ContractCallInput
	vmInput, err = sc.createVMCallInput(ctx, gasLimit, tc.GetAddress(), tc.GetCallValue())
	if err != nil {
		returnMessage := "cannot create VMInput, check the transaction data field"
		log.Debug("create vm call input error", "error", err.Error())
		return vmcommon.VMUserError, vmInput, sc.ProcessIfError(ctx, tc, err.Error(), []byte(returnMessage))
	}

	err = sc.checkUpgradePermission(acntDst, vmInput)
	if err != nil {
		log.Debug("checkUpgradePermission", "error", err.Error())
		return vmcommon.VMUserError, vmInput, sc.ProcessIfError(ctx, tc, err.Error(), []byte(err.Error()))
	}

	return vmcommon.Ok, vmInput, nil
}

func (sc *scProcessor) doExecuteSmartContractTransaction(
	ctx kapp.KappContext,
	tc data.SmartContractHandler,
	acntSnd, acntDst state.UserAccountHandler,
) (vmcommon.ReturnCode, error) {
	returnCode, vmInput, err := sc.prepareExecution(ctx, tc, acntSnd, acntDst)
	if err != nil || returnCode != vmcommon.Ok {
		return returnCode, err
	}

	vmOutput, err := sc.executeSmartContractCall(ctx, vmInput, tc, nil)
	if err != nil {
		return vmOutput.ReturnCode, err
	}
	if vmOutput.ReturnCode != vmcommon.Ok {
		return vmOutput.ReturnCode, nil
	}

	err = sc.processVMOutput(ctx, vmOutput)
	if err != nil {
		log.Trace("process vm output returned with problem ", "err", err.Error())
		return vmcommon.VMExecutionFailed, sc.ProcessIfError(ctx, tc, err.Error(), []byte(vmOutput.ReturnMessage))
	}

	return sc.finishSCExecution(ctx, tc, vmOutput)
}

func (sc *scProcessor) executeSmartContractCall(
	ctx kapp.KappContext,
	vmInput *vmcommon.ContractCallInput,
	tc data.SmartContractHandler,
	prevVmOutput *vmcommon.VMOutput,
) (*vmcommon.VMOutput, error) {
	userErrorVmOutput := &vmcommon.VMOutput{
		ReturnCode: vmcommon.VMUserError,
	}

	sc.wasmVMChangeLocker.RLock()
	vmExec, err := FindVMByScAddress(sc.vmContainer, vmInput.RecipientAddr)
	if err != nil {
		sc.wasmVMChangeLocker.RUnlock()
		returnMessage := "cannot get vm from address"
		log.Trace("get vm from address error", "error", err.Error())
		return userErrorVmOutput, sc.ProcessIfError(ctx, tc, err.Error(), []byte(returnMessage))
	}

	sc.blockChainHook.ResetCounters()
	defer sc.printBlockchainHookCounters(ctx, tc)

	var vmOutput *vmcommon.VMOutput
	vmOutput, err = vmExec.RunSmartContractCall(vmInput)
	sc.wasmVMChangeLocker.RUnlock()
	if err != nil {
		log.Debug("run smart contract call error", "error", err.Error())
		return userErrorVmOutput, sc.ProcessIfError(ctx, tc, err.Error(), []byte(""))
	}
	if vmOutput == nil {
		err = process.ErrNilVMOutput
		log.Debug("run smart contract call error", "error", err.Error())
		return userErrorVmOutput, sc.ProcessIfError(ctx, tc, err.Error(), []byte(""))
	}

	if vmOutput.ReturnCode != vmcommon.Ok {
		return userErrorVmOutput, sc.processIfErrorWithAddedLogs(ctx, tc, vmOutput.ReturnCode.String(), []byte(vmOutput.ReturnMessage), prevVmOutput, vmOutput.Logs)
	}

	return vmOutput, nil
}

func (sc *scProcessor) printBlockchainHookCounters(ctx kapp.KappContext, tc data.SmartContractHandler) {
	if logCounters.GetLevel() > logger.LogTrace {
		return
	}

	logCounters.Trace("blockchain hook counters",
		"counters", sc.getBlockchainHookCountersString(),
		"tx hash", ctx.TxHash(),
		"sender", sc.pubkeyConv.Encode(ctx.OriginalSender()),
		"value", tc.GetCallValue(),
		"data", ctx.GetExecData(),
	)
}

func (sc *scProcessor) getBlockchainHookCountersString() string {
	counters := sc.blockChainHook.GetCounterValues()
	keys := make([]string, len(counters))

	idx := 0
	for key := range counters {
		keys[idx] = key
		idx++
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	lines := make([]string, 0, len(counters))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s: %d", key, counters[key]))
	}

	return strings.Join(lines, ", ")
}

func (sc *scProcessor) finishSCExecution(
	ctx kapp.KappContext,
	tc data.SmartContractHandler,
	vmOutput *vmcommon.VMOutput,
) (vmcommon.ReturnCode, error) {
	txHash := ctx.TxHash()
	totalConsumedGas := sc.computeTotalConsumedGas(ctx.GetGasLimit(), vmOutput)

	logEntries := []*vmcommon.LogEntry{
		{
			Address:     tc.GetAddress(),
			Identifier:  []byte(core.ReturnDataString),
			Topics:      [][]byte{[]byte(vmOutput.ReturnMessage)},
			Data:        vmOutput.ReturnData,
			IsSystemLog: true,
		},
		{
			Address:     tc.GetAddress(),
			Identifier:  []byte(core.TotalConsumedGasString),
			Topics:      [][]byte{totalConsumedGas.Bytes()},
			IsSystemLog: true,
		},
	}
	vmOutput.Logs = append(vmOutput.Logs, logEntries...)
	// sub consumed gas from context
	err := ctx.SubGasUsed(totalConsumedGas.Uint64())
	if err != nil {
		log.Error("subGasUsed", "error", err.Error())
		return vmcommon.VMExecutionFailed, err
	}

	completedTxLog := sc.createCompleteEventLogIfNoMoreAction(tc, txHash)
	if completedTxLog != nil {
		vmOutput.Logs = append(vmOutput.Logs, completedTxLog)
	}

	ignorableError := sc.txLogsProcessor.SaveLog(txHash, ctx.OriginalSender(), tc, ctx.ContractID(), vmOutput.Logs)
	if ignorableError != nil {
		log.Debug("scProcessor.finishSCExecution txLogsProcessor.SaveLog()", "error", ignorableError.Error())
	}

	sc.vmOutputCacher.Put(txHash, vmOutput, 0)

	return vmcommon.Ok, nil
}

func (sc *scProcessor) computeTotalConsumedGas(
	gasLimit uint64,
	vmOutput *vmcommon.VMOutput,
) *big.Int {
	if gasLimit == 0 {
		return big.NewInt(0)
	}

	consumedGas, err := tools.SafeSubUint64(gasLimit, vmOutput.GasRemaining)
	log.LogIfError(err, "computeTotalConsumedGas", "vmOutput.GasRemaining")

	log.Debug("computeTotalConsumedGas", "consumedGas", consumedGas, "gasLimit", gasLimit)

	return big.NewInt(0).SetUint64(consumedGas)
}

// ProcessIfError creates a smart contract result, consumes the gas and returns the value to the user
func (sc *scProcessor) ProcessIfError(
	ctx kapp.KappContext,
	tc data.SmartContractHandler,
	returnCode string,
	returnMessage []byte,
) error {
	return sc.processIfErrorWithAddedLogs(ctx, tc, returnCode, returnMessage, nil, nil)
}

func (sc *scProcessor) processIfErrorWithAddedLogs(
	ctx kapp.KappContext,
	tc data.SmartContractHandler,
	returnCode string,
	returnMessage []byte,
	prevVmOutput *vmcommon.VMOutput,
	internalVMLogs []*vmcommon.LogEntry,
) error {
	txHash := ctx.TxHash()
	sc.vmOutputCacher.Put(txHash, &vmcommon.VMOutput{
		ReturnCode:    vmcommon.VMSimulateFailed,
		ReturnMessage: string(returnMessage),
	}, 0)

	returnMessage = []byte(returnCode)

	consumedFee := big.NewInt(0).SetUint64(ctx.GetGasLimit())
	// sub consumed gas from context
	err := ctx.SubGasUsed(consumedFee.Uint64())
	if err != nil {
		log.Error("subGasUsed", "error", err.Error())
		return err
	}

	processIfErrorLogs := make([]*vmcommon.LogEntry, 0)
	if prevVmOutput != nil && len(prevVmOutput.Logs) > 0 {
		processIfErrorLogs = append(processIfErrorLogs, prevVmOutput.Logs...)
	}

	if len(internalVMLogs) > 0 {
		processIfErrorLogs = append(processIfErrorLogs, internalVMLogs...)
	}

	ignorableError := sc.txLogsProcessor.SaveLog(txHash, ctx.OriginalSender(), tc, ctx.ContractID(), processIfErrorLogs)
	if ignorableError != nil {
		log.Debug("scProcessor.ProcessIfError() txLogsProcessor.SaveLog()", "error", ignorableError.Error())
	}

	log.Debug("processIfErrorWithAddedLogs", "totalConsumedFee", consumedFee)

	return fmt.Errorf("%s", returnMessage)
}

// DeploySmartContract processes the transaction, then deploy the smart contract into VM, final code is saved in account
func (sc *scProcessor) DeploySmartContract(ctx kapp.KappContext, tc data.SmartContractHandler) (vmcommon.ReturnCode, error) {
	if !sc.forkController.EnableSmartContracts() {
		return vmcommon.VMContractInvalid, common.ErrInvalidContract
	}

	err := sc.checkTxValidity(tc)
	if err != nil {
		log.Debug("invalid transaction", "error", err.Error())
		return vmcommon.VMContractInvalid, err
	}

	sw := tools.NewStopWatch()
	sw.Start("deploy")
	returnCode, err := sc.doDeploySmartContract(ctx, tc)
	sw.Stop("deploy")
	duration := sw.GetMeasurement("deploy")

	if duration > executeDurationAlarmThreshold {
		log.Debug(fmt.Sprintf("scProcessor.DeploySmartContract(): execution took > %s", executeDurationAlarmThreshold), "tx hash", ctx.TxHash(), "sc", tc.GetAddress(), "duration", duration, "returnCode", returnCode, "err", err)
	} else {
		log.Trace("scProcessor.DeploySmartContract()", "duration", duration, "returnCode", returnCode, "err", err)
	}

	return returnCode, err
}

func (sc *scProcessor) doDeploySmartContract(
	ctx kapp.KappContext,
	tc data.SmartContractHandler,
) (vmcommon.ReturnCode, error) {
	sc.blockChainHook.ResetCounters()
	defer sc.printBlockchainHookCounters(ctx, tc)

	if len(tc.GetAddress()) != 0 {
		log.Debug("wrong transaction - not empty address", "error", process.ErrWrongTransaction.Error())
		return vmcommon.VMContractInvalid, process.ErrWrongTransaction
	}

	txHash := ctx.TxHash()

	var vmOutput *vmcommon.VMOutput
	shouldAllowDeploy := sc.isGenesisProcessing
	if !shouldAllowDeploy {
		log.Trace("deploy is disabled")
		return vmcommon.VMUserError, sc.ProcessIfError(ctx, tc, process.ErrSmartContractDeploymentIsDisabled.Error(), []byte(""))
	}

	gasLimit := ctx.GetGasLimit()
	// transaction process transfer funds cost
	gasLimit, err := sc.consumeKDATransferGas(gasLimit, tc)
	if err != nil {
		return vmcommon.VMUserError, process.ErrNotEnoughGas
	}

	vmInput, vmType, err := sc.createVMDeployInput(ctx, gasLimit, tc.GetCallValue())
	if err != nil {
		log.Trace("Transaction data invalid", "error", err.Error())
		return vmcommon.VMUserError, sc.ProcessIfError(ctx, tc, err.Error(), []byte(""))
	}

	sc.wasmVMChangeLocker.RLock()
	vmExec, err := sc.vmContainer.Get(vmType)
	if err != nil {
		sc.wasmVMChangeLocker.RUnlock()
		log.Trace("VM not found", "error", err.Error())
		return vmcommon.VMUserError, sc.ProcessIfError(ctx, tc, err.Error(), []byte(""))
	}

	vmOutput, err = vmExec.RunSmartContractCreate(vmInput)
	sc.wasmVMChangeLocker.RUnlock()
	if err != nil {
		log.Debug("VM error", "error", err.Error())
		return vmcommon.VMUserError, sc.ProcessIfError(ctx, tc, err.Error(), []byte(""))
	}

	if vmOutput == nil {
		err = process.ErrNilVMOutput
		log.Trace("run smart contract create", "error", err.Error())
		return vmcommon.VMUserError, sc.ProcessIfError(ctx, tc, err.Error(), []byte(""))
	}

	if vmOutput.ReturnCode != vmcommon.Ok {
		return vmcommon.VMUserError, sc.processIfErrorWithAddedLogs(ctx, tc, vmOutput.ReturnCode.String(), []byte(vmOutput.ReturnMessage), nil, vmOutput.Logs)
	}

	err = sc.processVMOutput(ctx, vmOutput)
	if err != nil {
		log.Trace("Processing error", "error", err.Error())
		return vmcommon.VMExecutionFailed, sc.ProcessIfError(ctx, tc, err.Error(), []byte(vmOutput.ReturnMessage))
	}

	totalConsumedGas := sc.computeTotalConsumedGas(ctx.GetGasLimit(), vmOutput)
	log.Info("DeploySmartContract", "totalConsumedGas", totalConsumedGas)
	deployedSC := sc.printScDeployed(ctx.OriginalSender(), vmOutput)

	logEntries := []*vmcommon.LogEntry{
		{
			Address:     ctx.OriginalSender(),
			Identifier:  []byte(core.ReturnDataString),
			Topics:      [][]byte{[]byte(vmOutput.ReturnMessage)},
			Data:        vmOutput.ReturnData,
			IsSystemLog: true,
		},
		{
			Address:     ctx.OriginalSender(),
			Identifier:  []byte(core.TotalConsumedGasString),
			Topics:      [][]byte{totalConsumedGas.Bytes()},
			IsSystemLog: true,
		},
	}
	vmOutput.Logs = append(vmOutput.Logs, logEntries...)

	// sub consumed gas from context
	err = ctx.SubGasUsed(totalConsumedGas.Uint64())
	if err != nil {
		log.Error("subGasUsed", "error", err.Error())
		return vmcommon.VMUserError, sc.ProcessIfError(ctx, tc, err.Error(), []byte(""))
	}

	// add receipt for deploy
	for _, addr := range deployedSC {
		ctx.Receipts().Add(txProcess.NewReceipt(
			txProcess.SCTrigger,
			ctx.ContractID(),
			[]byte(strconv.FormatInt(int64(tc.GetType()), 10)),
			ctx.OriginalSender(),
			addr,
		))
	}

	sc.vmOutputCacher.Put(txHash, vmOutput, 0)

	ignorableError := sc.txLogsProcessor.SaveLog(txHash, ctx.OriginalSender(), tc, ctx.ContractID(), vmOutput.Logs)
	if ignorableError != nil {
		log.Debug("scProcessor.DeploySmartContract() txLogsProcessor.SaveLog()", "error", ignorableError.Error())
	}

	return 0, nil
}

func (sc *scProcessor) printScDeployed(sender []byte, vmOutput *vmcommon.VMOutput) [][]byte {
	deployedSC := make([][]byte, 0)
	scGenerated := make([]string, 0, len(vmOutput.OutputAccounts))
	for _, account := range vmOutput.OutputAccounts {
		if account == nil {
			continue
		}

		addr := account.Address
		if !core.IsSmartContractAddress(addr) {
			continue
		}

		deployedSC = append(deployedSC, addr)
		scGenerated = append(scGenerated, sc.pubkeyConv.Encode(addr))
	}

	log.Debug("SmartContract deployed",
		"owner", sc.pubkeyConv.Encode(sender),
		"SC address(es)", strings.Join(scGenerated, ", "))

	return deployedSC
}

// deposit call values into contract account
func (sc *scProcessor) processSCPayment(tc data.SmartContractHandler, acntSnd state.UserAccountHandler) error {
	accKapp := sc.blockChainHook.GetKAppController().GetAccountsKApp()

	dMap := types.NewDeterministicMap(tc.GetCallValue())
	// sub from sender the call value
	return dMap.Each(func(assetID string, cvwr *transaction.CallValue) error {
		// execute transfer without royalties as it will be deducted from sender account
		resultCode, err := accKapp.Transfer(transaction.TXContract_TransferContractType, acntSnd.AddressBytes(), &transaction.TransferContract{
			ToAddress:    tc.GetAddress(),
			Amount:       cvwr.Amount,
			AssetID:      []byte(assetID),
			KDARoyalties: cvwr.KDARoyalties,
			KLVRoyalties: cvwr.KLVRoyalties,
		})
		if err != nil {
			return fmt.Errorf("result code: %d, %v", resultCode, err)
		}
		return nil
	})
}

func (sc *scProcessor) processVMOutput(
	ctx kapp.KappContext,
	vmOutput *vmcommon.VMOutput,
) error {
	outPutAccounts := process.SortVMOutputInsideData(vmOutput)
	err := sc.processSCOutputAccounts(ctx, vmOutput, outPutAccounts)
	if err != nil {
		return err
	}

	err = sc.deleteAccounts(vmOutput.DeletedAccounts)
	if err != nil {
		return err
	}

	return nil
}

// save account changes in state from vmOutput - protected by VM - every output can be treated as is.
func (sc *scProcessor) processSCOutputAccounts(
	ctx kapp.KappContext,
	vmOutput *vmcommon.VMOutput,
	outputAccounts []*vmcommon.OutputAccount,
) error {
	for _, outAcc := range outputAccounts {
		acc, err := sc.getAccountFromAddress(outAcc.Address)
		if err != nil {
			return err
		}

		if !ctx.IsScSimulation() {
			// check if keyValue storage is updating in cacher or writing to AccOutputs...
			// If saved in cacher, then no need to update states here
			for _, storeUpdate := range outAcc.StorageUpdates {
				// BUG: Validate that all user keys are updated with PREFIX
				err = acc.SaveKeyValue(storeUpdate.Offset, storeUpdate.Data)
				if err != nil {
					log.Warn("saveKeyValue", "error", err)
					return err
				}
				log.Trace("storeUpdate", "acc", outAcc.Address, "key", storeUpdate.Offset, "data", storeUpdate.Data)
			}
		}

		err = sc.updateSmartContractCode(vmOutput, acc, outAcc)
		if err != nil {
			return err
		}

	}

	return nil
}

// updateSmartContractCode upgrades code for "direct" deployments & upgrades and for "indirect" deployments & upgrades
// It receives:
//
//	(1) the account as found in the State
//	(2) the account as returned in VM Output
//	(3) the transaction that, upon execution, produced the VM Output
func (sc *scProcessor) updateSmartContractCode(
	vmOutput *vmcommon.VMOutput,
	stateAccount state.UserAccountHandler,
	outputAccount *vmcommon.OutputAccount,
) error {
	if len(outputAccount.Code) == 0 {
		return nil
	}
	if len(outputAccount.CodeMetadata) == 0 {
		return nil
	}
	if !core.IsSmartContractAddress(outputAccount.Address) {
		return nil
	}

	outputAccountCodeMetadataBytes, err := sc.blockChainHook.FilterCodeMetadataForUpgrade(outputAccount.CodeMetadata)
	if err != nil {
		return err
	}

	// This check is desirable (not required though) since currently both Wasm VM and IELE send the code in the output account even for "regular" execution
	sameCode := bytes.Equal(outputAccount.Code, sc.accountsCacher.GetCode(stateAccount.GetCodeHash()))
	sameCodeMetadata := bytes.Equal(outputAccountCodeMetadataBytes, stateAccount.GetCodeMetadata())
	if sameCode && sameCodeMetadata {
		return nil
	}

	currentOwner := stateAccount.GetOwnerAddress()
	isCodeDeployerSet := len(outputAccount.CodeDeployerAddress) > 0
	isCodeDeployerOwner := bytes.Equal(currentOwner, outputAccount.CodeDeployerAddress) && isCodeDeployerSet

	noExistingCode := len(sc.accountsCacher.GetCode(stateAccount.GetCodeHash())) == 0
	noExistingOwner := len(currentOwner) == 0
	currentCodeMetadata := vmcommon.CodeMetadataFromBytes(stateAccount.GetCodeMetadata())
	newCodeMetadata := vmcommon.CodeMetadataFromBytes(outputAccountCodeMetadataBytes)
	isUpgradeable := currentCodeMetadata.Upgradeable
	isDeployment := noExistingCode && noExistingOwner
	isUpgrade := !isDeployment && isCodeDeployerOwner && isUpgradeable

	entry := &vmcommon.LogEntry{
		Address: stateAccount.AddressBytes(),
		Topics: [][]byte{
			outputAccount.Address, outputAccount.CodeDeployerAddress,
		},
		IsSystemLog: true,
	}

	if isDeployment {
		// At this point, we are under the condition "noExistingOwner"
		stateAccount.SetOwnerAddress(outputAccount.CodeDeployerAddress)
		stateAccount.SetCodeMetadata(outputAccountCodeMetadataBytes)
		stateAccount.SetCode(outputAccount.Code)

		log.Debug("updateSmartContractCode(): created", "address", sc.pubkeyConv.Encode(outputAccount.Address), "upgradeable", newCodeMetadata.Upgradeable)

		entry.Identifier = []byte(core.SCDeployIdentifier)
		vmOutput.Logs = append(vmOutput.Logs, entry)
		return nil
	}

	if isUpgrade {
		stateAccount.SetCodeMetadata(outputAccountCodeMetadataBytes)
		stateAccount.SetCode(outputAccount.Code)
		log.Debug("updateSmartContractCode(): upgraded", "address", sc.pubkeyConv.Encode(outputAccount.Address), "upgradeable", newCodeMetadata.Upgradeable)

		entry.Identifier = []byte(core.SCUpgradeIdentifier)
		vmOutput.Logs = append(vmOutput.Logs, entry)
		return nil
	}

	return nil
}

// delete accounts - only suicide by current SC or another SC called by current SC - protected by VM
func (sc *scProcessor) deleteAccounts(deletedAccounts [][]byte) error {
	for _, value := range deletedAccounts {
		acc, err := sc.accountsCacher.GetExistingUser(value)
		if err != nil {
			return err
		}

		// delete account code and trie
		err = sc.accountsCacher.RemoveCode(acc.AddressBytes())
		if err != nil {
			return err
		}
	}

	return nil
}

func (sc *scProcessor) getAccountFromAddress(address []byte) (state.UserAccountHandler, error) {
	acnt, err := sc.accountsCacher.LoadUser(address)
	if err != nil {
		return nil, err
	}

	return acnt, nil
}

func (sc *scProcessor) checkUpgradePermission(contract state.UserAccountHandler, vmInput *vmcommon.ContractCallInput) error {
	isUpgradeCalled := vmInput.Function == upgradeFunctionName
	if !isUpgradeCalled {
		return nil
	}
	if check.IfNil(contract) {
		return process.ErrUpgradeNotAllowed
	}

	codeMetadata := vmcommon.CodeMetadataFromBytes(contract.GetCodeMetadata())
	isUpgradeable := codeMetadata.Upgradeable
	callerAddress := vmInput.CallerAddr
	ownerAddress := contract.GetOwnerAddress()
	isCallerOwner := bytes.Equal(callerAddress, ownerAddress)

	if isUpgradeable && isCallerOwner {
		return nil
	}

	return process.ErrUpgradeNotAllowed
}

func (sc *scProcessor) createCompleteEventLogIfNoMoreAction(
	tc data.SmartContractHandler,
	txHash []byte,
) *vmcommon.LogEntry {
	return createCompleteEventLog(tc, txHash)
}

func (sc *scProcessor) consumeKDATransferGas(providedGas uint64, tc data.SmartContractHandler) (uint64, error) {
	count := len(tc.GetCallValue())
	if count == 0 {
		return providedGas, nil
	}

	sc.mutGasLock.RLock()
	gasCostPerTransfer := sc.builtInGasCosts[transferValueGasSchedule]
	sc.mutGasLock.RUnlock()

	gasCost := gasCostPerTransfer * uint64(count)
	return tools.SafeSubUint64(providedGas, gasCost)
}

func createCompleteEventLog(tc data.SmartContractHandler, txHash []byte) *vmcommon.LogEntry {
	newLog := &vmcommon.LogEntry{
		Identifier:  []byte(core.CompletedTxEventIdentifier),
		Address:     tc.GetAddress(),
		Topics:      [][]byte{txHash},
		IsSystemLog: true,
	}

	return newLog
}

// IsPayable returns if address is payable, smart contract ca set to false
func (sc *scProcessor) IsPayable(sndAddress []byte, recvAddress []byte) (bool, error) {
	return sc.blockChainHook.IsPayable(sndAddress, recvAddress)
}

// LastBlock returns the last committed block
func (sc *scProcessor) LastBlock() data.HeaderHandler {
	return sc.blockChainHook.LastBlock()
}

// IsInterfaceNil returns true if there is no value under the interface
func (sc *scProcessor) IsInterfaceNil() bool {
	return sc == nil
}
