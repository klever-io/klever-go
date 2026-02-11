package config

// ConsensusMonitoringConfig holds configuration for consensus monitoring and alerting
type ConsensusMonitoringConfig struct {
	// NetworkDegradedThreshold specifies the minimum number of failed validators
	// required to trigger a network degraded alert
	NetworkDegradedThreshold uint32 `yaml:"networkDegradedThreshold"`

	// NetworkDegradedCooldownSlots specifies the number of slots to wait
	// before sending another network degraded alert
	NetworkDegradedCooldownSlots uint32 `yaml:"networkDegradedCooldownSlots"`
}
