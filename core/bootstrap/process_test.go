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
	"github.com/klever-io/klever-go/data/state"
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

func TestEpochStartBootstrap_PeerAccountToValidatorInfo(t *testing.T) {
	t.Parallel()

	t.Run("Correctly maps Index from PeerAccount", func(t *testing.T) {
		args := createMockEpochStartBootstrapArgs()
		epochStartBootstrap, err := NewEpochStartBootstrap(args)
		require.NoError(t, err)
		require.NotNil(t, epochStartBootstrap)

		// Create a peer account and set its properties
		peerAcc, err := state.NewPeerAccount([]byte("peerAddress"))
		require.NoError(t, err)

		expectedIndex := uint32(123)
		expectedOwnerAddress := []byte("ownerAddress12345678901234567890")
		expectedBLSPubKey := []byte("blsPublicKey12345678901234567890")

		// Set the Index using SetListAndIndex
		peerAcc.SetListAndIndex(state.List_eligible, expectedIndex)
		err = peerAcc.SetOwnerAddress(expectedOwnerAddress)
		require.NoError(t, err)
		err = peerAcc.SetBLSPublicKey(expectedBLSPubKey)
		require.NoError(t, err)
		peerAcc.SetTempRating(85)
		peerAcc.SetRating(90)
		peerAcc.IncreaseLeaderSuccessRate(20)
		peerAcc.IncreaseValidatorSuccessRate(100)

		// Call PeerAccountToValidatorInfo
		validatorInfo := epochStartBootstrap.PeerAccountToValidatorInfo(peerAcc)

		// Assert that Index is correctly mapped
		assert.Equal(t, expectedIndex, validatorInfo.Index, "Index should match the value from PeerAccount.GetIndex()")
		assert.Equal(t, expectedOwnerAddress, validatorInfo.OwnerAddress)
		assert.Equal(t, expectedBLSPubKey, validatorInfo.PublicKey)
		assert.Equal(t, "eligible", validatorInfo.List)
		assert.Equal(t, uint32(85), validatorInfo.TempRating)
		assert.Equal(t, uint32(90), validatorInfo.Rating)
		assert.Equal(t, false, validatorInfo.IsPubKeyRevoked)
	})

	t.Run("Maps Index correctly for revoked validator", func(t *testing.T) {
		args := createMockEpochStartBootstrapArgs()
		epochStartBootstrap, err := NewEpochStartBootstrap(args)
		require.NoError(t, err)

		// Create a real peer account for revoked validator
		peerAcc, err := state.NewPeerAccount([]byte("revokedPeerAddress"))
		require.NoError(t, err)

		expectedIndex := uint32(456)
		peerAcc.SetListAndIndex(state.List_jailed, expectedIndex)
		peerAcc.SetRevoked()
		peerAcc.SetTempRating(20)

		validatorInfo := epochStartBootstrap.PeerAccountToValidatorInfo(peerAcc)

		// Assert Index is preserved even for revoked validators
		assert.Equal(t, expectedIndex, validatorInfo.Index, "Index should be preserved for revoked validators")
		assert.Equal(t, "jailed", validatorInfo.List)
		assert.Equal(t, true, validatorInfo.IsPubKeyRevoked)
	})
}
