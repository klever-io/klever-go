package presenter

import (
	"math"
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/core"
	"github.com/stretchr/testify/assert"
)

func TestPresenterStatusHandler_GetAppVersion(t *testing.T) {
	t.Parallel()

	appVersion := "version001"
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetStringValue(core.MetricAppVersion, appVersion)
	result := presenterStatusHandler.GetAppVersion()

	assert.Equal(t, appVersion, result)
}

func TestPresenterStatusHandler_GetNodeType(t *testing.T) {
	t.Parallel()

	nodeType := "validator"
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetStringValue(core.MetricNodeType, nodeType)
	result := presenterStatusHandler.GetNodeType()

	assert.Equal(t, nodeType, result)
}

func TestPresenterStatusHandler_GetPublicKeyBlockSign(t *testing.T) {
	t.Parallel()

	publicKeyBlock := "publicKeyBlockSign"
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetStringValue(core.MetricPublicKeyBlockSign, publicKeyBlock)
	result := presenterStatusHandler.GetPublicKeyBlockSign()

	assert.Equal(t, publicKeyBlock, result)
}

func TestPresenterStatusHandler_GetRedundancyIsActive(t *testing.T) {
	t.Parallel()

	t.Run("metric not present returns N/A so TUI hides redundancy row", func(t *testing.T) {
		t.Parallel()
		psh := NewPresenterStatusHandler()
		assert.Equal(t, metricNotAvailable, psh.GetRedundancyIsActive())
	})

	t.Run("uint64 1 returns true", func(t *testing.T) {
		t.Parallel()
		psh := NewPresenterStatusHandler()
		psh.SetUInt64Value(core.MetricRedundancyIsActive, 1)
		assert.Equal(t, "true", psh.GetRedundancyIsActive())
	})

	t.Run("uint64 0 returns false", func(t *testing.T) {
		t.Parallel()
		psh := NewPresenterStatusHandler()
		psh.SetUInt64Value(core.MetricRedundancyIsActive, 0)
		assert.Equal(t, "false", psh.GetRedundancyIsActive())
	})

	t.Run("wrong cached type returns N/A", func(t *testing.T) {
		t.Parallel()
		psh := NewPresenterStatusHandler()
		psh.SetStringValue(core.MetricRedundancyIsActive, "true")
		assert.Equal(t, metricNotAvailable, psh.GetRedundancyIsActive())
	})

	t.Run("uint64 outside {0,1} returns N/A instead of false", func(t *testing.T) {
		t.Parallel()
		psh := NewPresenterStatusHandler()
		psh.SetUInt64Value(core.MetricRedundancyIsActive, 2)
		assert.Equal(t, metricNotAvailable, psh.GetRedundancyIsActive())
	})
}

func TestPresenterStatusHandler_GetRedundancyLevel(t *testing.T) {
	t.Parallel()

	t.Run("not set returns 0", func(t *testing.T) {
		t.Parallel()
		psh := NewPresenterStatusHandler()
		assert.Equal(t, int64(0), psh.GetRedundancyLevel())
	})

	t.Run("main producer", func(t *testing.T) {
		t.Parallel()
		psh := NewPresenterStatusHandler()
		psh.SetInt64Value(core.MetricRedundancyLevel, 0)
		assert.Equal(t, int64(0), psh.GetRedundancyLevel())
	})

	t.Run("backup level 2", func(t *testing.T) {
		t.Parallel()
		psh := NewPresenterStatusHandler()
		psh.SetInt64Value(core.MetricRedundancyLevel, 2)
		assert.Equal(t, int64(2), psh.GetRedundancyLevel())
	})

	t.Run("permanently inactive (negative)", func(t *testing.T) {
		t.Parallel()
		psh := NewPresenterStatusHandler()
		psh.SetInt64Value(core.MetricRedundancyLevel, -1)
		assert.Equal(t, int64(-1), psh.GetRedundancyLevel())
	})

	t.Run("legacy uint64 storage tolerated for non-negative values", func(t *testing.T) {
		t.Parallel()
		psh := NewPresenterStatusHandler()
		psh.SetUInt64Value(core.MetricRedundancyLevel, 3)
		assert.Equal(t, int64(3), psh.GetRedundancyLevel())
	})

	t.Run("uint64 above MaxInt64 returns 0 instead of wrapping negative", func(t *testing.T) {
		t.Parallel()
		psh := NewPresenterStatusHandler()
		psh.SetUInt64Value(core.MetricRedundancyLevel, math.MaxInt64+1)
		assert.Equal(t, int64(0), psh.GetRedundancyLevel())
	})

	t.Run("wrong cached type returns 0", func(t *testing.T) {
		t.Parallel()
		psh := NewPresenterStatusHandler()
		psh.SetStringValue(core.MetricRedundancyLevel, "2")
		assert.Equal(t, int64(0), psh.GetRedundancyLevel())
	})
}

func TestPresenterStatusHandler_GetCountConsensus(t *testing.T) {
	t.Parallel()

	countConsensus := uint64(100)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricCountConsensus, countConsensus)
	result := presenterStatusHandler.GetCountConsensus()

	assert.Equal(t, countConsensus, result)
}

func TestPresenterStatusHandler_GetCountLeader(t *testing.T) {
	t.Parallel()

	countLeader := uint64(100)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricCountLeader, countLeader)
	result := presenterStatusHandler.GetCountLeader()

	assert.Equal(t, countLeader, result)
}

func TestPresenterStatusHandler_GetCountAcceptedBlocks(t *testing.T) {
	t.Parallel()

	countAcceptedBlocks := uint64(100)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricCountAcceptedBlocks, countAcceptedBlocks)
	result := presenterStatusHandler.GetCountAcceptedBlocks()

	assert.Equal(t, countAcceptedBlocks, result)
}

