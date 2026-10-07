package tui

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jorden-parker/claude-config-cli/internal/statusline"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

// statusModel starts with a status line already set in the user file, the
// way a plugin or another tool leaves one.
func statusModel(t *testing.T, existing string) (*model, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if existing != "" {
		if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
		body := `{"statusLine":{"type":"command","command":"` + existing + `","padding":2}}`
		if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	check := statusCheck
	statusCheck = func(string, string) error { return nil }
	t.Cleanup(func() { statusCheck = check })
	m := newModel(t.TempDir())
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 50})
	return m, home
}

// run delivers the messages a command produces, as the program would.
func run(m *model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			run(m, c)
		}
	case statusPreviewMsg, statusActivateMsg:
		_, next := m.Update(msg)
		run(m, next)
	}
}

// pressTabAndRun presses tab and delivers what the key starts, so a turn-on
// finishes the way it does in the program.
func pressTabAndRun(m *model) {
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	run(m, cmd)
}

func TestStatusLineKeepsExistingCommandAndPreviewsIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	m, _ := statusModel(t, "printf [CAVEMAN]")
	run(m, m.Init())
	selectKey(t, m, slUse)

	doc := ansi.Strip(m.doc.GetContent())
	if !strings.Contains(doc, "[CAVEMAN]") || !strings.Contains(doc, "○ not in use") {
		t.Fatalf("preview should show the existing status line before anything is saved:\n%s", doc)
	}

	selectKey(t, m, slField+"model")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	run(m, cmd)
	if doc = ansi.Strip(m.doc.GetContent()); !strings.Contains(doc, "[CAVEMAN] Opus") {
		t.Fatalf("preview should join the existing line and the new field:\n%s", doc)
	}
	if v, _ := m.files[store.ScopeUser].Get("statusLine"); statusline.CommandOf(v) != "printf [CAVEMAN]" {
		t.Fatalf("picking a field changed settings before it was turned on: %v", v)
	}

	selectKey(t, m, slUse)
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	run(m, cmd)
	saved, _ := store.Open(store.ScopeUser, m.cwd)
	v, _ := saved.Get("statusLine")
	obj := v.(map[string]any)
	if !statusline.IsOurs(statusline.CommandOf(v)) || obj["padding"] != float64(2) {
		t.Fatalf("statusLine on disk = %v", v)
	}
	cfg, err := statusline.Load(statusline.ConfigPath(store.ScopeUser, m.cwd))
	if err != nil || cfg.Inherited != "printf [CAVEMAN]" || !cfg.Has("model") {
		t.Fatalf("config = %+v, %v", cfg, err)
	}
	if got := statusline.Render(statusline.ConfigPath(store.ScopeUser, m.cwd), []byte(`{"model":{"display_name":"Opus"}}`)); ansi.Strip(got) != "[CAVEMAN] Opus" {
		t.Fatalf("rendered %q", got)
	}
	if doc = ansi.Strip(m.doc.GetContent()); !strings.Contains(doc, "● in use") || !strings.Contains(doc, "[CAVEMAN] Opus") {
		t.Fatalf("doc after turning on:\n%s", doc)
	}

	// Turning it off puts the original value back exactly.
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	run(m, cmd)
	saved, _ = store.Open(store.ScopeUser, m.cwd)
	if v, _ := saved.Get("statusLine"); store.Format(v) != `{"command":"printf [CAVEMAN]","padding":2,"type":"command"}` {
		t.Fatalf("restored %s", store.Format(v))
	}
}

func TestStatusLineFieldsToggleReorderAndPlace(t *testing.T) {
	m, _ := statusModel(t, "true")
	for _, id := range []string{"model", "dir", "cost"} {
		selectKey(t, m, slField+id)
		pressTab(m)
		if st := m.selected(); st == nil || st.Key != slField+id {
			t.Fatalf("cursor left %s after turning it on", id)
		}
	}
	m.Update(tea.KeyPressMsg{Code: 'K', Text: "K"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift})
	if got := strings.Join(m.sl.cfg.Fields, ","); got != "cost,model,dir" {
		t.Fatalf("fields = %s", got)
	}
	m.Update(tea.KeyPressMsg{Code: 'J', Text: "J"})
	pressTab(m) // hide cost again
	cfg, _ := statusline.Load(m.statusPath())
	if got := strings.Join(cfg.Fields, ","); got != "model,dir" {
		t.Fatalf("saved fields = %s", got)
	}
	if n := m.sectionCount(statusSection); n != 2 {
		t.Fatalf("section count = %d", n)
	}

	selectKey(t, m, slExisting)
	for _, want := range []string{"end", "above", "below", "hidden", "start"} {
		pressTab(m)
		if m.sl.cfg.Existing != want {
			t.Fatalf("existing = %s, want %s", m.sl.cfg.Existing, want)
		}
	}
	selectKey(t, m, slSeparator)
	pressTab(m)
	selectKey(t, m, slColor)
	pressTab(m)
	if cfg, _ = statusline.Load(m.statusPath()); cfg.Separator != " | " || cfg.Color {
		t.Fatalf("config = %+v", cfg)
	}

	// u and reordering a hidden field explain themselves instead of acting.
	selectKey(t, m, slField+"vim")
	m.Update(tea.KeyPressMsg{Code: 'J', Text: "J"})
	m.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	if m.mode != modeBrowse || m.sl.cfg.Has("vim") {
		t.Fatalf("mode=%v fields=%v", m.mode, m.sl.cfg.Fields)
	}
}

