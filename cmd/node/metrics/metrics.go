package metrics

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/appStatusPolling"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/factory"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

const millisecondsInSecond = 1000

// InitMetrics will init metrics for status handler
func InitMetrics(
	appStatusHandler core.AppStatusHandler,
	pubkeyStr string,
	nodeType core.NodeType,
	nodesConfig *sharding.NodesSetup,
	version string,
	genesisTotalSupply string,
	slotsPerEpoch uint64,
) error {
	if check.IfNil(appStatusHandler) {
		return fmt.Errorf("nil AppStatusHandler when initializing metrics")
	}
	if nodesConfig == nil {
		return fmt.Errorf("nil nodes config when initializing metrics")
	}

	slotInterval := nodesConfig.SlotInterval
	isSyncing := uint64(1)
	initUint := uint64(0)
	initString := ""
	initZeroString := "0"

	appStatusHandler.SetStringValue(core.MetricPublicKeyBlockSign, pubkeyStr)
	appStatusHandler.SetStringValue(core.MetricNodeType, string(nodeType))
	appStatusHandler.SetUInt64Value(core.MetricSlotTime, slotInterval/millisecondsInSecond)
	appStatusHandler.SetStringValue(core.MetricAppVersion, version)
	appStatusHandler.SetUInt64Value(core.MetricSlotsPerEpoch, uint64(slotsPerEpoch))
	appStatusHandler.SetUInt64Value(core.MetricCountConsensus, initUint)
	appStatusHandler.SetUInt64Value(core.MetricCountLeader, initUint)
	appStatusHandler.SetUInt64Value(core.MetricCountAcceptedBlocks, initUint)
	appStatusHandler.SetUInt64Value(core.MetricNumTxInBlock, initUint)
	appStatusHandler.SetStringValue(core.MetricConsensusState, initString)
	appStatusHandler.SetStringValue(core.MetricConsensusSlotState, initString)
	appStatusHandler.SetUInt64Value(core.MetricIsSyncing, isSyncing)
	appStatusHandler.SetStringValue(core.MetricCurrentBlockHash, initString)
	appStatusHandler.SetUInt64Value(core.MetricNumProcessedTxs, initUint)
	appStatusHandler.SetUInt64Value(core.MetricCurrentSlotTimestamp, initUint)
	appStatusHandler.SetUInt64Value(core.MetricHeaderSize, initUint)
	appStatusHandler.SetUInt64Value(core.MetricBodyBlocksSize, initUint)
	appStatusHandler.SetUInt64Value(core.MetricTXsBlocksSize, initUint)
	appStatusHandler.SetUInt64Value(core.MetricHighestFinalBlock, initUint)
	appStatusHandler.SetUInt64Value(core.MetricCountConsensusAcceptedBlocks, initUint)
	appStatusHandler.SetUInt64Value(core.MetricSlotAtEpochStart, initUint)
	appStatusHandler.SetUInt64Value(core.MetricNonceAtEpochStart, initUint)
	appStatusHandler.SetUInt64Value(core.MetricSlotsPassedInCurrentEpoch, initUint)
	appStatusHandler.SetUInt64Value(core.MetricNoncesPassedInCurrentEpoch, initUint)
	appStatusHandler.SetUInt64Value(core.MetricNumConnectedPeers, initUint)
	appStatusHandler.SetStringValue(core.MetricLatestTagSoftwareVersion, initString)

	appStatusHandler.SetStringValue(core.MetricP2PPeerInfo, initString)
	appStatusHandler.SetStringValue(core.MetricP2PIntraShardValidators, initString)
	appStatusHandler.SetStringValue(core.MetricP2PIntraShardObservers, initString)
	appStatusHandler.SetStringValue(core.MetricP2PCrossShardValidators, initString)
	appStatusHandler.SetStringValue(core.MetricP2PCrossShardObservers, initString)
	appStatusHandler.SetStringValue(core.MetricP2PUnknownPeers, initString)
	appStatusHandler.SetUInt64Value(core.MetricNumNodes, uint64(nodesConfig.MinNodes))
	appStatusHandler.SetUInt64Value(core.MetricStartTime, tools.SafeI64ToU64(nodesConfig.StartTime))
	appStatusHandler.SetUInt64Value(core.MetricSlotInterval, nodesConfig.SlotInterval)
	appStatusHandler.SetUInt64Value(core.MetricMinTransactionVersion, uint64(nodesConfig.MinTransactionVersion))
	appStatusHandler.SetStringValue(core.MetricTotalSupply, genesisTotalSupply)
	appStatusHandler.SetStringValue(core.MetricInflation, initZeroString)
	appStatusHandler.SetStringValue(core.MetricDevRewards, initZeroString)
	appStatusHandler.SetStringValue(core.MetricTotalFees, initZeroString)
	appStatusHandler.SetUInt64Value(core.MetricEpochForEconomicsData, initUint)
	appStatusHandler.SetUInt64Value(core.MetricBlockProcessDuration, initUint)
	appStatusHandler.SetUInt64Value(core.MetricBlockCommitDuration, initUint)
	appStatusHandler.SetUInt64Value(core.MetricTxProcessingDuration, initUint)
	appStatusHandler.SetUInt64Value(core.MetricSystemCPUPercent, initUint)
	appStatusHandler.SetUInt64Value(core.MetricDiskTotalBytes, initUint)
	appStatusHandler.SetUInt64Value(core.MetricDiskAvailableBytes, initUint)
	appStatusHandler.SetUInt64Value(core.MetricDiskUsagePercent, initUint)
	appStatusHandler.SetUInt64Value(core.MetricDbSizeBytes, initUint)
	appStatusHandler.SetUInt64Value(core.MetricNodeUptimeSeconds, initUint)
	appStatusHandler.SetUInt64Value(core.MetricNodeStartTimestamp, initUint)
	appStatusHandler.SetUInt64Value(core.MetricRedundancySlotsInactive, initUint)
	appStatusHandler.SetInt64Value(core.MetricRedundancyLevel, 0)
	// MetricRedundancyIsActive is intentionally NOT pre-initialised so the
	// presenter sees "key absent" and hides the TUI redundancy row until the
	// first polling run publishes the real value. MetricRedundancyIsMainActive
	// is only emitted at runtime when level>0, so it is also not pre-initialised.

	validatorsNodes, _, err := nodesConfig.InitialNodesInfo()
	if err != nil {
		return err
	}

	appStatusHandler.SetUInt64Value(core.MetricNumValidators, uint64(len(validatorsNodes)))
	appStatusHandler.SetUInt64Value(core.MetricConsensusGroupSize, uint64(nodesConfig.ConsensusGroupSize))

	return nil
}

