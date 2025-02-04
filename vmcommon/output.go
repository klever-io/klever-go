package vmcommon

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/tools/check"

	"github.com/klever-io/klever-go/data/vm"
)

// StorageUpdate represents a change in the account storage (insert, update or delete)
// Note: current implementation might also return unmodified storage entries.
type StorageUpdate struct {
	// Offset is the storage key.
	Offset []byte

	// Data is the new storage value.
	// Zero indicates missing data for the key (or even a missing key),
	// therefore a value of zero here indicates that
	// the storage map entry with the given key can be deleted.
	Data []byte

	// Written represents that this storage was change and needs to be persisted
	// into the chain
	Written bool
}

// OutputAccount shows the state of an account after contract execution.
// It can be an existing account or a new account created by the transaction.
// Note: the current implementation might also return unmodified accounts.
type OutputAccount struct {
	// Address is the public key of the account.
	Address []byte

	// StorageUpdates is a map containing pointers to StorageUpdate structs,
	// indexed with strings produced by `string(StorageUpdate.Offset)`, for fast
	// access by the Offset of the StorageUpdate. These StorageUpdate structs
	// will be processed by the Node to modify the storage of the SmartContract.
	// Please note that it is likely that not all existing account storage keys
	// show up here.
	StorageUpdates map[string]*StorageUpdate

	// Code is the assembled code of a smart contract account.
	// This field will be populated when a new SC must be created after the transaction.
	Code []byte

	// CodeMetadata is the metadata of the code
	// Like "Code", this field will be populated when a new SC must be created after the transaction.
	CodeMetadata []byte

	// CodeDeployerAddress will be populated in case of contract deployment or upgrade (both direct and indirect)
	CodeDeployerAddress []byte

	// OutputTransfers represents transfer executed in the contract
	OutputTransfers []OutputTransfer

	// GasUsed is the amount of gas used by the SC execution.
	GasUsed uint64

	// BytesAddedToStorage for this output account
	BytesAddedToStorage uint64

	// BytesDeletedFromStorage for this output account
	BytesDeletedFromStorage uint64
}

func (oa *OutputAccount) Clone() *OutputAccount {
	clonedTransfers := make([]OutputTransfer, len(oa.OutputTransfers))

	for i, transfer := range oa.OutputTransfers {
		clonedTransfers[i] = *transfer.Clone()
	}

	clonedStorageUpdates := make(map[string]*StorageUpdate, len(oa.StorageUpdates))
	for key, value := range oa.StorageUpdates {
		if value != nil {
			clonedStorageUpdates[key] = &StorageUpdate{
				Offset:  append([]byte{}, value.Offset...),
				Data:    append([]byte{}, value.Data...),
				Written: value.Written,
			}
		}
	}

	cloned := &OutputAccount{
		Address:                 append([]byte{}, oa.Address...),
		StorageUpdates:          clonedStorageUpdates,
		Code:                    append([]byte{}, oa.Code...),
		CodeMetadata:            append([]byte{}, oa.CodeMetadata...),
		CodeDeployerAddress:     append([]byte{}, oa.CodeDeployerAddress...),
		OutputTransfers:         clonedTransfers,
		GasUsed:                 oa.GasUsed,
		BytesAddedToStorage:     oa.BytesAddedToStorage,
		BytesDeletedFromStorage: oa.BytesDeletedFromStorage,
	}

	return cloned
}

// OutputTransfer contains the fields with result
type OutputTransfer struct {
	// Index of the transfer
	Index uint32
	// SenderAddress is the actual sender for the given output transfer, this is needed when
	// contract A calls contract B and contract B does the transfers
	SenderAddress []byte
	// Recipient address
	RcvAddr []byte
	// Parsed KDA Transfer
	KDATransfers KDATransfer
}

func (ot *OutputTransfer) Clone() *OutputTransfer {
	cloned := &OutputTransfer{
		Index:         ot.Index,
		SenderAddress: make([]byte, 0),
		RcvAddr:       make([]byte, 0),
		KDATransfers:  ot.KDATransfers.Clone(),
	}
	cloned.SenderAddress = append(cloned.SenderAddress, ot.SenderAddress...)
	cloned.RcvAddr = append(cloned.RcvAddr, ot.RcvAddr...)

	return cloned
}

// LogEntry represents an entry in the contract execution log.
type LogEntry struct {
	// Identifier is the identifier of the log entry.
	Identifier []byte
	// Address is the address involved in the log entry.
	Address []byte
	// Topics contains the events of the log entry.
	Topics [][]byte
	// Data is the data of the log entry.
	Data [][]byte
	// IsSystemLog is a flag that indicates if the log entry is a system log entry.
	IsSystemLog bool
}

// VMOutput is the return data and final account state after a SC execution.
type VMOutput struct {
	// ReturnData is the function call returned result.
	// This value does not influence the account state in any way.
	// The value should be accessible in a UI.
	// ReturnData is part of the transaction receipt.
	ReturnData [][]byte

	// ReturnCode is the function call error code.
	// If it is not `Ok`, the transaction failed in some way - gas is, however, consumed anyway.
	// This value does not influence the account state in any way.
	// The value should be accessible to a UI.
	// ReturnCode is part of the transaction receipt.
	ReturnCode ReturnCode

	// ReturnMessage is a message set by the SmartContract, destined for the
	// caller
	ReturnMessage string

	// GasRemaining = VMInput.GasProvided - gas used.
	// It is necessary to compute how much to charge the sender for the transaction.
	GasRemaining uint64

	// OutputAccounts contains data about all accounts changed as a result of the
	// Transaction. It is a map containing pointers to OutputAccount structs,
	// indexed with strings produced by `string(OutputAccount.Address)`, for fast
	// access by the Address of the OutputAccount.
	// This information tells the Node how to update the account data.
	// It can contain new accounts or existing changed accounts.
	// Note: the current implementation might also retrieve accounts that were not changed.
	OutputAccounts map[string]*OutputAccount

	// DeletedAccounts is a list of public keys of accounts that need to be deleted
	// as a result of the transaction.
	DeletedAccounts [][]byte

	// Logs is a list of event data logged by the vmcommon.
	// Smart contracts can choose to log certain events programmatically.
	// There are 3 main use cases for events and logs:
	// 1. smart contract return values for the user interface;
	// 2. synchronous triggers with data;
	// 3. a cheaper form of storage (e.g. storing historical data that can be rendered by the frontend).
	// The logs should be accessible to the UI.
	// The logs are part of the transaction receipt.
	Logs []*LogEntry
}

