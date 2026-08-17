package hostCore

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/kvm/math"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/kvm/vmhost/contexts"
	"github.com/klever-io/klever-go/kvm/vmhost/vmhooks"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

func (host *vmHost) doRunSmartContractCreate(input *vmcommon.ContractCreateInput) *vmcommon.VMOutput {
	host.InitState()
	defer func() {
		errs := host.GetRuntimeErrors()
		if errs != nil {
			log.Trace("doRunSmartContractCreate full error list", "error", errs)
		}
	}()

	_, blockchain, metering, output, runtime, storage := host.GetContexts()

	var vmOutput *vmcommon.VMOutput
	defer func() {
		if vmOutput == nil || vmOutput.ReturnCode == vmcommon.VMExecutionFailed {
			runtime.CleanInstance()
		}
	}()

	address, err := blockchain.NewAddress(input.CallerAddr)
	if err != nil {
		vmOutput = output.CreateVMOutputInCaseOfError(err)
		return vmOutput
	}

	contractCallInput := &vmcommon.ContractCallInput{
		VMInput:       input.VMInput,
		RecipientAddr: address,
		Function:      vmhost.InitFunctionName,
	}
	runtime.SetVMInput(contractCallInput)
	runtime.SetCodeAddress(address)
	metering.InitStateFromContractCallInput(&input.VMInput)
	storage.SetAddress(runtime.GetContextAddress())

	codeDeployInput := vmhost.CodeDeployInput{
		ContractCode:         input.ContractCode,
		ContractCodeMetadata: input.ContractCodeMetadata,
		ContractAddress:      address,
		CodeDeployerAddress:  input.CallerAddr,
	}

	// perform transfer value from sender to new contract
	// execution already ordered by the caller
	for _, kda := range input.KDATransfers {
		if kda.IsExecuted() {
			log.Warn("doRunSmartContractCreate skip executed transfers", "assetID", string(kda.KDATokenName), "amount", kda.KDAValue.Int64())
			continue
		}

		// Reject oversized/invalid transfer values instead of letting
		// Int64() silently narrow them: otherwise a value such as 2^64+500 would wrap to a small
		// amount on the ledger while the original oversized value still reaches the new contract as
		// call value, fabricating unpaid call value on the deploy path.
		amount, err := host.validatedCreateTransferValue(kda.KDAValue)
		if err != nil {
			log.Trace("doRunSmartContractCreate invalid transfer value", "error", err,
				"assetID", string(kda.KDATokenName), "value", kda.KDAValue)
			vmOutput = output.CreateVMOutputInCaseOfError(err)
			return vmOutput
		}

		accKapp := blockchain.GetKAppController().GetAccountsKApp()
		resultCode, err := accKapp.Transfer(transaction.TXContract_SmartContractType, input.CallerAddr, &transaction.TransferContract{
			ToAddress:    address,
			AssetID:      kda.KDATokenName,
			Amount:       amount,
			KDARoyalties: kda.KDARoyalties,
			KLVRoyalties: kda.KLVRoyalties,
		})
		if err == nil && resultCode != transaction.Transaction_Ok {
			err = fmt.Errorf("transfer failed with result code: %d", resultCode)
		}
		if err != nil {
			log.Trace("doRunSmartContractCreate during transfer", "error", err,
				"assetID", string(kda.KDATokenName), "amount", kda.KDAValue.Int64(),
				"kdaRoyalties", kda.KDARoyalties, "klvRoyalties", kda.KLVRoyalties)
			vmOutput = output.CreateVMOutputInCaseOfError(err)
			return vmOutput
		}

		// mark as executed
		kda.SetExecuted()
	}

	vmOutput, err = host.performCodeDeploymentAtContractCreate(codeDeployInput)
	if err != nil {
		log.Trace("doRunSmartContractCreate", "error", err)
		vmOutput = output.CreateVMOutputInCaseOfError(err)
		return vmOutput
	}

	log.Trace("doRunSmartContractCreate",
		"retCode", vmOutput.ReturnCode,
		"message", vmOutput.ReturnMessage,
		"data", vmOutput.ReturnData)

	return vmOutput
}

func (host *vmHost) validatedCreateTransferValue(value *big.Int) (int64, error) {
	if host.ForkController().FixAuditChangesV3() {
		if value == nil || value.Sign() < 0 || !value.IsInt64() {
			return 0, common.ErrInvalidValue
		}
		return value.Int64(), nil
	}

	if value == nil {
		return 0, nil
	}
	return value.Int64(), nil
}

func (host *vmHost) performCodeDeployment(input vmhost.CodeDeployInput, initFunction func() error) (*vmcommon.VMOutput, error) {
	log.Trace("performCodeDeployment", "address", input.ContractAddress, "len(code)", len(input.ContractCode), "metadata", input.ContractCodeMetadata)

	_, _, metering, output, runtime, _ := host.GetContexts()
	err := metering.DeductInitialGasForDirectDeployment(input)
	if err != nil {
		output.SetReturnCode(vmcommon.VMOutOfGas)
		return nil, err
	}

	runtime.MustVerifyNextContractCode()

	err = runtime.StartWasmerInstance(input.ContractCode, metering.GetGasForExecution(), true)
	if err != nil {
		log.Trace("performCodeDeployment/StartWasmerInstance", "err", err)
		if host.ForkController().FixAuditChangesV4() && errors.Is(err, vmhost.ErrContractInvalid) {
			return nil, err
		}
		return nil, vmhost.ErrContractInvalid
	}

	err = initFunction()
	if err != nil {
		return nil, err
	}

	output.DeployCode(input)
	output.RemoveNonUpdatedStorage()

	vmOutput := output.GetVMOutput()
	return vmOutput, nil
}

func (host *vmHost) performCodeDeploymentAtContractCreate(input vmhost.CodeDeployInput) (*vmcommon.VMOutput, error) {
	return host.performCodeDeployment(input, host.callInitFunction)
}

