package validators

import (
	"fmt"
	"math"
	"math/big"

	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/validatorInfo"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/sharding"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
)

// ProcessRatingsEndOfEpoch makes end of epoch process on the rating
func (v *validatorsKApp) ProcessRatingsEndOfEpoch(
	validatorInfos []*state.ValidatorInfo,
) error {
	err := v.accountsCacher.SaveAll()
	if err != nil {
		return err
	}

	app, err := v.getKApp()
	if err != nil {
		return err
	}

	signedThreshold := v.rater.GetSignedBlocksThreshold()
	for _, validator := range validatorInfos {
		// only process rating of validators that has been in Elected list or
		// active Produced/Missed > 0 and on leaving list
		if validator.List != string(core.ElectedList) && !validatorInfo.WasLeavingElectedInCurrentEpoch(validator) {
			continue
		}

		err := v.verifySignaturesBelowSignedThreshold(app, validator, signedThreshold)
		if err != nil {
			return err
		}
	}

	err = v.saveKApp(app)
	if err != nil {
		return err
	}

	return v.accountsCacher.SaveAll()
}

func (v *validatorsKApp) ResetValidatorStatisticsAtNewEpoch(vInfos []*state.ValidatorInfo) ([]*state.ValidatorInfo, error) {
	err := v.accountsCacher.SaveAll()
	if err != nil {
		return nil, err
	}

	updatedList := make([]*state.ValidatorInfo, 0)

	for _, validator := range vInfos {
		// skip validator pubkey if have been revoked
		if validator.IsPubKeyRevoked {
			continue
		}

		peerAcc, err := v.loadPeerAccount(validator.PublicKey)
		if err != nil {
			return nil, err
		}

		peerAcc.ResetAtNewEpoch()
		v.setToJailedIfNeeded(peerAcc, validator)

		err = v.accountsCacher.UpdatePeer(peerAcc)
		if err != nil {
			return nil, err
		}

		updatedList = append(updatedList, v.PeerAccountToValidatorInfo(validator.GetPublicKey(), validator.IsPubKeyRevoked, peerAcc))
	}

	err = v.accountsCacher.SaveAll()
	if err != nil {
		return nil, err
	}

	return updatedList, nil
}

func (v *validatorsKApp) UpdateMissedBlocksCounters(mb map[string]kapp.RateChange) error {
	err := v.accountsCacher.SaveAll()
	if err != nil {
		return err
	}

	app, err := v.getKApp()
	if err != nil {
		return err
	}

	for addr, slotCounters := range mb {
		err = v.decreasePeerCounter(app, []byte(addr), slotCounters)
		if err != nil {
			return err
		}
	}

	return v.accountsCacher.SaveAll()
}

func (v *validatorsKApp) decreasePeerCounter(app state.KAppAccountHandler, addr []byte, slotCounters kapp.RateChange) error {
	peerAcc, err := v.getPeerAccount(addr, app)
	if err != nil {
		return err
	}

	if slotCounters.Leader > 0 {
		peerAcc.DecreaseLeaderSuccessRate(slotCounters.Leader)
	}

	if slotCounters.Validator > 0 {
		if v.forkController.EnableSmartContracts() {
			peerAcc.DecreaseValidatorSuccessRate(slotCounters.Validator)
		} else {
			peerAcc.DecreaseLeaderSuccessRate(slotCounters.Validator)
		}
	}

	return v.accountsCacher.UpdatePeer(peerAcc)
}

func (v *validatorsKApp) SaveUpdatesForNodesMap(
	nodeOwners [][]byte,
	peerType string,
) (bool, error) {
	err := v.accountsCacher.SaveAll()
	if err != nil {
		return false, err
	}

	nodeForcedToRemain := false

	tmpNodeForcedToRemain, err := v.saveUpdatesForList(nodeOwners, peerType)
	if err != nil {
		return false, err
	}

	nodeForcedToRemain = nodeForcedToRemain || tmpNodeForcedToRemain

	return nodeForcedToRemain, v.accountsCacher.SaveAll()
}

