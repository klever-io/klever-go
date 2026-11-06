package block

import (
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/crypto/hashing"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
	"github.com/klever-io/klever-go/tools/marshal"
)

type MetaProcessorForTests struct {
	*metaProcessor
}

func NewMetaProcessorForTests(mp *metaProcessor) *MetaProcessorForTests {
	controller, _ := kapps.NewProposalController(mp.forkController)
	_ = mp.SetProposalController(controller)

	return &MetaProcessorForTests{
		metaProcessor: mp,
	}
}

func (m *MetaProcessorForTests) ProcessProposalsEndOfEpoch(headerHandler data.HeaderHandler) ([]string, error) {
	return m.processProposalsEndOfEpoch(headerHandler)
}

func (m *MetaProcessorForTests) ApplyEpochStartSideEffects(header *block.Block, validatorsInfo []*state.ValidatorInfo, proposalsToIndex []string) error {
	return m.applyEpochStartSideEffects(header, &epochStartSideEffects{
		validatorsInfo:   validatorsInfo,
		proposalsToIndex: proposalsToIndex,
	})
}

func (m *MetaProcessorForTests) GetKAppAdapter() state.AccountsAdapter {
	return m.accountsDB[state.KAppAccountsState]
}

func (m *MetaProcessorForTests) GetMarshalizer() marshal.Marshalizer {
	return m.marshalizer
}

func (m *MetaProcessorForTests) GetProposalKApp(proposalKApp state.KAppAccountHandler, proposalID uint64) (*kapps.ProposalData, error) {
	return m.getProposalKApp(proposalKApp, proposalID)
}

func (m *MetaProcessorForTests) SetProposalKApp(kdaKapp state.KAppAccountHandler, proposalID uint64, proposal *kapps.ProposalData) error {
	return m.setProposalKApp(kdaKapp, proposalID, proposal)
}

func (m *MetaProcessorForTests) GetActiveParameters() map[int32]*kapps.Parameter {
	return m.proposalController.Get().GetActiveParameters()
}

// ClearProposalController unpublishes the controller, as storage bootstrap is.
func ClearProposalController(proc process.BlockProcessor) {
	proc.(*metaProcessor).proposalController.Clear()
}

// UpdateState exposes the updateState method for testing
func (m *MetaProcessorForTests) UpdateState(lastMetaBlock data.HeaderHandler) {
	m.updateState(lastMetaBlock)
}

// NewMetaProcessorForEpochStartTests builds a metaProcessor holding only what the epoch start
// needs: the given KApp accounts DB, fork controller and epoch notifier, plus a fresh proposal
// controller, returned so that tests can set the governance parameters
func NewMetaProcessorForEpochStartTests(
	kappAccountsDB state.AccountsAdapter,
	hasher hashing.Hasher,
	marshalizer marshal.Marshalizer,
	forkController core.ForkController,
	epochNotifier process.EpochNotifier,
) (*MetaProcessorForTests, kapps.ActiveProposalController, error) {
	controller, err := kapps.NewProposalController(forkController)
	if err != nil {
		return nil, nil, err
	}

	mp := &metaProcessor{
		baseProcessor: &baseProcessor{
			accountsDB:     map[state.AccountsDbIdentifier]state.AccountsAdapter{state.KAppAccountsState: kappAccountsDB},
			hasher:         hasher,
			marshalizer:    marshalizer,
			forkController: forkController,
			epochNotifier:  epochNotifier,
		},
	}
	err = mp.SetProposalController(controller)
	if err != nil {
		return nil, nil, err
	}

	return &MetaProcessorForTests{metaProcessor: mp}, controller, nil
}

// EpochStartNativeStakingKapps exposes the epochStartNativeStakingKapps method for testing
func (m *MetaProcessorForTests) EpochStartNativeStakingKapps(blk data.HeaderHandler) error {
	return m.epochStartNativeStakingKapps(blk)
}

// GetNativeStakingKApps returns the KLV and the KFI staking data for testing
func (m *MetaProcessorForTests) GetNativeStakingKApps() (*kapps.StakingData, *kapps.StakingData, error) {
	_, klv, kfi, err := m.getNativeStakingKApps()

	return klv, kfi, err
}

// GetAccountsDB returns the accountsDB for testing
func (m *MetaProcessorForTests) GetAccountsDB() map[state.AccountsDbIdentifier]state.AccountsAdapter {
	return m.accountsDB
}
