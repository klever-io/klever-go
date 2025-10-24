package executorwrapper

// HookCategory defines the expected execution time for a VM hook.
//
// This is used at compile-time to determine the timeout protection strategy.
// The category is baked into the generated code for each hook, eliminating
// any runtime lookup overhead.
type HookCategory int

const (
	// HookCategoryFast: < 1µs - most hooks (233 out of 242)
	//
	// These hooks are either:
	//   - Instant operations (GetGasLeft, GetBlockTimestamp, etc.)
	//   - Gas-bounded operations (crypto, storage, arithmetic)
	//
	// Fast hooks use direct execution with just a quick context check (~5ns overhead).
	HookCategoryFast HookCategory = iota

	// HookCategorySlow: Can take significant time (9 core hooks + managed variants)
	//
	// These hooks can execute for extended periods:
	//   - Contract execution (ExecuteOnDestContext, CreateContract, etc.)
	//     → Can run arbitrary WASM code
	//   - Exponential math (BigIntPow, BigFloatPow)
	//     → Can iterate many times with large exponents
	//
	// Slow hooks use goroutine-based protection for full interruptibility (~500ns overhead).
	HookCategorySlow
)

// NOTE: The actual categorization logic is defined at code generation time in:
//   kvm/vmhost/vmhooks/generate/eiGenWriteVMHooksWrapper.go::getHookCategoryConstant()
//
// In summary:
//   - HookCategorySlow: ~20 hooks (contract execution + BigIntPow/BigFloatPow)
//   - HookCategoryFast: ~222 hooks (all other operations)
