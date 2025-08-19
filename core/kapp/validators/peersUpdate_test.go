package validators

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/common/mock"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/rating"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/sharding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getValidators(n int) [][]byte {
	validators := make([][]byte, n)
	for i := 0; i < n; i++ {
		validators[i] = []byte(fmt.Sprintf("validator%02d", i))
	}
	return validators
}

const (
	validatorIncreaseRatingStep        = int32(1)
	validatorDecreaseRatingStep        = int32(-2)
	proposerIncreaseRatingStep         = int32(2)
	proposerDecreaseRatingStep         = int32(-4)
	minRating                          = uint32(1)
	maxRating                          = uint32(10000000)
	startRating                        = uint32(5000001)
	consecutiveMissedBlocksPenaltyMeta = 1.2
)

func createDefaultChances() []process.SelectionChance {
	chances := []process.SelectionChance{
		&rating.SelectionChance{MaxThreshold: 0, ChancePercent: 5},
		&rating.SelectionChance{MaxThreshold: 1000000, ChancePercent: 0},
		&rating.SelectionChance{MaxThreshold: 2000000, ChancePercent: 16},
		&rating.SelectionChance{MaxThreshold: 3000000, ChancePercent: 17},
		&rating.SelectionChance{MaxThreshold: 4000000, ChancePercent: 18},
		&rating.SelectionChance{MaxThreshold: 5000000, ChancePercent: 19},
		&rating.SelectionChance{MaxThreshold: 6000000, ChancePercent: 20},
		&rating.SelectionChance{MaxThreshold: 7000000, ChancePercent: 21},
		&rating.SelectionChance{MaxThreshold: 8000000, ChancePercent: 22},
		&rating.SelectionChance{MaxThreshold: 9000000, ChancePercent: 23},
		&rating.SelectionChance{MaxThreshold: 10000000, ChancePercent: 24},
	}

	return chances
}

func createDefaultRatingsData() *mock.RatingsInfoMock {
	ratingsData := &mock.RatingsInfoMock{
		StartRatingProperty: startRating,
		MaxRatingProperty:   maxRating,
		MinRatingProperty:   minRating,
		RatingsStepDataProperty: &mock.RatingStepMock{
			ProposerIncreaseRatingStepProperty:     proposerIncreaseRatingStep,
			ProposerDecreaseRatingStepProperty:     proposerDecreaseRatingStep,
			ValidatorIncreaseRatingStepProperty:    validatorIncreaseRatingStep,
			ValidatorDecreaseRatingStepProperty:    validatorDecreaseRatingStep,
			ConsecutiveMissedBlocksPenaltyProperty: consecutiveMissedBlocksPenaltyMeta,
		},
		SignedBlocksThresholdProperty: 0.5,
		SelectionChancesProperty:      createDefaultChances(),
	}

	return ratingsData
}

func setupValidatorsKApp(t *testing.T) *validatorsKApp {
	args := createMockArgs()

	v, err := NewValidatorKApp(args)
	require.NoError(t, err)
	require.NotNil(t, v)

	accCacher := &mock.AccountsCacherStub{
		LoadPeerCalled: func(peer []byte) (state.PeerAccountHandler, error) {
			return state.NewPeerAccount(peer)
		},
	}
	require.NoError(t, v.SetAccountsCacher(accCacher))

	// set functional rater
	rd := createDefaultRatingsData()
	bsr, err := rating.NewBlockSigningRater(rd)
	require.NoError(t, err)
	v.SetRater(bsr)

	return v
}

func addFunctionalCacher(t *testing.T, v *validatorsKApp) {
	peersMapper := make(map[string]state.PeerAccountHandler)
	usersMapper := make(map[string]state.UserAccountHandler)

	accCacher := &mock.AccountsCacherStub{
		LoadPeerCalled: func(peer []byte) (state.PeerAccountHandler, error) {
			if _, ok := peersMapper[string(peer)]; ok {
				return peersMapper[string(peer)], nil
			}

			acc, err := state.NewPeerAccount(peer)
			if err != nil {
				return nil, err
			}

			peersMapper[string(peer)] = acc
			return acc, nil
		},
		GetExistingPeerCalled: func(peer []byte) (state.PeerAccountHandler, error) {
			if acc, ok := peersMapper[string(peer)]; ok {
				return acc, nil
			}

			return nil, common.ErrAccNotFound
		},
		LoadUserCalled: func(user []byte) (state.UserAccountHandler, error) {
			if _, ok := usersMapper[string(user)]; ok {
				return usersMapper[string(user)], nil
			}

			acc, err := state.NewUserAccount(user)
			if err != nil {
				return nil, err
			}

			usersMapper[string(user)] = acc
			return acc, nil
		},
		GetExistingUserCalled: func(user []byte) (state.UserAccountHandler, error) {
			if acc, ok := usersMapper[string(user)]; ok {
				return acc, nil
			}

			return nil, common.ErrAccNotFound
		},
	}

	require.NoError(t, v.SetAccountsCacher(accCacher))
}

func updatePeers(addresses [][]byte, v *validatorsKApp, f func(state.PeerAccountHandler) error) error {
	for _, address := range addresses {
		peerAcc, err := v.accountsCacher.LoadPeer(address)
		if err != nil {
			return err
		}

		err = f(peerAcc)
		if err != nil {
			return err
		}
	}

	return nil
}

func checkPeers(addresses [][]byte, v *validatorsKApp, f func(state.PeerAccountHandler) error) error {
	for _, address := range addresses {
		peerAcc, err := v.accountsCacher.GetExistingPeer(address)
		if err != nil {
			return err
		}

		err = f(peerAcc)
		if err != nil {
			return err
		}
	}

	return nil
}

func addPeersFromValidatorInfo(validators []*state.ValidatorInfo, v *validatorsKApp) {
	// set peers info in cache
	for _, val := range validators {
		peerAcc, _ := v.accountsCacher.LoadPeer(val.PublicKey)
		peerAcc.SetListAndIndex(state.List(state.List_value[val.List]), 0)
		peerAcc.SetTempRating(val.TempRating)
	}
}

