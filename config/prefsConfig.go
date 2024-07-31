package config

// PreferencesConfig will hold the fields which are node specific such as the display name
type PreferencesConfig struct {
	NodeDisplayName          string `yaml:"nodeDisplayName"`
	Identity                 string `yaml:"identity"`
	RedundancyLevel          int64  `yaml:"redundancyLevel"`
	StatusPollingIntervalSec int64  `yaml:"statusPollingIntervalSec"`
	MaxComputableSlots       uint64 `yaml:"maxComputableSlots"`
}