// DecreaseAll applies penalties to all validators for a series of missed slots.
// It calculates and applies rating decreases for both leader and validator roles.
//
// Parameters:
//   - validators: Slice of all elected validators (both leaders and participants)
//   - missedSlots: Number of consecutive slots that were missed
//   - consensusGroupSize: Total number of validators in the consensus group
//
// This function is typically called when a significant number of slots (e.g., >100) have been missed in sequence.
func (v *validatorsKApp) DecreaseAll(
	validators [][]byte,
	missedSlots uint64,
	consensusGroupSize int,
) error {
	// Persist current state before applying changes
	err := v.accountsCacher.SaveAll()
	if err != nil {
		return err
	}

	validatorsCount := len(validators)
	// Calculate the average number of missed participation per validator
	percentageSlotMissedFromTotalValidators := float64(missedSlots) / float64(validatorsCount)
	// Estimate the number of times each validator should have been a leader during missed slots
	// Adding (1 - math.SmallestNonzeroFloat64) ensures rounding up to at least 1
	leaderAppearances := uint32(percentageSlotMissedFromTotalValidators + 1 - math.SmallestNonzeroFloat64)
	// Estimate the total number of times each validator should have participated in consensus as validator
	// Adding (1 - math.SmallestNonzeroFloat64) ensures rounding up to at least 1
	consensusGroupSizeValidators := consensusGroupSize
	if v.forkController.EnableSmartContracts() {
		consensusGroupSizeValidators -= 1
	}
	consensusGroupAppearances := uint32(float64(consensusGroupSizeValidators)*percentageSlotMissedFromTotalValidators +
		1 - math.SmallestNonzeroFloat64)
	ratingDifference := uint32(0)

	// Apply penalties to each validator
	for i, validator := range validators {
		peerAcc, err := v.loadPeerAccount(validator)
		if err != nil {
			return err
		}

		// Decrease success rates for both leader and validator roles
		peerAcc.DecreaseLeaderSuccessRate(leaderAppearances)
		peerAcc.DecreaseValidatorSuccessRate(consensusGroupAppearances)

		currentTempRating := peerAcc.GetTempRating()
		// Apply rating decrease for missed leader opportunities
		for ct := uint32(0); ct < leaderAppearances; ct++ {
			currentTempRating = v.rater.ComputeDecreaseProposer(currentTempRating, 0)
		}

		// Apply rating decrease for missed validator opportunities
		for ct := uint32(0); ct < consensusGroupAppearances; ct++ {
			currentTempRating = v.rater.ComputeDecreaseValidator(currentTempRating)
		}

		// Calculate rating difference (for logging purposes)
		if i == 0 {
			ratingDifference = peerAcc.GetTempRating() - currentTempRating
		}

		// Update the validator's temporary rating
		peerAcc.SetTempRating(currentTempRating)
		// Check if the validator should be jailed due to low rating
		v.jailValidatorIfBadRating(peerAcc)
		// Persist the updated peer account
		err = v.accountsCacher.UpdatePeer(peerAcc)
		if err != nil {
			return err
		}

		// Log or display updated validator information
		v.display(validator, peerAcc)
	}

	// Log summary of applied penalties
	log.Trace(fmt.Sprintf("Decrease leader: %v, decrease validator: %v, ratingDifference: %v", leaderAppearances, consensusGroupAppearances, ratingDifference))

	// Persist all changes
	return v.accountsCacher.SaveAll()
}

func (v *validatorsKApp) DecreaseTempRating(validator []byte, isProposer bool) error {
	err := v.accountsCacher.SaveAll()
	if err != nil {
		return err
	}

	app, err := v.getKApp()
	if err != nil {
		return err
	}

	peerAcc, err := v.getPeerAccount(validator, app)
	if err != nil {
		return err
	}

	var newRating uint32
	if isProposer {
		newRating = v.rater.ComputeDecreaseProposer(
			peerAcc.GetTempRating(),
			peerAcc.GetConsecutiveProposerMisses())
		peerAcc.SetConsecutiveProposerMisses(peerAcc.GetConsecutiveProposerMisses() + 1)
	} else {
		newRating = v.rater.ComputeDecreaseValidator(peerAcc.GetTempRating())
	}

	peerAcc.SetTempRating(newRating)
	v.jailValidatorIfBadRating(peerAcc)

	err = v.accountsCacher.UpdatePeer(peerAcc)
	if err != nil {
		return err
	}

	return v.accountsCacher.SaveAll()
}

