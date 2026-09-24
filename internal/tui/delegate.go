package tui

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// rowDelegate draws each setting or tool on one line: key on the left, the
// value in effect and the file it comes from on the right.
type rowDelegate struct{ st styles }

func (rowDelegate) Height() int                         { return 1 }
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
	case it.st.Deprecated != "":
		right = s.subtle.Render("removed")
	case it.off:
		right = s.err.Render("off") + " " + s.subtle.Render(it.scope)
	case it.tool:
		right = s.subtle.Render("on")
	case it.val != "":
		right = s.set.Render(ansi.Truncate(it.val, max(4, width/3), "…")) + " " + s.subtle.Render(it.scope)
	}

	name := it.st.Key
	room := max(4, width-2-lipgloss.Width(right)-1)
	name = ansi.Truncate(name, room, "…")
	unmatched := keyStyle
	matched := keyStyle.Underline(true).Foreground(s.p.accent)
	var idx []int
	for _, i := range lm.MatchesForItem(index) {
		if i < len([]rune(name)) {
			idx = append(idx, i)
		}
	}
	keyView := keyStyle.Render(name)
	if len(idx) > 0 {
		keyView = lipgloss.StyleRunes(name, idx, matched, unmatched)
	}

	gap := max(1, width-2-lipgloss.Width(keyView)-lipgloss.Width(right))
	fmt.Fprint(w, gutter+keyView+fmt.Sprintf("%*s", gap, "")+right)
}
