package notifier

import (
	"sync"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/eventNotifier"
)

var log = logger.GetOrCreate("notifier/EpochNotifier")

// manualEpochStartNotifier will notice all recorded handlers to a provided epoch (from other components)
// TODO think about a better name as this does not get its data from the exterior of the node
type manualEpochStartNotifier struct {
	mutHandlers     sync.RWMutex
	handlers        []eventNotifier.ActionHandler
	mutCurrentEpoch sync.RWMutex
	currentEpoch    uint32
}

// NewManualEpochStartNotifier creates a new instance of a manual epoch start notifier
func NewManualEpochStartNotifier() *manualEpochStartNotifier {
	return &manualEpochStartNotifier{
		handlers: make([]eventNotifier.ActionHandler, 0),
	}
}

// RegisterHandler registers an epoch start action handler
func (mesn *manualEpochStartNotifier) RegisterHandler(handler eventNotifier.ActionHandler) {
	mesn.mutHandlers.Lock()
	defer mesn.mutHandlers.Unlock()

	mesn.handlers = append(mesn.handlers, handler)
}

// NewEpoch signals that a new epoch event has occurred
func (mesn *manualEpochStartNotifier) NewEpoch(epoch uint32) {
	mesn.mutCurrentEpoch.Lock()
	if mesn.currentEpoch >= epoch {
		mesn.mutCurrentEpoch.Unlock()
		return
	}
	mesn.currentEpoch = epoch
	mesn.mutCurrentEpoch.Unlock()

	log.Info("manualEpochStartNotifier.NewEpoch", "epoch", epoch)

	mesn.mutHandlers.RLock()
	defer mesn.mutHandlers.RUnlock()

	for _, handler := range mesn.handlers {
		handler.EpochStartAction(&block.Block{
			Header: &block.BlockHeader{
				Epoch: epoch,
			},
		})
	}
}

// CurrentEpoch returns the current epoch saved
func (mesn *manualEpochStartNotifier) CurrentEpoch() uint32 {
	mesn.mutCurrentEpoch.RLock()
	defer mesn.mutCurrentEpoch.RUnlock()

	return mesn.currentEpoch
}

// IsInterfaceNil returns true if there is no value under the interface
func (mesn *manualEpochStartNotifier) IsInterfaceNil() bool {
	return mesn == nil
}
