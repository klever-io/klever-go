package proposal

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strconv"
	"unicode/utf8"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/core/process/kda/kdautils"
	txProcess "github.com/klever-io/klever-go/core/process/transaction"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/data/transaction"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/marshal"
)

var _ kapp.ProposalKapp = (*proposalKapp)(nil)

// MaxVotersPerProposal limits the number of unique voters on proposals created before FixAuditChangesV7,
// to prevent exceeding the MaxLeafSize (786KB) storage limit.
// Each voter entry is ~100 bytes serialized, so 7000 voters ≈ 700KB (safe margin).
const MaxVotersPerProposal = 7000

type proposalKapp struct {
	hasher         hashing.Hasher
	marshalizer    marshal.Marshalizer
	pubkeyConv     core.PubkeyConverter
	accountsCacher state.AccountsCacher
	forkController core.ForkController
	addressLen     int
	KAppController kapp.KAppController
}

// ArgsNewProposalKapp holds the arguments needed to create a ProposalKapp
type ArgsNewProposalKApp struct {
	Hasher         hashing.Hasher
	Marshalizer    marshal.Marshalizer
	PubkeyConv     core.PubkeyConverter
	ForkController core.ForkController
}

// NewProposalKapp creates a validator KApp
func NewProposalKApp(
	args *ArgsNewProposalKApp,
) (*proposalKapp, error) {
	if check.IfNil(args.Marshalizer) {
		return nil, common.ErrNilMarshalizer
	}
	if check.IfNil(args.PubkeyConv) {
		return nil, common.ErrNilPubkeyConverter
	}

	v := &proposalKapp{
		hasher:         args.Hasher,
		marshalizer:    args.Marshalizer,
		addressLen:     args.PubkeyConv.Len(),
		pubkeyConv:     args.PubkeyConv,
		forkController: args.ForkController,
	}

	return v, nil
}

// IsInterfaceNil verifies if the underlying object is nil or not
func (p *proposalKapp) IsInterfaceNil() bool {
	return p == nil
}

func (p *proposalKapp) SetKAppController(controller kapp.KAppController) error {
	p.KAppController = controller

	return nil
}

func (p *proposalKapp) SetAccountsCacher(cacher state.AccountsCacher) error {
	if check.IfNil(cacher) {
		return common.ErrNilAccountsAdapter
	}

	p.accountsCacher = cacher

	return nil
}

func (p *proposalKapp) GetAccountsCacher() state.AccountsCacher {
	return p.accountsCacher
}

func (p *proposalKapp) GetExistingUserAccount(pubkey []byte) (state.UserAccountHandler, error) {
	acc, err := p.accountsCacher.GetExistingUser(pubkey)
	if err != nil {
		return nil, err
	}

	return acc, nil
}

func (p *proposalKapp) LoadUserAccount(pubkey []byte) (state.UserAccountHandler, error) {
	acc, err := p.accountsCacher.LoadUser(pubkey)
	if err != nil {
		return nil, err
	}

	return acc, nil
}

