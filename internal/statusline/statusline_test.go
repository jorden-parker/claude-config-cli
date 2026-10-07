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

func TestComposeTrimsPaddingFromExistingStatusLine(t *testing.T) {
	input := []byte(`{"model":{"display_name":"Opus"}}`)
	for _, tc := range []struct{ pos, existing, want string }{
		{ExistingStart, "  a  \n", "a | Opus"},
		{ExistingEnd, "  a  \n", "Opus | a"},
		{ExistingAbove, " a\n", "a\nOpus"},
		{ExistingBelow, " a\r\n", "Opus\na"},
		// Padding inside colour codes goes; the codes stay.
		{ExistingStart, "\x1b[38;5;214m a \x1b[0m", "\x1b[38;5;214ma\x1b[0m | Opus"},
		{ExistingEnd, " \x1b[1m\x1b[31m a b\x1b[0m ", "Opus | \x1b[1m\x1b[31ma b\x1b[0m"},
		// Rows with nothing to see are dropped.
		{ExistingAbove, "a\n   \n\x1b[0m\nb", "a\nb\nOpus"},
		{ExistingStart, " \x1b[0m \n", "Opus"},
	} {
		c := plain("model")
		c.Existing = tc.pos
		if got := Compose(c, input, tc.existing, now); got != tc.want {
			t.Errorf("%s %q: got %q, want %q", tc.pos, tc.existing, got, tc.want)
		}
	}
}

