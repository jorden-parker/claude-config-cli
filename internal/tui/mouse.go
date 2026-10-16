package tui

import (
	"os"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

// Mouse support. Bubble Tea v2 turns mouse reporting on from the View
// (View.MouseMode), so it is a model field. Each event is mapped to the pane,
// key row, or footer hint it lands on, using the rectangles the last View drew.

// wheelRows is how many key rows one wheel notch moves.
const wheelRows = 3

// rect is a block of screen cells: its top-left corner and its size.
type rect struct{ x, y, w, h int }

func (r rect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

// hitMap is where the last View drew each pane. A zero rect is not on screen.
type hitMap struct {
	side, list, doc rect
}

// footHit is one footer hint and the cells it was drawn in.
type footHit struct {
	x0, x1 int
	hint   hint
}

// hitArea is the pane under the pointer.
type hitArea int

const (
	areaNone hitArea = iota
	areaSidebar
	areaList
	areaDetails
)

// mouseOnFromEnv reports whether mouse reporting starts on. CCFG_NO_MOUSE=1
// starts it off; unset, empty, or 0 leaves it on.
func mouseOnFromEnv() bool {
	v := os.Getenv("CCFG_NO_MOUSE")
	return v == "" || v == "0"
}

func (m *model) mouseMode() tea.MouseMode {
	if m.mouseOn {
		return tea.MouseModeCellMotion
	}
	return tea.MouseModeNone
}

// toggleMouse turns mouse reporting on or off. Off lets the terminal select text.
func (m *model) toggleMouse() {
	m.mouseOn = !m.mouseOn
	if m.mouseOn {
		m.setStatus("Mouse on: click and scroll. Press m to select text instead.", false)
		return
	}
	m.setStatus("Mouse off: your terminal can select text. Press m to turn it back on.", false)
}

// handleMouse routes one event. The browse screen and the field guide respond;
// forms, dialogs and the status line check ignore the mouse, so their own
// input is never disturbed.
func (m *model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if !m.mouseOn {
		return nil
	}
	ev := msg.Mouse()
	wheel, click := 0, false
	switch msg.(type) {
	case tea.MouseWheelMsg:
		switch ev.Button {
		case tea.MouseWheelUp:
			wheel = -1
		case tea.MouseWheelDown:
			wheel = 1
		}
	case tea.MouseClickMsg:
		click = ev.Button == tea.MouseLeft
	}
	if wheel == 0 && !click {
		return nil
	}
	switch m.mode {
	case modeHelp:
		m.guideOffset = max(0, m.guideOffset+wheel)
		return nil
	case modeBrowse:
	default:
		return nil
	}
	if wheel != 0 {
		return m.wheel(ev.X, ev.Y, wheel)
	}
	return m.click(ev.X, ev.Y)
}

// wheel scrolls the pane under the pointer, not the focused one.
func (m *model) wheel(x, y, dir int) tea.Cmd {
	switch m.hitTest(x, y) {
	case areaSidebar:
		return m.setSection(m.sec + dir)
	case areaList:
		cmds := make([]tea.Cmd, 0, wheelRows)
		for range wheelRows {
			cmds = append(cmds, m.moveCursor(dir))
		}
		return tea.Batch(cmds...)
	case areaDetails:
		if dir < 0 {
			m.doc.ScrollUp(wheelRows)
		} else {
			m.doc.ScrollDown(wheelRows)
		}
	}
	return nil
}

// click selects a section or key row. A second click on the row that is
// already selected does what enter does. Clicks on the footer run that hint.
func (m *model) click(x, y int) tea.Cmd {
	if y == m.height-1 {
		if k, ok := m.footerKey(x); ok {
			_, cmd := m.updateBrowse(k)
			return cmd
		}
		return nil
	}
	switch m.hitTest(x, y) {
	case areaSidebar:
		i, ok := m.sidebarRowAt(y)
		m.sideFocus = false
		if !ok || i == m.sec {
			return nil
		}
		return m.setSection(i)
	case areaList:
		idx, ok := m.listRowAt(y)
		if !ok || m.list.VisibleItems()[idx].(item).header {
			return nil
		}
		wasFocused := !m.sideFocus
		m.sideFocus = false
		if wasFocused && idx == m.list.Index() {
			_, cmd := m.updateBrowse(tea.KeyPressMsg{Code: tea.KeyEnter})
			return cmd
		}
		before := m.selected()
		m.list.Select(idx)
		if m.selected() != before {
			m.refreshDoc()
		}
	case areaDetails:
		// The details pane has no focus of its own; clicking it moves the
		// keyboard off the section list, so its scroll keys act on the settings.
		m.sideFocus = false
	}
	return nil
}

// moveCursor moves the key list one row up or down, skipping section headings.
func (m *model) moveCursor(dir int) tea.Cmd {
	before := m.selected()
	k := tea.KeyPressMsg{Code: tea.KeyDown}
	if dir < 0 {
		k = tea.KeyPressMsg{Code: tea.KeyUp}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(k)
	m.skipHeader(dir)
	if m.selected() != before {
		m.refreshDoc()
	}
	return paneCommand(cmd)
}

// hitTest names the pane at a screen cell, from the last View.
func (m *model) hitTest(x, y int) hitArea {
	switch {
	case m.hits.side.contains(x, y):
		return areaSidebar
	case m.hits.list.contains(x, y):
		return areaList
	case m.hits.doc.contains(x, y):
		return areaDetails
	}
	return areaNone
}

// sidebarRowAt maps a screen row inside the sidebar to a section index.
func (m *model) sidebarRowAt(y int) (int, bool) {
	top := m.hits.side.y + 1 // first line inside the border
	h := m.hits.side.h - 2
	if y < top || y >= top+h {
		return 0, false
	}
	i := m.sidebarTop(h) + y - top - sidebarHead
	if i < 0 || i >= len(m.sections) {
		return 0, false
	}
	return i, true
}

// listRowAt maps a screen row inside the key list to an index into
// VisibleItems. Rows differ in height (a description wraps to 1 to 3 lines),
// so each row's height is measured from the same delegate that draws it.
func (m *model) listRowAt(y int) (int, bool) {
	items := m.list.VisibleItems()
	start, end := m.list.Paginator.GetSliceBounds(len(items))
	line := m.hits.list.y + 1 + m.listTitleRows()
	for i := start; i < end; i++ {
		n := m.rowRows(i, items[i])
		if y >= line && y < line+n {
			return i, true
		}
		line += n
	}
	return 0, false
}

// listTitleRows is the height of the list's title bar, above its first row.
func (m *model) listTitleRows() int {
	return 1 + m.list.Styles.TitleBar.GetVerticalFrameSize()
}

// rowRows is how many screen lines row i of the key list takes.
func (m *model) rowRows(i int, it list.Item) int {
	var b strings.Builder
	rowDelegate{m.st}.Render(&b, m.list, i, it)
	return strings.Count(b.String(), "\n") + 1
}

// footerKey is the key press a click on the footer at column x stands for.
func (m *model) footerKey(x int) (tea.KeyPressMsg, bool) {
	for _, f := range m.footHits {
		if x >= f.x0 && x < f.x1 {
			return hintKey(f.hint.key)
		}
	}
	return tea.KeyPressMsg{}, false
}

// hintKey is the key a footer hint names. A hint that names several keys, such
// as "J K" or "[ ]", has no single key and reports false.
func hintKey(label string) (tea.KeyPressMsg, bool) {
	named := map[string]tea.KeyPressMsg{
		"enter":  {Code: tea.KeyEnter},
		"esc":    {Code: tea.KeyEscape},
		"tab":    {Code: tea.KeyTab},
		"←":      {Code: tea.KeyLeft},
		"ctrl+d": {Code: 'd', Mod: tea.ModCtrl},
		"ctrl+k": {Code: 'k', Mod: tea.ModCtrl},
	}
	if k, ok := named[label]; ok {
		return k, true
	}
	if r, size := utf8.DecodeRuneInString(label); size > 0 && size == len(label) && r > ' ' {
		return tea.KeyPressMsg{Code: r, Text: label}, true
	}
	return tea.KeyPressMsg{}, false
}
