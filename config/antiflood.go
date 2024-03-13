package config

// IncreaseFactorConfig defines the configurations used to increase the set values of a flood preventer
type IncreaseFactorConfig struct {
	Threshold uint32  `yaml:"threshold"`
	Factor    float32 `yaml:"factor"`
}

// BlackListConfig will hold the p2p peer black list threshold values
type BlackListConfig struct {
	ThresholdNumMessagesPerInterval uint32 `yaml:"thresholdNumMessagesPerInterval"`
	ThresholdSizePerInterval        uint64 `yaml:"thresholdSizePerInterval"`
	NumFloodingSlots                uint32 `yaml:"numFloodingSlots"`
	PeerBanDurationInSeconds        uint32 `yaml:"peerBanDurationInSeconds"`
}

// AntifloodLimitsConfig will hold the maximum antiflood limits in both number of messages and total
// size of the messages
type AntifloodLimitsConfig struct {
	BaseMessagesPerInterval uint32               `yaml:"baseMessagesPerInterval"`
	TotalSizePerInterval    uint64               `yaml:"totalSizePerInterval"`
	IncreaseFactor          IncreaseFactorConfig `yaml:"increaseFactor"`
}

// FloodPreventerConfig will hold all flood preventer parameters
type FloodPreventerConfig struct {
	IntervalInSeconds uint32                `yaml:"intervalInSeconds"`
	ReservedPercent   float32               `yaml:"reservedPercent"`
	PeerMaxInput      AntifloodLimitsConfig `yaml:"peerMaxInput"`
	BlackList         BlackListConfig       `yaml:"blackList"`
}

// EndpointsThrottlersConfig holds a pair of an endpoint and its maximum number of simultaneous go routines
type EndpointsThrottlersConfig struct {
	Endpoint         string `yaml:"rndpoint"`
	MaxNumGoRoutines int32  `yaml:"maxNumGoRoutines"`
}

// WebServerAntifloodConfig will hold the anti-flooding parameters for the web server
type WebServerAntifloodConfig struct {
	SimultaneousRequests         uint32                      `yaml:"simultaneousRequests"`
	SameSourceRequests           uint32                      `yaml:"sameSourceRequests"`
	SameSourceResetIntervalInSec uint32                      `yaml:"sameSourceResetIntervalInSec"`
	EndpointsThrottlers          []EndpointsThrottlersConfig `yaml:"endpointsThrottlers"`
}

// TopicMaxMessagesConfig will hold the maximum number of messages/sec per topic value
type TopicMaxMessagesConfig struct {
	Topic             string `yaml:"topic"`
	NumMessagesPerSec uint32 `yaml:"numMessagesPerSec"`
}

// TopicAntifloodConfig will hold the maximum values per second to be used in certain topics
type TopicAntifloodConfig struct {
	DefaultMaxMessagesPerSec uint32                   `yaml:"defaultMaxMessagesPerSec"`
	MaxMessages              []TopicMaxMessagesConfig `yaml:"maxMessages"`
}

// TxAccumulatorConfig will hold the tx accumulator config values
type TxAccumulatorConfig struct {
	MaxAllowedTimeInMilliseconds   uint32 `yaml:"maxAllowedTimeInMilliseconds"`
	MaxDeviationTimeInMilliseconds uint32 `yaml:"maxDeviationTimeInMilliseconds"`
}

// AntifloodConfig will hold all p2p antiflood parameters
type AntifloodConfig struct {
	Enabled                   bool                     `yaml:"enabled"`
	NumConcurrentResolverJobs int32                    `yaml:"numConcurrentResolverJobs"`
	OutOfSpecs                FloodPreventerConfig     `yaml:"outOfSpecs"`
	FastReacting              FloodPreventerConfig     `yaml:"fastReacting"`
	SlowReacting              FloodPreventerConfig     `yaml:"slowReacting"`
	PeerMaxOutput             AntifloodLimitsConfig    `yaml:"peerMaxOutput"`
	Cache                     CacheConfig              `yaml:"cache"`
	WebServer                 WebServerAntifloodConfig `yaml:"webServer"`
	Topic                     TopicAntifloodConfig     `yaml:"topic"`
	TxAccumulator             TxAccumulatorConfig      `yaml:"txAccumulator"`
}
