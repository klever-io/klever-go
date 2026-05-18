package presenter

import (
	"math/big"
	"strings"

	"github.com/klever-io/klever-go/core"
)

const precisionRewards = 2

// GetAppVersion will return application version
func (psh *PresenterStatusHandler) GetAppVersion() string {
	return psh.getFromCacheAsString(core.MetricAppVersion)
}

// GetNodeType will return type of node
func (psh *PresenterStatusHandler) GetNodeType() string {
	return psh.getFromCacheAsString(core.MetricNodeType)
}

// GetPeerType will return type of peer (eligible, elected, and so on)
func (psh *PresenterStatusHandler) GetPeerType() string {
	return psh.getFromCacheAsString(core.MetricPeerType)
}

// GetPublicKeyBlockSign will return node public key for sign blocks
func (psh *PresenterStatusHandler) GetPublicKeyBlockSign() string {
	return psh.getFromCacheAsString(core.MetricPublicKeyBlockSign)
}

// GetRedundancyLevel returns the redundancy level (see core.MetricRedundancyLevel).
// Returns 0 for missing/wrong-type AND for the real level=0 main producer; to
// distinguish those, check GetRedundancyIsActive() for "N/A".
func (psh *PresenterStatusHandler) GetRedundancyLevel() int64 {
	return psh.getFromCacheAsInt64(core.MetricRedundancyLevel)
}

// GetRedundancyIsActive returns "true"/"false" from the uint64 metric (1/0),
// or metricNotAvailable when unset or out of range — the TUI uses the absent
// signal to hide the redundancy row at startup. The value's meaning is uniform
// across roles: "true" means this node is currently the active block producer.
func (psh *PresenterStatusHandler) GetRedundancyIsActive() string {
	val, ok := psh.presenterMetrics.Load(core.MetricRedundancyIsActive)
	if !ok {
		return metricNotAvailable
	}

	valUint64, ok := val.(uint64)
	if !ok {
		return metricNotAvailable
	}

	switch valUint64 {
	case 1:
		return "true"
	case 0:
		return "false"
	default:
		// Surface N/A rather than coercing unknown values, which would
		// misreport an undefined state.
		return metricNotAvailable
	}
}

// GetChainID will return node chainID
func (psh *PresenterStatusHandler) GetChainID() string {
	return psh.getFromCacheAsString(core.MetricChainID)
}

// GetCountConsensus will return count of how many times node was in consensus group
func (psh *PresenterStatusHandler) GetCountConsensus() uint64 {
	return psh.getFromCacheAsUint64(core.MetricCountConsensus)
}

// GetCountConsensusAcceptedBlocks will return a count if how many times the node was in consensus group and
// a block was produced
func (psh *PresenterStatusHandler) GetCountConsensusAcceptedBlocks() uint64 {
	return psh.getFromCacheAsUint64(core.MetricCountConsensusAcceptedBlocks)
}

// GetCountLeader will return count of how many time node was leader in consensus group
func (psh *PresenterStatusHandler) GetCountLeader() uint64 {
	return psh.getFromCacheAsUint64(core.MetricCountLeader)
}

// GetCountAcceptedBlocks will return count of how many accepted blocks was proposed by the node
func (psh *PresenterStatusHandler) GetCountAcceptedBlocks() uint64 {
	return psh.getFromCacheAsUint64(core.MetricCountAcceptedBlocks)
}

// CheckSoftwareVersion will check if node is the latest version and will return latest stable version
func (psh *PresenterStatusHandler) CheckSoftwareVersion() (bool, string) {
	latestStableVersion := psh.getFromCacheAsString(core.MetricLatestTagSoftwareVersion)
	appVersion := psh.getFromCacheAsString(core.MetricAppVersion)

	if strings.Contains(appVersion, latestStableVersion) || latestStableVersion == "" {
		return false, latestStableVersion
	}

	return true, latestStableVersion
}

// GetNodeName will return node's display name
func (psh *PresenterStatusHandler) GetNodeName() string {
	nodeName := psh.getFromCacheAsString(core.MetricNodeDisplayName)
	if nodeName == "" {
		nodeName = "noname"
	}

	return nodeName
}

// GetTotalRewardsValue will return total value of rewards and how the rewards were increased on every second
// Rewards estimation will be equal with :
// numSignedBlocks * denomination * Rewards
func (psh *PresenterStatusHandler) GetTotalRewardsValue() (string, string) {
	numSignedBlocks := psh.getFromCacheAsUint64(core.MetricCountConsensusAcceptedBlocks)
	rewardsInKLV := psh.computeRewardsInKLV()

	totalRewardsFloat := big.NewFloat(float64(numSignedBlocks))
	totalRewardsFloat.Mul(totalRewardsFloat, rewardsInKLV)
	difRewards := big.NewFloat(0).Sub(totalRewardsFloat, psh.totalRewardsOld)

	defer func() {
		psh.totalRewardsOld = totalRewardsFloat
	}()

	return psh.totalRewardsOld.Text('f', precisionRewards), difRewards.Text('f', precisionRewards)
}

// CalculateRewardsPerHour will return an approximation of how many KLVs a validator will earn per hour
// Rewards estimation per hour will be equals with :
// chanceToBeInConsensus * slotsPerHour * hitRate * denominationCoefficient * Rewards
func (psh *PresenterStatusHandler) CalculateRewardsPerHour() string {
	chanceToBeInConsensus := psh.computeChanceToBeInConsensus()
	slotsPerHourAccordingToHitRate := psh.computeSlotsPerHourAccordingToHitRate()
	rewardsInKLV := psh.computeRewardsInKLV()
	if chanceToBeInConsensus == 0 || slotsPerHourAccordingToHitRate == 0 || rewardsInKLV.Cmp(big.NewFloat(0)) <= 0 {
		return "0"
	}

	rewardsPerHourCoefficient := chanceToBeInConsensus * slotsPerHourAccordingToHitRate
	totalRewardsPerHourFloat := big.NewFloat(rewardsPerHourCoefficient)
	totalRewardsPerHourFloat.Mul(totalRewardsPerHourFloat, rewardsInKLV)
	return totalRewardsPerHourFloat.Text('f', precisionRewards)
}

// GetZeros will return a string with a specific number of zeros
func (psh *PresenterStatusHandler) GetZeros() string {
	retValue := "." + strings.Repeat("0", precisionRewards)
	if retValue == "." {
		return ""
	}

	return retValue
}
