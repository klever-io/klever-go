package proposal

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

// This file exports private functions for testing purposes only.
// It's only compiled during tests (note the _test.go suffix).

// FinalizeProposal exports the private finalizeProposal method for testing
func (p *proposalKapp) FinalizeProposal(
	ctx kapp.KappContext,
	proposalKapp state.KAppAccountHandler,
	proposal *kapps.ProposalData,
	controller *kapps.ProposalController,
	staking *kapps.StakingData,
	tc *transaction.ProposalContract,
	sender []byte,
) (transaction.Transaction_TXResultCode, error) {
	return p.finalizeProposal(ctx, proposalKapp, proposal, controller, staking, tc, sender)
}

// CopyProposalDetails exports the private copyProposalDetails function for testing
func CopyProposalDetails(
	ctx kapp.KappContext,
	proposal *kapps.ProposalData,
	totalStaked int64,
	tc *transaction.ProposalContract,
	sender []byte,
) {
	copyProposalDetails(ctx, proposal, totalStaked, tc, sender)
}

// UpdateActiveProposals exports the private updateActiveProposals function for testing
func UpdateActiveProposals(controller *kapps.ProposalController, proposal *kapps.ProposalData) {
	updateActiveProposals(controller, proposal)
}

// ValidateCreateInput exports the private validateCreateInput method for testing
func (p *proposalKapp) ValidateCreateInput(tc *transaction.ProposalContract) error {
	return p.validateCreateInput(tc)
}

// CheckStakingRequirements exports the private checkStakingRequirements method for testing
func (p *proposalKapp) CheckStakingRequirements(staking *kapps.StakingData, sender []byte) (transaction.Transaction_TXResultCode, error) {
	return p.checkStakingRequirements(staking, sender)
}