func (p *proposalKapp) GetProposal(proposalID uint64) (state.KAppAccountHandler, *kapps.ProposalData, *kapps.ProposalController, error) {
	proposalKApp, err := p.accountsCacher.GetExistingKapp(kapps.ProposalKAppAddress)
	if err != nil {
		return nil, nil, nil, err
	}

	controllerBytes, err := proposalKApp.DataTrieTracker().RetrieveValue(kdautils.ProposalControllerKey)
	if err != nil {
		return nil, nil, nil, err
	}

	proposalController := &kapps.ProposalController{}
	err = p.marshalizer.Unmarshal(proposalController, controllerBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	proposal := &kapps.ProposalData{}
	if proposalID != 0 {
		proposal, err = p.GetProposalData(proposalKApp, proposalID)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	return proposalKApp, proposal, proposalController, nil
}

// GetProposalData reads one proposal from an already loaded proposal kapp account. GetProposal
// re-reads and unmarshals the controller on every call; an unfreeze walking its index holds the
// account and the controller already and must not pay that once per id.
func (p *proposalKapp) GetProposalData(proposalKapp state.KAppAccountHandler, proposalID uint64) (*kapps.ProposalData, error) {
	proposalBytes, err := proposalKapp.DataTrieTracker().RetrieveValue(kdautils.ToProposalKey(proposalID))
	if err != nil {
		return nil, err
	}
	if len(proposalBytes) == 0 {
		return nil, common.ErrProposalNotFound
	}

	proposal := &kapps.ProposalData{}
	if err := p.marshalizer.Unmarshal(proposal, proposalBytes); err != nil {
		return nil, err
	}

	return proposal, nil
}

func (p *proposalKapp) SetProposal(proposalKapp state.KAppAccountHandler, proposalID uint64, proposal *kapps.ProposalData, controller *kapps.ProposalController) error {
	if controller != nil {
		data, err := p.marshalizer.Marshal(controller)
		if err != nil {
			return err
		}

		err = proposalKapp.DataTrieTracker().SaveKeyValue(kdautils.ProposalControllerKey, data)
		if err != nil {
			return err
		}
	}

	data, err := p.marshalizer.Marshal(proposal)
	if err != nil {
		return err
	}

	key := kdautils.ToProposalKey(proposalID)
	err = proposalKapp.DataTrieTracker().SaveKeyValue(key, data)
	if err != nil {
		return err
	}

	return nil
}

func (p *proposalKapp) validateCreateInput(tc *transaction.ProposalContract) error {
	if len(tc.GetParameters()) > core.MaxProposalsLength {
		return common.ErrInvalidValue
	}

	if len(tc.GetDescription()) > core.MaxDescriptionLength {
		return common.ErrInvalidValue
	}

	if uint64(tc.GetEpochsDuration()) >
		p.KAppController.GetProposalController().GetParameterUint(kapps.EnumParameter_ProposalMaxEpochsDuration) ||
		len(tc.GetParameters()) == 0 {
		return common.ErrInvalidValue
	}

	if p.forkController.FixAuditChangesV5() && tc.GetEpochsDuration() == 0 {
		// A zero-duration proposal ends in the epoch it is created in, whose bucket settlement
		// processed at that epoch's start, so it would never be settled: active and votable
		// forever, holding its bucket, and holding an entry in the index of every account that
		// ever voted on it, since an entry is only prunable once its proposal leaves the buckets.
		return common.ErrInvalidValue
	}

	return nil
}

func (p *proposalKapp) validateNewParameters(params map[int32][]byte, controller *kapps.ProposalController) error {
	for parameter, value := range params {
		if len(kapps.EnumParameter_name[parameter]) == 0 ||
			(parameter == int32(kapps.EnumParameter_MinKFIVoteAmount) && !p.forkController.FixAuditChangesV7()) {
			return common.ErrInvalidParameter
		}

		if len(value) == 0 {
			return common.ErrInvalidValue
		}

		if !utf8.Valid(value) ||
			len(value) > core.MaxProposalParamLength {
			return common.ErrInvalidValue
		}

		if _, err := controller.ParseParamAndValidate(kapps.EnumParameter(parameter), value, p.forkController); err != nil {
			return err
		}
	}

	return nil
}

func (p *proposalKapp) checkStakingRequirements(staking *kapps.StakingData, sender []byte) (transaction.Transaction_TXResultCode, error) {
	ctx := p.KAppController.GetCurrentKAppContext()

	if staking.TotalStaked < p.KAppController.GetProposalController().GetParameterInt(kapps.EnumParameter_MinKFIStakedToEnableProposals) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMinKFIStakedUnreached, process.ErrMinKFIStaked.Error())
		return transaction.Transaction_MinKFIStakedUnreached, process.ErrMinKFIStaked
	}

	ownerAcc, err := p.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	userKFIStaking, err := ownerAcc.GetUserKDA(kdautils.KFIIdentifier, nil, p.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	if userKFIStaking.FrozenBalance == 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInsufficientFrozenKFI, common.ErrBalance.Error())
		return transaction.Transaction_OutOfFunds, common.ErrBalance
	}

	return transaction.Transaction_Ok, nil
}

