package mock

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/transaction"
)

var _ kapp.ReceiptsContext = (*ReceiptsContextStub)(nil)

// ReceiptsContextStub implements kapp.ReceiptsContext for testing
type ReceiptsContextStub struct {
	receipts []*transaction.Transaction_Receipt
}

// NewReceiptsContextStub creates a new ReceiptsContextStub
func NewReceiptsContextStub() *ReceiptsContextStub {
	return &ReceiptsContextStub{
		receipts: make([]*transaction.Transaction_Receipt, 0),
	}
}

// Get returns all receipts
func (r *ReceiptsContextStub) Get() []*transaction.Transaction_Receipt {
	return r.receipts
}

// GetByType returns receipts that match the given type
func (r *ReceiptsContextStub) GetByType(receiptType int8) []*transaction.Transaction_Receipt {
	var filtered []*transaction.Transaction_Receipt
	for _, receipt := range r.receipts {
		if len(receipt.Data) > 0 && len(receipt.Data[0]) > 0 && int8(receipt.Data[0][0]) == receiptType {
			filtered = append(filtered, receipt)
		}
	}
	return filtered
}

// GetPreserved returns system receipts (type >= kapp.SystemReceiptTypeStart) that should be preserved on TX failure
func (r *ReceiptsContextStub) GetPreserved() []*transaction.Transaction_Receipt {
	var filtered []*transaction.Transaction_Receipt
	for _, receipt := range r.receipts {
		if len(receipt.Data) > 0 && len(receipt.Data[0]) > 0 && receipt.Data[0][0] >= kapp.SystemReceiptTypeStart {
			filtered = append(filtered, receipt)
		}
	}
	return filtered
}

// Add appends a receipt to the list
func (r *ReceiptsContextStub) Add(receipt *transaction.Transaction_Receipt) {
	r.receipts = append(r.receipts, receipt)
}
