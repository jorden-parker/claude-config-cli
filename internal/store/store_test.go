package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
)

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
