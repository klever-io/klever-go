package termuiRenders

import (
	"testing"

	"github.com/gizak/termui/v3/widgets"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/statusHandler/presenter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeRedundancyStr(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		level          int64
		isActive       string
		expected       string
		expectedHidden bool
	}{
		{
			name:           "metric not yet polled hides the row",
			level:          0,
			isActive:       statusNotApplicable,
			expectedHidden: true,
		},
		{
			name:     "main producer has no suffix",
			level:    0,
			isActive: "true",
			expected: "Redundancy: main machine",
		},
		{
			name:     "inactive node has no suffix",
			level:    -1,
			isActive: "false",
			expected: "Redundancy: inactive",
		},
		{
			name:     "backup on standby",
			level:    2,
			isActive: "false",
			expected: "Redundancy: back-up #2 (standby)",
		},
		{
			name:     "backup taking over",
			level:    1,
			isActive: "true",
			expected: "Redundancy: back-up #1 (taking over)",
		},
		{
			name:     "main with is_active=false still shows just main (no suffix)",
			level:    0,
			isActive: "false",
			expected: "Redundancy: main machine",
		},
		{
			name:     "deeply negative level still shows inactive",
			level:    -100,
			isActive: "false",
			expected: "Redundancy: inactive",
		},
		{
			name:     "high backup rank formats correctly",
			level:    100,
			isActive: "false",
			expected: "Redundancy: back-up #100 (standby)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := computeRedundancyStr(tc.level, tc.isActive)
			if tc.expectedHidden {
				assert.Equal(t, "", got)
				return
			}
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestPrepareBlockInfoConsensusSlotState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		syncing   uint64
		nodeType  string
		peerType  string
		slotState string
		want      string
	}{
		{
			name:     "syncing hides the slot state",
			syncing:  1,
			nodeType: string(core.NodeTypeValidator),
			peerType: string(core.JailedList),
			want:     "Consensus slot state: N/A (syncing)",
		},
		{
			name:     "observer",
			nodeType: string(core.NodeTypeObserver),
			want:     "Consensus slot state: N/A (observer)",
		},
		{
			name:     "waiting validator",
			nodeType: string(core.NodeTypeValidator),
			peerType: string(core.WaitingList),
			want:     "Consensus slot state: N/A (waiting)",
		},
		{
			name:      "jailed validator does not show signed",
			nodeType:  string(core.NodeTypeValidator),
			peerType:  string(core.JailedList),
			slotState: "signed",
			want:      "Consensus slot state: N/A (jailed)",
		},
		{
			name:      "elected validator shows the metric",
			nodeType:  string(core.NodeTypeValidator),
			peerType:  string(core.ElectedList),
			slotState: "signed",
			want:      "Consensus slot state: signed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := presenter.NewPresenterStatusHandler()
			p.SetUInt64Value(core.MetricIsSyncing, tc.syncing)
			p.SetStringValue(core.MetricNodeType, tc.nodeType)
			p.SetStringValue(core.MetricPeerType, tc.peerType)
			p.SetStringValue(core.MetricConsensusSlotState, tc.slotState)

			wr := &WidgetsRender{
				presenter: p,
				blockInfo: widgets.NewTable(),
			}
			wr.prepareBlockInfo()

			require.Greater(t, len(wr.blockInfo.Rows), 6)
			assert.Equal(t, []string{tc.want}, wr.blockInfo.Rows[6])
		})
	}
}
