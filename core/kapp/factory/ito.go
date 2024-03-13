package factory

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/ito"
	"github.com/klever-io/klever-go/tools/marshal"
)

// NewITOKApp creates a ITO KApp
func NewITOKApp(
	Marshalizer marshal.Marshalizer,
	PubkeyConv core.PubkeyConverter,
	ForkController core.ForkController,
) (kapp.ITOKapp, error) {

	args := &ito.ArgsNewITOKApp{
		Marshalizer:    Marshalizer,
		PubkeyConv:     PubkeyConv,
		ForkController: ForkController,
	}

	return ito.NewITOKApp(args)
}
