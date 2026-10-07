package statusline

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jorden-parker/claude-config-cli/internal/store"
)

var now = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func plain(fields ...string) *Config {
	c := Default()
	c.Fields, c.Color, c.Separator = fields, false, " | "
	return c
}

func TestComposeRendersDocumentedFields(t *testing.T) {
	all := make([]string, len(Fields))
	for i, f := range Fields {
		all[i] = f.ID
	}
	got := Compose(plain(all...), Sample("/work/my-app", now), "", now)
	for _, want := range []string{
		"Opus", "claude-opus-5-5", "high", "thinking", "NORMAL", "security-reviewer", "my-session", "abc12345", "v2.1.90",
		"my-app", "anthropics/claude-code", "my-feature", "#1234 pending",
		"8% ctx", "92% left", "200k window", "15.5k↑ 1.2k↓", "cache 91%", "cache warm 1h",
		"$0.01", "45s", "api 2s", "+156 -23",
		"5h 24%", "5h ↺ 14:00", "7d 41%", "7d ↺ Sat 12:00", "spend 63% $314/$500",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	// Off in the example session, so they take no room.
	for _, absent := range []string{"fast", ">200k", "dirs"} {
		if strings.Contains(got, absent) {
			t.Errorf("%q should be dropped:\n%s", absent, got)
		}
	}
}

// The docs list fields that are absent or null early in a session.
func TestComposeDropsAbsentAndNullFields(t *testing.T) {
	all := make([]string, len(Fields))
	for i, f := range Fields {
		all[i] = f.ID
	}
	for _, input := range []string{
		``, `not json`, `{}`,
		`{"context_window":{"used_percentage":null,"remaining_percentage":null,"current_usage":null},"rate_limits":{"five_hour":null},"prompt_cache":{"hit_ratio":null}}`,
	} {
		if got := Compose(plain(all...), []byte(input), "", now); got != "" {
			t.Errorf("input %q rendered %q, want nothing", input, got)
		}
	}
	got := Compose(plain("model", "ctx_used", "vim", "dir"), []byte(`{"model":{"display_name":"Opus"},"cwd":"/a/b"}`), "", now)
	if got != "Opus | b" {
		t.Fatalf("got %q", got)
	}
}

func TestComposePlacesExistingStatusLine(t *testing.T) {
	input := []byte(`{"model":{"display_name":"Opus"}}`)
	for pos, want := range map[string]string{
		ExistingStart:  "[CAVEMAN] | Opus",
		ExistingEnd:    "Opus | [CAVEMAN]",
		ExistingAbove:  "[CAVEMAN]\nOpus",
		ExistingBelow:  "Opus\n[CAVEMAN]",
		ExistingHidden: "Opus",
	} {
		c := plain("model")
		c.Existing = pos
		if got := Compose(c, input, "[CAVEMAN]\n", now); got != want {
			t.Errorf("%s: got %q, want %q", pos, got, want)
		}
	}
	// With no fields of its own, the existing line passes through untouched.
	if got := Compose(plain(), input, "\x1b[31mA\x1b[0m\nB\n", now); got != "\x1b[31mA\x1b[0m\nB" {
		t.Fatalf("got %q", got)
	}
	// A multi-line existing line gains the fields on the row they sit next to.
	c := plain("model")
	if got := Compose(c, input, "A\nB", now); got != "A\nB | Opus" {
		t.Fatalf("got %q", got)
	}
}

func TestColorWrapsOnlyOwnFields(t *testing.T) {
	c := plain("model")
	c.Color = true
	got := Compose(c, []byte(`{"model":{"display_name":"Opus"}}`), "kept", now)
	if !strings.HasPrefix(got, "kept") || !strings.Contains(got, "\x1b[38;5;214mOpus\x1b[0m") {
		t.Fatalf("got %q", got)
	}
}

func TestRenderRunsInheritedCommandWithTheSameInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "ccfg-statusline.json")
	input := []byte(`{"model":{"display_name":"Opus"},"workspace":{"project_dir":"` + dir + `"}}`)

	c := plain("model")
	c.Inherited = `printf '[%s]' "$(cat | grep -o Opus)"`
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	if got := Render(path, input); got != "[Opus] | Opus" {
		t.Fatalf("got %q", got)
	}
	// A config path relative to the project directory, as project settings use.
	if got := Render("ccfg-statusline.json", input); got != "[Opus] | Opus" {
		t.Fatalf("relative config: got %q", got)
	}

	// A failing or silent command is left out; the rest still renders.
	for _, bad := range []string{"echo nope; exit 3", "true", "exit 1", "sleep 30"} {
		if bad == "sleep 30" && testing.Short() {
			continue
		}
		c.Inherited = bad
		if err := Save(path, c); err != nil {
			t.Fatal(err)
		}
		if got := Render(path, input); got != "Opus" {
			t.Errorf("%q: got %q", bad, got)
		}
	}

	// The renderer never runs itself.
	c.Inherited = "ccfg" + renderArgs + path
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	if got := Render(path, input); got != "Opus" {
		t.Fatalf("recursion: got %q", got)
	}
	// A missing config renders nothing rather than failing.
	if got := Render(filepath.Join(dir, "missing.json"), input); got != "" {
		t.Fatalf("missing config: got %q", got)
	}
}

