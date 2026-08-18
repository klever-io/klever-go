package wasmer2

// #include <stdlib.h>
import "C"
import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"unsafe"

	"github.com/klever-io/klever-go/kvm/executor"
)

var _ executor.Instance = (*Wasmer2Instance)(nil)

// Wasmer2Instance represents a WebAssembly instance.
type Wasmer2Instance struct {
	// mutex serializes native destruction (Clean) against every other native
	// accessor (points/breakpoint/reset/memory/function-introspection), so none
	// of them can read cgoInstance while Clean is freeing it. The one exception
	// is CallFunction, which runs the whole WASM module and re-enters the instance
	// through the import hooks on the same goroutine (sync.Mutex is not reentrant);
	// that path is kept safe from Clean by joining the worker before unwinding into
	// CleanInstance (see executorwrapper.FailAfterTimeout), not by this lock.
	mutex sync.Mutex

	cgoInstance *cWasmerInstanceT

	memory Wasmer2Memory

	// id is captured at construction so it survives Clean clearing cgoInstance;
	// the instance tracker uses it as a map key.
	id string

	alreadyClean bool
}

func newInstance(c_instance *cWasmerInstanceT) (*Wasmer2Instance, error) {
	return &Wasmer2Instance{
		cgoInstance: c_instance,
		memory: Wasmer2Memory{
			cgoInstance: c_instance,
		},
		id: fmt.Sprintf("%p", c_instance),
	}, nil
}

// Clean destroys the native instance. It is safe to call concurrently and
// idempotent: the cleaned check, the native destruction and the zeroing of the
// native pointers happen as a single atomic transition under the instance mutex,
// so no other goroutine can observe a destroyed-but-still-referenced instance.
func (instance *Wasmer2Instance) Clean() bool {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()

	logWasmer2.Trace("cleaning instance", "id", instance.ID())
	if instance.alreadyClean || instance.cgoInstance == nil {
		logWasmer2.Trace("clean: already cleaned instance", "id", instance.ID())
		return false
	}

	cWasmerInstanceDestroy(instance.cgoInstance)

	instance.alreadyClean = true
	instance.cgoInstance = nil
	instance.memory.cgoInstance = nil
	logWasmer2.Trace("cleaned instance", "id", instance.ID())

	return true
}

// logCleanedAccess records that a native accessor ran on an already-cleaned
// instance. Reaching any of these guards means a caller retained the instance
// past Clean, which is a lifecycle bug rather than a normal branch: the accessor
// then returns a fabricated zero value with no other signal to the caller.
func (instance *Wasmer2Instance) logCleanedAccess(op string) {
	logWasmer2.Error("native access on cleaned wasmer2 instance", "op", op, "id", instance.ID())
}

// IsAlreadyCleaned returns whether the native instance has already been destroyed.
func (instance *Wasmer2Instance) IsAlreadyCleaned() bool {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	return instance.alreadyClean
}

// SetGasLimit sets the gas limit for the instance
func (instance *Wasmer2Instance) SetGasLimit(gasLimit uint64) {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		instance.logCleanedAccess("SetGasLimit")
		return
	}
	cWasmerInstanceSetGasLimit(instance.cgoInstance, gasLimit)
}

// SetPointsUsed sets the internal instance gas counter
func (instance *Wasmer2Instance) SetPointsUsed(points uint64) {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		instance.logCleanedAccess("SetPointsUsed")
		return
	}
	cWasmerInstanceSetPointsUsed(instance.cgoInstance, points)
}

// GetPointsUsed returns the internal instance gas counter
func (instance *Wasmer2Instance) GetPointsUsed() uint64 {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		instance.logCleanedAccess("GetPointsUsed")
		return 0
	}
	return cWasmerInstanceGetPointsUsed(instance.cgoInstance)
}

// SetBreakpointValue sets the breakpoint value for the instance
func (instance *Wasmer2Instance) SetBreakpointValue(value uint64) {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		instance.logCleanedAccess("SetBreakpointValue")
		return
	}
	cWasmerInstanceSetBreakpointValue(instance.cgoInstance, value)
}

// GetBreakpointValue returns the breakpoint value
func (instance *Wasmer2Instance) GetBreakpointValue() uint64 {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		instance.logCleanedAccess("GetBreakpointValue")
		return 0
	}
	return cWasmerInstanceGetBreakpointValue(instance.cgoInstance)
}

// Cache caches the instance
func (instance *Wasmer2Instance) Cache() ([]byte, error) {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		return nil, ErrInstanceCleaned
	}

	var cacheBytes *cUchar
	var cacheLen cUint32T

	var cacheResult = cWasmerInstanceCache(
		instance.cgoInstance,
		&cacheBytes,
		&cacheLen,
	)

	if cacheResult != cWasmerOk {
		return nil, ErrCachingFailed
	}

	goBytes := C.GoBytes(unsafe.Pointer(cacheBytes), C.int(cacheLen)) // #nosec G103: unsafe.Pointer is used to convert to GoBytes

	C.free(unsafe.Pointer(cacheBytes)) // #nosec G103: free is used to free the memory allocated by C
	cacheBytes = nil
	return goBytes, nil
}

// IsFunctionImported returns true if the instance imports the specified function
func (instance *Wasmer2Instance) IsFunctionImported(name string) bool {
	return false
}

// CallFunction executes given function from loaded contract.
func (instance *Wasmer2Instance) CallFunction(functionName string) error {
	var wasmFunctionName = cCString(functionName)
	defer cFree(unsafe.Pointer(wasmFunctionName)) // #nosec G103: free is used to free the memory allocated by cCString

	var callResult = cWasmerInstanceCall(
		instance.cgoInstance,
		wasmFunctionName,
	)

	if callResult != cWasmerOk {
		err := fmt.Errorf("failed to call the `%s` exported function", functionName)
		return newWrappedError(err)
	}

	return nil
}

