package parsers

import (
	"math/big"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/dkda"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

// MinArgsForKDATransfer defines the minimum arguments needed for an kda transfer
const MinArgsForKDATransfer = 2

// MinArgsForKDANFTTransfer defines the minimum arguments needed for an nft transfer
const MinArgsForKDANFTTransfer = 4

// MinArgsForMultiKDANFTTransfer defines the minimum arguments needed for a multi transfer
const MinArgsForMultiKDANFTTransfer = 4

// ArgsPerTransfer defines the number of arguments per transfer in multi transfer
const ArgsPerTransfer = 3

type kdaTransferParser struct {
	marshaller vmcommon.Marshalizer
}

// NewKDATransferParser creates a new kda transfer parser
func NewKDATransferParser(
	marshaller vmcommon.Marshalizer,
) (*kdaTransferParser, error) {
	if check.IfNil(marshaller) {
		return nil, ErrNilMarshalizer
	}

	return &kdaTransferParser{marshaller: marshaller}, nil
}

// ParseKDATransfers returns the list of kda transfers, the callFunction and callArgs from the given arguments
func (e *kdaTransferParser) ParseKDATransfers(
	rcvAddr []byte,
	args [][]byte,
) (*vmcommon.ParsedKDATransfers, error) {
	return e.parseMultiKDANFTTransfer(rcvAddr, args)
}

func (e *kdaTransferParser) parseMultiKDANFTTransfer(rcvAddr []byte, args [][]byte) (*vmcommon.ParsedKDATransfers, error) {
	if len(args) < MinArgsForMultiKDANFTTransfer {
		return nil, ErrNotEnoughArguments
	}
	kdaTransfers := &vmcommon.ParsedKDATransfers{
		RcvAddr:      rcvAddr,
		CallArgs:     make([][]byte, 0),
		CallFunction: "",
	}

	numOfTransfer := big.NewInt(0).SetBytes(args[0])
	startIndex := uint64(1)
	isTxAtSender := false

	isFirstArgumentAnAddress := len(args[0]) == len(rcvAddr) && !numOfTransfer.IsUint64()
	if isFirstArgumentAnAddress {
		kdaTransfers.RcvAddr = args[0]
		numOfTransfer.SetBytes(args[1])
		startIndex = 2
		isTxAtSender = true
	}

	minLenArgs := ArgsPerTransfer*numOfTransfer.Uint64() + startIndex
	if uint64(len(args)) < minLenArgs {
		return nil, ErrNotEnoughArguments
	}

	if uint64(len(args)) > minLenArgs {
		kdaTransfers.CallFunction = string(args[minLenArgs])
	}
	if uint64(len(args)) > minLenArgs+1 {
		kdaTransfers.CallArgs = append(kdaTransfers.CallArgs, args[minLenArgs+1:]...)
	}

	var err error
	kdaTransfers.KDATransfers = make([]*vmcommon.KDATransfer, numOfTransfer.Uint64())
	for i := uint64(0); i < numOfTransfer.Uint64(); i++ {
		tokenStartIndex := startIndex + i*ArgsPerTransfer
		kdaTransfers.KDATransfers[i], err = e.createNewKDATransfer(tokenStartIndex, args, isTxAtSender)
		if err != nil {
			return nil, err
		}
	}

	return kdaTransfers, nil
}

func (e *kdaTransferParser) createNewKDATransfer(
	tokenStartIndex uint64,
	args [][]byte,
	isTxAtSender bool,
) (*vmcommon.KDATransfer, error) {
	kdaTransfer := &vmcommon.KDATransfer{
		KDAValue:      big.NewInt(0).SetBytes(args[tokenStartIndex+2]),
		KDATokenName:  args[tokenStartIndex],
		KDATokenType:  uint32(core.Fungible),
		KDATokenNonce: big.NewInt(0).SetBytes(args[tokenStartIndex+1]).Uint64(),
	}
	if kdaTransfer.KDATokenNonce > 0 {
		kdaTransfer.KDATokenType = uint32(core.NonFungible)

		if !isTxAtSender && len(args[tokenStartIndex+2]) > vmcommon.MaxLengthForValueToOptTransfer {
			transferKDAData := &dkda.KDigitalToken{}
			err := e.marshaller.Unmarshal(transferKDAData, args[tokenStartIndex+2])
			if err != nil {
				return nil, err
			}
			kdaTransfer.KDAValue.Set(big.NewInt(transferKDAData.Value))
		}
	}

	return kdaTransfer, nil
}

// IsInterfaceNil returns true if underlying object is nil
func (e *kdaTransferParser) IsInterfaceNil() bool {
	return e == nil
}
