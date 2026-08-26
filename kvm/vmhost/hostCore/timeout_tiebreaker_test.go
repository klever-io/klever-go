package hostCore

import (
	"context"
	"testing"
	"time"

	contextmock "github.com/klever-io/klever-go/kvm/mock/context"
	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/stretchr/testify/require"
)

func TestVmHost_WaitExecutionWithDeterministicCompletion_DoneOnly(t *testing.T) {
	t.Parallel()

	runtimeCtx := &contextmock.RuntimeContextMock{}
	host := &vmHost{runtimeContext: runtimeCtx}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hookCtx, cancelHook := context.WithCancel(context.Background())
	done := make(chan struct{})
	close(done)

	err := host.waitExecutionWithDeterministicCompletion(ctx, cancelHook, done)
	require.NoError(t, err)
	require.Nil(t, runtimeCtx.FailExecutionErr)

	select {
	case <-hookCtx.Done():
		t.Fatalf("hook context should not be canceled on normal completion")
	default:
	}
}

func TestVmHost_WaitExecutionWithDeterministicCompletion_TimeoutWhileExecutionInFlight(t *testing.T) {
	t.Parallel()

	runtimeCtx := &contextmock.RuntimeContextMock{}
	host := &vmHost{runtimeContext: runtimeCtx}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	hookCtx, cancelHook := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		close(done)
	}()

	err := host.waitExecutionWithDeterministicCompletion(ctx, cancelHook, done)
	require.Equal(t, vmhost.ErrExecutionFailedWithTimeout, err)
	require.Equal(t, vmhost.ErrExecutionFailedWithTimeout, runtimeCtx.FailExecutionErr)

	select {
	case <-done:
	default:
		t.Fatalf("timeout path should wait for execution to finish before returning")
	}

	select {
	case <-hookCtx.Done():
	default:
		t.Fatalf("hook context should be canceled on timeout")
	}
}

func TestVmHost_WaitExecutionWithDeterministicCompletion_TieBreakerCompletionWins(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		runtimeCtx := &contextmock.RuntimeContextMock{}
		host := &vmHost{runtimeContext: runtimeCtx}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		hookCtx, cancelHook := context.WithCancel(context.Background())
		done := make(chan struct{})
		close(done)

		err := host.waitExecutionWithDeterministicCompletion(ctx, cancelHook, done)
		require.NoError(t, err)
		require.Nil(t, runtimeCtx.FailExecutionErr)

		select {
		case <-hookCtx.Done():
			t.Fatalf("hook context should not be canceled when completion wins the tie")
		default:
		}
	}
}