func (host *vmHost) performCodeDeploymentAtContractUpgrade(input vmhost.CodeDeployInput) (*vmcommon.VMOutput, error) {
	return host.performCodeDeployment(input, host.callUpgradeFunction)
}

// doRunSmartContractUpgrade upgrades a contract directly
func (host *vmHost) doRunSmartContractUpgrade(input *vmcommon.ContractCallInput) *vmcommon.VMOutput {
	host.InitState()
	defer func() {
		errs := host.GetRuntimeErrors()
		if errs != nil {
			log.Trace("doRunSmartContractUpgrade full error list", "error", errs)
		}
	}()

	_, _, metering, output, runtime, storage := host.GetContexts()

	var vmOutput *vmcommon.VMOutput
	defer func() {
		if vmOutput == nil || vmOutput.ReturnCode == vmcommon.VMExecutionFailed {
			runtime.CleanInstance()
		}
	}()

	err := host.checkUpgradePermission(input)
	if err != nil {
		log.Trace("doRunSmartContractUpgrade", "error", vmhost.ErrUpgradeNotAllowed)
		vmOutput = output.CreateVMOutputInCaseOfError(err)
		return vmOutput
	}

	runtime.InitStateFromContractCallInput(input)
	metering.InitStateFromContractCallInput(&input.VMInput)
	storage.SetAddress(runtime.GetContextAddress())

	code, codeMetadata, err := runtime.ExtractCodeUpgradeFromArgs()
	if err != nil {
		vmOutput = output.CreateVMOutputInCaseOfError(vmhost.ErrInvalidUpgradeArguments)
		return vmOutput
	}

	codeDeployInput := vmhost.CodeDeployInput{
		ContractCode:         code,
		ContractCodeMetadata: codeMetadata,
		ContractAddress:      input.RecipientAddr,
		CodeDeployerAddress:  input.CallerAddr,
	}

	vmOutput, err = host.performCodeDeploymentAtContractUpgrade(codeDeployInput)
	if err != nil {
		log.Trace("doRunSmartContractUpgrade", "error", err)
		vmOutput = output.CreateVMOutputInCaseOfError(err)
		return vmOutput
	}

	return vmOutput
}

func (host *vmHost) checkGasForGetCode(input *vmcommon.ContractCallInput, metering vmhost.MeteringContext) error {
	getCodeBaseCost := metering.GasSchedule().BaseOperationCost.GetCode
	if input.GasProvided < getCodeBaseCost {
		return vmhost.ErrNotEnoughGas
	}

	return nil
}

// doRunSmartContractDelete deletes a contract directly
func (host *vmHost) doRunSmartContractDelete(input *vmcommon.ContractCallInput) (vmOutput *vmcommon.VMOutput) {
	host.InitState()
	defer func() {
		errs := host.GetRuntimeErrors()
		if errs != nil {
			log.Trace(fmt.Sprintf("doRunSmartContractDelete full error list for %s", input.Function), "error", errs)
		}
	}()

	_, _, metering, output, runtime, storage := host.GetContexts()

	defer func() {
		if vmOutput == nil || vmOutput.ReturnCode == vmcommon.VMExecutionFailed {
			runtime.CleanInstance()
		}
	}()

	runtime.InitStateFromContractCallInput(input)
	metering.InitStateFromContractCallInput(&input.VMInput)
	storage.SetAddress(runtime.GetContextAddress())

	if err := metering.DeductInitialGasForDirectDelete(); err != nil {
		return output.CreateVMOutputInCaseOfError(vmhost.ErrNotEnoughGas)
	}

	return host.doExecContractDelete(input)
}

func (host *vmHost) doExecContractDelete(input *vmcommon.ContractCallInput) *vmcommon.VMOutput {
	output := host.Output()
	// KLC-2347 defense-in-depth: primary read-only enforcement is at execute() dispatch.
	if host.ForkController().FixAuditChangesV2() && host.Runtime().ReadOnly() {
		return output.CreateVMOutputInCaseOfError(vmhost.ErrInvalidCallOnReadOnlyMode)
	}
	err := host.checkUpgradePermission(input)
	if err != nil {
		log.Trace("doExecContractDelete", "error", vmhost.ErrUpgradeNotAllowed)
		return output.CreateVMOutputInCaseOfError(err)
	}

	vmOutput := output.GetVMOutput()
	vmOutput.DeletedAccounts = append(vmOutput.DeletedAccounts, input.RecipientAddr)

	return vmOutput
}

func (host *vmHost) doRunSmartContractCall(input *vmcommon.ContractCallInput) *vmcommon.VMOutput {
	host.InitState()
	defer func() {
		errs := host.GetRuntimeErrors()
		if errs != nil {
			log.Trace(fmt.Sprintf("doRunSmartContractCall full error list for %s", input.Function), "error", errs)
		}
	}()

	_, _, metering, output, runtime, storage := host.GetContexts()

	var vmOutput *vmcommon.VMOutput
	defer func() {
		if vmOutput == nil || vmOutput.ReturnCode == vmcommon.VMExecutionFailed {
			host.Runtime().CleanInstance()
		}
	}()

	runtime.InitStateFromContractCallInput(input)
	metering.InitStateFromContractCallInput(&input.VMInput)
	storage.SetAddress(runtime.GetContextAddress())

	err := host.checkGasForGetCode(input, metering)
	if err != nil {
		log.Trace("doRunSmartContractCall check gas for GetSCCode", "error", vmhost.ErrNotEnoughGas)
		vmOutput = output.CreateVMOutputInCaseOfError(vmhost.ErrNotEnoughGas)
		return vmOutput
	}

	contract, err := runtime.GetSCCode()
	if err != nil {
		log.Trace("doRunSmartContractCall get code", "error", vmhost.ErrContractNotFound)
		vmOutput = output.CreateVMOutputInCaseOfError(vmhost.ErrContractNotFound)
		return vmOutput
	}

	// consume gas per byte of code
	err = metering.DeductInitialGasForExecution(contract)
	if err != nil {
		log.Trace("doRunSmartContractCall initial gas", "error", vmhost.ErrNotEnoughGas)
		vmOutput = output.CreateVMOutputInCaseOfError(vmhost.ErrNotEnoughGas)
		return vmOutput
	}

	err = runtime.StartWasmerInstance(contract, metering.GetGasForExecution(), false)
	if err != nil {
		vmOutput = output.CreateVMOutputInCaseOfError(vmhost.ErrContractInvalid)
		return vmOutput
	}

	err = host.callSCMethod()
	if err != nil {
		log.Trace("doRunSmartContractCall", "error", err)
		vmOutput = output.CreateVMOutputInCaseOfError(err)
		return vmOutput
	}

	output.RemoveNonUpdatedStorage()
	vmOutput = output.GetVMOutput()
	host.CompleteLogEntriesWithCallType(vmOutput, vmhost.DirectCallString)

	log.Trace("doRunSmartContractCall finished",
		"retCode", vmOutput.ReturnCode,
		"message", vmOutput.ReturnMessage,
		"data", vmOutput.ReturnData)

	return vmOutput
}

