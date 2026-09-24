package tui

import (
	"encoding/json"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func settlePalette(t *testing.T, m *model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			settlePalette(t, m, c)
		}
		return
	}
	_, next := m.Update(msg)
	settlePalette(t, m, next)
}

func TestCabinetNavigationAndCancel(t *testing.T) {
	m := envModel(t)
	selectKey(t, m, "sandbox.network.allowedDomains")
	before := m.selected().Key
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	settlePalette(t, m, cmd)
	if m.mode != modePalette {
		t.Fatal("ctrl+k did not open the cabinet")
	}
	for _, width := range []int{80, 120} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		v := m.View().Content
		if lipgloss.Width(v) > width || lipgloss.Height(v) > 24 {
			t.Fatalf("cabinet exceeds %dx24", width)
		}
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.mode != modeBrowse || m.selected().Key != before {
		t.Fatal("cancel did not preserve navigation")
	}
	settlePalette(t, m, m.openPalette())
	// Surprise me is followed by the first section.
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	settlePalette(t, m, cmd)
	if m.mode != modeBrowse || m.section() != m.sections[0] || m.sideFocus {
		t.Fatal("choosing a section did not focus its settings")
	}
}

func TestSurpriseMeOnlyNavigates(t *testing.T) {
	m := envModel(t)
	before, _ := json.Marshal(m.files)
	selectKey(t, m, "model")
	settlePalette(t, m, m.openPalette())
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	settlePalette(t, m, cmd)
	if m.mode != modeBrowse || m.selected().Key == "model" {
		t.Fatal("surprise me did not reveal a different setting")
	}
	for range 30 {
		previous := m.selected().Key
		m.startSearch()
		m.discoverSetting()
		if m.selected().Key == previous || m.selected().Deprecated != "" || m.onGroup() || m.searching {
			t.Fatal("discovery did not leave search and reveal a new editable setting")
		}
	}
	after, _ := json.Marshal(m.files)
	if string(before) != string(after) {
		t.Fatal("discovery changed settings")
	}
}
