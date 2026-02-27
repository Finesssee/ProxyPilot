//go:build windows

package winutil

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32     = syscall.NewLazyDLL("kernel32.dll")
	createMutexW = kernel32.NewProc("CreateMutexW")
	releaseMutex = kernel32.NewProc("ReleaseMutex")
	closeHandle  = kernel32.NewProc("CloseHandle")
	getLastError = kernel32.NewProc("GetLastError")
)

const (
	ERROR_ALREADY_EXISTS = 183
)

// SingleInstance holds a Windows mutex handle for single-instance enforcement.
type SingleInstance struct {
	handle syscall.Handle
	name   string
}

// AcquireSingleInstance tries to acquire a named mutex. Returns nil if another instance is running.
func AcquireSingleInstance(name string) (*SingleInstance, error) {
	if name == "" {
		name = "ProxyPilot"
	}
	mutexName := "Global\\" + name + "_SingleInstance"

	namePtr, err := syscall.UTF16PtrFromString(mutexName)
	if err != nil {
		return nil, fmt.Errorf("invalid mutex name: %w", err)
	}

	// CreateMutexW(NULL, FALSE, name)
	handle, _, err := createMutexW.Call(0, 0, uintptr(unsafe.Pointer(namePtr)))
	if handle == 0 {
		return nil, fmt.Errorf("CreateMutexW failed: %w", err)
	}

	// Check if mutex already existed
	lastErr, _, _ := getLastError.Call()
	if lastErr == ERROR_ALREADY_EXISTS {
		closeHandle.Call(handle)
		return nil, nil // Another instance is running
	}

	return &SingleInstance{
		handle: syscall.Handle(handle),
		name:   mutexName,
	}, nil
}

// Release releases the mutex.
func (s *SingleInstance) Release() {
	if s != nil && s.handle != 0 {
		releaseMutex.Call(uintptr(s.handle))
		closeHandle.Call(uintptr(s.handle))
		s.handle = 0
	}
}
