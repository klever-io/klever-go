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

// Add appends a receipt to the list
func (r *ReceiptsContextStub) Add(receipt *transaction.Transaction_Receipt) {
	r.receipts = append(r.receipts, receipt)
}
