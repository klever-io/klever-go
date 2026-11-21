package config

// P2PConfig will hold all the P2P settings
type P2PConfig struct {
	Node                NodeConfig                `yaml:"node"`
	KadDhtPeerDiscovery KadDhtPeerDiscoveryConfig `yaml:"kadDhtPeerDiscovery"`
	Sharding            ShardingConfig            `yaml:"sharding"`
	ResourceManager     ResourceManagerConfig     `yaml:"resourceManager"`
	DirectSend          DirectSendConfig          `yaml:"directSend"`
	ConnectionManager   ConnectionManagerConfig   `yaml:"connectionManager"`
}

// NodeConfig will hold basic p2p settings
type NodeConfig struct {
	Port                       string `yaml:"port"`
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

// DirectSendConfig bounds what a node accepts over direct send: the inbound streams (0 = built-in
// default of 4 per peer, 512 in total) and the replay cache that deduplicates frames per peer
// (MaxSeenMessages, 0 = 131072; every peer holds a fixed share of 64, so this also sets how many
// peers are tracked at once).
type DirectSendConfig struct {
	MaxInboundStreamsPerPeer int `yaml:"maxInboundStreamsPerPeer"`
	MaxInboundStreamsTotal   int `yaml:"maxInboundStreamsTotal"`
	MaxSeenMessages          int `yaml:"maxSeenMessages"`
}

// ConnectionManagerConfig configures regular (non-seed) nodes' libp2p BasicConnMgr and the inbound
// cap of the peer protector; seed nodes ignore it and never trim. LowWater, HighWater and
// GracePeriodInSec are the BasicConnMgr watermarks (in connections) and grace period;
// MaxUnprotectedInboundConns caps the inbound connections of peers that are not validators. A zero
// value selects the built-in default: 100, 300, 60 and 100. A nonzero GracePeriodInSec must be
// above the peer protector's 10s interval, or a trim could close a validator before it is
// protected; below 60s the node warns, because a validator with a fresh peer ID can stay
// unidentified until its next heartbeat. How the watermarks, the cap and its
// closing order interact is documented on peerProtector (network/p2p/libp2p/peerProtector.go) and,
// for operators, in config/node/config.yaml.
type ConnectionManagerConfig struct {
	LowWater                   int `yaml:"lowWater"`
	HighWater                  int `yaml:"highWater"`
	GracePeriodInSec           int `yaml:"gracePeriodInSec"`
	MaxUnprotectedInboundConns int `yaml:"maxUnprotectedInboundConns"`
}
