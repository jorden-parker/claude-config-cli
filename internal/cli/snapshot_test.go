package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	project := t.TempDir()
	archive := filepath.Join(t.TempDir(), "backup.json")
	run := func(args ...string) string {
		t.Helper()
		root := Root()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetErr(&output)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		return output.String()
	}
	run("set", "effortLevel", "high")
	run("set", "env", "FOO=bar", "-s", "project", "-C", project)
	run("snapshot", archive, "-C", project)
	user := filepath.Join(home, ".claude", "settings.json")
	local := filepath.Join(project, ".claude", "settings.json")
	for _, path := range []string{user, local} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	if result := run("restore", archive); !strings.Contains(result, "Restored 2 files") {
		t.Fatal(result)
	}
	for path, want := range map[string]string{user: `"effortLevel": "high"`, local: `"FOO": "bar"`} {
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), want) {
			t.Fatalf("%s: %s, %v", path, data, err)
		}
	}
	help := run("--help")
	if strings.Contains(strings.ToLower(help), "rtk") {
		t.Fatal("RTK remains in help")
	}
	if !strings.Contains(help, "snapshot") || !strings.Contains(help, "restore") {
		t.Fatal(help)
	}
}
