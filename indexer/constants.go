package indexer

var headerContentTypeJSON = []string{"application/json"}

// StringKeyType defines the type for the context key
type StringKeyType string

const (
	headerXSRF        = "kbn-xsrf"
	headerContentType = "Content-Type"
	kibanaPluginPath  = "_plugin/kibana/api"

	blockIndex        = "blocks"
	txIndex           = "transactions"
	logsIndex         = "logs"
	scDeploysIndex    = "scdeploys"
	assetsIndex       = "assets"
	proposalsIndex    = "proposals"
	marketplacesIndex = "marketplaces"

	iTOsIndex = "itos"

	marketplaceOrdersIndex = "marketplaceorders"
	accountsIndex          = "accounts"
	peersAccountsIndex     = "peersaccounts"
	accountsKDAIndex       = "accountskda"
	accountsHistoryIndex   = "accountshistory"
	epochIndex             = "epoch"
	kdaPoolsIndex          = "kdapools"

	txPolicy              = "transactions_policy"
	blockPolicy           = "blocks_policy"
	ratingPolicy          = "rating_policy"
	accountsHistoryPolicy = "accountshistory_policy"

	// BulkTopic is the identifier for the bulk requests metrics
	bulkTopic string = "req_bulk"
	// ContextKey the key for the value that will be added in the context
	contextKey StringKeyType = "key"
)
