package wasmer2

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/klever-io/klever-go/kvm/executor"
)

var _ = (executor.ExecutorAbstractFactory)((*Wasmer2ExecutorFactory)(nil))

// Wasmer2ExecutorFactory builds Wasmer2 Executors.
type Wasmer2ExecutorFactory struct{}

// ExecutorFactory returns the Wasmer executor factory.
func ExecutorFactory() *Wasmer2ExecutorFactory {
	return &Wasmer2ExecutorFactory{}
}

var sigHandler = []os.Signal{
	// Critical signals for memory safety
	syscall.SIGSEGV, // Segmentation violation - memory access errors
	syscall.SIGILL,  // Illegal instruction
	syscall.SIGBUS,  // Bus error - alignment issues

	// Arithmetic-related signals
	syscall.SIGFPE, // Floating point exception

	// Timing and scheduling signals
	syscall.SIGALRM, // Alarm clock - used for timeouts
	syscall.SIGTRAP, // Trap - used for debugging

	// Additional signals that might interfere with VM execution
	syscall.SIGABRT, // Abort - might be used internally by Wasmer
	syscall.SIGPIPE, // Broken pipe - network operations

	// Rarely needed but potentially relevant
	syscall.SIGSYS, // Bad system call

	// Resource limitation signals
	syscall.SIGXCPU, // CPU time limit exceeded
	syscall.SIGXFSZ, // File size limit exceeded
}

// CreateExecutor creates a new Executor instance.
func (wef *Wasmer2ExecutorFactory) CreateExecutor(args executor.ExecutorFactoryArgs) (executor.Executor, error) {
	// reset all signals handlers but SIGINT/SIGTERM
	signal.Reset(sigHandler...)

	executor, err := CreateExecutor()
	if err != nil {
		return nil, err
	}
	executor.initVMHooks(args.VMHooks)
	if args.OpcodeCosts != nil {
		// opcode costs are sometimes not initialized at this point in certain tests
		executor.SetOpcodeCosts(args.OpcodeCosts)
	}

	return executor, nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (wef *Wasmer2ExecutorFactory) IsInterfaceNil() bool {
	return wef == nil
}
