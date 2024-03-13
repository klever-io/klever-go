package trigger

import "github.com/klever-io/klever-go/core/closing"

const HardforkTriggerString = hardforkTriggerString
const PayloadSeparator = dataSeparator
const HardforkGracePeriod = hardforkGracePeriod
const MinimumEpochForHardfork = minimumEpochForHardfork
const DeltaSlotsForForcedEpoch = deltaSlotsForForcedEpoch

func (t *trigger) SetTimeHandler(handler func() int64) {
	t.getTimestampHandler = handler
}

func (t *trigger) Closers() []closing.Closer {
	t.mutClosers.RLock()
	defer t.mutClosers.RUnlock()

	return t.closers
}

func (t *trigger) TriggerExecuting() bool {
	return t.triggerExecuting
}

func (t *trigger) Epoch() uint32 {
	return t.epoch
}

func (t *trigger) SetReceivedExecutingEpoch(
	triggerReceived bool,
	triggerExecuting bool,
	shouldTriggerFromEpochChange bool,
	epoch uint32,
) {
	t.triggerExecuting = triggerExecuting
	t.shouldTriggerFromEpochChange = shouldTriggerFromEpochChange
	t.triggerReceived = triggerReceived
	t.epoch = epoch
}

func (t *trigger) ComputeTriggerStartOfEpoch(epoch uint32) bool {
	return t.computeTriggerStartOfEpoch(epoch)
}

func (ish *importStartHandler) SetVersion(version string) {
	ish.currentVersion = version
}
