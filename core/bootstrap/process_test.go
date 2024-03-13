package bootstrap

import (
	"errors"
	"testing"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	consensusMock "github.com/klever-io/klever-go/core/consensus/mock"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createMockEpochStartBootstrapArgs() ArgsEpochStartBootstrap {
	return ArgsEpochStartBootstrap{
		PublicKey: &cryptoMock.PublicKeyStub{
			ToByteArrayStub: func() ([]byte, error) {
				return []byte("pubKey"), nil
			},
		},
		Marshalizer:       &mock.MarshalizerMock{},
		TxSignMarshalizer: &mock.MarshalizerMock{},
		Hasher:            &mock.HasherMock{},
		Messenger:         &mock.MessengerStub{},
		GeneralConfig: config.Config{
			WhiteListPool: config.CacheConfig{
				Type:     "LRU",
				Capacity: 10,
				Shards:   10,
			},
			TrieSync: config.TrieSyncConfig{
				NumConcurrentTrieSyncers:  200,
				MaxHardCapForMissingNodes: 5000,
			},
			StateTriesConfig: config.StateTriesConfig{
				CheckpointSlotsModulus:      5,
				AccountsStatePruningEnabled: true,
				PeerStatePruningEnabled:     true,
				KAppStatePruningEnabled:     true,
				MaxStateTrieLevelInMemory:   5,
				MaxPeerTrieLevelInMemory:    5,
				MaxKAppTrieLevelInMemory:    5,
			},
			EvictionWaitingList: config.EvictionWaitingListConfig{
				Size: 100,
				DB: config.DBConfig{
					FilePath:          "EvictionWaitingList",
					Type:              "MemoryDB",
					BatchDelaySeconds: 30,
					MaxBatchSize:      6,
					MaxOpenFiles:      10,
				},
			},
			TrieSnapshotDB: config.DBConfig{
				FilePath:          "TrieSnapshot",
				Type:              "MemoryDB",
				BatchDelaySeconds: 30,
				MaxBatchSize:      6,
				MaxOpenFiles:      10,
			},
			AccountsTrieStorage: config.StorageConfig{
				Cache: config.CacheConfig{
					Capacity: 10000,
					Type:     "LRU",
					Shards:   1,
				},
				DB: config.DBConfig{
					FilePath:          "AccountsTrie/MainDB",
					Type:              "MemoryDB",
					BatchDelaySeconds: 30,
					MaxBatchSize:      6,
					MaxOpenFiles:      10,
				},
			},
			PeerAccountsTrieStorage: config.StorageConfig{
				Cache: config.CacheConfig{
					Capacity: 10000,
					Type:     "LRU",
					Shards:   1,
				},
				DB: config.DBConfig{
					FilePath:          "PeerAccountsTrie/MainDB",
					Type:              "MemoryDB",
					BatchDelaySeconds: 30,
					MaxBatchSize:      6,
					MaxOpenFiles:      10,
				},
			},
			KAppAccountsTrieStorage: config.StorageConfig{
				Cache: config.CacheConfig{
					Capacity: 10000,
					Type:     "LRU",
					Shards:   1,
				},
				DB: config.DBConfig{
					FilePath:          "StakingAccountsTrie/MainDB",
					Type:              "MemoryDB",
					BatchDelaySeconds: 30,
					MaxBatchSize:      6,
					MaxOpenFiles:      10,
				},
			},
			TrieStorageManagerConfig: config.TrieStorageManagerConfig{
				PruningBufferLen:   1000,
				SnapshotsBufferLen: 10,
				MaxSnapshots:       2,
			},
		},
		SingleSigner:              &cryptoMock.SingleSignerStub{},
		BlockSingleSigner:         &cryptoMock.SingleSignerStub{},
		KeyGen:                    &cryptoMock.KeyGenMock{},
		BlockKeyGen:               &cryptoMock.KeyGenMock{},
		GenesisNodesConfig:        &mock.NodesSetupStub{},
		PathManager:               &mock.PathManagerStub{},
		WorkingDir:                "test_directory",
		DefaultDBPath:             "test_db",
		DefaultEpochString:        "test_epoch",
		Uint64Converter:           &mock.Uint64ByteSliceConverterMock{},
		NodeShuffler:              &mock.NodeShufflerMock{},
		SlotManager:               &consensusMock.SlotManagerMock{},
		AddressPubkeyConverter:    &cryptoMock.PubkeyConverterMock{},
		LatestStorageDataProvider: &mock.LatestStorageDataProviderStub{},
		StorageUnitOpener:         &mock.UnitOpenerStub{},
		StatusHandler:             &mock.AppStatusHandlerStub{},
		HeaderIntegrityVerifier:   &mock.HeaderIntegrityVerifierStub{},
		TxSignHasher:              &mock.HasherMock{},
		EpochNotifier:             &mock.EpochNotifierStub{},
	}
}

func TestNewEpochStartBootstrap(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, err)
	assert.False(t, check.IfNil(epochStartProvider))
}

func TestNewEpochStartBootstrap_NilTxSignHasherShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.TxSignHasher = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilHasher))
}

