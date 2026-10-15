package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCLI executes the root command with args and returns its output; a
// non-nil error is returned rather than failing, so tests can expect it.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := Root()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func seedUser(t *testing.T, home string, doc string) string {
	t.Helper()
	p := filepath.Join(home, ".claude", "settings.json")
	os.MkdirAll(filepath.Dir(p), 0o700)
	if err := os.WriteFile(p, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func readPerms(t *testing.T, p string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	json.Unmarshal(b, &doc)
	perms, _ := doc["permissions"].(map[string]any)
	return perms
}

func TestSetPermissionRemovesConflicts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	p := seedUser(t, home, `{"permissions":{"allow":["Bash","Bash(npm *)","Read"],"ask":["Bash(rm *)"]}}`)

	// set uses fmt.Printf, so capture stdout.
	r, w, _ := os.Pipe()
	saved := os.Stdout
	os.Stdout = w
	_, err := runCLI(t, "set", "permissions.deny", "Bash,Bash")
	w.Close()
	os.Stdout = saved
	var out bytes.Buffer
	out.ReadFrom(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `removed "Bash(rm *)" from permissions.ask and "Bash", "Bash(npm *)" from permissions.allow`) {
		t.Fatalf("output = %q", out.String())
	}
	perms := readPerms(t, p)
	if got := perms["allow"]; len(got.([]any)) != 1 || got.([]any)[0] != "Read" {
		t.Fatalf("allow = %v", got)
	}
	if got := perms["deny"]; len(got.([]any)) != 1 {
		t.Fatalf("deny not deduped: %v", got)
	}
	if got := perms["ask"]; len(got.([]any)) != 0 {
		t.Fatalf("ask = %v", got)
	}

	// Allowing a narrower rule again removes the whole-tool deny.
	os.Stdout, _ = os.Open(os.DevNull)
	_, err = runCLI(t, "set", "permissions.allow", "Bash(npm *)")
	os.Stdout = saved
	if err != nil {
		t.Fatal(err)
	}
	perms = readPerms(t, p)
	if got := perms["deny"]; len(got.([]any)) != 0 {
		t.Fatalf("deny = %v", got)
	}
}

func TestCheckReportsConflictsAcrossFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	project := t.TempDir()
	seedUser(t, home, `{"permissions":{"deny":["Bash"],"allow":["Bash(npm *)"]}}`)
	os.MkdirAll(filepath.Join(project, ".claude"), 0o700)
	os.WriteFile(filepath.Join(project, ".claude", "settings.json"), []byte(`{"permissions":{"ask":["Edit"],"allow":["Edit","Read"]}}`), 0o600)

	out, err := runCLI(t, "check", "-C", project)
	if err == nil || !strings.Contains(err.Error(), "2 permission conflict") {
		t.Fatalf("err = %v\n%s", err, out)
	}
	for _, want := range []string{"user", "project", `deny "Bash" makes allow "Bash(npm *)" dead`, `"Edit" is in both ask and allow; ask wins`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in\n%s", want, out)
		}
	}

	os.WriteFile(filepath.Join(project, ".claude", "settings.json"), []byte(`{"permissions":{"allow":["Read"]}}`), 0o600)
	seedUser(t, home, `{}`)
	out, err = runCLI(t, "check", "-C", project)
	if err != nil || !strings.Contains(out, "no permission conflicts") {
		t.Fatalf("err = %v\n%s", err, out)
	}
}
