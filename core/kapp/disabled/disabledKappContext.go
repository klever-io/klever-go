package disabled

import (
	"time"

	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/transaction"
)

var _ kapp.KappContext = (*kappContext)(nil)

type kappContext struct {
	originalSender []byte
	contractID     int
	receipts       *ReceiptSlice
	block          *block.Block
	txHash         []byte
	txNonce        uint64
	returnData     [][]byte
}

type ReceiptSlice []*transaction.Transaction_Receipt

func (r *ReceiptSlice) Add(receipt *transaction.Transaction_Receipt) {
	*r = append(*r, receipt)
}

func (r *ReceiptSlice) Get() []*transaction.Transaction_Receipt {
	return append([]*transaction.Transaction_Receipt{}, *r...)
}

func NewDisabledKappContext() kapp.KappContext {
	receipts := make(ReceiptSlice, 0)
	return &kappContext{
		originalSender: make([]byte, 0),
		contractID:     0,
		receipts:       &receipts,
		block:          &block.Block{},
		txHash:         make([]byte, 0),
		txNonce:        0,
		returnData:     make([][]byte, 0),
	}
}

func (k *kappContext) OriginalSender() []byte {
	return k.originalSender
}

func (k *kappContext) ContractID() int {
	return 0
}

func (k *kappContext) ContractType() transaction.TXContract_ContractType {
	return 0
}

func (k *kappContext) Receipts() kapp.ReceiptsContext {
	return k.receipts
}

func (k *kappContext) Block() *block.Block {
	return &block.Block{}
}

func (k *kappContext) TxHash() []byte {
	return []byte{}
}

func (k *kappContext) TxNonce() uint64 {
	return 0
}

func (k *kappContext) IsScSimulation() bool {
	return false
}

func (k *kappContext) SetContractID(_ int) {
}

func (k *kappContext) SetSender(_ []byte) {
}

func (k *kappContext) AddReturnData(data []byte) {
}

func (k *kappContext) SetReturnData(data [][]byte) {
}

func (k *kappContext) GetAndClearReturnData() [][]byte {
	return [][]byte{}
}

func (k *kappContext) GetExecData() []byte {
	return nil
}

func (k *kappContext) GetGasLimit() uint64 {
	return 0
}

func (k *kappContext) SubGasUsed(_ uint64) error {
	return nil
}

func (k *kappContext) SetExecutionTime(_ time.Duration) {
}

func (k *kappContext) GetExecutionTime() time.Duration {
	return 0
}
