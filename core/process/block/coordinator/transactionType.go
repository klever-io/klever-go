package coordinator

import (
	"bytes"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

var _ process.TxTypeHandler = (*txTypeHandler)(nil)

type txTypeHandler struct {
	pubkeyConv        core.PubkeyConverter
	builtInFunctions  vmcommon.BuiltInFunctionContainer
	argumentParser    process.CallArgumentsParser
	kdaTransferParser vmcommon.KDATransferParser
	forkController    core.ForkController
}

// ArgNewTxTypeHandler defines the arguments needed to create a new tx type handler
type ArgNewTxTypeHandler struct {
	PubkeyConverter   core.PubkeyConverter
	BuiltInFunctions  vmcommon.BuiltInFunctionContainer
	ArgumentParser    process.CallArgumentsParser
	KDATransferParser vmcommon.KDATransferParser
	ForkController    core.ForkController
}

// NewTxTypeHandler creates a transaction type handler
func NewTxTypeHandler(
	args ArgNewTxTypeHandler,
) (*txTypeHandler, error) {
	if check.IfNil(args.PubkeyConverter) {
		return nil, process.ErrNilPubkeyConverter
	}
	if check.IfNil(args.ArgumentParser) {
		return nil, process.ErrNilArgumentParser
	}
	if check.IfNil(args.BuiltInFunctions) {
		return nil, process.ErrNilBuiltInFunction
	}
	//if check.IfNil(args.KDATransferParser) {
	//	return nil, process.ErrNilKDATransferParser
	//}
	if check.IfNil(args.ForkController) {
		return nil, process.ErrNilEnableEpochsHandler
	}

	tc := &txTypeHandler{
		pubkeyConv:        args.PubkeyConverter,
		argumentParser:    args.ArgumentParser,
		builtInFunctions:  args.BuiltInFunctions,
		kdaTransferParser: args.KDATransferParser,
		forkController:    args.ForkController,
	}

	return tc, nil
}

// ComputeTransactionType calculates the transaction type
func (tth *txTypeHandler) ComputeTransactionType(ctx kapp.KappContext, tx data.TransactionHandler, tc data.SmartContractHandler) (process.TransactionType, process.TransactionType) {
	err := tth.checkTxValidity(tx, tc)
	if err != nil {
		return process.InvalidTransaction, process.InvalidTransaction
	}

	isEmptyAddress := tth.isDestAddressEmpty(tc)
	if isEmptyAddress {
		if len(tx.GetData()) > 0 {
			return process.SCDeployment, process.SCDeployment
		}
		return process.InvalidTransaction, process.InvalidTransaction
	}

	if len(tx.GetData()) == 0 {
		return process.MoveBalance, process.MoveBalance
	}

	funcName, _ := tth.getFunctionFromArguments(tx.GetDataWithIdx(0))
	isBuiltInFunction := tth.isBuiltInFunctionCall(funcName)

	if isBuiltInFunction {
		return process.BuiltInFunctionCall, process.BuiltInFunctionCall
	}

	if len(funcName) == 0 {
		return process.MoveBalance, process.MoveBalance
	}

	if core.IsSmartContractAddress(tc.GetAddress()) {
		return process.MoveBalance, process.SCInvoking
	}

	return process.MoveBalance, process.MoveBalance
}

func (tth *txTypeHandler) getFunctionFromArguments(txData []byte) (string, [][]byte) {
	if len(txData) == 0 {
		return "", nil
	}

	function, args, err := tth.argumentParser.ParseData(string(txData))
	if err != nil {
		return "", nil
	}

	return function, args
}

func (tth *txTypeHandler) isBuiltInFunctionCall(functionName string) bool {
	function, err := tth.builtInFunctions.Get(functionName)
	if err != nil {
		return false
	}

	return function.IsActive()
}

func (tth *txTypeHandler) isDestAddressEmpty(tc data.SmartContractHandler) bool {
	isEmptyAddress := bytes.Equal(tc.GetAddress(), make([]byte, tth.pubkeyConv.Len()))
	return isEmptyAddress
}

func (tth *txTypeHandler) checkTxValidity(tx data.TransactionHandler, tc data.SmartContractHandler) error {
	if check.IfNil(tx) || check.IfNil(tc) {
		return process.ErrNilTransaction
	}

	recvAddressIsInvalid := tth.pubkeyConv.Len() != len(tc.GetAddress())
	if recvAddressIsInvalid {
		return process.ErrWrongTransaction
	}

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (tth *txTypeHandler) IsInterfaceNil() bool {
	return tth == nil
}
