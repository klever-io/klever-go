package account

import (
	"time"

	"github.com/klever-io/klever-go/cmd/tests/common"
)

type Option func(a *Account) error

func WithSync() Option {
	time.Sleep(time.Second * 4)

	return func(a *Account) error {
		acc, err := common.GetAccount(a.Address)
		if err != nil {
			return err
		}

		a.Data = acc

		return nil
	}
}

func WithKeyFile(keyfile string) Option {
	return func(a *Account) error {
		a.KeyFile = keyfile
		return nil
	}
}
