package tui

import (
	"charm.land/lipgloss/v2"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
		view := m.View().Content
		for _, text := range []string{"Tools", "reloaded settings files", "←/→ pane", "tab toggle", "r reload", "q quit"} {
			if !strings.Contains(view, text) {
				t.Fatalf("width %d: missing %q", width, text)
			}
		}
		if strings.Contains(view, "effortLevel") || strings.Contains(view, "Settings") {
			t.Fatalf("width %d: settings visible while tools selected", width)
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
		settingsView := m.View().Content
		if !strings.Contains(settingsView, "effortLevel") || strings.Contains(settingsView, "NotebookEdit") {
			t.Fatalf("width %d: switching did not replace the visible list", width)
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
		if h := lipgloss.Height(view); h > 40 {
			t.Fatalf("width %d: height = %d; list %dx%d tools %dx%d doc %dx%d\n%s", width, h, lipgloss.Width(m.list.View()), lipgloss.Height(m.list.View()), lipgloss.Width(m.tools.View()), lipgloss.Height(m.tools.View()), lipgloss.Width(m.doc.View()), lipgloss.Height(m.doc.View()), view)
		}
		if w := lipgloss.Width(view); w > width {
			t.Fatalf("width %d: rendered width = %d", width, w)
		}
	}
}
