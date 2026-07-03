package facade

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data/vm"
	"github.com/klever-io/klever-go/vmcommon"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	kdafeespool "github.com/klever-io/klever-go/core/kapp/kdaFeesPool"
	"github.com/klever-io/klever-go/core/statistics"
	"github.com/klever-io/klever-go/core/throttler"
	dataAPI "github.com/klever-io/klever-go/data/api"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	indexerData "github.com/klever-io/klever-go/indexer/data"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/network/api"
	"github.com/klever-io/klever-go/network/api/address"
	"github.com/klever-io/klever-go/network/api/middleware"
	"github.com/klever-io/klever-go/network/api/models"
	"github.com/klever-io/klever-go/network/api/node"
	transactionAPI "github.com/klever-io/klever-go/network/api/transaction"
	"github.com/klever-io/klever-go/network/api/validator"
	"github.com/klever-io/klever-go/node/heartbeat/data"
	"github.com/klever-io/klever-go/ntp"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/debug"
)

var log = logger.GetOrCreate("facade")

// DefaultRestInterface is the default interface the rest API will start on if not specified
const DefaultRestInterface = "localhost:8080"

// DefaultRestPortOff is the default value that should be passed if it is desired
// to start the node without a REST endpoint available
const DefaultRestPortOff = "off"

var _ = address.FacadeHandler(&nodeFacade{})
var _ = node.FacadeHandler(&nodeFacade{})
var _ = transactionAPI.FacadeHandler(&nodeFacade{})
var _ = validator.FacadeHandler(&nodeFacade{})

type resetHandler interface {
	Reset()
	IsInterfaceNil() bool
}

// ArgNodeFacade represents the argument for the nodeFacade
type ArgNodeFacade struct {
	Node                   NodeHandler
	APIResolver            APIResolver
	RestAPIServerDebugMode bool
	WsAntifloodConfig      config.WebServerAntifloodConfig
	FacadeConfig           config.FacadeConfig
	APIRoutesConfig        config.APIRoutesConfig
	AccountsState          state.AccountsAdapter
	KAppsState             state.AccountsAdapter
	PeerState              state.AccountsAdapter
}

// nodeFacade represents a facade for grouping the functionality for the node
type nodeFacade struct {
	node                   NodeHandler
	apiResolver            APIResolver
	syncer                 ntp.SyncTimer
	tpsBenchmark           *statistics.TpsBenchmark
	config                 config.FacadeConfig
	apiRoutesConfig        config.APIRoutesConfig
	endpointsThrottlers    map[string]core.Throttler
	wsAntifloodConfig      config.WebServerAntifloodConfig
	restAPIServerDebugMode bool
	accountsState          state.AccountsAdapter
	kappsState             state.AccountsAdapter
	peerState              state.AccountsAdapter
	ctx                    context.Context
	cancelFunc             func()
}

// NewNodeFacade creates a new Facade with a NodeWrapper
func NewNodeFacade(arg ArgNodeFacade) (*nodeFacade, error) {
	if check.IfNil(arg.Node) {
		return nil, common.ErrNilNode
	}
	if check.IfNil(arg.APIResolver) {
		return nil, common.ErrNilAPIResolver
	}
	if len(arg.APIRoutesConfig.APIPackages) == 0 {
		return nil, common.ErrNoAPIRoutesConfig
	}
	if arg.WsAntifloodConfig.SimultaneousRequests == 0 {
		return nil, fmt.Errorf("%w, SimultaneousRequests should not be 0", common.ErrInvalidValue)
	}
	if arg.WsAntifloodConfig.SameSourceRequests == 0 {
		return nil, fmt.Errorf("%w, SameSourceRequests should not be 0", common.ErrInvalidValue)
	}
	if arg.WsAntifloodConfig.SameSourceResetIntervalInSec == 0 {
		return nil, fmt.Errorf("%w, SameSourceResetIntervalInSec should not be 0", common.ErrInvalidValue)
	}
	if check.IfNil(arg.AccountsState) {
		return nil, common.ErrNilAccountState
	}
	if check.IfNil(arg.KAppsState) {
		return nil, common.ErrNilKappState
	}
	if check.IfNil(arg.PeerState) {
		return nil, common.ErrNilPeerState
	}

	throttlersMap := computeEndpointsNumGoRoutinesThrottlers(arg.WsAntifloodConfig)

	nf := &nodeFacade{
		node:                   arg.Node,
		apiResolver:            arg.APIResolver,
		restAPIServerDebugMode: arg.RestAPIServerDebugMode,
		wsAntifloodConfig:      arg.WsAntifloodConfig,
		config:                 arg.FacadeConfig,
		apiRoutesConfig:        arg.APIRoutesConfig,
		endpointsThrottlers:    throttlersMap,
		accountsState:          arg.AccountsState,
		kappsState:             arg.KAppsState,
		peerState:              arg.PeerState,
	}
	nf.ctx, nf.cancelFunc = context.WithCancel(context.Background())

	return nf, nil
}

