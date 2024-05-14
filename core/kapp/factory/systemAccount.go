package factory

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/systemAccount"
	"github.com/klever-io/klever-go/tools/marshal"
)

func NewSystemAccountKApp(
	Marshalizer marshal.Marshalizer,
	PubkeyConv core.PubkeyConverter,
	ForkController core.ForkController,
) (kapp.SystemAccountKapp, error) {
	args := &systemAccount.ArgsNewSystemAccountKApp{
		Marshalizer:    Marshalizer,
		PubkeyConv:     PubkeyConv,
		ForkController: ForkController,
	}

	return systemAccount.NewSystemAccountKApp(args)
}