func (p *proposalKapp) Create(sender []byte, tc *transaction.ProposalContract) (transaction.Transaction_TXResultCode, error) {
	ctx := p.KAppController.GetCurrentKAppContext()

	// Validate input parameters
	if err := p.validateCreateInput(tc); err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidProposal, err.Error())
		return transaction.Transaction_ParameterInvalid, err
	}

	// Retrieve proposal kapp
	proposalKapp, proposal, controller, err := p.GetProposal(0)
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	// Validate new parameters
	if err := p.validateNewParameters(tc.GetParameters(), controller); err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidProposalParams, err.Error())
		return transaction.Transaction_ParameterInvalid, err
	}

	// Retrieve staking
	_, staking, err := p.KAppController.GetKDAKApp().GetStaking(kdautils.KFIIdentifier)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	// Check staking requirements
	if errCode, err := p.checkStakingRequirements(staking, sender); err != nil {
		return errCode, err
	}

	// Finalize and create proposal
	return p.finalizeProposal(ctx, proposalKapp, proposal, controller, staking, tc, sender)

}

func (p *proposalKapp) finalizeProposal(
	ctx kapp.KappContext,
	proposalKapp state.KAppAccountHandler,
	proposal *kapps.ProposalData,
	controller *kapps.ProposalController,
	staking *kapps.StakingData,
	tc *transaction.ProposalContract,
	sender []byte,
) (transaction.Transaction_TXResultCode, error) {
	// Prepare proposal details
	copyProposalDetails(ctx, proposal, staking.TotalStaked, tc, sender)

	// Record which ids predate the fork before taking the next one, so that this proposal, and
	// every proposal created after it, falls above the bound a KFI unfreeze scans. Its own votes
	// are all indexed, so nothing is lost by leaving it out of that scan.
	if p.forkController.FixAuditChangesV5() {
		if _, err := p.PreForkVoteIDBound(proposalKapp, controller); err != nil {
			return transaction.Transaction_KAPPError, err
		}
	}

	// Update controller and proposal
	controller.ProposalCount++
	updateActiveProposals(controller, proposal)

	if err := p.SetProposal(proposalKapp, controller.ProposalCount, proposal, controller); err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}
	if err := p.accountsCacher.UpdateKapp(proposalKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	proposalID := []byte(strconv.FormatUint(controller.ProposalCount, 10))
	ctx.SetReturnData([][]byte{proposalID})
	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.Proposal,
		ctx.ContractID(),
		proposalID,
	))

	return transaction.Transaction_Ok, nil
}

func copyProposalDetails(
	ctx kapp.KappContext,
	proposal *kapps.ProposalData,
	totalStaked int64,
	tc *transaction.ProposalContract,
	sender []byte,
) {
	proposal.Proposer = append([]byte(nil), sender...)
	proposal.TXHash = append([]byte(nil), ctx.TxHash()...)
	proposal.ProposalStatus = kapps.ProposalData_ActiveProposal
	proposal.Parameters = tc.GetParameters()
	proposal.Description = append([]byte(nil), tc.GetDescription()...)
	proposal.EpochStart = ctx.Block().GetEpoch()
	proposal.EpochEnd = ctx.Block().GetEpoch() + tc.GetEpochsDuration()
	proposal.Voters = make(map[string]*kapps.ProposalData_VoteDetail)
	proposal.Votes = make(map[int32]int64)
	proposal.TotalStaked = totalStaked
}

func updateActiveProposals(controller *kapps.ProposalController, proposal *kapps.ProposalData) {
	if controller.ActiveProposals == nil {
		controller.ActiveProposals = make(map[uint32]*kapps.ActiveProposals)
	}

	if controller.ActiveProposals[proposal.EpochEnd] == nil {
		controller.ActiveProposals[proposal.EpochEnd] = &kapps.ActiveProposals{ProposalIDs: []uint64{controller.ProposalCount}}
	} else {
		controller.ActiveProposals[proposal.EpochEnd].ProposalIDs = append(controller.ActiveProposals[proposal.EpochEnd].ProposalIDs, controller.ProposalCount)
	}
}

func (p *proposalKapp) storesVotersUnderOwnKeys(proposal *kapps.ProposalData) bool {
	return p.forkController.FixAuditChangesV7InEpoch(proposal.GetEpochStart())
}

