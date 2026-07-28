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
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/epochStart"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/trie"
	"github.com/klever-io/klever-go/sharding"
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
		ForkController:            &mock.ForkControllerStub{},
	}
}

func TestNewEpochStartBootstrap(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, err)
	assert.False(t, check.IfNil(epochStartProvider))
}

func TestNewEpochStartBootstrap_NilPathManagerShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.PathManager = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilPathManager))
}

func TestNewEpochStartBootstrap_NilMessengerShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.Messenger = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilMessenger))
}

func TestNewEpochStartBootstrap_NilPublicKeyShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.PublicKey = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, sharding.ErrNilPubKey))
}

func TestNewEpochStartBootstrap_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.Marshalizer = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilMarshalizer))
}

func TestNewEpochStartBootstrap_NilBlockKeyGenShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.BlockKeyGen = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilBlockKeyGen))
}

func TestNewEpochStartBootstrap_NilKeyGenShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.KeyGen = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilKeyGen))
}

func TestNewEpochStartBootstrap_NilSingleSignerShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.SingleSigner = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilSingleSigner))
}

func TestNewEpochStartBootstrap_NilBlockSingleSignerShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.BlockSingleSigner = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilBlockSingleSigner))
}

func TestNewEpochStartBootstrap_NilTxSignMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.TxSignMarshalizer = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilTxSignMarshalizer))
}

func TestNewEpochStartBootstrap_NilGenesisNodesConfigShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.GenesisNodesConfig = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilGenesisNodesConfig))
}

func TestNewEpochStartBootstrap_NilTxSignHasherShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.TxSignHasher = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilHasher))
}

func TestNewEpochStartBootstrap_InvalidDefaultDBPathShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.DefaultDBPath = ""

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrInvalidDefaultDBPath))
}

func TestNewEpochStartBootstrap_NilPubkeyConverterShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.AddressPubkeyConverter = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilPubkeyConverter))
}

func TestNewEpochStartBootstrap_InvalidDefaultEpochStringShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.DefaultEpochString = ""

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrInvalidDefaultEpochString))
}

func TestNewEpochStartBootstrap_InvalidWorkingDirShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.WorkingDir = ""

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrInvalidWorkingDir))
}

func TestNewEpochStartBootstrap_NilSlotManagerShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.SlotManager = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilSlotManager))
}

func TestNewEpochStartBootstrap_NilStorageUnitOpenerShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.StorageUnitOpener = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilStorageUnitOpener))
}

func TestNewEpochStartBootstrap_NilLatestStorageDataProviderShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.LatestStorageDataProvider = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilLatestStorageDataProvider))
}

func TestNewEpochStartBootstrap_NilUint64ConverterShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.Uint64Converter = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilUint64Converter))
}

func TestNewEpochStartBootstrap_NilNodeShufflerShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.NodeShuffler = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilShuffler))
}

func TestNewEpochStartBootstrap_NilStatusHandlerShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.StatusHandler = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilStatusHandler))
}

func TestNewEpochStartBootstrap_NilHeaderIntegrityVerifierShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.HeaderIntegrityVerifier = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilHeaderIntegrityVerifier))
}

func TestNewEpochStartBootstrap_NilEpochNotifierShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.EpochNotifier = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilEpochNotifier))
}

func TestNewEpochStartBootstrap_NilForkControllerShouldErr(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	args.ForkController = nil

	epochStartProvider, err := NewEpochStartBootstrap(args)
	assert.Nil(t, epochStartProvider)
	assert.True(t, errors.Is(err, common.ErrNilForkController))
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

	rootHash := make([]byte, len(trie.EmptyTrieHash))
	rootHash[0] = 1
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

	rootHash := make([]byte, len(trie.EmptyTrieHash))
	rootHash[0] = 1
	err = epochStartProvider.syncUserAccountsState(rootHash)
	assert.Equal(t, common.ErrNilRequestHandler, err)
}

func TestValidateStateTrieRootHash(t *testing.T) {
	t.Parallel()

	t.Run("nil root hash should be rejected as invalid length", func(t *testing.T) {
		t.Parallel()

		err := validateStateTrieRootHash(nil)
		assert.Equal(t, common.ErrInvalidRootHash, err)
	})
	t.Run("zero-length root hash should be rejected as invalid length", func(t *testing.T) {
		t.Parallel()

		err := validateStateTrieRootHash([]byte{})
		assert.Equal(t, common.ErrInvalidRootHash, err)
	})
	t.Run("short (non 32-byte) root hash should be rejected as invalid length", func(t *testing.T) {
		t.Parallel()

		err := validateStateTrieRootHash([]byte("short"))
		assert.Equal(t, common.ErrInvalidRootHash, err)
	})
	t.Run("canonical empty trie hash should be rejected", func(t *testing.T) {
		t.Parallel()

		err := validateStateTrieRootHash(trie.EmptyTrieHash)
		assert.Equal(t, common.ErrEmptyStateTrieRootHash, err)
	})
	t.Run("non-empty 32-byte root hash should be accepted", func(t *testing.T) {
		t.Parallel()

		realRootHash := make([]byte, len(trie.EmptyTrieHash))
		realRootHash[0] = 1

		err := validateStateTrieRootHash(realRootHash)
		assert.Nil(t, err)
	})
}

