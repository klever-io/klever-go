package mock

import "github.com/klever-io/klever-go/data"

// TxValidatorHandlerStub -
type TxValidatorHandlerStub struct {
	SenderAddressCalled               func() []byte
	FeeCalled                         func() int64
	KDAFeeCalled                      func() data.KDAFeeHandler
	NonceCalled                       func() uint64
	SignatureCalled                   func() [][]byte
	ValidatePermissionOperationCalled func([]byte) error
}

// SenderAddress -
func (tvhs *TxValidatorHandlerStub) SenderAddress() []byte {
	return tvhs.SenderAddressCalled()
}

// Fee -
func (tvhs *TxValidatorHandlerStub) Fee() int64 {
	return tvhs.FeeCalled()
}

// KDAFee -
func (tvhs *TxValidatorHandlerStub) KDAFee() data.KDAFeeHandler {
	return tvhs.KDAFeeCalled()
}

// Fee -
func (tvhs *TxValidatorHandlerStub) Nonce() uint64 {
	return tvhs.NonceCalled()
}

// Hash -
func (tvhs *TxValidatorHandlerStub) Hash() []byte {
	return nil
}

// PermissionID -
func (tvhs *TxValidatorHandlerStub) PermissionID() int32 {
	return 0
}

// ValidatePermission -
func (tvhs *TxValidatorHandlerStub) ValidatePermissionOperation(permission []byte) error {
	if tvhs.ValidatePermissionOperationCalled != nil {
		return tvhs.ValidatePermissionOperationCalled(permission)
	}
	return nil
}

// Signature -
func (tvhs *TxValidatorHandlerStub) Signature() [][]byte {
	if tvhs.SignatureCalled != nil {
		return tvhs.SignatureCalled()
	}
	return make([][]byte, 0)
}
