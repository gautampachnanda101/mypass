//go:build darwin || linux || freebsd || openbsd || netbsd
// +build darwin linux freebsd openbsd netbsd

package memprotect

import (
	"syscall"
	"unsafe"
)

// mlockBytes locks memory pages containing the given region.
func mlockBytes(ptr unsafe.Pointer, size uintptr) error {
	return syscall.Mlock((*[1 << 30]byte)(ptr)[:size])
}

// munlockBytes unlocks memory pages.
func munlockBytes(ptr unsafe.Pointer, size uintptr) error {
	return syscall.Munlock((*[1 << 30]byte)(ptr)[:size])
}