func TestDecreaseAll(t *testing.T) {
	t.Parallel()

	// 21 validators
	validators := getValidators(21)

	t.Run("Basic Decrease Calculation prior fork", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		v.forkController.(*mock.ForkControllerStub).EnableSmartContractsValue = false

		missedSlots := uint64(100)
		consensusGroupSize := 21

		err := v.DecreaseAll(validators, missedSlots, consensusGroupSize)
		assert.NoError(t, err)

		// Verify calculations
		expectedLeaderAppearances := uint32(5)      // 100 / 21 + 1 rounded down
		expectedValidatorAppearances := uint32(101) // 21 * 4.7619... + 1 rounded down

		for _, validator := range validators {
			peerAcc, err := v.accountsCacher.GetExistingPeer(validator)
			assert.NoError(t, err)
			assert.Equal(t, expectedLeaderAppearances, peerAcc.GetLeaderSuccessRateFailure())
			assert.Equal(t, expectedValidatorAppearances, peerAcc.GetValidatorSuccessRateFailure())
		}
	})

	t.Run("Basic Decrease Calculation post fork", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		missedSlots := uint64(100)
		consensusGroupSize := 21

		err := v.DecreaseAll(validators, missedSlots, consensusGroupSize)
		assert.NoError(t, err)

		// Verify calculations
		expectedLeaderAppearances := uint32(5)     // 100 / 21 + 1 rounded down
		expectedValidatorAppearances := uint32(96) // 20 * 4.7619... + 1  rounded down

		for _, validator := range validators {
			peerAcc, err := v.accountsCacher.GetExistingPeer(validator)
			assert.NoError(t, err)
			assert.Equal(t, expectedLeaderAppearances, peerAcc.GetLeaderSuccessRateFailure())
			assert.Equal(t, expectedValidatorAppearances, peerAcc.GetValidatorSuccessRateFailure())
		}
	})

	t.Run("Rating Decrease", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		validators := getValidators(21)
		missedSlots := uint64(100)
		consensusGroupSize := 21

		// set initial rating
		err := updatePeers(validators, v, func(peerAcc state.PeerAccountHandler) error {
			peerAcc.SetTempRating(startRating)
			return nil
		})
		assert.NoError(t, err)

		// missed slots = 100
		// consensus group size = 21
		// estimate leader appearances = 5
		// estimate validator appearances = 96
		// leader decrease step = -4 * 5 = -20
		// validator decrease step = -2 * 96 = -192
		// estimate final rating = 5000001 - 20 - 192 = 4999789
		estimateFinalRating := uint32(4999789)

		err = v.DecreaseAll(validators, missedSlots, consensusGroupSize)
		assert.NoError(t, err)

		err = checkPeers(validators, v, func(peerAcc state.PeerAccountHandler) error {
			assert.Less(t, peerAcc.GetTempRating(), startRating)
			assert.Equal(t, estimateFinalRating, peerAcc.GetTempRating())
			return nil
		})

		assert.NoError(t, err)
	})

	t.Run("Jailing Check", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)
		validators := [][]byte{[]byte("validator1")}
		missedSlots := uint64(100) // Large number to ensure rating drops significantly
		consensusGroupSize := 21

		err := updatePeers(validators, v, func(peerAcc state.PeerAccountHandler) error {
			// Set temp rating closer 0 change
			peerAcc.SetTempRating(1000000)
			return nil
		})
		assert.NoError(t, err)

		err = v.DecreaseAll(validators, missedSlots, consensusGroupSize)
		assert.NoError(t, err)

		err = checkPeers(validators, v, func(peerAcc state.PeerAccountHandler) error {
			assert.Equal(t, state.List_jailed, peerAcc.GetList())
			return nil
		})
		assert.NoError(t, err)
	})
}

func TestProcessRatingsEndOfEpoch(t *testing.T) {
	t.Parallel()

	t.Run("Happy path - process eligible validators", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		// Create test validators
		validators := []*state.ValidatorInfo{
			{
				PublicKey:        []byte("validator1"),
				List:             string(core.ElectedList),
				TempRating:       5000000,
				ValidatorSuccess: 60,
				ValidatorFailure: 40,
			},
			{
				PublicKey:        []byte("validator2"),
				List:             string(core.WaitingList),
				TempRating:       5000000,
				ValidatorSuccess: 40,
				ValidatorFailure: 60,
			},
		}
		// set peers info in cache
		addPeersFromValidatorInfo(validators, v)

		err := v.ProcessRatingsEndOfEpoch(validators)
		assert.NoError(t, err)

		// Check that the eligible validator's rating was processed
		peerAcc1, _ := v.accountsCacher.GetExistingPeer([]byte("validator1"))
		assert.Equal(t, uint32(5000000), peerAcc1.GetTempRating()) // Rating unchanged as it's above threshold

		// Check that the ineligible validator's rating was not processed
		peerAcc2, _ := v.accountsCacher.GetExistingPeer([]byte("validator2"))
		assert.Equal(t, uint32(5000000), peerAcc2.GetTempRating()) // Rating unchanged as it wasn't elected
	})

	t.Run("Process validator leaving elected list", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		validators := []*state.ValidatorInfo{
			{
				PublicKey:        []byte("validator1"),
				List:             string(core.LeavingList),
				TempRating:       5000000,
				ValidatorSuccess: 40,
				ValidatorFailure: 60,
			},
		}
		addPeersFromValidatorInfo(validators, v)

		err := v.ProcessRatingsEndOfEpoch(validators)
		assert.NoError(t, err)

		peerAcc, _ := v.accountsCacher.GetExistingPeer([]byte("validator1"))
		// Rating decreased by 40 (revert success block steps as it did not reach the threshold)
		assert.Equal(t, peerAcc.GetTempRating(), uint32(5000000-40))
	})

	t.Run("Error in verifySignaturesBelowSignedThreshold", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		v.accountsCacher = &mock.AccountsCacherStub{
			LoadPeerCalled: func(peer []byte) (state.PeerAccountHandler, error) {
				return nil, common.ErrNilTrie
			},
		}

		validators := []*state.ValidatorInfo{
			{
				PublicKey: []byte("validator1"),
				List:      string(core.ElectedList),
			},
		}

		err := v.ProcessRatingsEndOfEpoch(validators)

		assert.Error(t, err)
		assert.Equal(t, common.ErrNilTrie, err)
	})

	t.Run("Error getting KApp", func(t *testing.T) {
		v := setupValidatorsKApp(t)

		v.accountsCacher = &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return nil, common.ErrNilTrie
			},
		}

		validators := []*state.ValidatorInfo{
			{
				PublicKey: []byte("validator1"),
				List:      string(core.ElectedList),
			},
		}

		err := v.ProcessRatingsEndOfEpoch(validators)

		assert.Error(t, err)
		assert.Equal(t, common.ErrNilTrie, err)
	})
}

func TestResetValidatorStatisticsAtNewEpoch(t *testing.T) {
	t.Parallel()

	t.Run("Happy path - all validators processed", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		// Create test validators
		validators := []*state.ValidatorInfo{
			{
				PublicKey:       []byte("validator1"),
				IsPubKeyRevoked: false,
			},
			{
				PublicKey:       []byte("validator2"),
				IsPubKeyRevoked: false,
			},
		}

		// Set initial state for validators
		for _, val := range validators {
			peerAcc, _ := v.accountsCacher.LoadPeer(val.PublicKey)
			peerAcc.IncreaseLeaderSuccessRate(5)
			peerAcc.IncreaseValidatorSuccessRate(10)
			peerAcc.SetListAndIndex(state.List_eligible, 0)
		}

		updatedList, err := v.ResetValidatorStatisticsAtNewEpoch(validators)

		assert.NoError(t, err)
		assert.Len(t, updatedList, 2)

		for _, updatedVal := range updatedList {
			peerAcc, _ := v.accountsCacher.GetExistingPeer(updatedVal.PublicKey)
			assert.Equal(t, uint32(0), peerAcc.GetLeaderSuccessRate().NumSuccess)
			assert.Equal(t, uint32(0), peerAcc.GetValidatorSuccessRate().NumSuccess)
			assert.Equal(t, uint32(5), peerAcc.GetTotalLeaderSuccessRate().NumSuccess)
			assert.Equal(t, uint32(10), peerAcc.GetTotalValidatorSuccessRate().NumSuccess)
		}
	})

	t.Run("Skip revoked validator", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		validators := []*state.ValidatorInfo{
			{
				PublicKey:       []byte("validator1"),
				IsPubKeyRevoked: false,
			},
			{
				PublicKey:       []byte("validator2"),
				IsPubKeyRevoked: true,
			},
		}

		updatedList, err := v.ResetValidatorStatisticsAtNewEpoch(validators)

		assert.NoError(t, err)
		assert.Len(t, updatedList, 1)
		assert.Equal(t, []byte("validator1"), updatedList[0].PublicKey)
	})

	t.Run("Jail validator if needed", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		validators := []*state.ValidatorInfo{
			{
				PublicKey:       []byte("validator1"),
				IsPubKeyRevoked: false,
				List:            string(state.List_waiting),
			},
		}

		// Set initial state for validator
		peerAcc, _ := v.accountsCacher.LoadPeer(validators[0].PublicKey)
		peerAcc.SetTempRating(1) // Set a very low rating to trigger jailing

		updatedList, err := v.ResetValidatorStatisticsAtNewEpoch(validators)

		assert.NoError(t, err)
		assert.Len(t, updatedList, 1)

		peerAcc, _ = v.accountsCacher.GetExistingPeer(updatedList[0].PublicKey)
		assert.Equal(t, state.List_jailed, peerAcc.GetList())
	})

	t.Run("Error loading peer account", func(t *testing.T) {
		v := setupValidatorsKApp(t)

		// Mock an error when loading peer account
		v.accountsCacher = &mock.AccountsCacherStub{
			LoadPeerCalled: func(peer []byte) (state.PeerAccountHandler, error) {
				return nil, errors.New("load error")
			},
		}

		validators := []*state.ValidatorInfo{
			{
				PublicKey:       []byte("validator1"),
				IsPubKeyRevoked: false,
			},
		}

		updatedList, err := v.ResetValidatorStatisticsAtNewEpoch(validators)

		assert.Error(t, err)
		assert.Nil(t, updatedList)
	})
}

