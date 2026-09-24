package tui

import "charm.land/lipgloss/v2"

// workshopHint gives the quiet status row a little personality and a useful
// next action. It stays still until the context changes; errors and save
// receipts always take priority.
func (m *model) workshopHint() string {
	switch {
	case m.details:
		return "✧ A closer look. PgUp/PgDn scroll · ctrl+d returns to settings."
	case m.mode == modeHelp:
		return "✧ The workshop field guide. ↑/↓ scroll · esc takes you back."
	case m.list.SettingFilter() || m.list.IsFiltered():
		return "✧ A little treasure hunt. Esc clears the search."
	case m.sideFocus:
		return "˙ᵕ˙ Pick a corner of the workshop. Enter opens it."
	case m.section() == envSection:
		return "✿ A little care for your variable garden. Enter edits a value."
	case m.section() == toolsSection:
		return "✧ Welcome to the toolbox. Tab turns a tool on or off."
	case m.onGroup():
		return "✧ A drawer full of possibilities. Enter opens this group."
	case m.width < 100:
		return "˙ᵕ˙ Enter edits · ctrl+d opens details · ctrl+k explores"
	default:
		return "˙ᵕ˙ Small tweaks, big possibilities. Enter edits · ? field guide"
	}
}

// dialog gives Huh forms a shared, bounded workbench surface.
func (m *model) dialogWidth() int { return max(1, min(88, m.width-4)) }

func (m *model) dialog(title, subtitle, content, help string) string {
	w := m.dialogWidth()
	inner := max(1, w-4)
	head := m.st.title.Render(title) + "\n" + m.st.subtle.Render(short(subtitle, inner))
	card := m.st.paneFocus.Width(w).Render(head + "\n\n" + content)
	body := card + "\n" + m.st.subtle.Render(short(help, w))
	return lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(m.height).Render(
		lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body))
}
