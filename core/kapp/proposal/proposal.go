package proposal

import (
	"encoding/hex"
	"strconv"
	"unicode/utf8"

	logger "github.com/klever-io/klever-go-logger"
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

var log = logger.GetOrCreate("kapp/proposal")

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

func (p *proposalKapp) Create(sender []byte, tc *transaction.ProposalContract) (transaction.Transaction_TXResultCode, error) {
	ctx := p.KAppController.GetCurrentKAppContext()

	if len(tc.GetParameters()) > core.MaxProposalsLength {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if len(tc.GetDescription()) > core.MaxDescriptionLength {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	if tc.GetEpochsDuration() >
		uint32(p.KAppController.GetProposalController().GetParameterUint(kapps.EnumParameter_ProposalMaxEpochsDuration)) ||
		len(tc.GetParameters()) == 0 {
		return transaction.Transaction_ParameterInvalid, common.ErrInvalidValue
	}

	proposalKapp, proposal, controller, err := p.GetProposal(0)
	if err != nil {
		return transaction.Transaction_AccountError, err
	}

	for parameter, value := range tc.GetParameters() {
		if len(kapps.EnumParameter_name[parameter]) == 0 {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidParameter
		}

		if len(value) == 0 {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidParameter
		}

		if !utf8.Valid(value) ||
			len(value) > core.MaxProposalParamLength {
			return transaction.Transaction_ParameterInvalid, common.ErrInvalidParameter
		}

		_, err = controller.Validate(kapps.EnumParameter(parameter), value)
		if err != nil {
			return transaction.Transaction_ParameterInvalid, err
		}
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

	if userKFIStaking.FrozenBalance == 0 {
		return transaction.Transaction_OutOfFunds, common.ErrBalance
	}

	proposal.Proposer = make([]byte, len(sender))
	copy(proposal.Proposer, sender)
	proposal.TXHash = make([]byte, len(ctx.TxHash()))
	copy(proposal.TXHash, ctx.TxHash())
	proposal.ProposalStatus = kapps.ProposalData_ActiveProposal
	proposal.Parameters = tc.GetParameters()
	if len(tc.GetDescription()) > 0 {
		proposal.Description = make([]byte, len(tc.GetDescription()))
		copy(proposal.Description, tc.GetDescription())
	}
	proposal.EpochStart = ctx.Block().GetEpoch()
	proposal.EpochEnd = ctx.Block().GetEpoch() + tc.GetEpochsDuration()
	proposal.Voters = make(map[string]*kapps.ProposalData_VoteDetail)
	proposal.Votes = make(map[int32]int64)
	proposal.TotalStaked = staking.TotalStaked

	if controller.ActiveProposals == nil {
		controller.ActiveProposals = make(map[uint32]*kapps.ActiveProposals)
	}

	controller.ProposalCount++
	if controller.ActiveProposals[proposal.EpochEnd] == nil {
		controller.ActiveProposals[proposal.EpochEnd] = &kapps.ActiveProposals{ProposalIDs: []uint64{controller.ProposalCount}}
	} else {
		controller.ActiveProposals[proposal.EpochEnd].ProposalIDs = append(controller.ActiveProposals[proposal.EpochEnd].ProposalIDs, controller.ProposalCount)
	}

	err = p.SetProposal(proposalKapp, controller.ProposalCount, proposal, controller)
	if err != nil {
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

	oldAmount := int64(0)
	if v, ok := proposal.Voters[encodedAddr]; ok && v != nil {
		if v.Type != kapps.ProposalData_VoteDetail_EnumVoteType(tc.GetType()) {
			proposal.Votes[int32(v.Type)] -= v.Amount
		} else {
			oldAmount = v.Amount
		}
	}

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
