package facade

import (
	"encoding/json"

	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	indexerData "github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/node/heartbeat/data"
	"github.com/klever-io/klever-go/tools/debug"
	"github.com/klever-io/klever-go/vmcommon"
)

// NodeHandler contains all functions that a node should contain.
type NodeHandler interface {
	// StartConsensus will start the consesus service for the current node
	StartConsensus() error

	// ValidateTransaction will validate a transaction
	ValidateTransaction(tx *transaction.Transaction, checkSignature bool) error

	// SendTransaction sends the provided transaction to the network
	SendTransaction(tx *transaction.Transaction) (string, error)

	// SendBulkTransactions will send a bulk of transactions on the 'send transactions pipe' channel
	SendBulkTransactions(txs []*transaction.Transaction) ([]string, error)

	// CreateTransaction creates a transaction from all needed fields
	CreateTransaction(txType uint32, base *transaction.TXBaseInfo, contracts []json.RawMessage, skipValidate bool) (*transaction.Transaction, []byte, error)

	// DecodeTransaction decodes all fields of the provided transaction
	DecodeTransaction(tx *transaction.Transaction) (*indexerData.Transaction, error)

	// GetTransaction will return a transaction based on the hash
	GetTransaction(hash string, withResults bool) (*api.Transaction, error)

	// TXPool will retrun list od txs in mem pool
	TXPool(string, int, int) ([]*api.Transaction, int, error)

	// GetAccount returns an accountResponse containing information
	//  about the account correlated with provided address
	GetAccount(address string) (state.UserAccountHandler, error)

	// GetNextNonce returns account next nonce
	GetNextNonce(address string) (uint64, uint64, uint64, error)

	// GetBalance returns the balance for a specific address
	GetBalance(address, kda string) (int64, error)

	// GetAvailableClaim returns the rewards avaible for a specific asset in an account
	GetAvailableClaim(address string, assetId string) (int64, map[string]int64, int64, error)

	// GetAsset returns an assetResponse containing information
	//  about the asset correlated with provided assetID
	GetAsset(assetID string) (*kapps.KDAData, error)

	// GetNFT returns an assetResponse containing information
	// about the asset and userKDA correlated with provided address
	GetNFT(owner string, address string) (*kapps.UserKDA, *kapps.KDAData, error)

	// GetKDAFeePool returns an poolResponse containing information
	// about the pool of kda correlated with provided address
	GetKDAFeePool(address string) (*kdafeespool.KDAFeesPoolData, error)

	// GetMarketplace returns an assetResponse containing information
	//  about the marketplace correlated with provided marketplaceId
	GetMarketplace(marketplaceId string) (*api.Marketplace, error)

	// GetHeartbeats returns the heartbeat status for each public key defined in genesis.json
	GetHeartbeats() []data.PubKeyHeartbeat

	// IsInterfaceNil returns true if there is no value under the interface
	IsInterfaceNil() bool

	// ValidatorStatisticsAPI return the statistics for all the validators
	ValidatorStatisticsAPI() (map[string]*state.ValidatorApiResponse, error)

	// PeersAPI return the peers
	PeersAPI() ([]state.PeerAccountHandler, error)

	EncodeAddressPubkey(pk []byte) (string, error)
	DecodeAddressPubkey(pk string) ([]byte, error)

	GetQueryHandler(name string) (debug.QueryHandler, error)
	GetPeerInfo(pid string) ([]core.QueryP2PPeerInfo, error)
	GetProposalParameters() (map[int32]*kapps.Parameter, error)

	GetBlockByHash(hash string, withTxs bool) (*api.Block, error)
	GetBlockByNonce(nonce uint64, withTxs bool) (*api.Block, error)

	SetRedundancy(level int64) error
	GetRedundancy() int64

	GetEnableEpochs() (config.EnableEpochsConfig, error)
}

// APIResolver defines a structure capable of resolving REST API requests
type APIResolver interface {
	EstimateTransactionGas(tx *transaction.Transaction) (*transaction.CostResponse, error)
	ExecuteSCQuery(query *process.SCQuery) (*vmcommon.VMOutput, error)
	StatusMetrics() core.StatusMetricsHandler
	GetTotalStakedValue() (int64, error)
	IsInterfaceNil() bool
}

// HardforkTrigger defines the structure used to trigger hardforks
type HardforkTrigger interface {
	Trigger(epoch uint32, withEarlyEndOfEpoch bool) error
	IsSelfTrigger() bool
	IsInterfaceNil() bool
}