func TestVerifySyncedTrieRootHash(t *testing.T) {
	t.Parallel()

	rootHash := []byte("rootHash")

	t.Run("missing synced trie should error", func(t *testing.T) {
		t.Parallel()

		err := verifySyncedTrieRootHash(map[string]data.Trie{}, rootHash)
		assert.ErrorIs(t, err, common.ErrMissingSyncedTrieAfterSync)
	})
	t.Run("nil synced trie should error", func(t *testing.T) {
		t.Parallel()

		syncedTries := map[string]data.Trie{string(rootHash): nil}
		err := verifySyncedTrieRootHash(syncedTries, rootHash)
		assert.ErrorIs(t, err, common.ErrMissingSyncedTrieAfterSync)
	})
	t.Run("RootHash error should be propagated", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("root hash error")
		syncedTries := map[string]data.Trie{
			string(rootHash): &mock.TrieStub{
				RootCalled: func() ([]byte, error) {
					return nil, expectedErr
				},
			},
		}
		err := verifySyncedTrieRootHash(syncedTries, rootHash)
		assert.Equal(t, expectedErr, err)
	})
	t.Run("mismatching synced root hash should error", func(t *testing.T) {
		t.Parallel()

		syncedTries := map[string]data.Trie{
			string(rootHash): &mock.TrieStub{
				RootCalled: func() ([]byte, error) {
					return []byte("a different root hash"), nil
				},
			},
		}
		err := verifySyncedTrieRootHash(syncedTries, rootHash)
		assert.ErrorIs(t, err, common.ErrTrieRootHashMismatch)
	})
	t.Run("matching synced root hash should pass", func(t *testing.T) {
		t.Parallel()

		syncedTries := map[string]data.Trie{
			string(rootHash): &mock.TrieStub{
				RootCalled: func() ([]byte, error) {
					return rootHash, nil
				},
			},
		}
		err := verifySyncedTrieRootHash(syncedTries, rootHash)
		assert.Nil(t, err)
	})
}

func TestSyncAccountsState_EmptyRootHashIsRejected(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	epochStartProvider, _ := NewEpochStartBootstrap(args)

	// Malformed-length roots (nil, zero-length) must be rejected as invalid before any sync is
	// attempted, guarding fast bootstrap against peers advertising empty state roots.
	assert.Equal(t, common.ErrInvalidRootHash, epochStartProvider.syncUserAccountsState(nil))
	assert.Equal(t, common.ErrInvalidRootHash, epochStartProvider.syncValidatorAccountsState([]byte{}))
	assert.Equal(t, common.ErrInvalidRootHash, epochStartProvider.syncKappAccountsState(nil))

	// The canonical empty trie hash has a valid length but still bootstraps an empty state, so it
	// must be rejected too instead of short-circuiting the syncer.
	assert.Equal(t, common.ErrEmptyStateTrieRootHash, epochStartProvider.syncUserAccountsState(trie.EmptyTrieHash))
	assert.Equal(t, common.ErrEmptyStateTrieRootHash, epochStartProvider.syncValidatorAccountsState(trie.EmptyTrieHash))
	assert.Equal(t, common.ErrEmptyStateTrieRootHash, epochStartProvider.syncKappAccountsState(trie.EmptyTrieHash))
}

// accountsDBSyncerStub is a configurable epochStart.AccountsDBSyncer used to drive
// syncAndVerifyAccountsState through each of its branches without a real trie syncer.
type accountsDBSyncerStub struct {
	SyncAccountsCalled   func(rootHash []byte) error
	GetSyncedTriesCalled func() map[string]data.Trie
}

func (a *accountsDBSyncerStub) SyncAccounts(rootHash []byte) error {
	if a.SyncAccountsCalled != nil {
		return a.SyncAccountsCalled(rootHash)
	}
	return nil
}

func (a *accountsDBSyncerStub) GetSyncedTries() map[string]data.Trie {
	if a.GetSyncedTriesCalled != nil {
		return a.GetSyncedTriesCalled()
	}
	return nil
}

func (a *accountsDBSyncerStub) IsInterfaceNil() bool {
	return a == nil
}

