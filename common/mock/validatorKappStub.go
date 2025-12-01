package mock

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/sharding"
)

type ValidatorsKAppStub struct {
	ResetValidatorStatisticsAtNewEpochCalled func([]*state.ValidatorInfo) ([]*state.ValidatorInfo, error)
	ProcessRatingsEndOfEpochCalled           func([]*state.ValidatorInfo) error
	UndelegateCalled                         func(blockEpoch uint32, validator []byte, sender []byte, tc *transaction.UndelegateContract) (transaction.Transaction_TXResultCode, error)
	ClaimPendingRewardsCalled                func(address []byte) (int64, error)
	GetPendingRewardsCalled                  func(address []byte) (int64, error)
}

// SetKAppController sets the KApp controller.
func (v *ValidatorsKAppStub) SetKAppController(controller kapp.KAppController) error {
	// Stub implementation
	return nil
}

// Register registers a new validator.
func (v *ValidatorsKAppStub) Register(tc *transaction.CreateValidatorContract) (transaction.Transaction_TXResultCode, error) {
	// Stub implementation
	return transaction.Transaction_TXResultCode(0), nil
}

// UpdateValidator updates the validator's configuration.
func (v *ValidatorsKAppStub) UpdateValidator(sender []byte, tc *transaction.ValidatorConfigContract) (transaction.Transaction_TXResultCode, error) {
	// Stub implementation
	return transaction.Transaction_TXResultCode(0), nil
}

// Delegate handles validator delegation.
func (v *ValidatorsKAppStub) Delegate(sender []byte, blockTime int64, blockEpoch uint32, tc *transaction.DelegateContract) (transaction.Transaction_TXResultCode, [][]byte, error) {
	// Stub implementation
	return transaction.Transaction_TXResultCode(0), nil, nil
}

// Undelegate handles validator undelegation.
func (v *ValidatorsKAppStub) Undelegate(blockEpoch uint32, validator []byte, sender []byte, tc *transaction.UndelegateContract) (transaction.Transaction_TXResultCode, error) {
	if v.UndelegateCalled != nil {
		return v.UndelegateCalled(blockEpoch, validator, sender, tc)
	}
	return transaction.Transaction_Ok, nil
}

// Unjail unjails a jailed validator.
func (v *ValidatorsKAppStub) Unjail(sender []byte, tc *transaction.UnjailContract) (transaction.Transaction_TXResultCode, error) {
	// Stub implementation
	return transaction.Transaction_TXResultCode(0), nil
}

// ProcessEconomicsEndOfEpoch processes the economics at the end of the epoch.
func (v *ValidatorsKAppStub) ProcessEconomicsEndOfEpoch(currentEpoch uint32, validatorInfos []*state.ValidatorInfo) error {
	// Stub implementation
	return nil
}

// SaveUpdatesForNodesMap saves updates for the node owners map.
func (v *ValidatorsKAppStub) SaveUpdatesForNodesMap(nodeOwners [][]byte, peerType string) (bool, error) {
	// Stub implementation
	return false, nil
}

// GetValidatorsInfo gets the validator information for a list of validators.
func (v *ValidatorsKAppStub) GetValidatorsInfo(validators [][]byte) ([]kapp.ValidatorAccountInfoHandler, error) {
	// Stub implementation
	return nil, nil
}

// DecreaseTempRating decreases the temporary rating of a validator.
func (v *ValidatorsKAppStub) DecreaseTempRating(validator []byte, isProposer bool) error {
	// Stub implementation
	return nil
}

// SetRater sets the rater for the validators.
func (v *ValidatorsKAppStub) SetRater(rater sharding.PeerAccountListAndRatingHandler) {
	// Stub implementation
}

// SetAccountsCacher sets the accounts cacher.
func (v *ValidatorsKAppStub) SetAccountsCacher(cacher state.AccountsCacher) error {
	// Stub implementation
	return nil
}

// GetAccountsCacher gets the accounts cacher.
func (v *ValidatorsKAppStub) GetAccountsCacher() state.AccountsCacher {
	// Stub implementation
	return nil
}

// UpdateMissedBlocksCounters updates missed blocks counters.
func (v *ValidatorsKAppStub) UpdateMissedBlocksCounters(mb map[string]kapp.RateChange) error {
	// Stub implementation
	return nil
}

// ResetValidatorStatisticsAtNewEpoch resets the validator statistics at the new epoch.
func (v *ValidatorsKAppStub) ResetValidatorStatisticsAtNewEpoch(vInfos []*state.ValidatorInfo) ([]*state.ValidatorInfo, error) {
	if v.ResetValidatorStatisticsAtNewEpochCalled != nil {
		return v.ResetValidatorStatisticsAtNewEpochCalled(vInfos)
	}

	return nil, nil
}

// ProcessRatingsEndOfEpoch processes ratings at the end of the epoch.
func (v *ValidatorsKAppStub) ProcessRatingsEndOfEpoch(validatorInfos []*state.ValidatorInfo) error {
	if v.ProcessRatingsEndOfEpochCalled != nil {
		return v.ProcessRatingsEndOfEpochCalled(validatorInfos)
	}

	return nil
}

// UpdateValidatorInfoOnSuccessfulBlock updates the validator information after a successful block.
func (v *ValidatorsKAppStub) UpdateValidatorInfoOnSuccessfulBlock(validatorList []sharding.Validator, signingBitmap []byte, accumulatedFees int64) error {
	// Stub implementation
	return nil
}

// DecreaseAll decreases the rating of all validators based on missed slots.
func (v *ValidatorsKAppStub) DecreaseAll(validators [][]byte, missedSlots uint64, consensusGroupSize int) error {
	// Stub implementation
	return nil
}

// DisplayRating displays the rating of the validators.
func (v *ValidatorsKAppStub) DisplayRating(owners [][]byte) {
	// Stub implementation
}

// GetPendingRewards retrieves pending rewards for a user address (v2 epoch rewards).
func (v *ValidatorsKAppStub) GetPendingRewards(address []byte) (int64, error) {
	if v.GetPendingRewardsCalled != nil {
		return v.GetPendingRewardsCalled(address)
	}
	return 0, nil
}

// ClaimPendingRewards claims pending rewards for a user from the KApp data trie.
func (v *ValidatorsKAppStub) ClaimPendingRewards(address []byte) (int64, error) {
	if v.ClaimPendingRewardsCalled != nil {
		return v.ClaimPendingRewardsCalled(address)
	}
	return 0, nil
}

// IsInterfaceNil checks if the interface is nil.
func (v *ValidatorsKAppStub) IsInterfaceNil() bool {
	// Stub implementation
	return false
}