func TestUpdateMissedBlocksCounters(t *testing.T) {
	t.Parallel()

	t.Run("Update counters for multiple validators prior fork", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		v.forkController.(*mock.ForkControllerStub).EnableSmartContractsValue = false

		validators := map[string]*ValidatorData{
			"addr1": {BlsPubKey: []byte("bls1")},
			"addr2": {BlsPubKey: []byte("bls2")},
		}

		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					data, _ := v.marshalizer.Marshal(validators[strings.Replace(string(key), "VAL/", "", 1)])
					return data
				},
			}, nil
		}

		// Create test data
		mb := map[string]kapp.RateChange{
			"addr1": {Leader: 5, Validator: 10},
			"addr2": {Leader: 3, Validator: 7},
		}

		err := v.UpdateMissedBlocksCounters(mb)
		assert.NoError(t, err)

		// Verify the counters were updated correctly
		for addr, counters := range mb {
			peerAcc, _ := v.accountsCacher.GetExistingPeer(validators[addr].BlsPubKey)
			assert.Equal(t, counters.Leader+counters.Validator, peerAcc.GetLeaderSuccessRate().NumFailure)
			assert.Equal(t, uint32(0), peerAcc.GetValidatorSuccessRate().NumFailure)
		}
	})

	t.Run("Update counters for multiple validators after fork", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		validators := map[string]*ValidatorData{
			"addr1": {BlsPubKey: []byte("bls1")},
			"addr2": {BlsPubKey: []byte("bls2")},
		}

		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					data, _ := v.marshalizer.Marshal(validators[strings.Replace(string(key), "VAL/", "", 1)])
					return data
				},
			}, nil
		}

		// Create test data
		mb := map[string]kapp.RateChange{
			"addr1": {Leader: 5, Validator: 10},
			"addr2": {Leader: 3, Validator: 7},
		}

		err := v.UpdateMissedBlocksCounters(mb)
		assert.NoError(t, err)

		// Verify the counters were updated correctly
		for addr, counters := range mb {
			peerAcc, _ := v.accountsCacher.GetExistingPeer(validators[addr].BlsPubKey)
			assert.Equal(t, counters.Leader, peerAcc.GetLeaderSuccessRate().NumFailure)
			assert.Equal(t, counters.Validator, peerAcc.GetValidatorSuccessRate().NumFailure)
		}
	})

	t.Run("No missed blocks", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		saveAllCalled := false
		v.accountsCacher.(*mock.AccountsCacherStub).SaveAllCalled = func() error {
			saveAllCalled = true
			return nil
		}

		mb := map[string]kapp.RateChange{}

		err := v.UpdateMissedBlocksCounters(mb)
		assert.NoError(t, err)
		// Verify that SaveAll was called even with empty input
		assert.True(t, saveAllCalled)
	})

	t.Run("Error getting KApp", func(t *testing.T) {
		v := setupValidatorsKApp(t)

		v.accountsCacher = &mock.AccountsCacherStub{
			LoadKAppCalled: func(address []byte) (state.KAppAccountHandler, error) {
				return nil, common.ErrNilTrie
			},
		}

		mb := map[string]kapp.RateChange{
			"addr1": {Leader: 5, Validator: 10},
		}

		err := v.UpdateMissedBlocksCounters(mb)

		assert.Error(t, err)
		assert.Equal(t, common.ErrNilTrie, err)
	})

	t.Run("Error getting validator", func(t *testing.T) {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					return make([]byte, 0)
				},
			}, nil
		}

		mb := map[string]kapp.RateChange{
			"addr1": {Leader: 5, Validator: 10},
		}

		err := v.UpdateMissedBlocksCounters(mb)

		assert.Error(t, err)
		assert.Equal(t, common.ErrValidatorNotFound, err)
	})

	t.Run("Error loading peer account", func(t *testing.T) {
		v := setupValidatorsKApp(t)

		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					data, _ := v.marshalizer.Marshal(&ValidatorData{BlsPubKey: []byte("bls1")})
					return data
				},
			}, nil
		}
		v.accountsCacher.(*mock.AccountsCacherStub).LoadPeerCalled = func(peer []byte) (state.PeerAccountHandler, error) {
			return nil, common.ErrNilTrie
		}

		mb := map[string]kapp.RateChange{
			"addr1": {Leader: 5, Validator: 10},
		}

		err := v.UpdateMissedBlocksCounters(mb)

		assert.Error(t, err)
		assert.Equal(t, common.ErrNilTrie, err)
	})
}
func TestSaveUpdatesForNodesMap(t *testing.T) {
	t.Parallel()

	setupTest := func() *validatorsKApp {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					blsPubKey := strings.Replace(string(key), "VAL/", "", 1)
					data, _ := v.marshalizer.Marshal(&ValidatorData{BlsPubKey: []byte(blsPubKey)})
					return data
				},
			}, nil
		}

		return v
	}

	t.Run("Happy path - no nodes forced to remain", func(t *testing.T) {
		v := setupTest()

		nodeOwners := [][]byte{[]byte("owner1"), []byte("owner2")}
		peerType := state.List_eligible.String()

		// Setup initial peer accounts
		for _, owner := range nodeOwners {
			peerAcc, _ := v.accountsCacher.LoadPeer(owner)
			peerAcc.SetRating(6000000) // High rating
			peerAcc.SetListAndIndex(state.List_eligible, 0)
		}

		nodeForcedToRemain, err := v.SaveUpdatesForNodesMap(nodeOwners, peerType)
		assert.NoError(t, err)
		assert.False(t, nodeForcedToRemain)

		// Check that peer accounts were updated correctly
		for _, owner := range nodeOwners {
			peerAcc, _ := v.accountsCacher.GetExistingPeer(owner)
			assert.Equal(t, state.List_eligible, peerAcc.GetList())
		}
	})

	t.Run("Some nodes forced to remain", func(t *testing.T) {
		v := setupTest()

		nodeOwners := [][]byte{[]byte("owner1"), []byte("owner2")}
		peerType := state.List_eligible.String()

		// Setup initial peer accounts
		validators := []*state.ValidatorInfo{
			{
				PublicKey: []byte("owner1"),
				List:      state.List_eligible.String(),
				Rating:    6000000,
			},
			{
				PublicKey: []byte("owner2"),
				List:      state.List_waiting.String(),
				Rating:    6000000,
			},
		}
		addPeersFromValidatorInfo(validators, v)

		nodeForcedToRemain, err := v.SaveUpdatesForNodesMap(nodeOwners, peerType)
		assert.NoError(t, err)
		assert.True(t, nodeForcedToRemain)

		// Check that peer accounts were updated correctly
		peerAcc1, _ := v.accountsCacher.GetExistingPeer(nodeOwners[0])
		assert.Equal(t, state.List_eligible, peerAcc1.GetList())

		peerAcc2, _ := v.accountsCacher.GetExistingPeer(nodeOwners[1])
		assert.Equal(t, state.List_eligible, peerAcc2.GetList())
	})

	t.Run("Node with low rating gets jailed", func(t *testing.T) {
		v := setupTest()

		nodeOwners := [][]byte{[]byte("owner1")}
		peerType := state.List_eligible.String()

		// Setup initial peer account with low rating
		peerAcc, _ := v.accountsCacher.LoadPeer(nodeOwners[0])
		peerAcc.SetTempRating(100000) // Low rating
		peerAcc.SetListAndIndex(state.List_eligible, 0)

		nodeForcedToRemain, err := v.SaveUpdatesForNodesMap(nodeOwners, peerType)

		assert.NoError(t, err)
		assert.False(t, nodeForcedToRemain)

		// Check that peer account was jailed
		peerAcc, _ = v.accountsCacher.GetExistingPeer(nodeOwners[0])
		assert.Equal(t, state.List_jailed, peerAcc.GetList())
	})

	t.Run("Error getting KApp", func(t *testing.T) {
		v := setupTest()
		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return nil, common.ErrNilTrie
		}

		nodeOwners := [][]byte{[]byte("owner1")}
		peerType := state.List_eligible.String()

		nodeForcedToRemain, err := v.SaveUpdatesForNodesMap(nodeOwners, peerType)

		assert.Error(t, err)
		assert.Equal(t, common.ErrNilTrie, err)
		assert.False(t, nodeForcedToRemain)
	})

	t.Run("Error getting validator", func(t *testing.T) {
		v := setupTest()
		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					return make([]byte, 0)
				},
			}, nil
		}

		nodeOwners := [][]byte{[]byte("owner1")}
		peerType := state.List_eligible.String()

		nodeForcedToRemain, err := v.SaveUpdatesForNodesMap(nodeOwners, peerType)

		assert.Error(t, err)
		assert.Equal(t, common.ErrValidatorNotFound, err)
		assert.False(t, nodeForcedToRemain)
	})

	t.Run("Different peer types", func(t *testing.T) {
		v := setupTest()

		nodeOwners := [][]byte{[]byte("owner1")}
		tests := []struct {
			peerType    state.List
			currentList state.List
			expected    bool
		}{
			{state.List_eligible, state.List_elected, false},
			{state.List_waiting, state.List_elected, false},
			{state.List_elected, state.List_elected, false},
			{state.List_leaving, state.List_elected, false},
			{state.List_eligible, state.List_waiting, true},
			{state.List_waiting, state.List_waiting, false},
			{state.List_elected, state.List_waiting, true},
			{state.List_leaving, state.List_waiting, false},
		}
		for _, test := range tests {
			// Setup initial peer account
			peerAcc, _ := v.accountsCacher.LoadPeer(nodeOwners[0])
			peerAcc.SetListAndIndex(test.currentList, 0)

			nodeForcedToRemain, err := v.SaveUpdatesForNodesMap(nodeOwners, test.peerType.String())

			assert.NoError(t, err)
			assert.Equal(t, test.expected, nodeForcedToRemain)

			// Check that peer account was updated correctly
			peerAcc, _ = v.accountsCacher.GetExistingPeer(nodeOwners[0])
			assert.Equal(t, test.peerType, peerAcc.GetList())
		}
	})
}

