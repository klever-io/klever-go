package sync

import (
	"bytes"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

type headerInfo struct {
	epoch uint32
	nonce uint64
	slot  uint64
	hash  []byte
	state process.BlockHeaderState
}

type checkpointInfo struct {
	nonce uint64
	slot  uint64
	hash  []byte
}

type forkInfo struct {
	checkpoint             []*checkpointInfo
	finalCheckpoint        *checkpointInfo
	probableHighestNonce   uint64
	highestNonceReceived   uint64
	rollBackNonce          uint64
	lastSlotWithForcedFork int64
}

// baseForkDetector defines a struct with necessary data needed for fork detection
type baseForkDetector struct {
	slotManager consensus.SlotManager

	headers    map[uint64][]*headerInfo
	mutHeaders sync.RWMutex
	fork       forkInfo
	mutFork    sync.RWMutex

	blackListHandler   process.TimeCacher
	genesisTime        int64
	forkDetector       forkDetector
	genesisNonce       uint64
	genesisSlot        uint64
	maxForkHeaderEpoch uint32
	tmpStuck           time.Time
	// lastCheckpointAheadWarnSlot throttles the checkpoint-ahead warning to once
	// per slot; CheckFork can run on every sync-loop iteration while the node is
	// not synchronized.
	lastCheckpointAheadWarnSlot atomic.Int64
}

// SetRollBackNonce sets the nonce where the chain should roll back
func (bfd *baseForkDetector) SetRollBackNonce(nonce uint64) {
	bfd.mutFork.Lock()
	bfd.fork.rollBackNonce = nonce
	bfd.mutFork.Unlock()
}

func (bfd *baseForkDetector) getRollBackNonce() uint64 {
	bfd.mutFork.RLock()
	nonce := bfd.fork.rollBackNonce
	bfd.mutFork.RUnlock()

	return nonce
}

func (bfd *baseForkDetector) setLastSlotWithForcedFork(slot int64) {
	bfd.mutFork.Lock()
	bfd.fork.lastSlotWithForcedFork = slot
	bfd.mutFork.Unlock()
}

func (bfd *baseForkDetector) lastSlotWithForcedFork() int64 {
	bfd.mutFork.RLock()
	slot := bfd.fork.lastSlotWithForcedFork
	bfd.mutFork.RUnlock()

	return slot
}

func (bfd *baseForkDetector) removePastOrInvalidRecords() {
	bfd.removePastHeaders()
	bfd.removeInvalidReceivedHeaders()
	bfd.removePastCheckpoints()
}

func (bfd *baseForkDetector) checkBlockBasicValidity(
	header data.HeaderHandler,
	headerHash []byte,
) error {
	if check.IfNil(header) {
		return ErrNilHeader
	}
	if headerHash == nil {
		return ErrNilHash
	}

	slotDif := tools.SafeU64ToI64(header.GetSlot()) - tools.SafeU64ToI64(bfd.finalCheckpoint().slot)
	nonceDif := tools.SafeU64ToI64(header.GetNonce()) - tools.SafeU64ToI64(bfd.finalCheckpoint().nonce)
	//TODO: Analyze if the acceptance of some headers which came for the next slot could generate some attack vectors
	nextSlot := bfd.slotManager.Index() + 1
	genesisTimeFromHeader := bfd.computeGenesisTimeFromHeader(header)

	bfd.blackListHandler.Sweep()
	if bfd.blackListHandler.Has(string(header.GetParentHash())) {
		process.AddHeaderToBlackList(bfd.blackListHandler, headerHash)
		return process.ErrHeaderIsBlackListed
	}

	if genesisTimeFromHeader != bfd.genesisTime {
		process.AddHeaderToBlackList(bfd.blackListHandler, headerHash)
		return ErrGenesisTimeMismatch
	}
	if slotDif < 0 {
		return ErrLowerSlotInBlock
	}
	if nonceDif < 0 {
		return ErrLowerNonceInBlock
	}
	if header.GetSlot() > tools.SafeI64ToU64(nextSlot) {
		return ErrHigherSlotInBlock
	}
	if slotDif < nonceDif {
		return ErrHigherNonceInBlock
	}

	return nil

}

func (bfd *baseForkDetector) removePastHeaders() {
	finalCheckpointNonce := bfd.finalCheckpoint().nonce

	bfd.mutHeaders.Lock()
	for nonce := range bfd.headers {
		if nonce < finalCheckpointNonce {
			delete(bfd.headers, nonce)
		}
	}
	bfd.mutHeaders.Unlock()
}

func (bfd *baseForkDetector) removeInvalidReceivedHeaders() {
	finalCheckpointSlot := bfd.finalCheckpoint().slot
	finalCheckpointNonce := bfd.finalCheckpoint().nonce

	bfd.mutHeaders.Lock()
	for nonce, hdrInfos := range bfd.headers {
		validHdrInfos := make([]*headerInfo, 0)
		for i := 0; i < len(hdrInfos); i++ {
			slotDif := tools.SafeU64ToI64(hdrInfos[i].slot) - tools.SafeU64ToI64(finalCheckpointSlot)
			nonceDif := tools.SafeU64ToI64(hdrInfos[i].nonce) - tools.SafeU64ToI64(finalCheckpointNonce)
			hasStateReceived := hdrInfos[i].state == process.BHReceived || hdrInfos[i].state == process.BHReceivedTooLate
			isReceivedHeaderInvalid := hasStateReceived && slotDif < nonceDif
			if isReceivedHeaderInvalid {
				continue
			}

			validHdrInfos = append(validHdrInfos, hdrInfos[i])
		}
		if len(validHdrInfos) == 0 {
			delete(bfd.headers, nonce)
			continue
		}

		bfd.headers[nonce] = validHdrInfos
	}
	bfd.mutHeaders.Unlock()
}

func (bfd *baseForkDetector) removePastCheckpoints() {
	bfd.removeCheckpointsBehindNonce(bfd.finalCheckpoint().nonce)
}

func (bfd *baseForkDetector) removeCheckpointsBehindNonce(nonce uint64) {
	bfd.mutFork.Lock()
	preservedCheckpoint := make([]*checkpointInfo, 0)

	for i := 0; i < len(bfd.fork.checkpoint); i++ {
		if bfd.fork.checkpoint[i].nonce < nonce {
			continue
		}

		preservedCheckpoint = append(preservedCheckpoint, bfd.fork.checkpoint[i])
	}

	bfd.fork.checkpoint = preservedCheckpoint
	bfd.mutFork.Unlock()
}

// computeProbableHighestNonce computes the probable highest nonce from the valid received/processed headers
func (bfd *baseForkDetector) computeProbableHighestNonce() uint64 {
	probableHighestNonce := bfd.finalCheckpoint().nonce

	bfd.mutHeaders.RLock()
	for nonce := range bfd.headers {
		if nonce <= probableHighestNonce {
			continue
		}
		probableHighestNonce = nonce
	}
	bfd.mutHeaders.RUnlock()

	return probableHighestNonce
}

// RemoveHeader removes the stored header with the given nonce and hash
func (bfd *baseForkDetector) RemoveHeader(nonce uint64, hash []byte) {
	bfd.removeCheckpointWithNonce(nonce)

	preservedHdrsInfo := make([]*headerInfo, 0)

	bfd.mutHeaders.Lock()

	hdrsInfo := bfd.headers[nonce]
	for _, hdrInfo := range hdrsInfo {
		if hdrInfo.state != process.BHNotarized && bytes.Equal(hash, hdrInfo.hash) {
			continue
		}

		preservedHdrsInfo = append(preservedHdrsInfo, hdrInfo)
	}

	if len(preservedHdrsInfo) == 0 {
		delete(bfd.headers, nonce)
	} else {
		bfd.headers[nonce] = preservedHdrsInfo
	}

	bfd.mutHeaders.Unlock()

	bfd.forkDetector.computeFinalCheckpoint()

	probableHighestNonce := bfd.computeProbableHighestNonce()
	bfd.setProbableHighestNonce(probableHighestNonce)

	log.Debug("forkDetector.RemoveHeader",
		"nonce", nonce,
		"hash", hash,
		"probable highest nonce", probableHighestNonce,
		"final check point nonce", bfd.finalCheckpoint().nonce)
}

func (bfd *baseForkDetector) removeCheckpointWithNonce(nonce uint64) {
	bfd.mutFork.Lock()
	preservedCheckpoint := make([]*checkpointInfo, 0)

	for i := 0; i < len(bfd.fork.checkpoint); i++ {
		if bfd.fork.checkpoint[i].nonce == nonce {
			continue
		}

		preservedCheckpoint = append(preservedCheckpoint, bfd.fork.checkpoint[i])
	}

	bfd.fork.checkpoint = preservedCheckpoint
	bfd.mutFork.Unlock()

	log.Debug("forkDetector.removeCheckpointWithNonce",
		"nonce", nonce,
		"last check point nonce", bfd.lastCheckpoint().nonce)
}

// append adds a new header in the slice found in nonce position
// it not adds the header if its hash is already stored in the slice
func (bfd *baseForkDetector) append(hdrInfo *headerInfo) bool {
	bfd.mutHeaders.Lock()
	defer bfd.mutHeaders.Unlock()

	hdrInfos := bfd.headers[hdrInfo.nonce]
	isHdrInfosNilOrEmpty := len(hdrInfos) == 0 // no need for nil check, len() for nil returns 0
	if isHdrInfosNilOrEmpty {
		bfd.headers[hdrInfo.nonce] = []*headerInfo{hdrInfo}
		return true
	}

	for _, hdrInfoStored := range hdrInfos {
		if bytes.Equal(hdrInfoStored.hash, hdrInfo.hash) && hdrInfoStored.state == hdrInfo.state {
			return false
		}
	}

	bfd.headers[hdrInfo.nonce] = append(bfd.headers[hdrInfo.nonce], hdrInfo)
	return true
}

// GetHighestFinalBlockNonce gets the highest nonce of the block which is final and it can not be reverted anymore
func (bfd *baseForkDetector) GetHighestFinalBlockNonce() uint64 {
	return bfd.finalCheckpoint().nonce
}

// GetHighestFinalBlockHash gets the hash of the block which is final and it can not be reverted anymore
func (bfd *baseForkDetector) GetHighestFinalBlockHash() []byte {
	return bfd.finalCheckpoint().hash
}

// ProbableHighestNonce gets the probable highest nonce
func (bfd *baseForkDetector) ProbableHighestNonce() uint64 {
	return bfd.probableHighestNonce()
}

// ResetFork resets the forced fork
func (bfd *baseForkDetector) ResetFork() {
	bfd.ResetProbableHighestNonce()
	bfd.setLastSlotWithForcedFork(bfd.slotManager.Index())

	log.Debug("forkDetector.ResetFork",
		"last slot with forced fork", bfd.lastSlotWithForcedFork())
}

// SoftResetFork resets the forking state and apply a new highest nonce
func (bfd *baseForkDetector) SoftResetFork(newProbableHighestNonce uint64) {
	bfd.setProbableHighestNonce(newProbableHighestNonce)
	bfd.setLastSlotWithForcedFork(bfd.slotManager.Index())

	log.Debug("forkDetector.SoftResetFork",
		"last slot with forced fork", bfd.lastSlotWithForcedFork())
}

// ResetProbableHighestNonce resets the probable highest nonce to the last checkpoint nonce / highest notarized nonce
func (bfd *baseForkDetector) ResetProbableHighestNonce() {
	bfd.cleanupReceivedHeadersHigherThanNonce(bfd.lastCheckpoint().nonce)
	probableHighestNonce := bfd.computeProbableHighestNonce()
	bfd.setProbableHighestNonce(probableHighestNonce)

	log.Debug("forkDetector.ResetProbableHighestNonce",
		"probable highest nonce", bfd.probableHighestNonce())
}

func (bfd *baseForkDetector) addCheckpoint(checkpoint *checkpointInfo) {
	bfd.mutFork.Lock()
	bfd.fork.checkpoint = append(bfd.fork.checkpoint, checkpoint)
	bfd.mutFork.Unlock()
}

func (bfd *baseForkDetector) lastCheckpoint() *checkpointInfo {
	bfd.mutFork.RLock()
	lastIndex := len(bfd.fork.checkpoint) - 1
	if lastIndex < 0 {
		bfd.mutFork.RUnlock()
		return &checkpointInfo{
			nonce: bfd.genesisNonce,
			slot:  bfd.genesisSlot,
		}
	}
	lastCheckpoint := bfd.fork.checkpoint[lastIndex]
	bfd.mutFork.RUnlock()

	return lastCheckpoint
}

func (bfd *baseForkDetector) setFinalCheckpoint(finalCheckpoint *checkpointInfo) {
	bfd.mutFork.Lock()
	bfd.fork.finalCheckpoint = finalCheckpoint
	bfd.mutFork.Unlock()
}

// RestoreToGenesis sets class variables to theirs initial values
func (bfd *baseForkDetector) RestoreToGenesis() {
	bfd.mutHeaders.Lock()
	bfd.headers = make(map[uint64][]*headerInfo)
	bfd.mutHeaders.Unlock()

	bfd.mutFork.Lock()

	checkpoint := &checkpointInfo{
		nonce: bfd.genesisNonce,
		slot:  bfd.genesisSlot,
	}
	bfd.fork.checkpoint = []*checkpointInfo{checkpoint}
	bfd.fork.finalCheckpoint = checkpoint
	bfd.fork.probableHighestNonce = bfd.genesisNonce
	bfd.fork.highestNonceReceived = bfd.genesisNonce
	bfd.mutFork.Unlock()
}

func (bfd *baseForkDetector) finalCheckpoint() *checkpointInfo {
	bfd.mutFork.RLock()
	finalCheckpoint := bfd.fork.finalCheckpoint
	bfd.mutFork.RUnlock()

	return finalCheckpoint
}

func (bfd *baseForkDetector) setProbableHighestNonce(nonce uint64) {
	bfd.mutFork.Lock()
	bfd.fork.probableHighestNonce = nonce
	bfd.mutFork.Unlock()
}

func (bfd *baseForkDetector) probableHighestNonce() uint64 {
	bfd.mutFork.RLock()
	probableHighestNonce := bfd.fork.probableHighestNonce
	bfd.mutFork.RUnlock()

	return probableHighestNonce
}

func (bfd *baseForkDetector) setHighestNonceReceived(nonce uint64) {
	if nonce <= bfd.highestNonceReceived() {
		return
	}

	bfd.mutFork.Lock()
	bfd.fork.highestNonceReceived = nonce
	bfd.mutFork.Unlock()

	log.Debug("forkDetector.setHighestNonceReceived",
		"highest nonce received", nonce)
}

func (bfd *baseForkDetector) highestNonceReceived() uint64 {
	bfd.mutFork.RLock()
	highestNonceReceived := bfd.fork.highestNonceReceived
	bfd.mutFork.RUnlock()

	return highestNonceReceived
}

// IsInterfaceNil returns true if there is no value under the interface
func (bfd *baseForkDetector) IsInterfaceNil() bool {
	return bfd == nil
}

// CheckFork method checks if the node could be on the fork
func (bfd *baseForkDetector) CheckFork() *process.ForkInfo {
	var (
		forkHeaderSlot  uint64
		forkHeaderHash  []byte
		selfHdrInfo     *headerInfo
		forkHeaderEpoch uint32
	)

	forkInfoObject := process.NewForkInfo()

	if bfd.isConsensusStuck() {
		forkInfoObject.IsDetected = true
		return forkInfoObject
	}

	rollBackNonce := bfd.getRollBackNonce()
	if rollBackNonce < math.MaxUint64 {
		forkInfoObject.IsDetected = true
		forkInfoObject.Nonce = rollBackNonce
		bfd.SetRollBackNonce(math.MaxUint64)
		return forkInfoObject
	}

	finalCheckpointNonce := bfd.finalCheckpoint().nonce

	bfd.mutHeaders.Lock()
	for nonce, hdrsInfo := range bfd.headers {
		if len(hdrsInfo) == 1 {
			continue
		}
		if nonce <= finalCheckpointNonce {
			continue
		}

		selfHdrInfo = nil
		forkHeaderSlot = math.MaxUint64
		forkHeaderHash = nil
		forkHeaderEpoch = 0
		bfd.maxForkHeaderEpoch = getMaxEpochFromHdrsInfo(hdrsInfo)

		for i := 0; i < len(hdrsInfo); i++ {
			if hdrsInfo[i].state == process.BHProcessed {
				selfHdrInfo = hdrsInfo[i]
				continue
			}

			forkHeaderHash, forkHeaderSlot, forkHeaderEpoch = bfd.computeForkInfo(
				hdrsInfo[i],
				forkHeaderHash,
				forkHeaderSlot,
				forkHeaderEpoch,
			)
		}

		if selfHdrInfo == nil {
			// if current nonce has not been processed yet, then skip and check the next one.
			continue
		}

		if bfd.shouldSignalFork(selfHdrInfo, forkHeaderHash, forkHeaderSlot, forkHeaderEpoch) {
			forkInfoObject.IsDetected = true
			if nonce < forkInfoObject.Nonce {
				forkInfoObject.Nonce = nonce
				forkInfoObject.Slot = forkHeaderSlot
				forkInfoObject.Hash = forkHeaderHash
			}
		}
	}
	bfd.mutHeaders.Unlock()

	return forkInfoObject
}

func getMaxEpochFromHdrsInfo(hdrInfos []*headerInfo) uint32 {
	maxEpoch := uint32(0)
	for _, hdrInfo := range hdrInfos {
		if hdrInfo.epoch > maxEpoch {
			maxEpoch = hdrInfo.epoch
		}
	}
	return maxEpoch
}

func (bfd *baseForkDetector) computeForkInfo(
	hdrInfo *headerInfo,
	lastForkHash []byte,
	lastForkSlot uint64,
	lastForkEpoch uint32,
) ([]byte, uint64, uint32) {

	if hdrInfo.state == process.BHReceivedTooLate && bfd.highestNonceReceived() > hdrInfo.nonce {
		return lastForkHash, lastForkSlot, lastForkEpoch
	}

	currentForkSlot := hdrInfo.slot
	if hdrInfo.state == process.BHNotarized {
		currentForkSlot = process.MinForkSlot
	} else {
		if hdrInfo.epoch < bfd.maxForkHeaderEpoch {
			return lastForkHash, lastForkSlot, lastForkEpoch
		}
	}

	if currentForkSlot < lastForkSlot {
		return hdrInfo.hash, currentForkSlot, hdrInfo.epoch
	}

	lowerHashForSameSlot := currentForkSlot == lastForkSlot &&
		bytes.Compare(hdrInfo.hash, lastForkHash) < 0
	if lowerHashForSameSlot {
		return hdrInfo.hash, currentForkSlot, hdrInfo.epoch
	}

	return lastForkHash, lastForkSlot, lastForkEpoch
}

func (bfd *baseForkDetector) shouldSignalFork(
	headerInfo *headerInfo,
	lastForkHash []byte,
	lastForkSlot uint64,
	lastForkEpoch uint32,
) bool {
	sameHash := bytes.Equal(headerInfo.hash, lastForkHash)
	if sameHash {
		return false
	}

	if lastForkSlot != process.MinForkSlot {
		if headerInfo.epoch > lastForkEpoch {
			log.Trace("shouldSignalFork epoch change false")
			return false
		}

		if headerInfo.epoch < lastForkEpoch {
			log.Trace("shouldSignalFork epoch change true")
			return true
		}
	}

	higherHashForSameSlot := headerInfo.slot == lastForkSlot &&
		bytes.Compare(headerInfo.hash, lastForkHash) > 0
	higherNonceReceived := bfd.highestNonceReceived() > headerInfo.nonce
	shouldSignalFork := headerInfo.slot > lastForkSlot || (higherHashForSameSlot && !higherNonceReceived)
	if shouldSignalFork {
		log.Trace("shouldSignalFork in", "slot", headerInfo.slot, "lastForkSlot", lastForkSlot, "higherHashForSameSlot", higherHashForSameSlot, "higherNonceReceived", higherNonceReceived)
	}

	return shouldSignalFork
}

func (bfd *baseForkDetector) isHeaderReceivedTooLate(
	header data.HeaderHandler,
	state process.BlockHeaderState,
	finality int64,
) bool {
	if state == process.BHProcessed {
		return false
	}

	// This condition would avoid a stuck situation, when shards would set as final, block with nonce n received from
	// meta-chain, because they also received n+1. In the same time meta-chain would be reverted to an older block with
	// nonce n received it with latency but before n+1. Actually this condition would reject these older blocks.
	isHeaderReceivedTooLate := tools.SafeU64ToI64(header.GetSlot()) < bfd.slotManager.Index()-finality

	return isHeaderReceivedTooLate
}

func (bfd *baseForkDetector) isConsensusStuck() bool {
	if bfd.lastSlotWithForcedFork() == bfd.slotManager.Index() {
		return false
	}

	if bfd.isSyncing() {
		return false
	}

	currentSlot := tools.SafeI64ToU64(bfd.slotManager.Index())
	lastCheckpointSlot := bfd.lastCheckpoint().slot
	slotsDifference, err := tools.SafeSubUint64(currentSlot, lastCheckpointSlot)
	if err != nil {
		// The last checkpoint is ahead of our own slot index, so no slots have
		// elapsed since it. Subtracting would wrap around on uint64 and report an
		// enormous lag, which clears the threshold below and forces a rollback of
		// a block that was just committed. checkBlockBasicValidity deliberately
		// accepts headers one slot ahead of the local index, so a node whose clock
		// trails its peers can reach this state without any peer misbehaving.
		bfd.warnOnceCheckpointAheadOfSlotIndex(currentSlot, lastCheckpointSlot)
		return false
	}

	if slotsDifference <= process.MaxSlotsWithoutCommittedBlock {
		return false
	}

	if !process.IsInProperSlot(bfd.slotManager.Index()) {
		return false
	}

	return true
}

// warnOnceCheckpointAheadOfSlotIndex surfaces the clock-trails-tip state. While
// it lasts, the node is otherwise silent: incoming headers are dropped below the
// default log level because their slot exceeds the local index, the
// bootstrapper's slot lag reads zero so its stall warning cannot fire, and the
// node keeps reporting NsSynchronized. After an NTP step-back or a VM resume
// this can hold for a long time, so this line is the only operator-visible
// signal. Throttled to once per slot because CheckFork runs on every sync-loop
// iteration while the node is not synchronized.
//
// The format does happen under mutNodeState, since CheckFork's only production
// caller holds it. That is accepted here, unlike for the stall warning that was
// moved out of that lock: the throttle caps this at one format per slot, and
// the state has no out-of-lock observer to move it to.
func (bfd *baseForkDetector) warnOnceCheckpointAheadOfSlotIndex(currentSlot uint64, checkpointSlot uint64) {
	slotIndex := bfd.slotManager.Index()
	if bfd.lastCheckpointAheadWarnSlot.Swap(slotIndex) == slotIndex {
		return
	}

	log.Warn("last checkpoint is ahead of the local slot index, node clock appears to trail the network",
		"local slot index", currentSlot,
		"checkpoint slot", checkpointSlot,
		"checkpoint nonce", bfd.lastCheckpoint().nonce)
}

func (bfd *baseForkDetector) isSyncing() bool {
	// noncesDifference is used for comparison, allow the difference to be negative
	noncesDifference := tools.SafeU64ToI64(bfd.ProbableHighestNonce()) - tools.SafeU64ToI64(bfd.lastCheckpoint().nonce)
	isSyncing := noncesDifference > process.NonceDifferenceWhenSynced
	if isSyncing {
		bfd.tmpStuck = time.Now()
	}
	return isSyncing || time.Now().Before(bfd.tmpStuck.Add(2*time.Second))
}

// GetNotarizedHeaderHash returns the hash of the header with a given nonce, if it has been received with state notarized
func (bfd *baseForkDetector) GetNotarizedHeaderHash(nonce uint64) []byte {
	bfd.mutHeaders.RLock()
	defer bfd.mutHeaders.RUnlock()

	hdrInfos := bfd.headers[nonce]
	for _, hdrInfo := range hdrInfos {
		if hdrInfo.state == process.BHNotarized {
			return hdrInfo.hash
		}
	}

	return nil
}

func (bfd *baseForkDetector) cleanupReceivedHeadersHigherThanNonce(nonce uint64) {
	bfd.mutHeaders.Lock()
	for hdrsNonce, hdrsInfo := range bfd.headers {
		if hdrsNonce <= nonce {
			continue
		}

		preservedHdrsInfo := make([]*headerInfo, 0)

		for _, hdrInfo := range hdrsInfo {
			if hdrInfo.state != process.BHNotarized {
				continue
			}

			preservedHdrsInfo = append(preservedHdrsInfo, hdrInfo)
		}

		if len(preservedHdrsInfo) == 0 {
			delete(bfd.headers, hdrsNonce)
			continue
		}

		bfd.headers[hdrsNonce] = preservedHdrsInfo
	}
	bfd.mutHeaders.Unlock()
}

func (bfd *baseForkDetector) computeGenesisTimeFromHeader(headerHandler data.HeaderHandler) int64 {
	timeDiff := float64(headerHandler.GetSlot()-bfd.genesisSlot) * bfd.slotManager.TimeDuration().Seconds()
	// Check for overflow before converting to int64
	if timeDiff > float64(math.MaxInt64) {
		// Handle the overflow case (returning negative value)
		return -1
	}

	return headerHandler.GetTimestamp() - int64(timeDiff)
}

func (bfd *baseForkDetector) addHeader(
	header data.HeaderHandler,
	headerHash []byte,
	state process.BlockHeaderState,
	selfNotarizedHeaders []data.HeaderHandler,
	selfNotarizedHeadersHashes [][]byte,
	doJobOnBHProcessed func(data.HeaderHandler, []byte, []data.HeaderHandler, [][]byte),
) error {

	err := bfd.checkBlockBasicValidity(header, headerHash)
	if err != nil {
		return err
	}

	bfd.processReceivedBlock(header, headerHash, state, selfNotarizedHeaders, selfNotarizedHeadersHashes, doJobOnBHProcessed)
	return nil
}

func (bfd *baseForkDetector) processReceivedBlock(
	header data.HeaderHandler,
	headerHash []byte,
	state process.BlockHeaderState,
	selfNotarizedHeaders []data.HeaderHandler,
	selfNotarizedHeadersHashes [][]byte,
	doJobOnBHProcessed func(data.HeaderHandler, []byte, []data.HeaderHandler, [][]byte),
) {
	bfd.setHighestNonceReceived(header.GetNonce())

	if state == process.BHProposed {
		return
	}

	isHeaderReceivedTooLate := bfd.isHeaderReceivedTooLate(header, state, process.BlockFinality)
	if isHeaderReceivedTooLate {
		state = process.BHReceivedTooLate
	}

	appended := bfd.append(&headerInfo{
		epoch: header.GetEpoch(),
		nonce: header.GetNonce(),
		slot:  header.GetSlot(),
		hash:  headerHash,
		state: state,
	})
	if !appended {
		return
	}

	if state == process.BHProcessed {
		doJobOnBHProcessed(header, headerHash, selfNotarizedHeaders, selfNotarizedHeadersHashes)
	}

	probableHighestNonce := bfd.computeProbableHighestNonce()
	bfd.setProbableHighestNonce(probableHighestNonce)

	log.Debug("forkDetector.AddHeader",
		"slot", header.GetSlot(),
		"nonce", header.GetNonce(),
		"hash", headerHash,
		"state", state,
		"probable highest nonce", bfd.probableHighestNonce(),
		"last check point nonce", bfd.lastCheckpoint().nonce,
		"final check point nonce", bfd.finalCheckpoint().nonce)
}

// SetFinalToLastCheckpoint sets the final checkpoint to the last checkpoint added in list
func (bfd *baseForkDetector) SetFinalToLastCheckpoint() {
	bfd.setFinalCheckpoint(bfd.lastCheckpoint())
}
