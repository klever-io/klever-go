package facade

import (
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
)

// GetStringValue exports the private getStringValue helper for testing
func GetStringValue(value interface{}) string {
	return getStringValue(value)
}

// GetInt64Value exports the private getInt64Value helper for testing
func GetInt64Value(value interface{}) int64 {
	return getInt64Value(value)
}

// ComputeEndpointsNumGoRoutinesThrottlers exports the private function for testing
func ComputeEndpointsNumGoRoutinesThrottlers(config config.WebServerAntifloodConfig) map[string]core.Throttler {
	return computeEndpointsNumGoRoutinesThrottlers(config)
}
