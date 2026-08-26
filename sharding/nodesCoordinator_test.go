package sharding

import (
	"errors"
	"fmt"
	"testing"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/hashing/sha256"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/sharding/mock"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createDummyNodesList(nbNodes uint32, suffix string) []Validator {
	list := make([]Validator, 0)
	hasher := sha256.Sha256{}

	for j := uint32(0); j < nbNodes; j++ {
		pk := hasher.Compute(fmt.Sprintf("pk%s_%d", suffix, j))
		list = append(list, mock.NewValidatorMock(pk, pk, 1, DefaultSelectionChances))
	}

	return list
}

func isStringSubgroup(a []string, b []string) bool {
	var found bool
	for _, va := range a {
		found = false
		for _, vb := range b {
			if va == vb {
				found = true
				break
			}
		}
		if !found {
			return found
		}
	}

	return found
}

func createArguments() ArgNodesCoordinator {
	electedList := createDummyNodesList(4, "elected")
	eligibleList := createDummyNodesList(1, "eligible")

	epochStartSubscriber := &mock.EpochStartNotifierStub{}
	bootStorer := mock.NewStorerMock()

	shufflerArgs := &NodesShufflerArgs{
		Nodes:                10,
		MaxNodesEnableConfig: nil,
	}
	nodeShuffler, _ := NewHashValidatorsShuffler(shufflerArgs)

	arguments := ArgNodesCoordinator{
		ConsensusGroupSize:  4,
		Marshalizer:         &mock.MarshalizerMock{},
		Hasher:              &mock.HasherMock{},
		Shuffler:            nodeShuffler,
		EpochStartNotifier:  epochStartSubscriber,
		BootStorer:          bootStorer,
		ElectedNodes:        electedList,
		EligibleNodes:       eligibleList,
		SelfPublicKey:       []byte("test"),
		ConsensusGroupCache: &mock.NodesCoordinatorCacheMock{},
	}

	return arguments
}

func validatorsPubKeys(validators []Validator) []string {
	pKeys := make([]string, len(validators))
	for _, v := range validators {
		pKeys = append(pKeys, string(v.PubKey()))
	}

	return pKeys
}

//------- NewNodesCoordinator

func TestNewNodesCoordinator_NilHasherShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createArguments()
	arguments.Hasher = nil
	ihgs, err := NewNodesCoordinator(arguments)

	require.Equal(t, ErrNilHasher, err)
	require.Nil(t, ihgs)
}

func TestNewNodesCoordinator_NilSelfPublicKeyShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createArguments()
	arguments.SelfPublicKey = nil
	ihgs, err := NewNodesCoordinator(arguments)

	require.Equal(t, ErrNilPubKey, err)
	require.Nil(t, ihgs)
}

func TestNewNodesCoordinator_NilShufflerShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createArguments()
	arguments.Shuffler = nil
	ihgs, err := NewNodesCoordinator(arguments)

	require.Equal(t, ErrNilShuffler, err)
	require.Nil(t, ihgs)
}

func TestNewIndexHashedNodesCoordinator_NilBootStorerShouldErr(t *testing.T) {
	arguments := createArguments()
	arguments.BootStorer = nil
	ihgs, err := NewNodesCoordinator(arguments)

	require.Equal(t, ErrNilBootStorer, err)
	require.Nil(t, ihgs)
}

func TestNewNodesCoordinator_NilCacherShouldErr(t *testing.T) {
	arguments := createArguments()
	arguments.ConsensusGroupCache = nil
	ihgs, err := NewNodesCoordinator(arguments)

	require.Equal(t, ErrNilCacher, err)
	require.Nil(t, ihgs)
}

func TestNewNodesCoordinator_OkValsShouldWork(t *testing.T) {
	t.Parallel()

	arguments := createArguments()
	ihgs, err := NewNodesCoordinator(arguments)

	require.Nil(t, err)
	require.NotNil(t, ihgs)
}

//------- createSelector

func TestNodesCoordinator_SetEmptyElectedMapShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createArguments()
	arguments.ElectedNodes = make([]Validator, 0)
	ihgs, err := NewNodesCoordinator(arguments)

	require.Equal(t, ErrSmallElectedListSize, err)
	require.Nil(t, ihgs)
}

//------- ComputeValidatorsGroup

func TestNodesCoordinator_NewNodesCoordinatorGroup0SizeShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createArguments()
	arguments.ConsensusGroupSize = 0
	ihgs, err := NewNodesCoordinator(arguments)

	require.Equal(t, ErrInvalidConsensusGroupSize, err)
	require.Nil(t, ihgs)
}

