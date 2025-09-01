package tracing

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	logger "github.com/klever-io/klever-go-logger"
)

var log = logger.GetOrCreate("core/tracing")

// Config holds the tracing configuration
type Config struct {
	Enabled          bool
	ServerURL        string
	BatchSize        int
	PushInterval     time.Duration
	ServiceName      string
	SaveOnExit       bool
	SavePath         string
	MaxSpans         int     // Maximum number of spans to keep in memory (0 = unlimited)
	CleanupThreshold float64 // Percentage to keep when cleaning up spans (0.0-1.0, default 0.9)
}

// DefaultConfig returns the default tracing configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:          false,
		ServerURL:        "",
		BatchSize:        100,
		PushInterval:     5 * time.Second,
		ServiceName:      "KleverGo",
		SaveOnExit:       false,
		SavePath:         "./traces",
		MaxSpans:         10000, // Default limit of 10,000 spans in memory
		CleanupThreshold: 0.9,   // Keep 90% of spans when cleaning up
	}
}

// LoadFromEnv loads configuration from environment variables
func (c *Config) LoadFromEnv() {
	c.loadBoolFromEnv("KLEVER_TRACING_ENABLED", &c.Enabled)
	c.loadStringFromEnv("KLEVER_TRACING_SERVER_URL", &c.ServerURL)
	c.loadIntFromEnv("KLEVER_TRACING_BATCH_SIZE", &c.BatchSize, 1, 0)
	c.loadDurationFromEnv("KLEVER_TRACING_PUSH_INTERVAL", &c.PushInterval)
	c.loadServiceName()
	c.loadBoolFromEnv("KLEVER_TRACING_SAVE_ON_EXIT", &c.SaveOnExit)
	c.loadStringFromEnv("KLEVER_TRACING_SAVE_PATH", &c.SavePath)
	c.loadIntFromEnv("KLEVER_TRACING_MAX_SPANS", &c.MaxSpans, 0, 0)
	c.loadFloatFromEnv("KLEVER_TRACING_CLEANUP_THRESHOLD", &c.CleanupThreshold, 0.0, 1.0)
}

// loadBoolFromEnv loads a boolean value from environment
func (c *Config) loadBoolFromEnv(key string, target *bool) {
	if val := os.Getenv(key); val != "" {
		*target = strings.ToLower(val) == "true" || val == "1"
	}
}

// loadStringFromEnv loads a string value from environment
func (c *Config) loadStringFromEnv(key string, target *string) {
	if val := os.Getenv(key); val != "" {
		*target = val
	}
}

// loadIntFromEnv loads an integer value from environment with optional min/max validation
func (c *Config) loadIntFromEnv(key string, target *int, minVal, maxVal int) {
	val := os.Getenv(key)
	if val == "" {
		return
	}

	intVal, err := strconv.Atoi(val)
	if err != nil {
		return
	}

	// Apply min validation
	// For minVal=0, reject negative values
	// For minVal>0, reject values below minVal
	if intVal < minVal {
		return
	}

	// Apply max validation if specified
	if maxVal > 0 && intVal > maxVal {
		return
	}

	*target = intVal
}

// loadDurationFromEnv loads a duration value from environment
func (c *Config) loadDurationFromEnv(key string, target *time.Duration) {
	if val := os.Getenv(key); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			*target = duration
		}
	}
}

// loadFloatFromEnv loads a float value from environment with range validation
func (c *Config) loadFloatFromEnv(key string, target *float64, minVal, maxVal float64) {
	val := os.Getenv(key)
	if val == "" {
		return
	}

	floatVal, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return
	}

	// Validate range
	if floatVal <= minVal || floatVal > maxVal {
		return
	}

	*target = floatVal
}

// loadServiceName loads service name from environment or generates a unique one
func (c *Config) loadServiceName() {
	if val := os.Getenv("KLEVER_TRACING_SERVICE_NAME"); val != "" {
		c.ServiceName = val
	} else {
		c.ServiceName = generateUniqueServiceName()
	}
}

// String returns a string representation of the config
func (c *Config) String() string {
	return fmt.Sprintf(
		"TracingConfig{Enabled:%v, ServerURL:%s, BatchSize:%d, PushInterval:%v, ServiceName:%s, SaveOnExit:%v, SavePath:%s, MaxSpans:%d, CleanupThreshold:%.2f}",
		c.Enabled, c.ServerURL, c.BatchSize, c.PushInterval, c.ServiceName, c.SaveOnExit, c.SavePath, c.MaxSpans, c.CleanupThreshold,
	)
}

var (
	globalConfig     *Config
	configOnce       sync.Once
	configuredTracer *ZipkinTracer
	configMu         sync.RWMutex
)

// GetConfig returns the global tracing configuration
func GetConfig() *Config {
	configOnce.Do(func() {
		globalConfig = DefaultConfig()
		globalConfig.LoadFromEnv()

		// Log the configuration
		if globalConfig.Enabled {
			log.Info("tracing: Tracing enabled", "config", globalConfig)
		}
	})
	return globalConfig
}

// InitializeFromConfig initializes the singleton tracer based on configuration
func InitializeFromConfig() *ZipkinTracer {
	config := GetConfig()

	configMu.Lock()
	defer configMu.Unlock()

	// Check if already configured
	if configuredTracer != nil {
		return configuredTracer
	}

	// Create a new tracer instance directly
	// Don't use GetZipkinTracer() as it has its own initialization logic
	tracer := NewZipkinTracer()

	if !config.Enabled {
		// Store tracer but without server push
		configuredTracer = tracer
		return tracer
	}

	// Enable server push if URL is provided
	if config.ServerURL != "" {
		tracer.EnableServerPush(
			config.ServerURL,
			config.BatchSize,
			config.PushInterval,
		)
		log.Info("tracing: Tracing initialized with server", "url", config.ServerURL)
	} else {
		log.Info("tracing: Tracing initialized in local mode", "file_save", config.SaveOnExit, "save_path", config.SavePath)
	}

	configuredTracer = tracer
	return tracer
}