func (v *validatorsKApp) ProcessEconomicsEndOfEpoch(currentEpoch uint32, validatorInfos []*state.ValidatorInfo) error {
	err := v.accountsCacher.SaveAll()
	if err != nil {
		return err
	}

	if !v.forkController.EpochRewardsV2() {
		return v.ProcessEconomicsEndOfEpochV1(currentEpoch, validatorInfos)
	}

	return v.ProcessEconomicsEndOfEpochV2(currentEpoch, validatorInfos)
}

// getValidatorParams returns the minimum self-delegated and total-delegated amounts from proposal controller.
func (v *validatorsKApp) getValidatorParams() (minSelfDelegated, minTotalDelegated int64) {
	if check.IfNil(v.KAppController.GetProposalController()) {
		return 0, 0
	}
	minSelfDelegated = v.KAppController.GetProposalController().GetParameterInt(kapps.EnumParameter_MinSelfDelegatedAmount)
	minTotalDelegated = v.KAppController.GetProposalController().GetParameterInt(kapps.EnumParameter_MinTotalDelegatedAmount)
	return minSelfDelegated, minTotalDelegated
}

// updateValidatorJailStatus synchronizes the validator's jail status with the peer account list.
func (v *validatorsKApp) updateValidatorJailStatus(val *ValidatorData, peerAcc state.PeerAccountHandler, currentEpoch uint32) {
	// Clear jail if peer is no longer in jailed list
	if val.Jailed && peerAcc.GetList() != state.List_jailed {
		val.Jailed = false
		val.JailedEpoch = math.MaxUint32
	}

	// Set jail if peer was just jailed
	if !val.Jailed && peerAcc.GetList() == state.List_jailed {
		val.Jailed = true
		val.JailedEpoch = currentEpoch
		val.NumJailed++
	}
}

// updatePeerListStatus updates the peer account's list status based on delegation amounts
// and, when version enforcement is active, on the validator's attested node version.
func (v *validatorsKApp) updatePeerListStatus(val *ValidatorData, peerAcc state.PeerAccountHandler, minSelfDelegated, minTotalDelegated int64, totalDelegated int64, enforcement versionEnforcement) {
	if val.Jailed {
		return
	}

	if val.SelfStake < minSelfDelegated {
		peerAcc.SetList(state.List_inactive)
	} else if totalDelegated < minTotalDelegated {
		peerAcc.SetList(state.List_waiting)
	} else if enforcement.enforce && !versionSatisfies(val.AttestedVersion, enforcement.required) {
		// demoted instead of staying electable with an outdated version; the validator
		// returns to eligible at the first end-of-epoch after a satisfying attestation
		peerAcc.SetList(state.List_observer)
	} else if peerAcc.GetList() != state.List_elected {
		peerAcc.SetList(state.List_eligible)
	}
}

// bucketProcessingResultV1 holds the results of processing validator buckets in V1.
type bucketProcessingResultV1 struct {
	accumulatedDelegations map[string]int64
	totalDelegated         int64
	totalUnDelegated       int64
}

// processBucketDelegationsV1 processes all buckets for a validator and returns delegation metrics.
func (v *validatorsKApp) processBucketDelegationsV1(
	pd *PeerData,
	currentEpoch uint32,
) bucketProcessingResultV1 {
	result := bucketProcessingResultV1{
		accumulatedDelegations: make(map[string]int64),
	}

	fixDelegationSameEpoch := v.forkController.FixDelegationSameEpoch()

	for key, delegation := range pd.Buckets {
		delegatedEpoch := delegation.GetDelegatedEpoch()

		if fixDelegationSameEpoch {
			if delegation.GetUndelegatedEpoch() <= currentEpoch {
				delete(pd.Buckets, key)
				result.totalUnDelegated += delegation.GetValue()
				continue
			}
			delegatedEpoch += 1
		}

		v.processSingleBucketV1(&result, delegation, delegatedEpoch, currentEpoch)

		if delegation.GetUndelegatedEpoch() <= currentEpoch {
			delete(pd.Buckets, key)
			result.totalUnDelegated += delegation.GetValue()
		}
	}

	return result
}

