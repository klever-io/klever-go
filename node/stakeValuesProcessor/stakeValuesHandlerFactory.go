package stakeValuesProcessor

import (
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/network/api/apiResolver"
	"github.com/klever-io/klever-go/tools/marshal"
)

// ArgsTotalStakedValueHandler is struct that contains components that are needed to create a TotalStakedValueHandler
type ArgsTotalStakedValueHandler struct {
	NodePrice           string
	InternalMarshalizer marshal.Marshalizer
	Accounts            state.AccountsAdapter
}

// CreateTotalStakedValueHandler wil create a new instance of TotalStakedValueHandler
func CreateTotalStakedValueHandler(args *ArgsTotalStakedValueHandler) (apiResolver.TotalStakedValueHandler, error) {
	return NewTotalStakedValueProcessor(
		args.NodePrice,
		args.InternalMarshalizer,
		args.Accounts,
	)
}
