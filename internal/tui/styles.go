package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
)

type styles struct {
	title, subtle, key, kind, label, value, ok, warn, err, help, badge, badgeOn, sectionHdr lipgloss.Style
	pane, paneFocus                                                                         lipgloss.Style
}

func newStyles(isDark bool) styles {
	ld := lipgloss.LightDark(isDark)
	accent := ld(lipgloss.Color("#7D56F4"), lipgloss.Color("#B39DFF"))
	dim := ld(lipgloss.Color("#6b6b6b"), lipgloss.Color("#8a8a8a"))
	green := ld(lipgloss.Color("#1f8f4e"), lipgloss.Color("#5fd787"))
	yellow := ld(lipgloss.Color("#b26a00"), lipgloss.Color("#ffd75f"))
	red := ld(lipgloss.Color("#c62828"), lipgloss.Color("#ff5f5f"))
	fg := ld(lipgloss.Color("#1a1a1a"), lipgloss.Color("#e6e6e6"))
	return styles{
		title:      lipgloss.NewStyle().Bold(true).Foreground(accent),
		subtle:     lipgloss.NewStyle().Foreground(dim),
		key:        lipgloss.NewStyle().Bold(true).Foreground(fg),
		kind:       lipgloss.NewStyle().Foreground(accent),
		label:      lipgloss.NewStyle().Bold(true).Foreground(dim),
		value:      lipgloss.NewStyle().Foreground(fg),
		ok:         lipgloss.NewStyle().Foreground(green),
		warn:       lipgloss.NewStyle().Foreground(yellow),
		err:        lipgloss.NewStyle().Foreground(red),
		help:       lipgloss.NewStyle().Foreground(dim),
		badge:      lipgloss.NewStyle().Foreground(dim).Padding(0, 1),
		badgeOn:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff")).Background(accent).Padding(0, 1),
		sectionHdr: lipgloss.NewStyle().Bold(true).Foreground(accent).Underline(true),
		pane:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(dim).Padding(0, 1),
		paneFocus:  lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 1),
	}
}

// RenderDoc renders a setting's documentation block. Also used by `doc`.
func RenderDoc(st *schema.Setting, width int) string {
	s := newStyles(true)
	wrap := lipgloss.NewStyle().Width(width)
	var b strings.Builder
	b.WriteString(s.title.Render(st.Key) + "  " + s.kind.Render(string(st.Kind)) + "\n")
	b.WriteString(s.subtle.Render(st.Section) + "\n\n")
	if st.Deprecated != "" {
		b.WriteString(s.warn.Render(wrap.Render("⚠ "+st.Deprecated)) + "\n\n")
	}
	b.WriteString(wrap.Render(st.Desc) + "\n\n")
	rowWrap := lipgloss.NewStyle().Width(width - 9)
	row := func(k, v string) {
		if v == "" {
			return
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, s.label.Render(fmt.Sprintf("%-9s", k)), rowWrap.Render(v)) + "\n")
	}
	row("Scope", st.Scope)
	if st.ScopeNote != "" {
		row("", st.ScopeNote)
	}
	row("Type", st.Type)
	row("Default", st.Default)
	row("Hint", st.Hint)
	if len(st.Options) > 0 {
		b.WriteString(s.label.Render("Options") + "\n")
		for _, o := range st.Options {
			line := "  • " + s.value.Render(o)
			if h := st.OptionHelp[o]; h != "" {
				line += s.subtle.Render(" — " + h)
			}
			b.WriteString(wrap.Render(line) + "\n")
		}
	}
	if len(st.Suggestions) > 0 {
		row("Values", strings.Join(st.Suggestions, ", "))
	}
	if st.Example != "" {
		b.WriteString("\n" + s.label.Render("Example") + "\n" + s.subtle.Render(st.Example) + "\n")
	}
	b.WriteString("\n" + s.subtle.Render(st.DocURL()) + "\n")
	return b.String()
}
