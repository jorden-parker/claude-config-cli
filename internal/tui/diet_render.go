package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jorden-parker/claude-config-cli/internal/diet"
)

// dietGlyph is the mark for a state and the colour it is drawn in.
func dietGlyph(s styles, st diet.State) (string, lipgloss.Style) {
	switch st {
	case diet.Applied:
		return "✓", s.ok
	case diet.Warning:
		return "▲", s.warn
	case diet.Info:
		return "◇", s.subtle
	}
	return "○", s.accent
}

// dietLabel is the words of a row's badge: the state, and where it is applied.
func dietLabel(res diet.Result) string {
	label := res.State.String()
	if res.State == diet.Applied && res.Where() != "" {
		label += " · " + res.Where()
	}
	return label
}

// dietBadge is the glyph and label together, as the details pane shows them.
func dietBadge(s styles, res diet.Result) string {
	glyph, style := dietGlyph(s, res.State)
	return style.Render(glyph + " " + dietLabel(res))
}

// renderDietRow draws a checklist row: a glyph, the title, the state at the
// right, and the one-line why beneath it.
func renderDietRow(w io.Writer, s styles, width int, selected bool, it item) {
	res := it.diet.Result
	glyph, glyphStyle := dietGlyph(s, res.State)
	gutter, title := "  ", s.value
	if selected {
		gutter, title = s.accent.Render("❯ "), s.selection
	}
	badge := glyphStyle.Render(dietLabel(res))
	room := max(4, width-4-lipgloss.Width(badge)-1)
	name := ansi.Truncate(it.label(), room, "…")
	gap := max(1, width-4-lipgloss.Width(name)-lipgloss.Width(badge))
	fmt.Fprint(w, gutter+glyphStyle.Render(glyph)+" "+title.Render(name)+strings.Repeat(" ", gap)+badge)
	why := strings.Join(strings.Fields(it.diet.Rec.Why), " ")
	fmt.Fprint(w, "\n    "+s.subtle.Render(ansi.Truncate(why, max(1, width-4), "…")))
}
