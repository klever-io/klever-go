package factory

import (
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/state"
	indexer "github.com/klever-io/klever-go/indexer"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

// ArgsIndexerFactory holds all dependencies required by the data indexer factory in order to create
// new instances
type ArgsIndexerFactory struct {
	Enabled                  bool
	IndexerCacheSize         int
	Url                      string
	UserName                 string
	Password                 string
	Marshalizer              marshal.Marshalizer
	Hasher                   hashing.Hasher
	EpochStartNotifier       sharding.EpochStartEventNotifier
	NodesCoordinator         sharding.NodesCoordinator
	AddressPubkeyConverter   core.PubkeyConverter
	ValidatorPubkeyConverter core.PubkeyConverter
	UseKibana                bool
	EnabledIndexes           []string
	Denomination             int
	AccountsDB               state.AccountsAdapter
	KappsDB                  state.AccountsAdapter
	KAppController           kapp.KAppController
	IsInImportDBMode         bool
}

// NewIndexer will create a new instance of Indexer
func NewIndexer(args *ArgsIndexerFactory) (process.Indexer, error) {
	err := checkDataIndexerParams(args)
	if err != nil {
		return nil, err
	}

	if !args.Enabled {
		return indexer.NewNilIndexer(), nil
	}

	elasticProcessor, err := createElasticProcessor(args)
	if err != nil {
		return nil, err
	}

	dispatcher, err := indexer.NewDataDispatcher(args.IndexerCacheSize)
	if err != nil {
		return nil, err
	}

	dispatcher.StartIndexData()

	arguments := indexer.ArgDataIndexer{
		Marshalizer:            args.Marshalizer,
		UseKibana:              args.UseKibana,
		NodesCoordinator:       args.NodesCoordinator,
		EpochStartNotifier:     args.EpochStartNotifier,
		ElasticProcessor:       elasticProcessor,
		DataDispatcher:         dispatcher,
		AddressPubkeyConverter: args.AddressPubkeyConverter,
	}

	return indexer.NewDataIndexer(arguments)
}

func createDatabaseClient(url, userName, password string) (indexer.DatabaseClientHandler, error) {
	return indexer.NewElasticClient(elasticsearch.Config{
		Addresses: []string{url},
		Username:  userName,
		Password:  password,
	})
}

func createElasticProcessor(args *ArgsIndexerFactory) (indexer.ElasticProcessor, error) {
	databaseClient, err := createDatabaseClient(args.Url, args.UserName, args.Password)
	if err != nil {
		return nil, err
	}

	indexTemplates, indexPolicies, err := indexer.GetElasticTemplatesAndPolicies(args.UseKibana)
	if err != nil {
		return nil, err
	}

	enabledIndexesMap := make(map[string]struct{})
	for _, index := range args.EnabledIndexes {
		enabledIndexesMap[index] = struct{}{}
	}
	if len(enabledIndexesMap) == 0 {
		return nil, indexer.ErrEmptyEnabledIndexes
	}

	esIndexerArgs := indexer.ArgElasticProcessor{
		IndexTemplates:           indexTemplates,
		IndexPolicies:            indexPolicies,
		Marshalizer:              args.Marshalizer,
		Hasher:                   args.Hasher,
		AddressPubkeyConverter:   args.AddressPubkeyConverter,
		ValidatorPubkeyConverter: args.ValidatorPubkeyConverter,
		UseKibana:                args.UseKibana,
		DBClient:                 databaseClient,
		EnabledIndexes:           enabledIndexesMap,
		AccountsDB:               args.AccountsDB,
		KappsDB:                  args.KappsDB,
		KAppController:           args.KAppController,
		Denomination:             args.Denomination,
		IsInImportDBMode:         args.IsInImportDBMode,
		CacheExpirationTime:      time.Hour,
		CacheCleanUpInterval:     2 * time.Hour,
	}

	return indexer.NewElasticProcessor(esIndexerArgs)
}

func checkDataIndexerParams(arguments *ArgsIndexerFactory) error {
	if arguments.IndexerCacheSize < 0 {
		return indexer.ErrNegativeCacheSize
	}
	if check.IfNil(arguments.AddressPubkeyConverter) {
		return fmt.Errorf("%w when setting AddressPubkeyConverter in indexer", indexer.ErrNilPubkeyConverter)
	}
	if check.IfNil(arguments.ValidatorPubkeyConverter) {
		return fmt.Errorf("%w when setting ValidatorPubkeyConverter in indexer", indexer.ErrNilPubkeyConverter)
	}
	if arguments.Url == "" {
		return common.ErrNilUrl
	}
	if check.IfNil(arguments.Marshalizer) {
		return common.ErrNilMarshalizer
	}
	if check.IfNil(arguments.Hasher) {
		return common.ErrNilHasher
	}
	if check.IfNil(arguments.NodesCoordinator) {
		return common.ErrNilNodesCoordinator
	}
	if check.IfNil(arguments.EpochStartNotifier) {
		return common.ErrNilEpochStartNotifier
	}
	// if check.IfNil(arguments.TransactionFeeCalculator) {
	// 	return common.ErrNilTransactionFeeCalculator
	// }

	return nil
}