func computeEndpointsNumGoRoutinesThrottlers(webServerAntiFloodConfig config.WebServerAntifloodConfig) map[string]core.Throttler {
	throttlersMap := make(map[string]core.Throttler)
	for _, endpointSetting := range webServerAntiFloodConfig.EndpointsThrottlers {
		newThrottler, err := throttler.NewNumGoRoutinesThrottler(endpointSetting.MaxNumGoRoutines)
		if err != nil {
			log.Warn("error when setting the maximum go routines throttler for endpoint",
				"endpoint", endpointSetting.Endpoint,
				"max go routines", endpointSetting.MaxNumGoRoutines,
				"error", err,
			)
			continue
		}
		throttlersMap[endpointSetting.Endpoint] = newThrottler
	}

	return throttlersMap
}

// SetSyncer sets the current syncer
func (nf *nodeFacade) SetSyncer(syncer ntp.SyncTimer) {
	nf.syncer = syncer
}

// GetHeartbeats returns the heartbeat status for each public key from initial list or later joined to the network
func (nf *nodeFacade) GetHeartbeats() ([]data.PubKeyHeartbeat, error) {
	hbStatus := nf.node.GetHeartbeats()
	if hbStatus == nil {
		return nil, ErrHeartbeatsNotActive
	}

	return hbStatus, nil
}

// SetTpsBenchmark sets the tps benchmark handler
func (nf *nodeFacade) SetTpsBenchmark(tpsBenchmark *statistics.TpsBenchmark) {
	nf.tpsBenchmark = tpsBenchmark
}

// TpsBenchmark returns the tps benchmark handler
func (nf *nodeFacade) TpsBenchmark() *statistics.TpsBenchmark {
	return nf.tpsBenchmark
}

// StatusMetrics will return the node's status metrics
func (nf *nodeFacade) StatusMetrics() core.StatusMetricsHandler {
	return nf.apiResolver.StatusMetrics()
}

// GetNodeOverview returns the node overview information
func (nf *nodeFacade) GetNodeOverview() (models.NodeOverview, error) {
	metrics := nf.StatusMetrics().Overview()

	overview := models.NodeOverview{
		ChainID:              getStringValue(metrics["chainID"]),
		BaseTxSize:           getInt64Value(metrics["baseTxSize"]),
		SlotAtEpochStart:     getInt64Value(metrics["slotAtEpochStart"]),
		SlotsPerEpoch:        getInt64Value(metrics["slotsPerEpoch"]),
		CurrentSlot:          getInt64Value(metrics["currentSlot"]),
		SlotDuration:         getInt64Value(metrics["slotDuration"]),
		SlotCurrentTimestamp: getInt64Value(metrics["slotCurrentTimestamp"]),
		StartTime:            getInt64Value(metrics["startTime"]),
		EpochNumber:          getInt64Value(metrics["epochNumber"]),
		NonceAtEpochStart:    getInt64Value(metrics["nonceAtEpochStart"]),
		Nonce:                getInt64Value(metrics["nonce"]),
	}

	return overview, nil
}

