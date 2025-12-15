package workItems

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/statistics"
	nodeData "github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/indexer/data"
)

// WorkItemHandler defines the interface for item that needs to be saved in elasticsearch database
type WorkItemHandler interface {
	Save() error
	IsInterfaceNil() bool
}

type saveBlockIndexer interface {
	SaveHeader(header nodeData.HeaderHandler, signer []byte, txsSize int, validators []string) error
	SaveTransactions(header nodeData.HeaderHandler, pool *indexer.Pool) error
}

type removeIndexer interface {
	RemoveHeader(header nodeData.HeaderHandler) error
	RemoveTransactions(blk nodeData.HeaderHandler) error
	RemoveAccountsHistory(blockTimestamp int64) error
	RevertAccountBalances(blockTimestamp int64) error
}

type saveEpochInfo interface {
	SaveEpochInfo(epoch uint32, validators []kapp.ValidatorAccountInfoHandler) error
}

type saveProposalsInfo interface {
	UpdateProposalsAndParameters(proposalIDs []string) error
}

type saveTpsBenchmark interface {
	SaveNodeStatistics(tpsBenchmark statistics.TPSBenchmark) error
}

type saveAssetIndexer interface {
	SaveAssets(assetsSlice []*data.Asset) error
}

type saveAccountsIndexer interface {
	SaveAccounts(blockTimestamp int64, accounts []*data.Account) error
	SavePeersAccounts(validators []kapp.ValidatorAccountInfoHandler) error
}
