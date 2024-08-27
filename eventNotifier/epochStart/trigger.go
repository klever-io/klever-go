package epochStart

import (
	"bytes"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/klever-io/klever-go/common"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/display"
	"github.com/klever-io/klever-go/tools/marshal"
)

const disabledSlotForForceEpochStart = math.MaxUint64
const minSlotsToTrigger = uint64(5)

// ArgsNewEpochStartTrigger defines struct needed to create a new start of epoch trigger
type ArgsNewEpochStartTrigger struct {
	GenesisTime        time.Time
	Epoch              uint32
	EpochStartSlot     uint64
	SlotsPerEpoch      uint64
	EpochStartNotifier eventNotifier.Notifier
	Marshalizer        marshal.Marshalizer
	Hasher             hashing.Hasher
	Storage            retriever.StorageService
	ForkController     core.ForkController
}

type trigger struct {
	isEpochStart               bool
	epoch                      uint32
	epochStartBlock            *block.Block
	currentSlot                uint64
	epochFinalityAttestingSlot uint64
	currEpochStartSlot         uint64
	prevEpochStartSlot         uint64
	nextEpochStartSlot         uint64
	slotsPerEpoch              uint64
	epochStartBlockHash        []byte
	triggerStateKey            []byte
	epochStartTime             time.Time
	mutTrigger                 sync.RWMutex
	epochStartNotifier         eventNotifier.Notifier
	headerStorage              storage.Storer
	triggerStorage             storage.Storer
	marshalizer                marshal.Marshalizer
	hasher                     hashing.Hasher
	appStatusHandler           core.AppStatusHandler
	forkController             core.ForkController
}

// NewEpochStartTrigger creates a trigger for start of epoch
func NewEpochStartTrigger(args *ArgsNewEpochStartTrigger) (*trigger, error) {
	if args == nil {
		return nil, eventNotifier.ErrNilArgsNewEpochStartTrigger
	}
	if check.IfNil(args.EpochStartNotifier) {
		return nil, eventNotifier.ErrNilEpochStartNotifier
	}
	if check.IfNil(args.Marshalizer) {
		return nil, eventNotifier.ErrNilMarshalizer
	}
	if check.IfNil(args.Storage) {
		return nil, eventNotifier.ErrNilStorageService
	}
	if check.IfNil(args.Hasher) {
		return nil, eventNotifier.ErrNilHasher
	}
	if check.IfNil(args.ForkController) {
		return nil, common.ErrNilForkController
	}

	triggerStorage := args.Storage.GetStorer(retriever.BootstrapUnit)
	if check.IfNil(triggerStorage) {
		return nil, eventNotifier.ErrNilTriggerStorage
	}

	blockStorage := args.Storage.GetStorer(retriever.BlockUnit)
	if check.IfNil(triggerStorage) {
		return nil, eventNotifier.ErrNilBlockStorage
	}

	trigggerStateKey := core.TriggerRegistryInitialKeyPrefix + fmt.Sprintf("%d", args.Epoch)
	trig := &trigger{
		triggerStateKey:            []byte(trigggerStateKey),
		slotsPerEpoch:              args.SlotsPerEpoch,
		epochStartTime:             args.GenesisTime,
		currEpochStartSlot:         args.EpochStartSlot,
		prevEpochStartSlot:         args.EpochStartSlot,
		epoch:                      args.Epoch,
		mutTrigger:                 sync.RWMutex{},
		epochFinalityAttestingSlot: args.EpochStartSlot,
		epochStartNotifier:         args.EpochStartNotifier,
		headerStorage:              blockStorage,
		triggerStorage:             triggerStorage,
		marshalizer:                args.Marshalizer,
		hasher:                     args.Hasher,
		epochStartBlock:            &block.Block{Header: &block.BlockHeader{}},
		appStatusHandler:           &statusHandler.NilStatusHandler{},
		nextEpochStartSlot:         disabledSlotForForceEpochStart,
		forkController:             args.ForkController,
	}

	err := trig.saveState(trig.triggerStateKey)
	if err != nil {
		return nil, err
	}

	return trig, nil
}

// IsEpochStart return true if conditions are fulfilled for start of epoch
func (t *trigger) IsEpochStart() bool {
	t.mutTrigger.RLock()
	defer t.mutTrigger.RUnlock()

	return t.isEpochStart
}

// EpochStartSlot returns the start slot of the current epoch
func (t *trigger) EpochStartSlot() uint64 {
	t.mutTrigger.RLock()
	defer t.mutTrigger.RUnlock()

	return t.currEpochStartSlot
}

// PrevEpochStartSlot returns the start slot of the previous epoch
func (t *trigger) PrevEpochStartSlot() uint64 {
	t.mutTrigger.RLock()
	defer t.mutTrigger.RUnlock()

	return t.prevEpochStartSlot
}

// EpochFinalityAttestingSlot returns the slot when epoch start block was finalized
func (t *trigger) EpochFinalityAttestingSlot() uint64 {
	t.mutTrigger.RLock()
	defer t.mutTrigger.RUnlock()

	return t.epochFinalityAttestingSlot
}

