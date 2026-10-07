package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jorden-parker/claude-config-cli/internal/statusline"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

// realHome is read before any test points HOME at a temporary directory, so
// the build below keeps using the real Go caches.
var realHome, _ = os.UserHomeDir()

// buildCLI builds the real binary into a temporary directory.
func buildCLI(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "ccfg")
	build := exec.Command("go", "build", "-o", bin, "./cmd/ccfg")
	build.Dir = filepath.Join("..", "..")
	build.Env = append(os.Environ(), "HOME="+realHome)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func TestStatuslineCheckIntegration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	bin := buildCLI(t) // before HOME changes, so the Go caches are found
	home, project := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	marker := filepath.Join(t.TempDir(), "ran")
	settings := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"statusLine":{"type":"command","command":"touch '` + marker + `'; echo INHERITED"}}`
	if err := os.WriteFile(settings, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	// Name the binary the way the project settings do, so nothing else runs.
	t.Setenv("PATH", filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))
	run := func(stdin string, args ...string) (string, error) {
		cmd := exec.Command(bin, args...)
		cmd.Dir = project
		cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home)
		cmd.Stdin = strings.NewReader(stdin)
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		err := cmd.Run()
		return out.String(), err
	}
	ran := func() bool { _, err := os.Stat(marker); return err == nil }

	if out, err := run("", "statusline", "on", "-C", project); err != nil {
		t.Fatalf("statusline on: %v\n%s", err, out)
	}
	if ran() {
		t.Fatal("statusline on ran the existing command")
	}
	saved, _ := store.Open(store.ScopeUser, project)
	v, _ := saved.Get("statusLine")
	command := statusline.CommandOf(v)
	if !statusline.IsOurs(command) || strings.Contains(command, "--check") {
		t.Fatalf("statusLine command = %q", command)
	}
	config := statusline.ConfigPath(store.ScopeUser, project)
	cfg, err := statusline.Load(config)
	if err != nil || !strings.Contains(cfg.Inherited, "INHERITED") || cfg.InheritedScope != store.ScopeUser {
		t.Fatalf("config = %+v, %v", cfg, err)
	}

	input := `{"model":{"display_name":"Opus"}}`
	if out, err := run(input, "statusline", "render", "--check", "--config", config); err != nil {
		t.Fatalf("render --check: %v\n%s", err, out)
	}
	if ran() {
		t.Fatal("render --check ran the existing command")
	}

	// A config that cannot be read is an error in check mode only.
	if err := os.WriteFile(config, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := run(input, "statusline", "render", "--check", "--config", config); err == nil {
		t.Fatal("render --check accepted a malformed config")
	}
	if _, err := run(input, "statusline", "render", "--config", config); err != nil {
		t.Fatalf("a normal render must always exit 0: %v", err)
	}
	if err := statusline.Save(config, cfg); err != nil {
		t.Fatal(err)
	}

	// Ordinary rendering still runs the kept command.
	out, err := run(input, "statusline", "render", "--config", config)
	if err != nil || !strings.Contains(out, "INHERITED") || !ran() {
		t.Fatalf("render: %v %q ran=%v", err, out, ran())
	}
}
