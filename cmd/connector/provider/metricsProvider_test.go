package provider

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/statusHandler/presenter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Non-fixed path: JSON -1 → float64(-1) → uint64(0) on gc → int64(0), which
// would display an inactive node as a main producer.
func TestSetPresenterValue_ConnectorPath_PreservesNegativeRedundancyLevel(t *testing.T) {
	t.Parallel()

	rawJSON := []byte(`{"klv_redundancy_level": -1, "klv_num_tx_block": 42}`)
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(rawJSON, &decoded))
	require.IsType(t, float64(0), decoded[core.MetricRedundancyLevel])

	psh := presenter.NewPresenterStatusHandler()
	smp := &StatusMetricsProvider{presenter: psh}

	for key, value := range decoded {
		require.NoError(t, smp.setPresenterValue(key, value))
	}

	assert.Equal(t, int64(-1), psh.GetRedundancyLevel())
	assert.Equal(t, uint64(42), psh.GetNumTxInBlock())
}

func TestSetPresenterValue_NonSignedMetricStaysUInt64(t *testing.T) {
	t.Parallel()

	psh := presenter.NewPresenterStatusHandler()
	smp := &StatusMetricsProvider{presenter: psh}

	require.NoError(t, smp.setPresenterValue(core.MetricNumTxInBlock, float64(1234)))
	assert.Equal(t, uint64(1234), psh.GetNumTxInBlock())
}

func TestSetPresenterValue_StringMetric(t *testing.T) {
	t.Parallel()

	psh := presenter.NewPresenterStatusHandler()
	smp := &StatusMetricsProvider{presenter: psh}

	require.NoError(t, smp.setPresenterValue(core.MetricChainID, "klv-test"))
	assert.Equal(t, "klv-test", psh.GetChainID())
}

func TestSetPresenterValue_UnsupportedType(t *testing.T) {
	t.Parallel()

	psh := presenter.NewPresenterStatusHandler()
	smp := &StatusMetricsProvider{presenter: psh}

	err := smp.setPresenterValue(core.MetricChainID, []byte{1, 2, 3})
	assert.ErrorIs(t, err, ErrTypeAssertionFailed)
}

func TestSetPresenterValue_RejectsOutOfRangeFloats(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		key   string
		value float64
	}{
		{"NaN signed", core.MetricRedundancyLevel, math.NaN()},
		{"+Inf signed", core.MetricRedundancyLevel, math.Inf(1)},
		{"-Inf signed", core.MetricRedundancyLevel, math.Inf(-1)},
		{"above MaxInt64 signed", core.MetricRedundancyLevel, math.MaxInt64 * 2.0},
		{"below MinInt64 signed", core.MetricRedundancyLevel, -math.MaxInt64 * 2.0},
		{"NaN unsigned", core.MetricNumTxInBlock, math.NaN()},
		{"+Inf unsigned", core.MetricNumTxInBlock, math.Inf(1)},
		{"negative unsigned", core.MetricNumTxInBlock, -1.0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			psh := presenter.NewPresenterStatusHandler()
			smp := &StatusMetricsProvider{presenter: psh}
			err := smp.setPresenterValue(tc.key, tc.value)
			assert.ErrorIs(t, err, ErrTypeAssertionFailed,
				"out-of-range float64 must be rejected to avoid implementation-defined cast")
		})
	}
}