func TestNodesCoordinator_NewNodesCoordinatorTooFewNodesShouldErr(t *testing.T) {
	t.Parallel()

	electedList := createDummyNodesList(4, "elected")
	eligibleList := createDummyNodesList(1, "eligible")
	shufflerArgs := &NodesShufflerArgs{
		Nodes:                10,
		MaxNodesEnableConfig: nil,
	}
	nodeShuffler, err := NewHashValidatorsShuffler(shufflerArgs)
	require.Nil(t, err)

	epochStartSubscriber := &mock.EpochStartNotifierStub{}
	bootStorer := mock.NewStorerMock()

	arguments := ArgNodesCoordinator{
		ConsensusGroupSize:  10,
		Marshalizer:         &mock.MarshalizerMock{},
		Hasher:              &mock.HasherMock{},
		Shuffler:            nodeShuffler,
		EpochStartNotifier:  epochStartSubscriber,
		BootStorer:          bootStorer,
		ElectedNodes:        electedList,
		EligibleNodes:       eligibleList,
		SelfPublicKey:       []byte("test"),
		ConsensusGroupCache: &mock.NodesCoordinatorCacheMock{},
	}
	ihgs, err := NewNodesCoordinator(arguments)

	require.Equal(t, ErrSmallElectedListSize, err)
	require.Nil(t, ihgs)
}

func TestNodesCoordinator_ComputeValidatorsGroupNilRandomnessShouldErr(t *testing.T) {
	t.Parallel()

	arguments := createArguments()
	ihgs, _ := NewNodesCoordinator(arguments)
	list2, err := ihgs.ComputeConsensusGroup(nil, 0, 0)

	require.Equal(t, ErrNilRandomness, err)
	require.Nil(t, list2)
}

//------- Functionality tests

func TestNodesCoordinator_ComputeValidatorsGroup1ValidatorShouldReturnSame(t *testing.T) {
	t.Parallel()

	electedList := createDummyNodesList(2, "elected")
	eligibleList := createDummyNodesList(0, "eligible")
	shufflerArgs := &NodesShufflerArgs{
		Nodes:                10,
		MaxNodesEnableConfig: nil,
	}
	nodeShuffler, err := NewHashValidatorsShuffler(shufflerArgs)
	require.Nil(t, err)

	epochStartSubscriber := &mock.EpochStartNotifierStub{}
	bootStorer := mock.NewStorerMock()

	arguments := ArgNodesCoordinator{
		ConsensusGroupSize:  1,
		Marshalizer:         &mock.MarshalizerMock{},
		Hasher:              &mock.HasherMock{},
		Shuffler:            nodeShuffler,
		EpochStartNotifier:  epochStartSubscriber,
		BootStorer:          bootStorer,
		ElectedNodes:        electedList,
		EligibleNodes:       eligibleList,
		SelfPublicKey:       []byte("test"),
		ConsensusGroupCache: &mock.NodesCoordinatorCacheMock{},
	}

	ihgs, _ := NewNodesCoordinator(arguments)
	finalList, err := ihgs.ComputeConsensusGroup([]byte("randomness"), 0, 0)

	require.Equal(t, electedList[1], finalList[0])
	require.Nil(t, err)
}

func TestNodesCoordinator_ComputeValidatorsGroup25of400(t *testing.T) {
	consensusGroupSize := 25
	nodes := uint32(400)
	electedList := createDummyNodesList(nodes, "elected")
	eligibleList := createDummyNodesList(0, "eligible")
	shufflerArgs := &NodesShufflerArgs{
		Nodes:                nodes,
		MaxNodesEnableConfig: nil,
	}
	nodeShuffler, err := NewHashValidatorsShuffler(shufflerArgs)
	require.Nil(t, err)

	epochStartSubscriber := &mock.EpochStartNotifierStub{}
	bootStorer := mock.NewStorerMock()

	arguments := ArgNodesCoordinator{
		ConsensusGroupSize:  consensusGroupSize,
		Marshalizer:         &mock.MarshalizerMock{},
		Hasher:              &mock.HasherMock{},
		Shuffler:            nodeShuffler,
		EpochStartNotifier:  epochStartSubscriber,
		BootStorer:          bootStorer,
		ElectedNodes:        electedList,
		EligibleNodes:       eligibleList,
		SelfPublicKey:       []byte("test"),
		ConsensusGroupCache: &mock.NodesCoordinatorCacheMock{},
	}

	ihgs, err := NewNodesCoordinator(arguments)
	require.Nil(t, err)

	finalList, _ := ihgs.ComputeConsensusGroup([]byte("randomness"), 0, 0)
	require.Equal(t, consensusGroupSize, len(finalList))
}

func TestNodesCoordinator_ComputeValidatorsGroupBiggerThanElected(t *testing.T) {
	consensusGroupSize := 25
	nodes := uint32(400)
	electedList := createDummyNodesList(nodes, "elected")
	eligibleList := createDummyNodesList(0, "eligible")
	shufflerArgs := &NodesShufflerArgs{
		Nodes:                nodes,
		MaxNodesEnableConfig: nil,
	}
	nodeShuffler, err := NewHashValidatorsShuffler(shufflerArgs)
	require.Nil(t, err)

	epochStartSubscriber := &mock.EpochStartNotifierStub{}
	bootStorer := mock.NewStorerMock()

	arguments := ArgNodesCoordinator{
		ConsensusGroupSize:  consensusGroupSize,
		Marshalizer:         &mock.MarshalizerMock{},
		Hasher:              &mock.HasherMock{},
		Shuffler:            nodeShuffler,
		EpochStartNotifier:  epochStartSubscriber,
		BootStorer:          bootStorer,
		ElectedNodes:        electedList,
		EligibleNodes:       eligibleList,
		SelfPublicKey:       []byte("test"),
		ConsensusGroupCache: &mock.NodesCoordinatorCacheMock{},
	}
	// create coordinator using valid configurations
	ihgs, err := NewNodesCoordinator(arguments)
	require.Nil(t, err)

	// force unexpected scenario of empty elected list
	currentConfig := ihgs.nodesConfig[0]
	currentConfig.electedList = createDummyNodesList(uint32(0), "elected")
	ihgs.nodesConfig[0] = currentConfig

	// try to compute
	_, err = ihgs.ComputeConsensusGroup([]byte("randomness"), 0, 0)
	require.Equal(t, ErrSmallElectedListSize, err)
}

