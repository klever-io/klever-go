package config

// HealthServiceConfig will hold health service (monitoring) configuration
type HealthServiceConfig struct {
	IntervalVerifyMemoryInSeconds             int    `yaml:"intervalVerifyMemoryInSeconds"`
	IntervalDiagnoseComponentsInSeconds       int    `yaml:"intervalDiagnoseComponentsInSeconds"`
	IntervalDiagnoseComponentsDeeplyInSeconds int    `yaml:"intervalDiagnoseComponentsDeeplyInSeconds"`
	MemoryUsageToCreateProfiles               int    `yaml:"memoryUsageToCreateProfiles"`
	NumMemoryUsageRecordsToKeep               int    `yaml:"numMemoryUsageRecordsToKeep"`
	FolderPath                                string `yaml:"folderPath"`
}

// HeartbeatConfig will hold all heartbeat settings
type HeartbeatConfig struct {
	MinTimeToWaitBetweenBroadcastsInSec int    `yaml:"minTimeToWaitBetweenBroadcastsInSec"`
	MaxTimeToWaitBetweenBroadcastsInSec int    `yaml:"maxTimeToWaitBetweenBroadcastsInSec"`
	DurationToConsiderUnresponsiveInSec int    `yaml:"durationToConsiderUnresponsiveInSec"`
	HeartbeatRefreshIntervalInSec       uint32 `yaml:"heartbeatRefreshIntervalInSec"`
	HideInactiveValidatorIntervalInSec  uint32 `yaml:"hideInactiveValidatorIntervalInSec"`
}
