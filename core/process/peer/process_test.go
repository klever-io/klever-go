package peer_test

import (
	"errors"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core/fork"
	"github.com/klever-io/klever-go/core/keyValStorage"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/economics"
	"github.com/klever-io/klever-go/core/process/peer"
	cryptoMock "github.com/klever-io/klever-go/crypto/mock"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/sharding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	validatorIncreaseRatingStep = int32(1)
	validatorDecreaseRatingStep = int32(-2)
	proposerIncreaseRatingStep  = int32(2)
	proposerDecreaseRatingStep  = int32(-4)
	minRating                   = uint32(1)
	maxRating                   = uint32(100)
	startRating                 = uint32(50)
	defaultChancesSelection     = uint32(1)
	consensusGroupFormat        = "%s_%v_%v"
)

func createMockPubkeyConverter() *cryptoMock.PubkeyConverterMock {
	return cryptoMock.NewPubkeyConverterMock(32)
}

func createMockArguments() peer.ArgValidatorStatisticsProcessor {
	epochNotifier := &mock.EpochNotifierStub{}
	forkController, _ := fork.NewForkController(config.EnableEpochs{
		ClaimKFI:              0,
		ProcessorFlowITOPrice: 0,
		FixStakingBuckets:     0,
		KdaFpr:                0,
	}, epochNotifier)

	proposalController, _ := kapps.NewProposalController(forkController)

	argsNewEconomicsData := economics.ArgsNewEconomicsData{
		EpochNotifier: &mock.EpochNotifierStub{},
	}
	economicsData, _ := economics.NewEconomicsData(argsNewEconomicsData)

	_ = economicsData.SetProposalController(proposalController)

	arguments := peer.ArgValidatorStatisticsProcessor{
		Marshalizer: &mock.ProtoMarshalizerMock{},
		DataPool: &mock.PoolsHolderStub{
			HeadersCalled: func() retriever.HeadersPool {
				return nil
			},
		},
		StorageService: &mock.ChainStorerMock{},
		NodesCoordinator: &mock.NodesCoordinatorMock{
			ComputeValidatorsGroupCalled: func(
				_ []byte,
				_ uint64, _ uint32,
			) (validatorsGroup []sharding.Validator, err error) {
				return []sharding.Validator{
					mock.NewValidatorMock([]byte("address1"), []byte("pubkey1"), 1, 1),
					mock.NewValidatorMock([]byte("address2"), []byte("pubkey2"), 1, 1),
				}, nil
			},
		},
		PubkeyConv:         createMockPubkeyConverter(),
		PeerAdapter:        getAccountsMock(),
		Rater:              createMockRater(),
		RewardsHandler:     economicsData,
		MaxComputableSlots: 1000,
		NodesSetup:         &mock.NodesSetupStub{},
		EpochNotifier:      &mock.EpochNotifierStub{},
		VKApp:              &mock.ValidatorsKAppStub{},
	}

	return arguments
}

func createMockRater() *mock.RaterMock {
	rater := mock.GetNewMockRater()
	rater.MinRating = minRating
	rater.MaxRating = maxRating
	rater.StartRating = startRating
	rater.IncreaseProposer = proposerIncreaseRatingStep
	rater.DecreaseProposer = proposerDecreaseRatingStep
	rater.IncreaseValidator = validatorIncreaseRatingStep
	rater.DecreaseValidator = validatorDecreaseRatingStep
	return rater
}

func getAccountsMock() *mock.AccountsStub {
	return &mock.AccountsStub{
		CommitCalled: func() (bytes []byte, e error) {
			return make([]byte, 32), nil
		},
		LoadAccountCalled: func(address []byte) (handler state.AccountHandler, e error) {
			return &mock.PeerAccountHandlerMock{}, nil
		},
		RootHashCalled: func() ([]byte, error) {
			return make([]byte, 32), nil
		},
	}
}

func TestNewValidatorStatisticsProcessor_NilPeerAdaptersShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.PeerAdapter = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrNilPeerAccountsAdapter, err)
}

func TestNewValidatorStatisticsProcessor_NilPubkeyConverterShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.PubkeyConv = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrNilPubkeyConverter, err)
}

func TestNewValidatorStatisticsProcessor_NilNodesCoordinatorShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.NodesCoordinator = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrNilNodesCoordinator, err)
}