func TestNodesCoordinator_GetValidatorWithPublicKeyShouldReturnErrNilPubKey(t *testing.T) {
	t.Parallel()

	arguments := createArguments()
	ihgs, _ := NewNodesCoordinator(arguments)

	_, err := ihgs.GetValidatorWithPublicKey(nil)
	require.Equal(t, ErrNilPubKey, err)
}

func TestNodesCoordinator_GetValidatorWithPublicKeyShouldReturnErrValidatorNotFound(t *testing.T) {
	t.Parallel()

	arguments := createArguments()
	ihgs, _ := NewNodesCoordinator(arguments)

	_, err := ihgs.GetValidatorWithPublicKey([]byte("pk1"))
	require.Equal(t, ErrValidatorNotFound, err)
}

func TestIndexHashedNodesCoordinator_GetValidatorWithPublicKeyShouldWork(t *testing.T) {
	t.Parallel()

	listNode := []Validator{
		mock.NewValidatorMock([]byte("oa0_node"), []byte("pk0_node"), 1, DefaultSelectionChances),
		mock.NewValidatorMock([]byte("oa1_node"), []byte("pk1_node"), 1, DefaultSelectionChances),
		mock.NewValidatorMock([]byte("oa2_node"), []byte("pk2_node"), 1, DefaultSelectionChances),
	}

	electedList := listNode
	eligibleList := createDummyNodesList(0, "eligible")
	shufflerArgs := &NodesShufflerArgs{
		Nodes:                10,
		MaxNodesEnableConfig: nil,
	}
	nodeShuffler, err := NewHashValidatorsShuffler(shufflerArgs)
	require.Nil(t, err)

	epochStartSubscriber := &mock.EpochStartNotifierStub{}
	bootStorer := mock.NewStorerMock()

	arguments := ArgNodesCoordinator{
		ConsensusGroupSize:  1,
		Marshalizer:         &mock.MarshalizerMock{},
		Hasher:              &mock.HasherMock{},
		Shuffler:            nodeShuffler,
		EpochStartNotifier:  epochStartSubscriber,
		BootStorer:          bootStorer,
		ElectedNodes:        electedList,
		EligibleNodes:       eligibleList,
		SelfPublicKey:       []byte("test"),
		ConsensusGroupCache: &mock.NodesCoordinatorCacheMock{},
	}
	ihgs, _ := NewNodesCoordinator(arguments)

	v, err := ihgs.GetValidatorWithPublicKey([]byte("pk0_node"))
	require.Nil(t, err)
	require.Equal(t, []byte("pk0_node"), v.PubKey())

	v, err = ihgs.GetValidatorWithPublicKey([]byte("pk1_node"))
	require.Nil(t, err)
	require.Equal(t, []byte("pk1_node"), v.PubKey())

	v, err = ihgs.GetValidatorWithPublicKey([]byte("pk2_node"))
	require.Nil(t, err)
	require.Equal(t, []byte("pk2_node"), v.PubKey())
}

func TestNewNodesCoordinator_EpochStart(t *testing.T) {
	t.Parallel()

	arguments := createArguments()

	ihgs, err := NewNodesCoordinator(arguments)
	require.Nil(t, err)
	epoch := uint32(1)

	block := &block.Block{
		Header: &block.BlockHeader{
			Nonce:        10,
			IsEpochStart: true,
		},
	}

	ihgs.nodesConfig[epoch] = ihgs.nodesConfig[0]

	ihgs.EpochStartPrepare(block)
	ihgs.EpochStartAction(block)

	validators, err := ihgs.GetAllElectedValidatorsKeys(epoch, false)
	require.Nil(t, err)
	require.NotNil(t, validators)
}

