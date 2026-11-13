package sharding

import (
	"crypto/rand"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	elected  = 100
	eligible = 30
)

func generateRandomByteArray(size int) []byte {
	r := make([]byte, size)
	_, _ = rand.Read(r)

	return r
}

func generateValidatorList(number int) []Validator {
	validators := make([]Validator, 0)

	for i := 0; i < number; i++ {
		var validator = &validator{
			pubKey: generateRandomByteArray(32),
		}

		validators = append(validators, validator)
	}

	return validators
}

func contains(a []Validator, b []Validator) bool {
	var found bool
	for _, va := range a {
		found = false
		for _, vb := range b {
			if reflect.DeepEqual(va, vb) {
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

// elected 40: 0 in elected, 10 eligible
func Test_promoteEligibleToElected_ZeroElected(t *testing.T) {
	t.Parallel()

	numNodes := uint32(40)
	numEligible := 10

	electedList := generateValidatorList(0)
	eligibleList := generateValidatorList(numEligible)

	finalElectedList, finalEligibleList, err := moveMaxNumNodesToList(electedList, eligibleList, numNodes)
	assert.Nil(t, err)

	assert.Equal(t, append(electedList, eligibleList[0:numEligible]...), finalElectedList)
	assert.Empty(t, finalEligibleList)
}

// elected 40: 30 in elected, 6 eligible
func Test_promoteEligibleToElected_LessEligibleThanRemainingSize(t *testing.T) {
	t.Parallel()

	numNodes := uint32(40)
	numEligible := 6

	electedList := generateValidatorList(30)
	eligibleList := generateValidatorList(numEligible)

	finalElectedList, finalEligibleList, err := moveMaxNumNodesToList(electedList, eligibleList, numNodes)
	assert.Nil(t, err)

	assert.Equal(t, append(electedList, eligibleList[0:numEligible]...), finalElectedList)
	assert.Empty(t, finalEligibleList)
}

// elected 40: 30 in elected, 10 eligible
func Test_promoteEligibleToElected_ExactlyEligibleThanRemainingSize(t *testing.T) {
	t.Parallel()

	numNodes := uint32(40)
	numEligible := 10

	electedList := generateValidatorList(30)
	eligibleList := generateValidatorList(numEligible)

	finalElectedList, finalEligibleList, err := moveMaxNumNodesToList(electedList, eligibleList, numNodes)
	assert.Nil(t, err)

	assert.Equal(t, finalElectedList, append(electedList, eligibleList...))
	assert.Empty(t, finalEligibleList)
}

// elected 40: 30 in elected, 15 eligible
func Test_promoteEligibleToElected_MoreEligibleThanRemainingSize(t *testing.T) {
	t.Parallel()

	numNodes := uint32(40)
	numEligible := 10
	numRemaining := 10

	electedList := generateValidatorList(30)
	eligibleList := generateValidatorList(numEligible)

	finalElectedList, finalEligibleList, err := moveMaxNumNodesToList(electedList, eligibleList, numNodes)
	assert.Nil(t, err)

	assert.Equal(t, append(electedList, eligibleList[0:numRemaining]...), finalElectedList)
	assert.Equal(t, eligibleList[numRemaining:], finalEligibleList)
}

func Test_promoteEligibleToElected(t *testing.T) {
	t.Parallel()

	electedList := generateValidatorList(30)
	eligibleList := generateValidatorList(22)

	finalElectedList, finalEligibleList, err := moveNodesToList(electedList, eligibleList)
	assert.Nil(t, err)

	assert.Equal(t, finalElectedList, append(electedList, eligibleList...))
	assert.Empty(t, finalEligibleList)
}

func Test_promoteEligibleToNilElected(t *testing.T) {
	t.Parallel()

	eligibleList := generateValidatorList(22)

	_, _, err := moveNodesToList(nil, eligibleList)

	assert.Equal(t, ErrNilOrEmptyDestinationForDistribute, err)
}

func testRemoveValidators(
	t *testing.T,
	initialValidators []Validator,
	validatorsToRemove []Validator,
	remaining []Validator,
	removed []Validator,
	maxToRemove int,
) {
	nbRemoved := maxToRemove
	if nbRemoved > len(validatorsToRemove) {
		nbRemoved = len(validatorsToRemove)
	}

	assert.Equal(t, nbRemoved, len(removed))
	assert.Equal(t, len(initialValidators)-len(remaining), nbRemoved)

	all := append(remaining, removed...)
	assert.True(t, contains(all, initialValidators))
	assert.Equal(t, len(initialValidators), len(all))
}

func testShuffledOut(
	t *testing.T,
	electedMap []Validator,
	eligibleMap []Validator,
	newEligible []Validator,
	shuffledOut []Validator,
) {
	assert.Equal(t, len(electedMap)-len(shuffledOut), len(newEligible))

	newNodes := append(newEligible, shuffledOut...)
	assert.NotEqual(t, electedMap, newNodes)
	assert.True(t, contains(newNodes, electedMap))
}

func Test_removeValidatorFromListFirst(t *testing.T) {
	t.Parallel()

	validators := generateValidatorList(30)
	validatorsCopy := make([]Validator, len(validators))
	_ = copy(validatorsCopy, validators)

	v := removeValidatorFromList(validators, 0)
	assert.Equal(t, validatorsCopy[len(validatorsCopy)-1], v[0])
	assert.NotEqual(t, validatorsCopy[0], v[0])
	assert.Equal(t, len(validatorsCopy)-1, len(v))

	for i := 1; i < len(v); i++ {
		assert.Equal(t, validatorsCopy[i], v[i])
	}
}

func Test_removeValidatorFromListLast(t *testing.T) {
	t.Parallel()

	validators := generateValidatorList(30)
	validatorsCopy := make([]Validator, len(validators))
	_ = copy(validatorsCopy, validators)

	v := removeValidatorFromList(validators, len(validators)-1)
	assert.Equal(t, len(validatorsCopy)-1, len(v))
	assert.Equal(t, validatorsCopy[:len(validatorsCopy)-1], v)
}

func Test_removeValidatorFromListMiddle(t *testing.T) {
	t.Parallel()

	validators := generateValidatorList(30)
	validatorsCopy := make([]Validator, len(validators))
	_ = copy(validatorsCopy, validators)

	v := removeValidatorFromList(validators, len(validators)/2)
	assert.Equal(t, len(validatorsCopy)-1, len(v))
	assert.Equal(t, validatorsCopy[len(validatorsCopy)-1], v[len(validatorsCopy)/2])
}

func Test_removeValidatorFromListIndexNegativeNoAction(t *testing.T) {
	t.Parallel()

	validators := generateValidatorList(30)
	validatorsCopy := make([]Validator, len(validators))
	_ = copy(validatorsCopy, validators)

	v := removeValidatorFromList(validators, -1)
	assert.Equal(t, len(validatorsCopy), len(v))
	assert.Equal(t, validatorsCopy, v)
}

func Test_removeValidatorFromListIndexTooBigNoAction(t *testing.T) {
	t.Parallel()

	validators := generateValidatorList(30)
	validatorsCopy := make([]Validator, len(validators))
	_ = copy(validatorsCopy, validators)

	v := removeValidatorFromList(validators, len(validators))
	assert.Equal(t, len(validatorsCopy), len(v))
	assert.Equal(t, validatorsCopy, v)
}

func Test_removeValidatorsFromListRemoveFromStart(t *testing.T) {
	t.Parallel()

	validatorsToRemoveFromStart := 3
	validators := generateValidatorList(30)
	validatorsCopy := make([]Validator, len(validators))
	validatorsToRemove := make([]Validator, 0)

	_ = copy(validatorsCopy, validators)
	validatorsToRemove = append(validatorsToRemove, validators[:validatorsToRemoveFromStart]...)

	v, removed := removeValidatorsFromList(validators, validatorsToRemove, len(validatorsToRemove))
	testRemoveValidators(t, validatorsCopy, validatorsToRemove, v, removed, len(validatorsToRemove))
}

func Test_removeValidatorsFromListRemoveFromLast(t *testing.T) {
	t.Parallel()

	validatorsToRemoveFromEnd := 3
	validators := generateValidatorList(30)
	validatorsCopy := make([]Validator, len(validators))
	validatorsToRemove := make([]Validator, 0)

	_ = copy(validatorsCopy, validators)
	validatorsToRemove = append(validatorsToRemove, validators[len(validators)-validatorsToRemoveFromEnd:]...)

	v, removed := removeValidatorsFromList(validators, validatorsToRemove, len(validatorsToRemove))
	testRemoveValidators(t, validatorsCopy, validatorsToRemove, v, removed, len(validatorsToRemove))
}

func Test_removeValidatorsFromListRemoveFromFirstMaxSmaller(t *testing.T) {
	t.Parallel()

	validatorsToRemoveFromStart := 3
	validators := generateValidatorList(30)
	validatorsCopy := make([]Validator, len(validators))
	validatorsToRemove := make([]Validator, 0)
	maxToRemove := validatorsToRemoveFromStart - 1

	_ = copy(validatorsCopy, validators)
	validatorsToRemove = append(validatorsToRemove, validators[:validatorsToRemoveFromStart]...)

	v, removed := removeValidatorsFromList(validators, validatorsToRemove, maxToRemove)
	testRemoveValidators(t, validatorsCopy, validatorsToRemove, v, removed, maxToRemove)
}

func Test_removeValidatorsFromListRemoveFromFirstMaxGreater(t *testing.T) {
	t.Parallel()

	validatorsToRemoveFromStart := 3
	validators := generateValidatorList(30)
	validatorsCopy := make([]Validator, len(validators))
	validatorsToRemove := make([]Validator, 0)
	maxToRemove := validatorsToRemoveFromStart + 1

	_ = copy(validatorsCopy, validators)
	validatorsToRemove = append(validatorsToRemove, validators[:validatorsToRemoveFromStart]...)

	v, removed := removeValidatorsFromList(validators, validatorsToRemove, maxToRemove)
	testRemoveValidators(t, validatorsCopy, validatorsToRemove, v, removed, maxToRemove)
}

func Test_removeValidatorsFromListRemoveFromLastMaxSmaller(t *testing.T) {
	t.Parallel()

	validatorsToRemoveFromEnd := 3
	validators := generateValidatorList(30)
	validatorsCopy := make([]Validator, len(validators))
	validatorsToRemove := make([]Validator, 0)
	maxToRemove := validatorsToRemoveFromEnd - 1

	_ = copy(validatorsCopy, validators)
	validatorsToRemove = append(validatorsToRemove, validators[len(validators)-validatorsToRemoveFromEnd:]...)
	assert.Equal(t, validatorsToRemoveFromEnd, len(validatorsToRemove))

	v, removed := removeValidatorsFromList(validators, validatorsToRemove, maxToRemove)
	testRemoveValidators(t, validatorsCopy, validatorsToRemove, v, removed, maxToRemove)
}

func Test_removeValidatorsFromListRemoveFromLastMaxGreater(t *testing.T) {
	t.Parallel()

	validatorsToRemoveFromEnd := 3
	validators := generateValidatorList(30)
	validatorsCopy := make([]Validator, len(validators))
	validatorsToRemove := make([]Validator, 0)
	maxToRemove := validatorsToRemoveFromEnd + 1

	_ = copy(validatorsCopy, validators)
	validatorsToRemove = append(validatorsToRemove, validators[len(validators)-validatorsToRemoveFromEnd:]...)
	assert.Equal(t, validatorsToRemoveFromEnd, len(validatorsToRemove))

	v, removed := removeValidatorsFromList(validators, validatorsToRemove, maxToRemove)
	testRemoveValidators(t, validatorsCopy, validatorsToRemove, v, removed, maxToRemove)
}

func Test_removeValidatorsFromListRandomValidatorsMaxSmaller(t *testing.T) {
	t.Parallel()

	nbValidatotrsToRemove := 10
	maxToRemove := nbValidatotrsToRemove - 3
	validators := generateValidatorList(30)
	validatorsCopy := make([]Validator, len(validators))
	validatorsToRemove := make([]Validator, 0)

	_ = copy(validatorsCopy, validators)

	sort.Sort(validatorList(validators))

	validatorsToRemove = append(validatorsToRemove, validators[:nbValidatotrsToRemove]...)

	v, removed := removeValidatorsFromList(validators, validatorsToRemove, maxToRemove)
	testRemoveValidators(t, validatorsCopy, validatorsToRemove, v, removed, maxToRemove)
}

func Test_removeValidatorsFromListRandomValidatorsMaxGreater(t *testing.T) {
	t.Parallel()

	nbValidatotrsToRemove := 10
	maxToRemove := nbValidatotrsToRemove + 3
	validators := generateValidatorList(30)
	validatorsCopy := make([]Validator, len(validators))
	validatorsToRemove := make([]Validator, 0)

	_ = copy(validatorsCopy, validators)

	sort.Sort(validatorList(validators))

	validatorsToRemove = append(validatorsToRemove, validators[:nbValidatotrsToRemove]...)

	v, removed := removeValidatorsFromList(validators, validatorsToRemove, maxToRemove)
	testRemoveValidators(t, validatorsCopy, validatorsToRemove, v, removed, maxToRemove)
}

func Test_removeDupplicates_NoDupplicates(t *testing.T) {
	t.Parallel()

	firstList := generateValidatorList(30)
	secondList := generateValidatorList(30)

	firstListCopy := make([]Validator, len(firstList))
	copy(firstListCopy, firstList)

	secondListCopy := make([]Validator, len(secondList))
	copy(secondListCopy, secondList)

	assert.Equal(t, firstListCopy, firstList)
}

func Test_removeDupplicates_SomeDupplicates(t *testing.T) {
	t.Parallel()

	firstList := generateValidatorList(30)
	secondList := generateValidatorList(20)
	validatorsFromFirstList := firstList[0:10]
	secondList = append(secondList, validatorsFromFirstList...)

	firstListCopy := make([]Validator, len(firstList))
	copy(firstListCopy, firstList)

	secondListCopy := make([]Validator, len(secondList))
	copy(secondListCopy, secondList)

	assert.Equal(t, firstListCopy, firstList)

}

func Test_removeDupplicates_AllDupplicates(t *testing.T) {
	t.Parallel()

	firstList := generateValidatorList(30)
	secondList := make([]Validator, len(firstList))
	copy(secondList, firstList)

	firstListCopy := make([]Validator, len(firstList))
	copy(firstListCopy, firstList)
	secondListCopy := make([]Validator, len(secondList))
	copy(secondListCopy, secondList)

	assert.Equal(t, firstListCopy, firstList)
}

func Test_shuffleList(t *testing.T) {
	t.Parallel()

	randomness := generateRandomByteArray(32)
	validators := generateValidatorList(30)
	validatorsCopy := make([]Validator, 0)
	validatorsCopy = append(validatorsCopy, validators...)

	shuffled := shuffleList(validators, randomness)
	assert.Equal(t, len(validatorsCopy), len(shuffled))
	assert.NotEqual(t, validatorsCopy, shuffled)
	assert.True(t, contains(shuffled, validatorsCopy))
}

func Test_shuffleListParameterNotChanged(t *testing.T) {
	t.Parallel()

	randomness := generateRandomByteArray(32)
	validators := generateValidatorList(30)
	validatorsCopy := make([]Validator, len(validators))
	_ = copy(validatorsCopy, validators)

	_ = shuffleList(validators, randomness)
	assert.Equal(t, validatorsCopy, validators)
}

func Test_shuffleListConsistentShuffling(t *testing.T) {
	t.Parallel()

	randomness := generateRandomByteArray(32)
	validators := generateValidatorList(30)

	nbTrials := 10
	shuffled := shuffleList(validators, randomness)
	for i := 0; i < nbTrials; i++ {
		shuffled2 := shuffleList(validators, randomness)
		assert.Equal(t, shuffled, shuffled2)
	}
}

func Test_shuffleOutNodesNoLeaving(t *testing.T) {
	t.Parallel()

	randomness := generateRandomByteArray(32)
	electedMap := generateValidatorList(30)
	eligibleMap := generateValidatorList(100)

	numToRemove := 1

	shuffledOut, newElected := shuffleOutNodes(electedMap, numToRemove, randomness)

	testShuffledOut(t, electedMap, eligibleMap, newElected, shuffledOut)
}

func TestNewHashValidatorsShuffler(t *testing.T) {
	t.Parallel()

	shufflerArgs := &NodesShufflerArgs{
		Nodes:                elected,
		MaxNodesEnableConfig: nil,
		ForkController:       &forkControllerStub{},
	}
	shuffler, err := NewHashValidatorsShuffler(shufflerArgs)
	assert.Nil(t, err)
	assert.NotNil(t, shuffler)
}

func TestNewHashValidatorsShuffler_NilForkControllerShouldErr(t *testing.T) {
	t.Parallel()

	shufflerArgs := &NodesShufflerArgs{
		Nodes:                elected,
		MaxNodesEnableConfig: nil,
	}
	shuffler, err := NewHashValidatorsShuffler(shufflerArgs)
	assert.Equal(t, ErrNilForkController, err)
	assert.True(t, check.IfNil(shuffler))
}

func TestRandHashShuffler_UpdateNodeLists(t *testing.T) {
	t.Parallel()

	shufflerArgs := &NodesShufflerArgs{
		Nodes:                10,
		MaxNodesEnableConfig: nil,
		ForkController:       &forkControllerStub{},
	}

	shuffler, err := NewHashValidatorsShuffler(shufflerArgs)
	require.Nil(t, err)

	randomness := generateRandomByteArray(32)

	electedList := generateValidatorList(20)
	eligibleList := generateValidatorList(5)

	args := ArgsUpdateNodes{
		Elected:  electedList,
		Eligible: eligibleList,
		Rand:     randomness,
	}

	resUpdateNodeList, err := shuffler.UpdateNodeLists(args)
	require.Nil(t, err)

	fmt.Println(len(resUpdateNodeList.Elected), len(resUpdateNodeList.Eligible))
	assert.NotEqual(t, electedList, resUpdateNodeList.Elected)
	assert.Equal(t, len(electedList)+len(eligibleList), len(resUpdateNodeList.Elected)+len(resUpdateNodeList.Eligible))
}

func TestRandHashShuffler_ShuffleOutNodes_WithMoreThanValidatorsShuffledOut(t *testing.T) {
	t.Parallel()

	validatorsNumber := 10
	numToShuffle := 10
	randomness := generateRandomByteArray(32)

	validators := generateValidatorList(validatorsNumber)

	shuffledOut, remaining := shuffleOutNodes(validators, numToShuffle, randomness)

	assert.Equal(t, validatorsNumber, len(shuffledOut))
	assert.Equal(t, 0, len(remaining))
}

func TestRandHashShuffler_ShuffleOutNodes_WithLessThanValidatorsShuffledOut(t *testing.T) {
	t.Parallel()

	validatorsNumber := 10
	numToShuffle := 6
	randomness := generateRandomByteArray(32)

	validators := generateValidatorList(validatorsNumber)

	shuffledOut, remaining := shuffleOutNodes(validators, numToShuffle, randomness)

	assert.Equal(t, numToShuffle, len(shuffledOut))
	assert.Equal(t, validatorsNumber-numToShuffle, len(remaining))
}

func TestRandHashShuffler_ShuffleOutNodes_WithZeroValidatorsShuffledOut(t *testing.T) {
	t.Parallel()

	validatorsNumber := 10
	numToShuffle := 0
	randomness := generateRandomByteArray(32)

	validators := generateValidatorList(validatorsNumber)

	shuffledOut, remaining := shuffleOutNodes(validators, numToShuffle, randomness)

	assert.Equal(t, 0, len(shuffledOut))
	assert.Equal(t, validatorsNumber, len(remaining))
}

type forkControllerStub struct {
	core.ForkController
	FixAuditChangesV7Value bool
	FixAuditChangesV7Epoch *uint32
}

func (f *forkControllerStub) FixAuditChangesV7InEpoch(epoch uint32) bool {
	if f.FixAuditChangesV7Epoch == nil {
		return f.FixAuditChangesV7Value
	}

	return epoch >= *f.FixAuditChangesV7Epoch
}

func (f *forkControllerStub) IsInterfaceNil() bool {
	return f == nil
}

func createShufflerWithNodesToShuffle(t *testing.T, nodes uint32, nodesToShuffle uint32) *randHashShuffler {
	return createShufflerWithForkController(t, nodes, nodesToShuffle, &forkControllerStub{})
}

func createShufflerWithForkController(
	t *testing.T,
	nodes uint32,
	nodesToShuffle uint32,
	forkController core.ForkController,
) *randHashShuffler {
	shufflerArgs := &NodesShufflerArgs{
		Nodes: nodes,
		MaxNodesEnableConfig: []config.MaxNodesChangeConfig{
			{EpochEnable: 0, NodesToShuffle: nodesToShuffle},
		},
		ForkController: forkController,
	}

	shuffler, err := NewHashValidatorsShuffler(shufflerArgs)
	require.Nil(t, err)

	return shuffler
}

func countPubKeys(lists ...[]Validator) map[string]int {
	counts := make(map[string]int)
	for _, list := range lists {
		for _, v := range list {
			counts[string(v.PubKey())]++
		}
	}

	return counts
}

func TestRandHashShuffler_UpdateNodeLists_ElectedSmallerThanNodesToShuffleKeepsSurvivors(t *testing.T) {
	t.Parallel()

	for _, numElected := range []int{0, 1, 2, 3} {
		shuffler := createShufflerWithNodesToShuffle(t, 21, 4)

		electedList := generateValidatorList(numElected)
		eligibleList := generateValidatorList(30)

		args := ArgsUpdateNodes{
			Elected:  electedList,
			Eligible: eligibleList,
			Rand:     generateRandomByteArray(32),
		}

		var res *ResUpdateNodes
		var err error
		require.NotPanics(t, func() {
			res, err = shuffler.UpdateNodeLists(args)
		})
		require.Nil(t, err)

		assert.Equal(t, 21, len(res.Elected))
		assert.Equal(t, numElected+30, len(res.Elected)+len(res.Eligible))

		electedKeys := countPubKeys(res.Elected)
		for _, v := range electedList {
			assert.Equal(t, 1, electedKeys[string(v.PubKey())])
		}

		for _, count := range countPubKeys(res.Elected, res.Eligible) {
			assert.Equal(t, 1, count)
		}
	}
}

func TestRandHashShuffler_UpdateNodeLists_ElectedSmallerThanNodesToShuffleRotatesWhenNoneDeparted(t *testing.T) {
	t.Parallel()

	shuffler := createShufflerWithNodesToShuffle(t, 1, 4)

	electedList := generateValidatorList(1)
	eligibleList := generateValidatorList(3)

	res, err := shuffler.UpdateNodeLists(ArgsUpdateNodes{
		Elected:  electedList,
		Eligible: eligibleList,
		Rand:     generateRandomByteArray(32),
	})
	require.Nil(t, err)

	assert.Equal(t, 1, len(res.Elected))
	assert.Equal(t, 3, len(res.Eligible))
	assert.Equal(t, 0, countPubKeys(res.Elected)[string(electedList[0].PubKey())])
	assert.Equal(t, 1, countPubKeys(res.Eligible)[string(electedList[0].PubKey())])
}

func TestRandHashShuffler_UpdateNodeLists_ElectedSmallerThanNodesToShuffleRotatesTheRemainderOfDeparted(t *testing.T) {
	t.Parallel()

	shuffler := createShufflerWithNodesToShuffle(t, 5, 4)

	electedList := generateValidatorList(3)
	eligibleList := generateValidatorList(10)

	res, err := shuffler.UpdateNodeLists(ArgsUpdateNodes{
		Elected:  electedList,
		Eligible: eligibleList,
		Rand:     generateRandomByteArray(32),
	})
	require.Nil(t, err)

	assert.Equal(t, 5, len(res.Elected))
	assert.Equal(t, 8, len(res.Eligible))

	survivors, rotatedOut := countRotation(electedList, res)
	assert.Equal(t, 1, survivors)
	assert.Equal(t, 2, rotatedOut)
}

func countRotation(electedList []Validator, res *ResUpdateNodes) (int, int) {
	electedKeys := countPubKeys(res.Elected)
	eligibleKeys := countPubKeys(res.Eligible)
	survivors := 0
	rotatedOut := 0
	for _, v := range electedList {
		survivors += electedKeys[string(v.PubKey())]
		rotatedOut += eligibleKeys[string(v.PubKey())]
	}

	return survivors, rotatedOut
}

func TestRandHashShuffler_UpdateNodeLists_ElectedEqualToNodesToShuffleRotatesAllOut(t *testing.T) {
	t.Parallel()

	shuffler := createShufflerWithNodesToShuffle(t, 21, 4)

	electedList := generateValidatorList(4)
	eligibleList := generateValidatorList(30)

	res, err := shuffler.UpdateNodeLists(ArgsUpdateNodes{
		Elected:  electedList,
		Eligible: eligibleList,
		Rand:     generateRandomByteArray(32),
	})
	require.Nil(t, err)

	assert.Equal(t, 21, len(res.Elected))
	assert.Equal(t, 13, len(res.Eligible))

	survivors, rotatedOut := countRotation(electedList, res)
	assert.Equal(t, 0, survivors)
	assert.Equal(t, 4, rotatedOut)
}

func TestRandHashShuffler_UpdateNodeLists_NodesToShuffleAboveElectedAndNodesRotatesAllOut(t *testing.T) {
	t.Parallel()

	shuffler := createShufflerWithNodesToShuffle(t, 21, 40)

	electedList := generateValidatorList(21)
	eligibleList := generateValidatorList(30)

	var res *ResUpdateNodes
	var err error
	require.NotPanics(t, func() {
		res, err = shuffler.UpdateNodeLists(ArgsUpdateNodes{
			Elected:  electedList,
			Eligible: eligibleList,
			Rand:     generateRandomByteArray(32),
		})
	})
	require.Nil(t, err)

	assert.Equal(t, 21, len(res.Elected))
	assert.Equal(t, 30, len(res.Eligible))

	survivors, rotatedOut := countRotation(electedList, res)
	assert.Equal(t, 0, survivors)
	assert.Equal(t, 21, rotatedOut)
}

func TestRandHashShuffler_UpdateNodeLists_ScarceEligibleCountsDeparturesAgainstSurplus(t *testing.T) {
	t.Parallel()

	shuffler := createShufflerWithNodesToShuffle(t, 5, 10)

	electedList := generateValidatorList(3)
	eligibleList := generateValidatorList(6)

	res, err := shuffler.UpdateNodeLists(ArgsUpdateNodes{
		Elected:  electedList,
		Eligible: eligibleList,
		Rand:     generateRandomByteArray(32),
	})
	require.Nil(t, err)

	assert.Equal(t, 5, len(res.Elected))
	assert.Equal(t, 4, len(res.Eligible))

	survivors, rotatedOut := countRotation(electedList, res)
	assert.Equal(t, 1, survivors)
	assert.Equal(t, 2, rotatedOut)
}

func updateNodeListsWithDepartureCounting(
	t *testing.T,
	nodes uint32,
	nodesToShuffle uint32,
	numElected int,
	numEligible int,
	countDepartures bool,
) ([]Validator, *ResUpdateNodes) {
	shuffler := createShufflerWithForkController(t, nodes, nodesToShuffle, &forkControllerStub{FixAuditChangesV7Value: countDepartures})

	electedList := generateValidatorList(numElected)
	eligibleList := generateValidatorList(numEligible)

	var res *ResUpdateNodes
	var err error
	require.NotPanics(t, func() {
		res, err = shuffler.UpdateNodeLists(ArgsUpdateNodes{
			Elected:  electedList,
			Eligible: eligibleList,
			Rand:     generateRandomByteArray(32),
		})
	})
	require.Nil(t, err)
	require.Equal(t, numElected+numEligible, len(res.Elected)+len(res.Eligible))
	for _, count := range countPubKeys(res.Elected, res.Eligible) {
		require.Equal(t, 1, count)
	}

	return electedList, res
}

func TestRandHashShuffler_UpdateNodeLists_DeparturesDoNotCountAgainstNodesToShuffleBeforeFork(t *testing.T) {
	t.Parallel()

	electedList, res := updateNodeListsWithDepartureCounting(t, 21, 6, 19, 109, false)

	assert.Equal(t, 21, len(res.Elected))
	survivors, rotatedOut := countRotation(electedList, res)
	assert.Equal(t, 13, survivors)
	assert.Equal(t, 6, rotatedOut)
	assert.Equal(t, 8, len(res.Elected)-survivors)
}

func TestRandHashShuffler_UpdateNodeLists_DeparturesCountAgainstNodesToShuffle(t *testing.T) {
	t.Parallel()

	electedList, res := updateNodeListsWithDepartureCounting(t, 21, 6, 19, 109, true)

	assert.Equal(t, 21, len(res.Elected))
	survivors, rotatedOut := countRotation(electedList, res)
	assert.Equal(t, 15, survivors)
	assert.Equal(t, 4, rotatedOut)
	assert.Equal(t, 6, len(res.Elected)-survivors)
}

func TestRandHashShuffler_UpdateNodeLists_DeparturesAtOrAboveNodesToShuffleOnlyRefillSeats(t *testing.T) {
	t.Parallel()

	for _, numElected := range []int{15, 14, 0} {
		electedList, res := updateNodeListsWithDepartureCounting(t, 21, 6, numElected, 109, true)

		assert.Equal(t, 21, len(res.Elected))
		survivors, rotatedOut := countRotation(electedList, res)
		assert.Equal(t, numElected, survivors)
		assert.Equal(t, 0, rotatedOut)
	}
}

func TestRandHashShuffler_UpdateNodeLists_NoDeparturesRotatesFullNodesToShuffleAfterFork(t *testing.T) {
	t.Parallel()

	electedList, res := updateNodeListsWithDepartureCounting(t, 21, 6, 21, 109, true)

	assert.Equal(t, 21, len(res.Elected))
	survivors, rotatedOut := countRotation(electedList, res)
	assert.Equal(t, 15, survivors)
	assert.Equal(t, 6, rotatedOut)
}

func TestRandHashShuffler_UpdateNodeLists_NodesToShuffleAboveNodesRotatesAllOutAfterFork(t *testing.T) {
	t.Parallel()

	for _, numElected := range []int{21, 19} {
		electedList, res := updateNodeListsWithDepartureCounting(t, 21, 40, numElected, 30, true)

		assert.Equal(t, 21, len(res.Elected))
		survivors, rotatedOut := countRotation(electedList, res)
		assert.Equal(t, 0, survivors)
		assert.Equal(t, numElected, rotatedOut)
	}
}

func TestRandHashShuffler_UpdateNodeLists_NodesToShuffleJustAboveNodesSubtractsDeparturesOnce(t *testing.T) {
	t.Parallel()

	electedList, res := updateNodeListsWithDepartureCounting(t, 21, 22, 19, 30, true)

	assert.Equal(t, 21, len(res.Elected))
	survivors, rotatedOut := countRotation(electedList, res)
	assert.Equal(t, 0, survivors)
	assert.Equal(t, 19, rotatedOut)
}

func TestRandHashShuffler_UpdateNodeLists_ScarceEligibleKeepsRotationAfterFork(t *testing.T) {
	t.Parallel()

	electedList, res := updateNodeListsWithDepartureCounting(t, 5, 10, 3, 6, true)

	assert.Equal(t, 5, len(res.Elected))
	survivors, rotatedOut := countRotation(electedList, res)
	assert.Equal(t, 1, survivors)
	assert.Equal(t, 2, rotatedOut)
}

func TestRandHashShuffler_UpdateNodeLists_DepartureCountingFollowsPreparedEpoch(t *testing.T) {
	t.Parallel()

	activationEpoch := uint32(10)
	shuffler := createShufflerWithForkController(t, 21, 6, &forkControllerStub{FixAuditChangesV7Epoch: &activationEpoch})

	for epoch, expectedRotatedOut := range map[uint32]int{9: 6, 10: 4, 11: 4} {
		electedList := generateValidatorList(19)
		res, err := shuffler.UpdateNodeLists(ArgsUpdateNodes{
			Elected:  electedList,
			Eligible: generateValidatorList(109),
			Rand:     generateRandomByteArray(32),
			Epoch:    epoch,
		})
		require.Nil(t, err)

		_, rotatedOut := countRotation(electedList, res)
		assert.Equal(t, expectedRotatedOut, rotatedOut, "epoch %d", epoch)
	}
}

func createShufflerWithConfigs(t *testing.T, nodes uint32, configs []config.MaxNodesChangeConfig) *randHashShuffler {
	shuffler, err := NewHashValidatorsShuffler(&NodesShufflerArgs{
		Nodes:                nodes,
		MaxNodesEnableConfig: configs,
		ForkController:       &forkControllerStub{},
	})
	require.Nil(t, err)

	return shuffler
}

func TestRandHashShuffler_ElectionLimitsFollowsConfigSwitchEpoch(t *testing.T) {
	t.Parallel()

	shuffler := createShufflerWithConfigs(t, 21, []config.MaxNodesChangeConfig{
		{EpochEnable: 256, NodesToShuffle: 6},
		{EpochEnable: 0, NodesToShuffle: 4},
	})

	for _, epoch := range []uint32{0, 255, 256, 6000} {
		expectedNodesToShuffle := uint32(4)
		if epoch >= 256 {
			expectedNodesToShuffle = 6
		}

		minNodes, nodesToShuffle := shuffler.ElectionLimits(epoch)
		assert.Equal(t, uint32(21), minNodes, "epoch %d", epoch)
		assert.Equal(t, expectedNodesToShuffle, nodesToShuffle, "epoch %d", epoch)

		shuffler.UpdateShufflerConfig(epoch)
		assert.Equal(t, shuffler.activeNodesConfig.NodesToShuffle, nodesToShuffle, "epoch %d", epoch)
	}
}

func TestRandHashShuffler_ElectionLimitsReturnsNodesWhenNoConfigApplies(t *testing.T) {
	t.Parallel()

	withoutConfigs := createShufflerWithConfigs(t, 21, nil)
	minNodes, nodesToShuffle := withoutConfigs.ElectionLimits(5)
	assert.Equal(t, uint32(21), minNodes)
	assert.Equal(t, uint32(21), nodesToShuffle)

	withLaterConfig := createShufflerWithConfigs(t, 21, []config.MaxNodesChangeConfig{
		{EpochEnable: 10, NodesToShuffle: 6},
	})
	minNodes, nodesToShuffle = withLaterConfig.ElectionLimits(9)
	assert.Equal(t, uint32(21), minNodes)
	assert.Equal(t, uint32(21), nodesToShuffle)

	withLaterConfig.UpdateShufflerConfig(9)
	assert.Equal(t, withLaterConfig.activeNodesConfig.NodesToShuffle, nodesToShuffle)
}

func TestRandHashShuffler_ElectionLimitsLeavesActiveConfigUnchanged(t *testing.T) {
	t.Parallel()

	shuffler := createShufflerWithConfigs(t, 21, []config.MaxNodesChangeConfig{
		{EpochEnable: 0, NodesToShuffle: 4},
		{EpochEnable: 256, NodesToShuffle: 6},
	})
	shuffler.UpdateShufflerConfig(0)
	activeBefore := shuffler.activeNodesConfig

	_, nodesToShuffle := shuffler.ElectionLimits(6000)
	assert.Equal(t, uint32(6), nodesToShuffle)
	assert.Equal(t, activeBefore, shuffler.activeNodesConfig)
	assert.Equal(t, uint32(21), shuffler.nodes)
}

func TestRandHashShuffler_UpdateShufflerConfigCopiesTheSelectedConfig(t *testing.T) {
	t.Parallel()

	shuffler := createShufflerWithConfigs(t, 21, []config.MaxNodesChangeConfig{
		{EpochEnable: 256, NodesToShuffle: 6},
		{EpochEnable: 10, NodesToShuffle: 4},
	})

	shuffler.UpdateShufflerConfig(300)
	assert.Equal(t, config.MaxNodesChangeConfig{EpochEnable: 256, NodesToShuffle: 6}, shuffler.activeNodesConfig)

	shuffler.UpdateShufflerConfig(100)
	assert.Equal(t, config.MaxNodesChangeConfig{EpochEnable: 10, NodesToShuffle: 4}, shuffler.activeNodesConfig)

	shuffler.UpdateShufflerConfig(5)
	assert.Equal(t, config.MaxNodesChangeConfig{EpochEnable: 10, NodesToShuffle: 21}, shuffler.activeNodesConfig)
}

func pubKeysOf(validators []Validator) [][]byte {
	pubKeys := make([][]byte, len(validators))
	for i, v := range validators {
		pubKeys[i] = v.PubKey()
	}

	return pubKeys
}

var goldenShuffleRandomness = []byte("klc-2827-golden-randomness")

var goldenShuffleOrder = []struct {
	pubKey string
	index  uint32
}{
	{pubKey: "golden-pk-5", index: 5},
	{pubKey: "golden-pk-3", index: 8},
	{pubKey: "golden-pk-3", index: 8},
	{pubKey: "golden-pk-1", index: 1},
	{pubKey: "golden-pk-4", index: 4},
	{pubKey: "golden-pk-2", index: 2},
	{pubKey: "golden-pk-7", index: 7},
	{pubKey: "golden-pk-0", index: 0},
	{pubKey: "golden-pk-6", index: 6},
}

func goldenShuffleValidators(t *testing.T) []Validator {
	validators := make([]Validator, 0, 9)
	for i := 0; i < 8; i++ {
		v, err := NewValidator([]byte(fmt.Sprintf("owner-%d", i)), []byte(fmt.Sprintf("golden-pk-%d", i)), DefaultSelectionChances, uint32(i))
		require.Nil(t, err)
		validators = append(validators, v)
	}

	duplicate, err := NewValidator([]byte("owner-8"), []byte("golden-pk-3"), DefaultSelectionChances, 8)
	require.Nil(t, err)

	return append(validators, duplicate)
}

func Test_shuffleListKeepsHistoricalOrder(t *testing.T) {
	t.Parallel()

	shuffled := shuffleList(goldenShuffleValidators(t), goldenShuffleRandomness)

	require.Len(t, shuffled, len(goldenShuffleOrder))
	for i, expected := range goldenShuffleOrder {
		assert.Equal(t, expected.pubKey, string(shuffled[i].PubKey()), i)
		assert.Equal(t, expected.index, shuffled[i].Index(), i)
	}
}

func TestShufflePublicKeys_KeepsHistoricalOrder(t *testing.T) {
	t.Parallel()

	pubKeys := pubKeysOf(goldenShuffleValidators(t))
	original := make([][]byte, len(pubKeys))
	for i, pubKey := range pubKeys {
		original[i] = append([]byte(nil), pubKey...)
	}

	shuffled := ShufflePublicKeys(pubKeys, goldenShuffleRandomness)

	require.Len(t, shuffled, len(goldenShuffleOrder))
	for i, expected := range goldenShuffleOrder {
		assert.Equal(t, expected.pubKey, string(shuffled[i]), i)
	}
	assert.Equal(t, original, pubKeys)
}

func TestShufflePublicKeys_EmptyInputReturnsEmptyList(t *testing.T) {
	t.Parallel()

	randomness := generateRandomByteArray(32)
	assert.Equal(t, 0, len(shuffleList(nil, randomness)))
	assert.NotNil(t, ShufflePublicKeys(nil, randomness))
	assert.Equal(t, 0, len(ShufflePublicKeys(nil, randomness)))
	assert.Equal(t, 0, len(ShufflePublicKeys([][]byte{}, randomness)))
}

func TestShufflePublicKeys_LeavesInputBackingArraysUntouched(t *testing.T) {
	t.Parallel()

	randomness := generateRandomByteArray(32)
	backing := make([][]byte, 4)
	pubKeys := make([][]byte, 4)
	tails := make([][]byte, 4)
	for i := range pubKeys {
		backing[i] = make([]byte, 64)
		copy(backing[i], generateRandomByteArray(32))
		pubKeys[i] = backing[i][:32]
		tails[i] = append([]byte{}, backing[i][32:]...)
	}

	first := ShufflePublicKeys(pubKeys, randomness)
	require.Equal(t, 4, len(first))
	for i := range backing {
		assert.Equal(t, tails[i], backing[i][32:])
	}
	assert.Equal(t, first, ShufflePublicKeys(pubKeys, randomness))
}
