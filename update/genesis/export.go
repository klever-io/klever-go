package genesis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
	"github.com/klever-io/klever-go/update"
)

var _ update.ExportHandler = (*stateExport)(nil)

// ArgsNewStateExporter defines the arguments needed to create new state exporter
type ArgsNewStateExporter struct {
	StateSyncer              update.StateSyncer
	Marshalizer              marshal.Marshalizer
	Hasher                   hashing.Hasher
	HardforkStorer           update.HardforkStorer
	ExportFolder             string
	AddressPubKeyConverter   core.PubkeyConverter
	ValidatorPubKeyConverter core.PubkeyConverter
	GenesisNodesSetupHandler update.GenesisNodesSetupHandler
}

type stateExport struct {
	stateSyncer              update.StateSyncer
	marshalizer              marshal.Marshalizer
	hasher                   hashing.Hasher
	hardforkStorer           update.HardforkStorer
	exportFolder             string
	addressPubKeyConverter   core.PubkeyConverter
	validatorPubKeyConverter core.PubkeyConverter
	genesisNodesSetupHandler update.GenesisNodesSetupHandler
}

var log = logger.GetOrCreate("update/genesis")

// NewStateExporter exports all the data at a specific moment to a hardfork storer
func NewStateExporter(args ArgsNewStateExporter) (*stateExport, error) {
	if check.IfNil(args.StateSyncer) {
		return nil, update.ErrNilStateSyncer
	}
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.Hasher) {
		return nil, common.ErrNilHasher
	}
	if check.IfNil(args.HardforkStorer) {
		return nil, update.ErrNilHardforkStorer
	}
	if len(args.ExportFolder) == 0 {
		return nil, update.ErrEmptyExportFolderPath
	}
	if check.IfNil(args.AddressPubKeyConverter) {
		return nil, fmt.Errorf("%w for address", common.ErrNilPubKeyConverter)
	}
	if check.IfNil(args.ValidatorPubKeyConverter) {
		return nil, fmt.Errorf("%w for validators", common.ErrNilPubKeyConverter)
	}
	if check.IfNil(args.GenesisNodesSetupHandler) {
		return nil, update.ErrNilGenesisNodesSetupHandler
	}

	se := &stateExport{
		stateSyncer:              args.StateSyncer,
		marshalizer:              args.Marshalizer,
		hasher:                   args.Hasher,
		hardforkStorer:           args.HardforkStorer,
		exportFolder:             args.ExportFolder,
		addressPubKeyConverter:   args.AddressPubKeyConverter,
		validatorPubKeyConverter: args.ValidatorPubKeyConverter,
		genesisNodesSetupHandler: args.GenesisNodesSetupHandler,
	}

	return se, nil
}

// ExportAll syncs and exports all the data from every shard for a certain epoch start block
func (se *stateExport) ExportAll(epoch uint32) error {
	err := se.stateSyncer.SyncAllState(epoch)
	if err != nil {
		return err
	}

	defer func() {
		errClose := se.hardforkStorer.Close()
		log.LogIfError(errClose)
	}()

	err = se.exportEpochStartMetaBlock()
	if err != nil {
		return err
	}

	err = se.exportUnFinishedMetaBlocks()
	if err != nil {
		return err
	}

	err = se.exportAllTries()
	if err != nil {
		return err
	}

	err = se.exportAllTransactions()
	if err != nil {
		return err
	}

	return nil
}

func (se *stateExport) exportAllTransactions() error {
	toExportTransactions, err := se.stateSyncer.GetAllTransactions()
	if err != nil {
		return err
	}

	log.Debug("Starting export for transactions", "len", len(toExportTransactions))
	for key, tx := range toExportTransactions {
		errExport := se.exportTx(key, tx)
		if errExport != nil {
			return errExport
		}
	}

	return se.hardforkStorer.FinishedIdentifier(TransactionsIdentifier)
}

func (se *stateExport) exportAllTries() error {
	toExportTries, err := se.stateSyncer.GetAllTries()
	if err != nil {
		return err
	}

	log.Debug("Starting export for tries", "len", len(toExportTries))
	for key, trie := range toExportTries {
		err = se.exportTrie(key, trie)
		if err != nil {
			return err
		}
	}

	return nil
}

func (se *stateExport) exportEpochStartMetaBlock() error {
	metaBlock, err := se.stateSyncer.GetEpochStartMetaBlock()
	if err != nil {
		return err
	}

	log.Debug("Starting export for epoch start metaBlock")
	err = se.exportMetaBlock(metaBlock, EpochStartMetaBlockIdentifier)
	if err != nil {
		return err
	}

	err = se.hardforkStorer.FinishedIdentifier(EpochStartMetaBlockIdentifier)
	if err != nil {
		return err
	}

	return nil
}

func (se *stateExport) exportUnFinishedMetaBlocks() error {
	unFinishedMetaBlocks, err := se.stateSyncer.GetUnFinishedMetaBlocks()
	if err != nil {
		return err
	}

	log.Debug("Starting export for unFinished metaBlocks", "len", len(unFinishedMetaBlocks))
	for _, metaBlock := range unFinishedMetaBlocks {
		errExportMetaBlock := se.exportMetaBlock(metaBlock, UnFinishedMetaBlocksIdentifier)
		if errExportMetaBlock != nil {
			return errExportMetaBlock
		}
	}

	err = se.hardforkStorer.FinishedIdentifier(UnFinishedMetaBlocksIdentifier)
	if err != nil {
		return err
	}

	return nil
}

