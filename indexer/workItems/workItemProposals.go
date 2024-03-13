package workItems

type itemProposals struct {
	indexer      saveProposalsInfo
	proposalsIDs []string
}

// NewItemProposals will create a new instance of itemProposals
func NewItemProposals(indexer saveProposalsInfo, proposalsIDs []string) WorkItemHandler {
	return &itemProposals{
		indexer:      indexer,
		proposalsIDs: proposalsIDs,
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (wip *itemProposals) IsInterfaceNil() bool {
	return wip == nil
}

// Save will update proposals info in elasticsearch database
func (wip *itemProposals) Save() error {
	err := wip.indexer.UpdateProposalsAndParameters(wip.proposalsIDs)
	if err != nil {
		log.Warn("itemProposals.Save", "could not index proposals", err.Error())
		return err
	}

	return nil
}
