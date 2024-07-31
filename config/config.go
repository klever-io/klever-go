package config

import (
	"errors"
	"fmt"
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

// HardforkConfig holds the configuration for the hardfork trigger
type HardforkConfig struct {
	ExportStateStorageConfig     StorageConfig `yaml:"exportStateStorageConfig"`
	ExportKeysStorageConfig      StorageConfig `yaml:"exportKeysStorageConfig"`
	ExportTriesStorageConfig     StorageConfig `yaml:"exportTriesStorageConfig"`
	ImportStateStorageConfig     StorageConfig `yaml:"importStateStorageConfig"`
	ImportKeysStorageConfig      StorageConfig `yaml:"importKeysStorageConfig"`
	PublicKeyToListenFrom        string        `yaml:"publicKeyToListenFrom"`
	ImportFolder                 string        `yaml:"importFolder"`
	GenesisTime                  int64         `yaml:"genesisTime"`
	StartSlot                    uint64        `yaml:"startSlot"`
	StartNonce                   uint64        `yaml:"startNonce"`
	CloseAfterExportInMinutes    uint32        `yaml:"closeAfterExportInMinutes"`
	StartEpoch                   uint32        `yaml:"startEpoch"`
	ValidatorGracePeriodInEpochs uint32        `yaml:"validatorGracePeriodInEpochs"`
	EnableTrigger                bool          `yaml:"enableTrigger"`
	EnableTriggerFromP2P         bool          `yaml:"enableTriggerFromP2P"`
	MustImport                   bool          `yaml:"mustImport"`
	AfterHardFork                bool          `yaml:"afterHardFork"`
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

	NTP         NTPConfig         `yaml:"ntp"`
	Preferences PreferencesConfig `yaml:"preferences"`

	Hardfork HardforkConfig      `yaml:"hardfork"`
	Health   HealthServiceConfig `yaml:"health"`

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

	// EnableEpochs configuration
	EnableEpochs EnableEpochs

	// gasSchedule configuration
	GasScheduleConfig GasScheduleConfig

	// Shuffler
	MaxNodesChangeEnableEpoch []MaxNodesChangeConfig

	ConsensusMonitorList []string `yaml:"consensusMonitorList"`
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
	VirtualMachineConfig
	NumConcurrentVMs int `yaml:"numConcurrentVMs"`
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

// LoadFromPath of all config files
func LoadFromPath(relativePath string) (Config, error) {
	var config Config
	data, err := tools.ReadFile(relativePath)
	if err != nil {
		log.Error("failed to load config file", "file", relativePath, err)
		return config, err
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Error("failed to unmarshal config", err)
		return config, err
	}

	return config, nil
}

// LoadAPIConfig of all config files
func LoadAPIConfig(relativePath string) (APIRoutesConfig, error) {
	var config APIRoutesConfig
	data, err := tools.ReadFile(relativePath)

	if err != nil {
		log.Error("failed to load API config file", "file", relativePath, err)
		return config, err
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Error("failed to unmarshal config", err)
		return config, err
	}
	return config, nil
}

// LoadExternalConfig of all config files
func LoadExternalConfig(relativePath string) (ExternalConfig, error) {
	var config ExternalConfig
	data, err := tools.ReadFile(relativePath)
	if err != nil {
		log.Error("failed to load External config file", "file", relativePath, err)
		return config, err
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Error("failed to unmarshal config", err)
		return config, err
	}

	return config, nil
}

// LoadEnableEpochsConfig -
func LoadEnableEpochsConfig(relativePath string) (EnableEpochsConfig, error) {
	var config EnableEpochsConfig
	data, err := tools.ReadFile(relativePath)

	if err != nil {
		log.Error("failed to load EnableEpochs config file", "file", relativePath, err)
		return config, err
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Error("failed to unmarshal enable epochs config", err)
		return config, err
	}
	return config, nil
}

// LoadGasScheduleConfig returns a map[string]uint64 of gas costs read from the provided config file
func LoadGasScheduleConfig(filepath string) (map[string]map[string]uint64, error) {
	gasScheduleConfig := make(map[string]interface{})

	data, err := tools.ReadFile(filepath)
	if err != nil {
		log.Error("failed to load gas schedule file", "file", filepath, err)
		return nil, err
	}

	if err := yaml.Unmarshal(data, &gasScheduleConfig); err != nil {
		log.Error("failed to unmarshal gas schedule", err)
		return nil, err
	}

	flattenedGasSchedule := make(map[string]map[string]uint64)
	for libType, costs := range gasScheduleConfig {
		flattenedGasSchedule[libType] = make(map[string]uint64)
		costsMap := costs.(map[interface{}]interface{})
		for operationName, cost := range costsMap {
			operationNameStr, ok := operationName.(string)
			if !ok {
				return nil, errors.New("failed to unmarshal gas schedule operation name")
			}

			var costUint64 uint64
			switch costTyped := cost.(type) {
			case int:
				costUint64 = uint64(costTyped)
			case int64:
				costUint64 = uint64(costTyped)
			case float64:
				costUint64 = uint64(costTyped)
			default:
				log.Error("failed to unmarshal gas schedule cost - unknown type", "type", reflect.TypeOf(cost))
				return nil, fmt.Errorf("failed to unmarshal gas schedule cost - unknown type: %v", reflect.TypeOf(cost))
			}

			flattenedGasSchedule[libType][operationNameStr] = costUint64
		}
	}

	return flattenedGasSchedule, nil
}
