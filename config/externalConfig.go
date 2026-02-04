package config

// ExternalConfig will hold the configurations for external tools, such as Explorer or Elastic Search
type ExternalConfig struct {
	ElasticSearchConnector ElasticSearchConfig `yaml:"elasticSearchConnector"`
	Websocket              WebsocketConfig     `yaml:"websocket"`
}

// WebsocketConfig holds the configuration for websocket event broadcasting
type WebsocketConfig struct {
	Enabled bool `yaml:"enabled"`
}

// ElasticSearchConfig will hold the configuration for the elastic search
type ElasticSearchConfig struct {
	Enabled          bool     `yaml:"enabled"`
	IndexerCacheSize int      `yaml:"indexerCacheSize"`
	URL              string   `yaml:"url"`
	UseKibana        bool     `yaml:"useKibana"`
	Username         string   `yaml:"username"`
	Password         string   `yaml:"password"`
	EnabledIndexes   []string `yaml:"enabledIndexes"`
}