func copyTxHashesFromContext(runtime vmhost.RuntimeContext, input *vmcommon.ContractCallInput) {
	if input.CallType != vm.DirectCall {
		return
	}
	currentVMInput := runtime.GetVMInput()
	if len(currentVMInput.OriginalTxHash) > 0 {
		input.OriginalTxHash = currentVMInput.OriginalTxHash
	}
	if len(currentVMInput.CurrentTxHash) > 0 {
		input.CurrentTxHash = currentVMInput.CurrentTxHash
	}
}

// ExecuteOnDestContext pushes each context to the corresponding stack
// and initializes new contexts for executing the contract call with the given input
func (host *vmHost) ExecuteOnDestContext(input *vmcommon.ContractCallInput) (vmOutput *vmcommon.VMOutput, err error) {
	log.Trace("ExecuteOnDestContext", "caller", input.CallerAddr, "dest", input.RecipientAddr, "function", input.Function, "gas", input.GasProvided)

	scExecutionInput := input

	blockchain := host.Blockchain()

	blockchain.PushState()

	// if ExecuteOnDestContext is called with an embedded builtin function call
	// then parser the builtin function call and execute it i.e. (with_kda_transfer , with_klv_transfer)
	if host.IsBuiltinFunctionName(input.Function) {
		// scExecutionInput will have the function call embedded as the new input function call
		scExecutionInput, vmOutput, err = host.handleBuiltinFunctionCall(input)
		if err != nil {
			blockchain.PopSetActiveState()
			host.Runtime().AddError(err, input.Function)
			vmOutput = host.Output().CreateVMOutputInCaseOfError(err)
			return
		}

		host.completeLogEntriesAfterBuiltinCall(input, vmOutput)
	}

	// execute OnDestContext Calls
	if scExecutionInput != nil {
		vmOutput, err = host.executeOnDestContextNoBuiltinFunction(scExecutionInput)
		host.addNewBackTransfersFromVMOutput(vmOutput, scExecutionInput.CallerAddr, scExecutionInput.RecipientAddr)
	}

	if err != nil {
		blockchain.PopSetActiveState()
	} else {
		blockchain.PopDiscard()
	}

	return
}

func (host *vmHost) addNewBackTransfersFromVMOutput(vmOutput *vmcommon.VMOutput, parent, child []byte) {
	if vmOutput == nil || vmOutput.ReturnCode != vmcommon.Ok {
		return
	}
	callerOutAcc, ok := vmOutput.OutputAccounts[string(parent)]
	if !ok {
		return
	}

	// Clean previous back transfers before add the new ones
	host.managedTypesContext.CleanBackTransfers()

	for _, transfer := range callerOutAcc.OutputTransfers {
		transfer := transfer

		if !bytes.Equal(transfer.SenderAddress, child) {
			continue
		}

		if len(transfer.KDATransfers.KDATokenName) == 0 ||
			bytes.Equal(transfer.KDATransfers.KDATokenName, kdautils.KLVIdentifier) {
			host.managedTypesContext.AddValueOnlyBackTransfer(transfer.KDATransfers.KDAValue)
			continue
		}

		host.managedTypesContext.AddBackTransfers([]*vmcommon.KDATransfer{&transfer.KDATransfers})
	}
}

func (host *vmHost) completeLogEntriesAfterBuiltinCall(input *vmcommon.ContractCallInput, vmOutput *vmcommon.VMOutput) {
	switch input.CallType {
	default:
		host.CompleteLogEntriesWithCallType(vmOutput, vmhost.ExecuteOnDestContextString)
	}
}

func (host *vmHost) handleFunctionCallOnOtherVM(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	output := host.Output()

	vmOutput, err := host.callFunctionOnOtherVM(input)
	if err != nil {
		log.Trace("ExecuteOnDestContext function on other VM", "error", err)
		return nil, err
	}

	output.AddToActiveState(vmOutput)

	return vmOutput, nil
}

func (host *vmHost) handleBuiltinFunctionCall(input *vmcommon.ContractCallInput) (*vmcommon.ContractCallInput, *vmcommon.VMOutput, error) {
	output := host.Output()

	postBuiltinInput, builtinOutput, err := host.callBuiltinFunction(input)
	if err != nil {
		log.Trace("ExecuteOnDestContext builtin function", "error", err)
		return nil, nil, err
	}

	output.AddToActiveState(builtinOutput)

	return postBuiltinInput, builtinOutput, nil
}

