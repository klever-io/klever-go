package prewarmer

import (
	"github.com/klever-io/klever-go/kvm/executor"
)

// CodeFetcher returns the codeHash and bytecode for a smart-contract address.
// Implementations wrap a BlockChainHook (or test stub) to keep the prewarmer
// independent of the heavy UserAccountHandler interface.
//
// FetchCode returns nil codeHash when the address has no contract code. The
// prewarmer treats that case as a no-op rather than an error.
type CodeFetcher interface {
	FetchCode(address []byte) (codeHash []byte, code []byte, err error)
}

// CompiledCodeStore is the cache the prewarmer fills. Production wires this
// to BlockChainHookImpl's GetCompiledCode/SaveCompiledCode pair, which writes
// through the in-memory compiledScPool and the on-disk compiledScStorage.
type CompiledCodeStore interface {
	HasCompiledCode(codeHash []byte) bool
	SaveCompiledCode(codeHash []byte, code []byte)
}

// Compiler compiles raw WASM bytecode and returns AOT bytes ready to be
// stored in the compiled-code cache. The implementation is expected to call
// executor.NewInstanceWithOptions, capture the instance's Cache() output,
// and Clean() the instance.
type Compiler interface {
	CompileToAOT(code []byte, options executor.CompilationOptions) ([]byte, error)
}
