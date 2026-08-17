// Package logger provides a small dependency-free file logger with daily
// rotation and automatic cleanup. Levels are INFO / WARN / ERROR / AUDIT,
// each written to its own daily file (e.g. logs/info-2006-01-02.log). Console
// mirroring is opt-in so background activity does not flood a service console.
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type logger struct {
	mu            sync.Mutex
	logDir        string
	retentionDays int
	console       bool
	files         map[string]*os.File
	done          chan struct{}
}

var l *logger

// retentionFunc, when set, is called by the cleanup goroutine to obtain the
// current retention-days value at runtime (e.g. from the sysconfig table).
// When nil the static value passed to Init is used.
var retentionFunc func() int

// SetRetentionFunc registers a callback that returns the current log retention
// days. The cleanup goroutine calls it on every tick so admin changes take
// effect without a restart.
func SetRetentionFunc(f func() int) {
	retentionFunc = f
}

func currentRetention(fallback int) int {
	if retentionFunc != nil {
		return retentionFunc()
	}
	return fallback
}

// Init starts the logger writing into logDir, keeping retentionDays of history
// (a value <= 0 disables cleanup). mirrorToConsole is intended only for
// interactive debugging; normal service operation keeps logs in files.
func Init(logDir string, retentionDays int, mirrorToConsole bool) error {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	l = &logger{
		logDir:        logDir,
		retentionDays: retentionDays,
		console:       mirrorToConsole,
		files:         make(map[string]*os.File),
		done:          make(chan struct{}),
	}

	// Run initial cleanup if retention is configured.
	if days := currentRetention(retentionDays); days > 0 {
		cleanOldLogs(logDir, days)
	}

	// Always start the ticker so that a later admin change from 0 → N
	// takes effect without a restart.
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if days := currentRetention(retentionDays); days > 0 {
					cleanOldLogs(logDir, days)
				}
			case <-l.done:
				return
			}
		}
	}()

	return nil
}

// Close flushes and closes open log files. Safe to call when uninitialized.
func Close() {
	if l == nil {
		return
	}
	close(l.done)
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, f := range l.files {
		f.Close()
	}
	l = nil
}

func Info(format string, args ...interface{})  { write("INFO", format, args...) }
func Warn(format string, args ...interface{})  { write("WARN", format, args...) }
func Error(format string, args ...interface{}) { write("ERROR", format, args...) }

// Audit records a security-relevant action to the audit log file. The DB-backed
// audit trail lives in the audit package; this is the human-readable mirror.
func Audit(format string, args ...interface{}) { write("AUDIT", format, args...) }

// NewWriter adapts standard-library loggers, such as GORM's logger, so their
// output follows the same file-only policy and daily rotation as app logs.
func NewWriter(level string) io.Writer {
	return lineWriter{level: strings.ToUpper(strings.TrimSpace(level))}
}

type lineWriter struct {
	level string
}

func (w lineWriter) Write(p []byte) (int, error) {
	message := strings.TrimSpace(string(p))
	if message != "" {
		level := w.level
		if level == "" {
			level = "INFO"
		}
		write(level, "%s", message)
	}
	return len(p), nil
}

func write(level, format string, args ...interface{}) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf("%s [%s] %s\n", ts, level, fmt.Sprintf(format, args...))

	if l == nil {
		// Init has not run yet, so a file destination is unavailable. Startup
		// failures should still be visible to the invoking terminal.
		fmt.Fprint(os.Stderr, msg)
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if f := l.getFile(level); f != nil {
		f.WriteString(msg)
	}
	if l.console {
		fmt.Fprint(os.Stdout, msg)
	}
}

func (l *logger) getFile(level string) *os.File {
	key := strings.ToLower(level)
	today := time.Now().Format("2006-01-02")
	want := filepath.Join(l.logDir, fmt.Sprintf("%s-%s.log", key, today))

	if f, ok := l.files[key]; ok {
		if f.Name() == want {
			return f
		}
		f.Close() // date rolled over
	}

	f, err := os.OpenFile(want, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file %s: %v\n", want, err)
		return nil
	}
	l.files[key] = f
	return f
}

func cleanOldLogs(logDir string, days int) {
	cutoff := time.Now().AddDate(0, 0, -days)
	for _, level := range []string{"info", "warn", "error", "audit"} {
		matches, err := filepath.Glob(filepath.Join(logDir, level+"-*.log"))
		if err != nil {
			continue
		}
		for _, path := range matches {
			base := filepath.Base(path)
			dateStr := strings.TrimSuffix(strings.TrimPrefix(base, level+"-"), ".log")
			t, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				continue
			}
			if t.Before(cutoff) {
				os.Remove(path)
			}
		}
	}
}
