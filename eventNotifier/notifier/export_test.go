package notifier

import (
	"sync"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/eventNotifier"
)

func (essh *epochStartSubscriptionHandler) RegisteredHandlers() ([]eventNotifier.ActionHandler, *sync.RWMutex) {
	return essh.epochStartHandlers, &essh.mutEpochStartHandler
}

func (mesn *manualEpochStartNotifier) Handlers() []eventNotifier.ActionHandler {
	mesn.mutHandlers.RLock()
	defer mesn.mutHandlers.RUnlock()

	handlers := make([]eventNotifier.ActionHandler, len(mesn.handlers))
	copy(handlers, mesn.handlers)

	return handlers
}

func (gen *genericEpochNotifier) Handlers() []core.EpochSubscriberHandler {
	gen.mutHandler.RLock()
	defer gen.mutHandler.RUnlock()

	return gen.handlers
}