func TestNewValidatorStatisticsProcessor_NilStorageShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.StorageService = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrNilStorage, err)
}

func TestNewValidatorStatisticsProcessor_ZeroMaxComputableSlotsUseDefault(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.MaxComputableSlots = 0
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)
	require.Nil(t, err)
	assert.Equal(t, process.DefaultMaxComputableSlots, validatorStatistics.MaxComputableSlots())
}

func TestNewValidatorStatisticsProcessor_NilRaterShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.Rater = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrNilRater, err)
}

func TestNewValidatorStatisticsProcessor_NilRewardsHandlerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.RewardsHandler = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, process.ErrNilRewardsHandler, err)
}

func TestNewValidatorStatisticsProcessor_NilNodesSetupShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.NodesSetup = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrNilNodesSetup, err)
}

func TestNewValidatorStatisticsProcessor_NilEpochNotifierShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.EpochNotifier = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrNilEpochNotifier, err)
}

func TestNewValidatorStatisticsProcessor_NilVKAppShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.VKApp = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrNilKAppValidator, err)
}

func TestNewValidatorStatisticsProcessor_NilMarshalizerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.Marshalizer = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, process.ErrNilMarshalizer, err)
}

func TestNewValidatorStatisticsProcessor_NilDataPoolShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	arguments.DataPool = nil
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.Nil(t, validatorStatistics)
	assert.Equal(t, common.ErrNilDataPoolHolder, err)
}

func TestComputeEpoch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		currentHeader data.HeaderHandler
		expectedEpoch uint32
	}{
		{
			name: "regular block, not epoch start",
			currentHeader: &block.Block{Header: &block.BlockHeader{
				Epoch:        3,
				IsEpochStart: false,
			}},
			expectedEpoch: 3,
		},
		{
			name: "epoch start block, epoch > 0",
			currentHeader: &block.Block{Header: &block.BlockHeader{
				Epoch:        3,
				IsEpochStart: true,
			}},
			expectedEpoch: 2,
		},
		{
			name: "epoch start block, epoch = 0",
			currentHeader: &block.Block{Header: &block.BlockHeader{
				Epoch:        0,
				IsEpochStart: true,
			}},
			expectedEpoch: 0,
		},
		{
			name: "regular block, epoch = 0",
			currentHeader: &block.Block{Header: &block.BlockHeader{
				Epoch:        0,
				IsEpochStart: false,
			}},
			expectedEpoch: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			epoch := peer.ComputeEpoch(tt.currentHeader)
			assert.Equal(t, tt.expectedEpoch, epoch)
		})
	}
}

func TestComputeEpoch_WithMockHeader(t *testing.T) {
	t.Parallel()

	t.Run("mock header, not epoch start", func(t *testing.T) {
		header := &mock.HeaderHandlerStub{
			EpochField: 5,
			GetIsEpochStartCalled: func() bool {
				return false
			},
		}

		epoch := peer.ComputeEpoch(header)
		assert.Equal(t, uint32(5), epoch)
	})

	t.Run("mock header, epoch start", func(t *testing.T) {
		header := &mock.HeaderHandlerStub{
			EpochField: 5,
			GetIsEpochStartCalled: func() bool {
				return true
			},
		}

		epoch := peer.ComputeEpoch(header)
		assert.Equal(t, uint32(4), epoch)
	})
}

func TestNewValidatorStatisticsProcessor_Ok(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	validatorStatistics, err := peer.NewValidatorStatisticsProcessor(arguments)

	assert.NotNil(t, validatorStatistics)
	assert.Nil(t, err)
}

