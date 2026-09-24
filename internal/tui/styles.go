package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
)

// palette: blue marks where you are and where writes go; amber marks values
// written in a file; everything else stays neutral so those two read first.
type palette struct {
	ink, dim, faint, accent, set, ok, warn, err color.Color
}

func newPalette(isDark bool) palette {
	ld := lipgloss.LightDark(isDark)
	return palette{
		ink:    lipgloss.NoColor{}, // terminal default text, readable before the background is known
		dim:    ld(lipgloss.Color("#6A727C"), lipgloss.Color("#7A828C")),
		faint:  ld(lipgloss.Color("#C4C9CF"), lipgloss.Color("#3A4048")),
		accent: ld(lipgloss.Color("#0B63C5"), lipgloss.Color("#6CB6FF")),
		set:    ld(lipgloss.Color("#9A5B00"), lipgloss.Color("#E8B55A")),
		ok:     ld(lipgloss.Color("#1A7F4B"), lipgloss.Color("#5FD7A0")),
		warn:   ld(lipgloss.Color("#9A5B00"), lipgloss.Color("#E8B55A")),
		err:    ld(lipgloss.Color("#B42318"), lipgloss.Color("#FF7070")),
	}
}

type styles struct {
	p                                                     palette
	title, subtle, key, kind, label, value, ok, warn, err lipgloss.Style
	accent, set, faint, keyCap, chip, chipOn, tab, tabOn  lipgloss.Style
	pane, paneFocus                                       lipgloss.Style
}

func newStyles(isDark bool) styles {
	p := newPalette(isDark)
	fg := func(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }
	pane := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.faint).Padding(0, 1)
	return styles{
		p:         p,
		title:     fg(p.ink).Bold(true),
		subtle:    fg(p.dim),
		key:       fg(p.ink).Bold(true),
		kind:      fg(p.dim),
		label:     fg(p.ink).Bold(true),
		value:     fg(p.ink),
		ok:        fg(p.ok),
		warn:      fg(p.warn),
		err:       fg(p.err),
		accent:    fg(p.accent),
		set:       fg(p.set),
		faint:     fg(p.faint),
		keyCap:    fg(p.ink).Bold(true),
		chip:      fg(p.dim).Padding(0, 1),
		chipOn:    lipgloss.NewStyle().Bold(true).Foreground(p.accent).Reverse(true).Padding(0, 1),
		tab:       fg(p.dim),
		tabOn:     fg(p.accent).Bold(true).Underline(true),
		pane:      pane,
		paneFocus: pane.BorderForeground(p.accent),
	}
}

// formTheme keeps the edit form in the same colours as the browser.
// huh only learns the background from a message the browser consumes, so
// the form is themed with the browser's own isDark instead.
func formTheme(isDark bool) huh.Theme {
	return huh.ThemeFunc(func(bool) *huh.Styles {
		p := newPalette(isDark)
		t := huh.ThemeCharm(isDark)
		for _, f := range []*huh.FieldStyles{&t.Focused, &t.Blurred} {
			f.Title = f.Title.Foreground(p.accent)
			f.Description = f.Description.Foreground(p.dim)
			f.SelectSelector = f.SelectSelector.Foreground(p.accent)
			f.MultiSelectSelector = f.MultiSelectSelector.Foreground(p.accent)
			f.NextIndicator = f.NextIndicator.Foreground(p.accent)
			f.PrevIndicator = f.PrevIndicator.Foreground(p.accent)
			f.SelectedOption = f.SelectedOption.Foreground(p.set)
			f.SelectedPrefix = f.SelectedPrefix.Foreground(p.set)
			f.Option = f.Option.Foreground(p.ink)
			f.UnselectedOption = f.UnselectedOption.Foreground(p.ink)
			f.ErrorIndicator = f.ErrorIndicator.Foreground(p.err)
			f.ErrorMessage = f.ErrorMessage.Foreground(p.err)
			f.TextInput.Prompt = f.TextInput.Prompt.Foreground(p.accent)
			f.TextInput.Cursor = f.TextInput.Cursor.Foreground(p.accent)
			f.FocusedButton = lipgloss.NewStyle().Padding(0, 2).MarginRight(1).Bold(true).Foreground(p.accent).Reverse(true)
			f.Next = f.FocusedButton
		}
		t.Focused.Base = t.Focused.Base.BorderForeground(p.accent)
		t.Focused.Card = t.Focused.Base
		t.Group.Title = t.Focused.Title
		t.Group.Description = t.Focused.Description
		return t
	})
}

// RenderDoc renders a setting's documentation block. Also used by `doc`.
func RenderDoc(st *schema.Setting, width int) string {
	s := newStyles(true)
	return docHead(s, st) + "\n" + docBody(s, st, width)
}

func docHead(s styles, st *schema.Setting) string {
	return s.title.Render(st.Key) + "\n" + s.subtle.Render(st.Section+"  "+string(st.Kind)) + "\n"
}

func docBody(s styles, st *schema.Setting, width int) string {
	wrap := lipgloss.NewStyle().Width(width)
	var b strings.Builder
	if st.Deprecated != "" {
		b.WriteString(s.warn.Render(wrap.Render("Removed: "+st.Deprecated)) + "\n\n")
	}
	b.WriteString(wrap.Render(st.Desc) + "\n\n")
	rowWrap := lipgloss.NewStyle().Width(max(1, width-9))
	row := func(k, v string) {
		if v == "" {
			return
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, s.subtle.Render(fmt.Sprintf("%-9s", k)), rowWrap.Render(v)) + "\n")
	}
	row("Scope", st.Scope)
	if st.ScopeNote != "" {
		row("", st.ScopeNote)
	}
	row("Type", st.Type)
	row("Default", st.Default)
	row("Hint", st.Hint)
	if len(st.Suggestions) > 0 {
		row("Values", strings.Join(st.Suggestions, ", "))
	}
	if len(st.Options) > 0 {
		b.WriteString("\n" + s.label.Render("Options") + "\n")
		pad := 0
		for _, o := range st.Options {
			pad = max(pad, lipgloss.Width(o))
		}
		for _, o := range st.Options {
			name := "  " + s.value.Render(fmt.Sprintf("%-*s", pad, o)) + "  "
			help := st.OptionHelp[o]
			if help == "" {
				b.WriteString(name + "\n")
				continue
			}
			// Hang the help text under itself so wrapped lines stay aligned.
			hang := lipgloss.NewStyle().Width(max(1, width-lipgloss.Width(name)))
			b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, name, s.subtle.Render(hang.Render(help))) + "\n")
		}
	}
	if st.Example != "" {
		b.WriteString("\n" + s.label.Render("Example") + "\n" + s.subtle.Render(st.Example) + "\n")
	}
	b.WriteString("\n" + s.accent.Render(st.DocURL()) + "\n")
	return b.String()
}
