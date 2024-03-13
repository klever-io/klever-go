package factory

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/tools/marshal"
)

// NewKDAFeesPoolKApp creates a token Fees Pool KApp
func NewKDAFeesPoolKApp(
	Marshalizer marshal.Marshalizer,
	PubkeyConv core.PubkeyConverter,
	ForkController core.ForkController,
) (kapp.KDAFeesPoolKapp, error) {

	args := &kdafeespool.ArgsNewKDAFeesPoolKApp{
		Marshalizer:    Marshalizer,
		PubkeyConv:     PubkeyConv,
		ForkController: ForkController,
	}

	return kdafeespool.NewKDAFeesPoolKApp(args)
}
