package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// mouseModel is a browse screen drawn once, so its hit map matches the screen.
func mouseModel(t *testing.T, width, height int) *model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CCFG_NO_MOUSE", "")
	m := newModel(t.TempDir())
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m.View()
	return m
}

func screen(m *model) []string {
	return strings.Split(ansi.Strip(m.View().Content), "\n")
}

// listRow is the screen row where the key list draws label.
func listRow(t *testing.T, m *model, label string) int {
	t.Helper()
	l := m.hits.list
	for y, line := range screen(m) {
		if strings.Contains(ansi.Cut(line, l.x, l.x+l.w), label) {
			return y
		}
	}
	t.Fatalf("%s is not drawn in the key list", label)
	return 0
}

// sidebarRow is the screen row where the sidebar draws a section.
func sidebarRow(t *testing.T, m *model, section string) int {
	t.Helper()
	s := m.hits.side
	for y, line := range screen(m) {
		if strings.Contains(ansi.Cut(line, s.x, s.x+s.w), shortName(section)) {
			return y
		}
	}
	t.Fatalf("%s is not drawn in the sidebar", section)
	return 0
}

func click(m *model, x, y int) tea.Cmd {
	_, cmd := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	return cmd
}

func wheel(m *model, x, y int, button tea.MouseButton) {
	m.Update(tea.MouseWheelMsg{X: x, Y: y, Button: button})
}

