package mock

import (
	"math/big"

	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

// TxProcessorMock -
type TxProcessorMock struct {
	PreProcessTransactionCalled func(transaction *transaction.Transaction) (state.UserAccountHandler, []byte, error)
	ProcessTransactionCalled    func(block *block.Block, txHash []byte, transaction *transaction.Transaction) error
	ProcessBandwidthFeeCalled   func(txHash []byte, tx *transaction.Transaction, ownAcc state.UserAccountHandler) (int64, error)
	ProcessKAppFeeCalled        func(txHash []byte, tx *transaction.Transaction, ownAcc state.UserAccountHandler) (int64, error)
	SetBalancesToTrieCalled     func(accBalance map[string]*big.Int) (rootHash []byte, err error)
}

func (tp *TxProcessorMock) GetAccounts(
	adrSrc, adrDst []byte,
) (state.UserAccountHandler, state.UserAccountHandler, error) {
	return nil, nil, nil
}

// PreProcessTransaction -
func (tp *TxProcessorMock) PreProcessTransaction(transaction *transaction.Transaction) (state.UserAccountHandler, []byte, error) {
	if tp.PreProcessTransactionCalled != nil {
		return tp.PreProcessTransactionCalled(transaction)
	}
	return nil, nil, nil
}

func (tp *TxProcessorMock) SetProposalController(controller kapps.ActiveProposalController) error {

	return nil
}

// ProcessTransaction -
func (tp *TxProcessorMock) ProcessTransaction(block *block.Block, txHash []byte, transaction *transaction.Transaction) error {
	return tp.ProcessTransactionCalled(block, txHash, transaction)
}

func (tp *TxProcessorMock) ProcessBandwidthFee(txHash []byte, tx *transaction.Transaction, ownAcc state.UserAccountHandler) (int64, error) {
	if tp.ProcessBandwidthFeeCalled != nil {
		return tp.ProcessBandwidthFeeCalled(txHash, tx, ownAcc)
	}

	return 0, nil
}

func (tp *TxProcessorMock) ProcessKAppFee(txHash []byte, tx *transaction.Transaction, ownAcc state.UserAccountHandler) (int64, error) {
	if tp.ProcessKAppFeeCalled != nil {
		return tp.ProcessKAppFeeCalled(txHash, tx, ownAcc)
	}

	return 0, nil
}

// SetBalancesToTrie -
func (tp *TxProcessorMock) SetBalancesToTrie(accBalance map[string]*big.Int) (rootHash []byte, err error) {
	return tp.SetBalancesToTrieCalled(accBalance)
}

// IsInterfaceNil returns true if there is no value under the interface
func (tp *TxProcessorMock) IsInterfaceNil() bool {
	return tp == nil
}
