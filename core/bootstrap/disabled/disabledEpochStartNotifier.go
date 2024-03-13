package disabled

import (
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/eventNotifier"
)

// EpochStartNotifier -
type EpochStartNotifier struct {
}

// NewEpochStartNotifier returns the disabled epoch start notifier
func NewEpochStartNotifier() *EpochStartNotifier {
	return &EpochStartNotifier{}
}

// RegisterHandler -
func (desn *EpochStartNotifier) RegisterHandler(_ eventNotifier.ActionHandler) {
}

// UnregisterHandler -
func (desn *EpochStartNotifier) UnregisterHandler(_ eventNotifier.ActionHandler) {
}

// NotifyAllPrepare -
func (desn *EpochStartNotifier) NotifyAllPrepare(_ data.HeaderHandler) {
}

// NotifyAll -
func (desn *EpochStartNotifier) NotifyAll(_ data.HeaderHandler) {
}

// IsInterfaceNil -
func (desn *EpochStartNotifier) IsInterfaceNil() bool {
	return desn == nil
}