func TestMouseHitTestPanes(t *testing.T) {
	m := mouseModel(t, 120, 40)
	side, list, doc := m.hits.side, m.hits.list, m.hits.doc
	if side.w == 0 || list.w == 0 || doc.w == 0 {
		t.Fatalf("panes not recorded: %+v %+v %+v", side, list, doc)
	}
	if side.x+side.w != list.x || list.x+list.w != doc.x || doc.x+doc.w != 120 {
		t.Fatalf("panes do not tile the width: %+v %+v %+v", side, list, doc)
	}
	// The borders drawn on screen sit where the rectangles say they do.
	lines := screen(m)
	mid := lines[side.y+5]
	for _, x := range []int{side.x + side.w - 1, list.x, list.x + list.w - 1, doc.x} {
		if c := ansi.Cut(mid, x, x+1); c != "│" {
			t.Fatalf("cell %d on row %d is %q, not a pane border", x, side.y+5, c)
		}
	}
	cases := []struct {
		x, y int
		want hitArea
	}{
		{0, side.y + 5, areaSidebar},
		{side.w - 1, side.y + 5, areaSidebar},
		{side.w, side.y + 5, areaList},
		{list.x + list.w - 1, side.y + 5, areaList},
		{doc.x, side.y + 5, areaDetails},
		{119, side.y + 5, areaDetails},
		{5, 0, areaNone}, // the header is not a pane
		{5, 39, areaNone},
	}
	for _, c := range cases {
		if got := m.hitTest(c.x, c.y); got != c.want {
			t.Fatalf("hitTest(%d, %d) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestMouseHitTestMediumAndNarrow(t *testing.T) {
	m := mouseModel(t, 80, 30)
	if m.hits.doc.w != 0 {
		t.Fatal("details pane drawn at 80 columns")
	}
	if got := m.hitTest(79, 5); got != areaList {
		t.Fatalf("right edge at 80 columns = %v, want the key list", got)
	}
	if got := m.hitTest(3, 5); got != areaSidebar {
		t.Fatalf("left edge at 80 columns = %v, want the sidebar", got)
	}

	m = mouseModel(t, 50, 30)
	if got := m.hitTest(10, 5); got != areaList {
		t.Fatalf("50 columns shows %v, want only the key list", got)
	}
	m.sideFocus = true
	m.View()
	if got := m.hitTest(10, 5); got != areaSidebar {
		t.Fatalf("50 columns with sections focused shows %v, want the sidebar", got)
	}
}

// The row under the pointer must be the row the screen shows as selected,
// including after the list has scrolled to a later page and across rows that
// wrap to different heights.
func TestMouseKeyRowMatchesScreen(t *testing.T) {
	m := mouseModel(t, 120, 40)
	l := m.hits.list
	sawLaterPage := false
	for step := 0; step < 30; step++ {
		y := -1
		for i, line := range screen(m) {
			if strings.Contains(ansi.Cut(line, l.x, l.x+l.w), "❯") {
				y = i
				break
			}
		}
		if y < 0 {
			t.Fatalf("step %d: no selected row on screen", step)
		}
		idx, ok := m.listRowAt(y)
		if !ok || idx != m.list.Index() {
			t.Fatalf("step %d: row at y=%d is %d (ok=%v), screen selects %d", step, y, idx, ok, m.list.Index())
		}
		if m.list.Paginator.Page > 0 {
			sawLaterPage = true
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if !sawLaterPage {
		t.Fatal("the walk never reached a second page, so scrolled rows were not tested")
	}
}

func TestMouseClickSelectsSectionThenRow(t *testing.T) {
	m := mouseModel(t, 120, 40)
	click(m, 2, sidebarRow(t, m, "Sandbox settings"))
	if m.section() != "Sandbox settings" || m.sideFocus {
		t.Fatalf("clicking Sandbox opened %q (sideFocus=%v)", m.section(), m.sideFocus)
	}

	selectKey(t, m, "effortLevel")
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	row := listRow(t, m, "effortLevel")
	click(m, m.hits.list.x+3, row)
	if m.selected().Key != "effortLevel" || m.mode != modeBrowse {
		t.Fatalf("first click selected %q in mode %v", m.selected().Key, m.mode)
	}
	click(m, m.hits.list.x+3, row)
	if m.mode != modeEdit {
		t.Fatalf("second click on the selected row did not act like enter (mode %v)", m.mode)
	}
}

func TestMouseClickOnDetailsMovesFocusOffSections(t *testing.T) {
	m := mouseModel(t, 120, 40)
	m.sideFocus = true
	m.View()
	click(m, m.hits.doc.x+2, m.hits.doc.y+3)
	if m.sideFocus {
		t.Fatal("clicking the details pane left the sections focused")
	}
}

func TestMouseWheelScrollsPaneUnderPointer(t *testing.T) {
	m := mouseModel(t, 120, 40)
	selectKey(t, m, "effortLevel")
	before := m.list.Index()
	wheel(m, m.hits.list.x+3, m.hits.list.y+5, tea.MouseWheelDown)
	if m.list.Index() <= before {
		t.Fatalf("wheel down over the key list left row %d", m.list.Index())
	}
	wheel(m, m.hits.list.x+3, m.hits.list.y+5, tea.MouseWheelUp)
	if m.list.Index() != before {
		t.Fatalf("wheel up over the key list went to row %d, want %d", m.list.Index(), before)
	}

	sec := m.sec
	wheel(m, 2, sidebarRow(t, m, m.sections[sec+1]), tea.MouseWheelDown)
	if m.sec != sec+1 {
		t.Fatalf("wheel down over the sidebar opened section %d, want %d", m.sec, sec+1)
	}
	wheel(m, 2, sidebarRow(t, m, m.section()), tea.MouseWheelUp)
	if m.sec != sec {
		t.Fatalf("wheel up over the sidebar opened section %d, want %d", m.sec, sec)
	}

	selectKey(t, m, "hooks")
	if m.doc.TotalLineCount() <= m.doc.Height() {
		t.Fatalf("details for hooks fit on screen (%d lines in %d); test needs a longer one", m.doc.TotalLineCount(), m.doc.Height())
	}
	wheel(m, m.hits.doc.x+2, m.hits.doc.y+3, tea.MouseWheelDown)
	if m.doc.YOffset() == 0 {
		t.Fatal("wheel down over the details pane did not scroll it")
	}
	if got := m.selected().Key; got != "hooks" {
		t.Fatalf("wheel over details moved the selection to %s", got)
	}
}

func TestMouseIgnoredWhileEditing(t *testing.T) {
	m := mouseModel(t, 120, 40)
	selectKey(t, m, "effortLevel")
	sec := m.sec
	row := sidebarRow(t, m, m.sections[sec+1])
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != modeEdit {
		t.Fatal("enter did not open the editor")
	}
	// The form covers the screen, so the click lands on the hit map from before.
	click(m, 2, row)
	wheel(m, m.hits.list.x+3, m.hits.list.y+5, tea.MouseWheelDown)
	if m.mode != modeEdit || m.sec != sec || m.selected().Key != "effortLevel" {
		t.Fatalf("mouse changed the form: mode=%v section=%d key=%s", m.mode, m.sec, m.selected().Key)
	}
}

func TestMouseFooterHintsRunTheirKey(t *testing.T) {
	m := mouseModel(t, 120, 40)
	last := screen(m)[m.height-1]

	x := lipgloss.Width(last[:strings.Index(last, "ctrl+k")])
	click(m, x+1, m.height-1)
	if m.mode != modePalette {
		t.Fatalf("clicking ctrl+k in the footer left mode %v", m.mode)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	last = screen(m)[m.height-1]
	x = lipgloss.Width(last[:strings.Index(last, "? all keys")])
	click(m, x, m.height-1)
	if m.mode != modeHelp {
		t.Fatalf("clicking ? in the footer left mode %v", m.mode)
	}
}

func TestMouseToggleKey(t *testing.T) {
	m := mouseModel(t, 120, 40)
	if got := m.View().MouseMode; got != tea.MouseModeCellMotion {
		t.Fatalf("mouse mode at start = %v", got)
	}
	m.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	if m.mouseOn || m.View().MouseMode != tea.MouseModeNone {
		t.Fatal("m did not turn the mouse off")
	}
	if !strings.Contains(ansi.Strip(m.statusLine()), "Mouse off") || m.statusErr {
		t.Fatalf("status = %q", m.status)
	}

	sec := m.sec
	click(m, 2, sidebarRow(t, m, m.sections[sec+1]))
	if m.sec != sec {
		t.Fatal("a click arrived while the mouse was off and was handled")
	}

	m.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	if !m.mouseOn || m.View().MouseMode != tea.MouseModeCellMotion {
		t.Fatal("m did not turn the mouse back on")
	}
	if !strings.Contains(m.status, "Mouse on") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestMouseToggleKeyWaitsForFilter(t *testing.T) {
	m := mouseModel(t, 120, 40)
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	if !m.mouseOn {
		t.Fatal("m typed into the search still toggled the mouse")
	}
}

func TestCCFGNoMouseStartsOff(t *testing.T) {
	for value, on := range map[string]bool{"": true, "0": true, "1": false, "yes": false} {
		t.Setenv("CCFG_NO_MOUSE", value)
		if got := mouseOnFromEnv(); got != on {
			t.Fatalf("CCFG_NO_MOUSE=%q: mouse on = %v, want %v", value, got, on)
		}
	}

	t.Setenv("HOME", t.TempDir())
	t.Setenv("CCFG_NO_MOUSE", "1")
	m := newModel(t.TempDir())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if got := m.View().MouseMode; got != tea.MouseModeNone {
		t.Fatalf("CCFG_NO_MOUSE=1 started with mouse mode %v", got)
	}
	m.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	if got := m.View().MouseMode; got != tea.MouseModeCellMotion {
		t.Fatalf("m did not turn on mouse mode that CCFG_NO_MOUSE turned off (got %v)", got)
	}
}
