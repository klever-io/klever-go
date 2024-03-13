package genesis

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/common"
	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/keyValStorage"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/update"
	"github.com/klever-io/klever-go/update/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStateExporter(t *testing.T) {
	tests := []struct {
		name            string
		args            ArgsNewStateExporter
		requiresErrorIs bool
		exError         error
	}{
		{
			name: "NilStateSyncer",
			args: ArgsNewStateExporter{
				Marshalizer:              &cMock.MarshalizerMock{},
				StateSyncer:              nil,
				HardforkStorer:           &mock.HardforkStorerStub{},
				Hasher:                   &cMock.HasherStub{},
				AddressPubKeyConverter:   &cMock.PubkeyConverterStub{},
				ValidatorPubKeyConverter: &cMock.PubkeyConverterStub{},
				GenesisNodesSetupHandler: &mock.GenesisNodesSetupHandlerStub{},
				ExportFolder:             "test",
			},
			exError: update.ErrNilStateSyncer,
		},
		{
			name: "NilMarshalizer",
			args: ArgsNewStateExporter{
				Marshalizer:              nil,
				StateSyncer:              &mock.SyncStateStub{},
				HardforkStorer:           &mock.HardforkStorerStub{},
				Hasher:                   &cMock.HasherStub{},
				AddressPubKeyConverter:   &cMock.PubkeyConverterStub{},
				ValidatorPubKeyConverter: &cMock.PubkeyConverterStub{},
				GenesisNodesSetupHandler: &mock.GenesisNodesSetupHandlerStub{},
				ExportFolder:             "test",
			},
			exError: common.ErrNilMarshalizer,
		},
		{
			name: "NilHardforkStorer",
			args: ArgsNewStateExporter{
				Marshalizer:              &cMock.MarshalizerMock{},
				StateSyncer:              &mock.SyncStateStub{},
				HardforkStorer:           nil,
				Hasher:                   &cMock.HasherStub{},
				AddressPubKeyConverter:   &cMock.PubkeyConverterStub{},
				ValidatorPubKeyConverter: &cMock.PubkeyConverterStub{},
				GenesisNodesSetupHandler: &mock.GenesisNodesSetupHandlerStub{},
				ExportFolder:             "test",
			},
			exError: update.ErrNilHardforkStorer,
		},
		{
			name: "NilHasher",
			args: ArgsNewStateExporter{
				Marshalizer:              &cMock.MarshalizerMock{},
				StateSyncer:              &mock.SyncStateStub{},
				HardforkStorer:           &mock.HardforkStorerStub{},
				Hasher:                   nil,
				AddressPubKeyConverter:   &cMock.PubkeyConverterStub{},
				ValidatorPubKeyConverter: &cMock.PubkeyConverterStub{},
				GenesisNodesSetupHandler: &mock.GenesisNodesSetupHandlerStub{},
				ExportFolder:             "test",
			},
			exError: common.ErrNilHasher,
		},
		{
			name:            "NilAddressPubKeyConverter",
			requiresErrorIs: true,
			args: ArgsNewStateExporter{
				Marshalizer:              &cMock.MarshalizerMock{},
				StateSyncer:              &mock.SyncStateStub{},
				HardforkStorer:           &mock.HardforkStorerStub{},
				Hasher:                   &cMock.HasherStub{},
				AddressPubKeyConverter:   nil,
				ValidatorPubKeyConverter: &cMock.PubkeyConverterStub{},
				GenesisNodesSetupHandler: &mock.GenesisNodesSetupHandlerStub{},
				ExportFolder:             "test",
			},
			exError: common.ErrNilPubKeyConverter,
		},
		{
			name:            "NilValidatorPubKeyConverter",
			requiresErrorIs: true,
			args: ArgsNewStateExporter{
				Marshalizer:              &cMock.MarshalizerMock{},
				StateSyncer:              &mock.SyncStateStub{},
				HardforkStorer:           &mock.HardforkStorerStub{},
				Hasher:                   &cMock.HasherStub{},
				AddressPubKeyConverter:   &cMock.PubkeyConverterStub{},
				ValidatorPubKeyConverter: nil,
				GenesisNodesSetupHandler: &mock.GenesisNodesSetupHandlerStub{},
				ExportFolder:             "test",
			},
			exError: common.ErrNilPubKeyConverter,
		},
		{
			name: "NilGenesisNodesSetupHandler",
			args: ArgsNewStateExporter{
				Marshalizer:              &cMock.MarshalizerMock{},
				StateSyncer:              &mock.SyncStateStub{},
				HardforkStorer:           &mock.HardforkStorerStub{},
				Hasher:                   &cMock.HasherStub{},
				AddressPubKeyConverter:   &cMock.PubkeyConverterStub{},
				ValidatorPubKeyConverter: &cMock.PubkeyConverterStub{},
				GenesisNodesSetupHandler: nil,
				ExportFolder:             "test",
			},
			exError: update.ErrNilGenesisNodesSetupHandler,
		},
		{
			name: "EmptyExportFolder",
			args: ArgsNewStateExporter{
				Marshalizer:              &cMock.MarshalizerMock{},
				StateSyncer:              &mock.SyncStateStub{},
				HardforkStorer:           &mock.HardforkStorerStub{},
				Hasher:                   &cMock.HasherStub{},
				AddressPubKeyConverter:   &cMock.PubkeyConverterStub{},
				ValidatorPubKeyConverter: &cMock.PubkeyConverterStub{},
				GenesisNodesSetupHandler: &mock.GenesisNodesSetupHandlerStub{},
				ExportFolder:             "",
			},
			exError: update.ErrEmptyExportFolderPath,
		},
		{
			name: "Ok",
			args: ArgsNewStateExporter{
				Marshalizer:              &cMock.MarshalizerMock{},
				StateSyncer:              &mock.SyncStateStub{},
				HardforkStorer:           &mock.HardforkStorerStub{},
				Hasher:                   &cMock.HasherStub{},
				AddressPubKeyConverter:   &cMock.PubkeyConverterStub{},
				ValidatorPubKeyConverter: &cMock.PubkeyConverterStub{},
				ExportFolder:             "test",
				GenesisNodesSetupHandler: &mock.GenesisNodesSetupHandlerStub{},
			},
			exError: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStateExporter(tt.args)
			if tt.requiresErrorIs {
				require.True(t, errors.Is(err, tt.exError))
			} else {
				require.Equal(t, err, tt.exError)
			}
		})
	}
}

