package indexer

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/state"
	"github.com/klever-io/klever-go/kapps"
)

// NilIndexer will be used when an Indexer is required, but another one isn't necessary or available
type NilIndexer struct {
}

// NewNilIndexer will return a Nil indexer
func NewNilIndexer() *NilIndexer {
	return new(NilIndexer)
}

// SaveBlock will do nothing
func (ni *NilIndexer) SaveBlock(_ *indexer.ArgsSaveBlockData) {
}

// RevertIndexedBlock will do nothing
func (ni *NilIndexer) RevertIndexedBlock(_ data.HeaderHandler) {
}

// SaveAssets will do nothing
func (ni *NilIndexer) SaveAssets(_ []*kapps.KDAData) {
}

// SaveEpochInfo will do nothing
func (ni *NilIndexer) SaveEpochInfo(_ uint32, _ []kapp.ValidatorAccountInfoHandler) {
}

// SaveAccounts won't do anything as this is a nil implementation
func (ni *NilIndexer) SaveAccounts(_ int64, _ []state.UserAccountHandler) {
}

// SavePeersAccounts won't do anything as this is a nil implementation
func (ni *NilIndexer) SavePeersAccounts(_ []kapp.ValidatorAccountInfoHandler) {
}

// UpdateProposalsAndParameters will do nothing
func (ni *NilIndexer) UpdateProposalsAndParameters(_ []string) {
}

// Close will do nothing
func (ni *NilIndexer) Close() error {
	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (ni *NilIndexer) IsInterfaceNil() bool {
	return ni == nil
}

// IsNilIndexer will return a bool value that signals if the indexer's implementation is a NilIndexer
func (ni *NilIndexer) IsNilIndexer() bool {
	return true
}