// GetVoter returns a voter's record on the proposal, or nil when there is none.
func (p *proposalKapp) GetVoter(
	proposalKapp state.KAppAccountHandler,
	proposal *kapps.ProposalData,
	proposalID uint64,
	encodedAddr string,
) (*kapps.ProposalData_VoteDetail, error) {
	if !p.storesVotersUnderOwnKeys(proposal) {
		return proposal.Voters[encodedAddr], nil
	}

	raw, err := retrieveOptional(proposalKapp.DataTrieTracker(), kdautils.ToProposalVoterKey(proposalID, encodedAddr))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}

	detail := &kapps.ProposalData_VoteDetail{}
	if err := p.marshalizer.Unmarshal(detail, raw); err != nil {
		return nil, err
	}

	return detail, nil
}

// SetVoter stores a voter's record. For inline voters it only updates proposal.Voters, so the caller must still call SetProposal.
func (p *proposalKapp) SetVoter(
	proposalKapp state.KAppAccountHandler,
	proposal *kapps.ProposalData,
	proposalID uint64,
	encodedAddr string,
	detail *kapps.ProposalData_VoteDetail,
) error {
	if !p.storesVotersUnderOwnKeys(proposal) {
		if proposal.Voters == nil {
			proposal.Voters = make(map[string]*kapps.ProposalData_VoteDetail)
		}
		proposal.Voters[encodedAddr] = detail

		return nil
	}

	raw, err := p.marshalizer.Marshal(detail)
	if err != nil {
		return err
	}

	return proposalKapp.DataTrieTracker().SaveKeyValue(kdautils.ToProposalVoterKey(proposalID, encodedAddr), raw)
}

// RemoveVoter deletes a voter's record. For inline voters the caller must still call SetProposal.
func (p *proposalKapp) RemoveVoter(
	proposalKapp state.KAppAccountHandler,
	proposal *kapps.ProposalData,
	proposalID uint64,
	encodedAddr string,
) error {
	if !p.storesVotersUnderOwnKeys(proposal) {
		delete(proposal.Voters, encodedAddr)

		return nil
	}

	return proposalKapp.DataTrieTracker().SaveKeyValue(kdautils.ToProposalVoterKey(proposalID, encodedAddr), nil)
}

// isVoterLimitExceeded checks if adding a new voter would exceed the maximum limit.
// Returns true only if: EpochRewardsV2 is enabled, the proposal predates FixAuditChangesV7 (inline voters), the voter doesn't exist, and the limit is reached.
func (p *proposalKapp) isVoterLimitExceeded(proposal *kapps.ProposalData, encodedAddr string) bool {
	if !p.forkController.EpochRewardsV2() || p.storesVotersUnderOwnKeys(proposal) {
		return false
	}
	_, exists := proposal.Voters[encodedAddr]
	return !exists && len(proposal.Voters) >= MaxVotersPerProposal
}

// processExistingVote handles vote type changes and returns the old amount for same-type votes.
// If the voter changed their vote type, it subtracts from the old type's count and returns 0.
// If the voter is updating the same type, it returns the old amount for incremental update.
func (p *proposalKapp) processExistingVote(proposal *kapps.ProposalData, v *kapps.ProposalData_VoteDetail, newType kapps.ProposalData_VoteDetail_EnumVoteType) int64 {
	if v == nil {
		return 0
	}
	if v.Type != newType {
		proposal.Votes[int32(v.Type)] -= v.Amount
		return 0
	}
	return v.Amount
}

