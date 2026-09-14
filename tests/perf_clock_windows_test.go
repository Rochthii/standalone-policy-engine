//go:build windows

package tests

import (
	"syscall"
	"testing"
	"time"
	"unsafe"
)

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	queryPerformanceCounter   = kernel32.NewProc("QueryPerformanceCounter")
	queryPerformanceFrequency = kernel32.NewProc("QueryPerformanceFrequency")
)

type performanceClock struct {
	frequency int64
}

func newPerformanceClock(t testing.TB) performanceClock {
	t.Helper()
	var frequency int64
	if result, _, err := queryPerformanceFrequency.Call(uintptr(unsafe.Pointer(&frequency))); result == 0 {
		t.Fatalf("QueryPerformanceFrequency: %v", err)
	}
	return performanceClock{frequency: frequency}
}

func (c performanceClock) now() int64 {
	var counter int64
	if result, _, err := queryPerformanceCounter.Call(uintptr(unsafe.Pointer(&counter))); result == 0 {
		panic(err)
	}
	return counter
}

func (c performanceClock) elapsed(start int64) time.Duration {
	return time.Duration((c.now() - start) * int64(time.Second) / c.frequency)
}