func TestExportAll(t *testing.T) {
	t.Parallel()

	testFolderName := "testFiles"
	testPath := "./" + testFolderName
	defer func() {
		_ = os.RemoveAll(testPath)
	}()

	metaBlock := &block.Block{Header: &block.BlockHeader{Slot: 2, ChainID: []byte("chainId")}}
	unFinishedMetaBlocks := map[string]*block.Block{
		"hash": {Header: &block.BlockHeader{Slot: 1, ChainID: []byte("chainId")}},
	}

	tx := &transaction.Transaction{}
	stateSyncer := &mock.SyncStateStub{
		GetEpochStartMetaBlockCalled: func() (block *block.Block, err error) {
			return metaBlock, nil
		},
		GetUnFinishedMetaBlocksCalled: func() (map[string]*block.Block, error) {
			return unFinishedMetaBlocks, nil
		},
		GetAllTransactionsCalled: func() (m map[string]data.TransactionHandler, err error) {
			mt := make(map[string]data.TransactionHandler)
			mt["tx"] = tx
			return mt, nil
		},
	}

	defer func() {
		_ = os.RemoveAll("./" + testFolderName + "/")
	}()

	transactionsWereWrote := false
	epochStartMetablockWasWrote := false
	unFinishedMetablocksWereWrote := false
	hs := &mock.HardforkStorerStub{
		WriteCalled: func(identifier string, key []byte, value []byte) error {
			switch identifier {
			case TransactionsIdentifier:
				transactionsWereWrote = true
			case EpochStartMetaBlockIdentifier:
				epochStartMetablockWasWrote = true
			case UnFinishedMetaBlocksIdentifier:
				unFinishedMetablocksWereWrote = true

			}

			return nil
		},
	}

	args := ArgsNewStateExporter{
		Marshalizer:              &cMock.MarshalizerMock{},
		StateSyncer:              stateSyncer,
		HardforkStorer:           hs,
		Hasher:                   &cMock.HasherMock{},
		AddressPubKeyConverter:   &cMock.PubkeyConverterStub{},
		ValidatorPubKeyConverter: &cMock.PubkeyConverterStub{},
		ExportFolder:             "test",
		GenesisNodesSetupHandler: &mock.GenesisNodesSetupHandlerStub{},
	}

	stateExporter, _ := NewStateExporter(args)
	require.False(t, check.IfNil(stateExporter))

	err := stateExporter.ExportAll(1)
	require.Nil(t, err)

	assert.True(t, transactionsWereWrote)
	assert.True(t, epochStartMetablockWasWrote)
	assert.True(t, unFinishedMetablocksWereWrote)
}

