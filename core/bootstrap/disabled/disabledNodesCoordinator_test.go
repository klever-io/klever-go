package disabled

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNodesCoordinator_IsReady(t *testing.T) {
	t.Parallel()

	coordinator := NewNodesCoordinator()

	// the disabled coordinator is always ready: it never restores state
	require.True(t, coordinator.IsReady())
}
