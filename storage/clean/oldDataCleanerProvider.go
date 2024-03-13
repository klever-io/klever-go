package clean

import (
	"github.com/klever-io/klever-go/config"
)

type oldDataCleanerProvider struct {
	cleanOldEpochsData bool
}

// NewOldDataCleanerProvider returns a new instance of oldDataCleanerProvider
func NewOldDataCleanerProvider(
	pruningStorerConfig config.StoragePruningConfig,
) (*oldDataCleanerProvider, error) {
	return &oldDataCleanerProvider{
		cleanOldEpochsData: pruningStorerConfig.CleanOldEpochsData,
	}, nil
}

// ShouldClean returns true if old data can be cleaned, based on current configuration,
func (odcp *oldDataCleanerProvider) ShouldClean() bool {
	log.Debug("oldDataCleanerProvider.ShouldClean", "value", odcp.cleanOldEpochsData)

	return odcp.cleanOldEpochsData
}

// IsInterfaceNil returns true if there is no value under the interface
func (odcp *oldDataCleanerProvider) IsInterfaceNil() bool {
	return odcp == nil
}
