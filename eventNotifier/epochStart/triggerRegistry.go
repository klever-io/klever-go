package epochStart

import (
	"encoding/json"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/block"
)

// TriggerRegistry holds the data required to correctly initialize the trigger when booting from saved state
type TriggerRegistry struct {
	Epoch                      uint32
	CurrentSlot                uint64
	EpochFinalityAttestingSlot uint64
	CurrEpochStartSlot         uint64
	PrevEpochStartSlot         uint64
	EpochStartBlockHash        []byte
	EpochStartBlock            *block.Block
}

// LoadState loads into trigger the saved state
func (t *trigger) LoadState(key []byte) error {
	trigInternalKey := append([]byte(core.TriggerRegistryKeyPrefix), key...)
	log.Debug("getting start of epoch trigger state", "key", trigInternalKey)

	data, err := t.triggerStorage.Get(trigInternalKey)
	if err != nil {
		return err
	}

	state := &TriggerRegistry{}
	err = json.Unmarshal(data, state)
	if err != nil {
		return err
	}

	t.mutTrigger.Lock()
	t.triggerStateKey = key
	t.currentSlot = state.CurrentSlot
	t.epochFinalityAttestingSlot = state.EpochFinalityAttestingSlot
	t.currEpochStartSlot = state.CurrEpochStartSlot
	t.prevEpochStartSlot = state.PrevEpochStartSlot
	t.epoch = state.Epoch
	t.epochStartBlockHash = state.EpochStartBlockHash
	t.epochStartBlock = state.EpochStartBlock
	t.mutTrigger.Unlock()

	return nil
}

// saveState saves the trigger state. Needs to be called under mutex
func (t *trigger) saveState(key []byte) error {
	registry := &TriggerRegistry{}
	registry.CurrentSlot = t.currentSlot
	registry.EpochFinalityAttestingSlot = t.epochFinalityAttestingSlot
	registry.CurrEpochStartSlot = t.currEpochStartSlot
	registry.PrevEpochStartSlot = t.prevEpochStartSlot
	registry.Epoch = t.epoch
	registry.EpochStartBlockHash = t.epochStartBlockHash
	registry.EpochStartBlock = t.epochStartBlock
	data, err := json.Marshal(registry)
	if err != nil {
		return err
	}

	trigInternalKey := append([]byte(core.TriggerRegistryKeyPrefix), key...)
	log.Debug("saving start of epoch trigger state", "key", trigInternalKey)

	return t.triggerStorage.Put(trigInternalKey, data)
}