// processSingleBucketV1 processes a single bucket for delegation accumulation.
func (v *validatorsKApp) processSingleBucketV1(
	result *bucketProcessingResultV1,
	delegation *PeerBucket,
	delegatedEpoch, currentEpoch uint32,
) {
	// check if user has valid delegation over 1 epoch at least
	if delegation.GetDelegatedEpoch() == 0 || delegatedEpoch < currentEpoch {
		result.accumulatedDelegations[string(delegation.GetAddress())] += delegation.GetValue()
		result.totalDelegated += delegation.GetValue()
	}
}

// calculateDelegationRewardsV1 calculates commission and distributes rewards among delegators.
func (v *validatorsKApp) calculateDelegationRewardsV1(
	val *ValidatorData,
	accumulatedFees int64,
	accumulatedDelegations map[string]int64,
	totalDelegated int64,
	totalDelegations map[string]int64,
) {
	validatorCommission := float64(val.GetCommission()) / float64(core.HundredPercent)
	commissionAmount := int64(float64(accumulatedFees) * validatorCommission)

	totalDelegations[string(val.GetRewardsAddress())] += commissionAmount

	remainingFees := accumulatedFees - commissionAmount
	accumulatedFeesFloat := float64(remainingFees)

	for address, amount := range accumulatedDelegations {
		userShare := int64(accumulatedFeesFloat * (float64(amount) / float64(totalDelegated)))
		totalDelegations[address] += userShare
		remainingFees -= userShare
	}

	// remaining fees should be added to validator
	if remainingFees != 0 && v.forkController.EnableSmartContracts() {
		totalDelegations[string(val.GetRewardsAddress())] += remainingFees
	}
}

// updateUserAllowances updates user account allowances with their delegation rewards.
func (v *validatorsKApp) updateUserAllowances(totalDelegations map[string]int64) error {
	for address, amount := range totalDelegations {
		if amount == 0 {
			continue
		}

		userAcc, err := v.accountsCacher.LoadUser([]byte(address))
		if err != nil {
			return err
		}

		if err = userAcc.AddToAllowance(amount); err != nil {
			return err
		}

		if err = v.accountsCacher.UpdateUser(userAcc); err != nil {
			return err
		}
	}
	return nil
}

// updateStakingBucketsV1 updates staking buckets if the fork is enabled.
func (v *validatorsKApp) updateStakingBucketsV1(
	app state.KAppAccountHandler,
	addr []byte,
	pd *PeerData,
	totalDelegated, totalUnDelegated int64,
) (int64, error) {
	if !v.forkController.FixStakingBuckets() {
		return totalDelegated, nil
	}

	if !v.forkController.EnableSmartContracts() {
		totalDelegated -= totalUnDelegated
	}

	err := v.setValidatorBuckets(app, addr, pd)
	return totalDelegated, err
}

// processValidatorEpochV1 processes a single validator's epoch economics.
func (v *validatorsKApp) processValidatorEpochV1(
	app state.KAppAccountHandler,
	validatorInfo *state.ValidatorInfo,
	currentEpoch uint32,
	minSelfDelegated, minTotalDelegated int64,
	totalDelegations map[string]int64,
	enforcement versionEnforcement,
) error {
	addr := validatorInfo.GetOwnerAddress()

	pd, err := v.getValidatorBuckets(app, addr)
	if err != nil {
		return err
	}

	bucketResult := v.processBucketDelegationsV1(pd, currentEpoch)

	val, err := v.getValidator(app, addr)
	if err != nil {
		return err
	}

	peerAcc, err := v.loadPeerAccount(val.BlsPubKey)
	if err != nil {
		return err
	}

	v.calculateDelegationRewardsV1(val, peerAcc.GetAccumulatedFees(), bucketResult.accumulatedDelegations, bucketResult.totalDelegated, totalDelegations)

	val.SelfStaked = val.SelfStake >= minSelfDelegated
	v.updateValidatorJailStatus(val, peerAcc, currentEpoch)

	totalDelegated, err := v.updateStakingBucketsV1(app, addr, pd, bucketResult.totalDelegated, bucketResult.totalUnDelegated)
	if err != nil {
		return err
	}

	v.updatePeerListStatus(val, peerAcc, minSelfDelegated, minTotalDelegated, totalDelegated, enforcement)

	return v.finalizeValidatorEpoch(app, addr, val, peerAcc)
}

