package contexts

import (
	"bytes"
	"errors"
	"math/big"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/kvm/executor"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
	"github.com/klever-io/klever-go/vmcommon/parsers"
)

var _ vmhost.OutputContext = (*outputContext)(nil)

var logOutput = logger.GetOrCreate("vm/output")

type outputContext struct {
	host             vmhost.VMHost
	outputState      *vmcommon.VMOutput
	stateStack       []*vmcommon.VMOutput
	codeUpdates      map[string]struct{}
	crtTransferIndex uint32
	callArgsParser   vmcommon.CallArgsParser
}

// NewOutputContext creates a new outputContext
func NewOutputContext(host vmhost.VMHost) (*outputContext, error) {
	if check.IfNil(host) {
		return nil, vmhost.ErrNilVMHost
	}

	context := &outputContext{
		host:             host,
		stateStack:       make([]*vmcommon.VMOutput, 0),
		crtTransferIndex: 1,
		callArgsParser:   parsers.NewCallArgsParser(),
	}

	context.InitState()

	return context, nil
}

// InitState initializes the output state and the code updates.
func (context *outputContext) InitState() {
	context.outputState = newVMOutput()
	context.codeUpdates = make(map[string]struct{})
	context.crtTransferIndex = 1
}

func newVMOutput() *vmcommon.VMOutput {
	return &vmcommon.VMOutput{
		ReturnData:      make([][]byte, 0),
		ReturnCode:      vmcommon.Ok,
		ReturnMessage:   "",
		GasRemaining:    0,
		OutputAccounts:  make(map[string]*vmcommon.OutputAccount),
		DeletedAccounts: make([][]byte, 0),
		Logs:            make([]*vmcommon.LogEntry, 0),
	}
}

// NewVMOutputAccount creates a new output account and sets the given address
func NewVMOutputAccount(address []byte) *vmcommon.OutputAccount {
	return &vmcommon.OutputAccount{
		Address:                 address,
		StorageUpdates:          make(map[string]*vmcommon.StorageUpdate),
		BytesAddedToStorage:     0,
		BytesDeletedFromStorage: 0,
	}
}

// PushState appends the current vmOutput to the state stack
func (context *outputContext) PushState() {
	newState := newVMOutput()
	mergeVMOutputs(newState, context.outputState)
	context.stateStack = append(context.stateStack, newState)
}

// PopSetActiveState removes the latest entry from the state stack and sets it as the current vm output
func (context *outputContext) PopSetActiveState() {
	stateStackLen := len(context.stateStack)
	if stateStackLen == 0 {
		return
	}

	prevState := context.stateStack[stateStackLen-1]
	context.stateStack = context.stateStack[:stateStackLen-1]
	context.outputState = prevState
}

// PopMergeActiveState merges the current state into the head of the stateStack,
// then pop the head of the stateStack into the current state.
// Doing this allows the VM to execute a SmartContract into a context on top
// of an existing context (a previous SC) without allowing access to it, but
// later merging the output of the two SCs in chronological order.
func (context *outputContext) PopMergeActiveState() {
	stateStackLen := len(context.stateStack)
	if stateStackLen == 0 {
		return
	}

	prevState := context.stateStack[stateStackLen-1]
	context.stateStack = context.stateStack[:stateStackLen-1]

	mergeVMOutputs(prevState, context.outputState)
	context.outputState = newVMOutput()
	mergeVMOutputs(context.outputState, prevState)
}

// PopDiscard removes the latest entry from the state stack, but maintaining
// all GasUsed values.
func (context *outputContext) PopDiscard() {
	stateStackLen := len(context.stateStack)
	if stateStackLen == 0 {
		return
	}

	context.stateStack = context.stateStack[:stateStackLen-1]
}

// ClearStateStack reinitializes the state stack.
func (context *outputContext) ClearStateStack() {
	context.stateStack = make([]*vmcommon.VMOutput, 0)
}