func (host *vmHost) executeOnDestContextNoBuiltinFunction(input *vmcommon.ContractCallInput) (vmOutput *vmcommon.VMOutput, err error) {
	if host.IsOutOfVMFunctionExecution(input) {
		vmOutput, err = host.handleFunctionCallOnOtherVM(input)
		if err != nil {
			host.Runtime().AddError(err, input.Function)
			vmOutput = host.Output().CreateVMOutputInCaseOfError(err)
		}

		return vmOutput, err
	}

	managedTypes, _, metering, output, runtime, storage := host.GetContexts()
	managedTypes.PushState()
	managedTypes.InitState()

	output.PushState()
	output.CensorVMOutput()

	copyTxHashesFromContext(runtime, input)
	runtime.PushState()
	runtime.InitStateFromContractCallInput(input)

	metering.PushState()
	metering.InitStateFromContractCallInput(&input.VMInput)

	storage.PushState()
	storage.SetAddress(runtime.GetContextAddress())

	defer func() {
		vmOutput = host.finishExecuteOnDestContext(err)
		if err == nil && vmOutput.ReturnCode != vmcommon.Ok {
			err = vmhost.ErrExecutionFailed
		}
		runtime.AddError(err, input.Function)
	}()

	// Perform a value transfer to the called SC. If the execution fails, this
	// transfer will not persist.
	for _, transfer := range input.KDATransfers {
		if transfer.IsExecuted() {
			log.Trace("executeOnDestContextNoBuiltinFunction skip executed transfers", "assetID", string(transfer.KDATokenName), "amount", transfer.KDAValue.Int64())
			continue
		}

		// can only proceed with KLV transfers in no builtin calls
		if transfer.KDATokenNonce > 0 ||
			(len(transfer.KDATokenName) > 0 &&
				!bytes.Equal(transfer.KDATokenName, kdautils.KLVIdentifier)) {
			err = vmhost.ErrFailedTransfer
			return vmOutput, err
		}

		err = output.TransferValueOnly(input.RecipientAddr, input.CallerAddr, transfer.KDAValue, false)
		if err != nil {
			log.Trace("ExecuteOnDestContext transfer", "error", err)
			return vmOutput, err
		}

		output.WriteLogWithIdentifier(
			input.CallerAddr,
			[][]byte{transfer.KDAValue.Bytes(), input.RecipientAddr},
			vmcommon.FormatLogDataForCall(vmhost.ExecuteOnDestContextString, input.Function, input.Arguments),
			[]byte(vmhost.TransferValueOnlyString),
		)
	}

	err = host.execute(input)
	if err != nil {
		log.Trace("ExecuteOnDestContext execution", "error", err)
		return vmOutput, err
	}

	return vmOutput, err
}

func (host *vmHost) finishExecuteOnDestContext(executeErr error) *vmcommon.VMOutput {
	managedTypes, _, metering, output, runtime, storage := host.GetContexts()

	var vmOutput *vmcommon.VMOutput
	if executeErr != nil {
		// Execution failed: restore contexts as if the execution didn't happen,
		// but first create a vmOutput to capture the error.
		vmOutput = output.CreateVMOutputInCaseOfError(executeErr)
	} else {
		// Retrieve the VMOutput before popping the Runtime state and the previous
		// instance, to ensure accurate GasRemaining
		vmOutput = output.GetVMOutput()
	}

	gasSpentByChildContract := metering.GasSpentByContract()

	// Restore the previous context states
	managedTypes.PopSetActiveState()
	storage.PopSetActiveState()

	if vmOutput.ReturnCode == vmcommon.Ok {
		metering.PopMergeActiveState()
		output.PopMergeActiveState()
	} else {
		metering.PopSetActiveState()
		output.PopSetActiveState()
	}

	log.Trace("ExecuteOnDestContext finished", "sc", string(runtime.GetContextAddress()), "function", runtime.FunctionName())
	log.Trace("ExecuteOnDestContext finished", "gas spent", gasSpentByChildContract, "gas remaining", vmOutput.GasRemaining)

	// Return to the caller context completely
	runtime.PopSetActiveState()

	// Restore remaining gas to the caller Wasmer instance
	metering.RestoreGas(vmOutput.GasRemaining)

	return vmOutput
}

// ExecuteOnSameContext executes the contract call with the given input
// on the same runtime context. Some other contexts are backed up.
func (host *vmHost) ExecuteOnSameContext(input *vmcommon.ContractCallInput) error {
	log.Trace("ExecuteOnSameContext", "function", input.Function)

	if host.IsBuiltinFunctionName(input.Function) {
		return vmhost.ErrBuiltinCallOnSameContextDisallowed
	}

	managedTypes, blockchain, metering, output, runtime, _ := host.GetContexts()

	// Back up the states of the contexts (except Storage, which aren't affected
	// by ExecuteOnSameContext())
	managedTypes.PushState()
	managedTypes.InitState()
	output.PushState()

	librarySCAddress := make([]byte, len(input.RecipientAddr))
	copy(librarySCAddress, input.RecipientAddr)

	input.RecipientAddr = input.CallerAddr

	copyTxHashesFromContext(runtime, input)
	runtime.PushState()
	runtime.InitStateFromContractCallInput(input)
	runtime.SetCodeAddress(librarySCAddress)

	metering.PushState()
	metering.InitStateFromContractCallInput(&input.VMInput)

	blockchain.PushState()

	var err error

	defer func() {
		deferredErr := error(nil)
		if host.ForkController().FixAuditChangesV4() {
			deferredErr = err
		}
		host.finishExecuteOnSameContext(deferredErr)
	}()

	// Perform a value transfer to the called SC. If the execution fails, this
	// transfer will not persist.
	for _, transfer := range input.KDATransfers {
		if transfer.IsExecuted() {
			log.Trace("ExecuteOnSameContext skip executed transfers", "assetID", string(transfer.KDATokenName), "amount", transfer.KDAValue.Int64())
			continue
		}

		// can only proceed with KLV transfers in ExecuteOnSameContext calls
		if transfer.KDATokenNonce > 0 ||
			(len(transfer.KDATokenName) > 0 &&
				!bytes.Equal(transfer.KDATokenName, kdautils.KLVIdentifier)) {

			err = vmhost.ErrFailedTransfer
			return err
		}

		err = output.TransferValueOnly(input.RecipientAddr, input.CallerAddr, transfer.KDAValue, false)
		if err != nil {
			runtime.AddError(err, input.Function)
			return err
		}

		output.WriteLogWithIdentifier(
			input.CallerAddr,
			[][]byte{transfer.KDAValue.Bytes(), input.RecipientAddr},
			vmcommon.FormatLogDataForCall(vmhost.ExecuteOnSameContextString, input.Function, input.Arguments),
			[]byte(vmhost.TransferValueOnlyString),
		)
	}

	err = host.execute(input)
	runtime.AddError(err, input.Function)
	return err
}

