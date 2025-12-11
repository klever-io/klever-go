package kapp

import (
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data"
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
	returnData     [][]byte
	tx             data.TransactionHandler
	gasLimit       uint64
	isScSimulation bool
	executionTime  time.Duration
}

// ArgsNewKAppContext holds the arguments needed to create the KApp Context
type ArgsNewKAppContext struct {
	OriginalSender []byte
	ContractID     int
	ContractType   transaction.TXContract_ContractType
	Block          *block.Block
	TxHash         []byte
	TX             data.TransactionHandler
	IsScSimulation bool
}

type ReceiptSlice []*transaction.Transaction_Receipt

func (r *ReceiptSlice) Add(receipt *transaction.Transaction_Receipt) {
	*r = append(*r, receipt)
}

func (r *ReceiptSlice) Get() []*transaction.Transaction_Receipt {
	return append([]*transaction.Transaction_Receipt{}, *r...)
}

func (r *ReceiptSlice) GetByType(receiptType int8) []*transaction.Transaction_Receipt {
	var filtered []*transaction.Transaction_Receipt
	for _, receipt := range *r {
		if len(receipt.Data) > 0 && len(receipt.Data[0]) > 0 && int8(receipt.Data[0][0]) == receiptType {
			filtered = append(filtered, receipt)
		}
	}
	return filtered
}

func (r *ReceiptSlice) GetPreserved() []*transaction.Transaction_Receipt {
	var filtered []*transaction.Transaction_Receipt
	for _, receipt := range *r {
		if len(receipt.Data) > 0 && len(receipt.Data[0]) > 0 && receipt.Data[0][0] >= SystemReceiptTypeStart {
			filtered = append(filtered, receipt)
		}
	}
	return filtered
}

func (r *ReceiptSlice) AddError(contractID int, params ...string) {
	data := make([][]byte, len(params)+1)
	data[0] = []byte{byte(ReceiptTypeError), byte(contractID)}
	for i, param := range params {
		data[i+1] = []byte(param)
	}
	*r = append(*r, &transaction.Transaction_Receipt{Data: data})
}

func NewKappContext(args ArgsNewKAppContext) KappContext {

	// create empty TX if nil
	if args.TX == nil {
		args.TX = &transaction.Transaction{}
	}

	receipts := make(ReceiptSlice, 0)
	ctx := &kappContext{
		originalSender: append([]byte{}, args.OriginalSender...),
		contractID:     args.ContractID,
		receipts:       &receipts,
		block:          args.Block,
		txHash:         args.TxHash,
		returnData:     make([][]byte, 0),
		tx:             args.TX,
		gasLimit:       args.TX.GetGasLimit(),
		isScSimulation: args.IsScSimulation,
	}

	return ctx
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
	return k.tx.GetNonce()
}

func (k *kappContext) IsScSimulation() bool {
	return k.isScSimulation
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

func (k *kappContext) GetExecData() []byte {
	data := k.tx.GetData()
	if k.contractID < 0 || len(data) <= k.contractID {
		return nil
	}

	return append([]byte{}, data[k.contractID]...)
}

func (k *kappContext) SubGasUsed(gasUsed uint64) error {
	if gasUsed > k.gasLimit {
		return common.ErrNotEnoughGas
	}

	k.gasLimit -= gasUsed
	return nil
}

func (k *kappContext) GetGasLimit() uint64 {
	return k.gasLimit
}

func (k *kappContext) SetExecutionTime(duration time.Duration) {
	k.executionTime = duration
}

func (k *kappContext) GetExecutionTime() time.Duration {
	return k.executionTime
}