func TestStateExport_ExportTrieShouldExportNodesSetupJson(t *testing.T) {
	t.Parallel()

	testFolderName := "testFilesExportNodes"
	_ = os.Mkdir(testFolderName, 0777)

	defer func() {
		_ = os.RemoveAll(testFolderName)
	}()

	hs := &mock.HardforkStorerStub{
		WriteCalled: func(identifier string, key []byte, value []byte) error {
			return nil
		},
	}

	pubKeyConv := &cMock.PubkeyConverterStub{
		EncodeCalled: func(pkBytes []byte) string {
			return string(pkBytes)
		},
	}

	args := ArgsNewStateExporter{
		Marshalizer:              &cMock.MarshalizerMock{},
		StateSyncer:              &mock.SyncStateStub{},
		HardforkStorer:           hs,
		Hasher:                   &cMock.HasherMock{},
		ExportFolder:             testFolderName,
		AddressPubKeyConverter:   pubKeyConv,
		ValidatorPubKeyConverter: pubKeyConv,
		GenesisNodesSetupHandler: &mock.GenesisNodesSetupHandlerStub{},
	}

	trie := &mock.TrieStub{
		RootCalled: func() ([]byte, error) {
			return []byte{}, nil
		},
		GetAllLeavesOnChannelCalled: func(rootHash []byte) (chan data.KeyValueHolder, error) {
			ch := make(chan data.KeyValueHolder)

			mm := &cMock.MarshalizerMock{}
			valInfo := &state.ValidatorInfo{List: string(core.EligibleList)}
			pacB, _ := mm.Marshal(valInfo)

			go func() {
				ch <- keyValStorage.NewKeyValStorage([]byte("test"), pacB)
				close(ch)
			}()

			return ch, nil
		},
	}

	stateExporter, err := NewStateExporter(args)
	require.NoError(t, err)

	require.False(t, check.IfNil(stateExporter))

	err = stateExporter.exportTrie("test@1@9", trie)
	require.NoError(t, err)
}

