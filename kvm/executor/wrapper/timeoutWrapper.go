package executorwrapper

import (
	"context"

	"github.com/klever-io/klever-go/kvm/vmhost"
)

// FailAfterTimeout executes function f with category-appropriate timeout protection.
//
// OPTIMIZATION STRATEGY:
// Instead of creating goroutines for ALL 100k+ hook calls, we categorize hooks:
// - Fast hooks (99%): Direct execution with context check (~5ns overhead)
// - Slow hooks (1%): Goroutine protection for interruptibility (~500ns overhead)
//
// CONTEXT RETRIEVAL:
// The execution context is retrieved from the VMHost instance through the wrapper.
// Each RunSmartContractCall() sets its own context on the host, ensuring:
// - Thread safety: No race conditions between parallel queries
// - Call isolation: Nested calls don't overwrite parent contexts
//
// SAFETY:
// - Fast hooks: Protected by contract-level timeout + context checking
// - Slow hooks: Full goroutine-based interruption capability
// - Contract timeout in host.go provides defense-in-depth
func FailAfterTimeout[K any](f func() K, category HookCategory, vmHooks *WrapperVMHooks) K {
	// Get execution context from the VMHost (per-execution, not global)
	// Explicitly check for nil to avoid calling method on nil pointer
	var ctx context.Context
	if vmHooks != nil {
		ctx = vmHooks.getExecutionContext()
	}

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
	// - Simple operations (GetGasLeft, GetBlockTimestamp)
	// - Gas-bounded operations (crypto, storage, most BigInt)
	// - Cannot panic: errors handled by context.WithFault
	if category == HookCategoryFast {
		return f()
	}

	// Slow path: For hooks that can take significant time
	// - Contract execution (ExecuteOnDestContext, CreateContract, etc.) + BigIntPow/BigFloatPow
	// - Must be interruptible to avoid DoS attacks
	// - We run the hook in a goroutine and wait for either completion or timeout
	// - If the hook panics, we propagate the panic to the caller
	// - If the hook times out, we panic with ErrExecutionFailedWithTimeout and
	//   abandon the goroutine: it is NOT cancelled, it keeps running until f()
	//   returns and its result is discarded. Slow hooks must therefore bound
	//   their own work up front (e.g. the pow hooks charge gas before looping)
	//   so an abandoned goroutine finishes shortly after the timeout instead of
	//   mutating shared VM state indefinitely.
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
	// WARNING: If ctx is nil, we'll wait for completion without timeout protection.
	// This fallback is intended ONLY for tests.
	// In production, the execution context is set automatically by RunSmartContractCall in host.go.
	if ctx != nil {
		select {
		case rp := <-done:
			if rp.panic != nil {
				panic(rp.panic)
			}
			return rp.result
		case <-ctx.Done():
			// Contract timeout triggered. The timeout path in host.go has already set the
			// execution breakpoint (FailExecution) before cancelling this hook context, so the
			// in-flight worker will stop at the next breakpoint check and send on done.
			//
			// We must join the worker here before unwinding: this panic propagates up to the
			// recover in RunSmartContractCall, which calls CleanInstance and destroys the native
			// wasmer instance. Panicking while the worker is still inside cWasmerInstanceCall on
			// that same instance is the use-after-free this guards against.
			rp := <-done
			if rp.panic != nil {
				panic(rp.panic)
			}
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
