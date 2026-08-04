package config

import (
	"bufio"
	"os"
	"strings"
)

// LoadDotEnv loads simple KEY=VALUE pairs for local development. Existing
// process environment variables always win, so Docker and deployment settings
// cannot be accidentally overridden by a local file.
func LoadDotEnv(paths ...string) {
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimPrefix(line, "export ")
			key, value, ok := strings.Cut(line, "=")
			key = strings.TrimSpace(key)
			if !ok || key == "" {
				continue
			}
			if _, exists := os.LookupEnv(key); exists {
				continue
			}
			value = strings.TrimSpace(value)
			if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}
			_ = os.Setenv(key, value)
		}
		_ = file.Close()
	}
}
