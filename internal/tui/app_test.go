package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func selectKey(t *testing.T, m *model, key string) {
	t.Helper()
	for i, it := range m.list.Items() {
		if it.(item).st.Key == key {
			m.list.Select(i)
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
