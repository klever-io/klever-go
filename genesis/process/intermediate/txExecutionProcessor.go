package intermediate

import (
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/genesis"
	"github.com/klever-io/klever-go/tools/check"
)

type txExecutionProcessor struct {
	accounts state.AccountsAdapter
}

// NewTxExecutionProcessor is able to execute a transaction
func NewTxExecutionProcessor(
	accounts state.AccountsAdapter,
) (*txExecutionProcessor, error) {
	if check.IfNil(accounts) {
		return nil, common.ErrNilAccountsAdapter
	}

	return &txExecutionProcessor{
		accounts: accounts,
	}, nil
}

// ExecuteTransaction will try to assemble a transaction and execute it against the accounts db
func (tep *txExecutionProcessor) ExecuteTransaction() error {
	// TODO:
	return nil
}

// GetAccount returns if an account exists in the accounts DB
func (tep *txExecutionProcessor) GetAccount(address []byte) (state.UserAccountHandler, bool) {
	account, err := tep.accounts.GetExistingAccount(address)
	if err != nil {
		return nil, false
	}

	userAcc, ok := account.(state.UserAccountHandler)
	if !ok {
		return nil, false
	}

	return userAcc, true
}

// AddBalance adds the provided value on the balance field
func (tep *txExecutionProcessor) AddBalance(senderBytes []byte, value int64) error {
	accnt, err := tep.accounts.LoadAccount(senderBytes)
	if err != nil {
		return err
	}

	userAccnt, ok := accnt.(state.UserAccountHandler)
	if !ok {
		return genesis.ErrWrongTypeAssertion
	}

	err = userAccnt.AddToBalance(value, nil)
	if err != nil {
		return err
	}

	return tep.accounts.SaveAccount(userAccnt)
}

// IsInterfaceNil returns if underlying object is true
func (tep *txExecutionProcessor) IsInterfaceNil() bool {
	return tep == nil
}
