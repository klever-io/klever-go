package kapp

import (
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/transaction"
)

var _ KappContext = (*kappContext)(nil)

type kappContext struct {
	originalSender []byte
	contractID     int
	receipts       *ReceiptSlice
	block          *block.Block
	txHash         []byte
	txNonce        uint64
	returnData     [][]byte
}

// ArgsNewKAppContext holds the arguments needed to create the KApp Context
type ArgsNewKAppContext struct {
	OriginalSender []byte
	ContractID     int
	ContractType   transaction.TXContract_ContractType
	Block          *block.Block
	TxHash         []byte
	TxNonce        uint64
	TxData         [][]byte
}

type ReceiptSlice []*transaction.Transaction_Receipt

func (r *ReceiptSlice) Add(receipt *transaction.Transaction_Receipt) {
	*r = append(*r, receipt)
}

func (r *ReceiptSlice) Get() []*transaction.Transaction_Receipt {
	return append([]*transaction.Transaction_Receipt{}, *r...)
}

func NewKappContext(args ArgsNewKAppContext) KappContext {
	receipts := make(ReceiptSlice, 0)
	return &kappContext{
		originalSender: append([]byte{}, args.OriginalSender...),
		contractID:     args.ContractID,
		receipts:       &receipts,
		block:          args.Block,
		txHash:         args.TxHash,
		txNonce:        args.TxNonce,
		returnData:     make([][]byte, 0),
	}
}

func (k *kappContext) OriginalSender() []byte {
	return append([]byte{}, k.originalSender...)
}

func (k *kappContext) ContractID() int {
	return k.contractID
}

func (k *kappContext) Receipts() ReceiptsContext {
	return k.receipts
}

func (k *kappContext) Block() *block.Block {
	return k.block
}

func (k *kappContext) TxHash() []byte {
	return append([]byte{}, k.txHash...)
}

func (k *kappContext) TxNonce() uint64 {
	return k.txNonce
}

func (k *kappContext) SetContractID(id int) {
	k.contractID = id
}

func (k *kappContext) SetReturnData(data [][]byte) {
	// Create a new outer slice with the same length as src
	k.returnData = make([][]byte, len(data))

	// Iterate over each inner slice
	for i, s := range data {
		// Create a new inner slice with the same length as s
		k.returnData[i] = make([]byte, len(s))
		// Copy the bytes from s to dst[i]
		copy(k.returnData[i], s)
	}
}

func (k *kappContext) AddReturnData(data []byte) {
	data_dts := make([]byte, len(data))
	copy(data_dts, data)

	k.returnData = append(k.returnData, data_dts)
}

func (k *kappContext) GetAndClearReturnData() [][]byte {
	// Create a new outer slice with the same length as src
	dst := make([][]byte, len(k.returnData))

	// Iterate over each inner slice
	for i, s := range k.returnData {
		// Create a new inner slice with the same length as s
		dst[i] = make([]byte, len(s))
		// Copy the bytes from s to dst[i]
		copy(dst[i], s)
	}

	k.returnData = make([][]byte, 0)
	return dst
}
