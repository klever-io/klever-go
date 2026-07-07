package vmhooks

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestExecutionTimedOut pins the decision logic of the timeout guard used by the
// nested contract-execution slow hooks (Managed{Execute,Create,Deploy,Upgrade}*) to
// skip their late write into the shared managed-types maps when the parent execution
// has already timed out. The guard must:
//   - treat a nil context (direct-call / test path) as NOT timed out, never panic;
//   - report false while the context is still live;
//   - report true once the context is cancelled / its deadline has passed.
func TestExecutionTimedOut(t *testing.T) {
	t.Parallel()

	t.Run("nil context is not timed out", func(t *testing.T) {
		t.Parallel()
		require.False(t, executionTimedOut(nil))
	})

	t.Run("live context is not timed out", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		require.False(t, executionTimedOut(ctx))
	})

	t.Run("cancelled context is timed out", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.True(t, executionTimedOut(ctx))
	})

	t.Run("expired deadline is timed out", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()
		// Ensure the deadline has elapsed before checking.
		<-ctx.Done()
		require.True(t, executionTimedOut(ctx))
	})

	t.Run("transition from live to cancelled", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		require.False(t, executionTimedOut(ctx))
		cancel()
		require.True(t, executionTimedOut(ctx))
	})
}