func TestComposeSqueezesRepeatedSpaces(t *testing.T) {
	input := []byte(`{"model":{"display_name":"Opus"},"session_name":"a   b"}`)
	c := plain("model", "session")
	c.Separator = "  "
	for existing, want := range map[string]string{
		"":               "Opus a b",
		"[CAVEMAN]":      "[CAVEMAN] Opus a b",
		"x  y   z\nq  r": "x y z\nq r Opus a b",
		// Spaces either side of a colour code are one run.
		"\x1b[31mx \x1b[0m \x1b[32m y\x1b[0m": "\x1b[31mx \x1b[0m\x1b[32my\x1b[0m Opus a b",
	} {
		if got := Compose(c, input, existing, now); got != want {
			t.Errorf("%q: got %q, want %q", existing, got, want)
		}
	}
	// The existing line is squeezed even with no fields beside it.
	if got := Compose(plain(), input, "x  y", now); got != "x y" {
		t.Fatalf("got %q", got)
	}
	// A saved two-space separator loads as one space.
	path := filepath.Join(t.TempDir(), "c.json")
	if err := os.WriteFile(path, []byte(`{"separator":"  "}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if loaded, _ := Load(path); loaded.Separator != " " {
		t.Fatalf("separator = %q", loaded.Separator)
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
		if err := Enable(f, c, command, "", ""); err != nil {
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
	if err := Enable(f, c, command, "~/.claude/statusline.sh", store.ScopeUser); err != nil {
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
	if err := Enable(g, fresh, Command(store.ScopeLocal, cwd), "", ""); err != nil {
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
	if err := Enable(user, c, Command(store.ScopeUser, cwd), "", ""); err != nil {
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

func TestSaveAtomicIntegration(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "new", "c.json")
	if err := Save(path, Default()); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("new file mode: %v %v", fi, err)
	}

	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	c := Default()
	c.Toggle("model")
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v", fi.Mode().Perm())
	}
	got, err := Load(path)
	if err != nil || strings.Join(got.Fields, ",") != strings.Join(c.Fields, ",") {
		t.Fatalf("loaded %+v, %v", got, err)
	}

	bad := filepath.Join(dir, "isdir")
	if err := os.Mkdir(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Save(bad, Default()); err == nil {
		t.Fatal("expected error saving over a directory")
	}
	if fi, err := os.Stat(bad); err != nil || !fi.IsDir() {
		t.Fatalf("directory was disturbed: %v %v", fi, err)
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

// openWrapped opens a settings file for scope whose statusLine is ccfg's
// renderer, with companion as the text of its config file ("" writes none).
func openWrapped(t *testing.T, scope store.Scope, cwd, companion string) *store.File {
	t.Helper()
	f, err := store.Open(scope, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Set("statusLine", map[string]any{"type": "command", "command": Command(scope, cwd)}); err != nil {
		t.Fatal(err)
	}
	if companion != "" {
		path := ConfigPath(scope, cwd)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(companion), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func TestCheckRenderDoesNotRunInherited(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran")
	path := filepath.Join(dir, "ccfg-statusline.json")
	input := []byte(`{"model":{"display_name":"Opus"},"workspace":{"project_dir":"` + dir + `"}}`)
	c := plain("model")
	c.Inherited = "touch '" + marker + "'; echo INHERITED"
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	if err := CheckRender(path, input); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("CheckRender ran the inherited command")
	}
	// Check only ever runs ccfg's own renderer command.
	if err := Check("touch '"+marker+"'", dir); err == nil {
		t.Fatal("Check accepted a command that is not the renderer")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("Check ran a command that is not the renderer")
	}
	// A normal render does run it.
	if got := Render(path, input); !strings.Contains(got, "INHERITED") {
		t.Fatalf("render = %q", got)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("Render should run the inherited command")
	}
}

func TestCheckRenderRejectsInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ccfg-statusline.json")
	if err := CheckRender(path, nil); err != nil {
		t.Fatalf("a missing config is fine for a first activation: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CheckRender(path, nil); err == nil {
		t.Fatal("a malformed config was accepted")
	}
}

func TestInheritedScopeRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()
	user, _ := store.Open(store.ScopeUser, cwd)
	_ = user.Set("statusLine", map[string]any{"type": "command", "command": "badge.sh"})
	project, _ := store.Open(store.ScopeProject, cwd)
	files := map[store.Scope]*store.File{store.ScopeUser: user, store.ScopeProject: project}

	// Project settings inherit the user's command and remember where it came from.
	kept, from := Existing(files, store.ScopeProject, cwd)
	c := Default()
	if err := Enable(project, c, Command(store.ScopeProject, cwd), kept, from); err != nil {
		t.Fatal(err)
	}
	if c.InheritedScope != store.ScopeUser {
		t.Fatalf("scope = %q", c.InheritedScope)
	}
	// Enabling again keeps the captured origin.
	if err := Enable(project, c, Command(store.ScopeProject, cwd), "other.sh", store.ScopeProject); err != nil {
		t.Fatal(err)
	}
	if c.Inherited != "badge.sh" || c.InheritedScope != store.ScopeUser {
		t.Fatalf("re-enable changed %q from %q", c.Inherited, c.InheritedScope)
	}
	if err := Save(ConfigPath(store.ScopeProject, cwd), c); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(ConfigPath(store.ScopeProject, cwd))
	if err != nil || loaded.InheritedScope != store.ScopeUser {
		t.Fatalf("loaded %+v, %v", loaded, err)
	}
	// After a reload the project wrapper is still a project file's command.
	if got, from := Existing(files, store.ScopeProject, cwd); got != "badge.sh" || from != store.ScopeProject {
		t.Fatalf("after reload: %q from %q", got, from)
	}

	// A command set directly in the target file is that file's.
	direct, _ := store.Open(store.ScopeLocal, cwd)
	_ = direct.Set("statusLine", map[string]any{"type": "command", "command": "mine.sh"})
	d := Default()
	if err := Enable(direct, d, Command(store.ScopeLocal, cwd), "", ""); err != nil {
		t.Fatal(err)
	}
	if d.InheritedScope != store.ScopeLocal {
		t.Fatalf("direct scope = %q", d.InheritedScope)
	}
}

func TestWrappedCommandTrust(t *testing.T) {
	cases := []struct {
		name      string
		scope     store.Scope
		companion string
		want      store.Scope
		command   string
	}{
		{"user wrapper keeps user origin", store.ScopeUser, `{"inherited":"a.sh","inheritedScope":"user"}`, store.ScopeUser, "a.sh"},
		{"user wrapper keeps project origin", store.ScopeUser, `{"inherited":"a.sh","inheritedScope":"project"}`, store.ScopeProject, "a.sh"},
		{"legacy companion is unknown", store.ScopeUser, `{"inherited":"a.sh"}`, "", "a.sh"},
		{"invalid origin is unknown", store.ScopeUser, `{"inherited":"a.sh","inheritedScope":"global"}`, "", "a.sh"},
		{"malformed companion confers nothing", store.ScopeUser, `{not json`, "", ""},
		{"project wrapper cannot claim user", store.ScopeProject, `{"inherited":"a.sh","inheritedScope":"user"}`, store.ScopeProject, "a.sh"},
		{"local wrapper cannot claim managed", store.ScopeLocal, `{"inherited":"a.sh","inheritedScope":"managed"}`, store.ScopeLocal, "a.sh"},
		{"legacy project wrapper stays project", store.ScopeProject, `{"inherited":"a.sh"}`, store.ScopeProject, "a.sh"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			cwd := t.TempDir()
			f := openWrapped(t, tc.scope, cwd, tc.companion)
			files := map[store.Scope]*store.File{tc.scope: f}
			if got, from := Existing(files, tc.scope, cwd); got != tc.command || from != tc.want {
				t.Fatalf("got %q from %q, want %q from %q", got, from, tc.command, tc.want)
			}
		})
	}

	// A command set directly in the user file keeps its origin.
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()
	user, _ := store.Open(store.ScopeUser, cwd)
	_ = user.Set("statusLine", map[string]any{"type": "command", "command": "mine.sh"})
	if got, from := Existing(map[store.Scope]*store.File{store.ScopeUser: user}, store.ScopeProject, cwd); got != "mine.sh" || from != store.ScopeUser {
		t.Fatalf("direct: %q from %q", got, from)
	}
}
