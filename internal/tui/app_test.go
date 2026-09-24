package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
		for _, text := range []string{"Move", "Change", "switch the target file", "any key close"} {
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