// CensorVMOutput will cause the next executed SC to appear isolated, as if
// nothing was executed before. Required for ExecuteOnDestContext().
// StorageUpdates are not deleted from context.outputState.OutputAccounts,
// preserving the storage cache.
func (context *outputContext) CensorVMOutput() {
	context.outputState.ReturnData = make([][]byte, 0)
	context.outputState.ReturnCode = vmcommon.Ok
	context.outputState.ReturnMessage = ""
	context.outputState.GasRemaining = 0
	context.outputState.Logs = make([]*vmcommon.LogEntry, 0)

	for _, account := range context.outputState.OutputAccounts {
		account.OutputTransfers = append(
			[]vmcommon.OutputTransfer{},
			account.OutputTransfers...,
		)
	}

	logOutput.Trace("state content censored")
}

func (context *outputContext) SetOutputAccount(address []byte, data *vmcommon.OutputAccount) {
	if len(address) == 0 {
		return
	}
	if data == nil {
		return
	}
	context.outputState.OutputAccounts[string(address)] = data
}

// GetOutputAccount returns the output account present at the given address,
// and a bool that is true if the account is new. If no output account is present at that address,
// a new account will be created and added to the output accounts.
func (context *outputContext) GetOutputAccount(address []byte) (*vmcommon.OutputAccount, bool) {
	accountIsNew := false
	account, ok := context.outputState.OutputAccounts[string(address)]
	if !ok {
		account = NewVMOutputAccount(address)
		context.outputState.OutputAccounts[string(address)] = account
		accountIsNew = true
	}

	return account, accountIsNew
}

// GetOutputAccounts returns all the OutputAccounts in the current outputState.
func (context *outputContext) GetOutputAccounts() map[string]*vmcommon.OutputAccount {
	return context.outputState.OutputAccounts
}

// DeleteOutputAccount removes the given address from the output accounts and code updates
func (context *outputContext) DeleteOutputAccount(address []byte) {
	delete(context.outputState.OutputAccounts, string(address))
	delete(context.codeUpdates, string(address))
}

// ReturnData returns the data of the current output state.
func (context *outputContext) ReturnData() [][]byte {
	return context.outputState.ReturnData
}

// ReturnCode returns the code of the current output state
func (context *outputContext) ReturnCode() vmcommon.ReturnCode {
	return context.outputState.ReturnCode
}

// SetReturnCode sets the given return code as the return code for the current output state.
func (context *outputContext) SetReturnCode(returnCode vmcommon.ReturnCode) {
	context.outputState.ReturnCode = returnCode
}

// ReturnMessage returns a string that represents the return message for the current output state.
func (context *outputContext) ReturnMessage() string {
	return context.outputState.ReturnMessage
}

// SetReturnMessage sets the given string as a return message for the current output state.
func (context *outputContext) SetReturnMessage(returnMessage string) {
	context.outputState.ReturnMessage = returnMessage
}

// ClearReturnData reinitializes the return data for the current output state.
func (context *outputContext) ClearReturnData() {
	context.outputState.ReturnData = make([][]byte, 0)
}

// RemoveReturnData removes the return data item located at the specified index
func (context *outputContext) RemoveReturnData(index uint32) {
	returnData := context.outputState.ReturnData
	// #nosec G115
	if index >= uint32(len(returnData)) {
		return
	}
	context.outputState.ReturnData = append(returnData[:index], returnData[index+1:]...)
}

// Finish appends the given data to the return data of the current output state.
func (context *outputContext) Finish(data []byte) {
	context.outputState.ReturnData = append(context.outputState.ReturnData, data)
	logOutput.Trace("finish", "data", data)
}

// PrependFinish appends the given data to the return data of the current output state.
func (context *outputContext) PrependFinish(data []byte) {
	context.outputState.ReturnData = append([][]byte{data}, context.outputState.ReturnData...)
}

// DeleteFirstReturnData deletes the first return data, to be used after prepend
func (context *outputContext) DeleteFirstReturnData() {
	if len(context.outputState.ReturnData) > 0 {
		context.outputState.ReturnData = context.outputState.ReturnData[1:]
	}
}