func TestDecreaseTempRating(t *testing.T) {
	t.Parallel()

	setupTest := func() *validatorsKApp {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					blsPubKey := strings.Replace(string(key), "VAL/", "", 1)
					data, _ := v.marshalizer.Marshal(&ValidatorData{BlsPubKey: []byte(blsPubKey)})
					return data
				},
			}, nil
		}

		// Setup mock rater
		v.rater = &mock.RaterMock{
			ComputeDecreaseProposerCalled: func(previous uint32, consecutiveMisses uint32) uint32 {
				return previous - 100 // Simple decrease for testing
			},
			ComputeDecreaseValidatorCalled: func(previous uint32) uint32 {
				return previous - 50 // Simple decrease for testing
			},
			GetChancesCalled: func(val uint32) uint32 {
				// mock results for jail validator
				if val == 0 {
					return 5
				}

				if val < 4000000 {
					return 0
				}

				return 10
			},
		}

		return v
	}

	t.Run("Decrease proposer rating", func(t *testing.T) {
		v := setupTest()
		validator := []byte("validator1")

		// Setup initial peer account
		peerAcc, _ := v.accountsCacher.LoadPeer(validator)
		peerAcc.SetTempRating(5000000)
		peerAcc.SetConsecutiveProposerMisses(0)

		err := v.DecreaseTempRating(validator, true)
		assert.NoError(t, err)

		updatedPeerAcc, _ := v.accountsCacher.GetExistingPeer(validator)
		assert.Equal(t, uint32(4999900), updatedPeerAcc.GetTempRating())
		assert.Equal(t, uint32(1), updatedPeerAcc.GetConsecutiveProposerMisses())
	})

	t.Run("Decrease validator rating", func(t *testing.T) {
		v := setupTest()
		validator := []byte("validator2")

		// Setup initial peer account
		peerAcc, _ := v.accountsCacher.LoadPeer(validator)
		peerAcc.SetTempRating(5000000)

		err := v.DecreaseTempRating(validator, false)
		assert.NoError(t, err)

		updatedPeerAcc, _ := v.accountsCacher.GetExistingPeer(validator)
		assert.Equal(t, uint32(4999950), updatedPeerAcc.GetTempRating())
	})

	t.Run("Jail validator due to low rating", func(t *testing.T) {
		v := setupTest()
		validator := []byte("validator3")

		// Setup initial peer account with low rating
		peerAcc, _ := v.accountsCacher.LoadPeer(validator)
		peerAcc.SetTempRating(1000050)
		peerAcc.SetListAndIndex(state.List_eligible, 0)

		err := v.DecreaseTempRating(validator, false)
		assert.NoError(t, err)

		updatedPeerAcc, _ := v.accountsCacher.GetExistingPeer(validator)
		assert.Equal(t, uint32(1000000), updatedPeerAcc.GetTempRating())
		assert.Equal(t, state.List_jailed, updatedPeerAcc.GetList())
	})

	t.Run("Error getting KApp", func(t *testing.T) {
		v := setupTest()

		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return nil, common.ErrNilTrie
		}

		err := v.DecreaseTempRating([]byte("validator"), true)
		assert.Error(t, err)
		assert.Equal(t, common.ErrNilTrie, err)
	})

	t.Run("Error getting peer account", func(t *testing.T) {
		v := setupTest()
		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					return make([]byte, 0)
				},
			}, nil
		}

		err := v.DecreaseTempRating([]byte("validator"), true)
		assert.Error(t, err)
		assert.Equal(t, common.ErrValidatorNotFound, err)
	})

	t.Run("Error updating peer", func(t *testing.T) {
		v := setupTest()
		validator := []byte("validator4")

		// Setup initial peer account
		peerAcc, _ := v.accountsCacher.LoadPeer(validator)
		peerAcc.SetTempRating(5000000)

		// Mock UpdatePeer to return an error
		v.accountsCacher.(*mock.AccountsCacherStub).GetExistingPeerCalled = func(address []byte) (state.PeerAccountHandler, error) {
			return peerAcc, nil
		}
		v.accountsCacher.(*mock.AccountsCacherStub).UpdatePeerCalled = func(account state.AccountHandler) error {
			return errors.New("update peer error")
		}

		err := v.DecreaseTempRating(validator, true)
		assert.Error(t, err)
		assert.Equal(t, "update peer error", err.Error())
	})

	t.Run("Multiple consecutive proposer misses", func(t *testing.T) {
		v := setupTest()
		validator := []byte("validator5")

		// Setup initial peer account
		peerAcc, _ := v.accountsCacher.LoadPeer(validator)
		peerAcc.SetTempRating(5000000)
		peerAcc.SetConsecutiveProposerMisses(2)
		require.NoError(t, v.accountsCacher.UpdatePeer(peerAcc))

		err := v.DecreaseTempRating(validator, true)
		assert.NoError(t, err)

		updatedPeerAcc, _ := v.accountsCacher.GetExistingPeer(validator)
		assert.Equal(t, uint32(4999900), updatedPeerAcc.GetTempRating())
		assert.Equal(t, uint32(3), updatedPeerAcc.GetConsecutiveProposerMisses())
	})
}