func TestEnableKeepsAndDisableRestores(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()
	f, _ := store.Open(store.ScopeUser, cwd)
	original := map[string]any{"type": "command", "command": "~/.claude/statusline.sh", "padding": float64(2)}
	if err := f.Set("statusLine", original); err != nil {
		t.Fatal(err)
	}
	c := Default()
	command := Command(store.ScopeUser, cwd)
	if !IsOurs(command) {
		t.Fatalf("command %q not recognised", command)
	}

	for i := 0; i < 2; i++ { // enabling twice must not wrap the renderer in itself
		if err := Enable(f, c, command, ""); err != nil {
			t.Fatal(err)
		}
		v, _ := f.Get("statusLine")
		obj := v.(map[string]any)
		if obj["command"] != command || obj["padding"] != float64(2) || obj["type"] != "command" {
			t.Fatalf("statusLine = %v", obj)
		}
		if c.Inherited != "~/.claude/statusline.sh" {
			t.Fatalf("inherited = %q", c.Inherited)
		}
	}
	if len(c.Fields) != 0 {
		t.Fatalf("fields = %v, want the existing line untouched", c.Fields)
	}

	if err := Disable(f, c); err != nil {
		t.Fatal(err)
	}
	if v, _ := f.Get("statusLine"); store.Format(v) != store.Format(original) {
		t.Fatalf("restored %s", store.Format(v))
	}
	// Someone else's status line is left alone.
	if err := Disable(f, c); err != nil {
		t.Fatal(err)
	}
	if v, _ := f.Get("statusLine"); CommandOf(v) != "~/.claude/statusline.sh" {
		t.Fatalf("disable touched a foreign status line: %v", v)
	}
}

func TestEnableWithNothingToKeep(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()
	f, _ := store.Open(store.ScopeProject, cwd)
	c := Default()
	command := Command(store.ScopeProject, cwd)
	if command != "ccfg statusline render --config .claude/ccfg-statusline.json" {
		t.Fatalf("project command = %q", command)
	}
	// The user file's command is carried over when the project file sets none.
	if err := Enable(f, c, command, "~/.claude/statusline.sh"); err != nil {
		t.Fatal(err)
	}
	if c.Inherited != "~/.claude/statusline.sh" || c.Previous != nil {
		t.Fatalf("inherited = %q previous = %v", c.Inherited, c.Previous)
	}
	if err := Disable(f, c); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.Get("statusLine"); ok {
		t.Fatal("statusLine should be removed when the file had none")
	}

	fresh := Default()
	g, _ := store.Open(store.ScopeLocal, cwd)
	if err := Enable(g, fresh, Command(store.ScopeLocal, cwd), ""); err != nil {
		t.Fatal(err)
	}
	if strings.Join(fresh.Fields, ",") != strings.Join(DefaultFields, ",") {
		t.Fatalf("fields = %v", fresh.Fields)
	}
}

func TestExistingFollowsOurOwnRenderer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := t.TempDir()
	user, _ := store.Open(store.ScopeUser, cwd)
	_ = user.Set("statusLine", map[string]any{"type": "command", "command": "badge.sh"})
	files := map[store.Scope]*store.File{store.ScopeUser: user}
	for _, sc := range []store.Scope{store.ScopeProject, store.ScopeLocal} {
		files[sc], _ = store.Open(sc, cwd)
	}
	if got, from := Existing(files, store.ScopeProject, cwd); got != "badge.sh" || from != store.ScopeUser {
		t.Fatalf("got %q from %s", got, from)
	}

	c := Default()
	if err := Enable(user, c, Command(store.ScopeUser, cwd), ""); err != nil {
		t.Fatal(err)
	}
	if err := Save(ConfigPath(store.ScopeUser, cwd), c); err != nil {
		t.Fatal(err)
	}
	for _, sc := range []store.Scope{store.ScopeUser, store.ScopeProject} {
		if got, _ := Existing(files, sc, cwd); got != "badge.sh" {
			t.Fatalf("%s: got %q, want the command ccfg kept", sc, got)
		}
	}
}

func TestConfigRoundTripAndOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "c.json")
	c := Default()
	c.Toggle("model")
	c.Toggle("dir")
	c.Toggle("branch")
	if !c.Move("branch", -1) || !c.Move("branch", -1) || c.Move("branch", -1) || strings.Join(c.Fields, ",") != "branch,model,dir" {
		t.Fatalf("fields = %v", c.Fields)
	}
	if c.Toggle("model") || c.Has("model") {
		t.Fatal("toggle did not hide model")
	}
	c.Existing = "sideways"
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || strings.Join(got.Fields, ",") != "branch,dir" || got.Existing != ExistingStart {
		t.Fatalf("loaded %+v, %v", got, err)
	}
}

func TestGitBranch(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	head := filepath.Join(root, ".git", "HEAD")
	_ = os.WriteFile(head, []byte("ref: refs/heads/feat/x\n"), 0o644)
	if got := gitBranch(sub); got != "feat/x" {
		t.Fatalf("branch = %q", got)
	}
	_ = os.WriteFile(head, []byte("0123456789abcdef\n"), 0o644)
	if got := gitBranch(sub); got != "0123456" {
		t.Fatalf("detached = %q", got)
	}
	// A linked worktree's .git is a file pointing at its own directory.
	wt := t.TempDir()
	_ = os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+filepath.Join(root, ".git")+"\n"), 0o644)
	if got := gitBranch(wt); got != "0123456" {
		t.Fatalf("worktree = %q", got)
	}
	if got := gitBranch(t.TempDir()); got != "" {
		t.Fatalf("outside a repository = %q", got)
	}
}

func TestShellPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for in, want := range map[string]string{
		filepath.Join(home, ".local/bin/ccfg"): "~/.local/bin/ccfg",
		"/usr/local/bin/ccfg":                  "/usr/local/bin/ccfg",
		"/opt/my tools/it's":                   `'/opt/my tools/it'\''s'`,
		filepath.Join(home, "my apps/ccfg"):    "'" + filepath.ToSlash(home) + "/my apps/ccfg'",
	} {
		if got := shellPath(in); got != want {
			t.Errorf("shellPath(%q) = %q, want %q", in, got, want)
		}
	}
}
