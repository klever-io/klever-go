package prewarmer

import (
	"github.com/klever-io/klever-go/kvm/executor"
)

// executorCompiler is a thin Compiler that drives the Wasmer executor's
// NewInstanceWithOptions + instance.Cache + instance.Clean sequence. The
// instance is discarded immediately; only the AOT bytes produced by Cache
// are returned.
type executorCompiler struct {
	exec executor.Executor
}

// NewExecutorCompiler wraps an executor.Executor for use by the prewarmer.
func NewExecutorCompiler(exec executor.Executor) Compiler {
	return &executorCompiler{exec: exec}
}

// CompileToAOT performs a Wasmer JIT compile of the given bytecode and
// returns the AOT serialization. The instance produced by the compile is
// cleaned before returning, so this method is suitable for background use
// without holding native-code memory.
func (c *executorCompiler) CompileToAOT(code []byte, options executor.CompilationOptions) ([]byte, error) {
	inst, err := c.exec.NewInstanceWithOptions(code, options)
	if err != nil {
		return nil, err
	}
	defer inst.Clean()
	return inst.Cache()
}
