package cli

import (
	"os"
	"testing"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
)

// TestMain keeps the synced catalogue out of tests: every schema.Load reads
// the embedded copy.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ccfg-test-cache")
	if err != nil {
		panic(err)
	}
	os.Setenv(schema.EnvCacheDir, dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
