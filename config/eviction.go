package config

// EvictionWaitingListConfig will hold the configuration for the EvictionWaitingList
type EvictionWaitingListConfig struct {
	Size uint     `yaml:"size"`
	DB   DBConfig `yaml:"db"`
}
