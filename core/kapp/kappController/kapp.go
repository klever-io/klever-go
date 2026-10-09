package kappcontroller

import (
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/kapp/factory"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/marshal"
)

var log = logger.GetOrCreate("kapp")

const (
	ValidatorsKapp    = "KAPP_VALIDATORS"
	KDAFeesPoolKApp   = "KAPP_KDA_FEES_POOL"
	AccountsKApp      = "KAPP_ACCOUNTS"
	ITOKApp           = "KAPP_ITO"
	MarketKApp        = "KAPP_MARKET"
	KDAKapp           = "KAPP_KDA"
	ProposalKApp      = "KAPP_PROPOSAL"
	SystemAccountKApp = "KAPP_SYSTEM_ACCOUNT"
)

type KleverKApp interface {
	SetKAppController(controller kapp.KAppController) error
	SetAccountsCacher(cacher state.AccountsCacher) error
}

type kApp struct {
	validatorsKapp     kapp.ValidatorsKapp
	kdaFeesPoolKapp    kapp.KDAFeesPoolKapp
	systemAccountKapp  kapp.SystemAccountKapp
	accountsKapp       kapp.AccountsKapp
	itoKapp            kapp.ITOKapp
	marketKapp         kapp.MarketKapp
	kdaKapp            kapp.KDAKapp
	proposalKapp       kapp.ProposalKapp
	forkController     core.ForkController
	currentKAppContext kapp.KappContext
	proposalController kapps.ActiveProposalController
	readOnly           bool

	container map[string]KleverKApp
}

// ArgsNewKApp holds the arguments needed to create the KApp Controller
type ArgsNewKApp struct {
	Hasher         hashing.Hasher
	Marshalizer    marshal.Marshalizer
	PubkeyConv     core.PubkeyConverter
	ForkController core.ForkController
	AccountsCacher state.AccountsCacher
	RatingsData    process.RatingsInfoHandler
	// ReadOnly marks the controller as read-only for its whole lifetime. Set by
	// the VM query path (see cmd/node/sc.go). Construction-time only on purpose,
	// so the safety cannot be switched off on a live controller.
	ReadOnly bool
	// VersionsByEpochs is the versions.versionsByEpochs config used by the validators
	// KApp for node version attestation; nil disables version enforcement
	VersionsByEpochs []config.VersionByEpochs
	// MinElectableNodes is the nodes shuffler's minimum electable count (genesis
	// MinNumberOfNodes), used as a floor guard for version demotion
	MinElectableNodes uint32
}

func NewKappController(args ArgsNewKApp) (kapp.KAppController, error) {
	container := make(map[string]KleverKApp)

	validatorsKapp, err := factory.NewValidatorKApp(
		args.Marshalizer,
		args.PubkeyConv,
		args.ForkController,
		args.RatingsData,
		args.VersionsByEpochs,
		args.MinElectableNodes,
	)
	if err != nil {
		return nil, err
	}

	container[ValidatorsKapp] = validatorsKapp

	kdaFeesPoolKapp, err := factory.NewKDAFeesPoolKApp(
		args.Marshalizer,
		args.PubkeyConv,
		args.ForkController,
	)
	if err != nil {
		return nil, err
	}

	container[KDAFeesPoolKApp] = kdaFeesPoolKapp

	systemAccountKapp, err := factory.NewSystemAccountKApp(
		args.Marshalizer,
		args.PubkeyConv,
		args.ForkController,
	)
	if err != nil {
		return nil, err
	}

	container[SystemAccountKApp] = systemAccountKapp

	accountsKapp, err := factory.NewAccountKApp(
		args.Hasher,
		args.Marshalizer,
		args.PubkeyConv,
		args.ForkController,
	)
	if err != nil {
		return nil, err
	}

	container[AccountsKApp] = accountsKapp

	itoKapp, err := factory.NewITOKApp(
		args.Marshalizer,
		args.PubkeyConv,
		args.ForkController,
	)
	if err != nil {
		return nil, err
	}

	container[ITOKApp] = itoKapp

	MarketKapp, err := factory.NewMarketKApp(
		args.Hasher,
		args.Marshalizer,
		args.PubkeyConv,
		args.ForkController,
	)
	if err != nil {
		return nil, err
	}

	container[MarketKApp] = MarketKapp

	kdaKapp, err := factory.NewKDAKApp(
		args.Hasher,
		args.Marshalizer,
		args.PubkeyConv,
		args.ForkController,
	)
	if err != nil {
		return nil, err
	}

	container[KDAKapp] = kdaKapp

	proposalKapp, err := factory.NewProposalKApp(
		args.Marshalizer,
		args.PubkeyConv,
		args.ForkController,
	)
	if err != nil {
		return nil, err
	}

	container[ProposalKApp] = proposalKapp

	return &kApp{
		forkController:    args.ForkController,
		validatorsKapp:    validatorsKapp,
		kdaFeesPoolKapp:   kdaFeesPoolKapp,
		systemAccountKapp: systemAccountKapp,
		accountsKapp:      accountsKapp,
		itoKapp:           itoKapp,
		marketKapp:        MarketKapp,
		kdaKapp:           kdaKapp,
		proposalKapp:      proposalKapp,
		readOnly:          args.ReadOnly,

		container: container,
	}, nil
}