func getStringValue(value interface{}) string {
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

func getInt64Value(value interface{}) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case uint64:
		return int64(v) //nolint:gosec // G115: intentional conversion for metrics display
	case int:
		return int64(v)
	case uint:
		return int64(v) //nolint:gosec // G115: intentional conversion for metrics display
	default:
		return 0
	}
}

// GetQueryHandler returns the query handler if existing
func (nf *nodeFacade) GetQueryHandler(name string) (debug.QueryHandler, error) {
	return nf.node.GetQueryHandler(name)
}

// SetRedundancy set internal redundancy level
func (nf *nodeFacade) SetRedundancy(level int64) error {
	return nf.node.SetRedundancy(level)
}

// SetRedundancy set internal redundancy level
func (nf *nodeFacade) GetRedundancy() int64 {
	return nf.node.GetRedundancy()
}

// GetEnableEpochs -
func (nf *nodeFacade) GetEnableEpochs() (config.EnableEpochsConfig, error) {
	return nf.node.GetEnableEpochs()
}

// GetPeerInfo returns the peer info of a provided pid
func (nf *nodeFacade) GetPeerInfo(pid string) ([]core.QueryP2PPeerInfo, error) {
	return nf.node.GetPeerInfo(pid)
}

// GetProposalParameters returns the active parameters of the proposal controller
func (nf *nodeFacade) GetProposalParameters() (map[int32]*kapps.Parameter, error) {
	return nf.node.GetProposalParameters()
}

// StartNode starts the underlying node
func (nf *nodeFacade) StartNode() error {
	return nf.node.StartConsensus()
}

// StartBackgroundServices starts all background services needed for the correct functionality of the node
func (nf *nodeFacade) StartBackgroundServices(ctx context.Context) {
	go nf.startRest(ctx)
}

// RestAPIServerDebugMode return true is debug mode for Rest API is enabled
func (nf *nodeFacade) RestAPIServerDebugMode() bool {
	return nf.restAPIServerDebugMode
}

// RestAPIInterface returns the interface on which the rest API should start on, based on the config file provided.
// The API will start on the DefaultRestInterface value unless a correct value is passed or
// the value is explicitly set to off, in which case it will not start at all
func (nf *nodeFacade) RestAPIInterface() string {
	if nf.config.RestAPIInterface == "" {
		return DefaultRestInterface
	}

	return nf.config.RestAPIInterface
}

func (nf *nodeFacade) startRest(ctx context.Context) {
	log.Trace("starting REST api server")

	switch nf.RestAPIInterface() {
	case DefaultRestPortOff:
		log.Debug("web server is off")
	default:
		log.Debug("creating web server limiters")
		limiters, err := nf.CreateMiddlewareLimiters()
		if err != nil {
			log.Error("error creating web server limiters",
				"error", err.Error(),
			)
			log.Error("web server is off")
			return
		}

		log.Debug("starting web server",
			"SimultaneousRequests", nf.wsAntifloodConfig.SimultaneousRequests,
			"SameSourceRequests", nf.wsAntifloodConfig.SameSourceRequests,
			"SameSourceResetIntervalInSec", nf.wsAntifloodConfig.SameSourceResetIntervalInSec,
		)

		err = api.Start(ctx, nf, nf.apiRoutesConfig, limiters...)
		if err != nil {
			log.Error("could not start webserver",
				"error", err.Error(),
			)
		}
	}
}

// CreateMiddlewareLimiters will create the middleware limiters used in web server
func (nf *nodeFacade) CreateMiddlewareLimiters() ([]api.MiddlewareProcessor, error) {
	sourceLimiter, err := middleware.NewSourceThrottler(nf.wsAntifloodConfig.SameSourceRequests)
	if err != nil {
		return nil, err
	}
	go nf.sourceLimiterReset(sourceLimiter)

	globalLimiter, err := middleware.NewGlobalThrottler(nf.wsAntifloodConfig.SimultaneousRequests)
	if err != nil {
		return nil, err
	}

	return []api.MiddlewareProcessor{sourceLimiter, globalLimiter}, nil
}

