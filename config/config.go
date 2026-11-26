package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/tools"
	"gopkg.in/yaml.v2"
)

var log = logger.GetOrCreate("config")

type BlockSizeThrottleConfig struct {
	MinSizeInBytes uint32 `yaml:"minSizeInBytes"`
	MaxSizeInBytes uint32 `yaml:"maxSizeInBytes"`
}

// LogsConfig will hold settings related to the logging sub-system
type LogsConfig struct {
	LogFileLifeSpanInSec int   `yaml:"logFileLifeSpanInSec"`
	LogFileSizeSpanInSec int   `yaml:"logFileSizeSpanInSec"`
	MaxFileSize          int64 `yaml:"maxFileSize"`
	MaxBackups           int   `yaml:"maxBackups"`
}

// SoftwareVersionConfig will hold the configuration for software version checker
type SoftwareVersionConfig struct {
	StableTagLocation        string `yaml:"stableTagLocation"`
	PollingIntervalInMinutes int    `yaml:"pollingIntervalInMinutes"`
}

// TypeConfig will map the string type configuration
type TypeConfig struct {
	Type string `yaml:"type"`
}

// MaxNodesChangeConfig defines a config change tuple, with a maximum number enabled in a certain epoch number
type MaxNodesChangeConfig struct {
	EpochEnable    uint32 `yaml:"epochEnable"`
	NodesToShuffle uint32 `yaml:"nodesToShuffle"`
}

// TrieSyncConfig represents the trie synchronization configuration area
type TrieSyncConfig struct {
	NumConcurrentTrieSyncers  int `yaml:"numConcurrentTrieSyncers"`
	MaxHardCapForMissingNodes int `yaml:"maxHardCapForMissingNodes"`
}

// ImportDbConfig will hold the import-db parameters
type ImportDbConfig struct {
	IsImportDBMode         bool   `yaml:"isImportDBMode"`
	ImportDBWorkingDir     string `yaml:"importDBWorkingDir"`
	ImportDbNoSigCheckFlag bool   `yaml:"importDbNoSigCheckFlag"`
	ImportDBStartInEpoch   uint32 `yaml:"importDBStartInEpoch"`
}

