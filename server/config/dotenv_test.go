package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvPreservesProcessEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("DOTENV_TEST_VALUE=from-file\nDOTENV_TEST_QUOTED=\"hello world\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOTENV_TEST_VALUE", "from-process")
	_ = os.Unsetenv("DOTENV_TEST_QUOTED")
	defer os.Unsetenv("DOTENV_TEST_QUOTED")

	LoadDotEnv(path)
	if got := os.Getenv("DOTENV_TEST_VALUE"); got != "from-process" {
		t.Fatalf("process environment should win, got %q", got)
	}
	if got := os.Getenv("DOTENV_TEST_QUOTED"); got != "hello world" {
		t.Fatalf("quoted value = %q, want hello world", got)
	}
}
