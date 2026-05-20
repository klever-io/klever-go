package indexer

import (
	"bytes"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/marshal"
)

type Indexer = process.Indexer

// ArgDataIndexer is struct that is used to store all components that are needed to create a indexer
type ArgDataIndexer struct {
	Marshalizer            marshal.Marshalizer
	EpochStartNotifier     sharding.EpochStartEventNotifier
	NodesCoordinator       sharding.NodesCoordinator
	UseKibana              bool
	DataDispatcher         DispatcherHandler
	ElasticProcessor       ElasticProcessor
	AddressPubkeyConverter core.PubkeyConverter
}

// ArgElasticProcessor is struct that is used to store all components that are needed to an elastic indexer
type ArgElasticProcessor struct {
	IndexTemplates           map[string]*bytes.Buffer
	IndexPolicies            map[string]*bytes.Buffer
	Marshalizer              marshal.Marshalizer
	Hasher                   hashing.Hasher
	AddressPubkeyConverter   core.PubkeyConverter
	ValidatorPubkeyConverter core.PubkeyConverter
	UseKibana                bool
	DBClient                 DatabaseClientHandler
	EnabledIndexes           map[string]struct{}
	AccountsDB               state.AccountsAdapter
	KappsDB                  state.AccountsAdapter
	KAppController           kapp.KAppController
	Denomination             int
	IsInImportDBMode         bool
	CacheExpirationTime      time.Duration
	CacheCleanUpInterval     time.Duration
}

// ArgEventsProcessor is struct that is used to store all components needed for events dispatching
type ArgEventsProcessor struct {
	Marshalizer              marshal.Marshalizer
	Hasher                   hashing.Hasher
	AddressPubkeyConverter   core.PubkeyConverter
	ValidatorPubkeyConverter core.PubkeyConverter
	Indexer                  Indexer
	KAppController           kapp.KAppController
	AccountsDB               state.AccountsAdapter
}