func TestValidatorStatistics_SaveNodesCoordinatorUpdates(t *testing.T) {
	t.Parallel()

	var expectedErr = errors.New("nodes coordinator error")

	t.Run("should return error when nodes coordinator fails", func(t *testing.T) {
		arguments := createMockArguments()
		arguments.NodesCoordinator = &mock.NodesCoordinatorMock{
			GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
				return nil, expectedErr
			},
		}

		vs, _ := peer.NewValidatorStatisticsProcessor(arguments)
		_, err := vs.SaveNodesCoordinatorUpdates(0)
		assert.Equal(t, expectedErr, err)
	})

	t.Run("should succeed with valid data", func(t *testing.T) {
		arguments := createMockArguments()
		arguments.NodesCoordinator = &mock.NodesCoordinatorMock{
			GetAllElectedValidatorsKeysCalled: func() ([][]byte, error) {
				return [][]byte{[]byte("elected1"), []byte("elected2")}, nil
			},
			GetAllEligibleValidatorsKeysCalled: func() ([][]byte, error) {
				return [][]byte{[]byte("eligible1"), []byte("eligible2")}, nil
			},
			GetAllWaitingValidatorsKeysCalled: func() ([][]byte, error) {
				return [][]byte{[]byte("waiting1")}, nil
			},
		}

		vs, _ := peer.NewValidatorStatisticsProcessor(arguments)
		_, err := vs.SaveNodesCoordinatorUpdates(0)
		assert.NoError(t, err)
	})
}

func TestValidatorStatistics_UpdatePeerState(t *testing.T) {
	t.Parallel()

	t.Run("Roothash zeros for genesis block", func(t *testing.T) {
		arguments := createMockArguments()
		vs, _ := peer.NewValidatorStatisticsProcessor(arguments)

		header := &block.Block{Header: &block.BlockHeader{Nonce: arguments.GenesisNonce}}
		previousHeader := &block.Block{Header: &block.BlockHeader{Nonce: arguments.GenesisNonce - 1}}

		roothash, err := vs.UpdatePeerState(header, previousHeader)
		assert.NoError(t, err)
		assert.Equal(t, make([]byte, 32), roothash)
	})

	t.Run("should handle missed blocks empty consensus group", func(t *testing.T) {
		arguments := createMockArguments()
		arguments.NodesCoordinator.(*mock.NodesCoordinatorMock).ComputeValidatorsGroupCalled = nil

		header := &block.Block{Header: &block.BlockHeader{Nonce: 10, PrevRandSeed: []byte("prev"), Slot: 100}}
		previousHeader := &block.Block{Header: &block.BlockHeader{Nonce: 5, RandSeed: []byte("rand"), Slot: 50}}

		vs, _ := peer.NewValidatorStatisticsProcessor(arguments)

		_, err := vs.UpdatePeerState(header, previousHeader)
		assert.Equal(t, process.ErrEmptyConsensusGroup, err)
	})

	t.Run("should handle missed blocks", func(t *testing.T) {
		arguments := createMockArguments()
		vs, _ := peer.NewValidatorStatisticsProcessor(arguments)

		header := &block.Block{Header: &block.BlockHeader{Nonce: 10, PrevRandSeed: []byte("prev"), Slot: 100}}
		previousHeader := &block.Block{Header: &block.BlockHeader{Nonce: 5, RandSeed: []byte("rand"), Slot: 50}, PubKeysBitmap: []byte{0, 0, 0, 0}}

		_, err := vs.UpdatePeerState(header, previousHeader)
		assert.NoError(t, err)
	})
}

func TestValidatorStatistics_CheckForMissedBlocks(t *testing.T) {
	t.Parallel()

	t.Run("no missed blocks", func(t *testing.T) {
		arguments := createMockArguments()
		vs, _ := peer.NewValidatorStatisticsProcessor(arguments)
		err := vs.CheckForMissedBlocks(10, 9, []byte("prevRandSeed"), 0)
		assert.NoError(t, err)
	})

	t.Run("missed blocks within computable range", func(t *testing.T) {
		arguments := createMockArguments()
		vs, _ := peer.NewValidatorStatisticsProcessor(arguments)
		err := vs.CheckForMissedBlocks(15, 10, []byte("prevRandSeed"), 0)
		assert.NoError(t, err)
	})

	t.Run("missed blocks exceed computable range", func(t *testing.T) {
		arguments := createMockArguments()
		arguments.MaxComputableSlots = 3
		vs, _ := peer.NewValidatorStatisticsProcessor(arguments)

		err := vs.CheckForMissedBlocks(15, 10, []byte("prevRandSeed"), 0)
		assert.NoError(t, err)
	})
}

