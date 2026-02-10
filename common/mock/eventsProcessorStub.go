package mock

import (
	"github.com/klever-io/klever-go/core/kapp"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/indexer"
	"github.com/klever-io/klever-go/data/state"
)

type EventsProcessorStub struct {
	EnabledCalled                      func() bool
	SaveBlockCalled                    func(args *indexer.ArgsSaveBlockData)
	RevertIndexedBlockCalled           func(header data.HeaderHandler)
	SaveValidatorsRatingCalled         func(validators []kapp.ValidatorAccountInfoHandler)
	SaveEpochInfoCalled                func(epoch uint32, validators []kapp.ValidatorAccountInfoHandler)
	SaveAccountsCalled                 func(blockTimestamp int64, acc []state.UserAccountHandler)
	UpdateProposalsAndParametersCalled func(proposalIDs []string)
}

func (e *EventsProcessorStub) Enabled() bool {
	if e.EnabledCalled != nil {
		return e.EnabledCalled()
	}
	return true
}

func (e *EventsProcessorStub) SaveBlock(args *indexer.ArgsSaveBlockData) {
	if e.SaveBlockCalled != nil {
		e.SaveBlockCalled(args)
	}
}

func (e *EventsProcessorStub) RevertIndexedBlock(header data.HeaderHandler) {
	if e.RevertIndexedBlockCalled != nil {
		e.RevertIndexedBlockCalled(header)
	}
}

func (e *EventsProcessorStub) SaveValidatorsRating(validators []kapp.ValidatorAccountInfoHandler) {
	if e.SaveValidatorsRatingCalled != nil {
		e.SaveValidatorsRatingCalled(validators)
	}
}

func (e *EventsProcessorStub) SaveEpochInfo(epoch uint32, validators []kapp.ValidatorAccountInfoHandler) {
	if e.SaveEpochInfoCalled != nil {
		e.SaveEpochInfoCalled(epoch, validators)
	}
}

func (e *EventsProcessorStub) SaveAccounts(blockTimestamp int64, acc []state.UserAccountHandler) {
	if e.SaveAccountsCalled != nil {
		e.SaveAccountsCalled(blockTimestamp, acc)
	}
}

func (e *EventsProcessorStub) UpdateProposalsAndParameters(proposalIDs []string) {
	if e.UpdateProposalsAndParametersCalled != nil {
		e.UpdateProposalsAndParametersCalled(proposalIDs)
	}
}

func (e *EventsProcessorStub) IsInterfaceNil() bool {
	return e == nil
}