// ForceEpochStart sets the slot at which the new epoch will start
func (t *trigger) ForceEpochStart(slot uint64) {
	t.mutTrigger.Lock()
	defer t.mutTrigger.Unlock()

	t.nextEpochStartSlot = slot
	if t.nextEpochStartSlot > t.currEpochStartSlot+uint64(t.slotsPerEpoch) {
		t.nextEpochStartSlot = disabledSlotForForceEpochStart
		log.Debug("can not force epoch start because the resulting slot is in the next epoch")

		return
	}

	log.Debug("set new epoch start slot", "slot", t.nextEpochStartSlot)
}

// Update processes changes in the trigger
func (t *trigger) Update(slot uint64, nonce uint64) {
	t.mutTrigger.Lock()
	defer t.mutTrigger.Unlock()

	t.currentSlot = slot

	if t.isEpochStart {
		return
	}

	isNormalEpochStart := t.currentSlot >= t.currEpochStartSlot+t.slotsPerEpoch
	if t.forkController.EnableSmartContracts() {
		isNormalEpochStart = t.currentSlot >= (t.currEpochStartSlot-(t.currEpochStartSlot%t.slotsPerEpoch))+t.slotsPerEpoch
	}

	isWithEarlyEndOfEpoch := t.currentSlot >= t.nextEpochStartSlot
	hasMinimumForgeToTrigger := nonce > t.epochStartBlock.Header.Nonce+minSlotsToTrigger
	shouldTriggerEpochStart := (isNormalEpochStart || isWithEarlyEndOfEpoch) && hasMinimumForgeToTrigger
	if shouldTriggerEpochStart {
		t.epoch++
		t.isEpochStart = true
		t.prevEpochStartSlot = t.currEpochStartSlot
		t.currEpochStartSlot = t.currentSlot

		msg := fmt.Sprintf("EPOCH %d BEGINS IN SLOT (%d)", t.epoch, t.currentSlot)
		log.Debug(display.Headline(msg, "", "#"))
		log.Debug("trigger.Update", "isEpochStart", t.isEpochStart)
		logger.SetCorrelationEpoch(t.epoch)
		t.nextEpochStartSlot = disabledSlotForForceEpochStart
	}
}

// SetProcessed sets start of epoch to false and cleans underlying structure
func (t *trigger) SetProcessed(header data.HeaderHandler) {
	t.mutTrigger.Lock()
	defer t.mutTrigger.Unlock()

	metaBlock, ok := header.(*block.Block)
	if !ok {
		return
	}
	if !metaBlock.GetIsEpochStart() {
		return
	}

	metaBuff, errNotCritical := t.marshalizer.Marshal(metaBlock)
	if errNotCritical != nil {
		log.Debug("SetProcessed marshal", "error", errNotCritical.Error())
	}

	metaHeaderBuff, errNotCritical := t.marshalizer.Marshal(metaBlock.Header)
	if errNotCritical != nil {
		log.Debug("SetProcessed marshal header", "error", errNotCritical.Error())
	}

	t.appStatusHandler.SetUInt64Value(core.MetricSlotAtEpochStart, metaBlock.GetSlot())
	t.appStatusHandler.SetUInt64Value(core.MetricNonceAtEpochStart, metaBlock.GetNonce())

	metaHash := t.hasher.Compute(string(metaHeaderBuff))

	t.currEpochStartSlot = metaBlock.GetSlot()
	t.epoch = metaBlock.GetEpoch()
	t.isEpochStart = false
	t.currentSlot = metaBlock.GetSlot()
	t.epochStartBlock = metaBlock
	t.epochStartBlockHash = metaHash

	t.epochStartNotifier.NotifyAllPrepare(metaBlock)
	t.epochStartNotifier.NotifyAll(metaBlock)

	t.saveCurrentState(metaBlock.GetSlot())

	log.Debug("trigger.SetProcessed", "isEpochStart", t.isEpochStart)

	epochStartIdentifier := core.EpochStartIdentifier(metaBlock.GetEpoch())
	errNotCritical = t.triggerStorage.Put([]byte(epochStartIdentifier), metaBuff)
	if errNotCritical != nil {
		log.Warn("SetProcessed put into triggerStorage", "error", errNotCritical.Error())
	}

	errNotCritical = t.headerStorage.Put([]byte(epochStartIdentifier), metaBuff)
	if errNotCritical != nil {
		log.Warn("SetProcessed put into metaHdrStorage", "error", errNotCritical.Error())
	}
}

// SetFinalityAttestingSlot sets the slot which finalized the start of epoch block
func (t *trigger) SetFinalityAttestingSlot(slot uint64) {
	t.mutTrigger.Lock()
	defer t.mutTrigger.Unlock()

	if slot > t.currEpochStartSlot {
		t.epochFinalityAttestingSlot = slot
		t.saveCurrentState(slot)
		t.epochStartNotifier.NotifyEpochChangeConfirmed(t.epoch)
	}
}