// HasFunction checks if loaded contract has a function (endpoint) with given name.
func (instance *Wasmer2Instance) HasFunction(functionName string) bool {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		instance.logCleanedAccess("HasFunction")
		return false
	}

	var wasmFunctionName = cCString(functionName)
	defer cFree(unsafe.Pointer(wasmFunctionName)) // #nosec G103: free is used to free the memory allocated by cCString

	result := cWasmerInstanceHasFunction(
		instance.cgoInstance,
		wasmFunctionName,
	)

	return result == 1
}

// getFunctionNamesConcat returns the exported function names joined with "|".
func (instance *Wasmer2Instance) getFunctionNamesConcat() (string, error) {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		return "", ErrInstanceCleaned
	}

	var bufferLength = cWasmerInstanceExportedFunctionNamesLength(instance.cgoInstance)

	if bufferLength == 0 {
		return "", nil
	}

	var buffer = make([]cChar, bufferLength)
	var bufferPointer = (*cChar)(unsafe.Pointer(&buffer[0])) // #nosec G103: unsafe.Pointer is used to convert to cChar pointer

	var result = cWasmerInstanceExportedFunctionNames(instance.cgoInstance, bufferPointer, bufferLength)

	if result == -1 {
		return "", errors.New("cannot read function names")
	}

	return cGoString(bufferPointer), nil
}

// GetFunctionNames returns a list of the function names exported by the contract.
func (instance *Wasmer2Instance) GetFunctionNames() []string {
	buffer, err := instance.getFunctionNamesConcat()
	if err != nil {
		return nil
	}
	return strings.Split(buffer, "|")
}

// ValidateFunctionArities checks that no function (endpoint) of the given contract has any parameters or returns any result.
// All arguments and results should be transferred via the import functions.
func (instance *Wasmer2Instance) ValidateFunctionArities() error {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		return ErrInstanceCleaned
	}

	var result = cWasmerCheckSignatures(instance.cgoInstance)
	if result != cWasmerOk {
		return executor.ErrFunctionNonvoidSignature
	}
	return nil
}

// HasMemory checks whether the instance has at least one exported memory.
func (instance *Wasmer2Instance) HasMemory() bool {
	return true
}

// MaxDeclaredTableSize returns the largest maximum size declared among all of
// the instance's WASM tables (imported or local, exported or not), as parsed
// by Wasmer itself. Tables with no declared maximum are reported as
// math.MaxUint32. Returns 0 if the instance declares no tables - the same
// value returned below for a cleaned/nil instance, since 0 is the safe
// choice for a caller unable to distinguish "no tables" from "couldn't
// read it": the real enforcement for oversized tables (KLC-2526) runs
// earlier, during instantiation, so an instance reaching this call has
// already passed that check regardless of what this returns.
func (instance *Wasmer2Instance) MaxDeclaredTableSize() uint32 {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		instance.logCleanedAccess("MaxDeclaredTableSize")
		return 0
	}
	return uint32(cWasmerInstanceMaxDeclaredTableSize(instance.cgoInstance))
}

// MemLoad returns the contents from the given offset of the WASM memory.
func (instance *Wasmer2Instance) MemLoad(memPtr executor.MemPtr, length executor.MemLength) ([]byte, error) {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		return nil, ErrInstanceCleaned
	}
	return executor.MemLoadFromMemory(&instance.memory, memPtr, length)
}

// MemStore stores the given data in the WASM memory at the given offset.
func (instance *Wasmer2Instance) MemStore(memPtr executor.MemPtr, data []byte) error {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		return ErrInstanceCleaned
	}
	return executor.MemStoreToMemory(&instance.memory, memPtr, data)
}

// MemLength returns the length of the allocated memory. Only called directly in tests.
func (instance *Wasmer2Instance) MemLength() uint32 {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		instance.logCleanedAccess("MemLength")
		return 0
	}
	return instance.memory.Length()
}

// MemGrow allocates more pages to the current memory. Only called directly in tests.
func (instance *Wasmer2Instance) MemGrow(pages uint32) error {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		return ErrInstanceCleaned
	}
	return instance.memory.Grow(pages)
}

// MemDump yields the entire contents of the memory. Only used in tests.
func (instance *Wasmer2Instance) MemDump() []byte {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		instance.logCleanedAccess("MemDump")
		return nil
	}
	return instance.memory.Data()
}

// ID returns an identifier for the instance, unique at runtime. It is captured at
// construction and immutable thereafter, so it stays stable even after Clean has
// cleared the native pointer (the instance tracker uses it as a map key) and can be
// read without the mutex, including from within other locked methods.
func (instance *Wasmer2Instance) ID() string {
	return instance.id
}

// Reset resets the instance memories and globals
func (instance *Wasmer2Instance) Reset() bool {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.alreadyClean || instance.cgoInstance == nil {
		logWasmer2.Trace("reset: already cleaned instance", "id", instance.ID())
		return false
	}

	result := cWasmerInstanceReset(instance.cgoInstance)
	ok := result == cWasmerOk

	logWasmer2.Trace("reset: warm instance", "id", instance.ID(), "ok", ok)
	return ok
}

// IsInterfaceNil returns true if underlying object is nil
func (instance *Wasmer2Instance) IsInterfaceNil() bool {
	return instance == nil
}

// SetVMHooksPtr sets the VM hooks pointer
func (instance *Wasmer2Instance) SetVMHooksPtr(vmHooksPtr uintptr) {
}

// GetVMHooksPtr returns the VM hooks pointer
func (instance *Wasmer2Instance) GetVMHooksPtr() uintptr {
	return uintptr(0)
}
