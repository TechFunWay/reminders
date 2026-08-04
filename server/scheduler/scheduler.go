// Package scheduler is a tiny registry for background jobs, mirroring the apps
// registry. An app registers a periodic job once and the framework runs it for
// the process lifetime, recovering from panics so one bad job can't crash the
// server.
//
//	func init() {
//	    scheduler.Register(scheduler.Job{
//	        Name:     "cleanup",
//	        Interval: time.Hour,
//	        Run:      func() { /* ... */ },
//	    })
//	    // or a fixed daily time:
//	    scheduler.Register(scheduler.Job{Name: "digest", Daily: "08:00", Run: sendDigest})
//	}
package scheduler

import (
	"time"

	"smallgo/server/logger"
)

type Job struct {
	Name string
	// Interval runs Run every Interval. Takes precedence over Daily.
	Interval time.Duration
	// Daily runs Run once per day at this local "15:04" time (used when
	// Interval is 0).
	Daily string
	// RunAtStart runs Run once immediately when the scheduler starts.
	RunAtStart bool
	Run        func()
}

var registered []Job

// Register adds a job. Call from init() before the server starts.
func Register(j Job) { registered = append(registered, j) }

// All returns the registered jobs (useful for tests/introspection).
func All() []Job { return registered }

type Scheduler struct{ stop chan struct{} }

// Start launches every registered job in its own goroutine.
func Start() *Scheduler {
	s := &Scheduler{stop: make(chan struct{})}
	for _, j := range registered {
		go s.run(j)
	}
	if len(registered) > 0 {
		logger.Info("scheduler: started %d job(s)", len(registered))
	}
	return s
}

// Stop signals all jobs to exit. Running jobs finish their current iteration.
func (s *Scheduler) Stop() { close(s.stop) }

func (s *Scheduler) run(j Job) {
	safeRun := func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("scheduler: job %q panicked: %v", j.Name, r)
			}
		}()
		j.Run()
	}

	if j.RunAtStart {
		safeRun()
	}

	switch {
	case j.Interval > 0:
		t := time.NewTicker(j.Interval)
		defer t.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-t.C:
				safeRun()
			}
		}
	case j.Daily != "":
		for {
			timer := time.NewTimer(untilNext(j.Daily))
			select {
			case <-s.stop:
				timer.Stop()
				return
			case <-timer.C:
				safeRun()
			}
		}
	default:
		logger.Warn("scheduler: job %q has no Interval or Daily; not scheduled", j.Name)
	}
}

// untilNext returns the duration from now until the next occurrence of the
// "15:04" local time hhmm. Falls back to one hour on a parse error.
func untilNext(hhmm string) time.Duration {
	now := time.Now()
	t, err := time.ParseInLocation("15:04", hhmm, now.Location())
	if err != nil {
		logger.Warn("scheduler: invalid Daily time %q, retrying in 1h", hhmm)
		return time.Hour
	}
	next := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now)
}