// WriteLogWithIdentifier creates a new LogEntry and appends it to the logs of the current output state.
func (context *outputContext) WriteLogWithIdentifier(address []byte, topics [][]byte, data [][]byte, identifier []byte) {
	if context.host.Runtime().ReadOnly() {
		logOutput.Trace("log entry", "error", "cannot write logs in readonly mode")
		return
	}

	newLogEntry := &vmcommon.LogEntry{
		Address:    address,
		Data:       data,
		Identifier: identifier,
	}
	logOutput.Trace("log entry", "address", address, "data", data)

	if len(topics) == 0 {
		context.outputState.Logs = append(context.outputState.Logs, newLogEntry)
		return
	}

	newLogEntry.Topics = topics

	context.outputState.Logs = append(context.outputState.Logs, newLogEntry)
	logOutput.Trace("log entry", "endpoint", newLogEntry.Identifier, "topics", newLogEntry.Topics)
}

// WriteLog creates a new LogEntry and appends it to the logs of the current output state.
func (context *outputContext) WriteLog(address []byte, topics [][]byte, data [][]byte) {
	context.WriteLogWithIdentifier(address, topics, data, []byte(context.host.Runtime().FunctionName()))
}

// TransferValueOnly will transfer the big.int value and checks if it is possible
func (context *outputContext) TransferValueOnly(destination []byte, sender []byte, value *big.Int, checkPayable bool) error {
	logOutput.Debug("transfer value", "sender", sender, "dest", destination, "value", value)

	if value.Cmp(vmhost.Zero) < 0 {
		logOutput.Trace("transfer value", "error", vmhost.ErrTransferNegativeValue)
		return vmhost.ErrTransferNegativeValue
	}

	if !context.hasSufficientBalance(sender, value) {
		logOutput.Trace("transfer value", "error", vmhost.ErrTransferInsufficientFunds)
		return vmhost.ErrTransferInsufficientFunds
	}

	payable, err := context.host.Blockchain().IsPayable(sender, destination)
	if err != nil {
		logOutput.Trace("transfer value", "error", err)
		return err
	}

	hasValue := value.Cmp(vmhost.Zero) > 0
	if checkPayable && !payable && hasValue {
		logOutput.Trace("transfer value", "error", vmhost.ErrAccountNotPayable)
		return vmhost.ErrAccountNotPayable
	}

	if value.Cmp(vmhost.Zero) > 0 {
		if context.host.Runtime().ReadOnly() {
			return vmhost.ErrInvalidCallOnReadOnlyMode
		}
	}

	// if destination is equal to sender, skip transfer execution in kapp
	// this will register the transfer in the output state
	if bytes.Equal(destination, sender) {
		return nil
	}

	//Using Kapps
	err = context.host.Blockchain().TransferValueOnly(destination, sender, value)
	if err != nil {
		logOutput.Trace("TransferValueOnly error", "err", err.Error())
		return err
	}

	// create sender output account
	_, _ = context.GetOutputAccount(sender)
	// add output transfer to destination account
	destinationAccount, _ := context.GetOutputAccount(destination)
	outputTransfer := vmcommon.OutputTransfer{
		Index:         context.NextOutputTransferIndex(),
		SenderAddress: sender,
		RcvAddr:       destination,
		KDATransfers: vmcommon.KDATransfer{
			KDAValue:     value,
			KDATokenName: kdautils.KLVIdentifier,
		},
	}

	AppendOutputTransfers(destinationAccount, outputTransfer)

	return nil
}

func (context *outputContext) isBackTransferWithoutExecution(sender, destination []byte, input []byte) bool {
	if len(input) != 0 {
		return false
	}
	if !core.IsSmartContractAddress(destination) {
		return false
	}

	vmInput := context.host.Runtime().GetVMInput()

	currentExecutionCallerAddress := vmInput.CallerAddr
	currentExecutionDestinationAddress := vmInput.RecipientAddr

	if !bytes.Equal(currentExecutionCallerAddress, destination) ||
		!bytes.Equal(currentExecutionDestinationAddress, sender) {
		return false
	}

	return true
}

