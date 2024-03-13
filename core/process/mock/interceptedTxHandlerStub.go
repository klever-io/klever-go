package mock

import (
	"github.com/klever-io/klever-go/data"
)

// InterceptedTxHandlerStub -
type InterceptedTxHandlerStub struct {
	SenderAddressCalled func() []byte
	NonceCalled         func() uint64
	FeeCalled           func() int64
	KDAFeeCalled        func() data.KDAFeeHandler
	TransactionCalled   func() data.TransactionHandler
}

// SenderAddress -
func (iths *InterceptedTxHandlerStub) SenderAddress() []byte {
	return iths.SenderAddressCalled()
}

// Nonce -
func (iths *InterceptedTxHandlerStub) Nonce() uint64 {
	return iths.NonceCalled()
}

// PermissionID -
func (iths *InterceptedTxHandlerStub) PermissionID() int32 {
	return 0
}

// Signature -
func (iths *InterceptedTxHandlerStub) Signature() [][]byte {
	return nil
}

// ValidatePermission -
func (iths *InterceptedTxHandlerStub) ValidatePermission([]byte) error {
	return nil
}

// Fee -
func (iths *InterceptedTxHandlerStub) Fee() int64 {
	return iths.FeeCalled()
}

func (iths *InterceptedTxHandlerStub) KDAFee() data.KDAFeeHandler {
	return iths.KDAFeeCalled()
}

// Transaction -
func (iths *InterceptedTxHandlerStub) Transaction() data.TransactionHandler {
	return iths.TransactionCalled()
}
