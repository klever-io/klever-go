package peer

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatorCounters(t *testing.T) {
	t.Parallel()
	nrOfIncrements := 10
	testKey := []byte("testKey")
	vc := make(validatorSlotCounters)

	for i := 0; i < nrOfIncrements; i++ {
		vc.increaseLeader(testKey)
		vc.decreaseLeader(testKey)
		vc.increaseValidator(testKey)
		vc.decreaseValidator(testKey)
	}

	assert.Equal(t, uint32(nrOfIncrements), vc.get(testKey).leaderIncreaseCount)
	assert.Equal(t, uint32(nrOfIncrements), vc.get(testKey).leaderDecreaseCount)
	assert.Equal(t, uint32(nrOfIncrements), vc.get(testKey).validatorIncreaseCount)
	assert.Equal(t, uint32(nrOfIncrements), vc.get(testKey).validatorDecreaseCount)
}

func TestValidatorCountersReset(t *testing.T) {
	t.Parallel()
	nrOfIncrements := 10
	testKey := []byte("testKey")
	vc := make(validatorSlotCounters)

	for i := 0; i < nrOfIncrements; i++ {
		vc.increaseLeader(testKey)
		vc.decreaseLeader(testKey)
		vc.increaseValidator(testKey)
		vc.decreaseValidator(testKey)
	}

	vc.reset()

	assert.Equal(t, uint32(0), vc.get(testKey).leaderIncreaseCount)
	assert.Equal(t, uint32(0), vc.get(testKey).leaderDecreaseCount)
	assert.Equal(t, uint32(0), vc.get(testKey).validatorIncreaseCount)
	assert.Equal(t, uint32(0), vc.get(testKey).validatorDecreaseCount)
}

func TestValidatorCounters_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	vc := make(validatorSlotCounters)
	numGoroutines := 100
	numOperations := 1000
	testKey := []byte("testKey")

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 4) // 4 operations per goroutine
	// validatorsCounters is not thread-safe, so we need to use a lock
	lock := sync.Mutex{}

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				lock.Lock()
				vc.increaseLeader(testKey)
				lock.Unlock()
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				lock.Lock()
				vc.decreaseLeader(testKey)
				lock.Unlock()
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				lock.Lock()
				vc.increaseValidator(testKey)
				lock.Unlock()
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				lock.Lock()
				vc.decreaseValidator(testKey)
				lock.Unlock()
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, uint32(numGoroutines*numOperations), vc.get(testKey).leaderIncreaseCount)
	assert.Equal(t, uint32(numGoroutines*numOperations), vc.get(testKey).leaderDecreaseCount)
	assert.Equal(t, uint32(numGoroutines*numOperations), vc.get(testKey).validatorIncreaseCount)
	assert.Equal(t, uint32(numGoroutines*numOperations), vc.get(testKey).validatorDecreaseCount)
}

func TestValidatorCounters_LargeNumberOfIncrements(t *testing.T) {
	t.Parallel()

	vc := make(validatorSlotCounters)
	testKey := []byte("testKey")
	largeNumber := uint32(1000000) // 1 million

	for i := uint32(0); i < largeNumber; i++ {
		vc.increaseLeader(testKey)
		vc.decreaseLeader(testKey)
		vc.increaseValidator(testKey)
		vc.decreaseValidator(testKey)
	}

	assert.Equal(t, largeNumber, vc.get(testKey).leaderIncreaseCount)
	assert.Equal(t, largeNumber, vc.get(testKey).leaderDecreaseCount)
	assert.Equal(t, largeNumber, vc.get(testKey).validatorIncreaseCount)
	assert.Equal(t, largeNumber, vc.get(testKey).validatorDecreaseCount)
}

func TestValidatorCounters_MultipleKeys(t *testing.T) {
	t.Parallel()

	vc := make(validatorSlotCounters)
	keys := [][]byte{
		[]byte("key1"),
		[]byte("key2"),
		[]byte("key3"),
	}
	incrementsPerKey := 100

	for _, key := range keys {
		for i := 0; i < incrementsPerKey; i++ {
			vc.increaseLeader(key)
			vc.decreaseLeader(key)
			vc.increaseValidator(key)
			vc.decreaseValidator(key)
		}
	}

	for _, key := range keys {
		assert.Equal(t, uint32(incrementsPerKey), vc.get(key).leaderIncreaseCount)
		assert.Equal(t, uint32(incrementsPerKey), vc.get(key).leaderDecreaseCount)
		assert.Equal(t, uint32(incrementsPerKey), vc.get(key).validatorIncreaseCount)
		assert.Equal(t, uint32(incrementsPerKey), vc.get(key).validatorDecreaseCount)
	}
}

func TestValidatorCounters_ResetMultipleTimes(t *testing.T) {
	t.Parallel()

	vc := make(validatorSlotCounters)
	testKey := []byte("testKey")
	incrementsPerReset := 100
	numResets := 10

	for i := 0; i < numResets; i++ {
		for j := 0; j < incrementsPerReset; j++ {
			vc.increaseLeader(testKey)
			vc.decreaseLeader(testKey)
			vc.increaseValidator(testKey)
			vc.decreaseValidator(testKey)
		}

		vc.reset()

		assert.Equal(t, uint32(0), vc.get(testKey).leaderIncreaseCount)
		assert.Equal(t, uint32(0), vc.get(testKey).leaderDecreaseCount)
		assert.Equal(t, uint32(0), vc.get(testKey).validatorIncreaseCount)
		assert.Equal(t, uint32(0), vc.get(testKey).validatorDecreaseCount)
	}
}
