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
	m.toolsFocus = false
	for _, st := range toolSettings {
		if st.Key == key {
			m.toolsFocus = true
		}
	}
	for i, it := range m.activeList().Items() {
		if it.(item).st.Key == key {
			m.activeList().Select(i)
			return
		}
	}
	t.Fatalf("no setting %s", key)
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

func TestSharedListColumnAndPersistentReloadHelp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())
	for _, it := range m.list.Items() {
		if toolName(it.(item).st) != "" {
			t.Fatal("tool in settings pane")
		}
	}
	selectKey(t, m, "effortLevel")
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if !m.toolsFocus || toolName(m.selected()) == "" {
		t.Fatal("right did not focus tools")
	}
	selectKey(t, m, "NotebookEdit")
	pressTab(m)
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.selected().Key != "effortLevel" {
		t.Fatal("settings selection lost")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.selected().Key != "NotebookEdit" {
		t.Fatal("tool selection lost")
	}
	for _, width := range []int{80, 119, 120, 160} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 40})
		m.Update(tea.KeyPressMsg{Code: 'r'})
		view := ansi.Strip(m.View().Content)
		for _, text := range []string{"Tools 46", "Reloaded settings files", "tab turn on/off", "? all keys", "q quit"} {
			if !strings.Contains(view, text) {
				t.Fatalf("width %d: missing %q\n%s", width, text, view)
			}
		}
		if strings.Contains(view, "effortLevel") {
			t.Fatalf("width %d: settings rows visible while tools selected", width)
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
		settingsView := ansi.Strip(m.View().Content)
		if !strings.Contains(settingsView, "effortLevel") || regexp.MustCompile(`NotebookEdit\s+(on|off)`).MatchString(settingsView) {
			t.Fatalf("width %d: switching did not replace the visible list", width)
		}
		for _, text := range []string{"enter edit", "tab next value", "? all keys"} {
			if !strings.Contains(settingsView, text) {
				t.Fatalf("width %d: settings footer missing %q\n%s", width, text, settingsView)
			}
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
		for _, v := range []string{view, settingsView} {
			if h := lipgloss.Height(v); h > 40 {
				t.Fatalf("width %d: height = %d\n%s", width, h, v)
			}
			if w := lipgloss.Width(v); w > width {
				t.Fatalf("width %d: rendered width = %d", width, w)
			}
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
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	check("tools")
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