func TestNodesCoordinator_EpochStart_ElectedSortedAscendingByIndex(t *testing.T) {
	t.Parallel()

	pk1 := []byte{1}
	pk2 := []byte{2}

	electedList := []Validator{
		mock.NewValidatorMock(pk1, pk1, 1, 1),
		mock.NewValidatorMock(pk2, pk2, 1, 1),
	}
	eligibleList := createDummyNodesList(0, "eligible")

	shufflerArgs := &NodesShufflerArgs{
		Nodes:                2,
		MaxNodesEnableConfig: nil,
	}
	nodeShuffler, err := NewHashValidatorsShuffler(shufflerArgs)
	require.Nil(t, err)

	epochStartSubscriber := &mock.EpochStartNotifierStub{}
	bootStorer := mock.NewStorerMock()

	arguments := ArgNodesCoordinator{
		ConsensusGroupSize:  1,
		Marshalizer:         &mock.MarshalizerMock{},
		Hasher:              &mock.HasherMock{},
		Shuffler:            nodeShuffler,
		EpochStartNotifier:  epochStartSubscriber,
		BootStorer:          bootStorer,
		ElectedNodes:        electedList,
		EligibleNodes:       eligibleList,
		SelfPublicKey:       []byte("test"),
		ConsensusGroupCache: &mock.NodesCoordinatorCacheMock{},
	}

	ihgs, err := NewNodesCoordinator(arguments)
	require.Nil(t, err)
	epoch := uint32(1)

	block := &block.Block{
		Header: &block.BlockHeader{
			PrevRandSeed: []byte("rand seed"),
			IsEpochStart: true,
			Epoch:        epoch,
		},
	}

	ihgs.nodesConfig[epoch] = ihgs.nodesConfig[0]

	ihgs.EpochStartPrepare(block)

	newNodesConfig := ihgs.nodesConfig[1]

	firstEligible := newNodesConfig.electedList[0].PubKey()
	secondEligible := newNodesConfig.electedList[1].PubKey()

	require.Equal(t, pk1, firstEligible)
	require.Equal(t, pk2, secondEligible)
}

func TestNodesCoordinator_GetConsensusValidatorsPublicKeysNotExistingEpoch(t *testing.T) {
	t.Parallel()

	args := createArguments()
	ihgs, err := NewNodesCoordinator(args)
	require.Nil(t, err)

	ihgs.nodesConfig = make(map[uint32]*epochNodesConfig)

	var pKeys []string
	randomness := []byte("randomness")
	pKeys, err = ihgs.GetConsensusValidatorsPublicKeys(randomness, 0, 0)
	require.True(t, errors.Is(err, ErrEpochNodesConfigDoesNotExist))
	require.Nil(t, pKeys)
}

func TestNodesCoordinator_GetConsensusValidatorsPublicKeysExistingEpoch(t *testing.T) {
	t.Parallel()

	args := createArguments()
	ihgs, err := NewNodesCoordinator(args)
	require.Nil(t, err)

	validatorsPubKeys := validatorsPubKeys(args.ElectedNodes)

	var pKeys []string
	randomness := []byte("randomness")
	pKeys, err = ihgs.GetConsensusValidatorsPublicKeys(randomness, 0, 0)
	require.Nil(t, err)
	require.True(t, len(pKeys) > 0)
	require.True(t, isStringSubgroup(pKeys, validatorsPubKeys))
}

func TestNodesCoordinator_ConsensusGroupSize(t *testing.T) {
	t.Parallel()

	arguments := createArguments()
	ihgs, err := NewNodesCoordinator(arguments)
	require.Nil(t, err)

	consensusSizeMeta := ihgs.ConsensusGroupSize()

	require.Equal(t, arguments.ConsensusGroupSize, consensusSizeMeta)
}

func TestNodesCoordinator_GetOwnPublicKey(t *testing.T) {
	t.Parallel()

	arguments := createArguments()
	ihgs, err := NewNodesCoordinator(arguments)
	require.Nil(t, err)

	ownPubKey := ihgs.GetOwnPublicKey()
	require.Equal(t, arguments.SelfPublicKey, ownPubKey)
}

func TestNodesCoordinator_computeNodesConfigFromList_NoValidators(t *testing.T) {
	t.Parallel()

	arguments := createArguments()
	pk := []byte("pk")
	arguments.SelfPublicKey = pk
	ihgs, _ := NewNodesCoordinator(arguments)

	validatorInfos := make([]*block.EValidatorInfo, 0)
	newNodesConfig, err := ihgs.computeNodesConfigFromList(validatorInfos)

	assert.Nil(t, newNodesConfig)
	assert.True(t, errors.Is(err, ErrListSizeZero))

	newNodesConfig, err = ihgs.computeNodesConfigFromList(nil)

	assert.Nil(t, newNodesConfig)
	assert.True(t, errors.Is(err, ErrListSizeZero))
}

func TestNodesCoordinator_allValidatorsInfo_EpochNodesConfigDoesNotExist(t *testing.T) {
	t.Parallel()

	electedList := createDummyNodesList(6, "elected")
	eligibleList := createDummyNodesList(0, "eligible")

	shufflerArgs := &NodesShufflerArgs{
		Nodes:                5,
		MaxNodesEnableConfig: nil,
	}
	nodeShuffler, err := NewHashValidatorsShuffler(shufflerArgs)
	require.Nil(t, err)

	epochStartSubscriber := &mock.EpochStartNotifierStub{}
	bootStorer := mock.NewStorerMock()

	arguments := ArgNodesCoordinator{
		ConsensusGroupSize:  5,
		Marshalizer:         &mock.MarshalizerMock{},
		Hasher:              &mock.HasherMock{},
		Shuffler:            nodeShuffler,
		EpochStartNotifier:  epochStartSubscriber,
		BootStorer:          bootStorer,
		ElectedNodes:        electedList,
		EligibleNodes:       eligibleList,
		SelfPublicKey:       []byte("test"),
		ConsensusGroupCache: &mock.NodesCoordinatorCacheMock{},
	}

	ihgs, err := NewNodesCoordinator(arguments)
	require.Nil(t, err)

	ihgs.currentEpoch.Store(1)
}

