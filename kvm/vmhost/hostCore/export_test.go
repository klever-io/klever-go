package hostCore

import (
	"time"

	"github.com/klever-io/klever-go/kvm/vmhost"
)

// GetEffectiveTimeoutForTest exposes the private getEffectiveTimeout so external tests can assert
// the duration selected for each execution mode.
func GetEffectiveTimeoutForTest(host vmhost.VMHost) time.Duration {
	return host.(*vmHost).getEffectiveTimeout()
}
