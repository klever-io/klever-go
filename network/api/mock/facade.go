package mock

import (
	"encoding/hex"
	"encoding/json"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/core/statistics"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	indexerData "github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/node/heartbeat/data"
	"github.com/klever-io/klever-go/tools/debug"
)

// Facade is the mock implementation of a node router handler
type Facade struct {
	ShouldErrorStart               bool
	ShouldErrorStop                bool
	TpsBenchmarkHandler            func() *statistics.TpsBenchmark
	GetHeartbeatsHandler           func() ([]data.PubKeyHeartbeat, error)
	GetAccountHandler              func(address string) (state.UserAccountHandler, error)
	GetAssetHandler                func(address string) (*kapps.KDAData, error)
	GetNFTHandler                  func(owner string, address string) (*kapps.UserKDA, *kapps.KDAData, error)
	GetKDAFeePoolHandler           func(address string) (*kdafeespool.KDAFeesPoolData, error)
	BalanceHandler                 func(string, string) (*models.BalanceResponse, error)
	GetUserKDAHandler              func(string, string) (*kapps.UserKDA, error)
	RewardsAvailableToClaimHandler func(address string, assetId string) (*models.AvailableClaimResponse, error)
	StatusMetricsHandler           func() core.StatusMetricsHandler
	GetNodeOverviewHandler         func() (models.NodeOverview, error)
	ValidatorStatisticsHandler     func() (map[string]*state.ValidatorApiResponse, error)
	GetPeerInfoCalled              func(pid string) ([]core.QueryP2PPeerInfo, error)
	GetThrottlerForEndpointCalled  func(endpoint string) (core.Throttler, bool)
	NodeConfigCalled               func() map[string]interface{}
	GetQueryHandlerCalled          func(name string) (debug.QueryHandler, error)
	SetRedundancyCalled            func(level int64) error
	GetRedundancyCalled            func() int64
	GetValueForKeyCalled           func(address string, key string) (string, error)
	GetBlockByHashCalled           func(hash string, withTxs bool) (*api.Block, error)
	GetBlockByNonceCalled          func(nonce uint64, withTxs bool) (*api.Block, error)
	GetTotalStakedValueHandler     func() (int64, error)
	GetTransactionHandler          func(hash string, withResults bool) (*api.Transaction, error)
	CreateTransactionHandler       func(txType uint32, base *transaction.TXBaseInfo, contracts []json.RawMessage, skipValidate bool) (*transaction.Transaction, []byte, error)
	SendTransactionHandler         func(tx *transaction.Transaction) (string, error)
	DecodeTransactionHandler       func(tx *transaction.Transaction) (*indexerData.Transaction, error)
	SendBulkTransactionsHandler    func(txs []*transaction.Transaction) ([]string, error)
	GetNextNonceHandler            func(address string) (*models.AccountNonceResponse, error)
	EstimateTransactionGasHandler  func(tx *transaction.Transaction) (*transaction.CostResponse, error)
	EstimateTransactionFeesHandler func(tx *transaction.Transaction) (*transaction.FeesResponse, error)
}

// GetThrottlerForEndpoint -
func (f *Facade) GetThrottlerForEndpoint(endpoint string) (core.Throttler, bool) {
	if f.GetThrottlerForEndpointCalled != nil {
		return f.GetThrottlerForEndpointCalled(endpoint)
	}

	return nil, false
}

// GetThrottlerForEndpoint -
func (f *Facade) TXPool(sender string, page int, pageSize int) ([]*api.Transaction, int, error) {
	return make([]*api.Transaction, 0), 0, nil
}

// RestAPIInterface -
func (f *Facade) RestAPIInterface() string {
	return "localhost:8080"
}

// RestAPIServerDebugMode -
func (f *Facade) RestAPIServerDebugMode() bool {
	return false
}

// PprofEnabled -
func (f *Facade) PprofEnabled() bool {
	return false
}

// PprofEnabled -
func (f *Facade) WSConnectionURL() string {
	return ""
}

// PprofEnabled -
func (f *Facade) WSConnectionAPIKey() string {
	return ""
}

// TpsBenchmark is the mock implementation for retreiving the TpsBenchmark
func (f *Facade) TpsBenchmark() *statistics.TpsBenchmark {
	if f.TpsBenchmarkHandler != nil {
		return f.TpsBenchmarkHandler()
	}
	return nil
}

// GetHeartbeats returns the slice of heartbeat info
func (f *Facade) GetHeartbeats() ([]data.PubKeyHeartbeat, error) {
	return f.GetHeartbeatsHandler()
}

// GetValueForKey is the mock implementation of a handler's GetValueForKey method
func (f *Facade) GetValueForKey(address string, key string) (string, error) {
	if f.GetValueForKeyCalled != nil {
		return f.GetValueForKeyCalled(address, key)
	}

	return "", nil
}

// GetAccount is the mock implementation of a handler's GetAccount method
func (f *Facade) GetAccount(address string) (state.UserAccountHandler, error) {
	return f.GetAccountHandler(address)
}

// GetAvailableClaim is the mock implementation of a handler's GetAvailableClaim method
func (f *Facade) GetAvailableClaim(address string, assetId string) (*models.AvailableClaimResponse, error) {
	return f.RewardsAvailableToClaimHandler(address, assetId)
}

