package account

import (
	"github.com/klever-io/klever-go/cmd/tests/common"
)

type Account struct {
	Address string
	KeyFile string
	Data    *common.DataAccount
}

func NewAccount(address string, opts ...Option) (*Account, error) {
	account := &Account{Address: address}

	for _, opt := range opts {
		if err := opt(account); err != nil {
			return nil, err
		}
	}

	return account, nil
}

func (a *Account) Sync() error {
	return WithSync()(a)
}

func (a *Account) GetAllowance(token string) (map[string]int64, error) {
	return common.GetAllowance(a.Address, token)
}
