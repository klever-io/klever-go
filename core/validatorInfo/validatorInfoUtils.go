package validatorInfo

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/data/state"
)

// WasActiveInCurrentEpoch returns true if the node was active in current epoch
func WasActiveInCurrentEpoch(valInfo *state.ValidatorInfo) bool {
	active := valInfo.LeaderFailure > 0 || valInfo.LeaderSuccess > 0 || valInfo.ValidatorSuccess > 0 || valInfo.ValidatorFailure > 0
	return active
}

// WasLeavingElectedInCurrentEpoch returns true if the validator was eligible in the epoch but has done an unstake.
func WasLeavingElectedInCurrentEpoch(valInfo *state.ValidatorInfo) bool {
	if valInfo == nil {
		return false
	}

	return valInfo.List == string(core.LeavingList) && WasActiveInCurrentEpoch(valInfo)
}

// WasJailedElectedInCurrentEpoch returns true if the validator was jailed in the epoch but also active/eligible due to not enough
// nodes in shard.
func WasJailedElectedInCurrentEpoch(valInfo *state.ValidatorInfo) bool {
	if valInfo == nil {
		return false
	}

	return valInfo.List == string(core.JailedList) && WasActiveInCurrentEpoch(valInfo)
}

// WasElectedInCurrentEpoch returns true if the validator was elected for consensus in current epoch
func WasElectedInCurrentEpoch(valInfo *state.ValidatorInfo) bool {
	wasElectedInShard := valInfo.List == string(core.ElectedList) ||
		WasLeavingElectedInCurrentEpoch(valInfo) ||
		WasJailedElectedInCurrentEpoch(valInfo)

	return wasElectedInShard
}