func TestNewEpochStartBootstrap_NilEpochNotifierShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.EpochNotifier = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilEpochNotifier))
}

func TestIsStartInEpochZero(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.GenesisNodesConfig = &mock.NodesSetupStub{
		GetStartTimeCalled: func() int64 {
			return 1000
		},
	}

	epochStartProvider, _ := NewEpochStartBootstrap(args)

	result := epochStartProvider.isStartInEpochZero()
	assert.False(t, result)
}

func TestEpochStartBootstrap_BootstrapStartInEpochNotEnabled(t *testing.T) {
	args := createMockEpochStartBootstrapArgs()

	err := errors.New("localErr")
	args.LatestStorageDataProvider = &mock.LatestStorageDataProviderStub{
		GetCalled: func() (storage.LatestDataFromStorage, error) {
			return storage.LatestDataFromStorage{}, err
		},
	}
	epochStartProvider, _ := NewEpochStartBootstrap(args)

	params, err := epochStartProvider.Bootstrap()
	assert.Nil(t, err)
	assert.NotNil(t, params)
}

func TestEpochStartBootstrap_Bootstrap(t *testing.T) {
	slotInterval := uint64(60000)
	args := createMockEpochStartBootstrapArgs()
	args.GenesisNodesConfig = &mock.NodesSetupStub{
		GetSlotIntervalCalled: func() uint64 {
			return slotInterval
		},
	}
	args.GeneralConfig = mock.GetGeneralConfig()
	epochStartProvider, _ := NewEpochStartBootstrap(args)

	done := make(chan bool, 1)

	go func() {
		_, _ = epochStartProvider.Bootstrap()
		<-done
	}()

	for {
		select {
		case <-done:
			assert.Fail(t, "should not be reach")
		case <-time.After(time.Second):
			assert.True(t, true, "pass with timeout")
			return
		}
	}
}

func TestPrepareForEpochZero(t *testing.T) {
	args := createMockEpochStartBootstrapArgs()

	epochStartProvider, _ := NewEpochStartBootstrap(args)

	params, err := epochStartProvider.prepareEpochZero()
	assert.Nil(t, err)
	assert.Equal(t, uint32(0), params.Epoch)
}

func TestCreateSyncers(t *testing.T) {
	args := createMockEpochStartBootstrapArgs()

	epochStartProvider, _ := NewEpochStartBootstrap(args)
	epochStartProvider.dataPool = &mock.PoolsHolderStub{
		HeadersCalled: func() retriever.HeadersPool {
			return &mock.HeadersCacherStub{}
		},
		TransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return mock.NewShardedDataStub()
		},
		UnsignedTransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return mock.NewShardedDataStub()
		},
		RewardTransactionsCalled: func() retriever.ShardedDataCacherNotifier {
			return mock.NewShardedDataStub()
		},
		BlocksCalled: func() storage.Cacher {
			return mock.NewCacherStub()
		},
		TrieNodesCalled: func() storage.Cacher {
			return mock.NewCacherStub()
		},
	}
	epochStartProvider.whiteListHandler = &mock.WhiteListHandlerStub{}
	epochStartProvider.whiteListerVerifiedTxs = &mock.WhiteListHandlerStub{}
	epochStartProvider.requestHandler = &mock.RequestHandlerStub{}

	err := epochStartProvider.createSyncers()
	assert.Nil(t, err)
}

func TestSyncValidatorAccountsState_NilRequestHandlerErr(t *testing.T) {
	args := createMockEpochStartBootstrapArgs()
	args.GeneralConfig = mock.GetGeneralConfig()
	epochStartProvider, _ := NewEpochStartBootstrap(args)
	epochStartProvider.dataPool = &mock.PoolsHolderStub{
		TrieNodesCalled: func() storage.Cacher {
			return &mock.CacherStub{
				GetCalled: func(key []byte) (value interface{}, ok bool) {
					return nil, true
				},
			}
		},
	}

	err := epochStartProvider.createTriesComponents()
	require.Nil(t, err)

	rootHash := []byte("rootHash")
	err = epochStartProvider.syncValidatorAccountsState(rootHash)
	assert.Equal(t, common.ErrNilRequestHandler, err)
}

func TestCreateTriesForNewShardID(t *testing.T) {
	args := createMockEpochStartBootstrapArgs()
	epochStartProvider, _ := NewEpochStartBootstrap(args)

	err := epochStartProvider.createTriesComponents()
	assert.Nil(t, err)
}

func TestSyncUserAccountsState(t *testing.T) {
	args := createMockEpochStartBootstrapArgs()
	epochStartProvider, _ := NewEpochStartBootstrap(args)
	epochStartProvider.dataPool = &mock.PoolsHolderStub{
		TrieNodesCalled: func() storage.Cacher {
			return &mock.CacherStub{
				GetCalled: func(key []byte) (value interface{}, ok bool) {
					return nil, true
				},
			}
		},
	}

	err := epochStartProvider.createTriesComponents()
	require.Nil(t, err)

	rootHash := []byte("rootHash")
	err = epochStartProvider.syncUserAccountsState(rootHash)
	assert.Equal(t, common.ErrNilRequestHandler, err)
}
