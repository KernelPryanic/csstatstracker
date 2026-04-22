//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// notifyAlreadyRunning pops up a native Windows message box telling the user
// another instance is running. We can't use Fyne dialogs here because the app
// hasn't finished initialising its window yet.
func notifyAlreadyRunning() {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBoxW := user32.NewProc("MessageBoxW")

	title, _ := syscall.UTF16PtrFromString("CS Stats Tracker")
	text, _ := syscall.UTF16PtrFromString("CS Stats Tracker is already running.")

	// MB_OK | MB_ICONINFORMATION
	const flags = 0x00000000 | 0x00000040
	_, _, _ = messageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(text)),
		uintptr(unsafe.Pointer(title)),
		flags,
	)
}
