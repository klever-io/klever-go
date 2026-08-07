package disabled

import (
	"testing"

	"github.com/klever-io/klever-go/core/kapp"
	"github.com/stretchr/testify/require"
)

// The bootstrap stub must satisfy the full KAppController interface: IsReadOnly was
// added to it, and a missing implementation would only surface at the assignment
// sites deep in the bootstrap wiring.
func TestDisabledKAppsController_ImplementsKAppController(t *testing.T) {
	t.Parallel()

	var controller kapp.KAppController = NewKAppsController()
	require.False(t, controller.IsInterfaceNil())
}

// Bootstrap is not a query context. Reporting read-only here would make the guards
// in BlockChainHookImpl.ProcessBuiltInFunction and accountsKapp.checkReadOnly refuse
// operations during bootstrap rather than let the stub's nil KApps be reached.
func TestDisabledKAppsController_IsReadOnly_ReportsWritable(t *testing.T) {
	t.Parallel()

	require.False(t, NewKAppsController().IsReadOnly())
}
