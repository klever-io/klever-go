package sync

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
)

// BaseBootstrap is an alias so tests in the sync_test package can refer to
// the unexported baseBootstrap type by name.
type BaseBootstrap = baseBootstrap

// NewBaseBootstrapForKLC1920Test builds a minimal baseBootstrap wired only
// with the dependencies computeNodeState needs to exercise the KLC-1920
// gossip-ahead-of-probable branch. Internal-only helper, not for production.
func NewBaseBootstrapForKLC1920Test(
	forkDetector process.ForkDetector,
	chainHandler data.ChainHandler,
	slotManager consensus.SlotManager,
	networkWatcher process.NetworkConnectionWatcher,
	statusHandler core.AppStatusHandler,
) *BaseBootstrap {
	return &baseBootstrap{
		forkDetector:       forkDetector,
		chainHandler:       chainHandler,
		slotManager:        slotManager,
		networkWatcher:     networkWatcher,
		statusHandler:      statusHandler,
		syncStateListeners: []func(bool){},
		hasStarted:         true,
	}
}

func (boot *baseBootstrap) IsNodeSynchronized() bool {
	boot.mutNodeState.RLock()
	defer boot.mutNodeState.RUnlock()
	return boot.isNodeSynchronized
}

func (boot *baseBootstrap) HasLastBlock() bool {
	boot.mutNodeState.RLock()
	defer boot.mutNodeState.RUnlock()
	return boot.hasLastBlock
}

func (boot *MetaBootstrap) ReceivedHeaders(header data.HeaderHandler, key []byte) {
	boot.processReceivedHeader(header, key)
}

func (boot *MetaBootstrap) RollBack(revertUsingForkNonce bool) error {
	return boot.rollBack(revertUsingForkNonce)
}

func (bfd *baseForkDetector) GetHeaders(nonce uint64) []*headerInfo {
	bfd.mutHeaders.Lock()
	defer bfd.mutHeaders.Unlock()

	headers := bfd.headers[nonce]

	if headers == nil {
		return nil
	}

	newHeaders := make([]*headerInfo, len(headers))
	copy(newHeaders, headers)

	return newHeaders
}

func (bfd *baseForkDetector) LastCheckpointNonce() uint64 {
	return bfd.lastCheckpoint().nonce
}

func (bfd *baseForkDetector) LastCheckpointSlot() uint64 {
	return bfd.lastCheckpoint().slot
}

func (bfd *baseForkDetector) SetFinalCheckpoint(nonce uint64, slot uint64, hash []byte) {
	bfd.setFinalCheckpoint(&checkpointInfo{nonce: nonce, slot: slot, hash: hash})
}

func (bfd *baseForkDetector) FinalCheckpointNonce() uint64 {
	return bfd.finalCheckpoint().nonce
}

func (bfd *baseForkDetector) FinalCheckpointSlot() uint64 {
	return bfd.finalCheckpoint().slot
}

func (bfd *baseForkDetector) CheckBlockValidity(header *block.Block, headerHash []byte) error {
	return bfd.checkBlockBasicValidity(header, headerHash)
}

func (bfd *baseForkDetector) RemovePastHeaders() {
	bfd.removePastHeaders()
}

func (bfd *baseForkDetector) RemoveInvalidReceivedHeaders() {
	bfd.removeInvalidReceivedHeaders()
}

func (bfd *baseForkDetector) ComputeProbableHighestNonce() uint64 {
	return bfd.computeProbableHighestNonce()
}

func (bfd *baseForkDetector) IsConsensusStuck() bool {
	return bfd.isConsensusStuck()
}

func (hi *headerInfo) Hash() []byte {
	return hi.hash
}

func (hi *headerInfo) GetBlockHeaderState() process.BlockHeaderState {
	return hi.state
}

func (boot *MetaBootstrap) NotifySyncStateListeners() {
	isNodeSynchronized := boot.GetNodeState() == core.NsSynchronized
	boot.notifySyncStateListeners(isNodeSynchronized)
}

func (boot *MetaBootstrap) SyncStateListeners() []func(bool) {
	return boot.syncStateListeners
}

