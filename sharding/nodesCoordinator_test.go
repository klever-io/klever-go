package sharding

import (
	"encoding/json"
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

func TestNodesCoordinator_ConstructorDoesNotOverwriteSavedStateOnRestart(t *testing.T) {
	t.Parallel()

	// simulate a restart within the install epoch: the first coordinator
	// persists the real validators under hash(selfPubKey); a second
	// coordinator (the restart) is seeded with genesis nodes and must NOT
	// overwrite that saved state, otherwise LoadState reads genesis data back
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

	// EpochStartPrepare would rotate the key, but within the install epoch
	// the key is still hash(selfPubKey); persist under that key
	err = firstCoordinator.saveState(firstCoordinator.savedStateKey)
	require.Nil(t, err)

	// second coordinator (the restart) uses the same bootStorer and same
	// selfPubKey, so savedStateKey = hash(selfPubKey) is the same; it is
	// seeded with genesis nodes and StartEpoch > 0
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

	// now LoadState with the same key must return the REAL validators, not
	// genesis; if the constructor overwrote, this will return genesis data
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

	realPubKey := realElected[0].PubKey()
	validator, err := thirdCoordinator.GetValidatorWithPublicKey(realPubKey)
	require.Nil(t, err)
	require.Equal(t, realPubKey, validator.PubKey())

	genesisPubKey := genesisElected[0].PubKey()
	_, err = thirdCoordinator.GetValidatorWithPublicKey(genesisPubKey)
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

	// a second coordinator loading from the same key must see the saved state;
	// StartEpoch > 0 so the second constructor does not overwrite what was saved
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

func TestNodesCoordinator_LoadStateRefillsPublicKeyToValidatorMap(t *testing.T) {
	t.Parallel()

	// simulate a mid-epoch restart from storage: the persisted registry holds
	// the current epoch's validators, while the constructor is seeded from the
	// genesis nodes file only
	currentElected := createDummyNodesList(4, "currentElected")
	currentEligible := createDummyNodesList(1, "currentEligible")
	bootStorer := mock.NewStorerMock()

	argumentsBeforeRestart := createArguments()
	argumentsBeforeRestart.BootStorer = bootStorer
	argumentsBeforeRestart.ElectedNodes = currentElected
	argumentsBeforeRestart.EligibleNodes = currentEligible
	nodesCoordinatorBeforeRestart, err := NewNodesCoordinator(argumentsBeforeRestart)
	require.Nil(t, err)

	// persist the registry under a rotated key, as EpochStartPrepare does; the
	// constructor-time save under hash(selfPubKey) must not be reused here
	// because a subsequent constructor overwrites that key
	savedStateKey := []byte("rotated-saved-state-key")
	err = nodesCoordinatorBeforeRestart.saveState(savedStateKey)
	require.Nil(t, err)

	genesisElected := createDummyNodesList(4, "genesisElected")
	genesisEligible := createDummyNodesList(1, "genesisEligible")

	argumentsAfterRestart := createArguments()
	argumentsAfterRestart.BootStorer = bootStorer
	argumentsAfterRestart.ElectedNodes = genesisElected
	argumentsAfterRestart.EligibleNodes = genesisEligible
	nodesCoordinatorAfterRestart, err := NewNodesCoordinator(argumentsAfterRestart)
	require.Nil(t, err)

	currentPubKey := currentElected[0].PubKey()
	_, err = nodesCoordinatorAfterRestart.GetValidatorWithPublicKey(currentPubKey)
	require.Equal(t, ErrValidatorNotFound, err)

	err = nodesCoordinatorAfterRestart.LoadState(savedStateKey)
	require.Nil(t, err)

	validator, err := nodesCoordinatorAfterRestart.GetValidatorWithPublicKey(currentPubKey)
	require.Nil(t, err)
	require.Equal(t, currentPubKey, validator.PubKey())

	eligiblePubKey := currentEligible[0].PubKey()
	eligibleValidator, err := nodesCoordinatorAfterRestart.GetValidatorWithPublicKey(eligiblePubKey)
	require.Nil(t, err)
	require.Equal(t, eligiblePubKey, eligibleValidator.PubKey())

	// the map is rebuilt wholesale from the restored per-epoch configs, so the
	// genesis-seeded entries are gone
	_, err = nodesCoordinatorAfterRestart.GetValidatorWithPublicKey(genesisElected[0].PubKey())
	require.Equal(t, ErrValidatorNotFound, err)
}

// countingPutStorer counts Put calls and optionally fails them
type countingPutStorer struct {
	*mock.StorerMock
	puts    int
	failPut bool
}

func (c *countingPutStorer) Put(key, val []byte) error {
	c.puts++
	if c.failPut {
		return errors.New("put failed")
	}
	return c.StorerMock.Put(key, val)
}

func TestNodesCoordinator_ConstructorSurvivesFailedInitialSaveOnGenesis(t *testing.T) {
	t.Parallel()

	elected := createDummyNodesList(4, "genesisElected")

	t.Run("StartEpoch 0 attempts save and survives failure", func(t *testing.T) {
		t.Parallel()
		storer := &countingPutStorer{StorerMock: mock.NewStorerMock(), failPut: true}

		args := createArguments()
		args.Epoch = 0
		args.StartEpoch = 0
		args.ElectedNodes = elected
		args.BootStorer = storer

		coordinator, err := NewNodesCoordinator(args)
		require.Nil(t, err)
		require.NotNil(t, coordinator)
		require.Equal(t, 1, storer.puts)

		validator, err := coordinator.GetValidatorWithPublicKey(elected[0].PubKey())
		require.Nil(t, err)
		require.Equal(t, elected[0].PubKey(), validator.PubKey())
	})

	t.Run("replaces an unreadable stored blob", func(t *testing.T) {
		t.Parallel()
		bootStorer := mock.NewStorerMock()

		args := createArguments()
		args.Epoch = 5
		args.StartEpoch = 5
		args.ElectedNodes = elected

		// garbage under hash(selfPubKey) must be overwritten with a fresh,
		// readable registry
		savedKey := args.Hasher.Compute(string(args.SelfPublicKey))
		err := bootStorer.Put(append([]byte(core.NodesCoordinatorRegistryKeyPrefix), savedKey...), []byte("not json"))
		require.Nil(t, err)

		storer := &countingPutStorer{StorerMock: bootStorer}
		args.BootStorer = storer

		_, err = NewNodesCoordinator(args)
		require.Nil(t, err)
		require.Equal(t, 1, storer.puts)
	})

	t.Run("skips save when stored registry is at least as new", func(t *testing.T) {
		t.Parallel()
		bootStorer := mock.NewStorerMock()

		// a previous run at epoch 5 left its registry under hash(selfPubKey)
		firstArgs := createArguments()
		firstArgs.Epoch = 5
		firstArgs.StartEpoch = 5
		firstArgs.ElectedNodes = elected
		firstArgs.BootStorer = bootStorer
		_, err := NewNodesCoordinator(firstArgs)
		require.Nil(t, err)

		// the restart at the same epoch must not attempt any write
		storer := &countingPutStorer{StorerMock: bootStorer, failPut: true}
		secondArgs := createArguments()
		secondArgs.SelfPublicKey = firstArgs.SelfPublicKey
		secondArgs.Hasher = firstArgs.Hasher
		secondArgs.Epoch = 5
		secondArgs.StartEpoch = 5
		secondArgs.ElectedNodes = elected
		secondArgs.BootStorer = storer

		coordinator, err := NewNodesCoordinator(secondArgs)
		require.Nil(t, err)
		require.NotNil(t, coordinator)
		require.Equal(t, 0, storer.puts)
	})
}

func TestNodesCoordinator_ConstructorRefreshesStaleRegistryFromOlderEpoch(t *testing.T) {
	t.Parallel()

	// a node that once ran at genesis left an epoch-0 registry under
	// hash(selfPubKey); a restart at epoch 5 must refresh that key, otherwise a
	// bootstrap record stamped with it would silently restore genesis data on a
	// later restart and ComputeConsensusGroup would fail indefinitely
	bootStorer := mock.NewStorerMock()

	genesisElected := createDummyNodesList(4, "genesisElected")
	genesisArgs := createArguments()
	genesisArgs.Epoch = 0
	genesisArgs.StartEpoch = 0
	genesisArgs.ElectedNodes = genesisElected
	genesisArgs.BootStorer = bootStorer
	genesisCoordinator, err := NewNodesCoordinator(genesisArgs)
	require.Nil(t, err)

	currentElected := createDummyNodesList(4, "currentElected")
	restartArgs := createArguments()
	restartArgs.SelfPublicKey = genesisArgs.SelfPublicKey
	restartArgs.Hasher = genesisArgs.Hasher
	restartArgs.Epoch = 5
	restartArgs.StartEpoch = 5
	restartArgs.ElectedNodes = currentElected
	restartArgs.BootStorer = bootStorer
	_, err = NewNodesCoordinator(restartArgs)
	require.Nil(t, err)

	// loading hash(selfPubKey) must now resolve the epoch-5 validators, not the
	// stale genesis set
	loadArgs := createArguments()
	loadArgs.SelfPublicKey = genesisArgs.SelfPublicKey
	loadArgs.Hasher = genesisArgs.Hasher
	loadArgs.Epoch = 5
	loadArgs.StartEpoch = 5
	loadArgs.BootStorer = bootStorer
	loadCoordinator, err := NewNodesCoordinator(loadArgs)
	require.Nil(t, err)

	err = loadCoordinator.LoadState(genesisCoordinator.savedStateKey)
	require.Nil(t, err)

	validator, err := loadCoordinator.GetValidatorWithPublicKey(currentElected[0].PubKey())
	require.Nil(t, err)
	require.Equal(t, currentElected[0].PubKey(), validator.PubKey())
	require.Equal(t, uint32(5), loadCoordinator.currentEpoch.Load())
}

func TestNodesCoordinator_LoadStateRejectsEmptyRegistry(t *testing.T) {
	t.Parallel()

	// a registry blob that unmarshals cleanly but restores no epoch configs must
	// not be installed: the coordinator would report ready with an empty lookup
	// and every validator lookup would fail (the exact stall this PR fixes)
	bootStorer := mock.NewStorerMock()
	emptyRegistry := &NodesCoordinatorRegistry{
		EpochsConfig: map[string]*EpochValidators{},
		CurrentEpoch: 7,
	}
	data, err := json.Marshal(emptyRegistry)
	require.Nil(t, err)

	key := []byte("empty-registry-key")
	err = bootStorer.Put(append([]byte(core.NodesCoordinatorRegistryKeyPrefix), key...), data)
	require.Nil(t, err)

	args := createArguments()
	args.Epoch = 5
	args.StartEpoch = 5
	args.BootStorer = bootStorer
	coordinator, err := NewNodesCoordinator(args)
	require.Nil(t, err)
	require.False(t, coordinator.IsReady())

	keyBefore := coordinator.GetSavedStateKey()

	err = coordinator.LoadState(key)
	require.ErrorIs(t, err, ErrEmptyRestoredRegistry)

	// the failed restore left no trace: not ready, key and epoch untouched
	require.False(t, coordinator.IsReady())
	require.Equal(t, keyBefore, coordinator.GetSavedStateKey())
	require.Equal(t, uint32(5), coordinator.currentEpoch.Load())
}

func TestNodesCoordinator_LoadStateConversionFailureLeavesStateUntouched(t *testing.T) {
	t.Parallel()

	// registryToNodesCoordinator fails on a malformed epoch key; savedStateKey
	// and currentEpoch must not have been mutated on that path, otherwise a
	// later saveState would overwrite a still-valid registry under the foreign
	// key with genesis data
	bootStorer := mock.NewStorerMock()
	malformedRegistry := &NodesCoordinatorRegistry{
		EpochsConfig: map[string]*EpochValidators{
			"not-a-number": {},
		},
		CurrentEpoch: 7,
	}
	data, err := json.Marshal(malformedRegistry)
	require.Nil(t, err)

	key := []byte("malformed-registry-key")
	err = bootStorer.Put(append([]byte(core.NodesCoordinatorRegistryKeyPrefix), key...), data)
	require.Nil(t, err)

	args := createArguments()
	args.Epoch = 5
	args.StartEpoch = 5
	args.BootStorer = bootStorer
	coordinator, err := NewNodesCoordinator(args)
	require.Nil(t, err)

	keyBefore := coordinator.GetSavedStateKey()

	err = coordinator.LoadState(key)
	require.NotNil(t, err)

	require.False(t, coordinator.IsReady())
	require.Equal(t, keyBefore, coordinator.GetSavedStateKey())
	require.Equal(t, uint32(5), coordinator.currentEpoch.Load())
}

func TestNodesCoordinator_SetNodesUnblocksValidatorLookupBeforeLoadState(t *testing.T) {
	t.Parallel()

	// the fast-bootstrap path (cmd/node) calls SetNodes right after a
	// non-genesis construction, long before LoadState runs in StartConsensus;
	// the installed validators are authoritative and must resolve through
	// GetValidatorWithPublicKey immediately, while the startup readiness gate
	// (IsReady) must keep failing until LoadState restores the full state
	restored := createDummyNodesList(4, "restoredElected")

	args := createArguments()
	args.Epoch = 7
	args.StartEpoch = 7
	coordinator, err := NewNodesCoordinator(args)
	require.Nil(t, err)

	// before SetNodes the lookup only holds the genesis seeding and must refuse
	_, err = coordinator.GetValidatorWithPublicKey(restored[0].PubKey())
	require.Equal(t, ErrNodesCoordinatorNotReady, err)

	err = coordinator.SetNodes(restored, createDummyNodesList(1, "restoredEligible"), make([]Validator, 0), args.Epoch-1)
	require.Nil(t, err)

	validator, err := coordinator.GetValidatorWithPublicKey(restored[0].PubKey())
	require.Nil(t, err)
	require.Equal(t, restored[0].PubKey(), validator.PubKey())

	// the lookup unblocking must not weaken the startup gate
	require.False(t, coordinator.IsReady())
}

func TestNodesCoordinator_GetValidatorWithPublicKeyNotReady(t *testing.T) {
	t.Parallel()

	// before LoadState a non-genesis coordinator only holds its genesis seeding;
	// answering from it would present genesis-era data as authoritative to the
	// p2p peer classification, so the lookup must refuse until ready
	elected := createDummyNodesList(4, "elected")
	bootStorer := mock.NewStorerMock()

	firstArgs := createArguments()
	firstArgs.Epoch = 5
	firstArgs.StartEpoch = 5
	firstArgs.ElectedNodes = elected
	firstArgs.BootStorer = bootStorer
	firstCoordinator, err := NewNodesCoordinator(firstArgs)
	require.Nil(t, err)

	savedStateKey := []byte("ready-gate-saved-state-key")
	err = firstCoordinator.saveState(savedStateKey)
	require.Nil(t, err)

	secondArgs := createArguments()
	secondArgs.Epoch = 5
	secondArgs.StartEpoch = 5
	secondArgs.ElectedNodes = elected
	secondArgs.BootStorer = bootStorer
	coordinator, err := NewNodesCoordinator(secondArgs)
	require.Nil(t, err)

	_, err = coordinator.GetValidatorWithPublicKey(elected[0].PubKey())
	require.Equal(t, ErrNodesCoordinatorNotReady, err)

	err = coordinator.LoadState(savedStateKey)
	require.Nil(t, err)

	validator, err := coordinator.GetValidatorWithPublicKey(elected[0].PubKey())
	require.Nil(t, err)
	require.Equal(t, elected[0].PubKey(), validator.PubKey())
}

func TestNodesCoordinator_LoadStateLaterEpochWinsForSharedPublicKey(t *testing.T) {
	t.Parallel()

	// the same public key appears in five restored epochs with a different index
	// in each; computePublicKeyToValidatorMap merges epochs in ascending order,
	// so the entry from the latest epoch must survive in the lookup.
	// Go's map iteration randomization picks a start bucket/offset per map
	// instance, so a single LoadState can accidentally iterate in ascending
	// order even without the sort. Running 20 iterations (each creating a fresh
	// map via LoadState) makes the probability of all 20 being accidentally
	// correct negligible (< 10^-12 at the observed ~25% single-pass rate).
	sharedPubKey := []byte("sharedPubKey")

	epochs := []uint32{10, 20, 30, 40, 50}
	configs := make(map[uint32]*epochNodesConfig, len(epochs))
	var latestValidator Validator
	for i, epoch := range epochs {
		v, err := NewValidator([]byte(fmt.Sprintf("owner-epoch-%d", epoch)), sharedPubKey, 1, uint32(i))
		require.Nil(t, err)
		configs[epoch] = &epochNodesConfig{
			electedList:  append(createDummyNodesList(3, fmt.Sprintf("epoch%d", epoch)), v),
			eligibleList: make([]Validator, 0),
			leavingList:  make([]Validator, 0),
		}
		latestValidator = v
	}

	bootStorer := mock.NewStorerMock()
	argsBeforeRestart := createArguments()
	argsBeforeRestart.BootStorer = bootStorer
	coordinatorBeforeRestart, err := NewNodesCoordinator(argsBeforeRestart)
	require.Nil(t, err)

	coordinatorBeforeRestart.nodesConfig = configs

	savedStateKey := []byte("multi-epoch-saved-state-key")
	err = coordinatorBeforeRestart.saveState(savedStateKey)
	require.Nil(t, err)

	for attempt := 0; attempt < 20; attempt++ {
		argsAfterRestart := createArguments()
		argsAfterRestart.BootStorer = bootStorer
		coordinatorAfterRestart, err := NewNodesCoordinator(argsAfterRestart)
		require.Nil(t, err)

		err = coordinatorAfterRestart.LoadState(savedStateKey)
		require.Nil(t, err)

		validator, err := coordinatorAfterRestart.GetValidatorWithPublicKey(sharedPubKey)
		require.Nil(t, err)
		require.Equal(t, latestValidator.Index(), validator.Index(),
			"attempt %d: expected latest epoch's validator index", attempt)
	}
}

func TestNodesCoordinator_SetNodesRebuildsPublicKeyToValidatorMap(t *testing.T) {
	t.Parallel()

	// the fast-bootstrap path (cmd/node) seeds the coordinator from the genesis
	// nodes file and then calls SetNodes with the validators restored from the
	// bootstrap registry; the lookup map must follow that mutation, otherwise
	// GetValidatorWithPublicKey keeps resolving only genesis keys and the p2p
	// validator-originator gate rejects every gossiped header

	t.Run("existing epoch config is replaced in the lookup", func(t *testing.T) {
		t.Parallel()
		restoredElected := createDummyNodesList(4, "restoredElected")
		restoredEligible := createDummyNodesList(1, "restoredEligible")

		args := createArguments()
		coordinator, err := NewNodesCoordinator(args)
		require.Nil(t, err)

		err = coordinator.SetNodes(restoredElected, restoredEligible, make([]Validator, 0), args.Epoch)
		require.Nil(t, err)

		validator, err := coordinator.GetValidatorWithPublicKey(restoredElected[0].PubKey())
		require.Nil(t, err)
		require.Equal(t, restoredElected[0].PubKey(), validator.PubKey())

		eligibleValidator, err := coordinator.GetValidatorWithPublicKey(restoredEligible[0].PubKey())
		require.Nil(t, err)
		require.Equal(t, restoredEligible[0].PubKey(), eligibleValidator.PubKey())

		// the sole epoch config was replaced, so its previous seeding is gone
		_, err = coordinator.GetValidatorWithPublicKey(args.ElectedNodes[0].PubKey())
		require.Equal(t, ErrValidatorNotFound, err)
	})

	t.Run("nil elected or eligible input is rejected", func(t *testing.T) {
		t.Parallel()
		args := createArguments()
		coordinator, err := NewNodesCoordinator(args)
		require.Nil(t, err)

		err = coordinator.SetNodes(nil, createDummyNodesList(1, "eligible"), nil, args.Epoch)
		require.Equal(t, ErrNilInputNodesMap, err)

		err = coordinator.SetNodes(createDummyNodesList(4, "elected"), nil, nil, args.Epoch)
		require.Equal(t, ErrNilInputNodesMap, err)
	})

	t.Run("new epoch config is added to the lookup", func(t *testing.T) {
		t.Parallel()
		restoredElected := createDummyNodesList(4, "restoredElected")
		restoredEligible := createDummyNodesList(1, "restoredEligible")

		args := createArguments()
		coordinator, err := NewNodesCoordinator(args)
		require.Nil(t, err)

		// mirrors node.go: SetNodes targets currentEpoch-1, which may not have
		// a config yet on this coordinator
		err = coordinator.SetNodes(restoredElected, restoredEligible, make([]Validator, 0), args.Epoch+3)
		require.Nil(t, err)

		validator, err := coordinator.GetValidatorWithPublicKey(restoredElected[0].PubKey())
		require.Nil(t, err)
		require.Equal(t, restoredElected[0].PubKey(), validator.PubKey())
	})
}

func TestNodesCoordinator_IsReadyOnGenesisStart(t *testing.T) {
	t.Parallel()

	args := createArguments()
	args.Epoch = 0
	args.StartEpoch = 0
	coordinator, err := NewNodesCoordinator(args)
	require.Nil(t, err)

	require.True(t, coordinator.IsReady())
}

func TestNodesCoordinator_IsReadyFalseOnRestartBeforeLoadState(t *testing.T) {
	t.Parallel()

	args := createArguments()
	args.StartEpoch = 1
	coordinator, err := NewNodesCoordinator(args)
	require.Nil(t, err)

	require.False(t, coordinator.IsReady())
}

func TestNodesCoordinator_IsReadyTrueAfterSuccessfulLoadState(t *testing.T) {
	t.Parallel()

	elected := createDummyNodesList(4, "currentElected")
	eligible := createDummyNodesList(1, "currentEligible")
	bootStorer := mock.NewStorerMock()

	argsBeforeRestart := createArguments()
	argsBeforeRestart.BootStorer = bootStorer
	argsBeforeRestart.ElectedNodes = elected
	argsBeforeRestart.EligibleNodes = eligible
	coordinatorBeforeRestart, err := NewNodesCoordinator(argsBeforeRestart)
	require.Nil(t, err)

	savedStateKey := []byte("rotated-saved-state-key")
	err = coordinatorBeforeRestart.saveState(savedStateKey)
	require.Nil(t, err)

	argsAfterRestart := createArguments()
	argsAfterRestart.BootStorer = bootStorer
	argsAfterRestart.StartEpoch = 1
	coordinatorAfterRestart, err := NewNodesCoordinator(argsAfterRestart)
	require.Nil(t, err)
	require.False(t, coordinatorAfterRestart.IsReady())

	err = coordinatorAfterRestart.LoadState(savedStateKey)
	require.Nil(t, err)
	require.True(t, coordinatorAfterRestart.IsReady())
}

func TestNodesCoordinator_IsReadyStaysFalseAfterFailedLoadState(t *testing.T) {
	t.Parallel()

	args := createArguments()
	args.StartEpoch = 1
	coordinator, err := NewNodesCoordinator(args)
	require.Nil(t, err)
	require.False(t, coordinator.IsReady())

	// no registry saved under this key: LoadState fails and must not flip readiness
	err = coordinator.LoadState([]byte("missing-key"))
	require.NotNil(t, err)
	require.False(t, coordinator.IsReady())
}
