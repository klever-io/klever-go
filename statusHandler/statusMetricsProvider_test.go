package statusHandler_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/statusHandler"
	"github.com/stretchr/testify/assert"
)

func TestNewStatusMetricsProvider(t *testing.T) {
	t.Parallel()

	sm := statusHandler.NewStatusMetrics()
	assert.NotNil(t, sm)
	assert.False(t, sm.IsInterfaceNil())
}

func TestStatusMetricsProvider_IncrementCallNonExistingKey(t *testing.T) {
	t.Parallel()

	sm := statusHandler.NewStatusMetrics()
	key1 := "test-key1"
	sm.Increment(key1)

	retMap := sm.StatusMetricsMap()

	assert.Nil(t, retMap[key1])
}

func TestStatusMetricsProvider_IncrementNonUint64ValueShouldNotWork(t *testing.T) {
	t.Parallel()

	sm := statusHandler.NewStatusMetrics()
	key1 := "test-key2"
	value1 := "value2"
	// set a key which is initialized with a string and the Increment method won't affect the key
	sm.SetStringValue(key1, value1)
	sm.Increment(key1)

	retMap := sm.StatusMetricsMap()
	assert.Equal(t, value1, retMap[key1])
}

func TestStatusMetricsProvider_IncrementShouldWork(t *testing.T) {
	t.Parallel()

	sm := statusHandler.NewStatusMetrics()
	key1 := "test-key3"
	sm.SetUInt64Value(key1, 0)
	sm.Increment(key1)

	retMap := sm.StatusMetricsMap()

	assert.Equal(t, uint64(1), retMap[key1])
}

func TestStatusMetricsProvider_Decrement(t *testing.T) {
	t.Parallel()

	sm := statusHandler.NewStatusMetrics()
	key := "test-key4"
	sm.SetUInt64Value(key, 2)
	sm.Decrement(key)

	retMap := sm.StatusMetricsMap()

	assert.Equal(t, uint64(1), retMap[key])
}

func TestStatusMetricsProvider_SetInt64Value(t *testing.T) {
	t.Parallel()

	sm := statusHandler.NewStatusMetrics()
	key := "test-key5"
	value := int64(5)
	sm.SetInt64Value(key, value)
	sm.Decrement(key)

	retMap := sm.StatusMetricsMap()

	assert.Equal(t, value, retMap[key])
}

func TestStatusMetricsProvider_SetStringValue(t *testing.T) {
	t.Parallel()

	sm := statusHandler.NewStatusMetrics()
	key := "test-key6"
	value := "value"
	sm.SetStringValue(key, value)
	sm.Decrement(key)

	retMap := sm.StatusMetricsMap()

	assert.Equal(t, value, retMap[key])
}

func TestStatusMetricsProvider_AddUint64Value(t *testing.T) {
	t.Parallel()

	sm := statusHandler.NewStatusMetrics()
	key := "test-key6"
	value := uint64(100)
	sm.SetUInt64Value(key, value)
	sm.AddUint64(key, value)

	retMap := sm.StatusMetricsMap()
	assert.Equal(t, value+value, retMap[key])
}

func TestStatusMetrics_StatusMetricsWithoutP2PPrometheusStringShouldPutCorrectShardIDLabel(t *testing.T) {
	t.Parallel()
	sm := statusHandler.NewStatusMetrics()
	key1, value1 := "test-key7", uint64(100)
	sm.SetStringValue(core.MetricChainID, "meta")
	sm.SetUInt64Value(key1, value1)

	strRes := sm.StatusMetricsWithoutP2PPrometheusString()

	expectedMetricOutput := fmt.Sprintf("%s{chainID=\"%s\"} %v\n", key1, "meta", value1)
	assert.True(t, strings.Contains(strRes, expectedMetricOutput))
}

func TestStatusMetrics_NetworkConfig(t *testing.T) {
	t.Parallel()

	sm := statusHandler.NewStatusMetrics()

	sm.SetUInt64Value(core.MetricNumMetachainNodes, 50)
	sm.SetUInt64Value(core.MetricConsensusGroupSize, 25)
	sm.SetStringValue(core.MetricChainID, "local-id")
	sm.SetUInt64Value(core.MetricSlotInterval, 5000)
	sm.SetUInt64Value(core.MetricStartTime, 9999)
	sm.SetUInt64Value(core.MetricSlotsPerEpoch, uint64(144))

	expectedConfig := map[string]interface{}{
		"klv_base_tx_size":         250,
		"klv_chain_id":             "local-id",
		"klv_consensus_group_size": uint64(25),
		"klv_num_metachain_nodes":  uint64(50),
		"klv_slot_duration":        uint64(5000),
		"klv_start_time":           uint64(9999),
		"klv_slots_per_epoch":      uint64(144),
	}

	configMetrics := sm.ConfigMetrics()
	assert.Equal(t, expectedConfig, configMetrics)
}
