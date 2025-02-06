package bls

import (
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/consensus/slot"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/tools/check"
)

// factory defines the data needed by this factory to create all the subslots and give them their specific
// functionality
type factory struct {
	consensusCore  slot.ConsensusCoreHandler
	consensusState *slot.ConsensusState
	worker         slot.WorkerHandler

	appStatusHandler core.AppStatusHandler
	indexer          slot.ConsensusDataIndexer
	chainID          []byte
	currentPid       core.PeerID
}

// NewSubslotsFactory creates a new consensusState object
func NewSubslotsFactory(
	consensusDataContainer slot.ConsensusCoreHandler,
	consensusState *slot.ConsensusState,
	worker slot.WorkerHandler,
	chainID []byte,
	currentPid core.PeerID,
) (*factory, error) {
	err := checkNewFactoryParams(
		consensusDataContainer,
		consensusState,
		worker,
		chainID,
	)
	if err != nil {
		return nil, err
	}

	fct := factory{
		consensusCore:    consensusDataContainer,
		consensusState:   consensusState,
		worker:           worker,
		appStatusHandler: statusHandler.NewNilStatusHandler(),
		chainID:          chainID,
		currentPid:       currentPid,
	}

	return &fct, nil
}

func checkNewFactoryParams(
	container slot.ConsensusCoreHandler,
	state *slot.ConsensusState,
	worker slot.WorkerHandler,
	chainID []byte,
) error {
	err := slot.ValidateConsensusCore(container)
	if err != nil {
		return err
	}
	if state == nil {
		return slot.ErrNilConsensusState
	}
	if worker == nil || worker.IsInterfaceNil() {
		return slot.ErrNilWorker
	}
	if len(chainID) == 0 {
		return slot.ErrInvalidChainID
	}

	return nil
}

// SetAppStatusHandler method will update the value of the factory's appStatusHandler
func (fct *factory) SetAppStatusHandler(ash core.AppStatusHandler) error {
	if check.IfNil(ash) {
		return slot.ErrNilAppStatusHandler
	}
	fct.appStatusHandler = ash

	return fct.worker.SetAppStatusHandler(ash)
}

// SetIndexer method will update the value of the factory's indexer
func (fct *factory) SetIndexer(indexer slot.ConsensusDataIndexer) {
	fct.indexer = indexer
}

// GenerateSubslots will generate the subslots used in BLS Cns
func (fct *factory) GenerateSubslots() error {
	fct.initConsensusThreshold()
	fct.consensusCore.Chronology().RemoveAllSubslots()
	fct.worker.RemoveAllReceivedMessagesCalls()

	err := fct.generateStartSlotSubslot()
	if err != nil {
		return err
	}

	err = fct.generateBlockSubslot()
	if err != nil {
		return err
	}

	err = fct.generateSignatureSubslot()
	if err != nil {
		return err
	}

	err = fct.generateEndSlotSubslot()
	if err != nil {
		return err
	}

	return nil
}

func (fct *factory) getTimeDuration() time.Duration {
	return fct.consensusCore.SlotManager().TimeDuration()
}

func (fct *factory) generateStartSlotSubslot() error {
	subslot, err := slot.NewSubslot(
		-1,
		SrStartSlot,
		SrBlock,
		int64(float64(fct.getTimeDuration())*srStartStartTime),
		int64(float64(fct.getTimeDuration())*srStartEndTime),
		getSubslotName(SrStartSlot),
		fct.consensusState,
		fct.worker.GetConsensusStateChangedChannel(),
		fct.worker.ExecuteStoredMessages,
		fct.consensusCore,
		fct.chainID,
		fct.currentPid,
	)
	if err != nil {
		return err
	}

	err = subslot.SetAppStatusHandler(fct.appStatusHandler)
	if err != nil {
		return err
	}

	subslotStartSlot, err := NewSubslotStartSlot(
		subslot,
		fct.worker.Extend,
		processingThresholdPercent,
		fct.worker.ExecuteStoredMessages,
		fct.worker.ResetConsensusMessages,
	)
	if err != nil {
		return err
	}

	subslotStartSlot.SetIndexer(fct.indexer)

	fct.consensusCore.Chronology().AddSubslot(subslotStartSlot)

	return nil
}

