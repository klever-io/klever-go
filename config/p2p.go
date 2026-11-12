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

// ConnectionManagerConfig sets the watermarks of the libp2p BasicConnMgr that regular (non-seed)
// nodes run; seed nodes ignore it and never trim. Both watermarks count connections, not peers.
// Once the connection count reaches HighWater, the manager closes the connections of the
// lowest-value unprotected peers that are older than GracePeriodInSec, until LowWater unprotected,
// out-of-grace connections remain. Connections of protected peers (known validators, and peers
// gossipsub and kad-dht protect) are not counted against LowWater and never trimmed, so LowWater is
// the connection budget for unprotected peers and LowWater plus the protected peers' inbound
// connections must stay below the inbound ResourceManager limit. A zero value selects the built-in
// default: LowWater 100, HighWater 300, GracePeriodInSec 60.
//
// Trimming never closes a connection younger than GracePeriodInSec, so a peer that keeps rotating
// its identity and reconnecting is never trimmed. MaxUnprotectedInboundConns closes that gap: the
// node sums the inbound connections of the peers that are not known validators and not protected
// by any subsystem; at admission it refuses new inbound connections from such peers once the cap is
// reached, and every 10s it closes whole peers whatever their age until the sum is within it. It
// counts connections because the ResourceManager does and one identity can hold several. Closing
// order: unknown peers connected for over 60s (newest first), then observers (newest first), then
// unknown peers younger than 60s (oldest first), since a validator that just connected is unknown
// until its next heartbeat (every 20-25s). The cap is armed once the node's peer shard resolver is
// installed and enforced 60s after that, so reconnecting validators are identified first. Zero
// selects the default of 100.
type ConnectionManagerConfig struct {
	LowWater                   int `yaml:"lowWater"`
	HighWater                  int `yaml:"highWater"`
	GracePeriodInSec           int `yaml:"gracePeriodInSec"`
	MaxUnprotectedInboundConns int `yaml:"maxUnprotectedInboundConns"`
}