// SaveUint64Metric will save a uint64 metric in status handler
func SaveUint64Metric(ash core.AppStatusHandler, key string, value uint64) {
	ash.SetUInt64Value(key, value)
}

// SaveStringMetric will save a string metric in status handler
func SaveStringMetric(ash core.AppStatusHandler, key, value string) {
	ash.SetStringValue(key, value)
}

// StartStatusPolling will start save information in status handler about network.
// The returned io.Closer stops the polling goroutine.
func StartStatusPolling(
	ash core.AppStatusHandler,
	pollingInterval time.Duration,
	networkComponents *factory.NetworkComponents,
	processComponents *factory.Process,
) (io.Closer, error) {
	if check.IfNil(ash) {
		return nil, errors.New("nil AppStatusHandler")
	}
	if networkComponents == nil {
		return nil, errors.New("nil networkComponents")
	}
	if processComponents == nil {
		return nil, errors.New("nil processComponents")
	}

	appStatusPollingHandler, err := appStatusPolling.NewAppStatusPolling(ash, pollingInterval)
	if err != nil {
		return nil, fmt.Errorf("cannot init AppStatusPolling: %w", err)
	}

	err = registerPollConnectedPeers(appStatusPollingHandler, networkComponents)
	if err != nil {
		return nil, err
	}

	err = registerPollProbableHighestNonce(appStatusPollingHandler, processComponents)
	if err != nil {
		return nil, err
	}

	appStatusPollingHandler.Poll()

	return appStatusPollingHandler, nil
}

func registerPollConnectedPeers(
	appStatusPollingHandler *appStatusPolling.AppStatusPolling,
	networkComponents *factory.NetworkComponents,
) error {

	p2pMetricsHandlerFunc := func(appStatusHandler core.AppStatusHandler) {
		computeNumConnectedPeers(appStatusHandler, networkComponents)
		computeConnectedPeers(appStatusHandler, networkComponents)
	}

	err := appStatusPollingHandler.RegisterPollingFunc(p2pMetricsHandlerFunc)
	if err != nil {
		return errors.New("cannot register handler func for num of connected peers")
	}

	return nil
}

func computeNumConnectedPeers(
	appStatusHandler core.AppStatusHandler,
	networkComponents *factory.NetworkComponents,
) {
	numOfConnectedPeers := uint64(len(networkComponents.NetMessenger.ConnectedAddresses()))
	appStatusHandler.SetUInt64Value(core.MetricNumConnectedPeers, numOfConnectedPeers)
}

func computeConnectedPeers(
	appStatusHandler core.AppStatusHandler,
	networkComponents *factory.NetworkComponents,
) {
	peersInfo := networkComponents.NetMessenger.GetConnectedPeersInfo()

	setP2pConnectedPeersMetrics(appStatusHandler, peersInfo)
	setCurrentP2pNodeAddresses(appStatusHandler, networkComponents)
}

func setP2pConnectedPeersMetrics(appStatusHandler core.AppStatusHandler, info *p2p.ConnectedPeersInfo) {
	appStatusHandler.SetStringValue(core.MetricP2PUnknownPeers, sliceToString(info.UnknownPeers))
	appStatusHandler.SetStringValue(core.MetricP2PIntraShardValidators, mapToString(info.IntraShardValidators))
	appStatusHandler.SetStringValue(core.MetricP2PIntraShardObservers, mapToString(info.IntraShardObservers))
	appStatusHandler.SetStringValue(core.MetricP2PCrossShardValidators, mapToString(info.CrossShardValidators))
	appStatusHandler.SetStringValue(core.MetricP2PCrossShardObservers, mapToString(info.CrossShardObservers))
}

