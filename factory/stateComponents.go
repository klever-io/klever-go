package factory

import (
	"fmt"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	kappcontroller "github.com/klever-io/klever-go/core/kapp/kappController"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/state"
	factoryState "github.com/klever-io/klever-go/data/state/factory"
	"github.com/klever-io/klever-go/data/trie/factory"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
)

// StateComponentsFactoryArgs holds the arguments needed for creating a state components factory
type StateComponentsFactoryArgs struct {
	Config         config.Config
	Core           *CoreComponents
	Tries          *TriesComponents
	PathManager    storage.PathManagerHandler
	RatingsData    process.RatingsInfoHandler
	ProcessingMode core.NodeProcessingMode
}

type stateComponentsFactory struct {
	config         config.Config
	core           *CoreComponents
	tries          *TriesComponents
	pathManager    storage.PathManagerHandler
	ratingsData    process.RatingsInfoHandler
	processingMode core.NodeProcessingMode
}

// NewStateComponentsFactory will return a new instance of stateComponentsFactory
func NewStateComponentsFactory(args StateComponentsFactoryArgs) (*stateComponentsFactory, error) {
	if check.IfNil(args.PathManager) {
		return nil, ErrNilPathManager
	}
	if args.Core == nil {
		return nil, ErrNilCoreComponents
	}
	if args.Tries == nil {
		return nil, ErrNilTriesComponents
	}

	return &stateComponentsFactory{
		config:         args.Config,
		core:           args.Core,
		tries:          args.Tries,
		pathManager:    args.PathManager,
		ratingsData:    args.RatingsData,
		processingMode: args.ProcessingMode,
	}, nil
}

// Create creates the state components
func (scf *stateComponentsFactory) Create(forkController core.ForkController) (*StateComponents, error) {
	processPubkeyConverter, err := NewPubkeyConverter(Bech32Format, 32)
	if err != nil {
		return nil, fmt.Errorf("%w for ProcessPubkeyConverter: %s", ErrPubKeyConverterCreation, err.Error())
	}

	validatorPubkeyConverter, err := NewPubkeyConverter(HexFormat, 96)
	if err != nil {
		return nil, fmt.Errorf("%w for ValidatorPubkeyConverter: %s", ErrPubKeyConverterCreation, err.Error())
	}

	accountFactory := factoryState.NewAccountCreator()
	merkleTrie := scf.tries.TriesContainer.Get([]byte(factory.UserAccountTrie))
	accountsAdapter, err := state.NewAccountsDB(merkleTrie, scf.core.Hasher, scf.core.InternalMarshalizer, accountFactory, scf.processingMode)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrAccountsAdapterCreation, err.Error())
	}

	peerFactory := factoryState.NewPeerAccountCreator()
	merkleTrie = scf.tries.TriesContainer.Get([]byte(factory.PeerAccountTrie))
	peerAdapter, err := state.NewPeerAccountsDB(merkleTrie, scf.core.Hasher, scf.core.InternalMarshalizer, peerFactory, scf.processingMode)
	if err != nil {
		return nil, err
	}

	kappFactory := factoryState.NewKAppAccountCreator()
	merkleTrie = scf.tries.TriesContainer.Get([]byte(factory.KAppAccountTrie))
	kappAdapter, err := state.NewKAppAccountsDB(merkleTrie, scf.core.Hasher, scf.core.InternalMarshalizer, kappFactory, scf.processingMode)
	if err != nil {
		return nil, err
	}

	argsKapp := kappcontroller.ArgsNewKApp{
		Hasher:           scf.core.Hasher,
		Marshalizer:      scf.core.InternalMarshalizer,
		PubkeyConv:       processPubkeyConverter,
		ForkController:   forkController,
		RatingsData:      scf.ratingsData,
		VersionsByEpochs: scf.config.Versions.VersionsByEpochs,
	}

	kAppController, err := kappcontroller.NewKappController(argsKapp)
	if err != nil {
		return nil, common.ErrKAppController
	}

	kAppControllerSimulator, err := kappcontroller.NewKappController(argsKapp)
	if err != nil {
		return nil, common.ErrKAppController
	}

	return &StateComponents{
		PeersAdapter:             peerAdapter,
		AddressPubkeyConverter:   processPubkeyConverter,
		ValidatorPubkeyConverter: validatorPubkeyConverter,
		AccountsAdapter:          accountsAdapter,
		KAppsAdapter:             kappAdapter,
		KAppController:           kAppController,
		KAppControllerSimulator:  kAppControllerSimulator,
		KAppArgs:                 argsKapp,
	}, nil
}