// bucketProcessingResultV2 holds the results of processing validator buckets in V2.
type bucketProcessingResultV2 struct {
	accumulatedDelegations map[string]int64
	totalDelegated         int64
	totalUnDelegated       int64
	keysToDelete           []string
}

// processBucketDelegationsV2 processes all buckets for a validator and returns delegation metrics (V2 optimized).
func (v *validatorsKApp) processBucketDelegationsV2(
	pd *PeerData,
	currentEpoch uint32,
) bucketProcessingResultV2 {
	result := bucketProcessingResultV2{
		accumulatedDelegations: make(map[string]int64, len(pd.Buckets)),
		keysToDelete:           make([]string, 0, len(pd.Buckets)),
	}

	for key, delegation := range pd.Buckets {
		delegationValue := delegation.GetValue()
		originalDelegatedEpoch := delegation.GetDelegatedEpoch()
		delegatedEpoch := originalDelegatedEpoch

		if v.forkController.FixDelegationSameEpoch() {
			if delegation.GetUndelegatedEpoch() <= currentEpoch {
				result.keysToDelete = append(result.keysToDelete, key)
				result.totalUnDelegated += delegationValue
				continue
			}
			delegatedEpoch += 1
		}

		// check if user has valid delegation over 1 epoch at least
		if originalDelegatedEpoch == 0 || delegatedEpoch < currentEpoch {
			result.accumulatedDelegations[string(delegation.GetAddress())] += delegationValue
			result.totalDelegated += delegationValue
		}
	}

	return result
}

// calculateDelegationRewardsV2 calculates commission and distributes rewards using big.Int for overflow prevention.
func (v *validatorsKApp) calculateDelegationRewardsV2(
	val *ValidatorData,
	accumulatedFees int64,
	accumulatedDelegations map[string]int64,
	totalDelegated int64,
) map[string]int64 {
	rewardsAddrStr := string(val.GetRewardsAddress())
	commissionAmount := new(big.Int).Quo(
		new(big.Int).Mul(big.NewInt(accumulatedFees), big.NewInt(int64(val.GetCommission()))),
		big.NewInt(int64(core.HundredPercent)),
	).Int64()

	localDelegations := make(map[string]int64, len(accumulatedDelegations)+1)
	localDelegations[rewardsAddrStr] = commissionAmount

	remainingFees := accumulatedFees - commissionAmount
	totalRemainingFees := remainingFees

	for address, amount := range accumulatedDelegations {
		userShare := v.calculateUserShareV2(totalRemainingFees, amount, totalDelegated)
		userShare = min(userShare, remainingFees)

		localDelegations[address] += userShare
		remainingFees -= userShare
	}

	if remainingFees != 0 && v.forkController.EnableSmartContracts() {
		localDelegations[rewardsAddrStr] += remainingFees
	}

	return localDelegations
}

// calculateUserShareV2 calculates a user's share using big.Int to prevent overflow.
func (v *validatorsKApp) calculateUserShareV2(totalRemainingFees, amount, totalDelegated int64) int64 {
	bigRemainingFees := big.NewInt(totalRemainingFees)
	bigAmount := big.NewInt(amount)
	bigTotalDelegated := big.NewInt(totalDelegated)

	numerator := new(big.Int).Mul(bigRemainingFees, bigAmount)
	result := new(big.Int).Div(numerator, bigTotalDelegated)
	return result.Int64()
}

