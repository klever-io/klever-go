package mock

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

type ProposalKappStub struct {
	SetKAppControllerCalled func(controller kapp.KAppController) error
	SetAccountsCacherCalled func(cacher state.AccountsCacher) error
	GetAccountsCacherCalled func() state.AccountsCacher
	GetProposalCalled       func(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error)
	SetProposalCalled       func(proposalKapp state.KAppAccountHandler, proposalID uint64, proposal *kapps.ProposalData, controller *kapps.ProposalController) error
	CreateCalled            func(sender []byte, tc *transaction.ProposalContract) (transaction.Transaction_TXResultCode, error)
	VoteCalled              func(tsender []byte, c *transaction.VoteContract) (transaction.Transaction_TXResultCode, error)
	IsInterfaceNilCalled    func() bool
}

func (p *ProposalKappStub) SetKAppController(controller kapp.KAppController) error {
	if p.SetKAppControllerCalled != nil {
		return p.SetKAppControllerCalled(controller)
	}
	return nil
}

func (p *ProposalKappStub) SetAccountsCacher(cacher state.AccountsCacher) error {
	if p.SetAccountsCacherCalled != nil {
		return p.SetAccountsCacherCalled(cacher)
	}
	return nil
}

func (p *ProposalKappStub) GetAccountsCacher() state.AccountsCacher {
	if p.GetAccountsCacherCalled != nil {
		return p.GetAccountsCacherCalled()
	}
	return nil
}

func (p *ProposalKappStub) GetProposal(
	proposalID uint64,
) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
	if p.GetProposalCalled != nil {
		return p.GetProposalCalled(proposalID)
	}
	return nil, nil, nil, nil
}

func (p *ProposalKappStub) SetProposal(
	proposalKapp state.KAppAccountHandler,
	proposalID uint64,
	proposal *kapps.ProposalData,
	controller *kapps.ProposalController,
) error {
	if p.SetProposalCalled != nil {
		return p.SetProposalCalled(proposalKapp, proposalID, proposal, controller)
	}
	return nil
}

func (p *ProposalKappStub) Create(
	sender []byte,
	tc *transaction.ProposalContract,
) (transaction.Transaction_TXResultCode, error) {
	if p.CreateCalled != nil {
		return p.CreateCalled(sender, tc)
	}
	return 0, nil
}

func (p *ProposalKappStub) Vote(
	tsender []byte,
	c *transaction.VoteContract,
) (transaction.Transaction_TXResultCode, error) {
	if p.VoteCalled != nil {
		return p.VoteCalled(tsender, c)
	}
	return 0, nil
}

func (p *ProposalKappStub) IsInterfaceNil() bool {
	if p.IsInterfaceNilCalled != nil {
		return p.IsInterfaceNilCalled()
	}
	return false
}