func (host *vmHost) finishExecuteOnSameContext(executeErr error) {
	managedTypes, blockchain, metering, output, runtime, _ := host.GetContexts()

	if output.ReturnCode() != vmcommon.Ok || executeErr != nil {
		// Execution failed: restore contexts as if the execution didn't happen.
		managedTypes.PopSetActiveState()
		metering.PopSetActiveState()
		output.PopSetActiveState()
		blockchain.PopSetActiveState()
		runtime.PopSetActiveState()
		return
	}

	// Execution successful; retrieve the VMOutput before popping the Runtime
	// state and the previous instance, to ensure accurate GasRemaining and
	// GasUsed for all accounts.
	vmOutput := output.GetVMOutput()

	metering.PopMergeActiveState()
	output.PopDiscard()
	blockchain.PopDiscard()
	managedTypes.PopSetActiveState()
	runtime.PopSetActiveState()
	// Restore remaining gas to the caller (parent) Wasmer instance
	metering.RestoreGas(vmOutput.GasRemaining)
}

func (host *vmHost) isInitFunctionBeingCalled() bool {
	functionName := host.Runtime().FunctionName()
	return functionName == vmhost.InitFunctionName
}

// IsOutOfVMFunctionExecution returns true if the call should be executed on ahother VM
func (host *vmHost) IsOutOfVMFunctionExecution(input *vmcommon.ContractCallInput) bool {
	isSmartContract := host.Blockchain().IsSmartContract(input.RecipientAddr)
	if isSmartContract {
		vmType, err := vmcommon.ParseVMTypeFromContractAddress(input.RecipientAddr)
		if err != nil {
			return false
		}
		if !bytes.Equal(host.Runtime().GetVMType(), vmType) {
			return true
		}
	}
	return false
}

// IsBuiltinFunctionName returns true if the given function name is the same as any protocol builtin function
func (host *vmHost) IsBuiltinFunctionName(functionName string) bool {
	function, err := host.builtInFuncContainer.Get(functionName)
	if err != nil {
		return false
	}

	return function.IsActive()
}

// IsBuiltinFunctionCall returns true if the given data contains a call to a protocol builtin function
func (host *vmHost) IsBuiltinFunctionCall(data []byte) bool {
	functionName, _, _ := host.callArgsParser.ParseData(string(data))
	return host.IsBuiltinFunctionName(functionName)
}

func (host *vmHost) GetCallArgsParser() vmhost.CallArgsParser {
	return host.callArgsParser
}

// CreateNewContract creates a new contract indirectly (from another Smart Contract)
func (host *vmHost) CreateNewContract(input *vmcommon.ContractCreateInput, createContractCallType int) (newContractAddress []byte, err error) {
	newContractAddress = nil
	err = nil

	defer func() {
		if err != nil {
			newContractAddress = nil
		}
	}()

	_, blockchain, metering, output, runtime, _ := host.GetContexts()

	codeDeployInput := vmhost.CodeDeployInput{
		ContractCode:         input.ContractCode,
		ContractCodeMetadata: input.ContractCodeMetadata,
		ContractAddress:      nil,
		CodeDeployerAddress:  input.CallerAddr,
	}

	err = metering.UseGasForIndirectDeployment(codeDeployInput)
	if err != nil {
		return
	}

	if runtime.ReadOnly() {
		err = vmhost.ErrInvalidCallOnReadOnlyMode
		return
	}

	newContractAddress, err = blockchain.NewAddress(input.CallerAddr)
	if err != nil {
		return
	}

	if blockchain.AccountExists(newContractAddress) {
		err = vmhost.ErrDeploymentOverExistingAccount
		return
	}

	if host.ForkController().FixAuditChangesV4() && output.HasPendingCodeUpdate(newContractAddress) {
		err = vmhost.ErrDeploymentOverExistingAccount
		return
	}

	codeDeployInput.ContractAddress = newContractAddress

	err = host.verifyNoStartSectionOnDeploy(codeDeployInput.ContractCode)
	if err != nil {
		return
	}

	output.DeployCode(codeDeployInput)

	defer func() {
		if err != nil {
			output.DeleteOutputAccount(newContractAddress)
		}
	}()

	runtime.MustVerifyNextContractCode()

	initCallInput := &vmcommon.ContractCallInput{
		RecipientAddr:     newContractAddress,
		Function:          vmhost.InitFunctionName,
		AllowInitFunction: true,
		VMInput:           input.VMInput,
	}

	initVmOutput, err := host.ExecuteOnDestContext(initCallInput)
	if err != nil {
		return
	}

	if createContractCallType == vmhooks.DeployContract {
		host.CompleteLogEntriesWithCallType(initVmOutput, vmhost.DeployFromSourceString)
	} else {
		host.CompleteLogEntriesWithCallType(initVmOutput, vmhost.DeploySmartContractString)
	}

	if blockchain.IsSmartContract(input.CallerAddr) {
		blockchain.IncreaseNonce(input.CallerAddr)
	}

	return
}

func (host *vmHost) verifyNoStartSectionOnDeploy(contractCode []byte) error {
	if !host.ForkController().FixAuditChangesV4() {
		return nil
	}

	err := contexts.VerifyNoStartSection(contractCode)
	if err != nil {
		log.Trace("CreateNewContract/VerifyNoStartSection", "err", err)
	}

	return err
}

