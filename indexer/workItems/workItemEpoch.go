package workItems

import "github.com/klever-io/klever-go/core/kapp"

type itemEpoch struct {
	indexer    saveEpochInfo
	epoch      uint32
	validators []kapp.ValidatorAccountInfoHandler
}

// NewItemEpoch will create a new instance of itemEpoch
func NewItemEpoch(indexer saveEpochInfo, epoch uint32, validators []kapp.ValidatorAccountInfoHandler) WorkItemHandler {
	return &itemEpoch{
		indexer:    indexer,
		epoch:      epoch,
		validators: validators,
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (wie *itemEpoch) IsInterfaceNil() bool {
	return wie == nil
}

// Save will save epoch info in elasticsearch database
func (wie *itemEpoch) Save() error {
	err := wie.indexer.SaveEpochInfo(wie.epoch, wie.validators)
	if err != nil {
		log.Warn("itemRating.Save", "could not index epoch info", err.Error())
		return err
	}

	return nil
}