func TestValidatorStatistics_RevertPeerState(t *testing.T) {
	t.Parallel()

	t.Run("should call RecreateTrie", func(t *testing.T) {
		wasCalled := false
		arguments := createMockArguments()
		arguments.PeerAdapter = &mock.AccountsStub{
			RecreateTrieCalled: func(rootHash []byte) error {
				wasCalled = true
				return nil
			},
		}

		vs, _ := peer.NewValidatorStatisticsProcessor(arguments)
		header := &block.Block{Header: &block.BlockHeader{ValidatorsTrieRoot: []byte("validatorRootHash")}}
		err := vs.RevertPeerState(header)

		assert.NoError(t, err)
		assert.True(t, wasCalled)
	})

	t.Run("should return error if RecreateTrie fails", func(t *testing.T) {
		expectedErr := errors.New("recreate trie error")
		arguments := createMockArguments()
		arguments.PeerAdapter = &mock.AccountsStub{
			RecreateTrieCalled: func(rootHash []byte) error {
				return expectedErr
			},
		}

		vs, _ := peer.NewValidatorStatisticsProcessor(arguments)
		header := &block.Block{Header: &block.BlockHeader{ValidatorsTrieRoot: []byte("validatorRootHash")}}
		err := vs.RevertPeerState(header)

		assert.Equal(t, expectedErr, err)
	})
}

func TestValidatorStatistics_SetAndGetLastFinalizedRootHash(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()

	t.Run("should set and get last finalized root hash", func(t *testing.T) {
		rootHash := []byte("rootHash")
		vs, _ := peer.NewValidatorStatisticsProcessor(arguments)
		vs.SetLastFinalizedRootHash(rootHash)

		retrievedRootHash := vs.LastFinalizedRootHash()
		assert.Equal(t, rootHash, retrievedRootHash)
	})

	t.Run("should not set empty root hash", func(t *testing.T) {
		initialRootHash := []byte("initialRootHash")
		vs, _ := peer.NewValidatorStatisticsProcessor(arguments)
		vs.SetLastFinalizedRootHash(initialRootHash)

		vs.SetLastFinalizedRootHash([]byte{})

		retrievedRootHash := vs.LastFinalizedRootHash()
		assert.Equal(t, initialRootHash, retrievedRootHash)
	})
}

func TestValidatorStatistics_ResetValidatorStatisticsAtNewEpoch(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	vkAppMock := &mock.ValidatorsKAppStub{
		ResetValidatorStatisticsAtNewEpochCalled: func(vInfo []*state.ValidatorInfo) ([]*state.ValidatorInfo, error) {
			for _, v := range vInfo {
				v.LeaderSuccess = 0
				v.LeaderFailure = 0
				v.ValidatorSuccess = 0
				v.ValidatorFailure = 0
				v.ValidatorIgnoredSignatures = 0
			}
			return vInfo, nil
		},
	}
	arguments.VKApp = vkAppMock
	vs, _ := peer.NewValidatorStatisticsProcessor(arguments)

	// Create test validator info
	validatorInfo := []*state.ValidatorInfo{
		{
			PublicKey:                  []byte("pk1"),
			LeaderSuccess:              10,
			LeaderFailure:              2,
			ValidatorSuccess:           50,
			ValidatorFailure:           5,
			ValidatorIgnoredSignatures: 1,
		},
		{
			PublicKey:                  []byte("pk2"),
			LeaderSuccess:              20,
			LeaderFailure:              3,
			ValidatorSuccess:           60,
			ValidatorFailure:           7,
			ValidatorIgnoredSignatures: 2,
		},
	}

	resetValidatorInfo, err := vs.ResetValidatorStatisticsAtNewEpoch(validatorInfo)

	require.Nil(t, err)
	require.Len(t, resetValidatorInfo, 2)

	for _, v := range resetValidatorInfo {
		assert.Equal(t, uint32(0), v.LeaderSuccess)
		assert.Equal(t, uint32(0), v.LeaderFailure)
		assert.Equal(t, uint32(0), v.ValidatorSuccess)
		assert.Equal(t, uint32(0), v.ValidatorFailure)
		assert.Equal(t, uint32(0), v.ValidatorIgnoredSignatures)
	}
}