func TestStatusLineWithNothingSetAndPerScopeConfig(t *testing.T) {
	m, home := statusModel(t, "")
	selectKey(t, m, slExisting)
	pressTab(m)
	if m.sl.cfg.Existing != statusline.ExistingStart || !strings.Contains(m.status, "nothing to place") {
		t.Fatalf("existing=%s status=%q", m.sl.cfg.Existing, m.status)
	}
	if doc := ansi.Strip(m.doc.GetContent()); !strings.Contains(doc, "none set") || !strings.Contains(doc, "Nothing to show yet") {
		t.Fatalf("doc:\n%s", doc)
	}

	// A new status line starts with the default fields.
	selectKey(t, m, slUse)
	pressTabAndRun(m)
	if got := strings.Join(m.sl.cfg.Fields, ","); got != strings.Join(statusline.DefaultFields, ",") {
		t.Fatalf("fields = %s", got)
	}
	if !statusline.On(m.files[store.ScopeUser]) {
		t.Fatal("not turned on")
	}

	// The project file has its own config and keeps the user's line going.
	m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	if m.scope != store.ScopeProject || len(m.sl.cfg.Fields) != 0 {
		t.Fatalf("scope=%s fields=%v", m.scope, m.sl.cfg.Fields)
	}
	selectKey(t, m, slField+"vim")
	pressTab(m)
	if _, err := os.Stat(filepath.Join(m.cwd, ".claude", "ccfg-statusline.json")); err != nil {
		t.Fatal(err)
	}
	user, _ := statusline.Load(filepath.Join(home, ".claude", "ccfg-statusline.json"))
	if user.Has("vim") {
		t.Fatal("project edit changed the user config")
	}
}

