package genesis

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/common"
	cMock "github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/trie/factory"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/update"
	"github.com/klever-io/klever-go/update/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//TODO increase code coverage

func TestNewStateImport(t *testing.T) {
	trieStorageManagers := make(map[string]data.StorageManager)
	trieStorageManagers[factory.UserAccountTrie] = &cMock.StorageManagerStub{}
	tests := []struct {
		name    string
		args    ArgsNewStateImport
		exError error
	}{
		{
			name: "NilHardforkStorer",
			args: ArgsNewStateImport{
				HardforkStorer:      nil,
				Marshalizer:         &cMock.MarshalizerMock{},
				Hasher:              &cMock.HasherStub{},
				TrieStorageManagers: trieStorageManagers,
			},
			exError: update.ErrNilHardforkStorer,
		},
		{
			name: "NilMarshalizer",
			args: ArgsNewStateImport{
				HardforkStorer:      &mock.HardforkStorerStub{},
				Marshalizer:         nil,
				Hasher:              &cMock.HasherStub{},
				TrieStorageManagers: trieStorageManagers,
			},
			exError: common.ErrNilMarshalizer,
		},
		{
			name: "NilHasher",
			args: ArgsNewStateImport{
				HardforkStorer:      &mock.HardforkStorerStub{},
				Marshalizer:         &cMock.MarshalizerMock{},
				Hasher:              nil,
				TrieStorageManagers: trieStorageManagers,
			},
			exError: common.ErrNilHasher,
		},
		{
			name: "Ok",
			args: ArgsNewStateImport{
				HardforkStorer:      &mock.HardforkStorerStub{},
				Marshalizer:         &cMock.MarshalizerMock{},
				Hasher:              &cMock.HasherStub{},
				TrieStorageManagers: trieStorageManagers,
			},
			exError: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStateImport(tt.args)
			require.Equal(t, tt.exError, err)
		})
	}
}

func TestImportAll(t *testing.T) {
	t.Parallel()

	trieStorageManagers := make(map[string]data.StorageManager)
	trieStorageManagers[factory.UserAccountTrie] = &cMock.StorageManagerStub{}
	trieStorageManagers[factory.PeerAccountTrie] = &cMock.StorageManagerStub{}

	args := ArgsNewStateImport{
		HardforkStorer:      &mock.HardforkStorerStub{},
		Hasher:              &cMock.HasherMock{},
		Marshalizer:         &cMock.MarshalizerMock{},
		TrieStorageManagers: trieStorageManagers,
		ShardID:             0,
		StorageConfig:       config.StorageConfig{},
	}

	importState, _ := NewStateImport(args)
	require.False(t, check.IfNil(importState))

	err := importState.ImportAll()
	require.Nil(t, err)
}

func TestStateImport_ImportUnFinishedMetaBlocksShouldWork(t *testing.T) {
	t.Parallel()

	trieStorageManagers := make(map[string]data.StorageManager)
	trieStorageManagers[factory.UserAccountTrie] = &cMock.StorageManagerStub{}

	hasher := &cMock.HasherMock{}
	marshahlizer := &cMock.MarshalizerMock{}
	metaBlock := &block.Block{
		Header: &block.BlockHeader{
			Slot:    1,
			ChainID: []byte("chainId"),
		},
	}
	metaBlockHash, _ := tools.CalculateHash(marshahlizer, hasher, metaBlock.Header)

	args := ArgsNewStateImport{
		HardforkStorer: &mock.HardforkStorerStub{
			GetCalled: func(identifier string, key []byte) ([]byte, error) {
				return marshahlizer.Marshal(metaBlock)
			},
		},
		Hasher:              hasher,
		Marshalizer:         marshahlizer,
		TrieStorageManagers: trieStorageManagers,
		ShardID:             0,
		StorageConfig:       config.StorageConfig{},
	}

	importState, _ := NewStateImport(args)
	require.False(t, check.IfNil(importState))

	key := fmt.Sprintf("meta@chainId@%s", hex.EncodeToString(metaBlockHash))
	err := importState.importUnFinishedMetaBlocks(UnFinishedMetaBlocksIdentifier, [][]byte{
		[]byte(key),
	})

	require.Nil(t, err)
	assert.Equal(t, importState.importedUnFinishedMetaBlocks[string(metaBlockHash)], metaBlock)
}
