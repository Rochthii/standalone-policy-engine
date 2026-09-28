//go:build !windows

package security_test

import (
	"testing"
	"time"
)

type evalClock struct{}

func newEvalClock(_ testing.TB) evalClock       { return evalClock{} }
func (evalClock) now() time.Time                { return time.Now() }
func (evalClock) elapsed(start time.Time) int64 { return time.Since(start).Nanoseconds() }
func (evalClock) metadata() map[string]any {
	return map[string]any{"name": "time.Now/time.Since monotonic", "frequency_hz": nil,
		"nominal_tick_ns": nil, "overhead": "clock calls, function dispatch and result predicate retained; control not subtracted"}
}