func (p *proposalKapp) Vote(sender []byte, tc *transaction.VoteContract) (transaction.Transaction_TXResultCode, error) {
	ctx := p.KAppController.GetCurrentKAppContext()

	if len(kapps.ProposalData_VoteDetail_EnumVoteType_name[int32(tc.GetType())]) == 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidVoteType, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetProposalID() <= 0 || tc.GetAmount() <= 0 {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidVoteAmount, common.ErrInvalidValue.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if p.forkController.FixAuditChangesV7() &&
		tc.GetAmount() < p.KAppController.GetProposalController().GetParameterInt(kapps.EnumParameter_MinKFIVoteAmount) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInvalidVoteAmount, common.ErrProposalVoteBelowMinimum.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrProposalVoteBelowMinimum
	}

	proposalKapp, proposal, controller, err := p.GetProposal(tc.GetProposalID())
	if err != nil {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldProposalNotFound, err.Error())
		return transaction.Transaction_AccountError, err
	}

	if proposal.ProposalStatus != kapps.ProposalData_ActiveProposal {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldProposalNotActive, common.ErrInvalidParameter.Error())
		return transaction.Transaction_ProposalNotActive, common.ErrInvalidParameter
	}

	_, staking, err := p.KAppController.GetKDAKApp().GetStaking(kdautils.KFIIdentifier)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if staking.TotalStaked < p.KAppController.GetProposalController().GetParameterInt(kapps.EnumParameter_MinKFIStakedToEnableProposals) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldMinKFIStakedUnreached, process.ErrMinKFIStaked.Error())
		return transaction.Transaction_MinKFIStakedUnreached, process.ErrMinKFIStaked
	}

	ownerAcc, err := p.GetExistingUserAccount(sender)
	if err != nil {
		return transaction.Transaction_LoadAccountError, err
	}

	userKFIStaking, err := ownerAcc.GetUserKDA(kdautils.KFIIdentifier, nil, p.forkController.EnableSmartContracts())
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	if tc.GetAmount() > userKFIStaking.FrozenBalance {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldInsufficientFrozenKFI, common.ErrBalance.Error())
		return transaction.Transaction_OutOfFunds, common.ErrBalance
	}

	if proposal.Voters == nil {
		proposal.Voters = make(map[string]*kapps.ProposalData_VoteDetail)
	}

	if proposal.Votes == nil {
		proposal.Votes = make(map[int32]int64)
	}

	// must use string to marshal proto map due UTF8 issue
	encodedAddr := hex.EncodeToString(sender)

	// Check if adding a new voter would exceed the maximum limit
	// (skip check if voter already exists - it's a vote update)
	// Only enforced after EpochRewardsV2 fork to maintain consensus during upgrade
	if p.isVoterLimitExceeded(proposal, encodedAddr) {
		ctx.Receipts().AddError(ctx.ContractID(), common.ErrFieldProposalMaxVoters, common.ErrProposalMaxVotersReached.Error())
		return transaction.Transaction_ParameterInvalid, common.ErrProposalMaxVotersReached
	}

	existingVoter, err := p.GetVoter(proposalKapp, proposal, tc.GetProposalID(), encodedAddr)
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	oldAmount := p.processExistingVote(proposal, existingVoter, kapps.ProposalData_VoteDetail_EnumVoteType(tc.GetType()))
	proposal.Votes[int32(tc.GetType())] += tc.GetAmount() - oldAmount

	err = p.SetVoter(proposalKapp, proposal, tc.GetProposalID(), encodedAddr, &kapps.ProposalData_VoteDetail{
		Type:      kapps.ProposalData_VoteDetail_EnumVoteType(tc.GetType()),
		Amount:    tc.GetAmount(),
		Timestamp: ctx.Block().GetTimestamp(),
	})
	if err != nil {
		return transaction.Transaction_KAPPError, err
	}

	proposal.TotalStaked = staking.TotalStaked

	err = p.SetProposal(proposalKapp, tc.GetProposalID(), proposal, controller)
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	if p.forkController.FixAuditChangesV5() {
		if err := p.addVoterToIndex(proposalKapp, encodedAddr, tc.GetProposalID(), tc.GetAmount(), controller); err != nil {
			return transaction.Transaction_KAPPError, err
		}
	}

	if err := p.accountsCacher.UpdateKapp(proposalKapp); err != nil {
		return transaction.Transaction_SaveAccountError, err
	}

	ctx.Receipts().Add(txProcess.NewReceipt(
		txProcess.ProposalVote,
		ctx.ContractID(),
		[]byte(strconv.FormatUint(tc.GetProposalID(), 10)), // proposal ID
		ownerAcc.AddressBytes(),                            // Voter
		[]byte(strconv.FormatInt(int64(tc.GetType()), 10)), // Vote type
		[]byte(strconv.FormatInt(tc.GetAmount(), 10)),      // Vote weight
	))

	return transaction.Transaction_Ok, nil
}

