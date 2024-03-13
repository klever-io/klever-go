package sharding

import (
	"crypto/rand"
	"fmt"
	"reflect"
	"sort"
	"testing"

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
	}
	shuffler, err := NewHashValidatorsShuffler(shufflerArgs)
	assert.Nil(t, err)
	assert.NotNil(t, shuffler)
}

func TestRandHashShuffler_UpdateNodeLists(t *testing.T) {
	t.Parallel()

	shufflerArgs := &NodesShufflerArgs{
		Nodes:                10,
		MaxNodesEnableConfig: nil,
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