func TestUpdateValidatorInfoOnSuccessfulBlock(t *testing.T) {
	t.Parallel()

	setupTest := func() *validatorsKApp {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		v.KAppController = &stub.KAppControllerStub{
			GetProposalControllerCalled: func() kapps.ActiveProposalController {
				return &mock.ProposalControllerStub{
					GetParameterUintCalled: func(param kapps.EnumParameter) uint64 {
						if param == kapps.EnumParameter_LeaderValidatorRewardsPercentage {
							return 3000 // 30% for leader, 70% for validators
						}
						return 0
					},
				}
			},
		}

		// Setup mock rater
		v.rater = &mock.RaterMock{
			ComputeIncreaseProposerCalled: func(previous uint32) uint32 {
				return previous + 100
			},
			ComputeIncreaseValidatorCalled: func(previous uint32) uint32 {
				return previous + 50
			},
		}

		return v
	}

	createMockValidators := func(count int, v *validatorsKApp) []sharding.Validator {
		validators := make([]sharding.Validator, count)
		for i := 0; i < count; i++ {
			validators[i] = &mock.ValidatorMock{
				PubKeyCalled: func() []byte {
					return []byte(fmt.Sprintf("validator%d", i))
				},
			}

			// initiate peer account
			peerAcc, _ := v.accountsCacher.LoadPeer(validators[i].PubKey())
			peerAcc.SetTempRating(startRating)

		}
		return validators
	}

	t.Run("All validators signed", func(t *testing.T) {
		v := setupTest()
		validatorList := createMockValidators(4, v)
		signingBitmap := []byte{0b11111111} // All validators signed
		accumulatedFees := int64(1000)

		err := v.UpdateValidatorInfoOnSuccessfulBlock(validatorList, signingBitmap, accumulatedFees)
		assert.NoError(t, err)

		// Check leader
		leaderAcc, _ := v.accountsCacher.GetExistingPeer(validatorList[0].PubKey())
		assert.Equal(t, uint32(1), leaderAcc.GetLeaderSuccessRate().NumSuccess)
		assert.Equal(t, uint32(0), leaderAcc.GetConsecutiveProposerMisses())
		assert.Equal(t, int64(301), leaderAcc.GetAccumulatedFees()) // 30% of 1000 + 1(rounding)
		assert.Equal(t, uint32(5000101), leaderAcc.GetTempRating()) // Assuming initial rating of 5000001

		// Check validators
		for i := 1; i < len(validatorList); i++ {
			validatorAcc, _ := v.accountsCacher.GetExistingPeer(validatorList[i].PubKey())
			assert.Equal(t, uint32(1), validatorAcc.GetValidatorSuccessRate().NumSuccess)
			assert.Equal(t, int64(233), validatorAcc.GetAccumulatedFees()) // (70% of 1000) / 3
			assert.Equal(t, uint32(5000051), validatorAcc.GetTempRating()) // Assuming initial rating of 5000001
		}
	})

	t.Run("Some validators didn't sign", func(t *testing.T) {
		v := setupTest()
		validatorList := createMockValidators(4, v)
		signingBitmap := []byte{0b00001011} // Leader and two validators signed
		accumulatedFees := int64(1000)
		missedValidatorIndex := 2

		err := v.UpdateValidatorInfoOnSuccessfulBlock(validatorList, signingBitmap, accumulatedFees)
		assert.NoError(t, err)

		// Check leader
		leaderAcc, _ := v.accountsCacher.GetExistingPeer(validatorList[0].PubKey())
		assert.Equal(t, uint32(1), leaderAcc.GetLeaderSuccessRate().NumSuccess)
		assert.Equal(t, int64(300), leaderAcc.GetAccumulatedFees())

		// Check signed validators
		for i := 1; i < 4; i++ {
			validatorAcc, _ := v.accountsCacher.GetExistingPeer(validatorList[i].PubKey())
			if i == missedValidatorIndex {
				// Check unsigned validator
				assert.Equal(t, uint32(1), validatorAcc.GetValidatorIgnoredSignaturesRate())
				assert.Equal(t, int64(0), validatorAcc.GetAccumulatedFees())
			} else {
				assert.Equal(t, uint32(1), validatorAcc.GetValidatorSuccessRate().NumSuccess)
				assert.Equal(t, int64(350), validatorAcc.GetAccumulatedFees()) // (70% of 1000) / 2
			}
		}
	})

	t.Run("Only leader signed", func(t *testing.T) {
		v := setupTest()
		validatorList := createMockValidators(4, v)
		signingBitmap := []byte{0b00000001} // Only leader signed
		accumulatedFees := int64(1000)

		err := v.UpdateValidatorInfoOnSuccessfulBlock(validatorList, signingBitmap, accumulatedFees)
		assert.NoError(t, err)

		// Check leader
		leaderAcc, _ := v.accountsCacher.GetExistingPeer(validatorList[0].PubKey())
		assert.Equal(t, uint32(1), leaderAcc.GetLeaderSuccessRate().NumSuccess)
		assert.Equal(t, int64(1000), leaderAcc.GetAccumulatedFees()) // Leader gets all fees

		// Check validators
		for i := 1; i < len(validatorList); i++ {
			validatorAcc, _ := v.accountsCacher.GetExistingPeer(validatorList[i].PubKey())
			assert.Equal(t, uint32(1), validatorAcc.GetValidatorIgnoredSignaturesRate())
			assert.Equal(t, int64(0), validatorAcc.GetAccumulatedFees())
		}
	})

	t.Run("Error loading peer account", func(t *testing.T) {
		v := setupTest()
		validatorList := createMockValidators(1, v)
		signingBitmap := []byte{0b00000001}
		accumulatedFees := int64(1000)

		v.accountsCacher.(*mock.AccountsCacherStub).LoadPeerCalled = func(key []byte) (state.PeerAccountHandler, error) {
			return nil, common.ErrNilTrie
		}

		err := v.UpdateValidatorInfoOnSuccessfulBlock(validatorList, signingBitmap, accumulatedFees)
		assert.Equal(t, common.ErrNilTrie, err)
	})
}