func TestSyncAndVerifyAccountsState(t *testing.T) {
	t.Parallel()

	args := createMockEpochStartBootstrapArgs()
	epochStartProvider, _ := NewEpochStartBootstrap(args)

	validRootHash := make([]byte, len(trie.EmptyTrieHash))
	validRootHash[0] = 1

	t.Run("invalid root hash is rejected before the syncer is built", func(t *testing.T) {
		t.Parallel()

		syncerBuilt := false
		syncedTries, err := epochStartProvider.syncAndVerifyAccountsState(
			trie.EmptyTrieHash,
			func() (epochStart.AccountsDBSyncer, error) {
				syncerBuilt = true
				return &accountsDBSyncerStub{}, nil
			},
		)
		assert.Equal(t, common.ErrEmptyStateTrieRootHash, err)
		assert.Nil(t, syncedTries)
		assert.False(t, syncerBuilt, "syncer must not be built for an invalid root hash")
	})
	t.Run("syncer factory error is propagated", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("cannot build syncer")
		syncedTries, err := epochStartProvider.syncAndVerifyAccountsState(
			validRootHash,
			func() (epochStart.AccountsDBSyncer, error) {
				return nil, expectedErr
			},
		)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, syncedTries)
	})
	t.Run("SyncAccounts error is propagated", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("sync failed")
		syncedTries, err := epochStartProvider.syncAndVerifyAccountsState(
			validRootHash,
			func() (epochStart.AccountsDBSyncer, error) {
				return &accountsDBSyncerStub{
					SyncAccountsCalled: func(_ []byte) error {
						return expectedErr
					},
				}, nil
			},
		)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, syncedTries)
	})
	t.Run("mismatching synced trie root hash is rejected", func(t *testing.T) {
		t.Parallel()

		syncedTries, err := epochStartProvider.syncAndVerifyAccountsState(
			validRootHash,
			func() (epochStart.AccountsDBSyncer, error) {
				return &accountsDBSyncerStub{
					GetSyncedTriesCalled: func() map[string]data.Trie {
						return map[string]data.Trie{
							string(validRootHash): &mock.TrieStub{
								RootCalled: func() ([]byte, error) {
									return []byte("a different root hash"), nil
								},
							},
						}
					},
				}, nil
			},
		)
		assert.ErrorIs(t, err, common.ErrTrieRootHashMismatch)
		assert.Nil(t, syncedTries)
	})
	t.Run("successful sync returns the verified tries", func(t *testing.T) {
		t.Parallel()

		expectedTries := map[string]data.Trie{
			string(validRootHash): &mock.TrieStub{
				RootCalled: func() ([]byte, error) {
					return validRootHash, nil
				},
			},
		}
		syncedTries, err := epochStartProvider.syncAndVerifyAccountsState(
			validRootHash,
			func() (epochStart.AccountsDBSyncer, error) {
				return &accountsDBSyncerStub{
					GetSyncedTriesCalled: func() map[string]data.Trie {
						return expectedTries
					},
				}, nil
			},
		)
		assert.Nil(t, err)
		assert.Equal(t, expectedTries, syncedTries)
	})
}

func TestSyncKappAccountsState_NilRequestHandlerErr(t *testing.T) {
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

	rootHash := make([]byte, len(trie.EmptyTrieHash))
	rootHash[0] = 1
	err = epochStartProvider.syncKappAccountsState(rootHash)
	assert.Equal(t, common.ErrNilRequestHandler, err)
}

func createSyncableEpochStartBootstrap(t *testing.T) *epochStartBootstrap {
	args := createMockEpochStartBootstrapArgs()
	args.GeneralConfig = mock.GetGeneralConfig()
	epochStartProvider, _ := NewEpochStartBootstrap(args)
	epochStartProvider.dataPool = &mock.PoolsHolderStub{
		TrieNodesCalled: func() storage.Cacher {
			return mock.NewCacherStub()
		},
	}
	epochStartProvider.requestHandler = &mock.RequestHandlerStub{}

	err := epochStartProvider.createTriesComponents()
	require.Nil(t, err)

	return epochStartProvider
}

func TestSyncAccountsState_CanonicalEmptyTrieIsRejected(t *testing.T) {
	t.Parallel()

	// The canonical empty trie (trie.EmptyTrieHash, 32 zero bytes) would short-circuit the syncer and
	// bootstrap a completely empty state without requesting a single trie node. Even on a fully wired
	// provider it must be rejected up front, before the syncer is ever invoked, so no trie is stored.
	emptyRootHash := trie.EmptyTrieHash

	t.Run("user accounts", func(t *testing.T) {
		t.Parallel()

		epochStartProvider := createSyncableEpochStartBootstrap(t)

		err := epochStartProvider.syncUserAccountsState(emptyRootHash)
		assert.Equal(t, common.ErrEmptyStateTrieRootHash, err)
		assert.Nil(t, epochStartProvider.userAccountTries[string(emptyRootHash)])
	})
	t.Run("validator accounts", func(t *testing.T) {
		t.Parallel()

		epochStartProvider := createSyncableEpochStartBootstrap(t)

		err := epochStartProvider.syncValidatorAccountsState(emptyRootHash)
		assert.Equal(t, common.ErrEmptyStateTrieRootHash, err)
		assert.Nil(t, epochStartProvider.peerAccountTries[string(emptyRootHash)])
	})
	t.Run("kapp accounts", func(t *testing.T) {
		t.Parallel()

		epochStartProvider := createSyncableEpochStartBootstrap(t)

		err := epochStartProvider.syncKappAccountsState(emptyRootHash)
		assert.Equal(t, common.ErrEmptyStateTrieRootHash, err)
		assert.Nil(t, epochStartProvider.kappAccountTries[string(emptyRootHash)])
	})
}