func TestNodesCoordinator_allValidatorsInfo_KeepLeavingIfNotEnoughValidators(t *testing.T) {
	t.Parallel()

	electedList := createDummyNodesList(6, "elected")
	eligibleList := createDummyNodesList(0, "eligible")

	shufflerArgs := &NodesShufflerArgs{
		Nodes:                5,
		MaxNodesEnableConfig: nil,
	}
	nodeShuffler, err := NewHashValidatorsShuffler(shufflerArgs)
	require.Nil(t, err)

	epochStartSubscriber := &mock.EpochStartNotifierStub{}
	bootStorer := mock.NewStorerMock()

	arguments := ArgNodesCoordinator{
		ConsensusGroupSize:  5,
		Marshalizer:         &mock.MarshalizerMock{},
		Hasher:              &mock.HasherMock{},
		Shuffler:            nodeShuffler,
		EpochStartNotifier:  epochStartSubscriber,
		BootStorer:          bootStorer,
		ElectedNodes:        electedList,
		EligibleNodes:       eligibleList,
		SelfPublicKey:       []byte("test"),
		ConsensusGroupCache: &mock.NodesCoordinatorCacheMock{},
	}

	ihgs, err := NewNodesCoordinator(arguments)
	require.Nil(t, err)

	ihgs.nodesConfig[0].leavingList = append(ihgs.nodesConfig[0].leavingList, electedList[0])
}

func TestNodesCoordinator_computeNodesConfigFromList_NilPk(t *testing.T) {
	t.Parallel()

	arguments := createArguments()
	pk := []byte("pk")
	arguments.SelfPublicKey = pk
	ihgs, _ := NewNodesCoordinator(arguments)

	validatorInfos :=
		[]*block.EValidatorInfo{
			{
				OwnerAddress: pk,
				PublicKey:    pk,
				List:         "test1",
				Index:        0,
				TempRating:   0,
			},
			{
				OwnerAddress: nil,
				PublicKey:    nil,
				List:         "test",
				Index:        0,
				TempRating:   0,
			},
		}

	newNodesConfig, err := ihgs.computeNodesConfigFromList(validatorInfos)

	assert.Nil(t, newNodesConfig)
	assert.NotNil(t, err)
	assert.Equal(t, ErrNilPubKey, err)
}

func TestNodesCoordinator_computeNodesConfigFromList_Validators(t *testing.T) {
	t.Parallel()

	arguments := createArguments()
	pk := []byte("pk")
	arguments.SelfPublicKey = pk
	ihgs, _ := NewNodesCoordinator(arguments)

	nodeElected0 := &block.EValidatorInfo{
		OwnerAddress: []byte("pk0"),
		PublicKey:    []byte("pk0"),
		List:         string(core.ElectedList),
		Index:        1,
		TempRating:   2,
	}
	nodeElected1 := &block.EValidatorInfo{
		OwnerAddress: []byte("pk1"),
		PublicKey:    []byte("pk1"),
		List:         string(core.ElectedList),
		Index:        2,
		TempRating:   2,
	}
	nodeEligible0 := &block.EValidatorInfo{
		OwnerAddress: []byte("pk2"),
		PublicKey:    []byte("pk2"),
		List:         string(core.EligibleList),
		Index:        1,
		TempRating:   2,
	}
	nodeEligible1 := &block.EValidatorInfo{
		OwnerAddress: []byte("pk3"),
		PublicKey:    []byte("pk3"),
		List:         string(core.EligibleList),
		Index:        2,
		TempRating:   2,
	}
	nodeWaiting0 := &block.EValidatorInfo{
		OwnerAddress: []byte("pk4"),
		PublicKey:    []byte("pk4"),
		List:         string(core.WaitingList),
		Index:        1,
		TempRating:   2,
	}
	nodeWaiting1 := &block.EValidatorInfo{
		OwnerAddress: []byte("pk5"),
		PublicKey:    []byte("pk5"),
		List:         string(core.WaitingList),
		Index:        2,
		TempRating:   2,
	}
	nodeLeaving0 := &block.EValidatorInfo{
		OwnerAddress: []byte("pk6"),
		PublicKey:    []byte("pk6"),
		List:         string(core.LeavingList),
		Index:        1,
		TempRating:   2,
	}

	validatorInfos :=
		[]*block.EValidatorInfo{
			nodeElected0,
			nodeElected1,
			nodeEligible0,
			nodeEligible1,
			nodeWaiting0,
			nodeWaiting1,
			nodeLeaving0,
		}

	newNodesConfig, err := ihgs.computeNodesConfigFromList(validatorInfos)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(newNodesConfig.electedList))
	assert.Equal(t, nodeElected0.PublicKey, newNodesConfig.electedList[0].PubKey())

	assert.Equal(t, 2, len(newNodesConfig.eligibleList))
	assert.Equal(t, nodeEligible0.PublicKey, newNodesConfig.eligibleList[0].PubKey())

	assert.Equal(t, 2, len(newNodesConfig.electedList))
	assert.Equal(t, nodeWaiting0.PublicKey, newNodesConfig.waitingList[0].PubKey())

	assert.Equal(t, 0, len(newNodesConfig.leavingList))
}

