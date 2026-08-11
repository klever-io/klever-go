package block

import (
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
	return m.proposalController.GetActiveParameters()
}

// UpdateState exposes the updateState method for testing
func (m *MetaProcessorForTests) UpdateState(lastMetaBlock data.HeaderHandler) {
	m.updateState(lastMetaBlock)
}

// GetAccountsDB returns the accountsDB for testing
func (m *MetaProcessorForTests) GetAccountsDB() map[state.AccountsDbIdentifier]state.AccountsAdapter {
	return m.accountsDB
}
