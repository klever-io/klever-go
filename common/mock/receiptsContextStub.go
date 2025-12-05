package mock

import (
	"github.com/klever-io/klever-go/data/transaction"
)

// Receipt type constants - mirrored from core/kapp/interface.go to avoid circular imports
const (
	systemReceiptTypeStart = 120 // System receipts start at 120
	receiptTypeError       = 122 // Error receipt type (systemReceiptTypeStart + 2)
)

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

// GetPreserved returns system receipts (type >= systemReceiptTypeStart) that should be preserved on TX failure
func (r *ReceiptsContextStub) GetPreserved() []*transaction.Transaction_Receipt {
	var filtered []*transaction.Transaction_Receipt
	for _, receipt := range r.receipts {
		if len(receipt.Data) > 0 && len(receipt.Data[0]) > 0 && receipt.Data[0][0] >= systemReceiptTypeStart {
			filtered = append(filtered, receipt)
		}
	}
	return filtered
}

// AddError adds an error receipt with the given contract ID and parameters
func (r *ReceiptsContextStub) AddError(contractID int, params ...string) {
	data := make([][]byte, len(params)+1)
	data[0] = []byte{byte(receiptTypeError), byte(contractID)}
	for i, param := range params {
		data[i+1] = []byte(param)
	}
	r.receipts = append(r.receipts, &transaction.Transaction_Receipt{Data: data})
}

// Add appends a receipt to the list
func (r *ReceiptsContextStub) Add(receipt *transaction.Transaction_Receipt) {
	r.receipts = append(r.receipts, receipt)
}

// GetErrors returns all error receipts (type == receiptTypeError)
func (r *ReceiptsContextStub) GetErrors() []*transaction.Transaction_Receipt {
	var errors []*transaction.Transaction_Receipt
	for _, receipt := range r.receipts {
		if len(receipt.Data) > 0 && len(receipt.Data[0]) > 0 && receipt.Data[0][0] == receiptTypeError {
			errors = append(errors, receipt)
		}
	}
	return errors
}

// GetErrorField extracts the error field from an error receipt (first param after type/contractID)
func (r *ReceiptsContextStub) GetErrorField(receipt *transaction.Transaction_Receipt) string {
	if len(receipt.Data) > 1 {
		return string(receipt.Data[1])
	}
	return ""
}

// GetErrorReason extracts the error reason from an error receipt (second param after type/contractID)
func (r *ReceiptsContextStub) GetErrorReason(receipt *transaction.Transaction_Receipt) string {
	if len(receipt.Data) > 2 {
		return string(receipt.Data[2])
	}
	return ""
}

// Clear removes all receipts (useful for resetting between test cases)
func (r *ReceiptsContextStub) Clear() {
	r.receipts = make([]*transaction.Transaction_Receipt, 0)
}
