package executorwrapper

import (
	"github.com/klever-io/klever-go/kvm/vmhost"
)

// FailAfterTimeout executes function f with category-appropriate timeout protection.
//
// OPTIMIZATION STRATEGY:
// Instead of creating goroutines for ALL 100k+ hook calls, we categorize hooks:
// - Fast hooks (99%): Direct execution with context check (~5ns overhead)
// - Slow hooks (1%): Goroutine protection for interruptibility (~500ns overhead)
//
// CONTEXT SHARING:
// The execution context is set once per contract in RunSmartContractCall() and
// reused across all hook calls. This eliminates creating 100k separate timeouts.
//
// SAFETY:
// - Fast hooks: Protected by contract-level timeout + context checking
// - Slow hooks: Full goroutine-based interruption capability
// - Contract timeout in host.go provides defense-in-depth
func FailAfterTimeout[K any](f func() K, category HookCategory) K {
	// Get shared execution context
	ctx := getExecutionContext()

	// Quick pre-check: Has contract timeout already expired?
	if ctx != nil {
		select {
		case <-ctx.Done():
			panic(vmhost.ErrExecutionFailedWithTimeout)
		default:
			// Context still valid, continue
		}
	}

	// Fast path: For hooks that are known to be fast (<1µs)
	// These hooks are either:
	// - Simple operations (GetGasLeft, GetBlockTimestamp)
	// - Gas-bounded operations (crypto, storage, most BigInt)
	// - Have internal timeout checking (BigIntPow, BigFloatPow)
	if category == HookCategoryFast {
		return f()
	}

	// Slow path: For hooks that can take significant time
	// - Contract execution (ExecuteOnDestContext, CreateContract, etc.)
	// These MUST be interruptible via goroutines
	type resultOrPanic struct {
		result K
		panic  any
	}

	done := make(chan resultOrPanic, 1)

	go func() {
		var rp resultOrPanic
		defer func() {
			if r := recover(); r != nil {
				rp.panic = r
			}
			done <- rp
		}()
		rp.result = f()
	}()

	// Wait for either completion or timeout
	// If ctx is nil, we'll wait forever (fallback for tests)
	if ctx != nil {
		select {
		case rp := <-done:
			if rp.panic != nil {
				panic(rp.panic)
			}
			return rp.result
		case <-ctx.Done():
			// Contract timeout triggered - interrupt the hook
			panic(vmhost.ErrExecutionFailedWithTimeout)
		}
	} else {
		// No context available - wait for completion without timeout
		rp := <-done
		if rp.panic != nil {
			panic(rp.panic)
		}
		return rp.result
	}
}
