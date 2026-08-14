package config

// P2PConfig will hold all the P2P settings
type P2PConfig struct {
	Node                NodeConfig                `yaml:"node"`
	KadDhtPeerDiscovery KadDhtPeerDiscoveryConfig `yaml:"kadDhtPeerDiscovery"`
	Sharding            ShardingConfig            `yaml:"sharding"`
	ResourceManager     ResourceManagerConfig     `yaml:"resourceManager"`
	DirectSend          DirectSendConfig          `yaml:"directSend"`
}

// NodeConfig will hold basic p2p settings
type NodeConfig struct {
	Port                       string `yaml:"port"`
	Seed                       string `yaml:"seed"`
	LegacySeed                 bool   `yaml:"legacySeed"`
	MaximumExpectedPeerCount   uint64 `yaml:"maximumExpectedPeerCount"`
	ThresholdMinConnectedPeers uint32 `yaml:"thresholdMinConnectedPeers"`
	BroadcastIP                string `yaml:"broadcastIP"`
}

// KadDhtPeerDiscoveryConfig will hold the kad-dht discovery config settings
type KadDhtPeerDiscoveryConfig struct {
	Enabled                          bool     `yaml:"enabled"`
	RefreshIntervalInSec             uint32   `yaml:"refreshIntervalInSec"`
	ProtocolID                       string   `yaml:"protocolID"`
	InitialPeerList                  []string `yaml:"initialPeerList"`
	BucketSize                       uint32   `yaml:"bucketSize"`
	RoutingTableRefreshIntervalInSec uint32   `yaml:"routingTableRefreshIntervalInSec"`
}

// ShardingConfig will hold the network sharding config settings
type ShardingConfig struct {
	TargetPeerCount         int    `yaml:"targetPeerCount"`
	MaxIntraShardValidators uint32 `yaml:"maxIntraShardValidators"`
	MaxCrossShardValidators uint32 `yaml:"maxCrossShardValidators"`
	MaxIntraShardObservers  uint32 `yaml:"maxIntraShardObservers"`
	MaxCrossShardObservers  uint32 `yaml:"maxCrossShardObservers"`
	Type                    string `yaml:"type"`
}

// Values for ResourceManagerConfig.Strategy. The empty string is equivalent to
// "default" and selects libp2p's auto-scaled DefaultResourceManager.
const (
	ResourceManagerStrategyDefault       = ""
	ResourceManagerStrategyLibp2pDefault = "default"
	ResourceManagerStrategyNull          = "null"
	ResourceManagerStrategyScaled        = "scaled"
)

// ResourceManagerConfig configures libp2p's ResourceManager.
type ResourceManagerConfig struct {
	Strategy        string `yaml:"strategy"`
	ScaledMemoryMiB int    `yaml:"scaledMemoryMiB"`
	// The following per-subnet knobs apply only to strategy "scaled"; 0 = libp2p default.
	MaxConnsPerIPv4       int     `yaml:"maxConnsPerIPv4"`       // max connections per IPv4 /32
	MaxConnsPerIPv6Subnet int     `yaml:"maxConnsPerIPv6Subnet"` // max connections per IPv6 /56 (/48 derived as 8x)
	ConnRatePerSec        float64 `yaml:"connRatePerSec"`        // new connections/sec per subnet
	ConnRateBurst         int     `yaml:"connRateBurst"`         // burst allowance over ConnRatePerSec
}

// DirectSendConfig caps the inbound direct-send streams a node accepts; 0 = built-in default
// (4 per peer, 512 in total across all peers).
type DirectSendConfig struct {
	MaxInboundStreamsPerPeer int `yaml:"maxInboundStreamsPerPeer"`
	MaxInboundStreamsTotal   int `yaml:"maxInboundStreamsTotal"`
}
