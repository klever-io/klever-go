package bootstrap

import (
	"context"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/retriever"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/eventNotifier"
	"github.com/klever-io/klever-go/sharding"
)

// StartOfEpochMetaSyncer defines the methods to synchronize epoch start meta block from the network when nothing is known
type StartOfEpochMetaSyncer interface {
	SyncEpochStartMeta(waitTime time.Duration) (*block.Block, error)
	SyncPrevEpochStartMeta(waitTime time.Duration, epoch uint32) (*block.Block, error)
	IsInterfaceNil() bool
}

// StartOfEpochNodesConfigHandler defines the methods to process nodesConfig from epoch start metablocks
type StartOfEpochNodesConfigHandler interface {
	NodesConfigFromMetaBlock(currMetaBlock *block.Block, prevMetaBlock *block.Block, epochStartValidators []*state.ValidatorInfo, prevEpochStartValidators []*state.ValidatorInfo) (*sharding.NodesCoordinatorRegistry, error)
	IsInterfaceNil() bool
}

// EpochStartBlockInterceptorProcessor defines the methods to sync an epoch start metablock
type EpochStartBlockInterceptorProcessor interface {
	process.InterceptorProcessor
	GetEpochStartBlock(ctx context.Context) (*block.Block, error)
	GetPrevEpochStartBlock(ctx context.Context, epoch uint32) (*block.Block, error)
}

// StartInEpochNodesCoordinator defines the methods to process and save nodesCoordinator information to storage
type StartInEpochNodesCoordinator interface {
	EpochStartPrepare(metaHdr data.HeaderHandler)
	NodesCoordinatorToRegistry() *sharding.NodesCoordinatorRegistry
	SetEpochValidatorsInfo(epoch uint32, validatorsInfo []*state.ValidatorInfo) error
	IsInterfaceNil() bool
}

// Messenger defines which methods a p2p messenger should implement
type Messenger interface {
	retriever.MessageHandler
	retriever.TopicHandler
	UnregisterMessageProcessor(topic string) error
	UnregisterAllMessageProcessors() error
	UnjoinAllTopics() error
	ConnectedPeers() []core.PeerID
}

// RequestHandler defines which methods a request handler should implement
type RequestHandler interface {
	RequestStartOfEpochBlock(epoch uint32)
	SetNumPeersToQuery(topic string, intra int, cross int) error
	GetNumPeersToQuery(topic string) (int, int, error)
	IsInterfaceNil() bool
}

// EpochStartBootstrapper defines the epoch start bootstrap functionality
type EpochStartBootstrapper interface {
	Bootstrap() (Parameters, error)
	GetTriesComponents() (state.TriesHolder, map[string]data.StorageManager)
	IsInterfaceNil() bool
}

// ManualEpochStartNotifier represents a notifier that can be triggered manually for an epoch change event.
// Useful in storage resolvers (import-db process)
type ManualEpochStartNotifier interface {
	RegisterHandler(handler eventNotifier.ActionHandler)
	NewEpoch(epoch uint32)
	CurrentEpoch() uint32
	IsInterfaceNil() bool
}
