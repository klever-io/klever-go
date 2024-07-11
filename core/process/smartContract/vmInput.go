package smartContract

import (
	"math/big"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/vmcommon"
)

func (sc *scProcessor) createVMDeployInput(
	tx data.TransactionHandler,
	transferValues map[string]*transaction.CallValue,
	gasLimit uint64,
) (*vmcommon.ContractCreateInput, []byte, error) {
	deployData, err := sc.argsParser.ParseDeployData(string(tx.GetDataWithIdx(0)))
	if err != nil {
		return nil, nil, err
	}

	vmCreateInput := &vmcommon.ContractCreateInput{}
	vmCreateInput.ContractCode = deployData.Code
	// when executing SC deploys we should always apply the flags
	vmCreateInput.ContractCodeMetadata = deployData.CodeMetadata.ToBytes()
	vmCreateInput.VMInput = vmcommon.VMInput{}
	err = sc.initializeVMInputFromTx(&vmCreateInput.VMInput, tx, transferValues, gasLimit)
	if err != nil {
		return nil, nil, err
	}

	vmCreateInput.VMInput.Arguments = deployData.Arguments

	return vmCreateInput, deployData.VMType, nil
}

func (sc *scProcessor) initializeVMInputFromTx(
	vmInput *vmcommon.VMInput,
	tx data.TransactionHandler,
	callValue map[string]*transaction.CallValue,
	gasLimit uint64,
) error {
	vmInput.CallerAddr = tx.GetSender()
	vmInput.KDATransfers = make([]*vmcommon.KDATransfer, 0)
	for kda, cvwr := range callValue {
		// validate token name and extract nonce if any
		// nil or empty name will be taken as KLV
		id, nonce, assetType, err := kdautils.ExtractAssetIDAndNonce([]byte(kda))
		if err != nil {
			return err
		}

		vmInput.KDATransfers = append(vmInput.KDATransfers, &vmcommon.KDATransfer{
			KDAValue:      new(big.Int).Set(big.NewInt(cvwr.Amount)),
			KDATokenName:  id,
			KDATokenNonce: nonce,
			KDATokenType:  uint32(assetType),
			KDARoyalties:  cvwr.KDARoyalties,
			KLVRoyalties:  cvwr.KLVRoyalties,
		})
	}

	vmInput.GasProvided = gasLimit

	return nil
}

func (sc *scProcessor) createVMCallInput(
	tx data.TransactionHandler,
	contractAddress []byte,
	callValue map[string]*transaction.CallValue,
	gasLimit uint64,
	contractID int,
	txHash []byte,
) (*vmcommon.ContractCallInput, error) {
	// check if any input data is provided
	if len(tx.GetData()) <= contractID {
		return nil, common.ErrNilInputData
	}

	txData := string(tx.GetRaw().GetData()[contractID])

	function, arguments, err := sc.argsParser.ParseCallData(txData)
	if err != nil {
		return nil, err
	}

	vmCallInput := &vmcommon.ContractCallInput{}
	vmCallInput.VMInput = vmcommon.VMInput{}
	vmCallInput.RecipientAddr = contractAddress
	vmCallInput.Function = function
	vmCallInput.CurrentTxHash = txHash
	vmCallInput.OriginalTxHash = txHash

	err = sc.initializeVMInputFromTx(&vmCallInput.VMInput, tx, callValue, gasLimit)
	if err != nil {
		return nil, err
	}

	// set all initialized transfer inputs as executed
	for _, kdaTransfer := range vmCallInput.KDATransfers {
		kdaTransfer.SetExecuted()
	}

	vmCallInput.VMInput.Arguments = arguments
	if vmCallInput.GasProvided > gasLimit {
		return nil, process.ErrInvalidVMInputGasComputation
	}

	return vmCallInput, nil
}