// Config handle all configs
type Config struct {
	BlockSizeThrottle     BlockSizeThrottleConfig `yaml:"blockSizeThrottleConfig"`
	Logs                  LogsConfig              `yaml:"logs"`
	Debug                 DebugConfig             `yaml:"debug"`
	P2P                   P2PConfig               `yaml:"p2p"`
	SoftwareVersionConfig SoftwareVersionConfig   `yaml:"softwareVersionConfig"`
	TrieSync              TrieSyncConfig          `yaml:"trieSync"`

	NTP         NTPConfig           `yaml:"ntp"`
	Preferences PreferencesConfig   `yaml:"preferences"`
	Health      HealthServiceConfig `yaml:"health"`

	Storages StoragesConfig `yaml:"storages"`

	DbLookupExtensions     DbLookupExtensionsConfig  `yaml:"dbLookupExtensions"`
	StoragePruning         StoragePruningConfig      `yaml:"storagePruning"`
	LogsAndEvents          LogsAndEventsConfig       `yaml:"logsAndEvents"`
	WhiteListPool          CacheConfig               `yaml:"whiteListPool"`
	WhiteListerVerifiedTxs CacheConfig               `yaml:"whiteListerVerifiedTxs"`
	Antiflood              AntifloodConfig           `yaml:"antiflood"`
	Heartbeat              HeartbeatConfig           `yaml:"heartbeat"`
	ResourceStats          ResourceStatsConfig       `yaml:"resourceStats"`
	ValidatorStatistics    ValidatorStatisticsConfig `yaml:"validatorStatistics"`

	AccountsTrieStorage      StorageConfig             `yaml:"accountsTrieStorage"`
	PeerAccountsTrieStorage  StorageConfig             `yaml:"peerAccountsTrieStorage"`
	KAppAccountsTrieStorage  StorageConfig             `yaml:"kappAccountsTrieStorage"`
	EvictionWaitingList      EvictionWaitingListConfig `yaml:"evictionWaitingList"`
	StateTriesConfig         StateTriesConfig          `yaml:"stateTriesConfig"`
	TrieSnapshotDB           DBConfig                  `yaml:"trieSnapshotDB"`
	TrieStorageManagerConfig TrieStorageManagerConfig  `yaml:"trieStorageManager"`
	Versions                 VersionsConfig            `yaml:"versions"`

	TxDataPool            CacheConfig                  `yaml:"txDataPool"`
	HeadersPoolConfig     HeadersPoolConfig            `yaml:"headersPoolConfig"`
	TrieNodesDataPool     CacheConfig                  `yaml:"trieNodesDataPool"`
	SmartContractDataPool CacheConfig                  `yaml:"smartContractDataPool"`
	VirtualMachine        VirtualMachineServicesConfig `yaml:"virtualMachine"`

	// Network Cache Configs
	PublicKeyPeerID       CacheConfig `yaml:"publicKeyPeerID"`
	PeerIDShardID         CacheConfig `yaml:"peerIDShardID"`
	PublicKeyPIDSignature CacheConfig `yaml:"publicKeyPIDSignature"`
	PeerHonesty           CacheConfig `yaml:"peerHonesty"`
	VMOutputCacher        CacheConfig `yaml:"vmOutputCacher"`

	// Crypto
	Hasher            TypeConfig        `yaml:"hasher"`
	MultisigHasher    TypeConfig        `yaml:"multisigHasher"`
	TxSignHasher      TypeConfig        `yaml:"txSignHasher"`
	TxSignMarshalizer TypeConfig        `yaml:"txSignMarshalizer"`
	Marshalizer       MarshalizerConfig `yaml:"marshalizer"`

	// Ratings
	Ratings RatingsConfig `yaml:"ratings"`

	EpochStart EpochStartConfig `yaml:"epochStart"`

	// StartInEpoch is enable or not
	StartInEpoch bool `yaml:"startInEpoch"`

	// Config for import DB Mode
	ImportDbConfig ImportDbConfig `yaml:"importDbConfig"`

	// EnableEpochs is loaded from enableEpochs.yaml, never from config.yaml
	EnableEpochs EnableEpochs `yaml:"-"`

	// GasScheduleConfig is loaded from enableEpochs.yaml, never from config.yaml
	GasScheduleConfig GasScheduleConfig `yaml:"-"`

	// MaxNodesChangeEnableEpoch is loaded from enableEpochs.yaml, never from config.yaml
	MaxNodesChangeEnableEpoch []MaxNodesChangeConfig `yaml:"-"`

	ConsensusMonitorList []string                   `yaml:"consensusMonitorList"`
	ConsensusMonitoring  *ConsensusMonitoringConfig `yaml:"consensusMonitoring"`
}

// LogsAndEventsConfig hold the configuration for the logs and events
type LogsAndEventsConfig struct {
	SaveInStorageEnabled bool          `yaml:"saveInStorageEnabled"`
	TxLogsStorage        StorageConfig `yaml:"txLogsStorage"`
}

// VirtualMachineConfig holds configuration for a Virtual Machine service
type VirtualMachineConfig struct {
	WasmVMVersions                      []WasmVMVersionByEpoch `yaml:"wasmVMVersions"`
	TimeOutForSCExecutionInMilliseconds uint32                 `yaml:"timeOutForSCExecutionInMilliseconds"`
	TimeOutTolerancePercentage          uint32                 `yaml:"timeOutTolerancePercentage"` // Percentage to add for cosigner tolerance (e.g., 20 = 20%)
	WasmerSIGSEGVPassthrough            bool                   `yaml:"wasmerSIGSEGVPassthrough"`
}

// VirtualMachineServicesConfig holds configuration for the Virtual Machine(s): both querying and execution services.
type VirtualMachineServicesConfig struct {
	Execution VirtualMachineConfig      `yaml:"execution"`
	Querying  QueryVirtualMachineConfig `yaml:"querying"`
	GasConfig VirtualMachineGasConfig   `yaml:"gasConfig"`
}

// QueryVirtualMachineConfig holds the configuration for the virtual machine(s) used in query process
type QueryVirtualMachineConfig struct {
	VirtualMachineConfig `yaml:",inline"`
	NumConcurrentVMs     int `yaml:"numConcurrentVMs"`
	// MaxQueryWaitInMilliseconds bounds how long a query waits for a busy query VM
	// before it is refused. 0 means the default (1 second).
	MaxQueryWaitInMilliseconds uint32 `yaml:"maxQueryWaitInMilliseconds"`
}

// VirtualMachineGasConfig holds the configuration for the virtual machine(s) gas operations
type VirtualMachineGasConfig struct {
	MetaMaxGasPerVmQuery uint64 `yaml:"metaMaxGasPerVmQuery"`
}