func getExecutionTypeString(callType vm.CallType, isBackTransfer bool) string {
	if isBackTransfer {
		return vmhost.BackTransferString
	}

	switch callType {
	case vm.KDATransferAndExecute:
		return vmhost.TransferAndExecuteString
	}

	return vmhost.DirectCallString
}

// TransferKDA makes the kda/nft transfer and exports the data if it is cross shard
func (context *outputContext) TransferKDA(
	transfersArgs *vmhost.KDATransfersArgs,
	callInput *vmcommon.ContractCallInput,
) (uint64, error) {
	if len(transfersArgs.Transfers) == 0 {
		return 0, vmhost.ErrTransferValueOnKDACall
	}

	isSmartContract := context.host.Blockchain().IsSmartContract(transfersArgs.Destination)
	callType := vm.DirectCall
	isExecution := isSmartContract && callInput != nil
	isBackTransfer := !isExecution && context.isBackTransferWithoutExecution(transfersArgs.Sender, transfersArgs.Destination, nil)

	if callInput != nil {
		callType = callInput.CallType
		transfersArgs.Function = callInput.Function
		transfersArgs.Arguments = callInput.Arguments
	}
	executionType := callType
	if callType == vm.DirectCall && (isExecution || isBackTransfer) {
		executionType = vm.KDATransferAndExecute
	}

	vmOutput, gasConsumedByTransfer, err := context.host.ExecuteKDATransfer(transfersArgs, executionType)
	if err != nil {
		return 0, err
	}

	gasRemaining := uint64(0)

	if callInput != nil && isSmartContract {
		if gasConsumedByTransfer > callInput.GasProvided {
			logOutput.Trace("KDA post-transfer execution", "error", vmhost.ErrNotEnoughGas)
			return 0, vmhost.ErrNotEnoughGas
		}
		gasRemaining = callInput.GasProvided - gasConsumedByTransfer
	}

	if isExecution {
		if gasRemaining > context.host.Metering().GasLeft() {
			logOutput.Trace("KDA post-transfer execution", "error", vmhost.ErrNotEnoughGas)
			return 0, vmhost.ErrNotEnoughGas
		}
	}

	destAcc, _ := context.GetOutputAccount(transfersArgs.Destination)
	outputAcc, ok := vmOutput.OutputAccounts[string(transfersArgs.Destination)]

	if ok {
		AppendOutputTransfers(destAcc, outputAcc.OutputTransfers...)
	}

	context.host.CompleteLogEntriesWithCallType(vmOutput, getExecutionTypeString(executionType, isBackTransfer))

	context.outputState.Logs = append(context.outputState.Logs, vmOutput.Logs...)
	return gasRemaining, nil
}

func AppendOutputTransfers(account *vmcommon.OutputAccount, transfers ...vmcommon.OutputTransfer) {
	account.OutputTransfers = append(account.OutputTransfers, transfers...)
}

func (context *outputContext) hasSufficientBalance(address []byte, value *big.Int) bool {
	senderBalance := context.host.Blockchain().GetBalanceBigInt(address)
	return senderBalance.Cmp(value) >= 0
}

// RemoveNonUpdatedStorage removes non updated storage from output state
func (context *outputContext) RemoveNonUpdatedStorage() {
	for _, outAcc := range context.outputState.OutputAccounts {
		for _, storageUpdate := range outAcc.StorageUpdates {
			if !storageUpdate.Written {
				delete(outAcc.StorageUpdates, string(storageUpdate.Offset))
			}
		}
	}
}

// GetVMOutput updates the current VMOutput and returns it
func (context *outputContext) GetVMOutput() *vmcommon.VMOutput {
	context.removeNonUpdatedCode()

	metering := context.host.Metering()
	context.outputState.GasRemaining = metering.GasLeft()

	err := metering.UpdateGasStateOnSuccess(context.outputState)
	if err != nil {
		return context.CreateVMOutputInCaseOfError(err)
	}

	return context.outputState
}

