// Package memprotect provides memory protection primitives to prevent
// sensitive data from being swapped to disk or captured in core dumps.
//
// On Unix systems, uses mlock(2) to lock pages in RAM.
// On Windows, uses VirtualLock to prevent paging.
package memprotect

import (
	"fmt"
	"unsafe"
)

// Lock prevents the memory region from being swapped to disk.
// Returns an unlock function to release the lock.
func Lock(b []byte) (unlock func(), err error) {
	if len(b) == 0 {
		return func() {}, nil
	}

	ptr := unsafe.Pointer(&b[0])
	size := uintptr(len(b))

	if err := mlockBytes(ptr, size); err != nil {
		return nil, fmt.Errorf("mlock: %w", err)
	}

	return func() {
		_ = munlockBytes(ptr, size)
	}, nil
}

// Zero securely zeros the byte slice.
func Zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
