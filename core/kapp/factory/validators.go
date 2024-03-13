package factory

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/validators"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/tools/marshal"
)

// NewValidatorKApp creates a validator KApp
func NewValidatorKApp(
	Marshalizer marshal.Marshalizer,
	PubkeyConv core.PubkeyConverter,
	ForkController core.ForkController,
	RatingsData process.RatingsInfoHandler,
) (kapp.ValidatorsKapp, error) {

	args := &validators.ArgsNewValidatorKApp{
		Marshalizer:    Marshalizer,
		PubkeyConv:     PubkeyConv,
		RatingsData:    RatingsData,
		ForkController: ForkController,
	}

	return validators.NewValidatorKApp(args)
}
