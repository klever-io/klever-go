package vmcommon

import "github.com/klever-io/klever-go/data/transaction"

type ReturnCode transaction.Transaction_TXResultCode

// import "fmt"

// ReturnCode is an enum with the possible error codes returned by the VM
// type ReturnCode int
func (rc ReturnCode) String() string {
	return rc.ResultCode().String()
}

// ResultCode returns the result code as a transaction.Transaction_TXResultCode
func (rc ReturnCode) ResultCode() transaction.Transaction_TXResultCode {
	return transaction.Transaction_TXResultCode(rc)
}

const (
	Ok                       = ReturnCode(transaction.Transaction_Ok)
	VMFunctionNotFound       = ReturnCode(transaction.Transaction_VMFunctionNotFound)
	VMFunctionWrongSignature = ReturnCode(transaction.Transaction_VMFunctionWrongSignature)
	VMContractNotFound       = ReturnCode(transaction.Transaction_ContractNotFound)
	VMUserError              = ReturnCode(transaction.Transaction_VMUserError)
	VMOutOfGas               = ReturnCode(transaction.Transaction_VMOutOfGas)
	VMAccountCollision       = ReturnCode(transaction.Transaction_VMAccountCollision)
	VMOutOfFunds             = ReturnCode(transaction.Transaction_OutOfFunds)
	VMCallStackOverFlow      = ReturnCode(transaction.Transaction_VMCallStackOverFlow)
	VMContractInvalid        = ReturnCode(transaction.Transaction_ContractInvalid)
	VMExecutionPanicked      = ReturnCode(transaction.Transaction_VMExecutionPanicked)
	VMExecutionFailed        = ReturnCode(transaction.Transaction_VMExecutionFailed)
	VMUpgradeFailed          = ReturnCode(transaction.Transaction_VMUpgradeFailed)
	VMSimulateFailed         = ReturnCode(transaction.Transaction_VMSimulateFailed)
)