// updateStakingBucketsV2 updates staking buckets for V2.
func (v *validatorsKApp) updateStakingBucketsV2(
	app state.KAppAccountHandler,
	addr []byte,
	pd *PeerData,
	keysToDelete []string,
	totalDelegated, totalUnDelegated int64,
) (int64, error) {
	for _, key := range keysToDelete {
		delete(pd.Buckets, key)
	}

	if !v.forkController.FixStakingBuckets() {
		return totalDelegated, nil
	}

	if !v.forkController.EnableSmartContracts() {
		totalDelegated -= totalUnDelegated
	}

	err := v.setValidatorBuckets(app, addr, pd)
	return totalDelegated, err
}

// processValidatorEpochV2 processes a single validator's epoch economics (V2 optimized).
func (v *validatorsKApp) processValidatorEpochV2(
	app state.KAppAccountHandler,
	validatorInfo *state.ValidatorInfo,
	currentEpoch uint32,
	minSelfDelegated, minTotalDelegated int64,
	totalDelegations map[string]int64,
	enforcement versionEnforcement,
) error {
	addr := validatorInfo.GetOwnerAddress()

	pd, err := v.getValidatorBuckets(app, addr)
	if err != nil {
		return err
	}

	bucketResult := v.processBucketDelegationsV2(pd, currentEpoch)

	val, err := v.getValidator(app, addr)
	if err != nil {
		return err
	}

	peerAcc, err := v.loadPeerAccount(val.BlsPubKey)
	if err != nil {
		return err
	}

	localDelegations := v.calculateDelegationRewardsV2(val, peerAcc.GetAccumulatedFees(), bucketResult.accumulatedDelegations, bucketResult.totalDelegated)

	for a, amount := range localDelegations {
		totalDelegations[a] += amount
	}

	val.SelfStaked = val.SelfStake >= minSelfDelegated
	v.updateValidatorJailStatus(val, peerAcc, currentEpoch)

	totalDelegated, err := v.updateStakingBucketsV2(app, addr, pd, bucketResult.keysToDelete, bucketResult.totalDelegated, bucketResult.totalUnDelegated)
	if err != nil {
		return err
	}

	v.updatePeerListStatus(val, peerAcc, minSelfDelegated, minTotalDelegated, totalDelegated, enforcement)

	return v.finalizeValidatorEpoch(app, addr, val, peerAcc)
}

// savePendingRewardsV2 saves all pending rewards to the KApp data trie.
func (v *validatorsKApp) savePendingRewardsV2(app state.KAppAccountHandler, totalDelegations map[string]int64) error {
	for address, amount := range totalDelegations {
		if amount == 0 {
			continue
		}

		if err := v.addToPendingRewards(app, []byte(address), amount); err != nil {
			return err
		}
	}
	return nil
}

// finalizeValidatorEpoch updates validator state and saves both peer and validator accounts.
func (v *validatorsKApp) finalizeValidatorEpoch(app state.KAppAccountHandler, addr []byte, val *ValidatorData, peerAcc state.PeerAccountHandler) error {
	val.Waiting = peerAcc.GetList() == state.List_waiting
	val.TotalRewards += peerAcc.GetAccumulatedFees()

	peerAcc.ResetAtNewEpoch()

	if err := v.accountsCacher.UpdatePeer(peerAcc); err != nil {
		return err
	}

	return v.setValidator(app, addr, val)
}

func (v *validatorsKApp) ProcessEconomicsEndOfEpochV1(currentEpoch uint32, validatorInfos []*state.ValidatorInfo) error {
	totalDelegations := make(map[string]int64)
	minSelfDelegated, minTotalDelegated := v.getValidatorParams()

	app, err := v.getKApp()
	if err != nil {
		return err
	}

	enforcement := v.computeVersionEnforcement(app, validatorInfos, currentEpoch)

	for _, validatorInfo := range validatorInfos {
		if err := v.processValidatorEpochV1(app, validatorInfo, currentEpoch, minSelfDelegated, minTotalDelegated, totalDelegations, enforcement); err != nil {
			return err
		}
	}

	if err = v.saveKApp(app); err != nil {
		return err
	}

	if err = v.updateUserAllowances(totalDelegations); err != nil {
		return err
	}

	return v.accountsCacher.SaveAll()
}

