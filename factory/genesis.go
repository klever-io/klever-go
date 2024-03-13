package factory

import (
	"errors"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	genesisProcess "github.com/klever-io/klever-go/genesis/process"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

func generateGenesisHeadersAndApplyInitialBalances(args *processComponentsFactoryArgs, workingDir string) (data.HeaderHandler, error) {
	coreComponents := args.coreData
	stateComponents := args.state
	dataComponents := args.data
	nodesSetup := args.nodesConfig
	accountsParser := args.accountsParser
	economicsData := args.economicsData

	arg := genesisProcess.ArgsGenesisBlockCreator{
		GenesisTime:              nodesSetup.StartTime,
		StartEpochNum:            args.startEpochNum,
		PubkeyConv:               stateComponents.AddressPubkeyConverter,
		InitialNodesSetup:        nodesSetup,
		Economics:                economicsData,
		Store:                    dataComponents.Store,
		Blkc:                     dataComponents.Blkc,
		Marshalizer:              coreComponents.InternalMarshalizer,
		SignMarshalizer:          coreComponents.TxSignMarshalizer,
		Hasher:                   coreComponents.Hasher,
		Uint64ByteSliceConverter: coreComponents.Uint64ByteSliceConverter,
		DataPool:                 dataComponents.Datapool,
		AccountsParser:           accountsParser,
		Accounts:                 stateComponents.AccountsAdapter,
		PeerAccounts:             stateComponents.PeersAdapter,
		KAppAccounts:             stateComponents.KAppsAdapter,
		KAppController:           stateComponents.KAppController,
		Indexer:                  args.indexer,
		TxLogsProcessor:          args.txLogsProcessor,
		//HardForkConfig:           args.mainConfig.Hardfork,
		TrieStorageManagers: args.tries.TrieStorageManagers,
		ChainID:             string(args.coreComponents.ChainID),
		BlockSignKeyGen:     args.crypto.BlockSignKeyGen,
		//ImportStartHandler:       args.importStartHandler,
		WorkingDir: workingDir,
		//GenesisString:            args.mainConfig.GeneralSettings.GenesisString,
		Preferences: &args.mainConfig.Preferences,
	}

	gbc, err := genesisProcess.NewGenesisBlockCreator(arg)
	if err != nil {
		return nil, err
	}

	return gbc.CreateGenesisBlock()
}

func prepareGenesisBlock(args *processComponentsFactoryArgs, genesisBlock data.HeaderHandler) error {
	if check.IfNil(genesisBlock) {
		return errors.New("genesis block does not exist")
	}

	blck, ok := genesisBlock.(*block.Block)
	if !ok {
		return common.ErrWrongTypeAssertion
	}

	genesisBlockHash, err := tools.CalculateHash(args.coreData.InternalMarshalizer, args.coreData.Hasher, blck.Header)
	if err != nil {
		return err
	}

	err = args.data.Blkc.SetGenesisHeader(genesisBlock)
	if err != nil {
		return err
	}

	args.data.Blkc.SetGenesisHeaderHash(genesisBlockHash)

	marshalizedBlock, err := args.coreData.InternalMarshalizer.Marshal(genesisBlock)
	if err != nil {
		return err
	}

	errNotCritical := args.data.Store.Put(retriever.BlockUnit, genesisBlockHash, marshalizedBlock)
	if errNotCritical != nil {
		log.Error("error storing genesis metablock", "error", errNotCritical.Error())
	}

	nonceToByteSlice := args.uint64Converter.ToByteSlice(genesisBlock.GetNonce())
	errNotCritical = args.data.Store.Put(retriever.HdrNonceHashDataUnit, nonceToByteSlice, genesisBlockHash)
	if errNotCritical != nil {
		log.Error("error storing genesis metablock (nonce-hash)", "error", errNotCritical.Error())
	}

	return nil
}
