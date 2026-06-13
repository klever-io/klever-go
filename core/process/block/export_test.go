package block

import (
	"github.com/klever-io/klever-go/data"
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

func (m *MetaProcessorForTests) ProcessProposalsEndOfEpoch(headerHandler data.HeaderHandler) error {
	return m.processProposalsEndOfEpoch(headerHandler)
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

// RunScript exposes the runScript dispatcher for testing.
func (m *MetaProcessorForTests) RunScript(name string) (bool, error) {
	return m.runScript(name)
}

// ScriptExecutorNames returns the names of every wired script executor, for drift checks.
func (m *MetaProcessorForTests) ScriptExecutorNames() []string {
	names := make([]string, 0, len(m.scriptExecutors()))
	for name := range m.scriptExecutors() {
		names = append(names, name)
	}
	return names
}

// ExecuteInflationBurn exposes executeInflationBurn for testing.
func (m *MetaProcessorForTests) ExecuteInflationBurn() error {
	return m.executeInflationBurn()
}

// ApplyScript exposes applyScript for testing.
func (m *MetaProcessorForTests) ApplyScript(controller *kapps.ProposalController, value []byte) error {
	proposalKApp, err := m.getProposalKAppForScript()
	if err != nil {
		return err
	}
	return m.applyScript(proposalKApp, controller, value)
}

func (m *MetaProcessorForTests) getProposalKAppForScript() (state.KAppAccountHandler, error) {
	acnt, err := m.accountsDB[state.KAppAccountsState].LoadAccount(kapps.ProposalKAppAddress)
	if err != nil {
		return nil, err
	}
	return acnt.(state.KAppAccountHandler), nil
}

// SetInflationBurnAddresses overrides the burn target list for testing and returns a
// function that restores the original list.
func SetInflationBurnAddresses(addrs []string) func() {
	original := inflationBurnAddresses
	inflationBurnAddresses = addrs
	return func() { inflationBurnAddresses = original }
}