// WasmVMVersionByEpoch represents the Wasm VM version to be used starting with an epoch
type WasmVMVersionByEpoch struct {
	StartEpoch uint32 `yaml:"startEpoch"`
	Version    string `yaml:"version"`
}

// loadYAMLStrict decodes a yaml file into out, rejecting keys that match no struct tag so a typo
// fails at boot instead of leaving the field at its zero value.
func loadYAMLStrict(path string, kind string, out interface{}) error {
	data, err := tools.ReadFile(path)
	if err != nil {
		// A missing file is left to the caller, which may fall back to defaults.
		if !errors.Is(err, os.ErrNotExist) {
			log.Error("failed to load "+kind+" file", "file", path, "error", err)
		}
		return err
	}
	if err := decodeSingleYAMLDocument(data, out); err != nil {
		err = fmt.Errorf("%s file %s: %w", kind, path, err)
		log.Error("failed to unmarshal "+kind, "error", err)
		return err
	}
	return nil
}

// decodeSingleYAMLDocument decodes data strictly and rejects a second document, which
// yaml.UnmarshalStrict would silently skip.
func decodeSingleYAMLDocument(data []byte, out interface{}) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.SetStrict(true)
	if err := dec.Decode(out); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	// Trailing empty documents (a final "---") are allowed; any content after the first is not.
	for {
		var extra interface{}
		err := dec.Decode(&extra)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if extra != nil {
			return errors.New("multiple YAML documents are not supported")
		}
	}
}

// LoadFromPath of all config files
func LoadFromPath(relativePath string) (Config, error) {
	var config Config
	err := loadYAMLStrict(relativePath, "config", &config)
	return config, err
}

// LoadAPIConfig of all config files
func LoadAPIConfig(relativePath string) (APIRoutesConfig, error) {
	var config APIRoutesConfig
	err := loadYAMLStrict(relativePath, "API config", &config)
	return config, err
}

// LoadExternalConfig of all config files
func LoadExternalConfig(relativePath string) (ExternalConfig, error) {
	var config ExternalConfig
	err := loadYAMLStrict(relativePath, "external config", &config)
	return config, err
}

// LoadEnableEpochsConfig -
func LoadEnableEpochsConfig(relativePath string) (EnableEpochsConfig, error) {
	var config EnableEpochsConfig
	err := loadYAMLStrict(relativePath, "enable epochs config", &config)
	return config, err
}

// LoadGasScheduleConfig returns a map[string]uint64 of gas costs read from the provided config file
func LoadGasScheduleConfig(filepath string) (map[string]map[string]uint64, error) {
	gasScheduleConfig := make(map[string]interface{})
	if err := loadYAMLStrict(filepath, "gas schedule", &gasScheduleConfig); err != nil {
		return nil, err
	}

	flattenedGasSchedule := make(map[string]map[string]uint64)
	for libType, costs := range gasScheduleConfig {
		flattenedGasSchedule[libType] = make(map[string]uint64)
		costsMap, ok := costs.(map[interface{}]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to unmarshal gas schedule section %s: not a map of costs", libType)
		}
		for operationName, cost := range costsMap {
			operationNameStr, ok := operationName.(string)
			if !ok {
				return nil, errors.New("failed to unmarshal gas schedule operation name")
			}

			costUint64, err := gasScheduleCostToUint64(cost)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal gas schedule cost %s.%s: %w", libType, operationNameStr, err)
			}

			flattenedGasSchedule[libType][operationNameStr] = costUint64
		}
	}

	return flattenedGasSchedule, nil
}

// gasScheduleCostToUint64 converts a decoded gas schedule cost to uint64. A negative cost would wrap to a
// huge uint64 and a fractional one would be truncated, so both are rejected.
func gasScheduleCostToUint64(cost interface{}) (uint64, error) {
	switch costTyped := cost.(type) {
	case int:
		if costTyped >= 0 {
			return uint64(costTyped), nil // #nosec G115 -- checked non-negative
		}
	case int64:
		if costTyped >= 0 {
			return uint64(costTyped), nil // #nosec G115 -- checked non-negative
		}
	case float64:
		if costTyped >= 0 && costTyped == math.Trunc(costTyped) && costTyped < math.MaxUint64 {
			return uint64(costTyped), nil // #nosec G115 -- checked non-negative, integral and in range
		}
	default:
		log.Error("failed to unmarshal gas schedule cost - unknown type", "type", reflect.TypeOf(cost))
		return 0, fmt.Errorf("unknown type: %v", reflect.TypeOf(cost))
	}

	return 0, fmt.Errorf("%v is not a non-negative integer", cost)
}