// GetBalance is the mock implementation of a handler's GetBalance method
func (f *Facade) GetBalance(address, kda string) (*models.BalanceResponse, error) {
	return f.BalanceHandler(address, kda)
}

func (f *Facade) GetUserKDA(address string, kda string) (*kapps.UserKDA, error) {
	return f.GetUserKDAHandler(address, kda)
}

// GetAsset is the mock implementation of a handler's GetAsset method
func (f *Facade) GetAsset(assetID string) (*kapps.KDAData, error) {
	return f.GetAssetHandler(assetID)
}

// GetNFT is the mock implementation of a handler's GetNFT method
func (f *Facade) GetNFT(owner string, address string) (*kapps.UserKDA, *kapps.KDAData, error) {
	return f.GetNFTHandler(owner, address)
}

// GetKdaFeePool is the mock implementation of a handler's GetKdaFeePool method
func (f *Facade) GetKDAFeePool(address string) (*kdafeespool.KDAFeesPoolData, error) {
	return f.GetKDAFeePoolHandler(address)
}

// ValidatorStatisticsAPI is the mock implementation of a handler's ValidatorStatisticsAPI method
func (f *Facade) ValidatorStatisticsAPI() (map[string]*state.ValidatorApiResponse, error) {
	return f.ValidatorStatisticsHandler()
}

// StatusMetrics is the mock implementation for the StatusMetrics
func (f *Facade) StatusMetrics() core.StatusMetricsHandler {
	return f.StatusMetricsHandler()
}

// GetNodeOverview -
func (f *Facade) GetNodeOverview() (models.NodeOverview, error) {
	return f.GetNodeOverviewHandler()
}

// GetTotalStakedValue -
func (f *Facade) GetTotalStakedValue() (int64, error) {
	return f.GetTotalStakedValueHandler()
}

// NodeConfig -
func (f *Facade) NodeConfig() map[string]interface{} {
	return f.NodeConfigCalled()
}

// EncodeAddressPubkey -
func (f *Facade) EncodeAddressPubkey(pk []byte) (string, error) {
	return hex.EncodeToString(pk), nil
}

// DecodeAddressPubkey -
func (f *Facade) DecodeAddressPubkey(pk string) ([]byte, error) {
	return hex.DecodeString(pk)
}

// GetQueryHandler -
func (f *Facade) GetQueryHandler(name string) (debug.QueryHandler, error) {
	return f.GetQueryHandlerCalled(name)
}

// SetRedundancy -
func (f *Facade) SetRedundancy(level int64) error {
	return f.SetRedundancyCalled(level)
}

// SetRedundancy -
func (f *Facade) GetRedundancy() int64 {
	if f.GetRedundancyCalled != nil {
		return f.GetRedundancyCalled()
	}

	return 0
}

// GetPeerInfo -
func (f *Facade) GetPeerInfo(pid string) ([]core.QueryP2PPeerInfo, error) {
	return f.GetPeerInfoCalled(pid)
}

// GetBlockByNonce -
func (f *Facade) GetBlockByNonce(nonce uint64, withTxs bool) (*api.Block, error) {
	return f.GetBlockByNonceCalled(nonce, withTxs)
}

// GetBlockByHash -
func (f *Facade) GetBlockByHash(hash string, withTxs bool) (*api.Block, error) {
	return f.GetBlockByHashCalled(hash, withTxs)
}

// GetBlockByHash -
func (f *Facade) GetTransaction(hash string, withTxs bool) (*api.Transaction, error) {
	return f.GetTransactionHandler(hash, withTxs)
}

func (f *Facade) CreateTransaction(txType uint32, base *transaction.TXBaseInfo, contracts []json.RawMessage, skipValidate bool) (*transaction.Transaction, []byte, error) {
	return f.CreateTransactionHandler(txType, base, contracts, skipValidate)
}

func (f *Facade) DecodeTransaction(tx *transaction.Transaction) (*indexerData.Transaction, error) {
	return f.DecodeTransactionHandler(tx)
}

func (f *Facade) SendBulkTransactions(txs []*transaction.Transaction) ([]string, error) {
	return f.SendBulkTransactionsHandler(txs)
}

func (f *Facade) SendTransaction(tx *transaction.Transaction) (string, error) {
	return f.SendTransactionHandler(tx)
}

func (f *Facade) GetNextNonce(address string) (*models.AccountNonceResponse, error) {
	return f.GetNextNonceHandler(address)
}

func (f *Facade) GetProposalParameters() (map[int32]*kapps.Parameter, error) {
	return make(map[int32]*kapps.Parameter), nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (f *Facade) IsInterfaceNil() bool {
	return f == nil
}

// GetEnableEpochs -
func (f *Facade) GetEnableEpochs() (config.EnableEpochsConfig, error) {
	return config.EnableEpochsConfig{}, nil
}

// GetEnableEpochs -
func (f *Facade) EstimateTransactionGas(tx *transaction.Transaction) (*transaction.CostResponse, error) {
	return f.EstimateTransactionGasHandler(tx)
}

// EstimateTransactionFees -
func (f *Facade) EstimateTransactionFees(tx *transaction.Transaction) (*transaction.FeesResponse, error) {
	return f.EstimateTransactionFeesHandler(tx)
}

// WrongFacade is a struct that can be used as a wrong implementation of the node router handler
type WrongFacade struct {
}