func TestPresenterStatusHandler_CheckSoftwareVersionNeedUpdate(t *testing.T) {
	t.Parallel()

	appVersion := "v20/go123/adsds"
	softwareVersion := "v21"

	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetStringValue(core.MetricAppVersion, appVersion)
	presenterStatusHandler.SetStringValue(core.MetricLatestTagSoftwareVersion, softwareVersion)
	needUpdate, latestSoftwareVersion := presenterStatusHandler.CheckSoftwareVersion()

	assert.Equal(t, true, needUpdate)
	assert.Equal(t, softwareVersion, latestSoftwareVersion)
}

func TestPresenterStatusHandler_CheckSoftwareVersion(t *testing.T) {
	t.Parallel()

	appVersion := "v21/go123/adsds"
	softwareVersion := "v21"

	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetStringValue(core.MetricAppVersion, appVersion)
	presenterStatusHandler.SetStringValue(core.MetricLatestTagSoftwareVersion, softwareVersion)
	needUpdate, latestSoftwareVersion := presenterStatusHandler.CheckSoftwareVersion()

	assert.Equal(t, false, needUpdate)
	assert.Equal(t, softwareVersion, latestSoftwareVersion)
}

func TestPresenterStatusHandler_GetCountConsensusAcceptedBlocks(t *testing.T) {
	t.Parallel()

	countConsensusAcceptedBlocks := uint64(1000)
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricCountConsensusAcceptedBlocks, countConsensusAcceptedBlocks)
	result := presenterStatusHandler.GetCountConsensusAcceptedBlocks()

	assert.Equal(t, countConsensusAcceptedBlocks, result)

}

func TestPresenterStatusHandler_GetNodeNameShouldReturnDefaultName(t *testing.T) {
	t.Parallel()

	nodeName := ""
	expectedName := "noname"
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetStringValue(core.MetricNodeDisplayName, nodeName)
	result := presenterStatusHandler.GetNodeName()

	assert.Equal(t, expectedName, result)
}

func TestPresenterStatusHandler_GetNodeName(t *testing.T) {
	t.Parallel()

	nodeName := "node"
	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetStringValue(core.MetricNodeDisplayName, nodeName)
	result := presenterStatusHandler.GetNodeName()

	assert.Equal(t, nodeName, result)
}

func TestPresenterStatusHandler_CalculateRewardsTotal(t *testing.T) {
	t.Parallel()

	rewardsValue := "100000"

	numSignedBlocks := uint64(50)

	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetStringValue(core.MetricRewardsValue, rewardsValue)
	presenterStatusHandler.SetUInt64Value(core.MetricCountConsensusAcceptedBlocks, numSignedBlocks)
	totalRewards, diff := presenterStatusHandler.GetTotalRewardsValue()
	expectedDifValue := "5" + presenterStatusHandler.GetZeros()

	assert.Equal(t, "0"+presenterStatusHandler.GetZeros(), totalRewards)
	assert.Equal(t, expectedDifValue, diff)
}

func TestPresenterStatusHandler_CalculateRewardsTotalRewards(t *testing.T) {
	t.Parallel()

	rewardsValue := "1000"
	numSignedBlocks := uint64(5000000)

	presenterStatusHandler := NewPresenterStatusHandler()
	totalRewardsOld, _ := big.NewFloat(0).SetString(rewardsValue)
	presenterStatusHandler.totalRewardsOld = big.NewFloat(0).Set(totalRewardsOld)
	presenterStatusHandler.SetStringValue(core.MetricRewardsValue, rewardsValue)
	presenterStatusHandler.SetUInt64Value(core.MetricCountConsensusAcceptedBlocks, numSignedBlocks)
	totalRewards, diff := presenterStatusHandler.GetTotalRewardsValue()
	expectedDiffValue := "4000" + presenterStatusHandler.GetZeros()

	assert.Equal(t, totalRewardsOld.Text('f', precisionRewards), totalRewards)
	assert.Equal(t, expectedDiffValue, diff)
}

func TestPresenterStatusHandler_CalculateRewardsPerHourReturnZero(t *testing.T) {
	t.Parallel()

	presenterStatusHandler := NewPresenterStatusHandler()
	result := presenterStatusHandler.CalculateRewardsPerHour()

	assert.Equal(t, "0", result)
}

func TestPresenterStatusHandler_CalculateRewardsPerHourShouldWork(t *testing.T) {
	t.Parallel()

	consensusGroupSize := uint64(50)
	numValidators := uint64(100)
	totalBlocks := uint64(1000)
	totalSlots := uint64(1000)
	slotTime := uint64(6)
	rewardsValue := "1000000"

	presenterStatusHandler := NewPresenterStatusHandler()
	presenterStatusHandler.SetUInt64Value(core.MetricConsensusGroupSize, consensusGroupSize)
	presenterStatusHandler.SetUInt64Value(core.MetricNumValidators, numValidators)
	presenterStatusHandler.SetUInt64Value(core.MetricProbableHighestNonce, totalBlocks)
	presenterStatusHandler.SetStringValue(core.MetricRewardsValue, rewardsValue)
	presenterStatusHandler.SetUInt64Value(core.MetricCurrentSlot, totalSlots)
	presenterStatusHandler.SetUInt64Value(core.MetricSlotTime, slotTime)
	expectedValue := "300" + presenterStatusHandler.GetZeros()

	result := presenterStatusHandler.CalculateRewardsPerHour()
	assert.Equal(t, expectedValue, result)
}