func (host *vmHost) checkUpgradePermission(vmInput *vmcommon.ContractCallInput) error {
	contract, err := host.Blockchain().GetUserAccount(vmInput.RecipientAddr)
	if err != nil {
		return err
	}
	if check.IfNilReflect(contract) {
		return vmhost.ErrNilContract
	}

	codeMetadata := vmcommon.CodeMetadataFromBytes(contract.GetCodeMetadata())
	isUpgradeable := codeMetadata.Upgradeable
	callerAddress := vmInput.CallerAddr
	ownerAddress := contract.GetOwnerAddress()
	isCallerOwner := bytes.Equal(callerAddress, ownerAddress)

	if isUpgradeable && isCallerOwner {
		return nil
	}

	return vmhost.ErrUpgradeNotAllowed
}

// executeUpgrade upgrades a contract indirectly (from another contract). This
// function follows the convention of executeSmartContractCall().
func (host *vmHost) executeUpgrade(input *vmcommon.ContractCallInput) error {
	_, _, metering, output, runtime, _ := host.GetContexts()

	// KLC-2347 defense-in-depth: primary read-only enforcement is at execute() dispatch.
	if host.ForkController().FixAuditChangesV2() && runtime.ReadOnly() {
		return vmhost.ErrInvalidCallOnReadOnlyMode
	}

	err := host.checkUpgradePermission(input)
	if err != nil {
		return err
	}

	code, codeMetadata, err := runtime.ExtractCodeUpgradeFromArgs()
	if err != nil {
		return vmhost.ErrInvalidUpgradeArguments
	}

	codeDeployInput := vmhost.CodeDeployInput{
		ContractCode:         code,
		ContractCodeMetadata: codeMetadata,
		ContractAddress:      input.RecipientAddr,
		CodeDeployerAddress:  input.CallerAddr,
	}

	err = metering.DeductInitialGasForIndirectDeployment(codeDeployInput)
	if err != nil {
		output.SetReturnCode(vmcommon.VMOutOfGas)
		return err
	}

	runtime.MustVerifyNextContractCode()

	err = runtime.StartWasmerInstance(codeDeployInput.ContractCode, metering.GetGasForExecution(), true)
	if err != nil {
		log.Trace("performCodeDeployment/StartWasmerInstance", "err", err)
		if host.ForkController().FixAuditChangesV4() && errors.Is(err, vmhost.ErrContractInvalid) {
			return err
		}
		return vmhost.ErrContractInvalid
	}

	err = host.callUpgradeFunction()
	if err != nil {
		return err
	}

	output.DeployCode(codeDeployInput)
	if output.ReturnCode() != vmcommon.Ok {
		return vmhost.ErrReturnCodeNotOk
	}

	return nil
}

func (host *vmHost) executeDelete(input *vmcommon.ContractCallInput) error {
	_, _, metering, _, runtime, _ := host.GetContexts()

	// KLC-2347 defense-in-depth: primary read-only enforcement is at execute() dispatch.
	if host.ForkController().FixAuditChangesV2() && runtime.ReadOnly() {
		return vmhost.ErrInvalidCallOnReadOnlyMode
	}

	// end previous runtime if any, prevent points to the previous runtime
	runtime.EndExecution()

	err := metering.DeductInitialGasForDirectDelete()
	if err != nil {
		return err
	}

	vmOutput := host.doExecContractDelete(input)
	logGasTrace.Trace("executeDelete Contract", "gasRemaining", vmOutput.GasRemaining)
	return host.checkFinalGasAfterExit()
}

// execute executes an indirect call to a smart contract, assuming there is an
// already-running Wasmer instance of another contract that has requested the
// indirect call. This method creates a new Wasmer instance and pushes the
// previous one onto the Runtime instance stack, but it will not pop the
// previous instance back - that remains the responsibility of the calling
// code. Also, this method does not restore the gas remaining after the
// indirect call, it does not push the states of any Host Context onto their
// respective stacks, nor does it pop any state stack. Handling the state
// stacks and the remaining gas are responsibilities of the calling code, which
// must push and pop as required, before and after calling this method, and
// handle the remaining gas. These principles also apply to indirect contract
// upgrading (via host.executeUpgrade(), which also does not pop the previous
// instance from the Runtime instance stack, nor does it restore the remaining
// gas).
func (host *vmHost) execute(input *vmcommon.ContractCallInput) error {
	_, _, metering, output, runtime, _ := host.GetContexts()

	if host.isInitFunctionBeingCalled() && !input.AllowInitFunction {
		return vmhost.ErrInitFuncCalledInRun
	}

	// Use all gas initially, on the Wasmer instance of the caller. In case of
	// successful execution, the unused gas will be restored.
	metering.UseGas(input.GasProvided)

	isUpgrade := input.Function == vmhost.UpgradeFunctionName
	isDelete := input.Function == vmhost.DeleteFunctionName

	// KLC-2347: read-only nested execution must not produce delete/upgrade side effects.
	// This is the primary guard for indirect dispatch; per-helper guards below are defense-in-depth.
	if (isUpgrade || isDelete) && host.ForkController().FixAuditChangesV2() && runtime.ReadOnly() {
		return vmhost.ErrInvalidCallOnReadOnlyMode
	}

	if isUpgrade {
		return host.executeUpgrade(input)
	}

	if isDelete {
		return host.executeDelete(input)
	}

	contract, err := runtime.GetSCCode()
	if err != nil {
		return err
	}

	// consume gas per byte of code
	err = metering.DeductInitialGasForExecution(contract)
	if err != nil {
		return err
	}

	// Replace the current Wasmer instance of the Runtime with a new one; this
	// assumes that the instance was preserved on the Runtime instance stack
	// before calling executeSmartContractCall().
	err = runtime.StartWasmerInstance(contract, metering.GetGasForExecution(), false)
	if err != nil {
		return err
	}

	err = host.callSCMethodIndirect()
	if err != nil {
		return err
	}

	if host.ForkController().FixAuditChangesV4() {
		err = host.checkFinalGasAfterExit()
		if err != nil {
			return err
		}
	}

	if output.ReturnCode() != vmcommon.Ok {
		return vmhost.ErrReturnCodeNotOk
	}

	return nil
}

