package scheduler

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestRunAtStartAndStop(t *testing.T) {
	registered = nil // isolate from other registrations
	var runs int32
	Register(Job{
		Name:       "test",
		Interval:   10 * time.Millisecond,
		RunAtStart: true,
		Run:        func() { atomic.AddInt32(&runs, 1) },
	})

	s := Start()
	time.Sleep(35 * time.Millisecond)
	s.Stop()

	if atomic.LoadInt32(&runs) < 2 {
		t.Fatalf("expected job to run at least twice (start + ticks), got %d", runs)
	}
}

func TestUntilNextWithinDay(t *testing.T) {
	d := untilNext("23:59")
	if d <= 0 || d > 24*time.Hour {
		t.Fatalf("untilNext returned out-of-range duration: %v", d)
	}
	if d := untilNext("bogus"); d != time.Hour {
		t.Fatalf("invalid time should fall back to 1h, got %v", d)
	}
}