func (fct *factory) generateBlockSubslot() error {
	subslot, err := slot.NewSubslot(
		SrStartSlot,
		SrBlock,
		SrSignature,
		int64(float64(fct.getTimeDuration())*srBlockStartTime),
		int64(float64(fct.getTimeDuration())*srBlockEndTime),
		getSubslotName(SrBlock),
		fct.consensusState,
		fct.worker.GetConsensusStateChangedChannel(),
		fct.worker.ExecuteStoredMessages,
		fct.consensusCore,
		fct.chainID,
		fct.currentPid,
	)
	if err != nil {
		return err
	}

	err = subslot.SetAppStatusHandler(fct.appStatusHandler)
	if err != nil {
		return err
	}

	subslotBlock, err := NewSubslotBlock(
		subslot,
		fct.worker.Extend,
		processingThresholdPercent,
	)
	if err != nil {
		return err
	}

	fct.worker.AddReceivedMessageCall(MtBlockHeader, subslotBlock.receivedBlockHeader)
	fct.consensusCore.Chronology().AddSubslot(subslotBlock)

	return nil
}

func (fct *factory) generateSignatureSubslot() error {
	subslot, err := slot.NewSubslot(
		SrBlock,
		SrSignature,
		SrEndSlot,
		int64(float64(fct.getTimeDuration())*srSignatureStartTime),
		int64(float64(fct.getTimeDuration())*srSignatureEndTime),
		getSubslotName(SrSignature),
		fct.consensusState,
		fct.worker.GetConsensusStateChangedChannel(),
		fct.worker.ExecuteStoredMessages,
		fct.consensusCore,
		fct.chainID,
		fct.currentPid,
	)
	if err != nil {
		return err
	}

	subslotSignatureObject, err := NewSubslotSignature(
		subslot,
		fct.worker.Extend,
	)
	if err != nil {
		return err
	}

	err = subslotSignatureObject.SetAppStatusHandler(fct.appStatusHandler)
	if err != nil {
		return err
	}

	fct.worker.AddReceivedMessageCall(MtSignature, subslotSignatureObject.receivedSignature)
	fct.consensusCore.Chronology().AddSubslot(subslotSignatureObject)

	return nil
}

func (fct *factory) generateEndSlotSubslot() error {
	subslot, err := slot.NewSubslot(
		SrSignature,
		SrEndSlot,
		-1,
		int64(float64(fct.getTimeDuration())*srEndStartTime),
		int64(float64(fct.getTimeDuration())*srEndEndTime),
		getSubslotName(SrEndSlot),
		fct.consensusState,
		fct.worker.GetConsensusStateChangedChannel(),
		fct.worker.ExecuteStoredMessages,
		fct.consensusCore,
		fct.chainID,
		fct.currentPid,
	)
	if err != nil {
		return err
	}

	subslotEndSlotObject, err := NewSubslotEndSlot(
		subslot,
		fct.worker.Extend,
		slot.MaxThresholdPercent,
		fct.worker.DisplayStatistics,
	)
	if err != nil {
		return err
	}

	err = subslotEndSlotObject.SetAppStatusHandler(fct.appStatusHandler)
	if err != nil {
		return err
	}

	fct.worker.AddReceivedMessageCall(MtBlockHeaderFinalInfo, subslotEndSlotObject.receivedBlockHeaderFinalInfo)
	fct.worker.AddReceivedHeaderHandler(subslotEndSlotObject.receivedHeader)
	fct.consensusCore.Chronology().AddSubslot(subslotEndSlotObject)

	return nil
}

func (fct *factory) initConsensusThreshold() {
	pBFTThreshold := consensus.GetPBFTThreshold(fct.consensusState.ConsensusGroupSize())
	pBFTFallbackThreshold := consensus.GetPBFTFallbackThreshold(fct.consensusState.ConsensusGroupSize())
	fct.consensusState.SetThreshold(SrBlock, 1)
	fct.consensusState.SetThreshold(SrSignature, pBFTThreshold)
	fct.consensusState.SetFallbackThreshold(SrBlock, 1)
	fct.consensusState.SetFallbackThreshold(SrSignature, pBFTFallbackThreshold)
}

// IsInterfaceNil returns true if there is no value under the interface
func (fct *factory) IsInterfaceNil() bool {
	return fct == nil
}