func TestStateExport_ExportNodesSetupJsonShouldExportKeysInAlphabeticalOrder(t *testing.T) {
	t.Parallel()

	testFolderName := "testFilesExportNodes2"
	_ = os.Mkdir(testFolderName, 0777)

	defer func() {
		_ = os.RemoveAll(testFolderName)
	}()

	hs := &mock.HardforkStorerStub{
		WriteCalled: func(identifier string, key []byte, value []byte) error {
			return nil
		},
	}

	pubKeyConv := &cMock.PubkeyConverterStub{
		EncodeCalled: func(pkBytes []byte) string {
			return string(pkBytes)
		},
	}

	args := ArgsNewStateExporter{
		Marshalizer:              &cMock.MarshalizerMock{},
		StateSyncer:              &mock.SyncStateStub{},
		HardforkStorer:           hs,
		Hasher:                   &cMock.HasherMock{},
		ExportFolder:             testFolderName,
		AddressPubKeyConverter:   pubKeyConv,
		ValidatorPubKeyConverter: pubKeyConv,
		GenesisNodesSetupHandler: &mock.GenesisNodesSetupHandlerStub{},
	}

	stateExporter, err := NewStateExporter(args)
	require.NoError(t, err)

	require.False(t, check.IfNil(stateExporter))

	val50 := &state.ValidatorInfo{PublicKey: []byte("aaa"), List: string(core.EligibleList)}
	val51 := &state.ValidatorInfo{PublicKey: []byte("bbb"), List: string(core.EligibleList)}
	val10 := &state.ValidatorInfo{PublicKey: []byte("ccc"), List: string(core.EligibleList)}
	val11 := &state.ValidatorInfo{PublicKey: []byte("ddd"), List: string(core.EligibleList)}
	val00 := &state.ValidatorInfo{PublicKey: []byte("aaaaaa"), List: string(core.EligibleList)}
	val01 := &state.ValidatorInfo{PublicKey: []byte("bbbbbb"), List: string(core.EligibleList)}
	vals := []*state.ValidatorInfo{val50, val51, val00, val01, val10, val11}
	err = stateExporter.exportNodesSetupJson(vals)
	require.Nil(t, err)

	var nodesSetup sharding.NodesSetup

	nsBytes, err := os.ReadFile(filepath.Join(testFolderName, core.NodesSetupJsonFileName))
	require.NoError(t, err)

	err = json.Unmarshal(nsBytes, &nodesSetup)
	require.NoError(t, err)

	initialNodes := nodesSetup.InitialNodes

	// results should be in alphabetical order, sorted by public key
	require.Equal(t, string(val50.PublicKey), initialNodes[0].PubKey) // aaa
	require.Equal(t, string(val00.PublicKey), initialNodes[1].PubKey) // aaaaaa
	require.Equal(t, string(val51.PublicKey), initialNodes[2].PubKey) // bbb
	require.Equal(t, string(val01.PublicKey), initialNodes[3].PubKey) // bbbbbb
	require.Equal(t, string(val10.PublicKey), initialNodes[4].PubKey) // ccc
	require.Equal(t, string(val11.PublicKey), initialNodes[5].PubKey) // ddd
}

func TestStateExport_ExportUnfinishedMetaBlocksShouldWork(t *testing.T) {
	t.Parallel()

	unFinishedMetaBlocks := map[string]*block.Block{
		"hash": {Header: &block.BlockHeader{Slot: 1, ChainID: []byte("chainId")}},
	}
	stateSyncer := &mock.SyncStateStub{
		GetUnFinishedMetaBlocksCalled: func() (map[string]*block.Block, error) {
			return unFinishedMetaBlocks, nil
		},
	}

	unFinishedMetablocksWereWrote := false
	hs := &mock.HardforkStorerStub{
		WriteCalled: func(identifier string, key []byte, value []byte) error {
			if strings.Compare(identifier, UnFinishedMetaBlocksIdentifier) == 0 {
				unFinishedMetablocksWereWrote = true
			}
			return nil
		},
	}

	args := ArgsNewStateExporter{
		Marshalizer:              &cMock.MarshalizerMock{},
		StateSyncer:              stateSyncer,
		HardforkStorer:           hs,
		Hasher:                   &cMock.HasherMock{},
		AddressPubKeyConverter:   &cMock.PubkeyConverterStub{},
		ValidatorPubKeyConverter: &cMock.PubkeyConverterStub{},
		ExportFolder:             "test",
		GenesisNodesSetupHandler: &mock.GenesisNodesSetupHandlerStub{},
	}

	stateExporter, _ := NewStateExporter(args)
	require.False(t, check.IfNil(stateExporter))

	err := stateExporter.exportUnFinishedMetaBlocks()
	require.Nil(t, err)

	assert.True(t, unFinishedMetablocksWereWrote)
}
