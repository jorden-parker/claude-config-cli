package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// rowDelegate draws the key and effective value above a description preview.
type rowDelegate struct{ st styles }

func (rowDelegate) Height() int                         { return 2 }
func (rowDelegate) Spacing() int                        { return 0 }
func (rowDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d rowDelegate) Render(w io.Writer, lm list.Model, index int, li list.Item) {
	it, ok := li.(item)
	if !ok {
		return
	}
	s := d.st
	selected := index == lm.Index()
	width := lm.Width()
	if it.header {
		fmt.Fprint(w, s.subtle.Bold(true).Render(ansi.Truncate(it.section, width, "…")))
		return
	}

	gutter := "  "
	keyStyle := s.value
	if selected {
		gutter = s.accent.Render("▌ ")
		keyStyle = s.key
	}
	if it.st.Deprecated != "" {
		keyStyle = keyStyle.Foreground(s.p.dim).Strikethrough(true)
	}

	var right string
	switch {
	case it.group && it.nSet > 0:
		right = s.set.Render(fmt.Sprintf("%d set", it.nSet)) + s.subtle.Render(" ›")
	case it.group:
		right = s.subtle.Render("›")
	case it.st.Deprecated != "":
		right = s.subtle.Render("removed")
	case it.off:
		right = s.err.Render("off") + " " + s.subtle.Render(it.scope)
	case it.tool:
		right = s.subtle.Render("on")
	case it.val != "":
		right = s.set.Render(ansi.Truncate(it.val, max(4, width/3), "…")) + " " + s.subtle.Render(it.scope)
	}

	name := it.name
	if name == "" {
		name = it.st.Key
	}
	room := max(4, width-2-lipgloss.Width(right)-1)
	name = ansi.Truncate(name, room, "…")
	unmatched := keyStyle
	matched := keyStyle.Underline(true).Foreground(s.p.accent)
	// Filter matches index into the full key; shift them onto the shown name.
	offset := len([]rune(it.st.Key)) - len([]rune(it.name))
	if it.name == "" {
		offset = 0
	}
	var idx []int
	for _, i := range lm.MatchesForItem(index) {
		if j := i - offset; j >= 0 && j < len([]rune(name)) {
			idx = append(idx, j)
		}
	}
	keyView := keyStyle.Render(name)
	if len(idx) > 0 {
		keyView = lipgloss.StyleRunes(name, idx, matched, unmatched)
	}

	if it.parent != "" {
		if extra := width - 2 - lipgloss.Width(keyView) - lipgloss.Width(right) - 3; extra > 4 {
			keyView += " " + s.subtle.Render(ansi.Truncate(it.parent, extra, "…"))
		}
	}
	gap := max(1, width-2-lipgloss.Width(keyView)-lipgloss.Width(right))
	fmt.Fprint(w, gutter+keyView+fmt.Sprintf("%*s", gap, "")+right)
	desc := strings.Join(strings.Fields(it.st.Desc), " ")
	fmt.Fprint(w, "\n  "+s.subtle.Render(ansi.Truncate(desc, max(1, width-2), "…")))
}
