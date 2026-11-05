package wasmer2

import (
	"fmt"
	"unsafe"
)

// Wasmer2Memory is the linear memory of a WebAssembly instance. The native
// instance owns it and Clean releases it. The whole memory is never handed out
// as a []byte. Load and store build a view over the requested range, copy, and
// drop it, under the instance mutex.
type Wasmer2Memory struct {
	// The underlying WebAssembly instance.
	cgoInstance *cWasmerInstanceT
}

// dataLength reads the memory length in bytes once. The C API returns
// uint64_t and the bridge narrows it to uint32, as it always has. Narrowing
// fails closed: a wrapped length is smaller than the real one, so bounds
// checks only get stricter. The page caps keep the length far below 2^32
// (TestWasmer2_DeclaredMemoryPageCap, TestWasmer2_MemoryGrowCap).
func (memory *Wasmer2Memory) dataLength() uint32 {
	return uint32(cWasmerMemoryDataLength(memory.cgoInstance))
}

// dataBase reads the pointer to the first byte of linear memory once.
// The pointer is invalidated by Grow.
func (memory *Wasmer2Memory) dataBase() *byte {
	return (*byte)(cWasmerMemoryData(memory.cgoInstance))
}

// bytesAt views exactly n bytes at off in a memory of memLen bytes starting
// at base. The slice aliases wasmer-owned memory and must not be retained:
// Grow may reallocate the buffer.
//
// Callers pass a span from executor.MemLoadRange or MemStoreRange, which is
// always in bounds. bytesAt checks it again and panics otherwise, so a caller
// that skips those helpers fails fast instead of reading or writing outside
// linear memory.
func bytesAt(base *byte, memLen uint32, off uint32, n uint32) []byte {
	if off > memLen || n > memLen-off {
		panic(fmt.Sprintf("wasmer2: memory view [%d, %d+%d) outside length %d", off, off, n, memLen))
	}
	if n == 0 {
		return []byte{}
	}
	if base == nil {
		panic("wasmer2: memory view over a nil base")
	}

	// #nosec G103: range-scoped view over wasmer-owned linear memory.
	return unsafe.Slice((*byte)(unsafe.Add(unsafe.Pointer(base), off)), n)
}

// Grow the memory by a number of pages (65kb each).
func (memory *Wasmer2Memory) Grow(numberOfPages uint32) error {
	var growResult = cWasmerMemoryGrow(memory.cgoInstance, cUint32T(numberOfPages))

	if growResult != cWasmerOk {
		var lastError, err = GetLastError()
		var errorMessage = "Failed to grow the memory:\n    %s"

		if err != nil {
			errorMessage = fmt.Sprintf(errorMessage, "(unknown details)")
		} else {
			errorMessage = fmt.Sprintf(errorMessage, lastError)
		}

		return fmt.Errorf("memory grow error: %s", errorMessage)
	}

	return nil
}