// A project's settings can come from a repository, so its command is not
// run until asked for.
func TestStatusLineDoesNotAutoRunProjectCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()
	marker := filepath.Join(cwd, "ran")
	if err := os.MkdirAll(filepath.Join(cwd, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"statusLine":{"type":"command","command":"touch ` + marker + `; echo PROJECT"}}`
	if err := os.WriteFile(filepath.Join(cwd, ".claude", "settings.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newModel(cwd)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 50})
	run(m, m.Init())
	selectKey(t, m, slUse)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a project command ran without being asked")
	}
	if doc := ansi.Strip(m.doc.GetContent()); !strings.Contains(doc, "Press p to run it") {
		t.Fatalf("doc:\n%s", doc)
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	run(m, cmd)
	if doc := ansi.Strip(m.doc.GetContent()); !strings.Contains(doc, "PROJECT") {
		t.Fatalf("doc after p:\n%s", doc)
	}
}

func TestStatusLineIsSearchableAndRendersAtEverySize(t *testing.T) {
	m, _ := statusModel(t, "")
	for _, li := range m.searchItems() {
		if it := li.(item); it.header {
			t.Fatal("search results include a heading row")
		}
	}
	selectKey(t, m, slField+"rate_5h")
	for _, size := range [][2]int{{140, 50}, {80, 24}, {50, 20}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := ansi.Strip(m.View().Content)
		if !strings.Contains(view, "5-hour limit") {
			t.Fatalf("%v: row missing:\n%s", size, view)
		}
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatalf("%v: line wider than the terminal: %q", size, line)
			}
		}
	}
}

// A renderer that cannot run would blank the whole line, so it is not saved.
func TestStatusLineRefusesACommandThatDoesNotRun(t *testing.T) {
	m, _ := statusModel(t, "printf kept")
	statusCheck = func(string, string) error { return os.ErrNotExist }
	selectKey(t, m, slUse)
	pressTabAndRun(m)
	saved, _ := store.Open(store.ScopeUser, m.cwd)
	if v, _ := saved.Get("statusLine"); statusline.CommandOf(v) != "printf kept" || !m.statusErr || statusline.On(m.files[store.ScopeUser]) {
		t.Fatalf("statusLine = %v, status = %q", v, m.status)
	}
}

// An activation check that is still running must not freeze the editor, change
// the target underneath itself, or be applied after it was cancelled.
func TestStatusActivationPending(t *testing.T) {
	m, home := statusModel(t, "printf kept")
	started, release := make(chan struct{}, 4), make(chan struct{})
	statusCheck = func(string, string) error {
		started <- struct{}{}
		<-release
		return nil
	}
	settings := filepath.Join(home, ".claude", "settings.json")
	before, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	companion := statusline.ConfigPath(store.ScopeUser, m.cwd)
	wait := func(c <-chan tea.Msg) tea.Msg {
		t.Helper()
		select {
		case msg := <-c:
			return msg
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for the check")
			return nil
		}
	}
	start := func() <-chan tea.Msg {
		t.Helper()
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		if cmd == nil || m.mode != modeActivating || !strings.Contains(m.status, "Esc cancels") {
			t.Fatalf("mode=%v status=%q", m.mode, m.status)
		}
		out := make(chan tea.Msg, 1)
		go func() { out <- cmd() }() // the check runs here, never touching the model
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("check never started")
		}
		return out
	}

	selectKey(t, m, slUse)
	first := start()

	// Update still returns while the check is blocked.
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if m.width != 100 || m.height != 40 {
		t.Fatalf("resize ignored: %dx%d", m.width, m.height)
	}
	// Mutations, scope changes, and reloads wait.
	for _, k := range []tea.KeyPressMsg{{Code: tea.KeyTab}, {Code: tea.KeyEnter}, {Code: 's', Text: "s"}, {Code: 'r', Text: "r"}, {Code: 'J', Text: "J"}} {
		m.Update(k)
		if m.mode != modeActivating || m.scope != store.ScopeUser || m.sl.cfg.Has("model") {
			t.Fatalf("%v changed state: mode=%v scope=%s", k, m.mode, m.scope)
		}
	}
	if _, quit := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); quit == nil {
		t.Fatal("q should still quit")
	}

	// Esc cancels, and the late result changes nothing.
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.mode != modeBrowse || !strings.Contains(m.status, "Cancelled") {
		t.Fatalf("mode=%v status=%q", m.mode, m.status)
	}
	close(release)
	late := wait(first)
	if _, next := m.Update(late); next != nil {
		t.Fatal("a cancelled activation produced work")
	}
	if after, _ := os.ReadFile(settings); !bytes.Equal(before, after) {
		t.Fatalf("settings changed:\n%s", after)
	}
	if _, err := os.Stat(companion); err == nil {
		t.Fatal("companion written for a cancelled activation")
	}
	if statusline.On(m.files[store.ScopeUser]) {
		t.Fatal("cancelled activation went live")
	}

	// A stale result cannot finish a newer request. Only its own result does.
	second := start()
	if m.mode != modeActivating {
		t.Fatal("second request not pending")
	}
	m.Update(late)
	if m.mode != modeActivating || statusline.On(m.files[store.ScopeUser]) {
		t.Fatal("a stale result finished a newer request")
	}
	run(m, func() tea.Msg { return wait(second) })
	if m.mode != modeBrowse || !statusline.On(m.files[store.ScopeUser]) {
		t.Fatalf("mode=%v status=%q", m.mode, m.status)
	}
	if _, err := os.Stat(companion); err != nil {
		t.Fatal(err)
	}
}

// realHome is read before any test points HOME at a temporary directory, so
// the build below keeps using the real Go caches.
var realHome, _ = os.UserHomeDir()

// buildCLI builds the real binary into a temporary directory and puts it first
// on PATH, which is how project settings find ccfg.
func buildCLI(t *testing.T) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "ccfg")
	build := exec.Command("go", "build", "-o", bin, "./cmd/ccfg")
	build.Dir = filepath.Join("..", "..")
	build.Env = append(os.Environ(), "HOME="+realHome)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	t.Setenv("PATH", filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// consentModel opens an editor whose settings files hold the given statusLine
// commands, with the production renderer check and the real binary.
func consentModel(t *testing.T, user, project string) (*model, string) {
	t.Helper()
	home, cwd := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for dir, command := range map[string]string{filepath.Join(home, ".claude"): user, filepath.Join(cwd, ".claude"): project} {
		if command == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := `{"statusLine":{"type":"command","command":"` + command + `"}}`
		if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	check := statusCheck
	statusCheck = statusline.Check
	t.Cleanup(func() { statusCheck = check })
	return reopen(cwd), cwd
}

// reopen loads the settings from disk into a new editor, the way a restart does.
func reopen(cwd string) *model {
	m := newModel(cwd)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 50})
	run(m, m.Init())
	return m
}

func TestStatusActivationPreservesConsent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	buildCLI(t)
	markers := t.TempDir()
	ran := func(name string) bool {
		_, err := os.Stat(filepath.Join(markers, name))
		return err == nil
	}
	touch := func(name, echo string) string {
		return "touch '" + filepath.Join(markers, name) + "'; echo " + echo
	}
	notRun := func(m *model) {
		t.Helper()
		selectKey(t, m, slUse)
		if doc := ansi.Strip(m.doc.GetContent()); !strings.Contains(doc, "Press p to run it") {
			t.Fatalf("doc should ask for p:\n%s", doc)
		}
	}

	t.Run("project scope", func(t *testing.T) {
		m, cwd := consentModel(t, "", touch("project", "PROJECT"))
		m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
		if m.scope != store.ScopeProject {
			t.Fatalf("scope = %s", m.scope)
		}
		selectKey(t, m, slUse)
		pressTabAndRun(m)
		if !statusline.On(m.files[store.ScopeProject]) || m.statusErr {
			t.Fatalf("not activated: %q", m.status)
		}
		if ran("project") {
			t.Fatal("activation ran the project command")
		}
		m = reopen(cwd)
		if ran("project") {
			t.Fatal("reloading ran the project command")
		}
		notRun(m)
		_, cmd := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
		run(m, cmd)
		if !ran("project") || !strings.Contains(ansi.Strip(m.doc.GetContent()), "PROJECT") {
			t.Fatal("p should run it")
		}
	})

	t.Run("user scope inheriting a project command", func(t *testing.T) {
		m, cwd := consentModel(t, "", touch("inherited", "PROJECT"))
		notRun(m)
		pressTabAndRun(m)
		if !statusline.On(m.files[store.ScopeUser]) || m.statusErr {
			t.Fatalf("not activated: %q", m.status)
		}
		if ran("inherited") {
			t.Fatal("activation ran the inherited project command")
		}
		// Turning it on again checks against a companion that already holds the
		// command, which a normal render would run.
		pressTabAndRun(m)
		if statusline.On(m.files[store.ScopeUser]) {
			t.Fatal("not turned off")
		}
		pressTabAndRun(m)
		if !statusline.On(m.files[store.ScopeUser]) || m.statusErr || ran("inherited") {
			t.Fatalf("second activation: %q ran=%v", m.status, ran("inherited"))
		}
		m = reopen(cwd)
		if ran("inherited") {
			t.Fatal("reloading ran the inherited project command")
		}
		notRun(m)
		// Claude Code itself runs the saved command as usual.
		input := []byte(`{"model":{"display_name":"Opus"}}`)
		if got := statusline.Render(statusline.ConfigPath(store.ScopeUser, cwd), input); !strings.Contains(got, "PROJECT") || !ran("inherited") {
			t.Fatalf("render = %q", got)
		}
	})

	t.Run("hidden output stays unrun", func(t *testing.T) {
		m, cwd := consentModel(t, "", touch("hidden", "PROJECT"))
		selectKey(t, m, slExisting)
		for m.sl.cfg.Existing != statusline.ExistingHidden {
			pressTab(m)
		}
		selectKey(t, m, slUse)
		pressTabAndRun(m)
		m = reopen(cwd)
		selectKey(t, m, slUse)
		_, cmd := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
		run(m, cmd)
		if ran("hidden") {
			t.Fatal("a hidden command was run")
		}
	})

	t.Run("user commands still preview on their own", func(t *testing.T) {
		m, cwd := consentModel(t, touch("user", "USERCMD"), "")
		selectKey(t, m, slUse)
		if !ran("user") || !strings.Contains(ansi.Strip(m.doc.GetContent()), "USERCMD") {
			t.Fatal("a user command should preview without being asked")
		}
		if err := os.Remove(filepath.Join(markers, "user")); err != nil {
			t.Fatal(err)
		}
		selectKey(t, m, slUse)
		pressTabAndRun(m)
		if !statusline.On(m.files[store.ScopeUser]) || m.statusErr {
			t.Fatalf("not activated: %q", m.status)
		}
		if ran("user") {
			t.Fatal("activation ran the user command")
		}
		m = reopen(cwd)
		if !ran("user") {
			t.Fatal("the user origin was lost, so the preview did not run on reload")
		}
	})
}