func TestValidatorStatistics_ProcessRatingsEndOfEpoch(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	vkAppMock := &mock.ValidatorsKAppStub{
		ProcessRatingsEndOfEpochCalled: func(validatorInfos []*state.ValidatorInfo) error {
			for _, v := range validatorInfos {
				v.TempRating += 10
				v.Rating += 5
			}
			return nil
		},
	}
	arguments.VKApp = vkAppMock
	vs, _ := peer.NewValidatorStatisticsProcessor(arguments)

	// Create test validator info
	validatorInfo := []*state.ValidatorInfo{
		{
			PublicKey:  []byte("pk1"),
			TempRating: 50,
			Rating:     75,
		},
		{
			PublicKey:  []byte("pk2"),
			TempRating: 60,
			Rating:     80,
		},
	}

	err := vs.ProcessRatingsEndOfEpoch(validatorInfo, 1)

	require.Nil(t, err)
	assert.Equal(t, uint32(60), validatorInfo[0].TempRating)
	assert.Equal(t, uint32(80), validatorInfo[0].Rating)
	assert.Equal(t, uint32(70), validatorInfo[1].TempRating)
	assert.Equal(t, uint32(85), validatorInfo[1].Rating)
}

func TestValidatorStatistics_ProcessRatingsEndOfEpochWithZeroEpoch(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	vkAppMock := &mock.ValidatorsKAppStub{
		ProcessRatingsEndOfEpochCalled: func(validatorInfos []*state.ValidatorInfo) error {
			return errors.New("this should not be called")
		},
	}
	arguments.VKApp = vkAppMock
	vs, _ := peer.NewValidatorStatisticsProcessor(arguments)

	validatorInfo := []*state.ValidatorInfo{
		{
			PublicKey:  []byte("pk1"),
			TempRating: 50,
			Rating:     75,
		},
	}

	err := vs.ProcessRatingsEndOfEpoch(validatorInfo, 0)

	require.Nil(t, err)
	assert.Equal(t, uint32(50), validatorInfo[0].TempRating)
	assert.Equal(t, uint32(75), validatorInfo[0].Rating)
}

func TestValidatorStatistics_ProcessRatingsEndOfEpochWithNilValidatorInfos(t *testing.T) {
	t.Parallel()

	arguments := createMockArguments()
	vs, _ := peer.NewValidatorStatisticsProcessor(arguments)

	err := vs.ProcessRatingsEndOfEpoch(nil, 1)

	require.Equal(t, process.ErrNilValidatorInfos, err)
}

func TestValidatorStatistics_TruncatedPeerTrieWalkIsReported(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("trie iteration failed")

	marshalizer := &mock.ProtoMarshalizerMock{}
	peerAccount := state.NewEmptyPeerAccount()
	peerAccount.SetOwnerAddress([]byte("owner"))
	peerBytes, errMarshal := marshalizer.Marshal(peerAccount)
	require.Nil(t, errMarshal)

	newProcessor := func() process.ValidatorStatisticsProcessor {
		arguments := createMockArguments()
		arguments.ForkController = mock.NewForkControllerStub()
		peerAdapter := getAccountsMock()
		peerAdapter.GetAllLeavesCalled = func(_ []byte) (*data.TrieIteratorChannels, error) {
			return data.NewFailedTrieIteratorChannels(
				expectedErr,
				keyValStorage.NewKeyValStorage([]byte("pubkey"), peerBytes),
			), nil
		}
		arguments.PeerAdapter = peerAdapter

		vs, err := peer.NewValidatorStatisticsProcessor(arguments)
		require.Nil(t, err)

		return vs
	}

	t.Run("GetValidatorInfoForRootHash", func(t *testing.T) {
		t.Parallel()
		vInfos, err := newProcessor().GetValidatorInfoForRootHash([]byte("rootHash"))
		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, vInfos)
	})

	t.Run("ListPeerAccounts", func(t *testing.T) {
		t.Parallel()
		peers, err := newProcessor().ListPeerAccounts([]byte("rootHash"))
		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, peers)
	})

	t.Run("GetValidatorAccountRootHash", func(t *testing.T) {
		t.Parallel()
		infos, err := newProcessor().GetValidatorAccountRootHash([]byte("rootHash"))
		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, infos)
	})

	t.Run("GetPeersAccountsPubkeysFromRootHash", func(t *testing.T) {
		t.Parallel()
		pubkeys, err := newProcessor().GetPeersAccountsPubkeysFromRootHash([]byte("rootHash"))
		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, pubkeys)
	})
}
