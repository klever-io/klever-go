package factory

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/kda"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/tools/marshal"
)

// NewKDAKApp creates a KDA KApp
func NewKDAKApp(
	Hasher hashing.Hasher,
	Marshalizer marshal.Marshalizer,
	PubkeyConv core.PubkeyConverter,
	ForkController core.ForkController,
) (kapp.KDAKapp, error) {

	args := &kda.ArgsNewKDAKApp{
		Hasher:         Hasher,
		Marshalizer:    Marshalizer,
		PubkeyConv:     PubkeyConv,
		ForkController: ForkController,
	}

	return kda.NewKDAKApp(args)
}