func (nf *nodeFacade) sourceLimiterReset(reset resetHandler) {
	betweenResetDuration := time.Second * time.Duration(nf.wsAntifloodConfig.SameSourceResetIntervalInSec)
	for {
		select {
		case <-time.After(betweenResetDuration):
			log.Trace("calling reset on WS source limiter")
			reset.Reset()
		case <-nf.ctx.Done():
			log.Debug("closing nodeFacade.sourceLimiterReset go routine")
			return
		}
	}
}

func (nf *nodeFacade) PprofEnabled() bool {
	return nf.config.PprofEnabled
}

func (nf *nodeFacade) IsInterfaceNil() bool {
	return nf == nil
}

func (nf *nodeFacade) WSConnectionURL() string {
	return nf.config.WSConnectionURL
}

func (nf *nodeFacade) WSConnectionAPIKey() string {
	return nf.config.WSConnectionAPIKey
}

// WSMaxConnections returns the node-wide cap on simultaneous live /subscribe
// WebSocket connections (0 = unlimited).
func (nf *nodeFacade) WSMaxConnections() uint32 {
	return nf.wsAntifloodConfig.WebSocketConnections
}

// WSMaxConnectionsPerIP returns the per-source-IP cap on simultaneous live
// /subscribe WebSocket connections (0 = unlimited).
func (nf *nodeFacade) WSMaxConnectionsPerIP() uint32 {
	return nf.wsAntifloodConfig.WebSocketConnectionsPerIP
}

// WSMaxAddressesPerSubscribe returns the per-call address cap for a /subscribe
// request (0 = built-in default).
func (nf *nodeFacade) WSMaxAddressesPerSubscribe() uint32 {
	return nf.wsAntifloodConfig.WebSocketMaxAddressesPerSubscribe
}

// WSMaxAddressesPerClient returns the per-connection total address cap for
// /subscribe (0 = built-in default).
func (nf *nodeFacade) WSMaxAddressesPerClient() uint32 {
	return nf.wsAntifloodConfig.WebSocketMaxAddressesPerClient
}

/***********
 *  Block
 ***********/

// GetBlockByHash return the block for a given hash
func (nf *nodeFacade) GetBlockByHash(hash string, withTxs bool) (*dataAPI.Block, error) {
	return nf.node.GetBlockByHash(hash, withTxs)
}

// GetBlockByNonce returns the block for a given nonce
func (nf *nodeFacade) GetBlockByNonce(nonce uint64, withTxs bool) (*dataAPI.Block, error) {
	return nf.node.GetBlockByNonce(nonce, withTxs)
}

/****************
 *  Transaction
 ****************/

// GetThrottlerForEndpoint returns the throttler for a given endpoint if found
func (nf *nodeFacade) GetThrottlerForEndpoint(endpoint string) (core.Throttler, bool) {
	throttlerForEndpoint, ok := nf.endpointsThrottlers[endpoint]
	isThrottlerOk := ok && throttlerForEndpoint != nil

	return throttlerForEndpoint, isThrottlerOk
}

// TXPool returns list of transactions in mempool
func (nf *nodeFacade) TXPool(sender string, page int, pageSize int) ([]*dataAPI.Transaction, int, error) {
	return nf.node.TXPool(sender, page, pageSize)
}

// CreateTransaction creates a transaction from all needed fields
func (nf *nodeFacade) CreateTransaction(
	txType uint32,
	base *transaction.TXBaseInfo,
	contracts []json.RawMessage,
	skipValidate bool,
) (*transaction.Transaction, []byte, error) {
	return nf.node.CreateTransaction(txType, base, contracts, skipValidate)
}

// EncodeAddressPubkey returns readable address from bytes
func (nf *nodeFacade) EncodeAddressPubkey(addr []byte) (string, error) {
	return nf.node.EncodeAddressPubkey(addr)
}

// SendBulkTransactions will send a bulk of transactions on the topic channel
func (nf *nodeFacade) SendBulkTransactions(txs []*transaction.Transaction) ([]string, error) {
	return nf.node.SendBulkTransactions(txs)
}

