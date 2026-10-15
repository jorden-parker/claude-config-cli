package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jorden-parker/claude-config-cli/internal/catalog"
	"github.com/jorden-parker/claude-config-cli/internal/schema"
)

// docsDir copies the catalog test fixtures, with the checked-in reference,
// into a temp directory and returns it.
func docsDir(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "catalog", "testdata")
	dir := t.TempDir()
	for _, n := range []string{"ref.md", "env.md", "tools.md"} {
		b, err := os.ReadFile(filepath.Join(src, n))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dir, n), b, 0o600)
	}
	return dir
}

func TestSyncFromDirWritesCache(t *testing.T) {
	cache := t.TempDir()
	t.Setenv(schema.EnvCacheDir, cache)
	out, err := runCLI(t, "sync", "--from", docsDir(t))
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	p := filepath.Join(cache, "schema.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.Parse(b)
	if err != nil || s.Format != schema.CurrentFormat || len(s.Settings) < 200 {
		t.Fatalf("%v %+v", err, s)
	}
	if !strings.Contains(out, "saved to "+p) || !strings.Contains(out, "catalogue from the docs") {
		t.Fatalf("output:\n%s", out)
	}
	// The fixture env table is tiny, so the diff against the embedded
	// catalogue must mention env vars, but the second run is a no-op.
	if !strings.Contains(out, "envVars[") {
		t.Fatalf("no diff printed:\n%s", out)
	}
	out, err = runCLI(t, "sync", "--from", docsDir(t))
	if err != nil || !strings.Contains(out, "no changes") {
		t.Fatalf("second run: %v\n%s", err, out)
	}
}

func TestSyncCheckReportsDriftWithoutWriting(t *testing.T) {
	cache := t.TempDir()
	t.Setenv(schema.EnvCacheDir, cache)
	out, err := runCLI(t, "sync", "--check", "--from", docsDir(t))
	if err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("err = %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(cache, "schema.json")); !os.IsNotExist(err) {
		t.Fatal("--check wrote the cache")
	}
}

func TestSyncEmbedWritesOut(t *testing.T) {
	t.Setenv(schema.EnvCacheDir, t.TempDir())
	target := filepath.Join(t.TempDir(), "schema.json")
	os.WriteFile(target, schema.Embedded(), 0o600)
	out, err := runCLI(t, "sync", "--embed", "--out", target, "--from", docsDir(t))
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	b, _ := os.ReadFile(target)
	got, err := catalog.Decode(b)
	if err != nil || got.Format != schema.CurrentFormat {
		t.Fatal(err)
	}
	if !strings.Contains(out, "wrote "+target) {
		t.Fatalf("output:\n%s", out)
	}
	// Tools drift is fatal for --embed.
	dir := docsDir(t)
	os.WriteFile(filepath.Join(dir, "tools.md"), []byte("| Tool | Description |\n| - | - |\n| `Bash` | x |\n"), 0o600)
	if _, err := runCLI(t, "sync", "--embed", "--out", target, "--from", dir); err == nil || !strings.Contains(err.Error(), "Tools pane") {
		t.Fatalf("err = %v", err)
	}
}

func TestSyncRejectsUnreadableDocs(t *testing.T) {
	t.Setenv(schema.EnvCacheDir, t.TempDir())
	dir := docsDir(t)
	os.WriteFile(filepath.Join(dir, "ref.md"), []byte("# nothing here\n"), 0o600)
	if _, err := runCLI(t, "sync", "--from", dir); err == nil || !strings.Contains(err.Error(), "changed shape") {
		t.Fatalf("err = %v", err)
	}
}

func TestStaleNoteAfterCommands(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	defer func() { now = time.Now }()
	now = func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }
	out, err := runCLI(t, "ls", "sections")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "catalogue is") || !strings.Contains(out, "sync") {
		t.Fatalf("no staleness note:\n%s", out)
	}
	out, _ = runCLI(t, "sync", "--check", "--from", docsDir(t))
	if strings.Contains(out, "catalogue is") {
		t.Fatal("sync itself must not nag")
	}
	now = func() time.Time { return time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC) }
	if out, _ := runCLI(t, "ls", "sections"); strings.Contains(out, "catalogue is") {
		t.Fatalf("fresh catalogue flagged:\n%s", out)
	}
}

func TestNoCacheFlagParses(t *testing.T) {
	if _, err := runCLI(t, "ls", "sections", "--no-cache"); err != nil {
		t.Fatal(err)
	}
	if !flagNoCache {
		t.Fatal("flag not set")
	}
	flagNoCache = false
}