func (se *stateExport) exportMetaBlock(metaBlock *block.Block, identifier string) error {
	jsonData, err := json.Marshal(metaBlock)
	if err != nil {
		return err
	}

	metaHash := se.hasher.Compute(string(jsonData))
	versionKey := CreateVersionKey(metaBlock, metaHash)
	err = se.hardforkStorer.Write(identifier, []byte(versionKey), jsonData)
	if err != nil {
		return err
	}

	log.Debug("Exported metaBlock",
		"identifier", identifier,
		"version key", versionKey,
		"hash", metaHash,
		"epoch", metaBlock.Header.Epoch,
		"round", metaBlock.Header.Slot,
		"nonce", metaBlock.Header.Nonce,
		"start of epoch block", metaBlock.Header.Nonce == 0 || metaBlock.GetIsEpochStart(),
		"rootHash", metaBlock.Header.TrieRoot,
	)

	return nil
}

func (se *stateExport) exportTrie(key string, trie data.Trie) error {
	identifier := TrieIdentifier + atSep + key

	accType, err := GetTrieType(identifier)
	if err != nil {
		return err
	}

	rootHash, err := trie.RootHash()
	if err != nil {
		return err
	}

	ctx := context.Background()
	leavesChannel, err := trie.GetAllLeavesOnChannel(rootHash, ctx)
	if err != nil {
		return err
	}

	if accType == ValidatorAccount {
		var validatorData []*state.ValidatorInfo
		validatorData, err = getValidatorDataFromLeaves(leavesChannel, se.marshalizer)
		if err != nil {
			return err
		}

		nodesSetupFilePath := filepath.Join(se.exportFolder, core.NodesSetupJsonFileName)
		err = se.exportNodesSetupJson(validatorData)
		if err == nil {
			log.Debug("hardfork nodesSetup.json exported successfully", "file path", nodesSetupFilePath)
		} else {
			log.Warn("hardfork nodesSetup.json not exported", "file path", nodesSetupFilePath, "error", err)
		}

		return err
	}

	rootHashKey := CreateRootHashKey(key)

	err = se.hardforkStorer.Write(identifier, []byte(rootHashKey), rootHash)
	if err != nil {
		return err
	}

	if accType == DataTrie {
		return se.exportDataTries(leavesChannel, accType, identifier)
	}

	log.Debug("exporting trie",
		"identifier", identifier,
		"root hash", rootHash,
	)

	return se.exportAccountLeaves(leavesChannel, accType, identifier)
}

func (se *stateExport) exportDataTries(
	leavesChannel chan data.KeyValueHolder,
	accType Type,
	identifier string,
) error {
	for leaf := range leavesChannel {
		keyToExport := CreateAccountKey(accType, leaf.Key())
		err := se.hardforkStorer.Write(identifier, []byte(keyToExport), leaf.Value())
		if err != nil {
			return err
		}
	}

	err := se.hardforkStorer.FinishedIdentifier(identifier)
	if err != nil {
		return err
	}

	return nil
}

func (se *stateExport) exportAccountLeaves(
	leavesChannel chan data.KeyValueHolder,
	accType Type,
	identifier string,
) error {
	for leaf := range leavesChannel {
		keyToExport := CreateAccountKey(accType, leaf.Key())
		err := se.hardforkStorer.Write(identifier, []byte(keyToExport), leaf.Value())
		if err != nil {
			return err
		}
	}

	err := se.hardforkStorer.FinishedIdentifier(identifier)
	if err != nil {
		return err
	}

	return nil
}

func (se *stateExport) exportTx(key string, tx data.TransactionHandler) error {
	marshaledData, err := json.Marshal(tx)
	if err != nil {
		return err
	}

	keyToSave := CreateTransactionKey(key, tx)

	err = se.hardforkStorer.Write(TransactionsIdentifier, []byte(keyToSave), marshaledData)
	if err != nil {
		return err
	}

	return nil
}

func (se *stateExport) exportNodesSetupJson(validators []*state.ValidatorInfo) error {
	acceptedListsForExport := []core.PeerType{core.EligibleList, core.WaitingList, core.JailedList}
	initialNodes := make([]*sharding.InitialNode, 0)

	for _, validator := range validators {
		if shouldExportValidator(validator, acceptedListsForExport) {
			initialNodes = append(initialNodes, &sharding.InitialNode{
				PubKey:        se.validatorPubKeyConverter.Encode(validator.GetPublicKey()),
				Address:       se.addressPubKeyConverter.Encode(validator.GetOwnerAddress()),
				InitialRating: validator.GetRating(),
			})
		}
	}

	sort.SliceStable(initialNodes, func(i, j int) bool {
		return strings.Compare(initialNodes[i].PubKey, initialNodes[j].PubKey) < 0
	})

	genesisNodesSetupHandler := se.genesisNodesSetupHandler
	nodesSetup := &sharding.NodesSetup{
		StartTime:             genesisNodesSetupHandler.GetStartTime(),
		SlotInterval:          genesisNodesSetupHandler.GetSlotInterval(),
		ConsensusGroupSize:    genesisNodesSetupHandler.GetConsensusGroupSize(),
		ChainID:               genesisNodesSetupHandler.GetChainID(),
		MinTransactionVersion: genesisNodesSetupHandler.GetMinTransactionVersion(),
		InitialNodes:          initialNodes,
	}

	nodesSetupBytes, err := json.MarshalIndent(nodesSetup, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(se.exportFolder, core.NodesSetupJsonFileName), nodesSetupBytes, common.DefaultDirPermission)
}

// IsInterfaceNil returns true if underlying object is nil
func (se *stateExport) IsInterfaceNil() bool {
	return se == nil
}