// GetConfiguredTracer returns the tracer configured from environment
// This is the recommended way to get a tracer in production code
func GetConfiguredTracer() *ZipkinTracer {
	configMu.RLock()
	if configuredTracer != nil {
		configMu.RUnlock()
		return configuredTracer
	}
	configMu.RUnlock()

	return InitializeFromConfig()
}

// Shutdown gracefully shuts down the tracing system
func Shutdown() error {
	configMu.Lock()
	defer configMu.Unlock()

	if configuredTracer == nil {
		return nil
	}

	config := GetConfig()

	// Save traces if configured
	if config.SaveOnExit && config.SavePath != "" {
		// Build the full path with filename
		timestamp := time.Now().Format("20060102_150405")
		filename := fmt.Sprintf("%s/trace_%s", config.SavePath, timestamp)

		if savedFile, err := configuredTracer.SaveSpansLog(filename); err != nil {
			log.Error("tracing: Failed to save traces", "error", err)
		} else {
			log.Info("tracing: Traces saved", "file", savedFile)
		}
	}

	// Flush and disable server push based on tracer's actual state
	if configuredTracer.HasServerPush() {
		configuredTracer.Flush()
		configuredTracer.DisableServerPush()
		log.Info("tracing: Tracing server push disabled")
	}

	configuredTracer = nil
	return nil
}

// MustInitialize initializes tracing and panics on error
// This should be called early in the application lifecycle
func MustInitialize() {
	config := GetConfig()
	if !config.Enabled {
		log.Info("tracing: Tracing is disabled")
		return
	}

	tracer := InitializeFromConfig()
	if tracer == nil {
		log.Error("tracing: Failed to initialize tracer")
	}

	log.Info("tracing: Tracing initialized successfully", "config", config)
}

// IsEnabled returns whether tracing is enabled
func IsEnabled() bool {
	return GetConfig().Enabled
}

// testResetConfig resets the configuration for testing purposes
// This should only be used in tests
func testResetConfig(enabled bool) {
	configMu.Lock()
	defer configMu.Unlock()

	// Properly shutdown existing tracer if any
	if configuredTracer != nil {
		if configuredTracer.HasServerPush() {
			configuredTracer.DisableServerPush()
		}
		configuredTracer = nil
	}

	// Reset configOnce to allow GetConfig to reload from environment
	configOnce = sync.Once{}

	// Set a fully populated config
	globalConfig = &Config{
		Enabled:          enabled,
		ServerURL:        "",
		BatchSize:        100,
		PushInterval:     5 * time.Second,
		ServiceName:      "TestService",
		SaveOnExit:       false,
		SavePath:         "./test_traces",
		MaxSpans:         10000,
		CleanupThreshold: 0.9,
	}

	// Mark configOnce as already done by running it with a no-op
	// This prevents GetConfig from overwriting our test config
	configOnce.Do(func() {
		// Intentionally empty: we're using sync.Once to mark initialization as complete
		// without actually performing any initialization, preserving the test config set above
	})
}

// testResetConfigForEnv resets the configuration to allow reloading from environment
// This should only be used in tests
func testResetConfigForEnv() {
	configMu.Lock()
	defer configMu.Unlock()

	// Properly shutdown existing tracer if any
	if configuredTracer != nil {
		if configuredTracer.HasServerPush() {
			configuredTracer.DisableServerPush()
		}
		configuredTracer = nil
	}

	// Reset configOnce and globalConfig to force reload from environment
	configOnce = sync.Once{}
	globalConfig = nil

	// Reset the singleton tracer as well
	ResetZipkinTracer()
}

// generateUniqueServiceName generates a unique service name based on hostname and process ID
func generateUniqueServiceName() string {
	serviceName := "KleverGo"

	// Try to add hostname
	if hostname, err := os.Hostname(); err == nil {
		serviceName = fmt.Sprintf("%s-%s", serviceName, hostname)
	}

	// Add process ID to ensure uniqueness
	pid := os.Getpid()
	serviceName = fmt.Sprintf("%s-%d", serviceName, pid)

	// Also check for NODE_NAME environment variable (common in k8s)
	if nodeName := os.Getenv("NODE_NAME"); nodeName != "" {
		serviceName = fmt.Sprintf("KleverGo-%s-%d", nodeName, pid)
	}

	// Check for INSTANCE_ID environment variable
	if instanceID := os.Getenv("INSTANCE_ID"); instanceID != "" {
		serviceName = fmt.Sprintf("KleverGo-%s", instanceID)
	}

	return serviceName
}

// UpdateServiceName updates the service name for future spans
func UpdateServiceName(name string) {
	configMu.Lock()
	config := GetConfig()
	config.ServiceName = name
	configMu.Unlock()

	log.Info("tracing: Service name updated for future spans", "name", name)
}

// GetServiceName safely returns the configured service name
func GetServiceName() string {
	configMu.RLock()
	defer configMu.RUnlock()

	config := GetConfig()
	if config != nil && config.ServiceName != "" {
		return config.ServiceName
	}
	return "KleverGo"
}