// DecodeTransaction decodes all fields of the provided transaction
func (nf *nodeFacade) DecodeTransaction(tx *transaction.Transaction) (*indexerData.Transaction, error) {
	return nf.node.DecodeTransaction(tx)
}

// SendTransaction will send a transactions on the topic channel
func (nf *nodeFacade) SendTransaction(tx *transaction.Transaction) (string, error) {
	return nf.node.SendTransaction(tx)
}

// GetTransaction gets the transaction with a specified hash
func (nf *nodeFacade) GetTransaction(hash string, withResults bool) (*dataAPI.Transaction, error) {
	return nf.node.GetTransaction(hash, withResults)
}

// GetAccount returns an accountResponse containing information
// about the account correlated with provided address
func (nf *nodeFacade) GetAccount(address string) (state.UserAccountHandler, error) {
	return nf.node.GetAccount(address)
}

// GetNextNonce returns account next nonce
func (nf *nodeFacade) GetNextNonce(address string) (*models.AccountNonceResponse, error) {
	nonce, firstPendingNonce, txPending, err := nf.node.GetNextNonce(address)
	if err != nil {
		return nil, err
	}

	return &models.AccountNonceResponse{
		Nonce:             nonce,
		FirstPendingNonce: firstPendingNonce,
		TxPending:         txPending,
	}, nil
}

// GetBalance gets the current balance for a specified address
func (nf *nodeFacade) GetBalance(address, kda string) (*models.BalanceResponse, error) {
	balance, err := nf.node.GetBalance(address, kda)
	if err != nil {
		return nil, err
	}

	return &models.BalanceResponse{
		Balance: balance,
	}, nil
}

// GetUserKDA returns the userKDA for a specific address
func (nf *nodeFacade) GetUserKDA(address string, kda string) (*kapps.UserKDA, error) {
	return nf.node.GetUserKDA(address, kda)
}

// GetAvailableClaim returns the rewards available for a specific asset in an account
func (nf *nodeFacade) GetAvailableClaim(address string, assetId string) (*models.AvailableClaimResponse, error) {
	stakingRewards, allStakingRewards, allowance, err := nf.node.GetAvailableClaim(address, assetId)
	if err != nil {
		return nil, err
	}

	return &models.AvailableClaimResponse{
		StakingRewards:    stakingRewards,
		AllStakingRewards: allStakingRewards,
		Allowance:         allowance,
	}, nil
}

// GetAsset returns an assetResponse containing information
// about the asset correlated with provided address
func (nf *nodeFacade) GetAsset(assetID string) (*kapps.KDAData, error) {
	return nf.node.GetAsset(assetID)
}

// GetNFT returns an assetResponse containing information
// about the asset and userKDA correlated with provided address
func (nf *nodeFacade) GetNFT(owner string, address string) (*kapps.UserKDA, *kapps.KDAData, error) {
	return nf.node.GetNFT(owner, address)
}

// GetKDAFeePool returns an poolResponse containing information
// about the pool of kda correlated with provided address
func (nf *nodeFacade) GetKDAFeePool(address string) (*kdafeespool.KDAFeesPoolData, error) {
	return nf.node.GetKDAFeePool(address)
}

// GetMarketplace returns an assetResponse containing information
// about the marketplace correlated with provided marketplaceId
func (nf *nodeFacade) GetMarketplace(marketplaceId string) (*dataAPI.Marketplace, error) {
	return nf.node.GetMarketplace(marketplaceId)
}

/****************
 *  Validators
 ****************/

// ValidatorStatisticsAPI will return the statistics for all validators
func (nf *nodeFacade) ValidatorStatisticsAPI() (map[string]*state.ValidatorApiResponse, error) {
	return nf.node.ValidatorStatisticsAPI()
}

// PeersAPI will return the peers
func (nf *nodeFacade) PeersAPI() ([]state.PeerAccountHandler, error) {
	return nf.node.PeersAPI()
}