// RevertStateToBlock will revert the state of the trigger to the current block
func (t *trigger) RevertStateToBlock(header data.HeaderHandler) error {
	if check.IfNil(header) {
		return eventNotifier.ErrNilHeaderHandler
	}

	if header.GetIsEpochStart() {
		log.Debug("RevertStateToBlock with epoch start block called")
		t.SetProcessed(header)
		return nil
	}

	t.mutTrigger.RLock()
	prevMeta := t.epochStartBlock
	t.mutTrigger.RUnlock()

	if prevMeta == nil || prevMeta.Header == nil {
		return eventNotifier.ErrNilStratEpochHeader
	}

	currentHeaderHash, err := tools.CalculateHash(t.marshalizer, t.hasher, header.GetBlockHeader())
	if err != nil {
		log.Warn("RevertStateToBlock error on hashing", "error", err)
		return err
	}

	if !bytes.Equal(prevMeta.GetParentHash(), currentHeaderHash) {
		return nil
	}

	log.Debug("RevertStateToBlock to revert behind epoch start block is called")

	err = t.revert(prevMeta)
	if err != nil {
		return err
	}

	t.mutTrigger.Lock()
	t.currentSlot = header.GetSlot()
	t.mutTrigger.Unlock()

	return nil
}

// SetAppStatusHandler will set the satus handler for the trigger
func (t *trigger) SetAppStatusHandler(handler core.AppStatusHandler) error {
	if check.IfNil(handler) {
		return eventNotifier.ErrNilStatusHandler
	}

	t.appStatusHandler = handler
	return nil
}

func (t *trigger) revert(header data.HeaderHandler) error {
	if check.IfNil(header) || !header.GetIsEpochStart() || header.GetEpoch() == 0 {
		return nil
	}

	metaHdr, ok := header.(*block.Block)
	if !ok {
		log.Warn("wrong type assertion in Revert metachain trigger")
		return eventNotifier.ErrWrongTypeAssertion
	}

	t.mutTrigger.Lock()
	defer t.mutTrigger.Unlock()

	prevEpochStartIdentifier := core.EpochStartIdentifier(metaHdr.GetEpoch() - 1)
	epochStartBlockBuff, err := t.headerStorage.SearchFirst([]byte(prevEpochStartIdentifier))
	if err != nil {
		log.Warn("Revert get previous meta from storage", "error", err)
		return err
	}

	epochStartBlock := &block.Block{}
	err = t.marshalizer.Unmarshal(epochStartBlock, epochStartBlockBuff)
	if err != nil {
		log.Warn("Revert unmarshal previous meta", "error", err)
		return err
	}

	epochStartIdentifier := core.EpochStartIdentifier(metaHdr.GetEpoch())
	errNotCritical := t.triggerStorage.Remove([]byte(epochStartIdentifier))
	if errNotCritical != nil {
		log.Debug("Revert remove from triggerStorage", "error", errNotCritical.Error())
	}

	errNotCritical = t.headerStorage.Remove([]byte(epochStartIdentifier))
	if errNotCritical != nil {
		log.Debug("Revert remove from headerStorage", "error", errNotCritical.Error())
	}

	t.currEpochStartSlot = epochStartBlock.GetSlot()
	t.epoch = epochStartBlock.GetEpoch()
	t.isEpochStart = false
	t.epochStartBlock = epochStartBlock

	log.Debug("trigger.revert",
		"isEpochStart", t.isEpochStart,
		"epoch", t.epoch,
		"epochStartSlot", t.currEpochStartSlot)

	return nil
}

// Epoch return the current epoch
func (t *trigger) Epoch() uint32 {
	t.mutTrigger.RLock()
	defer t.mutTrigger.RUnlock()

	return t.epoch
}

// RequestEpochStartIfNeeded request the needed epoch start block if metablock with new epoch was received
func (t *trigger) RequestEpochStartIfNeeded(_ data.HeaderHandler) {
}

// EpochStartHdrHash returns the announcing header hash which created the new epoch
func (t *trigger) EpochStartHdrHash() []byte {
	return t.epochStartBlockHash
}

// GetSavedStateKey returns the last saved trigger state key
func (t *trigger) GetSavedStateKey() []byte {
	return t.triggerStateKey
}

// SetEpochStartHdrHash sets the epoch start header has
func (t *trigger) SetEpochStartHdrHash(metaHdrHash []byte) {
	t.epochStartBlockHash = metaHdrHash
}

// SetCurrentEpochStartSlot sets the slot when the current epoch started
func (t *trigger) SetCurrentEpochStartSlot(slot uint64) {
	t.mutTrigger.Lock()
	t.currEpochStartSlot = slot
	t.currentSlot = slot
	t.saveCurrentState(slot)
	t.mutTrigger.Unlock()
}

// Close will close the endless running go routine
func (t *trigger) Close() error {
	return nil
}

// IsInterfaceNil return true if underlying object is nil
func (t *trigger) IsInterfaceNil() bool {
	return t == nil
}

// needs to be called under locked mutex
func (t *trigger) saveCurrentState(slot uint64) {
	t.triggerStateKey = []byte(fmt.Sprint(slot))
	err := t.saveState(t.triggerStateKey)
	if err != nil {
		log.Warn("error saving trigger state", "error", err, "key", t.triggerStateKey)
	}
}
