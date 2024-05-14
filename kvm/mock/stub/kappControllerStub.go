package stub

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
)

var _ kapp.KAppController = (*KAppControllerStub)(nil)

// implement kapp.KAppController stub
type KAppControllerStub struct {
	InitKAppsCalled             func(state.AccountsCacher) error
	GetValidatorsKAppCalled     func() kapp.ValidatorsKapp
	GetKDAFeesPoolKAppCalled    func() kapp.KDAFeesPoolKapp
	GetAccountsKAppCalled       func() kapp.AccountsKapp
	GetITOKAppCalled            func() kapp.ITOKapp
	GetMarketKAppCalled         func() kapp.MarketKapp
	GetKDAKAppCalled            func() kapp.KDAKapp
	GetProposalKAppCalled       func() kapp.ProposalKapp
	SetCurrentKAppContextCalled func(kappContext kapp.KappContext)
	GetCurrentKAppContextCalled func() kapp.KappContext
	GetProposalControllerCalled func() kapps.ActiveProposalController
	SetProposalControllerCalled func(proposalController kapps.ActiveProposalController) error
	GetForkControllerCalled     func() core.ForkController
	GetSystemAccountsCalled     func() kapp.SystemAccountKapp
	IsInterfaceNilCalled        func() bool
}

func (stub *KAppControllerStub) GetSystemAccountKApp() kapp.SystemAccountKapp {
	if stub.GetSystemAccountsCalled != nil {
		return stub.GetSystemAccountsCalled()
	}

	return nil
}

func (stub *KAppControllerStub) InitKApps(accountsCacher state.AccountsCacher) error {
	if stub.InitKAppsCalled != nil {
		return stub.InitKAppsCalled(accountsCacher)
	}

	return nil
}

func (stub *KAppControllerStub) GetValidatorsKApp() kapp.ValidatorsKapp {
	if stub.GetValidatorsKAppCalled != nil {
		return stub.GetValidatorsKAppCalled()
	}

	return nil
}

func (stub *KAppControllerStub) GetKDAFeesPoolKApp() kapp.KDAFeesPoolKapp {
	if stub.GetKDAFeesPoolKAppCalled != nil {
		return stub.GetKDAFeesPoolKAppCalled()
	}

	return nil
}

func (stub *KAppControllerStub) GetAccountsKApp() kapp.AccountsKapp {
	if stub.GetAccountsKAppCalled != nil {
		return stub.GetAccountsKAppCalled()
	}

	return nil
}

func (stub *KAppControllerStub) GetITOKApp() kapp.ITOKapp {
	if stub.GetITOKAppCalled != nil {
		return stub.GetITOKAppCalled()
	}

	return nil
}

func (stub *KAppControllerStub) GetMarketKApp() kapp.MarketKapp {
	if stub.GetMarketKAppCalled != nil {
		return stub.GetMarketKAppCalled()
	}

	return nil
}

func (stub *KAppControllerStub) GetKDAKApp() kapp.KDAKapp {
	if stub.GetKDAKAppCalled != nil {
		return stub.GetKDAKAppCalled()
	}

	return nil
}

func (stub *KAppControllerStub) GetProposalKApp() kapp.ProposalKapp {
	if stub.GetProposalKAppCalled != nil {
		return stub.GetProposalKAppCalled()
	}

	return nil
}

func (stub *KAppControllerStub) SetCurrentKAppContext(kappContext kapp.KappContext) {
	if stub.SetCurrentKAppContextCalled != nil {
		stub.SetCurrentKAppContextCalled(kappContext)
	}
}

func (stub *KAppControllerStub) GetCurrentKAppContext() kapp.KappContext {
	if stub.GetCurrentKAppContextCalled != nil {
		return stub.GetCurrentKAppContextCalled()
	}

	return nil
}

func (stub *KAppControllerStub) GetProposalController() kapps.ActiveProposalController {
	if stub.GetProposalControllerCalled != nil {
		return stub.GetProposalControllerCalled()
	}

	return nil
}

func (stub *KAppControllerStub) SetProposalController(proposalController kapps.ActiveProposalController) error {
	if stub.SetProposalControllerCalled != nil {
		return stub.SetProposalControllerCalled(proposalController)
	}

	return nil
}

func (stub *KAppControllerStub) GetForkController() core.ForkController {
	if stub.GetForkControllerCalled != nil {
		return stub.GetForkControllerCalled()
	}

	return nil
}

func (stub *KAppControllerStub) IsInterfaceNil() bool {
	if stub.IsInterfaceNilCalled != nil {
		return stub.IsInterfaceNilCalled()
	}

	return stub == nil
}