// DeployCode sets the given code to a an account, and creates a new codeUpdates entry at the accounts address.
func (context *outputContext) DeployCode(input vmhost.CodeDeployInput) {
	newSCAccount, _ := context.GetOutputAccount(input.ContractAddress)
	newSCAccount.Code = input.ContractCode
	newSCAccount.CodeMetadata = input.ContractCodeMetadata
	newSCAccount.CodeDeployerAddress = input.CodeDeployerAddress

	var empty struct{}
	context.codeUpdates[string(input.ContractAddress)] = empty
}

// CreateVMOutputInCaseOfError creates a new vmOutput with the given error set as return message.
func (context *outputContext) CreateVMOutputInCaseOfError(err error) *vmcommon.VMOutput {
	runtime := context.host.Runtime()
	runtime.AddError(err, runtime.FunctionName())

	returnCode := context.resolveReturnCodeFromError(err)
	returnMessage := context.resolveReturnMessageFromError(err)

	vmOutput := &vmcommon.VMOutput{
		GasRemaining:  0,
		ReturnCode:    returnCode,
		ReturnMessage: returnMessage,
	}

	context.host.Metering().UpdateGasStateOnFailure(vmOutput)

	return vmOutput
}

func (context *outputContext) removeNonUpdatedCode() {
	for address, account := range context.outputState.OutputAccounts {
		_, ok := context.codeUpdates[address]
		if !ok {
			account.Code = nil
			account.CodeMetadata = nil
			account.CodeDeployerAddress = nil
		}
	}
}

func (context *outputContext) resolveReturnMessageFromError(err error) string {
	if errors.Is(err, vmhost.ErrSignalError) {
		return context.ReturnMessage()
	}
	if errors.Is(err, vmhost.ErrMemoryLimit) {
		// ErrMemoryLimit will still produce the 'execution failed' message.
		return vmhost.ErrExecutionFailed.Error()
	}
	if len(context.outputState.ReturnMessage) > 0 {
		// Another return message was already set.
		return context.outputState.ReturnMessage
	}

	return err.Error()
}

func (context *outputContext) resolveReturnCodeFromError(err error) vmcommon.ReturnCode {
	if err == nil {
		return vmcommon.Ok
	}

	if errors.Is(err, vmhost.ErrSignalError) {
		return vmcommon.VMUserError
	}
	if errors.Is(err, executor.ErrFuncNotFound) {
		return vmcommon.VMFunctionNotFound
	}
	if errors.Is(err, executor.ErrFunctionNonvoidSignature) {
		return vmcommon.VMFunctionWrongSignature
	}
	if errors.Is(err, executor.ErrInvalidFunction) {
		return vmcommon.VMUserError
	}
	if errors.Is(err, vmhost.ErrInitFuncCalledInRun) {
		return vmcommon.VMUserError
	}
	if errors.Is(err, vmhost.ErrCallBackFuncCalledInRun) {
		return vmcommon.VMUserError
	}
	if errors.Is(err, vmhost.ErrNotEnoughGas) {
		return vmcommon.VMOutOfGas
	}
	if errors.Is(err, vmhost.ErrContractNotFound) {
		return vmcommon.VMContractNotFound
	}
	if errors.Is(err, vmhost.ErrContractInvalid) {
		return vmcommon.VMContractInvalid
	}
	if errors.Is(err, vmhost.ErrUpgradeFailed) {
		return vmcommon.VMUpgradeFailed
	}
	if errors.Is(err, vmhost.ErrTransferInsufficientFunds) {
		return vmcommon.VMOutOfFunds
	}

	return vmcommon.VMExecutionFailed
}

// AddToActiveState merges the given vmOutput with the outputState.
func (context *outputContext) AddToActiveState(rightOutput *vmcommon.VMOutput) {
	mergeVMOutputsConditionally(context.outputState, rightOutput, true)
}

// NextOutputTransferIndex returns next available output transfer index
func (context *outputContext) NextOutputTransferIndex() uint32 {
	index := context.crtTransferIndex
	context.crtTransferIndex++
	return index
}

// GetCrtTransferIndex returns the current output transfer index
func (context *outputContext) GetCrtTransferIndex() uint32 {
	return context.crtTransferIndex
}

