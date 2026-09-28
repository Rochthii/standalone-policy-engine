//go:build windows

package security_test

import (
	"syscall"
	"testing"
	"unsafe"
)

// Follow tests/perf_clock_windows_test.go: this clock belongs only to the
// measurement harness, never to security validity/expiry decisions.
var evalKernel = syscall.NewLazyDLL("kernel32.dll")
var evalCounter = evalKernel.NewProc("QueryPerformanceCounter")
var evalFrequency = evalKernel.NewProc("QueryPerformanceFrequency")

type evalClock struct{ frequency int64 }

func newEvalClock(t testing.TB) evalClock {
	t.Helper()
	var frequency int64
	if ok, _, err := evalFrequency.Call(uintptr(unsafe.Pointer(&frequency))); ok == 0 || frequency <= 0 {
		t.Fatalf("QueryPerformanceFrequency: frequency=%d error=%v", frequency, err)
	}
	return evalClock{frequency: frequency}
}

func (c evalClock) now() int64 {
	var ticks int64
	if ok, _, err := evalCounter.Call(uintptr(unsafe.Pointer(&ticks))); ok == 0 {
		panic(err)
	}
	return ticks
}

func (c evalClock) elapsed(start int64) int64 {
	ticks := c.now() - start
	return (ticks/c.frequency)*1_000_000_000 + (ticks%c.frequency)*1_000_000_000/c.frequency
}

func (c evalClock) metadata() map[string]any {
	return map[string]any{"name": "QueryPerformanceCounter", "frequency_hz": c.frequency,
		"nominal_tick_ns": 1e9 / float64(c.frequency),
		"overhead":        "QPC syscall, function dispatch and result predicate retained; control not subtracted"}
}
