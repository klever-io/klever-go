package mock

import (
	"math"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
)

type ProposalKappStub struct {
	GetProposalDataCalled         func(proposalID uint64) (*kapps.ProposalData, error)
	GetAccountProposalVotesCalled func(encodedAddr string) ([]kapp.ProposalVoteIndexEntry, error)
	SetAccountProposalVotesCalled func(encodedAddr string, entries []kapp.ProposalVoteIndexEntry) error
	PreForkVoteIDBoundCalled      func() (uint64, error)
	GetVoterCalled                func(proposal *kapps.ProposalData, proposalID uint64, encodedAddr string) (*kapps.ProposalData_VoteDetail, error)
	SetVoterCalled                func(proposal *kapps.ProposalData, proposalID uint64, encodedAddr string, detail *kapps.ProposalData_VoteDetail) error
	RemoveVoterCalled             func(proposal *kapps.ProposalData, proposalID uint64, encodedAddr string) error
	SetKAppControllerCalled       func(controller kapp.KAppController) error
	SetAccountsCacherCalled       func(cacher state.AccountsCacher) error
	GetAccountsCacherCalled       func() state.AccountsCacher
	GetProposalCalled             func(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error)
	SetProposalCalled             func(proposalKapp state.KAppAccountHandler, proposalID uint64, proposal *kapps.ProposalData, controller *kapps.ProposalController) error
	CreateCalled                  func(sender []byte, tc *transaction.ProposalContract) (transaction.Transaction_TXResultCode, error)
	VoteCalled                    func(tsender []byte, c *transaction.VoteContract) (transaction.Transaction_TXResultCode, error)
	IsInterfaceNilCalled          func() bool
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

// GetProposalData - falls back to GetProposalCalled so a harness that records loads through the
// older hook keeps seeing them.
func (p *ProposalKappStub) GetProposalData(_ state.KAppAccountHandler, proposalID uint64) (*kapps.ProposalData, error) {
	if p.GetProposalDataCalled != nil {
		return p.GetProposalDataCalled(proposalID)
	}
	if p.GetProposalCalled != nil {
		_, proposal, _, err := p.GetProposalCalled(proposalID)
		return proposal, err
	}

	// Loud rather than nil: a harness that forgot the hook gets a not-found, not a nil deref.
	return nil, common.ErrProposalNotFound
}

// GetAccountProposalVotes -
func (p *ProposalKappStub) GetAccountProposalVotes(_ state.KAppAccountHandler, encodedAddr string) ([]kapp.ProposalVoteIndexEntry, error) {
	if p.GetAccountProposalVotesCalled != nil {
		return p.GetAccountProposalVotesCalled(encodedAddr)
	}

	return nil, nil
}

// SetAccountProposalVotes -
func (p *ProposalKappStub) SetAccountProposalVotes(_ state.KAppAccountHandler, encodedAddr string, entries []kapp.ProposalVoteIndexEntry) error {
	if p.SetAccountProposalVotesCalled != nil {
		return p.SetAccountProposalVotesCalled(encodedAddr, entries)
	}

	return nil
}

// PreForkVoteIDBound - unbounded by default, so a harness that forgot the hook scans every
// active proposal rather than silently filtering all of them out.
func (p *ProposalKappStub) PreForkVoteIDBound(_ state.KAppAccountHandler, _ *kapps.ProposalController) (uint64, error) {
	if p.PreForkVoteIDBoundCalled != nil {
		return p.PreForkVoteIDBoundCalled()
	}

	return math.MaxUint64, nil
}

func (p *ProposalKappStub) GetVoter(
	_ state.KAppAccountHandler,
	proposal *kapps.ProposalData,
	proposalID uint64,
	encodedAddr string,
) (*kapps.ProposalData_VoteDetail, error) {
	if p.GetVoterCalled != nil {
		return p.GetVoterCalled(proposal, proposalID, encodedAddr)
	}

	return proposal.Voters[encodedAddr], nil
}

func (p *ProposalKappStub) SetVoter(
	_ state.KAppAccountHandler,
	proposal *kapps.ProposalData,
	proposalID uint64,
	encodedAddr string,
	detail *kapps.ProposalData_VoteDetail,
) error {
	if p.SetVoterCalled != nil {
		return p.SetVoterCalled(proposal, proposalID, encodedAddr, detail)
	}

	if proposal.Voters == nil {
		proposal.Voters = make(map[string]*kapps.ProposalData_VoteDetail)
	}
	proposal.Voters[encodedAddr] = detail

	return nil
}

func (p *ProposalKappStub) RemoveVoter(
	_ state.KAppAccountHandler,
	proposal *kapps.ProposalData,
	proposalID uint64,
	encodedAddr string,
) error {
	if p.RemoveVoterCalled != nil {
		return p.RemoveVoterCalled(proposal, proposalID, encodedAddr)
	}

	delete(proposal.Voters, encodedAddr)

	return nil
}
