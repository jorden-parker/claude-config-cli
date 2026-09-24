package tui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

func TestScopePickerChangesDestinationOnly(t *testing.T) {
	m := envModel(t)
	before, _ := json.Marshal(m.files)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'S', Text: "S"})
	settlePalette(t, m, cmd)
	if m.mode != modeScope {
		t.Fatal("Shift+S did not open destination picker")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	settlePalette(t, m, cmd)
	if m.scope != store.ScopeProject || m.mode != modeBrowse {
		t.Fatal("destination not applied")
	}
	after, _ := json.Marshal(m.files)
	if string(before) != string(after) {
		t.Fatal("choosing a destination wrote settings")
	}
	settlePalette(t, m, m.openScopePicker())
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.scope != store.ScopeProject {
		t.Fatal("cancel changed destination")
	}
}

func TestWorkshopResponsiveScreens(t *testing.T) {
	for _, size := range [][2]int{{32, 12}, {48, 20}, {80, 24}, {120, 32}, {160, 40}} {
		m := envModel(t)
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		check := func(name string) {
			t.Helper()
			v := m.View().Content
			if lipgloss.Width(v) > size[0] || lipgloss.Height(v) > size[1] {
				t.Fatalf("%s at %v overflows: %dx%d\n%s", name, size, lipgloss.Width(v), lipgloss.Height(v), ansi.Strip(v))
			}
		}
		check("browse")
		selectKey(t, m, "model")
		selected := m.selected().Key
		m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
		check("details")
		if !m.details || !strings.Contains(ansi.Strip(m.View().Content), "model") {
			t.Fatal("details not shown")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if m.details || m.selected().Key != selected {
			t.Fatal("back lost selection")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		check("edit")
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		settlePalette(t, m, m.openScopePicker())
		check("scope")
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		settlePalette(t, m, m.openPalette())
		check("cabinet")
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
		check("help")
	}
}