func (host *vmHost) callSCMethodIndirect() error {
	if host.ForkController().FixAuditChangesV3() &&
		host.Runtime().FunctionName() == vmhost.ContractsUpgradeFunctionName {
		return vmhost.ErrInitFuncCalledInRun
	}

	functionName, err := host.Runtime().FunctionNameChecked()
	if err != nil {
		if errors.Is(err, vmhost.ErrNilCallbackFunction) {
			return nil
		}
		return err
	}

	err = host.Runtime().CallSCFunction(functionName)
	if err != nil {
		err = host.handleBreakpointIfAny(err)
	}

	return err
}

// ExecuteKDATransfer calls the process built-in function with the given transfer for KDA/KDANFT if nonce > 0
// there are no NFTs with nonce == 0, it will call multi transfer if multiple tokens are sent
func (host *vmHost) ExecuteKDATransfer(transfersArgs *vmhost.KDATransfersArgs, callType vm.CallType) (*vmcommon.VMOutput, uint64, error) {
	if len(transfersArgs.Transfers) == 0 {
		return nil, 0, vmhost.ErrFailedTransfer
	}

	if host.Runtime().ReadOnly() {
		return nil, 0, vmhost.ErrInvalidCallOnReadOnlyMode
	}

	// Verify if it should check the destination SC is payable
	// Backtransfers don't need to verify the destination SC, which is represented by callType == vm.KDATransferAndExecute
	mustVerifySCDestination := callType != vm.KDATransferAndExecute
	if mustVerifySCDestination {
		if err := host.validateSCDestination(transfersArgs.Sender, transfersArgs.Destination); err != nil {
			return nil, 0, err
		}
	}

	_, _, metering, _, _, _ := host.GetContexts()

	kdaTransferInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			OriginalCallerAddr: transfersArgs.OriginalCaller,
			CallerAddr:         transfersArgs.Sender,
			Arguments:          make([][]byte, 0),
			CallType:           callType,
			GasProvided:        metering.GasLeft(),
			KDATransfers:       transfersArgs.Transfers,
		},
		RecipientAddr:     transfersArgs.Destination,
		Function:          core.BuiltInFunctionTransfer,
		AllowInitFunction: false,
	}

	if log.GetLevel() == logger.LogTrace {
		for _, transfer := range transfersArgs.Transfers {
			log.Trace("KDA transfer", "token", string(transfer.KDATokenName), "nonce", transfer.KDATokenNonce, "value", transfer.KDAValue)
		}
	}
	log.Debug("KDA transfer", "sender", transfersArgs.Sender, "dest", transfersArgs.Destination)
	vmOutput, err := host.Blockchain().ProcessBuiltInFunction(kdaTransferInput)
	if err != nil {
		log.Trace("KDA transfer", "error", err.Error())
		return vmOutput, kdaTransferInput.GasProvided, err
	}

	if vmOutput.ReturnCode != vmcommon.Ok {
		log.Trace("KDA transfer", "retcode", vmOutput.ReturnCode, "message", vmOutput.ReturnMessage)
		return vmOutput, kdaTransferInput.GasProvided, vmhost.ErrExecutionFailed
	}

	err = vmOutput.ReindexTransfers(host.Output())
	if err != nil {
		return nil, 0, err
	}

	gasConsumed := math.SubUint64(kdaTransferInput.GasProvided, vmOutput.GasRemaining)

	if metering.GasLeft() < gasConsumed {
		log.Trace("KDA transfer", "error", vmhost.ErrNotEnoughGas)
		return vmOutput, kdaTransferInput.GasProvided, vmhost.ErrNotEnoughGas
	}
	metering.UseGas(gasConsumed)

	return vmOutput, gasConsumed, nil
}

// validateSCDestination checks if the destination is a valid SC address and if it is payable
func (host *vmHost) validateSCDestination(sender []byte, destination []byte) error {
	// IsPayable now handles all validation including uninitialized contract addresses
	isPayable, err := host.Blockchain().IsPayable(sender, destination)
	if err != nil {
		log.Trace("error checking if SC is payable", "address", destination, "error", err)
		return err
	}

	if !isPayable {
		log.Trace("SC is not payable", "address", destination)
		return vmhost.ErrAccountNotPayable
	}

	return nil
}

func (host *vmHost) callFunctionOnOtherVM(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, error) {
	metering := host.Metering()

	vmOutput, err := host.Blockchain().ExecuteSmartContractCallOnOtherVM(input)
	if err != nil {
		metering.UseGas(input.GasProvided)
		return nil, err
	}

	err = vmOutput.ReindexTransfers(host.Output())
	if err != nil {
		return nil, err
	}

	metering.TrackGasUsedByOutOfVMFunction(input, vmOutput)

	return vmOutput, nil
}

