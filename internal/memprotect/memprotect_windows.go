//go:build windows
// +build windows

package memprotect

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// mlockBytes locks memory pages using VirtualLock.
func mlockBytes(ptr unsafe.Pointer, size uintptr) error {
	err := windows.VirtualLock(uintptr(ptr), size)
	if err != nil {
		return err
	}
	return nil
}

// munlockBytes unlocks memory pages using VirtualUnlock.
func munlockBytes(ptr unsafe.Pointer, size uintptr) error {
	err := windows.VirtualUnlock(uintptr(ptr), size)
	if err != nil && err != windows.ERROR_NOT_LOCKED {
		return err
	}
	return nil
}
