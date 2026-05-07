package config

// PreferencesConfig will hold the fields which are node specific such as the display name
type PreferencesConfig struct {
	NodeDisplayName          string `yaml:"nodeDisplayName"`
	Identity                 string `yaml:"identity"`
	RedundancyLevel          int64  `yaml:"redundancyLevel"`
	StatusPollingIntervalSec int64  `yaml:"statusPollingIntervalSec"`
	MaxComputableSlots       uint64 `yaml:"maxComputableSlots"`
	// EnforceCPUPreflight controls whether the validator refuses to start
	// when the startup SHA-256 throughput bench falls below the leader-mode
	// floor. When true (default — including when the YAML key is absent so
	// existing operator configs upgrade safely), failure aborts startup
	// with a clear error. When explicitly set to false, the failure is
	// downgraded to a warning so operators can observe the issue without
	// bricking a running node — useful during fleet migration.
	// KLEVER_SKIP_CPU_CHECK=1 bypasses the check entirely.
	//
	// Pointer type so an absent YAML key is distinguishable from an
	// explicit false; use ShouldEnforceCPUPreflight() at call sites.
	EnforceCPUPreflight *bool `yaml:"enforceCpuPreflight"`
}

// ShouldEnforceCPUPreflight returns the effective enforcement decision,
// applying the safe default (true) when the YAML key is absent.
func (p PreferencesConfig) ShouldEnforceCPUPreflight() bool {
	if p.EnforceCPUPreflight == nil {
		return true
	}
	return *p.EnforceCPUPreflight
}