func (host *vmHost) callBuiltinFunction(input *vmcommon.ContractCallInput) (*vmcommon.ContractCallInput, *vmcommon.VMOutput, error) {
	metering := host.Metering()

	if host.Runtime().ReadOnly() {
		return nil, nil, vmhost.ErrInvalidCallOnReadOnlyMode
	}

	var parsedTransfer *vmcommon.ParsedKDATransfers

	if input.Function == core.BuiltInFunctionTransfer {
		var err error

		parsedTransfer, err = host.kdaTransferParser.ParseKDATransfers(input.RecipientAddr, input.Arguments)
		if err != nil {
			return nil, nil, err
		}

		input.KDATransfers = append(input.KDATransfers, parsedTransfer.KDATransfers...)
		// recipient may be updated by the parser
		input.RecipientAddr = parsedTransfer.RcvAddr
	}

	vmOutput, err := host.Blockchain().ProcessBuiltInFunction(input)
	if err != nil {
		metering.UseGas(input.GasProvided)
		return nil, nil, err
	}

	newVMInput, err := host.isSCExecutionAfterBuiltInFunc(parsedTransfer, input, vmOutput)
	if err != nil {
		metering.UseGas(input.GasProvided)
		return nil, nil, err
	}

	if newVMInput != nil {
		for _, outAcc := range vmOutput.OutputAccounts {
			outAcc.OutputTransfers = make([]vmcommon.OutputTransfer, 0)
		}
	}

	// reindex only for the case of no execution after builtin call
	err = vmOutput.ReindexTransfers(host.Output())
	if err != nil {
		return nil, nil, err
	}

	metering.TrackGasUsedByOutOfVMFunction(input, vmOutput)

	return newVMInput, vmOutput, nil
}

func (host *vmHost) isSCExecutionAfterBuiltInFunc(
	parsedTransfer *vmcommon.ParsedKDATransfers,
	vmInput *vmcommon.ContractCallInput,
	vmOutput *vmcommon.VMOutput,
) (*vmcommon.ContractCallInput, error) {
	if vmOutput.ReturnCode != vmcommon.Ok {
		return nil, nil
	}

	if parsedTransfer == nil || !host.Blockchain().IsSmartContract(parsedTransfer.RcvAddr) {
		return nil, nil
	}

	newVMInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			OriginalCallerAddr: vmInput.OriginalCallerAddr,
			CallerAddr:         vmInput.CallerAddr,
			Arguments:          parsedTransfer.CallArgs,
			CallType:           vmInput.CallType,
			GasProvided:        vmOutput.GasRemaining,
			OriginalTxHash:     vmInput.OriginalTxHash,
			CurrentTxHash:      vmInput.CurrentTxHash,
			KDATransfers:       make([]*vmcommon.KDATransfer, len(vmInput.KDATransfers)),
		},
		RecipientAddr:     parsedTransfer.RcvAddr,
		Function:          parsedTransfer.CallFunction,
		AllowInitFunction: false,
	}

	// load from inputs and mark as executed
	for i, transfer := range vmInput.KDATransfers {
		t := transfer.Clone()
		newVMInput.KDATransfers[i] = t.SetExecuted()
	}

	return newVMInput, nil
}

func (host *vmHost) checkFinalGasAfterExit() error {
	totalUsedPoints := host.Runtime().GetPointsUsed()

	if totalUsedPoints > host.Metering().GetGasForExecution() {
		log.Trace("checkFinalGasAfterExit failed", "totalUsedPoints", totalUsedPoints, "gasForExecution", host.Metering().GetGasForExecution())
		return vmhost.ErrNotEnoughGas
	}

	log.Trace("checkFinalGasAfterExit ok", "totalUsedPoints", totalUsedPoints, "gasForExecution", host.Metering().GetGasForExecution())
	return nil
}

func (host *vmHost) callInitFunction() error {
	return host.callSCFunction(vmhost.InitFunctionName)
}

func (host *vmHost) callUpgradeFunction() error {
	return host.callSCFunction(vmhost.ContractsUpgradeFunctionName)
}

func (host *vmHost) callSCFunction(functionName string) error {
	runtime := host.Runtime()
	if !runtime.HasFunction(functionName) {
		return executor.ErrFuncNotFound
	}

	err := runtime.CallSCFunction(functionName)
	if err != nil {
		err = host.handleBreakpointIfAny(err)
	}

	if err == nil {
		err = host.checkFinalGasAfterExit()
	}

	return err
}

func (host *vmHost) callSCMethod() error {
	log.Trace("callSCMethod")

	runtime := host.Runtime()
	callType := runtime.GetVMInput().CallType

	var err error
	switch callType {
	case vm.DirectCall:
		err = host.callSCMethodDirectCall()
	default:
		err = vmhost.ErrUnknownCallType
	}

	if err != nil {
		log.Trace("call SC method failed", "error", err, "callType", callType)
	}

	return err
}

func (host *vmHost) callSCMethodDirectCall() error {
	_, err := host.callFunctionAndExecute()
	return err
}

func (host *vmHost) callFunctionAndExecute() (bool, error) {
	runtime := host.Runtime()

	log.Trace("callFunctionAndExecute", "function", runtime.FunctionName(), "args", runtime.Arguments())

	if runtime.FunctionName() == "" {
		return false, executor.ErrInvalidFunction
	}

	err := host.verifyAllowedFunctionCall()
	if err != nil {
		log.Trace("call SC method failed", "error", err, "src", "verifyAllowedFunctionCall")
		return false, err
	}

	functionName, err := runtime.FunctionNameChecked()
	if err != nil {
		log.Trace("call SC method failed", "error", err, "src", "FunctionNameChecked")
		return false, err
	}

	err = runtime.CallSCFunction(functionName)
	if err != nil {
		err = host.handleBreakpointIfAny(err)
		log.Trace("breakpoint detected and handled", "err", err)
	}

	if err == nil {
		err = host.checkFinalGasAfterExit()
	}

	if err != nil {
		log.Trace("call SC method failed", "error", err, "src", "sc function", "runtime errors", host.GetRuntimeErrors())
		return true, err
	}

	return true, nil
}

func (host *vmHost) verifyAllowedFunctionCall() error {
	runtime := host.Runtime()
	functionName := runtime.FunctionName()

	isInit := functionName == vmhost.InitFunctionName
	if isInit {
		return vmhost.ErrInitFuncCalledInRun
	}
	isUpgrade := functionName == vmhost.ContractsUpgradeFunctionName
	if isUpgrade {
		return vmhost.ErrInitFuncCalledInRun
	}

	return nil
}