func (v *validatorsKApp) ProcessEconomicsEndOfEpochV2(currentEpoch uint32, validatorInfos []*state.ValidatorInfo) error {
	minSelfDelegated, minTotalDelegated := v.getValidatorParams()

	app, err := v.getKApp()
	if err != nil {
		return err
	}

	totalDelegations := make(map[string]int64)

	enforcement := v.computeVersionEnforcement(app, validatorInfos, currentEpoch)

	for _, validatorInfo := range validatorInfos {
		if err := v.processValidatorEpochV2(app, validatorInfo, currentEpoch, minSelfDelegated, minTotalDelegated, totalDelegations, enforcement); err != nil {
			return err
		}
	}

	if err = v.savePendingRewardsV2(app, totalDelegations); err != nil {
		return err
	}

	if err = v.saveKApp(app); err != nil {
		return err
	}

	log.Debug("ProcessEconomicsEndOfEpoch v2: stored pending rewards in KApp trie",
		"usersCount", len(totalDelegations))

	return v.accountsCacher.SaveAll()
}

func (v *validatorsKApp) UpdateValidatorInfoOnSuccessfulBlock(
	validatorList []sharding.Validator,
	signingBitmap []byte,
	accumulatedFees int64,
) error {
	err := v.accountsCacher.SaveAll()
	if err != nil {
		return err
	}

	// compute success validators
	totalSigned := 0
	for i := 0; i < len(validatorList); i++ {
		haveSigned := (signingBitmap[i/8] & (1 << (uint16(i) % 8))) != 0 // #nosec G115
		if haveSigned {
			totalSigned++
		}
	}

	leaderAccumulatedFees := accumulatedFees
	validatorAccumulatedFees := int64(0)
	if totalSigned > 1 {
		validatorPercent := core.HundredPercent - tools.SafeU64ToU32(
			v.KAppController.GetProposalController().GetParameterUint(kapps.EnumParameter_LeaderValidatorRewardsPercentage),
		)
		ParsedValidatorPercent := float64(validatorPercent) / float64(core.HundredPercent)
		validatorAccumulatedFees = int64(float64(accumulatedFees) * ParsedValidatorPercent / float64(totalSigned-1))
		leaderAccumulatedFees = accumulatedFees - int64(validatorAccumulatedFees*(int64(totalSigned)-1))
	}

	for i := 0; i < len(validatorList); i++ {
		peerAcc, err := v.loadPeerAccount(validatorList[i].PubKey())
		if err != nil {
			return err
		}

		peerAcc.IncreaseNumSelectedInSuccessBlocks()

		newRating := peerAcc.GetRating()
		isLeader := i == 0
		validatorSigned := (signingBitmap[i/8] & (1 << (uint16(i) % 8))) != 0 // #nosec G115
		actionType := v.computeValidatorActionType(isLeader, validatorSigned)

		switch actionType {
		case leaderSuccess:
			peerAcc.IncreaseLeaderSuccessRate(1)
			peerAcc.SetConsecutiveProposerMisses(0)
			newRating = v.rater.ComputeIncreaseProposer(peerAcc.GetTempRating())
			peerAcc.AddToAccumulatedFees(leaderAccumulatedFees)
		case validatorSuccess:
			peerAcc.IncreaseValidatorSuccessRate(1)
			newRating = v.rater.ComputeIncreaseValidator(peerAcc.GetTempRating())
			peerAcc.AddToAccumulatedFees(validatorAccumulatedFees)
		case validatorIgnoredSignature:
			peerAcc.IncreaseValidatorIgnoredSignaturesRate(1)
			newRating = v.rater.ComputeIncreaseValidator(peerAcc.GetTempRating())
		}

		peerAcc.SetTempRating(newRating)

		err = v.accountsCacher.UpdatePeer(peerAcc)
		if err != nil {
			return err
		}
	}

	return v.accountsCacher.SaveAll()
}