func TestNodesCoordinator_IsInterfaceNil(t *testing.T) {
	t.Parallel()

	var ihgs NodesCoordinator
	require.True(t, check.IfNil(ihgs))

	var ihgs2 *indexHashedNodesCoordinator
	require.True(t, check.IfNil(ihgs2))

	arguments := createArguments()
	ihgs3, err := NewNodesCoordinator(arguments)
	require.Nil(t, err)
	require.False(t, check.IfNil(ihgs3))
}

func TestNodesCoordinator_computePublicKeyToValidatorMap(t *testing.T) {
	t.Parallel()

	makeValidator := func(pubKey string, index uint32) Validator {
		pk := []byte(pubKey)
		return mock.NewValidatorMock(pk, pk, DefaultSelectionChances, index)
	}

	ihgs := &indexHashedNodesCoordinator{}

	t.Run("empty config yields an empty map", func(t *testing.T) {
		t.Parallel()

		require.Len(t, ihgs.computePublicKeyToValidatorMap(map[uint32]*epochNodesConfig{}), 0)
	})

	t.Run("epoch without validators yields an empty map", func(t *testing.T) {
		t.Parallel()

		require.Len(t, ihgs.computePublicKeyToValidatorMap(map[uint32]*epochNodesConfig{7: {}}), 0)
	})

	t.Run("elected and eligible are both present", func(t *testing.T) {
		t.Parallel()

		nodesConfig := map[uint32]*epochNodesConfig{
			3: {
				electedList:  []Validator{makeValidator("elected_pk", 1)},
				eligibleList: []Validator{makeValidator("eligible_pk", 2)},
			},
		}

		publicKeyToValidatorMap := ihgs.computePublicKeyToValidatorMap(nodesConfig)

		require.Len(t, publicKeyToValidatorMap, 2)
		require.Contains(t, publicKeyToValidatorMap, "elected_pk")
		require.Contains(t, publicKeyToValidatorMap, "eligible_pk")
		require.Equal(t, uint32(1), publicKeyToValidatorMap["elected_pk"].Index())
		require.Equal(t, uint32(2), publicKeyToValidatorMap["eligible_pk"].Index())
	})

	t.Run("within one epoch eligible takes precedence over elected", func(t *testing.T) {
		t.Parallel()

		sharedPK := "pk_present_in_both_lists"
		nodesConfig := map[uint32]*epochNodesConfig{
			3: {
				electedList:  []Validator{makeValidator(sharedPK, 1)},
				eligibleList: []Validator{makeValidator(sharedPK, 2)},
			},
		}

		publicKeyToValidatorMap := ihgs.computePublicKeyToValidatorMap(nodesConfig)

		require.Len(t, publicKeyToValidatorMap, 1)
		require.Contains(t, publicKeyToValidatorMap, sharedPK)
		require.Equal(t, uint32(2), publicKeyToValidatorMap[sharedPK].Index())
	})

	t.Run("the highest epoch wins and earlier epochs still contribute", func(t *testing.T) {
		t.Parallel()

		sharedPK := "pk_present_in_every_epoch"
		earlyOnlyPK := "pk_present_only_in_the_earliest_epoch"
		// non-contiguous epochs, so ordering by insertion or lexicographically would pick another one
		nodesConfig := map[uint32]*epochNodesConfig{
			2:  {electedList: []Validator{makeValidator(sharedPK, 2), makeValidator(earlyOnlyPK, 200)}},
			9:  {electedList: []Validator{makeValidator(sharedPK, 9)}},
			11: {electedList: []Validator{makeValidator(sharedPK, 11)}},
			40: {electedList: []Validator{makeValidator(sharedPK, 40)}},
		}

		// the range start offset is randomised on every execution, so repeat to make an
		// order-dependent regression fail reliably instead of flaking
		for i := 0; i < 100; i++ {
			publicKeyToValidatorMap := ihgs.computePublicKeyToValidatorMap(nodesConfig)

			require.Len(t, publicKeyToValidatorMap, 2)
			require.Contains(t, publicKeyToValidatorMap, sharedPK)
			require.Equal(t, uint32(40), publicKeyToValidatorMap[sharedPK].Index(), "iteration %d", i)
			// a validator that only appears in an earlier epoch must survive the merge,
			// so an implementation that reads the highest epoch alone is rejected
			require.Contains(t, publicKeyToValidatorMap, earlyOnlyPK)
			require.Equal(t, uint32(200), publicKeyToValidatorMap[earlyOnlyPK].Index(), "iteration %d", i)
		}
	})

	t.Run("the epoch order outranks the list order", func(t *testing.T) {
		t.Parallel()

		sharedPK := "pk_eligible_early_then_elected_later"
		nodesConfig := map[uint32]*epochNodesConfig{
			5: {eligibleList: []Validator{makeValidator(sharedPK, 5)}},
			9: {electedList: []Validator{makeValidator(sharedPK, 9)}},
		}

		// an implementation that wrote every epoch's elected list first and every
		// epoch's eligible list second would return the stale epoch 5 record here,
		// while passing every other assertion in this test
		for i := 0; i < 100; i++ {
			publicKeyToValidatorMap := ihgs.computePublicKeyToValidatorMap(nodesConfig)

			require.Len(t, publicKeyToValidatorMap, 1)
			require.Contains(t, publicKeyToValidatorMap, sharedPK)
			require.Equal(t, uint32(9), publicKeyToValidatorMap[sharedPK].Index(), "iteration %d", i)
		}
	})
}

