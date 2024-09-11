package presenter

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/tools"
)

var maxSpeedHistorySaved = 2000

// GetNonce will return current nonce of node
func (psh *PresenterStatusHandler) GetNonce() uint64 {
	return psh.getFromCacheAsUint64(core.MetricNonce)
}

// GetIsSyncing will return state of the node
func (psh *PresenterStatusHandler) GetIsSyncing() uint64 {
	return psh.getFromCacheAsUint64(core.MetricIsSyncing)
}

// GetTxPoolLoad will return how many transactions are in the pool
func (psh *PresenterStatusHandler) GetTxPoolLoad() uint64 {
	return psh.getFromCacheAsUint64(core.MetricTxPoolLoad)
}

// GetTPSCurrent will return how many transactions per second in current slot
func (psh *PresenterStatusHandler) GetTPSCurrent() uint64 {
	return psh.getFromCacheAsUint64(core.MetricLastBlockTxCount) / 4
}

// GetTPSPeak will return how many transactions per second at peak
func (psh *PresenterStatusHandler) GetTPSPeak() uint64 {
	return psh.getFromCacheAsUint64(core.MetricPeakTPS)
}

// GetProbableHighestNonce will return the highest nonce of blockchain
func (psh *PresenterStatusHandler) GetProbableHighestNonce() uint64 {
	return psh.getFromCacheAsUint64(core.MetricProbableHighestNonce)
}

// GetSynchronizedSlot will return number of synchronized slot
func (psh *PresenterStatusHandler) GetSynchronizedSlot() uint64 {
	return psh.getFromCacheAsUint64(core.MetricSynchronizedSlot)
}

// GetSlotTime will return duration of a slot
func (psh *PresenterStatusHandler) GetSlotTime() uint64 {
	return psh.getFromCacheAsUint64(core.MetricSlotTime)
}

// GetLiveValidatorNodes will return how many validator nodes are in blockchain
func (psh *PresenterStatusHandler) GetLiveValidatorNodes() uint64 {
	return psh.getFromCacheAsUint64(core.MetricLiveValidatorNodes)
}

// GetConnectedNodes will return how many nodes are connected
func (psh *PresenterStatusHandler) GetConnectedNodes() uint64 {
	return psh.getFromCacheAsUint64(core.MetricConnectedNodes)
}

// GetNumConnectedPeers will return how many peers are connected
func (psh *PresenterStatusHandler) GetNumConnectedPeers() uint64 {
	return psh.getFromCacheAsUint64(core.MetricNumConnectedPeers)
}

// GetCurrentSlot will return current slot of node
func (psh *PresenterStatusHandler) GetCurrentSlot() uint64 {
	return psh.getFromCacheAsUint64(core.MetricCurrentSlot)
}

// CalculateTimeToSynchronize will calculate and return an estimation of
// the time required for synchronization in a human friendly format
func (psh *PresenterStatusHandler) CalculateTimeToSynchronize(numMillisecondsRefreshTime int) string {
	if numMillisecondsRefreshTime < 1 {
		return "N/A"
	}

	currentSynchronizedSlot := psh.GetSynchronizedSlot()

	numSynchronizationSpeedHistory := len(psh.synchronizationSpeedHistory)

	sum := uint64(0)
	for i := 0; i < len(psh.synchronizationSpeedHistory); i++ {
		sum += psh.synchronizationSpeedHistory[i]
	}

	speed := float64(0)
	if numSynchronizationSpeedHistory > 0 {
		speed = float64(sum*1000) / float64(numSynchronizationSpeedHistory*numMillisecondsRefreshTime)
	}

	currentSlot := psh.GetCurrentSlot()
	if currentSlot < currentSynchronizedSlot || speed == 0 {
		return ""
	}

	remainingSlotsToSynchronize := currentSlot - currentSynchronizedSlot
	timeEstimationSeconds := float64(remainingSlotsToSynchronize) / speed
	remainingTime := tools.SecondsToHourMinSec(int(timeEstimationSeconds))

	return remainingTime
}

// CalculateSynchronizationSpeed will calculate and return speed of synchronization
// how many blocks per second are synchronized
func (psh *PresenterStatusHandler) CalculateSynchronizationSpeed(numMillisecondsRefreshTime int) uint64 {
	currentSynchronizedSlot := psh.GetSynchronizedSlot()
	if psh.oldSlot == 0 {
		psh.oldSlot = currentSynchronizedSlot
		return 0
	}

	slotsPerSecond := currentSynchronizedSlot - psh.oldSlot
	if currentSynchronizedSlot < psh.oldSlot {
		// adjust to zero if slot older than current slot
		slotsPerSecond = 0
	}

	if len(psh.synchronizationSpeedHistory) >= maxSpeedHistorySaved {
		psh.synchronizationSpeedHistory = psh.synchronizationSpeedHistory[1:len(psh.synchronizationSpeedHistory)]
	}
	psh.synchronizationSpeedHistory = append(psh.synchronizationSpeedHistory, slotsPerSecond)

	psh.oldSlot = currentSynchronizedSlot

	numSyncedBlocks := uint64(0)
	cumulatedTime := uint64(0)
	lastIndex := len(psh.synchronizationSpeedHistory) - 1
	millisecondsInASecond := uint64(1000)
	for {
		if lastIndex < 0 {
			break
		}
		if cumulatedTime >= millisecondsInASecond {
			break
		}

		numSyncedBlocks += psh.synchronizationSpeedHistory[lastIndex]
		lastIndex--
		cumulatedTime += uint64(numMillisecondsRefreshTime) // #nosec G115
	}
	if cumulatedTime == 0 || numSyncedBlocks == 0 {
		return 0
	}

	timeAdjustment := float64(millisecondsInASecond) / float64(cumulatedTime)
	syncedBlocksAdjustment := timeAdjustment * float64(numSyncedBlocks)

	return uint64(syncedBlocksAdjustment)
}

// GetNumTxProcessed will return number of processed transactions since node starts
func (psh *PresenterStatusHandler) GetNumTxProcessed() uint64 {
	return psh.getFromCacheAsUint64(core.MetricNumProcessedTxs)
}

// GetEpochInfo will return information about current epoch
func (psh *PresenterStatusHandler) GetEpochInfo() (uint64, uint64, int, string) {
	slotAtEpochStart := psh.getFromCacheAsUint64(core.MetricSlotAtEpochStart)
	slotsPerEpoch := psh.getFromCacheAsUint64(core.MetricSlotsPerEpoch)
	currentSlot := psh.getFromCacheAsUint64(core.MetricCurrentSlot)
	slotInterval := psh.getFromCacheAsUint64(core.MetricSlotInterval)

	epochFinishSlot := slotAtEpochStart + slotsPerEpoch
	slotsRemained := epochFinishSlot - currentSlot
	if epochFinishSlot < currentSlot {
		slotsRemained = 0
	}
	if slotsRemained <= 0 || slotsPerEpoch == 0 || slotInterval == 0 {
		return 0, 0, 0, ""
	}
	secondsRemainedInEpoch := slotsRemained * slotInterval / 1000

	remainingTime := tools.SecondsToHourMinSec(int(secondsRemainedInEpoch)) // #nosec G115

	epochLoadPercent := 100 - int(float64(slotsRemained)/float64(slotsPerEpoch)*100.0)

	return currentSlot, epochFinishSlot, epochLoadPercent, remainingTime
}
