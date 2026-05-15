package termuiRenders

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
