package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

func selectKey(t *testing.T, m *model, key string) {
	t.Helper()
	m.reveal(key)
	if st := m.selected(); st == nil || st.Key != key {
		t.Fatalf("no setting %s", key)
	}
}

func pressTab(m *model) { m.Update(tea.KeyPressMsg{Code: tea.KeyTab}) }

func TestTabCyclesSingleValueSettings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())

	selectKey(t, m, "effortLevel")
	for _, want := range []string{"low", "medium", "high", "xhigh", "low"} {
		pressTab(m)
		if got, _ := m.files["user"].Get("effortLevel"); got != want {
			t.Fatalf("effortLevel = %v, want %s", got, want)
		}
	}

	selectKey(t, m, "alwaysThinkingEnabled")
	for _, want := range []bool{true, false, true} {
		pressTab(m)
		if got, _ := m.files["user"].Get("alwaysThinkingEnabled"); got != want {
			t.Fatalf("alwaysThinkingEnabled = %v, want %v", got, want)
		}
	}
}

func TestTabLeavesMultiValueSettingsAlone(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())

	selectKey(t, m, "env")
	pressTab(m)
	if _, ok := m.files["user"].Get("env"); ok || m.mode != modeBrowse || !m.statusErr {
		t.Fatalf("tab changed a multi-value setting: mode=%v status=%q", m.mode, m.status)
	}
	if m.scope != "user" {
		t.Fatalf("tab changed the target file to %s", m.scope)
	}
}

func TestEachSectionIsItsOwnPane(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())
	selectKey(t, m, "effortLevel")
	for _, li := range m.list.Items() {
		if it := li.(item); !it.header && it.st.Section != "Model and responses" {
			t.Fatalf("%s from %s in the Model pane", it.st.Key, it.st.Section)
		}
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if !m.sideFocus {
		t.Fatal("left did not focus the sections")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.section() != "Permission settings" || m.selected().Section != "Permission settings" {
		t.Fatalf("down opened %s", m.section())
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.sideFocus {
		t.Fatal("right did not return to the keys")
	}
	m.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	if m.section() != "Sandbox settings" {
		t.Fatalf("] opened %s", m.section())
	}

	selectKey(t, m, "NotebookEdit")
	if m.section() != toolsSection {
		t.Fatal("tools are not in the Tools pane")
	}
	pressTab(m)
	for _, width := range []int{80, 119, 120, 160} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 40})
		m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
		view := ansi.Strip(m.View().Content)
		for _, text := range []string{"Sections", "Model", "Sandbox", "Reloaded settings files", "tab turn on/off", "? all keys", "q quit"} {
			if !strings.Contains(view, text) {
				t.Fatalf("width %d: missing %q\n%s", width, text, view)
			}
		}
		if strings.Contains(view, "effortLevel") {
			t.Fatalf("width %d: Model keys visible in the Tools pane", width)
		}
		if h, w := lipgloss.Height(view), lipgloss.Width(view); h > 40 || w > width {
			t.Fatalf("width %d: view is %dx%d\n%s", width, w, h, view)
		}
	}
}

func TestKeysOverlayOpensAndCloses(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())
	for _, width := range []int{80, 120} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 40})
		m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
		if m.mode != modeHelp {
			t.Fatal("? did not open the keys overlay")
		}
		view := ansi.Strip(m.View().Content)
		for _, text := range []string{"Move", "Change", "switch the target file", "esc close"} {
			if !strings.Contains(view, text) {
				t.Fatalf("width %d: overlay missing %q\n%s", width, text, view)
			}
		}
		if h, w := lipgloss.Height(view), lipgloss.Width(view); h > 40 || w > width {
			t.Fatalf("width %d: overlay is %dx%d", width, w, h)
		}
		m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
		if m.mode != modeBrowse || m.scope != "user" {
			t.Fatal("closing the overlay ran another command")
		}
	}
}

func TestSettingRowShowsValueAndFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	selectKey(t, m, "effortLevel")
	pressTab(m)
	view := ansi.Strip(m.View().Content)
	if !regexp.MustCompile(`effortLevel\s+"low" user`).MatchString(view) {
		t.Fatalf("row does not show value and file\n%s", view)
	}
	if !regexp.MustCompile(`● user\s+"low"\s+◂ target file`).MatchString(view) {
		t.Fatalf("details do not show where the value is set\n%s", view)
	}
}

func TestEscCancelsEdit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	selectKey(t, m, "model")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != modeEdit {
		t.Fatal("enter did not open the editor")
	}
	if w := lipgloss.Width(m.View().Content); w > 100 {
		t.Fatalf("edit view is %d wide", w)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.mode != modeBrowse {
		t.Fatal("esc did not cancel the edit")
	}
	if _, ok := m.files["user"].Get("model"); ok {
		t.Fatal("cancelled edit saved a value")
	}
}

