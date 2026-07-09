package hostCore

import (
	"context"
	"testing"

	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/stretchr/testify/require"
)

func TestVmHost_WaitExecutionWithDeterministicTimeout_DoneOnly(t *testing.T) {
	t.Parallel()

	runtimeCtx := &contextmock.RuntimeContextMock{}
	host := &vmHost{runtimeContext: runtimeCtx}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hookCtx, cancelHook := context.WithCancel(context.Background())
	done := make(chan struct{})
	close(done)

	err := host.waitExecutionWithDeterministicTimeout(ctx, cancelHook, done)
	require.NoError(t, err)
	require.Nil(t, runtimeCtx.FailExecutionErr)

	select {
	case <-hookCtx.Done():
		t.Fatalf("hook context should not be canceled on normal completion")
	default:
	}
}

func TestVmHost_WaitExecutionWithDeterministicTimeout_TimeoutOnly(t *testing.T) {
	t.Parallel()

	runtimeCtx := &contextmock.RuntimeContextMock{}
	host := &vmHost{runtimeContext: runtimeCtx}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	hookCtx, cancelHook := context.WithCancel(context.Background())
	done := make(chan struct{})
	close(done)

	err := host.waitExecutionWithDeterministicTimeout(ctx, cancelHook, done)
	require.Equal(t, vmhost.ErrExecutionFailedWithTimeout, err)
	require.Equal(t, vmhost.ErrExecutionFailedWithTimeout, runtimeCtx.FailExecutionErr)

	select {
	case <-hookCtx.Done():
		// expected: timeout path cancels hook context
	default:
		t.Fatalf("hook context should be canceled on timeout")
	}
}

func TestVmHost_WaitExecutionWithDeterministicTimeout_TieBreakerTimeoutWins(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		runtimeCtx := &contextmock.RuntimeContextMock{}
		host := &vmHost{runtimeContext: runtimeCtx}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		hookCtx, cancelHook := context.WithCancel(context.Background())
		done := make(chan struct{})
		close(done)

		err := host.waitExecutionWithDeterministicTimeout(ctx, cancelHook, done)
		require.Equal(t, vmhost.ErrExecutionFailedWithTimeout, err)
		require.Equal(t, vmhost.ErrExecutionFailedWithTimeout, runtimeCtx.FailExecutionErr)

		select {
		case <-hookCtx.Done():
			// expected in timeout-wins tie-breaker
		default:
			t.Fatalf("hook context should be canceled on timeout tie-breaker")
		}
	}
}
