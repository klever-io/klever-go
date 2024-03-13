package factory

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/proposal"
	"github.com/klever-io/klever-go/tools/marshal"
)

// NewProposalKApp creates a Proposal KApp
func NewProposalKApp(
	Marshalizer marshal.Marshalizer,
	PubkeyConv core.PubkeyConverter,
	ForkController core.ForkController,
) (kapp.ProposalKapp, error) {
	args := &proposal.ArgsNewProposalKApp{
		Marshalizer:    Marshalizer,
		PubkeyConv:     PubkeyConv,
		ForkController: ForkController,
	}

	return proposal.NewProposalKApp(args)
}
