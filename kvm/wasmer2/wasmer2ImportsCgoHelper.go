package wasmer2

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/klever-io/klever-go/kvm/executor"
)

type vmHooksRegistry struct {
	mutex   sync.Mutex
	entries atomic.Pointer[[]executor.VMHooks]
}

var registeredVMHooks vmHooksRegistry

func (registry *vmHooksRegistry) add(vmHooks executor.VMHooks) uintptr {
	registry.mutex.Lock()
	defer registry.mutex.Unlock()

	var current []executor.VMHooks
	loaded := registry.entries.Load()
	if loaded != nil {
		current = *loaded
	}

	next := make([]executor.VMHooks, len(current)+1)
	copy(next, current)
	next[len(current)] = vmHooks
	registry.entries.Store(&next)

	return uintptr(len(current))
}

func (registry *vmHooksRegistry) get(index uintptr) executor.VMHooks {
	return (*registry.entries.Load())[index]
}

func getVMHooksFromContextRawPtr(contextRawPtr unsafe.Pointer) executor.VMHooks {
	return registeredVMHooks.get(*(*uintptr)(contextRawPtr))
}

func funcPointer(cFuncPtr unsafe.Pointer) *[0]byte {
	return (*[0]byte)(cFuncPtr)
}
