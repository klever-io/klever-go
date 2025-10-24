package executorwrapper

import (
	"context"
	"testing"
	"time"

	"github.com/klever-io/klever-go/kvm/mock/stub"
	"github.com/klever-io/klever-go/kvm/vmhost"

	"github.com/stretchr/testify/assert"
)

func TestFailAfterTimeout(t *testing.T) {
	t.Run("Fast hook with nil wrapper - no timeout protection", func(t *testing.T) {
		result := FailAfterTimeout(func() int {
			time.Sleep(5 * time.Millisecond)
			return 42
		}, HookCategoryFast, nil)
		assert.Equal(t, 42, result)
	})

	t.Run("Slow hook with nil wrapper - no timeout protection", func(t *testing.T) {
		result := FailAfterTimeout(func() int {
			time.Sleep(5 * time.Millisecond)
			return 99
		}, HookCategorySlow, nil)
		assert.Equal(t, 99, result)
	})

	t.Run("Function panic is propagated", func(t *testing.T) {
		defer func() {
			r := recover()
			assert.NotNil(t, r)
			assert.Equal(t, vmhost.ErrArgIndexOutOfRange, r)
		}()

		FailAfterTimeout(func() int {
			panic(vmhost.ErrArgIndexOutOfRange)
		}, HookCategorySlow, nil)
	})
}

func TestFailAfterTimeoutWithContext(t *testing.T) {
	t.Run("Slow hook completes before timeout", func(t *testing.T) {
		// Create a mock with valid context
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		mockHost := &vmHostStubWithContext{ctx: ctx}
		wrapper := createWrapperWithHost(mockHost)

		result := FailAfterTimeout(func() int {
			time.Sleep(10 * time.Millisecond)
			return 42
		}, HookCategorySlow, wrapper)

		assert.Equal(t, 42, result)
	})

	t.Run("Slow hook times out", func(t *testing.T) {
		// Create a context with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		time.Sleep(2 * time.Millisecond) // Ensure context is expired

		mockHost := &vmHostStubWithContext{ctx: ctx}
		wrapper := createWrapperWithHost(mockHost)

		defer func() {
			r := recover()
			assert.NotNil(t, r, "Expected timeout panic")
			assert.Equal(t, vmhost.ErrExecutionFailedWithTimeout, r)
		}()

		FailAfterTimeout(func() int {
			time.Sleep(100 * time.Millisecond) // Takes too long
			return 42
		}, HookCategorySlow, wrapper)

		t.Fatal("Should have panicked with timeout error")
	})

	t.Run("Fast hook bypasses timeout goroutine", func(t *testing.T) {
		// Even with expired context, fast hooks complete directly
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(2 * time.Millisecond) // Ensure context is expired

		mockHost := &vmHostStubWithContext{ctx: ctx}
		wrapper := createWrapperWithHost(mockHost)

		// Fast hook should still panic on pre-check
		defer func() {
			r := recover()
			assert.NotNil(t, r, "Expected timeout panic on pre-check")
			assert.Equal(t, vmhost.ErrExecutionFailedWithTimeout, r)
		}()

		FailAfterTimeout(func() int {
			return 42
		}, HookCategoryFast, wrapper)

		t.Fatal("Should have panicked on context pre-check")
	})

	t.Run("Context checked before execution", func(t *testing.T) {
		// Context already expired before call
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(2 * time.Millisecond)

		mockHost := &vmHostStubWithContext{ctx: ctx}
		wrapper := createWrapperWithHost(mockHost)

		defer func() {
			r := recover()
			assert.NotNil(t, r, "Expected immediate timeout panic")
			assert.Equal(t, vmhost.ErrExecutionFailedWithTimeout, r)
		}()

		FailAfterTimeout(func() int {
			t.Fatal("Should not execute function when context already expired")
			return 42
		}, HookCategorySlow, wrapper)

		t.Fatal("Should have panicked immediately")
	})
}

// Test helpers

type vmHostStubWithContext struct {
	vmhost.VMHost
	ctx context.Context
}

func (h *vmHostStubWithContext) GetExecutionContext() context.Context {
	return h.ctx
}

func (h *vmHostStubWithContext) IsInterfaceNil() bool {
	return h == nil
}

type vmHooksImplStub struct {
	stub.VMHooksStub
	host vmhost.VMHost
}

func (v *vmHooksImplStub) GetVMHost() vmhost.VMHost {
	return v.host
}

func createWrapperWithHost(host vmhost.VMHost) *WrapperVMHooks {
	vmHooksStub := &vmHooksImplStub{host: host}
	return NewWrapperVMHooks(vmHooksStub, &NoLogger{})
}

func TestContextCleanup(t *testing.T) {
	// Test that nil wrapper doesn't cause issues
	result := FailAfterTimeout(func() int {
		return 123
	}, HookCategoryFast, nil)
	assert.Equal(t, 123, result)
}

func TestNilContextBehavior(t *testing.T) {
	// Test with nil wrapper
	t.Run("Fast hook should work without wrapper", func(t *testing.T) {
		result := FailAfterTimeout(func() int {
			time.Sleep(5 * time.Millisecond)
			return 42
		}, HookCategoryFast, nil)
		assert.Equal(t, 42, result, "Fast hook should complete without wrapper")
	})

	t.Run("Slow hook should work without wrapper (no timeout)", func(t *testing.T) {
		result := FailAfterTimeout(func() int {
			time.Sleep(5 * time.Millisecond)
			return 99
		}, HookCategorySlow, nil)
		assert.Equal(t, 99, result, "Slow hook should complete without wrapper (fallback behavior)")
	})
}
