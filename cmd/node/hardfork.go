package main

import (
	"fmt"
	"path/filepath"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto"
	"github.com/klever-io/klever-go/data/endProcess"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/eventNotifier/notifier"
	"github.com/klever-io/klever-go/factory"
	"github.com/klever-io/klever-go/node"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/update"
	exportFactory "github.com/klever-io/klever-go/update/factory"
	"github.com/klever-io/klever-go/update/trigger"
)

func createHardForkTrigger(
	config *config.Config,
	keyGen crypto.KeyGenerator,
	pubKey crypto.PublicKey,
	nodesCoordinator sharding.NodesCoordinator,
	coreData *factory.CoreComponents,
	stateComponents *factory.StateComponents,
	data *factory.DataComponents,
	crypto *factory.CryptoComponents,
	process *factory.Process,
	network *factory.NetworkComponents,
	whiteListRequest process.WhiteListHandler,
	whiteListerVerifiedTxs process.WhiteListHandler,
	chanStopNodeProcess chan endProcess.ArgEndProcess,
	epochStartNotifier notifier.EpochStartNotifier,
	importStartHandler update.ImportStartHandler,
	nodesSetup update.GenesisNodesSetupHandler,
	workingDir string,
	epochNotifier process.EpochNotifier,
) (node.HardforkTrigger, error) {

	selfPubKeyBytes, err := pubKey.ToByteArray()
	if err != nil {
		return nil, err
	}
	triggerPubKeyBytes, err := stateComponents.ValidatorPubkeyConverter.Decode(config.Hardfork.PublicKeyToListenFrom)
	if err != nil {
		return nil, fmt.Errorf("%w while decoding HardforkConfig.PublicKeyToListenFrom", err)
	}

	accountsDBs := make(map[state.AccountsDbIdentifier]state.AccountsAdapter)
	accountsDBs[state.UserAccountsState] = stateComponents.AccountsAdapter
	accountsDBs[state.PeerAccountsState] = stateComponents.PeersAdapter
	accountsDBs[state.KAppAccountsState] = stateComponents.KAppsAdapter
	hardForkConfig := config.Hardfork
	exportFolder := filepath.Join(workingDir, hardForkConfig.ImportFolder)
	argsExporter := exportFactory.ArgsExporter{
		TxSignMarshalizer:         coreData.TxSignMarshalizer,
		Marshalizer:               coreData.InternalMarshalizer,
		Hasher:                    coreData.Hasher,
		Uint64Converter:           coreData.Uint64ByteSliceConverter,
		DataPool:                  data.Datapool,
		StorageService:            data.Store,
		RequestHandler:            process.RequestHandler,
		Messenger:                 network.NetMessenger,
		ActiveAccountsDBs:         accountsDBs,
		ExistingResolvers:         process.ResolversFinder,
		ExportFolder:              exportFolder,
		ExportTriesStorageConfig:  hardForkConfig.ExportTriesStorageConfig,
		ExportStateStorageConfig:  hardForkConfig.ExportStateStorageConfig,
		ExportStateKeysConfig:     hardForkConfig.ExportKeysStorageConfig,
		WhiteListHandler:          whiteListRequest,
		WhiteListerVerifiedTxs:    whiteListerVerifiedTxs,
		InterceptorsContainer:     process.InterceptorsContainer,
		MultiSigner:               crypto.MultiSigner,
		NodesCoordinator:          nodesCoordinator,
		SingleSigner:              crypto.TxSingleSigner,
		AddressPubKeyConverter:    stateComponents.AddressPubkeyConverter,
		ValidatorPubKeyConverter:  stateComponents.ValidatorPubkeyConverter,
		BlockKeyGen:               keyGen,
		KeyGen:                    crypto.TxSignKeyGen,
		BlockSigner:               crypto.SingleSigner,
		HeaderSigVerifier:         process.HeaderSigVerifier,
		HeaderIntegrityVerifier:   process.HeaderIntegrityVerifier,
		MaxTrieLevelInMemory:      config.StateTriesConfig.MaxStateTrieLevelInMemory,
		InputAntifloodHandler:     network.InputAntifloodHandler,
		OutputAntifloodHandler:    network.OutputAntifloodHandler,
		ChainID:                   coreData.ChainID,
		SlotManager:               process.SlotManager,
		GenesisNodesSetupHandler:  nodesSetup,
		InterceptorDebugConfig:    config.Debug.InterceptorResolver,
		MinTxVersion:              coreData.MinTransactionVersion,
		TxSignHasher:              coreData.TxSignHasher,
		EpochNotifier:             epochNotifier,
		EpochStartTrigger:         process.EpochStartTrigger,
		NumConcurrentTrieSyncers:  config.TrieSync.NumConcurrentTrieSyncers,
		MaxHardCapForMissingNodes: config.TrieSync.MaxHardCapForMissingNodes,
		KAppController:            stateComponents.KAppController,
	}
	hardForkExportFactory, err := exportFactory.NewExportHandlerFactory(argsExporter)
	if err != nil {
		return nil, err
	}

	argTrigger := trigger.ArgHardforkTrigger{
		TriggerPubKeyBytes:        triggerPubKeyBytes,
		SelfPubKeyBytes:           selfPubKeyBytes,
		Enabled:                   config.Hardfork.EnableTrigger,
		EnabledAuthenticated:      config.Hardfork.EnableTriggerFromP2P,
		EpochProvider:             process.EpochStartTrigger,
		ExportFactoryHandler:      hardForkExportFactory,
		ChanStopNodeProcess:       chanStopNodeProcess,
		EpochConfirmedNotifier:    epochStartNotifier,
		CloseAfterExportInMinutes: config.Hardfork.CloseAfterExportInMinutes,
		ImportStartHandler:        importStartHandler,
		SlotManager:               process.SlotManager,
	}
	hardforkTrigger, err := trigger.NewTrigger(argTrigger)
	if err != nil {
		return nil, err
	}

	return hardforkTrigger, nil
}
