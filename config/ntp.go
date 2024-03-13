package config

// NTPConfig will hold the configuration for NTP queries
type NTPConfig struct {
	Hosts               []string `yaml:"hosts"`
	Port                int      `yaml:"port"`
	TimeoutMilliseconds int      `yaml:"timeoutMilliseconds"`
	SyncPeriodSeconds   int      `yaml:"syncPeriodSeconds"`
	Version             int      `yaml:"version"`
}
