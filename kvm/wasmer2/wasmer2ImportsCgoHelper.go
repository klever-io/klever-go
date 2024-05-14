package wasmer2

import (
	"unsafe"

	"github.com/klever-io/klever-go/kvm/executor"
)

func getVMHooksFromContextRawPtr(contextRawPtr unsafe.Pointer) executor.VMHooks {
	vmHooksPtrPtr := (*uintptr)(contextRawPtr)
	vmHooksPtr := *vmHooksPtrPtr
	// #nosec G103: unsafe.Pointer is used to convert to VMHooks pointer
	return *(*executor.VMHooks)(unsafe.Pointer(vmHooksPtr))
}

func funcPointer(cFuncPtr unsafe.Pointer) *[0]byte {
	return (*[0]byte)(cFuncPtr)
}
