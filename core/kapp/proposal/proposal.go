package proposal

import (
	"encoding/hex"
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

// MaxVotersPerProposal limits the number of unique voters per proposal
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
		key := kdautils.ToProposalKey(proposalID)

		proposalBytes, err := proposalKApp.DataTrieTracker().RetrieveValue(key)
		if err != nil {
			return nil, nil, nil, err
		}
		if len(proposalBytes) == 0 {
			return nil, nil, nil, common.ErrProposalNotFound
		}
		err = p.marshalizer.Unmarshal(proposal, proposalBytes)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	return proposalKApp, proposal, proposalController, nil
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

	return nil
}

func (p *proposalKapp) validateNewParameters(params map[int32][]byte, controller *kapps.ProposalController) error {
	for parameter, value := range params {
		if len(kapps.EnumParameter_name[parameter]) == 0 {
			return common.ErrInvalidParameter
		}

		if len(value) == 0 {
			return common.ErrInvalidValue
		}

		if !utf8.Valid(value) ||
			len(value) > core.MaxProposalParamLength {
			return common.ErrInvalidValue
		}

		if _, err := controller.ParseParamAndValidate(kapps.EnumParameter(parameter), value); err != nil {
			return err
		}
	}

	return nil
}

func (p *proposalKapp) checkStakingRequirements(staking *kapps.StakingData, sender []byte) (transaction.Transaction_TXResultCode, error) {
	if staking.TotalStaked < p.KAppController.GetProposalController().GetParameterInt(kapps.EnumParameter_MinKFIStakedToEnableProposals) {
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
		return transaction.Transaction_OutOfFunds, common.ErrBalance
	}

	return transaction.Transaction_Ok, nil
}

func (p *proposalKapp) Create(sender []byte, tc *transaction.ProposalContract) (transaction.Transaction_TXResultCode, error) {
	ctx := p.KAppController.GetCurrentKAppContext()

	// Validate input parameters
	if err := p.validateCreateInput(tc); err != nil {
		return transaction.Transaction_ParameterInvalid, err
	}

	// Retrieve proposal kapp
	proposalKapp, proposal, controller, err := p.GetProposal(0)
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	// Validate new parameters
	if err := p.validateNewParameters(tc.GetParameters(), controller); err != nil {
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

// isVoterLimitExceeded checks if adding a new voter would exceed the maximum limit.
// Returns true only if: EpochRewardsV2 is enabled, voter doesn't exist, and limit is reached.
func (p *proposalKapp) isVoterLimitExceeded(proposal *kapps.ProposalData, encodedAddr string) bool {
	if !p.forkController.EpochRewardsV2() {
		return false
	}
	_, exists := proposal.Voters[encodedAddr]
	return !exists && len(proposal.Voters) >= MaxVotersPerProposal
}

// processExistingVote handles vote type changes and returns the old amount for same-type votes.
// If the voter changed their vote type, it subtracts from the old type's count and returns 0.
// If the voter is updating the same type, it returns the old amount for incremental update.
func (p *proposalKapp) processExistingVote(proposal *kapps.ProposalData, encodedAddr string, newType kapps.ProposalData_VoteDetail_EnumVoteType) int64 {
	v, ok := proposal.Voters[encodedAddr]
	if !ok || v == nil {
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
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetProposalID() <= 0 || tc.GetAmount() <= 0 {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	proposalKapp, proposal, controller, err := p.GetProposal(tc.GetProposalID())
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	if proposal.ProposalStatus != kapps.ProposalData_ActiveProposal {
		return transaction.Transaction_ProposalNotActive, common.ErrInvalidParameter
	}

	_, staking, err := p.KAppController.GetKDAKApp().GetStaking(kdautils.KFIIdentifier)
	if err != nil {
		return transaction.Transaction_AssetError, err
	}

	if staking.TotalStaked < p.KAppController.GetProposalController().GetParameterInt(kapps.EnumParameter_MinKFIStakedToEnableProposals) {
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
		return transaction.Transaction_ParameterInvalid, common.ErrProposalMaxVotersReached
	}

	oldAmount := p.processExistingVote(proposal, encodedAddr, kapps.ProposalData_VoteDetail_EnumVoteType(tc.GetType()))
	proposal.Votes[int32(tc.GetType())] += tc.GetAmount() - oldAmount

	proposal.Voters[encodedAddr] = &kapps.ProposalData_VoteDetail{
		Type:      kapps.ProposalData_VoteDetail_EnumVoteType(tc.GetType()),
		Amount:    tc.GetAmount(),
		Timestamp: ctx.Block().GetTimestamp(),
	}

	proposal.TotalStaked = staking.TotalStaked

	err = p.SetProposal(proposalKapp, tc.GetProposalID(), proposal, controller)
	if err != nil {
		return transaction.Transaction_ParameterInvalid, err
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
