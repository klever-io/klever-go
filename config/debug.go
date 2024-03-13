package config

// InterceptorResolverDebugConfig will hold the interceptor-resolver debug configuration
type InterceptorResolverDebugConfig struct {
	Enabled                    bool `yaml:"enabled"`
	EnablePrint                bool `yaml:"enablePrint"`
	CacheSize                  int  `yaml:"cacheSize"`
	IntervalAutoPrintInSeconds int  `yaml:"intervalAutoPrintInSeconds"`
	NumRequestsThreshold       int  `yaml:"numRequestsThreshold"`
	NumResolveFailureThreshold int  `yaml:"numResolveFailureThreshold"`
	DebugLineExpiration        int  `yaml:"debugLineExpiration"`
}

// AntifloodDebugConfig will hold the antiflood debug configuration
type AntifloodDebugConfig struct {
	Enabled                    bool `yaml:"enabled"`
	CacheSize                  int  `yaml:"cacheSize"`
	IntervalAutoPrintInSeconds int  `yaml:"intervalAutoPrintInSeconds"`
}

// DebugConfig will hold debugging configuration
type DebugConfig struct {
	InterceptorResolver InterceptorResolverDebugConfig `yaml:"interceptorResolver"`
	Antiflood           AntifloodDebugConfig           `yaml:"antiflood"`
}
