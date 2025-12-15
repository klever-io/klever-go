package mock

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/core/statistics"
	nodeData "github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/indexer/data"
)

// ElasticProcessorStub -
type ElasticProcessorStub struct {
	SaveNodeStatisticsCalled           func(tpsBenchmark statistics.TPSBenchmark) error
	SaveHeaderCalled                   func(header nodeData.HeaderHandler, signer []byte, txsSize int, validators []string) error
	SaveAssetsCalled                   func(assetsSlice []*data.Asset) error
	SaveEpochInfoCalled                func(epoch uint32, validatorsPubkeys []kapp.ValidatorAccountInfoHandler) error
	RemoveHeaderCalled                 func(header nodeData.HeaderHandler) error
	RemoveTransactionsCalled           func(header nodeData.HeaderHandler) error
	RemoveAccountsHistoryCalled        func(blockTimestamp int64) error
	RevertAccountBalancesCalled        func(blockTimestamp int64) error
	SaveTransactionsCalled             func(header nodeData.HeaderHandler, pool *indexer.Pool) error
	SaveAccountsCalled                 func(timestamp int64, acc []*data.Account) error
	SavePeerAccountCalled              func(account *data.ValidatorAccountInfo) error
	UpdateProposalsAndParametersCalled func([]string) error
}

// SaveHeader -
func (eim *ElasticProcessorStub) SaveHeader(header nodeData.HeaderHandler, signer []byte, txsSize int, validators []string) error {
	if eim.SaveHeaderCalled != nil {
		return eim.SaveHeaderCalled(header, signer, txsSize, validators)
	}
	return nil
}

func (eim *ElasticProcessorStub) SavePeersAccounts(pubkeys []kapp.ValidatorAccountInfoHandler) error {
	return nil
}

// SaveAssets -
func (eim *ElasticProcessorStub) SaveAssets(assetsSlice []*data.Asset) error {
	if eim.SaveAssetsCalled != nil {
		return eim.SaveAssetsCalled(assetsSlice)
	}
	return nil
}

// SaveEpochInfo -
func (eim *ElasticProcessorStub) SaveEpochInfo(epoch uint32, validatorsPubkeys []kapp.ValidatorAccountInfoHandler) error {
	if eim.SaveEpochInfoCalled != nil {
		return eim.SaveEpochInfoCalled(epoch, validatorsPubkeys)
	}
	return nil
}

// RemoveHeader -
func (eim *ElasticProcessorStub) RemoveHeader(header nodeData.HeaderHandler) error {
	if eim.RemoveHeaderCalled != nil {
		return eim.RemoveHeaderCalled(header)
	}
	return nil
}

// // RemoveTransactions -
func (eim *ElasticProcessorStub) RemoveTransactions(header nodeData.HeaderHandler) error {
	if eim.RemoveTransactionsCalled != nil {
		return eim.RemoveTransactionsCalled(header)
	}
	return nil
}

// RemoveAccountsHistory -
func (eim *ElasticProcessorStub) RemoveAccountsHistory(blockTimestamp int64) error {
	if eim.RemoveAccountsHistoryCalled != nil {
		return eim.RemoveAccountsHistoryCalled(blockTimestamp)
	}
	return nil
}

// RevertAccountBalances -
func (eim *ElasticProcessorStub) RevertAccountBalances(blockTimestamp int64) error {
	if eim.RevertAccountBalancesCalled != nil {
		return eim.RevertAccountBalancesCalled(blockTimestamp)
	}
	return nil
}

// SaveTransactions -
func (eim *ElasticProcessorStub) SaveTransactions(header nodeData.HeaderHandler, pool *indexer.Pool) error {
	if eim.SaveTransactionsCalled != nil {
		return eim.SaveTransactionsCalled(header, pool)
	}
	return nil
}

// SaveNodeStatistics -
func (eim *ElasticProcessorStub) SaveNodeStatistics(tpsBenchmark statistics.TPSBenchmark) error {
	if eim.SaveNodeStatisticsCalled != nil {
		return eim.SaveNodeStatisticsCalled(tpsBenchmark)
	}
	return nil
}

// SaveAccounts -
func (eim *ElasticProcessorStub) SaveAccounts(timestamp int64, acc []*data.Account) error {
	if eim.SaveAccountsCalled != nil {
		return eim.SaveAccountsCalled(timestamp, acc)
	}

	return nil
}

// SavePeerAccount -
func (eim *ElasticProcessorStub) SavePeerAccount(acc *data.ValidatorAccountInfo) error {
	if eim.SavePeerAccountCalled != nil {
		return eim.SavePeerAccountCalled(acc)
	}

	return nil
}

func (eim *ElasticProcessorStub) UpdateProposalsAndParameters(proposalsIDs []string) error {
	if eim.UpdateProposalsAndParametersCalled != nil {
		return eim.UpdateProposalsAndParametersCalled(proposalsIDs)
	}

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (eim *ElasticProcessorStub) IsInterfaceNil() bool {
	return eim == nil
}
