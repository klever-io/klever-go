package disabled

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
)

type kappsController struct {
}

// NewAccountsAdapter returns a nil implementation of accountsAdapter
func NewKAppsController() *kappsController {
	return &kappsController{}
}

// InitKApps -
func (a *kappsController) InitKApps(state.AccountsCacher) error {
	return nil
}

// GetValidatorsKApp -
func (a *kappsController) GetValidatorsKApp() kapp.ValidatorsKapp {
	return nil
}

// GetKDAFeesPoolKApp -
func (a *kappsController) GetKDAFeesPoolKApp() kapp.KDAFeesPoolKapp {
	return nil
}

// GetAccountsKApp -
func (a *kappsController) GetAccountsKApp() kapp.AccountsKapp {
	return nil
}

func (a *kappsController) GetSystemAccountKApp() kapp.SystemAccountKapp {
	return nil
}

// GetKDAKApp -
func (a *kappsController) GetKDAKApp() kapp.KDAKapp {
	return nil
}

// GetITOKApp -
func (a *kappsController) GetITOKApp() kapp.ITOKapp {
	return nil
}

// GetMarketKApp -
func (a *kappsController) GetMarketKApp() kapp.MarketKapp {
	return nil
}

// GetProposalKApp -
func (a *kappsController) GetProposalKApp() kapp.ProposalKapp {
	return nil
}

func (a *kappsController) GetCurrentKAppContext() kapp.KappContext {
	return nil
}

// GetProposalKApp -
func (a *kappsController) SetCurrentKAppContext(kappContext kapp.KappContext) {
}

// GetProposalController
func (k *kappsController) GetProposalController() kapps.ActiveProposalController {
	return nil
}

// SetProposalController
func (k *kappsController) SetProposalController(_ kapps.ActiveProposalController) error {
	return nil
}

func (k *kappsController) GetForkController() core.ForkController {
	return nil
}

// IsReadOnly -
func (k *kappsController) IsReadOnly() bool {
	return false
}

// IsInterfaceNil -
func (a *kappsController) IsInterfaceNil() bool {
	return a == nil
}
