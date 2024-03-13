package mock

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
)

// IndexerMock is a mock implementation fot the Indexer interface
type IndexerMock struct {
	SaveBlockCalled func(args *indexer.ArgsSaveBlockData)
}

// SaveBlock -
func (im *IndexerMock) SaveBlock(args *indexer.ArgsSaveBlockData) {
	if im.SaveBlockCalled != nil {
		im.SaveBlockCalled(args)
	}
}

func (im *IndexerMock) UpdateProposalsAndParameters(proposalIDs []string) {}

// RevertIndexedBlock -
func (im *IndexerMock) RevertIndexedBlock(_ data.HeaderHandler) {
	panic("implement me")
}

// SaveEpochInfo -
func (im *IndexerMock) SaveEpochInfo(epoch uint32, validators []kapp.ValidatorAccountInfoHandler) {
}

// SaveValidatorsRating -
func (im *IndexerMock) SaveValidatorsRating(infoRating []*indexer.ValidatorRatingInfo) {
}

// Close will do nothing
func (im *IndexerMock) Close() error {
	return nil
}

// SaveAccounts -
func (im *IndexerMock) SaveAccounts(_ int64, _ []state.UserAccountHandler) {
}

// SavePeersAccounts -
func (im *IndexerMock) SavePeersAccounts(_ []kapp.ValidatorAccountInfoHandler) {
}

// SaveAccounts -
func (im *IndexerMock) SaveAssets(asset []*kapps.KDAData) {
}

// IsInterfaceNil returns true if there is no value under the interface
func (im *IndexerMock) IsInterfaceNil() bool {
	return im == nil
}

// IsNilIndexer -
func (im *IndexerMock) IsNilIndexer() bool {
	return true
}
