package validatorInfo

import (
	"testing"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/state"
	"github.com/stretchr/testify/require"
)

func Test_IsLeavingElected_NilValidatorStatisticsDoesNotErr(t *testing.T) {
	isLeavingElected := WasLeavingElectedInCurrentEpoch(nil)

	require.False(t, isLeavingElected)
}

func Test_IsLeavingElected_Eligible(t *testing.T) {
	valInfo := &state.ValidatorInfo{
		List:             string(core.EligibleList),
		LeaderSuccess:    0,
		LeaderFailure:    0,
		ValidatorSuccess: 0,
		ValidatorFailure: 0,
	}

	isLeavingElected := WasLeavingElectedInCurrentEpoch(valInfo)
	require.False(t, isLeavingElected)
}

func Test_IsLeavingElected_NotEligibleNotLeaving(t *testing.T) {
	valInfo := &state.ValidatorInfo{
		List:             string(core.InactiveList),
		LeaderSuccess:    1,
		LeaderFailure:    10,
		ValidatorSuccess: 11,
		ValidatorFailure: 11,
	}

	isLeavingElected := WasLeavingElectedInCurrentEpoch(valInfo)
	require.False(t, isLeavingElected)
}

func Test_IsLeavingElected_LeavingNoData(t *testing.T) {
	valInfo := &state.ValidatorInfo{
		List:             string(core.LeavingList),
		LeaderSuccess:    0,
		LeaderFailure:    0,
		ValidatorSuccess: 0,
		ValidatorFailure: 0,
	}

	isLeavingElected := WasLeavingElectedInCurrentEpoch(valInfo)
	require.False(t, isLeavingElected)
}

func Test_IsLeavingElected_LeavingWithData(t *testing.T) {
	// should be considered leaving eligible

	valInfo := &state.ValidatorInfo{
		List:             string(core.LeavingList),
		LeaderSuccess:    1,
		LeaderFailure:    10,
		ValidatorSuccess: 11,
		ValidatorFailure: 11,
	}

	isLeavingElected := WasLeavingElectedInCurrentEpoch(valInfo)
	require.True(t, isLeavingElected)
}
