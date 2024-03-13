package factory

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/market"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/tools/marshal"
)

// NewMarketKApp creates a Market KApp
func NewMarketKApp(
	Hasher hashing.Hasher,
	Marshalizer marshal.Marshalizer,
	PubkeyConv core.PubkeyConverter,
	ForkController core.ForkController,
) (kapp.MarketKapp, error) {

	args := &market.ArgsNewMarketKApp{
		Hasher:         Hasher,
		Marshalizer:    Marshalizer,
		PubkeyConv:     PubkeyConv,
		ForkController: ForkController,
	}

	return market.NewMarketKApp(args)
}
