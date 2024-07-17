package smartContract

import (
	"math/big"

	"github.com/klever-io/klever-go/common/types"

	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/vmcommon"
)

func (sc *scProcessor) createVMDeployInput(
	ctx kapp.KappContext,
	gasLimit uint64,
	transferValues map[string]*transaction.CallValue,
) (*vmcommon.ContractCreateInput, []byte, error) {
	deployData, err := sc.argsParser.ParseDeployData(string(ctx.GetExecData()))
	if err != nil {
		return nil, nil, err
	}

	vmCreateInput := &vmcommon.ContractCreateInput{}
	vmCreateInput.ContractCode = deployData.Code
	// when executing SC deploys we should always apply the flags
	vmCreateInput.ContractCodeMetadata = deployData.CodeMetadata.ToBytes()
	vmCreateInput.VMInput = vmcommon.VMInput{}
	err = sc.initializeVMInputFromTx(ctx.OriginalSender(), &vmCreateInput.VMInput, transferValues)
	if err != nil {
		return nil, nil, err
	}

	vmCreateInput.VMInput.GasProvided = gasLimit
	vmCreateInput.VMInput.Arguments = deployData.Arguments

	return vmCreateInput, deployData.VMType, nil
}

func (sc *scProcessor) initializeVMInputFromTx(
	sender []byte,
	vmInput *vmcommon.VMInput,
	callValue map[string]*transaction.CallValue,
) error {
	vmInput.CallerAddr = sender
	vmInput.KDATransfers = make([]*vmcommon.KDATransfer, 0)

	dm := types.NewDeterministicMap(callValue)
	err := dm.Each(func(kda string, cvwr *transaction.CallValue) error {
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
		return nil
	})

	return err
}

func (sc *scProcessor) createVMCallInput(
	ctx kapp.KappContext,
	gasLimit uint64,
	contractAddress []byte,
	callValue map[string]*transaction.CallValue,
) (*vmcommon.ContractCallInput, error) {
	txData := string(ctx.GetExecData())

	function, arguments, err := sc.argsParser.ParseCallData(txData)
	if err != nil {
		return nil, err
	}

	vmCallInput := &vmcommon.ContractCallInput{}
	vmCallInput.VMInput = vmcommon.VMInput{}
	vmCallInput.RecipientAddr = contractAddress
	vmCallInput.Function = function
	vmCallInput.CurrentTxHash = ctx.TxHash()
	vmCallInput.OriginalTxHash = ctx.TxHash()

	err = sc.initializeVMInputFromTx(ctx.OriginalSender(), &vmCallInput.VMInput, callValue)
	if err != nil {
		return nil, err
	}

	vmCallInput.VMInput.GasProvided = gasLimit
	vmCallInput.VMInput.Arguments = arguments

	// set all initialized transfer inputs as executed
	for _, kdaTransfer := range vmCallInput.KDATransfers {
		kdaTransfer.SetExecuted()
	}

	return vmCallInput, nil
}