func (boot *MetaBootstrap) SetForkNonce(nonce uint64) {
	boot.forkInfo.Nonce = nonce
}

func (boot *MetaBootstrap) IsForkDetected() bool {
	return boot.forkInfo.IsDetected
}

func (boot *MetaBootstrap) GetNotarizedInfo(
	lastNotarized map[uint32]*HdrInfo,
	finalNotarized map[uint32]*HdrInfo,
	blockWithLastNotarized map[uint32]uint64,
	blockWithFinalNotarized map[uint32]uint64,
	startNonce uint64,
) *notarizedInfo {
	return &notarizedInfo{
		lastNotarized:           lastNotarized,
		finalNotarized:          finalNotarized,
		blockWithLastNotarized:  blockWithLastNotarized,
		blockWithFinalNotarized: blockWithFinalNotarized,
		startNonce:              startNonce,
	}
}

func (boot *baseBootstrap) ProcessReceivedHeader(headerHandler data.HeaderHandler, headerHash []byte) {
	boot.processReceivedHeader(headerHandler, headerHash)
}

func (bfd *baseForkDetector) IsHeaderReceivedTooLate(header data.HeaderHandler, state process.BlockHeaderState, finality int64) bool {
	return bfd.isHeaderReceivedTooLate(header, state, finality)
}

func (bfd *baseForkDetector) SetProbableHighestNonce(nonce uint64) {
	bfd.setProbableHighestNonce(nonce)
}

func (bfd *baseForkDetector) AddCheckPoint(slot uint64, nonce uint64, hash []byte) {
	bfd.addCheckpoint(&checkpointInfo{slot: slot, nonce: nonce, hash: hash})
}

func (bfd *baseForkDetector) ComputeGenesisTimeFromHeader(headerHandler data.HeaderHandler) int64 {
	return bfd.computeGenesisTimeFromHeader(headerHandler)
}

func (boot *baseBootstrap) InitNotarizedMap() map[uint32]*HdrInfo {
	return make(map[uint32]*HdrInfo)
}

func (boot *baseBootstrap) SetNotarizedMap(notarizedMap map[uint32]*HdrInfo, shardId uint32, nonce uint64, hash []byte) {
	hdrInfo, ok := notarizedMap[shardId]
	if !ok {
		notarizedMap[shardId] = &HdrInfo{Nonce: nonce, Hash: hash}
		return
	}

	hdrInfo.Nonce = nonce
	hdrInfo.Hash = hash
}

func (boot *baseBootstrap) SetNodeStateCalculated(state bool) {
	boot.mutNodeState.Lock()
	boot.isNodeStateCalculated = state
	boot.mutNodeState.Unlock()
}

func (boot *baseBootstrap) ComputeNodeState() {
	boot.computeNodeState()
}

func (boot *baseBootstrap) DoJobOnSyncBlockFail(headerHandler data.HeaderHandler, err error) {
	boot.doJobOnSyncBlockFail(headerHandler, err)
}

func (boot *baseBootstrap) SetNumSyncedWithErrorsForNonce(nonce uint64, numSyncedWithErrors uint32) {
	boot.mutNonceSyncedWithErrors.Lock()
	boot.mapNonceSyncedWithErrors[nonce] = numSyncedWithErrors
	boot.mutNonceSyncedWithErrors.Unlock()
}

func (boot *baseBootstrap) GetNumSyncedWithErrorsForNonce(nonce uint64) uint32 {
	boot.mutNonceSyncedWithErrors.RLock()
	numSyncedWithErrors := boot.mapNonceSyncedWithErrors[nonce]
	boot.mutNonceSyncedWithErrors.RUnlock()

	return numSyncedWithErrors
}

func (boot *baseBootstrap) GetMapNonceSyncedWithErrorsLen() int {
	boot.mutNonceSyncedWithErrors.RLock()
	mapNonceSyncedWithErrorsLen := len(boot.mapNonceSyncedWithErrors)
	boot.mutNonceSyncedWithErrors.RUnlock()

	return mapNonceSyncedWithErrorsLen
}

func (boot *baseBootstrap) CleanNoncesSyncedWithErrorsBehindFinal() {
	boot.cleanNoncesSyncedWithErrorsBehindFinal()
}
