package epochproviders

import (
	"github.com/klever-io/klever-go/data/retriever"
)

var _ retriever.EpochProviderByNonce = (*simpleEpochProviderByNonce)(nil)

//TODO: implement a real epoch provider by nonce

// simpleEpochProviderByNonce will use the epoch handler to return the current epoch
type simpleEpochProviderByNonce struct {
	epochHandler retriever.EpochHandler
}

// NewSimpleEpochProviderByNonce will return a new instance of simpleEpochProviderByNonce
func NewSimpleEpochProviderByNonce(epochHandler retriever.EpochHandler) *simpleEpochProviderByNonce {
	return &simpleEpochProviderByNonce{
		epochHandler: epochHandler,
	}
}

// EpochForNonce will return the current epoch from the epoch handler
func (sepbn *simpleEpochProviderByNonce) EpochForNonce(_ uint64) (uint32, error) {
	return sepbn.epochHandler.Epoch(), nil
}

// IsInterfaceNil returns true if there is not value under the interface
func (sepbn *simpleEpochProviderByNonce) IsInterfaceNil() bool {
	return sepbn == nil
}
