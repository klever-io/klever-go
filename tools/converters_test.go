package tools

import (
	"math"
	"testing"

	"github.com/klever-io/klever-go/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ComputePercentageI64(t *testing.T) {
	tests := []struct {
		description    string
		value          int64
		percentage     int64
		expectedErr    error
		expectedResult int64
		forkActive     bool
	}{
		{
			description:    "should work pre fork",
			value:          1000000,
			percentage:     2000,
			expectedErr:    nil,
			expectedResult: 200000,
			forkActive:     false,
		},
		{
			description:    "should overflow pre fork",
			value:          math.MaxInt64,
			percentage:     math.MaxInt64,
			expectedErr:    nil,
			expectedResult: 0,
			forkActive:     false,
		},
		{
			description:    "should work",
			value:          1000000,
			percentage:     2000,
			expectedErr:    nil,
			expectedResult: 200000,
			forkActive:     true,
		},
		{
			description:    "Dust value",
			value:          1001,
			percentage:     3333,
			expectedResult: 333,
			expectedErr:    nil,
		},
		{
			description:    "Zero percentage",
			value:          1000,
			percentage:     0,
			expectedResult: 0,
			expectedErr:    nil,
		},
		{
			description:    "should overflow",
			value:          math.MaxInt64,
			percentage:     math.MaxInt64,
			expectedErr:    common.ErrInt64Overflow,
			expectedResult: 0,
			forkActive:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			result, err := ComputePercentageI64(tt.value, tt.percentage, tt.forkActive)
			require.Equal(tt.expectedErr, err)

			if tt.forkActive { // pre-fork overflow value can change
				assert.Equal(tt.expectedResult, result)
			}

		})
	}

}
