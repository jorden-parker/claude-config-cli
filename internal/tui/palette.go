package tui

import (
	"math/rand/v2"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

// The cabinet only navigates. Choosing a destination never changes a setting.
func (m *model) openPalette() tea.Cmd {
	m.destination = -1
	options := []huh.Option[int]{huh.NewOption("✧ Surprise me — discover a setting", -1)}
	for i, section := range m.sections {
		label := section
		if section == envSection {
			label = "✿ Environment — the variable garden"
		} else if section == toolsSection {
			label = "⚒ Tools — the toolbox"
		}
		options = append(options, huh.NewOption(label, i))
	}
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc", "back"))
	m.palette = huh.NewForm(huh.NewGroup(huh.NewSelect[int]().
		Title("Where shall we wander?").Description("↑/↓ to browse · / to filter · enter to explore").
		Options(options...).Value(&m.destination))).
		WithTheme(formTheme(m.isDark)).WithKeyMap(km).
		WithWidth(max(1, m.dialogWidth()-4)).WithHeight(max(1, m.height-editChrome))
	m.mode = modePalette
	return m.palette.Init()
}

func (m *model) updatePalette(msg tea.Msg) (tea.Model, tea.Cmd) {
	f, cmd := m.palette.Update(msg)
	if form, ok := f.(*huh.Form); ok {
		m.palette = form
	}
	switch m.palette.State {
	case huh.StateAborted:
		m.mode, m.palette = modeBrowse, nil
		return m, nil
	case huh.StateCompleted:
		if m.mode == modeScope {
			m.scope = m.scopeChoice
			m.mode, m.palette = modeBrowse, nil
			m.setStatus("Next tweaks go to the "+string(m.scope)+" file.", false)
			return m, m.afterWrite()
		}
		m.mode, m.palette = modeBrowse, nil
		m.sideFocus = false
		if m.destination >= 0 {
			return m, m.setSection(m.destination)
		}
		m.discoverSetting()
		return m, nil
	}
	return m, cmd
}

func (m *model) discoverSetting() {
	var candidates []string
	current := m.selected()
	for _, k := range m.sch.Keys() {
		st := m.sch.Get(k)
		if st.Deprecated == "" && !m.parents()[k] && (current == nil || current.Key != k) {
			candidates = append(candidates, k)
		}
	}
	if len(candidates) == 0 {
		return
	}
	m.searching = false
	m.list.ResetFilter()
	k := candidates[rand.IntN(len(candidates))]
	m.reveal(k)
	m.setStatus("Found a little gem: "+k+". Enter to edit; ctrl+k to wander again.", false)
}

// openScopePicker describes the impact of each destination before selecting it.
func (m *model) openScopePicker() tea.Cmd {
	m.scopeChoice = m.scope
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc", "back"))
	m.palette = huh.NewForm(huh.NewGroup(huh.NewSelect[store.Scope]().
		Title("Where should your settings live?").
		Description("Global-config keys always use ~/.claude.json.").
		Options(
			huh.NewOption("User · your defaults across projects", store.ScopeUser),
			huh.NewOption("Project · shared with this project's team", store.ScopeProject),
			huh.NewOption("Local · just you, in this project", store.ScopeLocal),
		).Value(&m.scopeChoice))).
		WithTheme(formTheme(m.isDark)).WithKeyMap(km).
		WithWidth(max(1, m.dialogWidth()-4)).WithHeight(max(1, m.height-editChrome))
	m.mode = modeScope
	return m.palette.Init()
}
