package indexer

import (
	"bytes"
	"context"
	"time"

	"github.com/klever-io/klever-go/core/kapp"
	nodeData "github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/indexer/templates"
	"github.com/klever-io/klever-go/indexer/workItems"

	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// DispatcherHandler defines the interface for the dispatcher that will manage when items are saved in elasticsearch database
type DispatcherHandler interface {
	StartIndexData()
	Close() error
	Add(item workItems.WorkItemHandler)
	IsInterfaceNil() bool
}

// ElasticProcessor defines the interface for the elastic search indexer
type ElasticProcessor interface {
	SaveHeader(header nodeData.HeaderHandler, signer []byte, txsSize int, validators []string) error
	RemoveHeader(header nodeData.HeaderHandler) error
	RemoveTransactions(blk nodeData.HeaderHandler) error
	RemoveAccountsHistory(blockTimestamp int64) error
	RevertAccountBalances(blockTimestamp int64) error
	SaveTransactions(header nodeData.HeaderHandler, pool *indexer.Pool, prepared any) error
	SaveEpochInfo(epoch uint32, validators []kapp.ValidatorAccountInfoHandler) error
	UpdateProposalsAndParameters(proposalIDs []string) error
	SaveAssets(assets []*data.Asset) error
	SaveAccounts(blockTimestamp int64, accounts []*data.Account) error
	SavePeersAccounts(validators []kapp.ValidatorAccountInfoHandler) error
	IsInterfaceNil() bool
}

// DatabaseClientHandler is an interface that do requests to elasticsearch server
type DatabaseClientHandler interface {
	DoRequest(req *esapi.IndexRequest) error
	DoBulkRequest(ctx context.Context, buff *bytes.Buffer, index string) error
	DoBulkRemove(index string, hashes []string) error
	DoBulkRemoveByTimestamp(index string, timestamp time.Duration) error
	DoMultiGet(query templates.Object, index string) (templates.Object, error)
	DoSearch(index string, body *bytes.Buffer) (templates.Object, error)
	DoUpdate(index string, id string, body *bytes.Buffer) error
	Get(index string, id string) (templates.Object, error)
	CheckAndCreateIndex(index string) error
	CheckAndCreateAlias(alias string, index string) error
	CheckAndCreateTemplate(templateName string, template *bytes.Buffer) error
	CheckAndCreatePolicy(policyName string, policy *bytes.Buffer) error
	CheckFieldMapping(index string, properties templates.Object) ([]string, error)
	CheckAndUpdateMapping(index string, properties templates.Object) error
	PutTemplate(templateName string, template *bytes.Buffer) error
	DocExists(index string, id string) bool
	ConvertObjectToOrder(obj object) (*data.Order, error)
	ConvertObjectToData(obj object, data any) error

	IsInterfaceNil() bool
}

type CommonProcessor interface {
	BuildTransaction(tx *transaction.Transaction, txHash string, header nodeData.HeaderHandler) *data.Transaction
	DecodeContract(dbTx *data.Transaction, tx *transaction.Transaction, alteredAccounts data.AlteredAccountsHandler, itoAccounts data.AlteredITOHandler, alteredSC data.AlteredSmartContractsHandler, blockTimestamp int64) error
}

// LogsAndEventsHandler defines the actions that a logs and events handler should do
type LogsAndEventsHandler interface {
	PrepareLogsForDB(logsAndEvents []*nodeData.LogData, txsMap map[string]*data.Transaction, timestamp int64) []*data.Logs
	ExtractDataFromLogs(
		pool *indexer.Pool,
		txs []*data.Transaction,
		timestamp int64,
	) *data.PreparedLogsResults
}