// GetFirstReturnData is a helper function that returns the first ReturnData of VMOutput, interpreted as specified.
func (vmOutput *VMOutput) GetFirstReturnData(asType vm.ReturnDataKind) (interface{}, error) {
	if len(vmOutput.ReturnData) == 0 {
		return nil, fmt.Errorf("no return data")
	}

	returnData := vmOutput.ReturnData[0]

	switch asType {
	case vm.AsBigInt:
		return big.NewInt(0).SetBytes(returnData), nil
	case vm.AsBigIntString:
		return big.NewInt(0).SetBytes(returnData).String(), nil
	case vm.AsString:
		return string(returnData), nil
	case vm.AsHex:
		return hex.EncodeToString(returnData), nil
	}

	return nil, fmt.Errorf("can't interpret return data")
}

// MergeOutputAccounts merges the given account into the current one
func (o *OutputAccount) MergeOutputAccounts(outAcc *OutputAccount) {
	if len(outAcc.Address) != 0 {
		o.Address = outAcc.Address
	}

	o.MergeStorageUpdates(outAcc)

	if len(outAcc.Code) > 0 {
		o.Code = outAcc.Code
	}
	if len(outAcc.CodeMetadata) > 0 {
		o.CodeMetadata = outAcc.CodeMetadata
	}

	lenLeftOutTransfers := len(o.OutputTransfers)
	lenRightOutTransfers := len(outAcc.OutputTransfers)
	if lenRightOutTransfers > lenLeftOutTransfers {
		o.OutputTransfers = append(o.OutputTransfers, outAcc.OutputTransfers[lenLeftOutTransfers:]...)
	}

	o.GasUsed = outAcc.GasUsed

	if outAcc.CodeDeployerAddress != nil {
		o.CodeDeployerAddress = outAcc.CodeDeployerAddress
	}
}

// MergeStorageUpdates will copy all the storage updates from the given output account
func (o *OutputAccount) MergeStorageUpdates(outAcc *OutputAccount) {
	if o.StorageUpdates == nil {
		o.StorageUpdates = make(map[string]*StorageUpdate)
	}
	for key, update := range outAcc.StorageUpdates {
		o.StorageUpdates[key] = update
	}
}

// MaxLengthForValueToOptTransfer defines the maximum length for value to optimize cross shard transfer
const MaxLengthForValueToOptTransfer = 32

// FormatLogDataForCall prepares Data field for a LogEntry
func FormatLogDataForCall(callType string, functionName string, functionArgs [][]byte) [][]byte {
	data := make([][]byte, 0)
	data = append(data, []byte(callType))
	data = append(data, []byte(functionName))
	data = append(data, functionArgs...)
	return data
}

// ReindexTransfers from VMOutput
func (vmOutput *VMOutput) ReindexTransfers(nextIndexProvider NextOutputTransferIndexProvider) error {
	if check.IfNil(nextIndexProvider) {
		return ErrNilTransferIndexer
	}

	reindexed := false
	crtIndex := nextIndexProvider.GetCrtTransferIndex() - 1
	for _, account := range vmOutput.OutputAccounts {
		for transferIdx, transfer := range account.OutputTransfers {
			if transfer.Index == 0 {
				return ErrTransfersNotIndexed
			}
			account.OutputTransfers[transferIdx].Index = transfer.Index + crtIndex
			reindexed = true
		}
	}
	if reindexed {
		nextIndexProvider.SetCrtTransferIndex(vmOutput.GetNextAvailableOutputTransferIndex())
	}

	return nil
}

// GetNextAvailableOutputTransferIndex returns the maximum output transfer index
func (vmOutput *VMOutput) GetNextAvailableOutputTransferIndex() uint32 {
	maxTransferIndex := uint32(0)
	for _, account := range vmOutput.OutputAccounts {
		for _, transfer := range account.OutputTransfers {
			if transfer.Index > maxTransferIndex {
				maxTransferIndex = transfer.Index
			}
		}
	}

	return maxTransferIndex + 1
}

// ComputeTotalGasConsumed returns the total gas consumed by SC execution from logs
func (vmOutput *VMOutput) ComputeTotalGasConsumed() *big.Int {
	totalGasConsumed := big.NewInt(0)
	for _, log := range vmOutput.Logs {
		if log.IsSystemLog && string(log.Identifier) == core.TotalConsumedGasString {
			if len(log.Topics) > 0 {
				totalGasConsumed.Add(totalGasConsumed, big.NewInt(0).SetBytes(log.Topics[0]))
			}
		}
	}

	return totalGasConsumed
}

// IsInterfaceNil returns true if there is no value under the interface
func (vmOutput *VMOutput) IsInterfaceNil() bool {
	return vmOutput == nil
}