func (k *kApp) InitKApps(accCacher state.AccountsCacher) error {
	log.Trace("creating proposal kapp")
	proposalKApp, err := accCacher.LoadKApp(kapps.ProposalKAppAddress)
	if err != nil {
		return err
	}

	// this will load all active proposals in current epoch
	proposalController, err := proposalKApp.StartProposalsKApp(k.forkController)
	if err != nil {
		return err
	}

	// keep it instead of discarding: controllers not wired through node.go
	// (e.g. the VM query controller in cmd/node/sc.go) would otherwise be left
	// with a nil ActiveProposalController. Shared-instance callers overwrite
	// this later via SetProposalController.
	k.proposalController = proposalController

	// persist the proposal kapp to update the active proposals
	// must use UpdateKapp as accCacher fork may not be available at the time
	err = accCacher.UpdateKapp(proposalKApp)
	if err != nil {
		return err
	}

	// saveAll in case of chain start with cacher enabled
	err = accCacher.SaveAll()
	if err != nil {
		return err
	}

	log.Trace("Setting Accounts Cacher/KApp Controller")
	for _, kapp := range k.container {
		err = kapp.SetAccountsCacher(accCacher)
		if err != nil {
			return err
		}

		err = kapp.SetKAppController(k)
		if err != nil {
			return err
		}
	}

	return nil
}

func (k *kApp) GetValidatorsKApp() kapp.ValidatorsKapp {
	return k.validatorsKapp
}

func (k *kApp) GetKDAFeesPoolKApp() kapp.KDAFeesPoolKapp {
	return k.kdaFeesPoolKapp
}

func (k *kApp) GetSystemAccountKApp() kapp.SystemAccountKapp {
	return k.systemAccountKapp
}

func (k *kApp) GetAccountsKApp() kapp.AccountsKapp {
	return k.accountsKapp
}

func (k *kApp) GetITOKApp() kapp.ITOKapp {
	return k.itoKapp
}

func (k *kApp) GetMarketKApp() kapp.MarketKapp {
	return k.marketKapp
}

func (k *kApp) GetKDAKApp() kapp.KDAKapp {
	return k.kdaKapp
}

func (k *kApp) GetProposalKApp() kapp.ProposalKapp {
	return k.proposalKapp
}

func (k *kApp) SetCurrentKAppContext(kappContext kapp.KappContext) {
	k.currentKAppContext = kappContext
}

func (k *kApp) GetCurrentKAppContext() kapp.KappContext {
	return k.currentKAppContext
}

func (k *kApp) GetProposalController() kapps.ActiveProposalController {
	return k.proposalController
}

func (k *kApp) SetProposalController(proposalController kapps.ActiveProposalController) error {
	k.proposalController = proposalController

	return nil
}

func (k *kApp) GetForkController() core.ForkController {
	return k.forkController
}

// IsReadOnly reports whether the controller is in read-only mode.
func (k *kApp) IsReadOnly() bool {
	return k.readOnly
}

// IsNilIndexer will return a bool value that signals if the indexer's implementation is a NilIndexer
func (k *kApp) IsInterfaceNil() bool {
	return k == nil
}