func TestProcessEconomicsEndOfEpoch(t *testing.T) {
	t.Parallel()

	setupTest := func() *validatorsKApp {
		v := setupValidatorsKApp(t)
		addFunctionalCacher(t, v)

		// Setup mock KAppController
		mockProposalController := &mock.ProposalControllerStub{
			GetParameterIntCalled: func(param kapps.EnumParameter) int64 {
				switch param {
				case kapps.EnumParameter_MinSelfDelegatedAmount:
					return 100000
				case kapps.EnumParameter_MinTotalDelegatedAmount:
					return 500000
				default:
					return 0
				}
			},
		}
		v.KAppController = &stub.KAppControllerStub{
			GetProposalControllerCalled: func() kapps.ActiveProposalController {
				return mockProposalController
			},
		}

		return v
	}

	createMockValidatorInfo := func(ownerAddress []byte, blsPubKey []byte) *state.ValidatorInfo {
		return &state.ValidatorInfo{
			PublicKey:    blsPubKey,
			OwnerAddress: ownerAddress,
			List:         string(state.List_eligible),
			TempRating:   5000000,
		}
	}

	kappStorage := func(storage map[string]interface{}, v *validatorsKApp) {
		rawData := make(map[string][]byte)
		for key, value := range storage {
			data, _ := v.marshalizer.Marshal(value)
			rawData[key] = data
		}

		v.accountsCacher.(*mock.AccountsCacherStub).LoadKAppCalled = func(address []byte) (state.KAppAccountHandler, error) {
			return &mock.KAppAccountHandlerStub{
				GetStorageCalled: func(key []byte) []byte {
					return rawData[string(key)]
				},
				SetStorageCalled: func(key []byte, value []byte) error {
					rawData[string(key)] = value
					return nil
				},
			}, nil
		}

	}

	validatorAddress := []byte("validator1")
	blsPubKey := []byte("blspubkey1")
	storageData := map[string]interface{}{
		"VALB/validator1": &PeerData{
			Buckets: map[string]*PeerBucket{
				"bucket1": {
					DelegatedEpoch:   5,
					UndelegatedEpoch: math.MaxUint32,
					Value:            200000,
					Address:          validatorAddress,
				},
				"bucket2": {
					DelegatedEpoch:   8,
					UndelegatedEpoch: 11,
					Value:            100000,
					Address:          []byte("delegator1"),
				},
			},
		},
		"VAL/validator1": &ValidatorData{
			BlsPubKey:      blsPubKey,
			RewardsAddress: validatorAddress,
			Commission:     1000, // 10%
			SelfStake:      200000,
		},
	}

	t.Run("Happy path - single validator", func(t *testing.T) {
		v := setupTest()
		currentEpoch := uint32(10)
		validatorInfos := []*state.ValidatorInfo{
			createMockValidatorInfo(validatorAddress, blsPubKey),
		}

		kappStorage(storageData, v)
		// set accumulated fees
		peerAcc, _ := v.accountsCacher.LoadPeer(blsPubKey)
		peerAcc.AddToAccumulatedFees(1000000)

		err := v.ProcessEconomicsEndOfEpoch(currentEpoch, validatorInfos)
		assert.NoError(t, err)

		// Verify results
		kappHandler, err := v.getKApp()
		require.NoError(t, err)
		updatedValidator, _ := v.getValidator(kappHandler, validatorAddress)
		assert.True(t, updatedValidator.SelfStaked)
		assert.False(t, updatedValidator.Jailed)
		assert.Equal(t, int64(1000000), updatedValidator.TotalRewards)

		// Verify delegations
		userAcc, _ := v.accountsCacher.LoadUser(validatorAddress)
		// 2/3 of the rewards (self stake) + (1/3) 10% commission
		// 666666 - 33333 = 699999
		// validator gets roundup value after distributing rewards, total 700000
		assert.Equal(t, int64(700000), userAcc.GetAllowance())

		delegatorAcc, _ := v.accountsCacher.LoadUser([]byte("delegator1"))
		// 1/3 of the rewards - 10% commission
		// 333333 - 10% = 300000
		assert.Equal(t, int64(300000), delegatorAcc.GetAllowance())
	})

	t.Run("Validator gets jailed", func(t *testing.T) {
		v := setupTest()
		currentEpoch := uint32(10)

		validatorInfos := []*state.ValidatorInfo{
			createMockValidatorInfo(validatorAddress, blsPubKey),
		}

		kappStorage(storageData, v)
		// set accumulated fees
		peerAcc, _ := v.accountsCacher.LoadPeer(blsPubKey)
		peerAcc.AddToAccumulatedFees(1000000)
		peerAcc.SetList(state.List_jailed)

		err := v.ProcessEconomicsEndOfEpoch(currentEpoch, validatorInfos)
		assert.NoError(t, err)

		// Verify results
		kappHandler, err := v.getKApp()
		require.NoError(t, err)
		updatedValidator, err := v.getValidator(kappHandler, validatorAddress)
		require.NoError(t, err)
		assert.True(t, updatedValidator.Jailed)
		assert.Equal(t, currentEpoch, updatedValidator.JailedEpoch)
		assert.Equal(t, uint32(1), updatedValidator.NumJailed)
	})

	t.Run("Validator becomes inactive due to low self stake", func(t *testing.T) {
		v := setupTest()
		currentEpoch := uint32(10)

		validatorInfos := []*state.ValidatorInfo{
			createMockValidatorInfo(validatorAddress, blsPubKey),
		}
		kappStorage(storageData, v)

		// Setup mock validator with low self stake
		kapp, _ := v.accountsCacher.LoadKApp(nil)
		data, _ := v.marshalizer.Marshal(&ValidatorData{
			BlsPubKey:      blsPubKey,
			RewardsAddress: validatorAddress,
			Commission:     1000,  // 10%
			SelfStake:      50000, // Below minSelfDelegated
		})
		require.NoError(t, kapp.SetStorage([]byte("VAL/validator1"), data))

		err := v.ProcessEconomicsEndOfEpoch(currentEpoch, validatorInfos)
		assert.NoError(t, err)

		// Verify results
		updatedPeerAcc, _ := v.loadPeerAccount(blsPubKey)
		assert.Equal(t, state.List_inactive, updatedPeerAcc.GetList())
	})

	t.Run("Multiple validators with different statuses", func(t *testing.T) {
		v := setupTest()
		currentEpoch := uint32(10)

		validatorAddress1 := []byte("validator1")
		validatorAddress2 := []byte("validator2")
		blsPubKey1 := []byte("blspubkey1")
		blsPubKey2 := []byte("blspubkey2")

		validatorInfos := []*state.ValidatorInfo{
			createMockValidatorInfo(validatorAddress1, blsPubKey1),
			createMockValidatorInfo(validatorAddress2, blsPubKey2),
		}

		storageData := map[string]interface{}{
			"VALB/validator1": &PeerData{
				Buckets: map[string]*PeerBucket{
					"bucket1": {
						DelegatedEpoch:   5,
						UndelegatedEpoch: math.MaxUint32,
						Value:            200000,
						Address:          validatorAddress1,
					},
				},
			},
			"VAL/validator1": &ValidatorData{
				BlsPubKey:      blsPubKey1,
				RewardsAddress: validatorAddress1,
				Commission:     1000,
				SelfStake:      200000,
			},
			"VALB/validator2": &PeerData{
				Buckets: map[string]*PeerBucket{
					"bucket1": {
						DelegatedEpoch:   5,
						UndelegatedEpoch: math.MaxUint32,
						Value:            400000,
						Address:          validatorAddress2,
					},
				},
			},
			"VAL/validator2": &ValidatorData{
				BlsPubKey:      blsPubKey2,
				RewardsAddress: validatorAddress2,
				Commission:     2000,
				SelfStake:      400000,
			},
		}

		kappStorage(storageData, v)

		peerAcc1, _ := v.accountsCacher.LoadPeer(blsPubKey1)
		peerAcc1.AddToAccumulatedFees(1000000)
		peerAcc1.SetList(state.List_eligible)

		peerAcc2, _ := v.accountsCacher.LoadPeer(blsPubKey2)
		peerAcc2.AddToAccumulatedFees(2000000)
		peerAcc2.SetList(state.List_waiting)

		err := v.ProcessEconomicsEndOfEpoch(currentEpoch, validatorInfos)
		assert.NoError(t, err)

		// Verify results for validator1
		kappHandler, _ := v.getKApp()
		updatedValidator1, _ := v.getValidator(kappHandler, validatorAddress1)
		assert.True(t, updatedValidator1.SelfStaked)
		assert.False(t, updatedValidator1.Jailed)
		assert.Equal(t, int64(1000000), updatedValidator1.TotalRewards)

		// Verify results for validator2
		updatedValidator2, _ := v.getValidator(kappHandler, validatorAddress2)
		assert.True(t, updatedValidator2.SelfStaked)
		assert.False(t, updatedValidator2.Jailed)
		assert.Equal(t, int64(2000000), updatedValidator2.TotalRewards)
		assert.True(t, updatedValidator2.Waiting)

		updatedPeerAcc2, _ := v.loadPeerAccount(blsPubKey2)
		assert.Equal(t, state.List_waiting, updatedPeerAcc2.GetList())
	})

	t.Run("Delegation and undelegation in the same epoch", func(t *testing.T) {
		v := setupTest()
		currentEpoch := uint32(10)

		validatorAddress := []byte("validator1")
		blsPubKey := []byte("blspubkey1")

		validatorInfos := []*state.ValidatorInfo{
			createMockValidatorInfo(validatorAddress, blsPubKey),
		}

		storageData := map[string]interface{}{
			"VALB/validator1": &PeerData{
				Buckets: map[string]*PeerBucket{
					"bucket1": {
						DelegatedEpoch:   10,
						UndelegatedEpoch: 10,
						Value:            100000,
						Address:          []byte("delegator1"),
					},
					"bucket2": {
						DelegatedEpoch:   5,
						UndelegatedEpoch: math.MaxUint32,
						Value:            200000,
						Address:          validatorAddress,
					},
				},
			},
			"VAL/validator1": &ValidatorData{
				BlsPubKey:      blsPubKey,
				RewardsAddress: validatorAddress,
				Commission:     1000,
				SelfStake:      200000,
			},
		}

		kappStorage(storageData, v)

		peerAcc, _ := v.accountsCacher.LoadPeer(blsPubKey)
		peerAcc.AddToAccumulatedFees(1000000)

		err := v.ProcessEconomicsEndOfEpoch(currentEpoch, validatorInfos)
		assert.NoError(t, err)

		// Verify that the bucket delegated and undelegated in the same epoch is removed
		kappHandler, _ := v.getKApp()
		updatedBuckets, _ := v.getValidatorBuckets(kappHandler, validatorAddress)
		assert.Len(t, updatedBuckets.Buckets, 1)
		assert.NotContains(t, updatedBuckets.Buckets, "bucket1")
	})

	t.Run("Validator with zero accumulated fees", func(t *testing.T) {
		v := setupTest()
		currentEpoch := uint32(10)

		validatorAddress := []byte("validator1")
		blsPubKey := []byte("blspubkey1")

		validatorInfos := []*state.ValidatorInfo{
			createMockValidatorInfo(validatorAddress, blsPubKey),
		}

		storageData := map[string]interface{}{
			"VALB/validator1": &PeerData{
				Buckets: map[string]*PeerBucket{
					"bucket1": {
						DelegatedEpoch:   5,
						UndelegatedEpoch: math.MaxUint32,
						Value:            200000,
						Address:          validatorAddress,
					},
				},
			},
			"VAL/validator1": &ValidatorData{
				BlsPubKey:      blsPubKey,
				RewardsAddress: validatorAddress,
				Commission:     1000,
				SelfStake:      200000,
			},
		}

		kappStorage(storageData, v)

		err := v.ProcessEconomicsEndOfEpoch(currentEpoch, validatorInfos)
		assert.NoError(t, err)

		// Verify that no rewards are distributed
		userAcc, _ := v.accountsCacher.LoadUser(validatorAddress)
		assert.Equal(t, int64(0), userAcc.GetAllowance())
	})

	t.Run("Validator with maximum commission", func(t *testing.T) {
		v := setupTest()
		currentEpoch := uint32(10)

		validatorAddress := []byte("validator1")
		blsPubKey := []byte("blspubkey1")

		validatorInfos := []*state.ValidatorInfo{
			createMockValidatorInfo(validatorAddress, blsPubKey),
		}

		storageData := map[string]interface{}{
			"VALB/validator1": &PeerData{
				Buckets: map[string]*PeerBucket{
					"bucket1": {
						DelegatedEpoch:   5,
						UndelegatedEpoch: math.MaxUint32,
						Value:            200000,
						Address:          validatorAddress,
					},
					"bucket2": {
						DelegatedEpoch:   5,
						UndelegatedEpoch: math.MaxUint32,
						Value:            100000,
						Address:          []byte("delegator1"),
					},
				},
			},
			"VAL/validator1": &ValidatorData{
				BlsPubKey:      blsPubKey,
				RewardsAddress: validatorAddress,
				Commission:     10000, // 100% commission
				SelfStake:      200000,
			},
		}

		kappStorage(storageData, v)

		peerAcc, _ := v.accountsCacher.LoadPeer(blsPubKey)
		peerAcc.AddToAccumulatedFees(1000000)

		err := v.ProcessEconomicsEndOfEpoch(currentEpoch, validatorInfos)
		assert.NoError(t, err)

		// Verify that all rewards go to the validator
		userAcc, _ := v.accountsCacher.LoadUser(validatorAddress)
		assert.Equal(t, int64(1000000), userAcc.GetAllowance())

		delegatorAcc, _ := v.accountsCacher.LoadUser([]byte("delegator1"))
		assert.Equal(t, int64(0), delegatorAcc.GetAllowance())
	})

	t.Run("Validator becomes eligible", func(t *testing.T) {
		v := setupTest()
		currentEpoch := uint32(10)

		validatorAddress := []byte("validator1")
		blsPubKey := []byte("blspubkey1")

		validatorInfos := []*state.ValidatorInfo{
			createMockValidatorInfo(validatorAddress, blsPubKey),
		}

		storageData := map[string]interface{}{
			"VALB/validator1": &PeerData{
				Buckets: map[string]*PeerBucket{
					"bucket1": {
						DelegatedEpoch:   5,
						UndelegatedEpoch: math.MaxUint32,
						Value:            600000, // Above minTotalDelegatedAmount
						Address:          validatorAddress,
					},
				},
			},
			"VAL/validator1": &ValidatorData{
				BlsPubKey:      blsPubKey,
				RewardsAddress: validatorAddress,
				Commission:     1000,
				SelfStake:      600000, // Above minSelfDelegatedAmount
			},
		}

		kappStorage(storageData, v)

		peerAcc, _ := v.accountsCacher.LoadPeer(blsPubKey)
		peerAcc.SetList(state.List_waiting)

		err := v.ProcessEconomicsEndOfEpoch(currentEpoch, validatorInfos)
		assert.NoError(t, err)

		// Verify that the validator becomes eligible
		updatedPeerAcc, _ := v.loadPeerAccount(blsPubKey)
		assert.Equal(t, state.List_eligible, updatedPeerAcc.GetList())
	})

	t.Run("Remaining fees check fork", func(t *testing.T) {
		storageData := map[string]interface{}{
			"VALB/validator1": &PeerData{
				Buckets: map[string]*PeerBucket{
					"bucket1": {
						DelegatedEpoch:   5,
						UndelegatedEpoch: math.MaxUint32,
						Value:            200000,
						Address:          validatorAddress,
					},
					"bucket2": {
						DelegatedEpoch:   5,
						UndelegatedEpoch: math.MaxUint32,
						Value:            100000,
						Address:          []byte("delegator1"),
					},
				},
			},
			"VAL/validator1": &ValidatorData{
				BlsPubKey:      blsPubKey,
				RewardsAddress: validatorAddress,
				Commission:     1000, // 10%
				SelfStake:      200000,
			},
		}

		tests := []struct {
			enableSmartContracts bool
			value                int64
		}{
			{true, 700001},
			{false, 700000},
		}

		for _, test := range tests {
			v := setupTest()
			currentEpoch := uint32(10)
			validatorInfos := []*state.ValidatorInfo{
				createMockValidatorInfo(validatorAddress, blsPubKey),
			}

			kappStorage(storageData, v)

			peerAcc, _ := v.accountsCacher.LoadPeer(blsPubKey)
			peerAcc.AddToAccumulatedFees(1000001) // Odd number to ensure remaining fees

			// Enable smart contracts
			v.forkController.(*mock.ForkControllerStub).EnableSmartContractsValue = test.enableSmartContracts

			err := v.ProcessEconomicsEndOfEpoch(currentEpoch, validatorInfos)
			assert.NoError(t, err)

			// Verify that remaining fee is added to validator's rewards
			userAcc, _ := v.accountsCacher.LoadUser(validatorAddress)
			// 200000/300000 * 900000 (90% of rewards) + 100000 (10% commission) + 1 (remaining fee) = 700001
			assert.Equal(t, test.value, userAcc.GetAllowance())
		}
	})

	t.Run("Jailed validator status update", func(t *testing.T) {
		v := setupTest()
		currentEpoch := uint32(10)

		validatorInfos := []*state.ValidatorInfo{
			createMockValidatorInfo(validatorAddress, blsPubKey),
		}

		storageData := map[string]interface{}{
			"VALB/validator1": &PeerData{
				Buckets: map[string]*PeerBucket{
					"bucket1": {
						DelegatedEpoch:   5,
						UndelegatedEpoch: math.MaxUint32,
						Value:            200000,
						Address:          validatorAddress,
					},
				},
			},
			"VAL/validator1": &ValidatorData{
				BlsPubKey:      blsPubKey,
				RewardsAddress: validatorAddress,
				Commission:     1000,
				SelfStake:      200000,
				Jailed:         true,
				JailedEpoch:    5,
			},
		}

		kappStorage(storageData, v)

		peerAcc, _ := v.accountsCacher.LoadPeer(blsPubKey)
		peerAcc.SetList(state.List_eligible) // Peer account is not jailed

		err := v.ProcessEconomicsEndOfEpoch(currentEpoch, validatorInfos)
		assert.NoError(t, err)

		// Verify that the validator is no longer jailed
		kappHandler, _ := v.getKApp()
		updatedValidator, _ := v.getValidator(kappHandler, validatorAddress)
		assert.False(t, updatedValidator.Jailed)
		assert.Equal(t, uint32(math.MaxUint32), updatedValidator.JailedEpoch)
	})

	t.Run("Validator remains jailed", func(t *testing.T) {
		v := setupTest()
		currentEpoch := uint32(10)

		validatorAddress := []byte("validator1")
		blsPubKey := []byte("blspubkey1")

		validatorInfos := []*state.ValidatorInfo{
			createMockValidatorInfo(validatorAddress, blsPubKey),
		}

		storageData := map[string]interface{}{
			"VALB/validator1": &PeerData{
				Buckets: map[string]*PeerBucket{
					"bucket1": {
						DelegatedEpoch:   5,
						UndelegatedEpoch: math.MaxUint32,
						Value:            200000,
						Address:          validatorAddress,
					},
				},
			},
			"VAL/validator1": &ValidatorData{
				BlsPubKey:      blsPubKey,
				RewardsAddress: validatorAddress,
				Commission:     1000,
				SelfStake:      200000,
				Jailed:         true,
				JailedEpoch:    5,
			},
		}

		kappStorage(storageData, v)

		peerAcc, _ := v.accountsCacher.LoadPeer(blsPubKey)
		peerAcc.SetList(state.List_jailed) // Peer account is jailed

		err := v.ProcessEconomicsEndOfEpoch(currentEpoch, validatorInfos)
		assert.NoError(t, err)

		// Verify that the validator remains jailed
		kappHandler, _ := v.getKApp()
		updatedValidator, _ := v.getValidator(kappHandler, validatorAddress)
		assert.True(t, updatedValidator.Jailed)
		assert.Equal(t, uint32(5), updatedValidator.JailedEpoch)
	})

	t.Run("Remove undelegated buckets and calculate total undelegated amount", func(t *testing.T) {
		v := setupTest()
		currentEpoch := uint32(10)

		validatorInfos := []*state.ValidatorInfo{
			createMockValidatorInfo(validatorAddress, blsPubKey),
		}

		storageData := map[string]interface{}{
			"VALB/validator1": &PeerData{
				Buckets: map[string]*PeerBucket{
					"bucket1": {
						DelegatedEpoch:   5,
						UndelegatedEpoch: 9, // Undelegated in previous epoch
						Value:            100000,
						Address:          []byte("delegator1"),
					},
					"bucket2": {
						DelegatedEpoch:   5,
						UndelegatedEpoch: 10, // Undelegated in current epoch
						Value:            200000,
						Address:          []byte("delegator2"),
					},
					"bucket3": {
						DelegatedEpoch:   5,
						UndelegatedEpoch: math.MaxUint32, // Still delegated
						Value:            300000,
						Address:          validatorAddress,
					},
				},
			},
			"VAL/validator1": &ValidatorData{
				BlsPubKey:      blsPubKey,
				RewardsAddress: validatorAddress,
				Commission:     1000,
				SelfStake:      300000,
			},
		}

		kappStorage(storageData, v)

		peerAcc, _ := v.accountsCacher.LoadPeer(blsPubKey)
		peerAcc.AddToAccumulatedFees(1000000)

		err := v.ProcessEconomicsEndOfEpoch(currentEpoch, validatorInfos)
		assert.NoError(t, err)

		// Verify that undelegated buckets are removed
		kappHandler, _ := v.getKApp()
		updatedBuckets, _ := v.getValidatorBuckets(kappHandler, validatorAddress)
		assert.Len(t, updatedBuckets.Buckets, 1)
		assert.NotContains(t, updatedBuckets.Buckets, "bucket1")
		assert.NotContains(t, updatedBuckets.Buckets, "bucket2")
		assert.Contains(t, updatedBuckets.Buckets, "bucket3")

		// Verify rewards distribution
		validatorAcc, _ := v.accountsCacher.LoadUser(validatorAddress)
		// Validator gets all rewards as it's the only remaining delegation
		assert.Equal(t, int64(1000000), validatorAcc.GetAllowance())

		delegator1Acc, _ := v.accountsCacher.LoadUser([]byte("delegator1"))
		assert.Equal(t, int64(0), delegator1Acc.GetAllowance())

		delegator2Acc, _ := v.accountsCacher.LoadUser([]byte("delegator2"))
		assert.Equal(t, int64(0), delegator2Acc.GetAllowance())
	})
}
