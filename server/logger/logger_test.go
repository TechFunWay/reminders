package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupTestLogger(t *testing.T) string {
	t.Helper()
	if l != nil {
		t.Fatal("test logger must start uninitialised")
	}
	dir := t.TempDir()
	if err := Init(dir, 0, false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(Close)
	return dir
}

func TestLoggerWritesToFileWithoutConsoleMirror(t *testing.T) {
	dir := setupTestLogger(t)
	Info("file-only message")
	if l.console {
		t.Fatal("console mirroring should be disabled")
	}

	path := filepath.Join(dir, "info-"+time.Now().Format("2006-01-02")+".log")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "[INFO] file-only message") {
		t.Fatalf("missing log message: %q", content)
	}
}

func TestNewWriterUsesConfiguredLogFile(t *testing.T) {
	dir := setupTestLogger(t)
	if _, err := NewWriter("warn").Write([]byte("database warning\n")); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "warn-"+time.Now().Format("2006-01-02")+".log")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "[WARN] database warning") {
		t.Fatalf("missing warning message: %q", content)
	}
}
