// Package executor contains the interfaces and definitions for the VM Executor
package executor

import (
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/vmcommon"
)

// CompilationOptions contains configurations for instantiating an executor instance.
//
// This struct crosses the cgo boundary as a raw pointer cast (see
// wasmer2Executor.go), not through per-field marshaling, so its field order
// must exactly match the Rust CompilationOptions struct (#[repr(C)], in
// vm-executor/src/instance.rs) - do not reorder without updating both sides.
type CompilationOptions struct {
	GasLimit           uint64
	UnmeteredLocals    uint64
	MaxMemoryGrow      uint64
	MaxMemoryGrowDelta uint64
	// MaxDeclaredTableSize rejects the module if any declared WASM table's
	// maximum exceeds it (see KLC-2526 / KLR-19); math.MaxUint64 disables the
	// check entirely. Unlike this struct's other numeric fields, the Go zero
	// value (0) is not a harmless default here - it rejects any table
	// declaration at all, not merely fails to cap one. Every construction
	// site must set this field explicitly.
	MaxDeclaredTableSize uint64
	OpcodeTrace          bool
	Metering             bool
	RuntimeBreakpoints   bool
}

// Executor defines the functionality needed to create any executor instance.
type Executor interface {
	check.NilInterfaceChecker

	// SetOpcodeCosts sets gas costs globally inside an executor.
	SetOpcodeCosts(opcodeCosts *WASMOpcodeCost)

	// FunctionNames return the low-level function names provided to contracts.
	FunctionNames() vmcommon.FunctionNames

	// NewInstanceWithOptions creates a new executor instance.
	NewInstanceWithOptions(
		contractCode []byte,
		options CompilationOptions) (Instance, error)

	// NewInstanceFromCompiledCodeWithOptions is used to restore an executor instance from cache.
	NewInstanceFromCompiledCodeWithOptions(
		compiledCode []byte,
		options CompilationOptions) (Instance, error)
}