func TestNodesCoordinator_LoadStateRefillsPublicKeyToValidatorMap(t *testing.T) {
	t.Parallel()

	currentElected := createDummyNodesList(4, "currentElected")
	currentEligible := createDummyNodesList(1, "currentEligible")
	bootStorer := mock.NewStorerMock()

	argsBefore := createArguments()
	argsBefore.BootStorer = bootStorer
	argsBefore.ElectedNodes = currentElected
	argsBefore.EligibleNodes = currentEligible
	coordinatorBefore, err := NewNodesCoordinator(argsBefore)
	require.Nil(t, err)

	savedStateKey := []byte("rotated-saved-state-key")
	err = coordinatorBefore.saveState(savedStateKey)
	require.Nil(t, err)

	genesisElected := createDummyNodesList(4, "genesisElected")
	genesisEligible := createDummyNodesList(1, "genesisEligible")

	argsAfter := createArguments()
	argsAfter.BootStorer = bootStorer
	argsAfter.ElectedNodes = genesisElected
	argsAfter.EligibleNodes = genesisEligible
	coordinatorAfter, err := NewNodesCoordinator(argsAfter)
	require.Nil(t, err)

	_, err = coordinatorAfter.GetValidatorWithPublicKey(currentElected[0].PubKey())
	require.Equal(t, ErrValidatorNotFound, err)

	err = coordinatorAfter.LoadState(savedStateKey)
	require.Nil(t, err)

	validator, err := coordinatorAfter.GetValidatorWithPublicKey(currentElected[0].PubKey())
	require.Nil(t, err)
	require.Equal(t, currentElected[0].PubKey(), validator.PubKey())

	eligibleValidator, err := coordinatorAfter.GetValidatorWithPublicKey(currentEligible[0].PubKey())
	require.Nil(t, err)
	require.Equal(t, currentEligible[0].PubKey(), eligibleValidator.PubKey())

	_, err = coordinatorAfter.GetValidatorWithPublicKey(genesisElected[0].PubKey())
	require.Equal(t, ErrValidatorNotFound, err)
}

func TestNodesCoordinator_LoadStateMultiEpochPrecedence(t *testing.T) {
	t.Parallel()

	sharedPK := []byte("shared-pk-appears-in-all-epochs")
	bootStorer := mock.NewStorerMock()

	baseEpoch := uint32(4)
	numEpochs := uint32(5)
	highestEpoch := baseEpoch + numEpochs - 1

	makeElected := func(epoch, index uint32) []Validator {
		return []Validator{
			mock.NewValidatorMock(sharedPK, sharedPK, DefaultSelectionChances, index),
			mock.NewValidatorMock([]byte(fmt.Sprintf("pk%d_1", epoch)), []byte(fmt.Sprintf("pk%d_1", epoch)), DefaultSelectionChances, 1),
			mock.NewValidatorMock([]byte(fmt.Sprintf("pk%d_2", epoch)), []byte(fmt.Sprintf("pk%d_2", epoch)), DefaultSelectionChances, 2),
			mock.NewValidatorMock([]byte(fmt.Sprintf("pk%d_3", epoch)), []byte(fmt.Sprintf("pk%d_3", epoch)), DefaultSelectionChances, 3),
		}
	}

	args := createArguments()
	args.BootStorer = bootStorer
	args.ElectedNodes = makeElected(baseEpoch, baseEpoch*10)
	args.EligibleNodes = createDummyNodesList(0, "eligible")
	args.Epoch = baseEpoch
	args.StartEpoch = baseEpoch
	coordinator, err := NewNodesCoordinator(args)
	require.Nil(t, err)

	for ep := baseEpoch + 1; ep <= highestEpoch; ep++ {
		err = coordinator.SetNodes(makeElected(ep, ep*10), []Validator{}, []Validator{}, ep)
		require.Nil(t, err)
	}

	savedKey := []byte("multi-epoch-key")
	err = coordinator.saveState(savedKey)
	require.Nil(t, err)

	for attempt := 0; attempt < 20; attempt++ {
		argsLoad := createArguments()
		argsLoad.BootStorer = bootStorer
		argsLoad.ElectedNodes = createDummyNodesList(4, "genesis")
		argsLoad.EligibleNodes = createDummyNodesList(0, "eligible")
		coordinatorLoad, err := NewNodesCoordinator(argsLoad)
		require.Nil(t, err)

		err = coordinatorLoad.LoadState(savedKey)
		require.Nil(t, err)

		v, err := coordinatorLoad.GetValidatorWithPublicKey(sharedPK)
		require.Nil(t, err)
		require.Equal(t, highestEpoch*10, v.Index(), "attempt %d", attempt)
	}
}

