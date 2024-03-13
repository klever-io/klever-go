package config

// VersionByEpochs represents a version entry that will be applied between the provided epochs
type VersionByEpochs struct {
	StartEpoch uint32 `yaml:"startEpoch"`
	Version    string `yaml:"version"`
}

// VersionsConfig represents the versioning config area
type VersionsConfig struct {
	DefaultVersion   string            `yaml:"defaultVersion"`
	VersionsByEpochs []VersionByEpochs `yaml:"versionsByEpochs"`
	Cache            CacheConfig       `yaml:"cache"`
}
