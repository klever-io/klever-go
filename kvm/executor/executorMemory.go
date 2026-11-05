package executor

import (
	"fmt"

	"github.com/klever-io/klever-go/kvm/math"
)

// MemPtr indicates that an argument refers to a location in WASM memory.
type MemPtr int32

// MemLength indicates that an argument refers to the length of a data section in WASM memory.
type MemLength = int32

// Offset adds to a pointer to WASM memory.
func (memPtr MemPtr) Offset(offset int32) MemPtr {
	return MemPtr(math.AddInt32(int32(memPtr), offset))
}

// MemLoadRange reports the byte span a load copies out of linear memory.
// memoryLength is the byte length, read once for this call. The checks are the
// historical ones, unchanged.
//
// n == 0 and a nil error is an empty success: the load returns []byte{}. A
// zero-length load is that success for any offset. A load whose end runs past
// memoryLength is also a success and n is the truncated tail, which is
// consensus-sensitive. Errors keep their historical order: bad bounds, then
// negative length.
func MemLoadRange(memoryLength uint32, memPtr MemPtr, length MemLength) (off uint32, n uint32, err error) {
	if length == 0 {
		return 0, 0, nil
	}

	requestedEnd := memPtr.Offset(length)

	isOffsetTooSmall := memPtr < 0
	isOffsetTooLarge := uint32(memPtr) > memoryLength             // #nosec G115
	isRequestedEndTooLarge := uint32(requestedEnd) > memoryLength // #nosec G115
	isLengthNegative := length < 0

	if isOffsetTooSmall || isOffsetTooLarge {
		return 0, 0, fmt.Errorf("mem load: %w", ErrMemoryBadBounds)
	}
	if isLengthNegative {
		return 0, 0, fmt.Errorf("mem load: %w", ErrMemoryNegativeLength)
	}

	// memPtr is non-negative and at most memoryLength here.
	if isRequestedEndTooLarge {
		return uint32(memPtr), memoryLength - uint32(memPtr), nil // #nosec G115
	}

	// length is positive and Offset saturates at MaxInt32, so the span is
	// non-negative.
	return uint32(memPtr), uint32(requestedEnd - memPtr), nil // #nosec G115
}

// MemStoreRange reports the byte span a store of dataLength bytes writes into
// linear memory. memoryLength is the byte length, read once for this call.
// It takes the length rather than the data so tests can exercise lengths that
// cannot be allocated; callers pass len(data).
//
// n == 0 and a nil error is a no-op, including a zero-length store at any
// offset. The data length is narrowed to int32 first, matching the historical
// check. A negative offset is ErrMemoryBadBoundsLower and a range that would
// pass the end of memory is ErrMemoryBadBoundsUpper, in that order.
//
// A data length that narrows to a negative int32 (2 GiB or more) is
// ErrMemoryBadBoundsUpper. Historically such a store either got that error or
// panicked on an inverted slice. Here the guard is required: without it the
// negative span would convert to a large uint32 and the caller would write
// past the end of memory. No host buffer reaches that size, so the change is
// not consensus-visible.
func MemStoreRange(memoryLength uint32, memPtr MemPtr, dataLength int) (off uint32, n uint32, err error) {
	narrowedLength := int32(dataLength) // #nosec G115
	if narrowedLength == 0 {
		return 0, 0, nil
	}

	requestedEnd := memPtr.Offset(narrowedLength)

	if memPtr < 0 {
		return 0, 0, ErrMemoryBadBoundsLower
	}

	if narrowedLength < 0 || uint32(requestedEnd) > memoryLength { // #nosec G115
		return 0, 0, ErrMemoryBadBoundsUpper
	}

	// narrowedLength is positive and Offset saturates at MaxInt32, so the span
	// is non-negative.
	return uint32(memPtr), uint32(requestedEnd - memPtr), nil // #nosec G115
}

// MemLoadBytes is MemLoad over a Go-owned memory buffer, for executors and
// mocks whose linear memory is a []byte. The result never aliases mem.
// A buffer of 4 GiB or more narrows to a smaller length, which only makes the
// bounds checks stricter.
func MemLoadBytes(mem []byte, memPtr MemPtr, length MemLength) ([]byte, error) {
	off, n, err := MemLoadRange(uint32(len(mem)), memPtr, length) // #nosec G115
	if err != nil {
		return nil, err
	}

	result := make([]byte, n)
	copy(result, mem[off:off+n])
	return result, nil
}

// MemStoreBytes is MemStore over a Go-owned memory buffer.
func MemStoreBytes(mem []byte, memPtr MemPtr, data []byte) error {
	off, n, err := MemStoreRange(uint32(len(mem)), memPtr, len(data)) // #nosec G115
	if err != nil {
		return err
	}

	copy(mem[off:off+n], data)
	return nil
}
