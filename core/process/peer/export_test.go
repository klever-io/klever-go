package peer

import (
	"github.com/klever-io/klever-go/data/state"
)

// CheckForMissedBlocks -
func (vs *validatorStatistics) CheckForMissedBlocks(
	currentHeaderSlot uint64,
	previousHeaderSlot uint64,
	prevRandSeed []byte,
	epoch uint32,
) error {
	return vs.checkForMissedBlocks(currentHeaderSlot, previousHeaderSlot, prevRandSeed, epoch)
}

// LoadPeerAccount -
func (vs *validatorStatistics) LoadPeerAccount(address []byte) (state.PeerAccountHandler, error) {
	return vs.loadPeerAccount(address)
}

// GetLeaderDecreaseCount -
func (vs *validatorStatistics) GetLeaderDecreaseCount(key []byte) uint32 {
	vs.mutValidatorStatistics.RLock()
	defer vs.mutValidatorStatistics.RUnlock()

	return vs.missedBlocksCounters.get(key).leaderDecreaseCount
}

// UpdateMissedBlocksCounters -
func (vs *validatorStatistics) UpdateMissedBlocksCounters() error {
	return vs.updateMissedBlocksCounters()
}

// GetCache -
func (ptp *PeerTypeProvider) GetCache() map[string]*peerListAndShard {
	ptp.mutCache.RLock()
	defer ptp.mutCache.RUnlock()
	return ptp.cache
}

// GetCache -
func (vp *validatorsProvider) GetCache() map[string]*state.ValidatorApiResponse {
	vp.lock.RLock()
	defer vp.lock.RUnlock()
	return vp.cache
}
