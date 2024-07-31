package gasschedules

import _ "embed"

//go:embed gasScheduleV1.toml
var gasScheduleV1 string

// GetV1 yields the schedule V1
func GetV1() string {
	return gasScheduleV1
}
