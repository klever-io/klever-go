package networksharding

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNodesCoordinatorStub_IsReady(t *testing.T) {
	t.Parallel()

	stub := &NodesCoordinatorStub{}

	// the stub is always ready: it never restores state
	require.True(t, stub.IsReady())
}
