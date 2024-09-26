package validators

import (
	"bytes"
	"fmt"
	"math"

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

	totalDelegations := make(map[string]int64)

	// Get Validators Param from KAPP
	minSelfDelegated := int64(0)
	minTotalDelegated := int64(0)
	if !check.IfNil(v.KAppController.GetProposalController()) {
		minSelfDelegated = v.KAppController.GetProposalController().GetParameterInt(kapps.EnumParameter_MinSelfDelegatedAmount)
		minTotalDelegated = v.KAppController.GetProposalController().GetParameterInt(kapps.EnumParameter_MinTotalDelegatedAmount)
	}

	// get App
	app, err := v.getKApp()
	if err != nil {
		return err
	}

	for _, validatorInfo := range validatorInfos {
		addr := validatorInfo.GetOwnerAddress()
		pd, err := v.getValidatorBuckets(app, addr)
		if err != nil {
			return err
		}

		accumulatedDelegations := make(map[string]int64)
		totalDelegated := int64(0)
		totalUnDelegated := int64(0)
		selfDelegated := int64(0)
		for key, delegation := range pd.Buckets {

			delegatedEpoch := delegation.DelegatedEpoch

			if v.forkController.FixDelegationSameEpoch() {
				if delegation.UndelegatedEpoch <= currentEpoch {
					// Delete staking from validator
					delete(pd.Buckets, key)
					totalUnDelegated += delegation.GetValue()
					continue
				}

				delegatedEpoch += 1
			}

			// check self staking
			if (delegation.DelegatedEpoch == 0 || delegation.DelegatedEpoch < currentEpoch) &&
				bytes.Equal(addr, delegation.GetAddress()) {
				selfDelegated += delegation.GetValue()
			}

			// check if user has valid delegation over 1 epoch at least
			if delegation.DelegatedEpoch == 0 || delegatedEpoch < currentEpoch {
				accumulatedDelegations[string(delegation.GetAddress())] += delegation.GetValue()
				totalDelegated += delegation.GetValue()
			}

			if delegation.UndelegatedEpoch <= currentEpoch {
				// Delete staking from validator
				delete(pd.Buckets, key)
				totalUnDelegated += delegation.GetValue()
			}
		}

		val, err := v.getValidator(app, addr)
		if err != nil {
			return err
		}

		peerAcc, err := v.loadPeerAccount(val.BlsPubKey)
		if err != nil {
			return err
		}

		validatorCommission := float64(val.GetCommission()) / float64(core.HundredPercent)
		commissionAmount := int64(float64(peerAcc.GetAccumulatedFees()) * validatorCommission)

		totalDelegations[string(val.GetRewardsAddress())] += commissionAmount

		remainingFees := peerAcc.GetAccumulatedFees() - commissionAmount
		accumulatedFees := float64(remainingFees)
		// move accumulated to top of validators lop
		for address, amount := range accumulatedDelegations {
			userShare := int64(accumulatedFees * (float64(amount) / float64(totalDelegated)))
			totalDelegations[address] += userShare
			remainingFees -= userShare
		}

		// remaining fees should be added to validator
		if remainingFees != 0 &&
			v.forkController.EnableSmartContracts() {
			totalDelegations[string(val.GetRewardsAddress())] += remainingFees
		}

		val.SelfStaked = val.SelfStake >= minSelfDelegated

		if val.Jailed && peerAcc.GetList() != state.List_jailed {
			val.Jailed = false
			val.JailedEpoch = math.MaxUint32
		}

		// Update Jail/List/Slash status
		if !val.Jailed && peerAcc.GetList() == state.List_jailed {
			val.Jailed = true
			val.JailedEpoch = currentEpoch
			val.NumJailed++
		}

		if v.forkController.FixStakingBuckets() {
			if !v.forkController.EnableSmartContracts() {
				totalDelegated -= totalUnDelegated
			}

			err = v.setValidatorBuckets(app, addr, pd)
			if err != nil {
				return err
			}
		}

		if !val.Jailed {
			if val.SelfStake < minSelfDelegated {
				peerAcc.SetList(state.List_inactive)
			} else if totalDelegated < minTotalDelegated {
				peerAcc.SetList(state.List_waiting)
			} else if peerAcc.GetList() != state.List_elected {
				peerAcc.SetList(state.List_eligible)
			}
		}

		val.Waiting = peerAcc.GetList() == state.List_waiting

		val.TotalRewards += peerAcc.GetAccumulatedFees()

		peerAcc.ResetAtNewEpoch()

		err = v.accountsCacher.UpdatePeer(peerAcc)
		if err != nil {
			return err
		}

		err = v.setValidator(app, addr, val)
		if err != nil {
			return err
		}
	}

	err = v.saveKApp(app)
	if err != nil {
		return err
	}

	// update to account allowance (REAWARDS to claim)
	for address, amount := range totalDelegations {
		if amount == 0 {
			continue
		}

		userAcc, err := v.accountsCacher.LoadUser([]byte(address))
		if err != nil {
			return err
		}

		err = userAcc.AddToAllowance(amount)
		if err != nil {
			return err
		}

		err = v.accountsCacher.UpdateUser(userAcc)
		if err != nil {
			return err
		}
	}

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
