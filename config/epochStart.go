package config

// EpochStartConfig will hold the configuration of EpochStart settings
type EpochStartConfig struct {
	MinNumConnectedPeersToStart       int `yaml:"minNumConnectedPeersToStart"`
	MinNumOfPeersToConsiderBlockValid int `yaml:"minNumOfPeersToConsiderBlockValid"`
}