// SetCrtTransferIndex sets the current output transfer index
func (context *outputContext) SetCrtTransferIndex(index uint32) {
	context.crtTransferIndex = index
}

func mergeVMOutputs(leftOutput *vmcommon.VMOutput, rightOutput *vmcommon.VMOutput) {
	mergeVMOutputsConditionally(leftOutput, rightOutput, false)
}

func mergeVMOutputsConditionally(leftOutput *vmcommon.VMOutput, rightOutput *vmcommon.VMOutput, mergeAllTransfers bool) {
	if leftOutput.OutputAccounts == nil {
		leftOutput.OutputAccounts = make(map[string]*vmcommon.OutputAccount)
	}

	for _, rightAccount := range rightOutput.OutputAccounts {
		leftAccount, ok := leftOutput.OutputAccounts[string(rightAccount.Address)]
		if !ok {
			leftAccount = &vmcommon.OutputAccount{}
			leftOutput.OutputAccounts[string(rightAccount.Address)] = leftAccount
		}
		mergeOutputAccounts(leftAccount, rightAccount, mergeAllTransfers)
	}

	leftOutput.Logs = append(leftOutput.Logs, rightOutput.Logs...)
	leftOutput.ReturnData = append(leftOutput.ReturnData, rightOutput.ReturnData...)
	leftOutput.GasRemaining = rightOutput.GasRemaining
	leftOutput.ReturnCode = rightOutput.ReturnCode
	leftOutput.ReturnMessage = rightOutput.ReturnMessage

	leftOutput.DeletedAccounts = append(leftOutput.DeletedAccounts, rightOutput.DeletedAccounts...)
}

func mergeOutputAccounts(
	leftAccount *vmcommon.OutputAccount,
	rightAccount *vmcommon.OutputAccount,
	mergeAllTransfers bool,
) {
	if len(rightAccount.Address) != 0 {
		leftAccount.Address = rightAccount.Address
	}

	mergeStorageUpdates(leftAccount, rightAccount)

	if len(rightAccount.Code) > 0 {
		leftAccount.Code = rightAccount.Code
	}
	if len(rightAccount.CodeMetadata) > 0 {
		leftAccount.CodeMetadata = rightAccount.CodeMetadata
	}

	mergeTransfers(leftAccount, rightAccount, mergeAllTransfers)

	leftAccount.GasUsed = rightAccount.GasUsed

	if rightAccount.CodeDeployerAddress != nil {
		leftAccount.CodeDeployerAddress = rightAccount.CodeDeployerAddress
	}

	if rightAccount.BytesAddedToStorage > leftAccount.BytesAddedToStorage {
		leftAccount.BytesAddedToStorage = rightAccount.BytesAddedToStorage
	}
	if rightAccount.BytesDeletedFromStorage > leftAccount.BytesDeletedFromStorage {
		leftAccount.BytesDeletedFromStorage = rightAccount.BytesDeletedFromStorage
	}
}

func mergeTransfers(leftAccount *vmcommon.OutputAccount, rightAccount *vmcommon.OutputAccount, mergeAllTransfers bool) {
	leftTransfers := leftAccount.OutputTransfers
	rightTransfers := rightAccount.OutputTransfers

	lenLeftTransfers := len(leftTransfers)
	lenRightTransfers := len(rightTransfers)

	if mergeAllTransfers {
		leftTransfers = append(leftTransfers, rightTransfers...)
	} else if lenRightTransfers > lenLeftTransfers {
		leftTransfers = append(leftTransfers, rightTransfers[lenLeftTransfers:]...)
	}

	leftAccount.OutputTransfers = leftTransfers
}

func mergeStorageUpdates(
	leftAccount *vmcommon.OutputAccount,
	rightAccount *vmcommon.OutputAccount,
) {
	if leftAccount.StorageUpdates == nil {
		leftAccount.StorageUpdates = make(map[string]*vmcommon.StorageUpdate)
	}
	for key, update := range rightAccount.StorageUpdates {
		leftAccount.StorageUpdates[key] = update
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (context *outputContext) IsInterfaceNil() bool {
	return context == nil
}
