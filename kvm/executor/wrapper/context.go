package executorwrapper

import (
	"context"
	"sync/atomic"
)

// Global execution context shared across all hooks in a contract execution
// We use a pointer to allow atomic operations and consistent typing
var globalExecutionContext atomic.Pointer[context.Context]

// SetExecutionContext sets the shared context for the current contract execution
// This should be called once at the start of contract execution in host.go
func SetExecutionContext(ctx context.Context) {
	globalExecutionContext.Store(&ctx)
}

// ClearExecutionContext clears the shared context after contract execution
func ClearExecutionContext() {
	globalExecutionContext.Store(nil)
}

// getExecutionContext retrieves the shared execution context
func getExecutionContext() context.Context {
	ctxPtr := globalExecutionContext.Load()
	if ctxPtr != nil {
		return *ctxPtr
	}
	return nil
}
