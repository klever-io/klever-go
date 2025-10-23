package executorwrapper

import (
	"context"
	"testing"
	"time"

	"github.com/klever-io/klever-go/kvm/vmhost"

	"github.com/stretchr/testify/assert"
)

func TestFailAfterTimeout(t *testing.T) {
	tests := []struct {
		// prepare
		name          string
		returns       any
		waits         time.Duration
		timeout       time.Duration
		functionPanic error
		category      HookCategory

		// assert
		expectedReturn any
		shouldReturn   bool
		shouldPanic    bool
	}{
		{
			name:     "Should preserve the return value (fast hook)",
			returns:  1,
			waits:    1 * time.Millisecond,
			timeout:  10 * time.Millisecond,
			category: HookCategoryFast,

			expectedReturn: 1,
		},
		{
			name:     "Should preserve the return value (slow hook)",
			returns:  1,
			waits:    1 * time.Millisecond,
			timeout:  10 * time.Millisecond,
			category: HookCategorySlow,

			expectedReturn: 1,
		},
		{
			name:     "Should throw a panic when the timeout is reached",
			returns:  123,
			waits:    20 * time.Millisecond,
			timeout:  10 * time.Millisecond,
			category: HookCategorySlow,

			shouldPanic: true,
		},
		{
			name:          "Should throw a panic if the original function panics",
			returns:       123,
			waits:         1 * time.Millisecond,
			timeout:       10 * time.Millisecond,
			functionPanic: vmhost.ErrArgIndexOutOfRange,
			category:      HookCategorySlow,

			shouldPanic: true,
		},
		{
			name:     "Fast hook should not timeout even if slow",
			returns:  42,
			waits:    20 * time.Millisecond,
			timeout:  10 * time.Millisecond,
			category: HookCategoryFast,

			expectedReturn: 42, // Should complete despite timeout
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up execution context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()
			SetExecutionContext(ctx)
			defer ClearExecutionContext()

			hadPanic := false
			var result any = nil
			done := make(chan bool, 1)
			defer close(done)
			// we need to run this in a go routine to handle the panic per test
			go func() {
				defer func() {
					if r := recover(); r != nil {
						hadPanic = true
					}
					done <- true
				}()
				result = FailAfterTimeout(func() any {
					if tt.functionPanic != nil {
						panic(tt.functionPanic)
					}
					time.Sleep(tt.waits)
					return tt.returns
				}, tt.category)
			}()
			select {
			case <-done:
			case <-time.After(1 * time.Second):
				t.Fatal("test exceeded the time limit")
			}

			assert.Equal(t, tt.shouldPanic, hadPanic)
			assert.Equal(t, tt.expectedReturn, result)
		})
	}
}

func TestContextCleanup(t *testing.T) {
	// Set a context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	SetExecutionContext(ctx)

	// Verify context is set (indirect check via execution)
	result := FailAfterTimeout(func() int {
		return 123
	}, HookCategoryFast)
	assert.Equal(t, 123, result)

	// Clear the context
	ClearExecutionContext()

	// Verify context is cleared - getExecutionContext should return nil
	clearedCtx := getExecutionContext()
	assert.Nil(t, clearedCtx, "Context should be nil after ClearExecutionContext()")
}

func TestNilContextBehavior(t *testing.T) {
	// Ensure no context is set
	ClearExecutionContext()

	t.Run("Fast hook should work without context", func(t *testing.T) {
		result := FailAfterTimeout(func() int {
			time.Sleep(5 * time.Millisecond)
			return 42
		}, HookCategoryFast)
		assert.Equal(t, 42, result, "Fast hook should complete without context")
	})

	t.Run("Slow hook should work without context (no timeout)", func(t *testing.T) {
		result := FailAfterTimeout(func() int {
			time.Sleep(5 * time.Millisecond)
			return 99
		}, HookCategorySlow)
		assert.Equal(t, 99, result, "Slow hook should complete without context (fallback behavior)")
	})
}