func (nf *nodeFacade) ExecuteSCQuery(query *process.SCQuery) (*vm.VMOutputApi, error) {
	vmOutput, err := nf.apiResolver.ExecuteSCQuery(query)
	if err != nil {
		return nil, err
	}

	return nf.convertVmOutputToApiResponse(vmOutput), nil
}

func (nf *nodeFacade) convertVmOutputToApiResponse(input *vmcommon.VMOutput) *vm.VMOutputApi {
	outputAccounts := make(map[string]*vm.OutputAccountApi)
	for key, acc := range input.OutputAccounts {
		outputAddress, err := nf.node.EncodeAddressPubkey(acc.Address)
		if err != nil {
			log.Warn("cannot encode address", "error", err)
			outputAddress = ""
		}

		storageUpdates := make(map[string]*vm.StorageUpdateApi)
		for updateKey, updateVal := range acc.StorageUpdates {
			storageUpdates[hex.EncodeToString([]byte(updateKey))] = &vm.StorageUpdateApi{
				Offset: updateVal.Offset,
				Data:   updateVal.Data,
			}
		}
		outKey := hex.EncodeToString([]byte(key))
		outAcc := &vm.OutputAccountApi{
			Address:        outputAddress,
			StorageUpdates: storageUpdates,
			Code:           acc.Code,
			CodeMetadata:   acc.CodeMetadata,
		}

		outAcc.OutputTransfers = make([]vm.OutputTransferApi, len(acc.OutputTransfers))
		for i, outTransfer := range acc.OutputTransfers {
			outTransferApi := vm.OutputTransferApi{
				SenderAddress: outputAddress,
				KDATransfer: vm.KDATransferApi{
					KDAValue:      outTransfer.KDATransfers.KDAValue,
					KDATokenName:  string(outTransfer.KDATransfers.KDATokenName),
					KDATokenType:  outTransfer.KDATransfers.KDATokenType,
					KDATokenNonce: outTransfer.KDATransfers.KDATokenNonce,
				},
			}

			if len(outTransfer.SenderAddress) == len(acc.Address) && !bytes.Equal(outTransfer.SenderAddress, acc.Address) {
				senderAddr, errEncode := nf.node.EncodeAddressPubkey(outTransfer.SenderAddress)
				if errEncode != nil {
					log.Warn("cannot encode address", "error", errEncode)
					senderAddr = outputAddress
				}
				outTransferApi.SenderAddress = senderAddr
			}

			outAcc.OutputTransfers[i] = outTransferApi
		}

		outputAccounts[outKey] = outAcc
	}

	logs := make([]*vm.LogEntryApi, 0, len(input.Logs))
	for i := 0; i < len(input.Logs); i++ {
		originalLog := input.Logs[i]
		logAddress, err := nf.node.EncodeAddressPubkey(originalLog.Address)
		if err != nil {
			log.Warn("cannot encode address", "error", err)
			logAddress = ""
		}

		logs = append(logs, &vm.LogEntryApi{
			Identifier: originalLog.Identifier,
			Address:    logAddress,
			Topics:     originalLog.Topics,
			Data:       originalLog.Data,
		})
	}

	return &vm.VMOutputApi{
		ReturnData:      input.ReturnData,
		ReturnCode:      input.ReturnCode.String(),
		ReturnMessage:   input.ReturnMessage,
		GasRemaining:    input.GasRemaining,
		OutputAccounts:  outputAccounts,
		DeletedAccounts: input.DeletedAccounts,
		Logs:            logs,
	}
}

func (nf *nodeFacade) DecodeAddressPubkey(pk string) ([]byte, error) {
	return nf.node.DecodeAddressPubkey(pk)
}

// EstimateTransactionGas will calculate how many gas a transaction will consume
func (nf *nodeFacade) EstimateTransactionGas(tx *transaction.Transaction) (*transaction.CostResponse, error) {
	return nf.apiResolver.EstimateTransactionGas(tx)
}

// EstimateTransactionGas will calculate how many gas a transaction will consume
func (nf *nodeFacade) EstimateTransactionFees(tx *transaction.Transaction) (*transaction.FeesResponse, error) {
	return nf.node.EstimateTransactionFees(tx)
}
