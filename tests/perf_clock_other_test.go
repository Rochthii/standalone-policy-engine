//go:build !windows

package tests

import (
	"testing"
	"time"
)

type performanceClock struct{}

func newPerformanceClock(_ testing.TB) performanceClock {
	return performanceClock{}
}

func (performanceClock) now() time.Time {
	return time.Now()
}

func (performanceClock) elapsed(start time.Time) time.Duration {
	return time.Since(start)
}
