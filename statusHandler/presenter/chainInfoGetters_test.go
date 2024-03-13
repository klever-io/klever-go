package presenter

import (
	"testing"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/tools"
	"github.com/stretchr/testify/assert"
)

func TestPresenterStatusHandler_GetNonce(t *testing.T) {
	t.Parallel()

	nonce := uint64(1000)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricNonce, nonce)
	result := presenterStatusHandler.GetNonce()

	assert.Equal(t, nonce, result)
}

func TestPresenterStatusHandler_GetIsSyncing(t *testing.T) {
	t.Parallel()

	isSyncing := uint64(1)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricIsSyncing, isSyncing)
	result := presenterStatusHandler.GetIsSyncing()

	assert.Equal(t, isSyncing, result)
}

func TestPresenterStatusHandler_GetTxPoolLoad(t *testing.T) {
	t.Parallel()

	txPoolLoad := uint64(1000)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricTxPoolLoad, txPoolLoad)
	result := presenterStatusHandler.GetTxPoolLoad()

	assert.Equal(t, txPoolLoad, result)
}

func TestPresenterStatusHandler_GetProbableHighestNonce(t *testing.T) {
	t.Parallel()

	probableHighestNonce := uint64(1000)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricProbableHighestNonce, probableHighestNonce)
	result := presenterStatusHandler.GetProbableHighestNonce()

	assert.Equal(t, probableHighestNonce, result)
}

func TestPresenterStatusHandler_GetSynchronizedSlot(t *testing.T) {
	t.Parallel()

	synchronizedSlot := uint64(1000)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricSynchronizedSlot, synchronizedSlot)
	result := presenterStatusHandler.GetSynchronizedSlot()

	assert.Equal(t, synchronizedSlot, result)
}

func TestPresenterStatusHandler_GetSlotTime(t *testing.T) {
	t.Parallel()

	slotTime := uint64(1000)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricSlotTime, slotTime)
	result := presenterStatusHandler.GetSlotTime()

	assert.Equal(t, slotTime, result)
}

func TestPresenterStatusHandler_GetLiveValidatorNodes(t *testing.T) {
	t.Parallel()

	numLiveValidatorNodes := uint64(1000)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricLiveValidatorNodes, numLiveValidatorNodes)
	result := presenterStatusHandler.GetLiveValidatorNodes()

	assert.Equal(t, numLiveValidatorNodes, result)
}

func TestPresenterStatusHandler_GetConnectedNodes(t *testing.T) {
	t.Parallel()

	numConnectedNodes := uint64(1000)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricConnectedNodes, numConnectedNodes)
	result := presenterStatusHandler.GetConnectedNodes()

	assert.Equal(t, numConnectedNodes, result)
}

func TestPresenterStatusHandler_GetNumConnectedPeers(t *testing.T) {
	t.Parallel()

	numConnectedPeers := uint64(1000)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricNumConnectedPeers, numConnectedPeers)
	result := presenterStatusHandler.GetNumConnectedPeers()

	assert.Equal(t, numConnectedPeers, result)
}

func TestPresenterStatusHandler_GetCurrentSlot(t *testing.T) {
	t.Parallel()

	currentSlot := uint64(1000)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricCurrentSlot, currentSlot)
	result := presenterStatusHandler.GetCurrentSlot()

	assert.Equal(t, currentSlot, result)
}

func TestPresenterStatusHandler_CalculateTimeToSynchronize(t *testing.T) {
	t.Parallel()

	currentBlockNonce := uint64(10)
	probableHighestNonce := uint64(200)
	synchronizationSpeed := uint64(10)
	presenterStatusHandler := NewPresenterStatusHandler()

	time.Sleep(time.Second)
	presenterStatusHandler.SetUInt64Value(core.MetricSynchronizedSlot, currentBlockNonce)
	presenterStatusHandler.SetUInt64Value(core.MetricCurrentSlot, probableHighestNonce)
	presenterStatusHandler.synchronizationSpeedHistory = append(presenterStatusHandler.synchronizationSpeedHistory, synchronizationSpeed)
	synchronizationEstimation := presenterStatusHandler.CalculateTimeToSynchronize(1000)

	// Node needs to synchronize 190 blocks and synchronization speed is 10 blocks/s
	// Synchronization estimation will be equals with ((200-10)/10) seconds
	numBlocksThatNeedToBeSynchronized := probableHighestNonce - currentBlockNonce
	synchronizationEstimationExpected := numBlocksThatNeedToBeSynchronized / synchronizationSpeed

	assert.Equal(t, tools.SecondsToHourMinSec(int(synchronizationEstimationExpected)), synchronizationEstimation)
}

func TestPresenterStatusHandler_CalculateSynchronizationSpeed(t *testing.T) {
	t.Parallel()

	initialNonce := uint64(10)
	currentNonce := uint64(20)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricSynchronizedSlot, initialNonce)
	_ = presenterStatusHandler.CalculateSynchronizationSpeed(1000)
	presenterStatusHandler.SetUInt64Value(core.MetricSynchronizedSlot, currentNonce)
	syncSpeed := presenterStatusHandler.CalculateSynchronizationSpeed(1000)

	expectedSpeed := currentNonce - initialNonce
	assert.Equal(t, expectedSpeed, syncSpeed)
}

func TestPresenterStatusHandler_CalculateSynchronizationSpeedMultipleSlotsPerSecond(t *testing.T) {
	t.Parallel()

	initialNonce := uint64(10)
	currentNonce := uint64(20)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricSynchronizedSlot, initialNonce)
	_ = presenterStatusHandler.CalculateSynchronizationSpeed(100)
	presenterStatusHandler.SetUInt64Value(core.MetricSynchronizedSlot, currentNonce)
	syncSpeed := presenterStatusHandler.CalculateSynchronizationSpeed(100)

	expectedSpeed := 10 * (currentNonce - initialNonce)
	assert.Equal(t, expectedSpeed, syncSpeed)
}

func TestPresenterStatusHandler_GetNumTxProcessed(t *testing.T) {
	t.Parallel()

	numTxProcessed := uint64(1000)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricNumProcessedTxs, numTxProcessed)
	result := presenterStatusHandler.GetNumTxProcessed()

	assert.Equal(t, numTxProcessed, result)
}

func TestPresenterStatusHandler_GetEpochInfo(t *testing.T) {
	t.Parallel()

	numSlotsPerEpoch := uint64(20)
	slotInterval := uint64(5000)
	slotAtEpochStart := uint64(60)
	currentSlot := uint64(70)

	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricSlotInterval, slotInterval)
	presenterStatusHandler.SetUInt64Value(core.MetricSlotsPerEpoch, numSlotsPerEpoch)
	presenterStatusHandler.SetUInt64Value(core.MetricSlotAtEpochStart, slotAtEpochStart)
	presenterStatusHandler.SetUInt64Value(core.MetricCurrentSlot, currentSlot)

	expectedRemainingTime := tools.SecondsToHourMinSec(int((slotAtEpochStart + numSlotsPerEpoch - currentSlot) * slotInterval / 1000))
	currentEpochSlot, currentEpochFinishSlot, epochLoadPercent, remainingTime := presenterStatusHandler.GetEpochInfo()
	assert.Equal(t, currentSlot, currentEpochSlot)
	assert.Equal(t, numSlotsPerEpoch+slotAtEpochStart, currentEpochFinishSlot)
	assert.Equal(t, expectedRemainingTime, remainingTime)
	assert.Equal(t, 50, epochLoadPercent)
}