func sliceToString(input []string) string {
	return strings.Join(input, ",")
}

func mapToString(input map[uint32][]string) string {
	strs := make([]string, 0, len(input))
	keys := make([]uint32, 0, len(input))
	for shard := range input {
		keys = append(keys, shard)
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	for _, key := range keys {
		strs = append(strs, sliceToString(input[key]))
	}

	return strings.Join(strs, ",")
}

func setCurrentP2pNodeAddresses(
	appStatusHandler core.AppStatusHandler,
	networkComponents *factory.NetworkComponents,
) {
	appStatusHandler.SetStringValue(core.MetricP2PPeerInfo, sliceToString(networkComponents.NetMessenger.Addresses()))
}

// StartNodeMetricsPolling starts polling for uptime and redundancy metrics on a single shared
// polling goroutine, avoiding the overhead of separate AppStatusPolling instances for each.
// The returned io.Closer stops the polling goroutine.
func StartNodeMetricsPolling(
	ash core.AppStatusHandler,
	pollingInterval time.Duration,
	redundancyHandler consensus.NodeRedundancyHandler,
) (io.Closer, error) {
	if check.IfNil(ash) {
		return nil, errors.New("nil AppStatusHandler")
	}
	if check.IfNil(redundancyHandler) {
		return nil, errors.New("nil NodeRedundancyHandler")
	}

	startTime := time.Now()
	ash.SetUInt64Value(core.MetricNodeStartTimestamp, uint64(startTime.Unix())) // #nosec G115

	appStatusPollingHandler, err := appStatusPolling.NewAppStatusPolling(ash, pollingInterval)
	if err != nil {
		return nil, fmt.Errorf("cannot init AppStatusPolling for node metrics: %w", err)
	}

	pollingFunc := buildNodeMetricsPollingFunc(startTime, redundancyHandler)
	err = appStatusPollingHandler.RegisterPollingFunc(pollingFunc)
	if err != nil {
		return nil, fmt.Errorf("cannot register node metrics polling function: %w", err)
	}

	// Prime before the recurring poll so /node/status returns the real level on
	// request 1 (AppStatusPolling.Poll sleeps before its first tick).
	pollingFunc(ash)

	appStatusPollingHandler.Poll()
	return appStatusPollingHandler, nil
}

func buildNodeMetricsPollingFunc(
	startTime time.Time,
	redundancyHandler consensus.NodeRedundancyHandler,
) func(core.AppStatusHandler) {
	return func(appStatusHandler core.AppStatusHandler) {
		appStatusHandler.SetUInt64Value(core.MetricNodeUptimeSeconds, uint64(time.Since(startTime).Seconds())) // #nosec G115

		// Snapshot the redundancy state under a single lock so the metrics
		// triple cannot disagree across separate getter calls (e.g. slots
		// reported pre-increment while is_main_active reflects post-increment).
		// redundancyLevel itself is effectively immutable post-construction —
		// SetInternalRedundancyLevel mutates slotsOfInactivity, not the level —
		// but reading slots and isMainMachineActive together still requires the
		// shared lock to stay consistent across slot-handler activity.
		level, slotsOfInactivity, upstreamAlive := redundancyHandler.Snapshot()

		appStatusHandler.SetUInt64Value(core.MetricRedundancySlotsInactive, slotsOfInactivity)
		appStatusHandler.SetInt64Value(core.MetricRedundancyLevel, level)

		var isActive uint64
		switch {
		case level == 0:
			// Main producer: active for its slots whenever it is emitting metrics.
			isActive = 1
		case level > 0:
			// Backup: active only when the upstream chain has gone silent.
			// Also emit is_main_active so operators can monitor upstream health
			// independently of failover status; this metric is backup-only.
			if !upstreamAlive {
				isActive = 1
			}
			var isMainActive uint64
			if upstreamAlive {
				isMainActive = 1
			}
			appStatusHandler.SetUInt64Value(core.MetricRedundancyIsMainActive, isMainActive)
		case level < 0:
			// Inactive node: never produces. The upstreamAlive bit from the
			// snapshot reflects the backup-perspective formula and is not
			// meaningful here, so it is deliberately ignored.
		}
		appStatusHandler.SetUInt64Value(core.MetricRedundancyIsActive, isActive)
	}
}

func registerPollProbableHighestNonce(
	appStatusPollingHandler *appStatusPolling.AppStatusPolling,
	processComponents *factory.Process,
) error {

	probableHighestNonceHandlerFunc := func(appStatusHandler core.AppStatusHandler) {
		probableHigherNonce := processComponents.ForkDetector.ProbableHighestNonce()
		appStatusHandler.SetUInt64Value(core.MetricProbableHighestNonce, probableHigherNonce)
	}

	err := appStatusPollingHandler.RegisterPollingFunc(probableHighestNonceHandlerFunc)
	if err != nil {
		return errors.New("cannot register handler func for forkdetector's probable higher nonce")
	}

	return nil
}
