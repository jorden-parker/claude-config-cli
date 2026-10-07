package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
)

func TestSaveAtomicIntegration(t *testing.T) {
	dir := t.TempDir()

	// New file: parents created, private mode.
	f := &File{Scope: ScopeProject, Path: filepath.Join(dir, "new", "settings.json"), Data: map[string]any{"model": "opus"}}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	if !f.Exists {
		t.Fatal("Exists should be true after a successful save")
	}
	if fi, err := os.Stat(f.Path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("new file mode: %v %v", fi, err)
	}

	// Existing file: mode kept, contents replaced.
	if err := os.Chmod(f.Path, 0o644); err != nil {
		t.Fatal(err)
	}
	f.Data["model"] = "sonnet"
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(f.Path); fi.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v", fi.Mode().Perm())
	}
	if got, _ := os.ReadFile(f.Path); string(got) != "{\n  \"model\": \"sonnet\"\n}\n" {
		t.Fatalf("got %q", got)
	}

	// Invalid destination: error propagates and nothing is marked saved.
	bad := &File{Scope: ScopeProject, Path: filepath.Join(dir, "isdir"), Data: map[string]any{}}
	if err := os.Mkdir(bad.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := bad.Save(); err == nil {
		t.Fatal("expected error saving over a directory")
	}
	if bad.Exists {
		t.Fatal("Exists must stay false after a failed save")
	}
	if fi, err := os.Stat(bad.Path); err != nil || !fi.IsDir() {
		t.Fatalf("directory was disturbed: %v %v", fi, err)
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	f, err := Open(ScopeProject, dir)
	if err != nil {
		t.Fatal(err)
	}
	if f.Exists {
		t.Fatal("fresh dir should have no project file")
	}
	if err := f.Set("permissions.allow", []string{"Bash"}); err != nil {
		t.Fatal(err)
	}
	if err := f.Set("theme", "light"); err != nil {
		t.Fatal(err)
	}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); err != nil {
		t.Fatal(err)
	}

	g, err := Open(ScopeProject, dir)
	if err != nil {
		t.Fatal(err)
	}
	v, ok := g.Get("permissions.allow")
	if !ok || Format(v) != `["Bash"]` {
		t.Fatalf("got %v", v)
	}
	if !g.Unset("theme") {
		t.Fatal("unset should report true")
	}
	if _, ok := g.Get("theme"); ok {
		t.Fatal("theme should be gone")
	}
	if _, ok := g.Get("permissions.allow"); !ok {
		t.Fatal("sibling key should survive unset")
	}
}

func TestAllowed(t *testing.T) {
	s := schema.Load()
	if !Allowed(s.Get("theme"), ScopeProject) {
		t.Error("theme should be allowed in project")
	}
	if Allowed(s.Get("diffTool"), ScopeProject) || !Allowed(s.Get("diffTool"), ScopeGlobal) {
		t.Error("diffTool is global-config only")
	}
	if Allowed(s.Get("allowManagedHooksOnly"), ScopeUser) {
		t.Error("managed-only key must not be writable")
	}
	if Allowed(s.Get("theme"), ScopeManaged) {
		t.Error("managed scope is read-only")
	}
}

func TestSaveKeepsKeyOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.json")
	before := `{
  "theme": "dark",
  "statusLine": {
    "type": "command",
    "command": "a <b>",
    "padding": 1
  },
  "hooks": [
    {
      "z": [],
      "a": {}
    }
  ],
  "env": {
    "B": "1",
    "A": "2"
  }
}
`
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := Open(ScopeProject, dir)
	if err != nil {
		t.Fatal(err)
	}
	// Untouched, the file comes back as it was, apart from JSON's own escaping.
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(before, "a <b>", `a \u003cb\u003e`, 1)
	if got, _ := os.ReadFile(path); string(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	// A replaced object keeps its order; new keys go last, sorted.
	_ = f.Set("statusLine", map[string]any{"padding": 2.0, "command": "c", "type": "command", "extra": true})
	_ = f.Set("model", "opus")
	_ = f.Set("apiKeyHelper", []string{"x"})
	f.Unset("hooks")
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	want = `{
  "theme": "dark",
  "statusLine": {
    "type": "command",
    "command": "c",
    "padding": 2,
    "extra": true
  },
  "env": {
    "B": "1",
    "A": "2"
  },
  "apiKeyHelper": [
    "x"
  ],
  "model": "opus"
}
`
	if got, _ := os.ReadFile(path); string(got) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func writeProjectFile(t *testing.T, dir, content string) {
	t.Helper()
	p := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestOpenRejectsInvalidDocuments(t *testing.T) {
	for _, tc := range []struct{ name, doc string }{
		{"null", "null"},
		{"array", `[1, 2]`},
		{"scalar", `42`},
		{"malformed", `{"a":`},
		{"trailing", `{"a": 1} x`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeProjectFile(t, dir, tc.doc)
			f, err := Open(ScopeProject, dir)
			if err == nil {
				t.Fatalf("Open(%q) returned no error", tc.doc)
			}
			if !strings.Contains(err.Error(), f.Path) {
				t.Fatalf("error %q does not name %s", err, f.Path)
			}
			if f.Data == nil {
				t.Fatal("an error-bearing file must keep a non-nil map for display")
			}
		})
	}
}

func TestOpenAcceptsEmptyDocuments(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write bool
		doc   string
	}{
		{"missing", false, ""},
		{"zero-byte", true, ""},
		{"whitespace", true, " \n\t "},
		{"empty-object", true, "{}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.write {
				writeProjectFile(t, dir, tc.doc)
			}
			f, err := Open(ScopeProject, dir)
			if err != nil {
				t.Fatal(err)
			}
			if f.Data == nil {
				t.Fatal("Data is nil")
			}
			if err := f.Set("theme", "dark"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
