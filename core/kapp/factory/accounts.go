package factory

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/accounts"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/tools/marshal"
)

// NewAccountKApp creates a Account KApp
func NewAccountKApp(
	Hasher hashing.Hasher,
	Marshalizer marshal.Marshalizer,
	PubkeyConv core.PubkeyConverter,
	ForkController core.ForkController,
) (kapp.AccountsKapp, error) {

	args := &accounts.ArgsNewAccountKApp{
		Hasher:         Hasher,
		Marshalizer:    Marshalizer,
		PubkeyConv:     PubkeyConv,
		ForkController: ForkController,
	}

	return accounts.NewAccountKApp(args)
}