// PreForkVoteIDBound returns the highest proposal id that can still hold a vote cast before
// FixAuditChangesV5, which is the last id handed out before the fork: ids are sequential, and
// every vote cast after the fork is written to the caller's index or the vote fails. A KFI
// unfreeze therefore has to scan ids at or below this bound and nothing above it.
//
// The bound is recorded the first time it is needed, from the controller's own count, and frozen
// in state. Every path that could move the count past the pre-fork ids records it first — Create
// before it takes the next id, the unfreeze before it scans — so whichever runs first stores the
// same number, and a proposal created after the fork is always above it.
func (p *proposalKapp) PreForkVoteIDBound(
	proposalKapp state.KAppAccountHandler,
	controller *kapps.ProposalController,
) (uint64, error) {
	raw, err := retrieveOptional(proposalKapp.DataTrieTracker(), kdautils.ProposalPreForkVotesKey)
	if err != nil {
		return 0, err
	}

	switch len(raw) {
	case proposalIDSize:
		return binary.BigEndian.Uint64(raw), nil
	case 0:
		// Not recorded yet.
	default:
		// Fail closed. A recorded bound of the wrong shape is corruption, and recomputing it
		// from the count as it stands now is exactly the "later proposals widen the scan" path
		// this key exists to close.
		return 0, common.ErrInvalidValue
	}

	idBound := controller.GetProposalCount()

	stored := binary.BigEndian.AppendUint64(make([]byte, 0, proposalIDSize), idBound)
	if err := proposalKapp.DataTrieTracker().SaveKeyValue(kdautils.ProposalPreForkVotesKey, stored); err != nil {
		return 0, err
	}

	return idBound, nil
}

// retrieveOptional reads a key that may legitimately be absent. A missing data trie means
// nothing was ever written; ErrNegativeValue is how an empty value written earlier in the same
// block reads back from the dirty cache, and an empty value is the delete on this trie. Both are
// "absent", not failures; anything else is.
func retrieveOptional(tracker state.DataTrieTracker, key []byte) ([]byte, error) {
	raw, err := tracker.RetrieveValue(key)
	if err != nil {
		if errors.Is(err, common.ErrNilTrie) || errors.Is(err, common.ErrNegativeValue) {
			return nil, nil
		}

		return nil, err
	}

	return raw, nil
}

// GetAccountProposalVotes returns the account's vote index: each active proposal it has voted on
// since FixAuditChangesV5 with the amount currently voted.
func (p *proposalKapp) GetAccountProposalVotes(proposalKapp state.KAppAccountHandler, encodedAddr string) ([]kapp.ProposalVoteIndexEntry, error) {
	raw, err := retrieveOptional(proposalKapp.DataTrieTracker(), kdautils.ToAccountProposalVotesKey(encodedAddr))
	if err != nil {
		return nil, err
	}

	return decodeProposalVoteIndex(raw)
}

// addVoterToIndex records the vote and, since the index is being rewritten anyway, drops every
// entry whose proposal is in none of the controller's buckets — settled, so no unfreeze needs it.
// That keeps the index bounded by the proposals still active, however long the account votes
// without unfreezing.
func (p *proposalKapp) addVoterToIndex(
	proposalKapp state.KAppAccountHandler,
	encodedAddr string,
	proposalID uint64,
	amount int64,
	controller *kapps.ProposalController,
) error {
	entries, err := p.GetAccountProposalVotes(proposalKapp, encodedAddr)
	if err != nil {
		return err
	}

	entries, pruned := dropInactiveProposalVotes(entries, controller)
	entries, changed := upsertProposalVoteIndex(entries, proposalID, amount)
	if !pruned && !changed {
		return nil
	}

	return p.SetAccountProposalVotes(proposalKapp, encodedAddr, entries)
}

// SetAccountProposalVotes stores the index in one write; an empty index is stored as an empty
// value, which is the delete on this trie.
func (p *proposalKapp) SetAccountProposalVotes(
	proposalKapp state.KAppAccountHandler,
	encodedAddr string,
	entries []kapp.ProposalVoteIndexEntry,
) error {
	raw, err := encodeProposalVoteIndex(entries)
	if err != nil {
		return err
	}

	return proposalKapp.DataTrieTracker().SaveKeyValue(kdautils.ToAccountProposalVotesKey(encodedAddr), raw)
}
