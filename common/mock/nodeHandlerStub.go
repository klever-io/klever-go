package mock

import (
	"encoding/json"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	indexerData "github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api/models"
	heartbeatData "github.com/klever-io/klever-go/node/heartbeat/data"
	"github.com/klever-io/klever-go/tools/debug"
)

// NodeHandlerStub - minimal stub for NodeHandler interface
type NodeHandlerStub struct {
	GetEconomicsCalled     func() (*models.EconomicsResponse, error)
	GetAccountTotalsCalled func() (*models.AccountTotalsResponse, error)
}

func (n *NodeHandlerStub) StartConsensus() error { return nil }
func (n *NodeHandlerStub) ValidateTransaction(*transaction.Transaction, bool) error {
	return nil
}
func (n *NodeHandlerStub) SendTransaction(*transaction.Transaction) (string, error) { return "", nil }
func (n *NodeHandlerStub) SendBulkTransactions([]*transaction.Transaction) ([]string, error) {
	return nil, nil
}
func (n *NodeHandlerStub) CreateTransaction(uint32, *transaction.TXBaseInfo, []json.RawMessage, bool) (*transaction.Transaction, []byte, error) {
	return nil, nil, nil
}
func (n *NodeHandlerStub) DecodeTransaction(*transaction.Transaction) (*indexerData.Transaction, error) {
	return nil, nil
}
func (n *NodeHandlerStub) GetTransaction(string, bool) (*api.Transaction, error) { return nil, nil }
func (n *NodeHandlerStub) EstimateTransactionFees(*transaction.Transaction) (*transaction.FeesResponse, error) {
	return nil, nil
}
func (n *NodeHandlerStub) TXPool(string, int, int) ([]*api.Transaction, int, error) {
	return nil, 0, nil
}
func (n *NodeHandlerStub) GetAccount(string) (state.UserAccountHandler, error) { return nil, nil }
func (n *NodeHandlerStub) GetNextNonce(string) (uint64, uint64, uint64, error) {
	return 0, 0, 0, nil
}
func (n *NodeHandlerStub) GetBalance(string, string) (int64, error) { return 0, nil }
func (n *NodeHandlerStub) GetUserKDA(string, string) (*kapps.UserKDA, error) {
	return nil, nil
}
func (n *NodeHandlerStub) GetAvailableClaim(string, string) (int64, map[string]int64, int64, error) {
	return 0, nil, 0, nil
}
func (n *NodeHandlerStub) GetAsset(string) (*kapps.KDAData, error) { return nil, nil }
func (n *NodeHandlerStub) GetEconomics() (*models.EconomicsResponse, error) {
	if n.GetEconomicsCalled != nil {
		return n.GetEconomicsCalled()
	}
	return nil, nil
}
func (n *NodeHandlerStub) GetAccountTotals() (*models.AccountTotalsResponse, error) {
	if n.GetAccountTotalsCalled != nil {
		return n.GetAccountTotalsCalled()
	}
	return nil, nil
}
func (n *NodeHandlerStub) GetNFT(string, string) (*kapps.UserKDA, *kapps.KDAData, error) {
	return nil, nil, nil
}
func (n *NodeHandlerStub) GetKDAFeePool(string) (*kdafeespool.KDAFeesPoolData, error) {
	return nil, nil
}
func (n *NodeHandlerStub) GetMarketplace(string) (*api.Marketplace, error) { return nil, nil }
func (n *NodeHandlerStub) GetHeartbeats() []heartbeatData.PubKeyHeartbeat  { return nil }
func (n *NodeHandlerStub) IsInterfaceNil() bool                            { return n == nil }
func (n *NodeHandlerStub) ValidatorStatisticsAPI() (map[string]*state.ValidatorApiResponse, error) {
	return nil, nil
}
func (n *NodeHandlerStub) PeersAPI() ([]state.PeerAccountHandler, error) { return nil, nil }
func (n *NodeHandlerStub) EncodeAddressPubkey([]byte) (string, error)    { return "", nil }
func (n *NodeHandlerStub) DecodeAddressPubkey(string) ([]byte, error)    { return nil, nil }
func (n *NodeHandlerStub) GetQueryHandler(string) (debug.QueryHandler, error) {
	return nil, nil
}
func (n *NodeHandlerStub) GetPeerInfo(string) ([]core.QueryP2PPeerInfo, error) {
	return nil, nil
}
func (n *NodeHandlerStub) GetProposalParameters() (map[int32]*kapps.Parameter, error) {
	return nil, nil
}
func (n *NodeHandlerStub) GetBlockByHash(string, bool) (*api.Block, error) { return nil, nil }
func (n *NodeHandlerStub) GetBlockByNonce(uint64, bool) (*api.Block, error) {
	return nil, nil
}
func (n *NodeHandlerStub) SetRedundancy(int64) error { return nil }
func (n *NodeHandlerStub) GetRedundancy() int64      { return 0 }
func (n *NodeHandlerStub) GetEnableEpochs() (config.EnableEpochsConfig, error) {
	return config.EnableEpochsConfig{}, nil
}