func TestNodesCoordinator_ConstructorDoesNotOverwriteSavedStateOnRestart(t *testing.T) {
	t.Parallel()

	realElected := createDummyNodesList(4, "realElected")
	realEligible := createDummyNodesList(1, "realEligible")
	bootStorer := mock.NewStorerMock()

	firstArgs := createArguments()
	firstArgs.BootStorer = bootStorer
	firstArgs.ElectedNodes = realElected
	firstArgs.EligibleNodes = realEligible
	firstArgs.Epoch = 5
	firstArgs.StartEpoch = 5
	firstCoordinator, err := NewNodesCoordinator(firstArgs)
	require.Nil(t, err)

	err = firstCoordinator.saveState(firstCoordinator.savedStateKey)
	require.Nil(t, err)

	genesisElected := createDummyNodesList(4, "genesisElected")
	genesisEligible := createDummyNodesList(1, "genesisEligible")

	secondArgs := createArguments()
	secondArgs.BootStorer = bootStorer
	secondArgs.SelfPublicKey = firstArgs.SelfPublicKey
	secondArgs.Hasher = firstArgs.Hasher
	secondArgs.ElectedNodes = genesisElected
	secondArgs.EligibleNodes = genesisEligible
	secondArgs.Epoch = 5
	secondArgs.StartEpoch = 5
	_, err = NewNodesCoordinator(secondArgs)
	require.Nil(t, err)

	thirdArgs := createArguments()
	thirdArgs.BootStorer = bootStorer
	thirdArgs.SelfPublicKey = firstArgs.SelfPublicKey
	thirdArgs.Hasher = firstArgs.Hasher
	thirdArgs.ElectedNodes = genesisElected
	thirdArgs.EligibleNodes = genesisEligible
	thirdArgs.Epoch = 5
	thirdArgs.StartEpoch = 5
	thirdCoordinator, err := NewNodesCoordinator(thirdArgs)
	require.Nil(t, err)

	err = thirdCoordinator.LoadState(firstCoordinator.savedStateKey)
	require.Nil(t, err)

	validator, err := thirdCoordinator.GetValidatorWithPublicKey(realElected[0].PubKey())
	require.Nil(t, err)
	require.Equal(t, realElected[0].PubKey(), validator.PubKey())

	_, err = thirdCoordinator.GetValidatorWithPublicKey(genesisElected[0].PubKey())
	require.Equal(t, ErrValidatorNotFound, err)
}

func TestNodesCoordinator_ConstructorSavesStateOnGenesisStart(t *testing.T) {
	t.Parallel()

	elected := createDummyNodesList(4, "genesisElected")
	eligible := createDummyNodesList(1, "genesisEligible")
	bootStorer := mock.NewStorerMock()

	args := createArguments()
	args.BootStorer = bootStorer
	args.ElectedNodes = elected
	args.EligibleNodes = eligible
	args.Epoch = 0
	args.StartEpoch = 0
	coordinator, err := NewNodesCoordinator(args)
	require.Nil(t, err)

	args2 := createArguments()
	args2.BootStorer = bootStorer
	args2.SelfPublicKey = args.SelfPublicKey
	args2.Hasher = args.Hasher
	args2.ElectedNodes = createDummyNodesList(4, "otherElected")
	args2.EligibleNodes = createDummyNodesList(1, "otherEligible")
	args2.StartEpoch = 1
	coordinator2, err := NewNodesCoordinator(args2)
	require.Nil(t, err)

	err = coordinator2.LoadState(coordinator.savedStateKey)
	require.Nil(t, err)

	validator, err := coordinator2.GetValidatorWithPublicKey(elected[0].PubKey())
	require.Nil(t, err)
	require.Equal(t, elected[0].PubKey(), validator.PubKey())
}

type selectorEqualLenStub struct {
	indexes []uint32
}

func (s *selectorEqualLenStub) Select(_ []byte, _ uint32) ([]uint32, error) {
	return s.indexes, nil
}

func (s *selectorEqualLenStub) IsInterfaceNil() bool {
	return s == nil
}

func TestSelectValidators_SelectedIndexEqualToElectedListLengthShouldErr(t *testing.T) {
	t.Parallel()

	electedList := createDummyNodesList(3, "elected")
	selector := &selectorEqualLenStub{indexes: []uint32{3}}

	_, err := selectValidators(selector, []byte("randomness"), 1, electedList, 0)
	require.Equal(t, ErrSmallElectedListSize, err)
}