func TestEveryScreenFitsA24RowTerminal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	check := func(name string) {
		t.Helper()
		v := m.View().Content
		if h, w := lipgloss.Height(v), lipgloss.Width(v); h > 24 || w > 80 {
			t.Fatalf("%s is %dx%d\n%s", name, w, h, ansi.Strip(v))
		}
	}
	check("settings")
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	check("sections focused")
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	check("keys overlay")
	m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	selectKey(t, m, "effortLevel")
	pressTab(m)
	m.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	check("confirm remove")
	m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	selectKey(t, m, "model")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	check("edit")
}

func TestNestedKeysLiveInsideGroups(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	for _, li := range m.list.Items() {
		if it := li.(item); !it.header && strings.Contains(it.st.Key, ".") {
			t.Fatalf("nested key %s on the top level", it.st.Key)
		}
	}
	if it, _ := m.selectedItem(); it.header {
		t.Fatal("cursor starts on a section heading")
	}
	for i := 0; i < 40; i++ {
		m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		if it, _ := m.selectedItem(); it.header {
			t.Fatal("cursor landed on a section heading")
		}
	}

	selectKey(t, m, "permissions")
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.levelPrefix() != "permissions" || m.mode != modeBrowse {
		t.Fatalf("enter did not open permissions: path=%v mode=%v", m.path, m.mode)
	}
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "Permissions › permissions") || strings.Contains(view, "permissions.allow") {
		t.Fatalf("permissions level shows dotted keys or no breadcrumb\n%s", view)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if len(m.path) != 0 || m.selected().Key != "permissions" {
		t.Fatal("esc did not return to the permissions row")
	}

	// Sandbox holds only the sandbox group, so its pane opens it directly.
	selectKey(t, m, "sandbox.enabled")
	if len(m.path) != 0 {
		t.Fatalf("sandbox pane not opened directly: path=%v", m.path)
	}
	selectKey(t, m, "sandbox.network.allowedDomains")
	if m.levelPrefix() != "sandbox.network" {
		t.Fatalf("path = %v", m.path)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.levelPrefix() != "sandbox" || len(m.path) != 0 || m.selected().Key != "sandbox.network" {
		t.Fatalf("left did not go back to the sandbox pane: path=%v", m.path)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if !m.sideFocus {
		t.Fatal("left at the top of a pane did not focus the sections")
	}
}

func TestSearchFindsNestedKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	if !m.searching {
		t.Fatal("/ did not start a search")
	}
	found := false
	for _, li := range m.list.Items() {
		if li.(item).st.Key == "sandbox.network.allowedDomains" {
			found = true
		}
	}
	if !found {
		t.Fatal("search set is missing nested keys")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.searching {
		t.Fatal("cancelling the search did not restore the level")
	}
	if it, _ := m.selectedItem(); it.header {
		t.Fatal("cursor on heading after search")
	}
}

// breakFile replaces a scope's settings file with a document that fails to
// load, then reloads the model so it holds the load error.
func breakFile(t *testing.T, m *model, sc store.Scope, fixture string) string {
	t.Helper()
	p := store.Path(sc, m.cwd)
	if err := os.RemoveAll(p); err != nil {
		t.Fatal(err)
	}
	if fixture == "dir" {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(fixture), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	m.reload()
	if m.errs[sc] == nil {
		t.Fatalf("%s fixture %q loaded without an error", sc, fixture)
	}
	return p
}

// diskState captures a path's bytes, or the error from reading it.
func diskState(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return "error: " + err.Error()
	}
	return string(b)
}

func TestOrdinaryEditsRejectLoadErrors(t *testing.T) {
	type target struct {
		name  string
		scope store.Scope
		key   string
	}
	user := target{"user", store.ScopeUser, "effortLevel"}
	global := target{"global", store.ScopeGlobal, "copyOnSelect"}
	for _, fixture := range []string{`{"effortLevel": `, "null", "dir"} {
		for _, tg := range []target{user, global} {
			for _, action := range []string{"open", "cycle", "unset"} {
				t.Run(tg.name+"/"+fixture+"/"+action, func(t *testing.T) {
					t.Setenv("HOME", t.TempDir())
					m := newModel(t.TempDir())
					m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
					selectKey(t, m, tg.key)
					p := breakFile(t, m, tg.scope, fixture)
					// The file shown is the one the selected setting writes to.
					m.files[tg.scope].Data[tg.key] = "seeded"
					disk, data := diskState(p), store.Format(m.files[tg.scope].Data)

					switch action {
					case "open":
						m.startEdit()
					case "cycle":
						m.cycleValue()
					case "unset":
						m.doUnset()
					}
					if m.mode != modeBrowse || m.edit != nil {
						t.Fatalf("mode = %v, edit open = %v", m.mode, m.edit != nil)
					}
					if !m.statusErr || !strings.Contains(m.status, "failed to load") {
						t.Fatalf("status = %q (error %v)", m.status, m.statusErr)
					}
					if got := diskState(p); got != disk {
						t.Fatalf("disk changed:\n%s\nwas:\n%s", got, disk)
					}
					if got := store.Format(m.files[tg.scope].Data); got != data {
						t.Fatalf("model changed: %s, was %s", got, data)
					}
				})
			}
		}
	}

	// A form opened on a good file stays open while the file goes bad; the
	// submit itself must refuse.
	for _, fixture := range []string{`{"apiKeyHelper": `, "null", "dir"} {
		t.Run("submit/"+fixture, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			m := newModel(t.TempDir())
			m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
			selectKey(t, m, "apiKeyHelper")
			runEnvCommand(m, m.startEdit())
			if m.mode != modeEdit {
				t.Fatal("form did not open on a good file")
			}
			p := breakFile(t, m, store.ScopeUser, fixture)
			m.files[store.ScopeUser].Data["apiKeyHelper"] = "seeded"
			disk, data := diskState(p), store.Format(m.files[store.ScopeUser].Data)

			m.Update(tea.PasteMsg{Content: "new-helper"})
			_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			runEnvCommand(m, cmd)
			if m.mode != modeBrowse || m.edit != nil {
				t.Fatalf("form did not complete: mode = %v", m.mode)
			}
			if !m.statusErr || !strings.Contains(m.status, "failed to load") {
				t.Fatalf("status = %q (error %v)", m.status, m.statusErr)
			}
			if got := diskState(p); got != disk {
				t.Fatalf("disk changed:\n%s\nwas:\n%s", got, disk)
			}
			if got := store.Format(m.files[store.ScopeUser].Data); got != data {
				t.Fatalf("model changed: %s, was %s", got, data)
			}
		})
	}
}

func TestOrdinaryEditsResumeAfterReload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())
	selectKey(t, m, "effortLevel")
	p := breakFile(t, m, store.ScopeUser, `{"custom": `)
	broken := diskState(p)

	m.cycleValue()
	if !m.statusErr || diskState(p) != broken {
		t.Fatalf("broken file was not refused: status %q", m.status)
	}
	// Repairing the file on disk is not enough until the model reloads it.
	if err := os.WriteFile(p, []byte(`{"custom": {"keep": 1}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m.cycleValue()
	if !m.statusErr || m.errs[store.ScopeUser] == nil {
		t.Fatalf("load error cleared before reload: status %q", m.status)
	}
	if got := diskState(p); got != `{"custom": {"keep": 1}}` {
		t.Fatalf("disk changed before reload: %s", got)
	}

	m.reload()
	if m.errs[store.ScopeUser] != nil {
		t.Fatalf("reload kept the error: %v", m.errs[store.ScopeUser])
	}
	m.cycleValue()
	if m.statusErr {
		t.Fatalf("edit after reload failed: %s", m.status)
	}
	f, err := store.Open(store.ScopeUser, m.cwd)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := f.Get("effortLevel"); got != "low" {
		t.Fatalf("effortLevel = %v", got)
	}
	if got, _ := f.Get("custom.keep"); got != 1.0 {
		t.Fatalf("sibling key lost: custom.keep = %v", got)
	}
}

// searchTargets starts a search and returns the list's filter values.
func searchTargets(t *testing.T, m *model) []string {
	t.Helper()
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	if !m.searching {
		t.Fatal("/ did not start a search")
	}
	var targets []string
	for _, li := range m.list.Items() {
		targets = append(targets, li.FilterValue())
	}
	return targets
}

func TestSearchRanksNameHitsFirst(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())
	targets := searchTargets(t, m)
	items := m.list.Items()

	ranks := rankFilter("sandbox", targets)
	if len(ranks) == 0 {
		t.Fatal("no results for sandbox")
	}
	seenOther := false
	for _, r := range ranks {
		key := strings.ToLower(items[r.Index].(item).st.Key)
		if !strings.Contains(key, "sandbox") {
			seenOther = true
		} else if seenOther {
			t.Fatalf("%s ranked after a description-only hit", key)
		}
	}
	if !seenOther {
		t.Fatal("expected at least one description hit after the sandbox keys")
	}

	for _, r := range rankFilter("on", targets) {
		it := items[r.Index].(item)
		text := strings.ToLower(it.label() + " " + it.st.Key)
		if !strings.Contains(text, "on") {
			t.Fatalf("%s matched \"on\" only by description", it.st.Key)
		}
	}
}

func TestSearchHighlightsShownName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())
	targets := searchTargets(t, m)
	items := m.list.Items()
	for _, r := range rankFilter("sep", targets) {
		if it := items[r.Index].(item); it.name == "Separator" {
			if got, want := r.MatchedIndexes, []int{0, 1, 2}; !reflect.DeepEqual(got, want) {
				t.Fatalf("Separator matches = %v, want %v", got, want)
			}
			return
		}
	}
	t.Fatal("Separator row not found for sep")
}
