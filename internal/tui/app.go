// Package tui is the interactive settings editor built on Bubble Tea, Bubbles,
// Lip Gloss and Huh.
package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
	"github.com/jorden-parker/claude-config-cli/internal/store"
	"github.com/jorden-parker/claude-config-cli/internal/value"
)

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }

type mode int

const (
	modeBrowse mode = iota
	modeEdit
	modeConfirmUnset
	modeHelp
)

// editChrome is the rows around the edit form: padding plus its heading.
const editChrome = 6

type item struct {
	header  bool   // section heading row; never selected
	section string // heading text
	group   bool   // has nested keys; enter opens it
	nSet    int    // nested keys with a value, for group rows
	name    string // label shown in the list: the last part of the key
	parent  string // parent path, shown next to search results
	st      *schema.Setting
	val     string // value in effect, compact JSON; empty when unset
	scope   string // file the value (or the tool's deny rule) comes from
	tool    bool
	off     bool // tool is disabled by a deny rule in some file
}

func (i item) FilterValue() string {
	if i.header {
		return ""
	}
	return i.st.Key + " " + i.st.Section + " " + i.st.Desc
}

type keymap struct {
	edit, cycle, unset, scope, docs, quit, reload, sectionNext, sectionPrev, help, esc, back key.Binding
}

var keys = keymap{
	edit:        key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
	unset:       key.NewBinding(key.WithKeys("u", "delete"), key.WithHelp("u", "remove")),
	back:        key.NewBinding(key.WithKeys("esc", "backspace", "left"), key.WithHelp("esc", "back")),
	cycle:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next value")),
	scope:       key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "target file")),
	docs:        key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open docs")),
	quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	reload:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reload files")),
	sectionNext: key.NewBinding(key.WithKeys("]"), key.WithHelp("[/]", "section")),
	sectionPrev: key.NewBinding(key.WithKeys("[")),
	help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "all keys")),
	esc:         key.NewBinding(key.WithKeys("esc")),
}

type model struct {
	cwd    string
	sch    *schema.Schema
	files  map[store.Scope]*store.File
	errs   map[store.Scope]error
	scope  store.Scope
	mode   mode
	isDark bool
	st     styles

	width, height int
	list          list.Model
	doc           viewport.Model
	sections      []string // panes in the sidebar, in reference order
	sec           int      // open section
	sideFocus     bool     // the sidebar has the keyboard
	edit          *editState
	status        string
	statusErr     bool

	path      []string // open group in the Settings pane, e.g. [sandbox network]
	searching bool     // Settings list holds every key while a filter is active
	parentSet map[string]bool
	synthetic map[string]*schema.Setting
}

// Run starts the interactive editor.
func Run(cwd string) error {
	m := newModel(cwd)
	_, err := tea.NewProgram(m).Run()
	return err
}

func newModel(cwd string) *model {
	m := &model{cwd: cwd, sch: schema.Load(), scope: store.ScopeUser, isDark: true}
	m.st = newStyles(true)
	m.reload()
	m.sections = m.sch.Sections()
	if !contains(m.sections, toolsSection) {
		m.sections = append(m.sections, toolsSection)
	}
	m.list = m.newList(m.items(), "setting", "settings")
	m.skipHeader(1)
	m.applyStyles()
	m.doc = viewport.New()
	m.refreshDoc()
	return m
}

func (m *model) newList(items []list.Item, singular, plural string) list.Model {
	l := list.New(items, rowDelegate{m.st}, 40, 20)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetStatusBarItemName(singular, plural)
	l.KeyMap.Quit.SetEnabled(false)
	l.KeyMap.ShowFullHelp.SetEnabled(false)
	l.KeyMap.CloseFullHelp.SetEnabled(false)
	l.FilterInput.Prompt = "/ "
	l.Filter = substringFilter
	return l
}

// applyStyles pushes the current palette into both lists; called again when
// the terminal reports whether its background is light or dark.
func (m *model) applyStyles() {
	for _, l := range []*list.Model{&m.list} {
		l.SetDelegate(rowDelegate{m.st})
		ls := list.DefaultStyles(m.isDark)
		ls.TitleBar = lipgloss.NewStyle().Padding(0, 0, 1, 0)
		ls.Title = lipgloss.NewStyle()
		ls.NoItems = m.st.subtle.PaddingLeft(2)
		ls.Filter.Focused.Prompt = m.st.accent.Bold(true)
		ls.Filter.Blurred.Prompt = m.st.subtle
		ls.Filter.Cursor.Color = m.st.p.accent
		l.Styles = ls
		l.FilterInput.SetStyles(ls.Filter)
	}
	m.list.Title = m.paneTitle()
}

// paneTitle names the open section, and the open group inside it.
func (m *model) paneTitle() string {
	if m.searching {
		return m.st.tabOn.Render("Search all settings")
	}
	if len(m.path) == 0 {
		return m.st.tabOn.Render(m.section())
	}
	return m.st.tabOn.Render(shortName(m.section())) + m.st.subtle.Render(" › ") + m.st.title.Render(m.breadcrumb())
}

func (m *model) reload() {
	m.files, m.errs = store.OpenAll(m.cwd)
}

// items rebuilds the Settings rows so each shows the live value.
func (m *model) items() []list.Item {
	if m.searching {
		return m.searchItems()
	}
	return m.levelItems()
}

func (m *model) toolItems() []list.Item {
	var out []list.Item
	for i := range toolSettings {
		st := &toolSettings[i]
		it := item{st: st, tool: true, name: st.Key}
		if sc := m.toolDeniedIn(st); sc != "" {
			it.off, it.scope = true, string(sc)
		}
		out = append(out, it)
	}
	return out
}

// effective returns the highest-precedence value set for a key.
func (m *model) effective(st *schema.Setting) (any, store.Scope) {
	order := []store.Scope{store.ScopeManaged, store.ScopeLocal, store.ScopeProject, store.ScopeUser}
	if st.Global {
		order = []store.Scope{store.ScopeGlobal}
	}
	for _, sc := range order {
		if f := m.files[sc]; f != nil {
			if v, ok := f.Get(st.Key); ok {
				return v, sc
			}
		}
	}
	return nil, ""
}

func (m *model) activeList() *list.Model { return &m.list }

func (m *model) selected() *schema.Setting {
	if it, ok := m.activeList().SelectedItem().(item); ok {
		return it.st
	}
	return nil
}

func (m *model) targetScope(st *schema.Setting) store.Scope {
	if st != nil && st.Global {
		return store.ScopeGlobal
	}
	return m.scope
}

func (m *model) Init() tea.Cmd { return tea.RequestBackgroundColor }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case paneMsg:
		updated, cmd := m.list.Update(msg.msg)
		m.list = updated
		m.refreshDoc()
		return m, paneCommand(cmd)
	case tea.BackgroundColorMsg:
		m.isDark = msg.IsDark()
		m.st = newStyles(m.isDark)
		m.applyStyles()
		m.refreshDoc()
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		if m.edit != nil {
			m.edit.form = m.edit.form.WithWidth(m.width - 4).WithHeight(m.height - editChrome)
		}
		return m, nil
	}

	switch m.mode {
	case modeEdit:
		return m.updateEdit(msg)
	case modeHelp:
		if _, ok := msg.(tea.KeyPressMsg); ok {
			m.mode = modeBrowse
		}
		return m, nil
	case modeConfirmUnset:
		if k, ok := msg.(tea.KeyPressMsg); ok {
			m.mode = modeBrowse
			switch k.String() {
			case "y", "Y", "enter":
				return m, m.doUnset()
			}
			m.setStatus("Kept "+m.selected().Key+". Nothing removed.", false)
		}
		return m, nil
	}
	return m.updateBrowse(msg)
}

func (m *model) updateBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, isKey := msg.(tea.KeyPressMsg)
	if isKey && !m.list.SettingFilter() {
		switch {
		case key.Matches(k, keys.quit):
			return m, tea.Quit
		case key.Matches(k, keys.help):
			m.mode = modeHelp
			return m, nil
		case key.Matches(k, keys.scope):
			m.cycleScope()
			return m, m.afterWrite()
		case key.Matches(k, keys.reload):
			m.reload()
			m.setStatus("Reloaded settings files", false)
			return m, m.afterWrite()
		case key.Matches(k, keys.sectionNext), key.Matches(k, keys.sectionPrev):
			if k.String() == "]" {
				return m, m.setSection(m.sec + 1)
			}
			return m, m.setSection(m.sec - 1)
		case k.String() == "pgdown", k.String() == "pgup":
			if k.String() == "pgdown" {
				m.doc.HalfPageDown()
			} else {
				m.doc.HalfPageUp()
			}
			return m, nil
		}
	}
	if isKey && m.sideFocus {
		return m.updateSidebar(k)
	}
	if isKey && !m.list.SettingFilter() {
		switch {
		case key.Matches(k, keys.back) && len(m.path) > 0 && !m.list.IsFiltered():
			return m, m.back()
		case k.String() == "left" && !m.list.IsFiltered():
			m.sideFocus = true
			return m, nil
		case key.Matches(k, keys.esc) && !m.list.IsFiltered():
			m.status = ""
			return m, nil
		case key.Matches(k, keys.edit) && m.onGroup(), k.String() == "right" && m.onGroup():
			return m, m.openGroup()
		case key.Matches(k, keys.cycle) && m.onGroup():
			it, _ := m.selectedItem()
			m.setStatus(fmt.Sprintf("%s is a group. Press enter to open it.", it.name), false)
			return m, nil
		case key.Matches(k, keys.unset) && m.onGroup():
			it, _ := m.selectedItem()
			m.setStatus(fmt.Sprintf("Open %s with enter to remove its settings one at a time.", it.name), false)
			return m, nil
		case key.Matches(k, m.list.KeyMap.Filter) && !m.searching:
			m.startSearch()
		case key.Matches(k, keys.edit):
			return m, m.startEdit()
		case key.Matches(k, keys.cycle):
			return m, m.cycleValue()
		case key.Matches(k, keys.unset):
			st := m.selected()
			if toolName(st) != "" {
				m.setStatus("Tools are turned on and off with tab. Nothing to remove.", false)
				return m, nil
			}
			if st == nil {
				return m, nil
			}
			f := m.files[m.targetScope(st)]
			if _, ok := f.Get(st.Key); !ok {
				m.setStatus(fmt.Sprintf("%s is not set in the %s file. Nothing to remove.", st.Key, m.targetScope(st)), true)
				return m, nil
			}
			m.mode = modeConfirmUnset
			return m, nil
		case key.Matches(k, keys.docs):
			if st := m.selected(); st != nil {
				url := st.DocURL()
				if toolName(st) != "" {
					url = "https://code.claude.com/docs/en/tools-reference"
				}
				openURL(url)
				m.setStatus("Opened docs in your browser: "+url, false)
			}
			return m, nil
		}
	}
	before, beforeIdx := m.selected(), m.list.Index()
	wasPicked := m.list.FilterState() == list.FilterApplied
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if m.searching && m.list.FilterState() == list.Unfiltered {
		// Leaving a search lands on the chosen key in its own section.
		m.searching = false
		if wasPicked && before != nil {
			m.reveal(before.Key)
			return m, paneCommand(cmd)
		}
		return m, tea.Batch(paneCommand(cmd), m.setLevel(""))
	}
	dir := 1
	if m.list.Index() < beforeIdx {
		dir = -1
	}
	m.skipHeader(dir)
	if m.selected() != before {
		m.refreshDoc()
	}
	return m, paneCommand(cmd)
}

// updateSidebar moves between sections; the list beside it follows.
func (m *model) updateSidebar(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "up", "k":
		return m, m.setSection(m.sec - 1)
	case "down", "j":
		return m, m.setSection(m.sec + 1)
	case "home", "g":
		return m, m.setSection(0)
	case "end", "G":
		return m, m.setSection(len(m.sections) - 1)
	case "right", "l", "enter", "tab":
		m.sideFocus = false
	case "/":
		m.sideFocus = false
		m.startSearch()
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(k)
		return m, paneCommand(cmd)
	case "esc":
		m.status = ""
	}
	return m, nil
}

type paneMsg struct{ msg tea.Msg }

func paneCommand(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			commands := make([]tea.Cmd, len(batch))
			for i, c := range batch {
				commands[i] = paneCommand(c)
			}
			return tea.BatchMsg(commands)
		}
		return paneMsg{msg: msg}
	}
}

func (m *model) updateEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	f, cmd := m.edit.form.Update(msg)
	if form, ok := f.(*huh.Form); ok {
		m.edit.form = form
	}
	switch m.edit.form.State {
	case huh.StateAborted:
		m.mode = modeBrowse
		m.edit = nil
		m.setStatus("Edit cancelled. Nothing saved.", false)
	case huh.StateCompleted:
		v, err := m.edit.result()
		st := m.edit.st
		m.mode = modeBrowse
		m.edit = nil
		if err != nil {
			m.setStatus(err.Error(), true)
			return m, nil
		}
		sc := m.targetScope(st)
		f := m.files[sc]
		if err := f.Set(st.Key, v); err != nil {
			m.setStatus(err.Error(), true)
			return m, nil
		}
		if err := f.Save(); err != nil {
			m.setStatus("Save failed: "+err.Error(), true)
			return m, nil
		}
		m.setStatus(fmt.Sprintf("Saved %s = %s in %s", st.Key, short(store.Format(v), 40), tildify(f.Path)), false)
		return m, m.afterWrite()
	}
	return m, cmd
}

func (m *model) startEdit() tea.Cmd {
	st := m.selected()
	if st == nil {
		return nil
	}
	if toolName(st) != "" {
		return m.toggleTool(st)
	}
	sc := m.targetScope(st)
	if !store.Allowed(st, sc) {
		m.setStatus(notReadHere(st, sc), true)
		return nil
	}
	cur, _ := m.files[sc].Get(st.Key)
	m.edit = newEdit(st, cur, m.sch, m.width-4, m.height-editChrome, m.isDark)
	m.mode = modeEdit
	return m.edit.form.Init()
}

// cycleValue sets a single-value setting to its next option and saves it.
// Settings without a fixed option list open the editor instead; settings that
// hold several values (arrays, maps, JSON) are left to enter.
func (m *model) cycleValue() tea.Cmd {
	st := m.selected()
	if st == nil {
		return nil
	}
	if toolName(st) != "" {
		return m.toggleTool(st)
	}
	var opts []string
	switch st.Kind {
	case schema.KindBool:
		opts = []string{"true", "false"}
	case schema.KindEnum, schema.KindEnumOrString:
		opts = st.Options
	case schema.KindString, schema.KindNumber:
		return m.startEdit()
	default:
		m.setStatus(fmt.Sprintf("%s holds several values. Press enter to edit it.", st.Key), true)
		return nil
	}
	if len(opts) == 0 {
		return m.startEdit()
	}
	sc := m.targetScope(st)
	if !store.Allowed(st, sc) {
		m.setStatus(notReadHere(st, sc), true)
		return nil
	}
	f := m.files[sc]
	next := opts[0]
	if cur, ok := f.Get(st.Key); ok {
		for i, o := range opts {
			if o == fmt.Sprint(cur) {
				next = opts[(i+1)%len(opts)]
				break
			}
		}
	}
	v, err := value.Parse(st, next)
	if err != nil {
		m.setStatus(err.Error(), true)
		return nil
	}
	if err := f.Set(st.Key, v); err != nil {
		m.setStatus(err.Error(), true)
		return nil
	}
	if err := f.Save(); err != nil {
		m.setStatus("Save failed: "+err.Error(), true)
		return nil
	}
	m.setStatus(fmt.Sprintf("Saved %s = %s in %s", st.Key, short(store.Format(v), 40), tildify(f.Path)), false)
	return m.afterWrite()
}

func (m *model) doUnset() tea.Cmd {
	st := m.selected()
	if st == nil {
		return nil
	}
	f := m.files[m.targetScope(st)]
	f.Unset(st.Key)
	if err := f.Save(); err != nil {
		m.setStatus("Save failed: "+err.Error(), true)
		return nil
	}
	m.setStatus(fmt.Sprintf("Removed %s from %s", st.Key, tildify(f.Path)), false)
	return m.afterWrite()
}

func (m *model) afterWrite() tea.Cmd {
	idx := m.list.Index()
	m.list.Title = m.paneTitle()
	cmd := m.list.SetItems(m.items())
	m.list.Select(idx)
	m.refreshDoc()
	return paneCommand(cmd)
}

func (m *model) cycleScope() {
	order := []store.Scope{store.ScopeUser, store.ScopeProject, store.ScopeLocal}
	for i, sc := range order {
		if sc == m.scope {
			m.scope = order[(i+1)%len(order)]
			return
		}
	}
	m.scope = store.ScopeUser
}

func (m *model) setStatus(s string, isErr bool) { m.status, m.statusErr = s, isErr }

// Three columns: sections, the open section's keys, then details.
func (m *model) sideWidth() int {
	if m.width >= 100 {
		return 24
	}
	return 20
}

func (m *model) listWidth() int {
	return min(56, max(26, (m.width-m.sideWidth())*2/5))
}

// Rows outside the panes: header, status line, key hints.
const chromeRows = 3

func (m *model) layout() {
	if m.width == 0 {
		return
	}
	lw := m.listWidth()
	inner := max(1, m.height-chromeRows-2) // minus pane borders
	m.list.SetSize(lw-4, max(1, inner-1))  // minus the position line
	m.doc.SetWidth(max(1, m.width-m.sideWidth()-lw-4))
	m.doc.SetHeight(inner)
	m.refreshDoc()
}

func (m *model) refreshDoc() {
	st := m.selected()
	if st == nil {
		m.doc.SetContent(m.st.subtle.Width(max(1, m.doc.Width())).Render("Nothing matches the filter. Press esc to clear it."))
		return
	}
	w := m.doc.Width()
	if w < 20 {
		w = 60
	}
	if toolName(st) != "" {
		m.doc.SetContent(lipgloss.NewStyle().Width(w).Render(m.toolDoc(st, w)))
		m.doc.GotoTop()
		return
	}
	if it, _ := m.selectedItem(); it.group {
		m.doc.SetContent(m.groupDoc(it, w))
		m.doc.GotoTop()
		return
	}
	head := m.st.title.Render(st.Key[strings.LastIndex(st.Key, ".")+1:]) + "\n"
	if i := strings.LastIndex(st.Key, "."); i >= 0 {
		head += m.st.subtle.Render("in "+strings.ReplaceAll(st.Key[:i], ".", " › ")+"  "+string(st.Kind)) + "\n"
	} else {
		head += m.st.subtle.Render(st.Section+"  "+string(st.Kind)) + "\n"
	}
	m.doc.SetContent(head + "\n" + docDescription(m.st, st, w) + m.ladder(st, w) + "\n" + docMetadata(m.st, st, w))
	m.doc.GotoTop()
}

// ladder lists every file that can hold the key, highest precedence first,
// and marks the one in effect and the one edits go to.
func (m *model) ladder(st *schema.Setting, w int) string {
	s := m.st
	order := []store.Scope{store.ScopeManaged, store.ScopeLocal, store.ScopeProject, store.ScopeUser}
	if st.Global {
		order = []store.Scope{store.ScopeGlobal}
	}
	target := m.targetScope(st)
	var b strings.Builder
	b.WriteString(s.label.Render("Where it's set") + "  " + s.subtle.Render("higher rows win") + "\n")
	won := false
	for _, sc := range order {
		f := m.files[sc]
		v, ok := any(nil), false
		if f != nil {
			v, ok = f.Get(st.Key)
		}
		tag := ""
		switch {
		case sc == target:
			tag = s.accent.Render("◂ target file")
		case sc == store.ScopeManaged:
			tag = s.subtle.Render("read-only")
		}
		room := max(4, w-12-lipgloss.Width(tag)-2)
		var marker, val string
		switch {
		case ok && !won:
			won = true
			marker, val = s.set.Render("● "), s.set.Render(short(store.Format(v), room))
		case ok:
			marker, val = s.subtle.Render("○ "), s.subtle.Strikethrough(true).Render(short(store.Format(v), room))
		default:
			marker, val = "  ", s.faint.Render("–")
		}
		name := s.value.Render(fmt.Sprintf("%-9s", sc))
		if sc == target {
			name = s.accent.Bold(true).Render(fmt.Sprintf("%-9s", sc))
		}
		line := marker + name + " " + val
		if tag != "" {
			line += strings.Repeat(" ", max(2, w-lipgloss.Width(line)-lipgloss.Width(tag))) + tag
		}
		b.WriteString(line + "\n")
	}
	def := st.Default
	if def == "" {
		def = "not documented"
	}
	marker := "  "
	if !won {
		marker = s.set.Render("● ")
	}
	b.WriteString(marker + s.subtle.Render(fmt.Sprintf("%-9s", "default")) + " " + s.subtle.Render(short(def, max(4, w-12))) + "\n")
	if !store.Allowed(st, target) {
		b.WriteString("\n" + s.warn.Width(w).Render(notReadHere(st, target)) + "\n")
	}
	return b.String()
}

type hint struct{ key, desc string }

// hints are the keys that do something useful right now, most useful first.
func (m *model) hints() []hint {
	l := m.activeList()
	switch {
	case m.mode == modeHelp:
		return []hint{{"any key", "close"}}
	case m.mode == modeConfirmUnset:
		return []hint{{"y", "remove"}, {"n", "keep"}}
	case l.SettingFilter():
		return []hint{{"enter", "apply filter"}, {"esc", "cancel"}}
	}
	var h []hint
	if m.sideFocus {
		return []hint{{"↑ ↓", "section"}, {"→ enter", "open"}, {"/", "search all"}, {"s", "target file"}, {"r", "reload"}}
	}
	if l.IsFiltered() {
		h = append(h, hint{"esc", "clear search"})
	}
	st := m.selected()
	back := hint{"←", "sections"}
	if len(m.path) > 0 {
		back = hint{"esc", "back"}
	}
	if toolName(st) != "" {
		h = append(h, hint{"tab", "turn on/off"}, back, hint{"s", "target file"}, hint{"/", "search all"}, hint{"[ ]", "section"})
	} else if m.onGroup() {
		h = append(h, hint{"enter", "open"}, back, hint{"s", "target file"}, hint{"/", "search all"}, hint{"[ ]", "section"})
	} else {
		h = append(h, hint{"enter", "edit"})
		if st != nil && cyclable(st) {
			h = append(h, hint{"tab", "next value"})
		}
		if st != nil {
			if f := m.files[m.targetScope(st)]; f != nil {
				if _, ok := f.Get(st.Key); ok {
					h = append(h, hint{"u", "remove"})
				}
			}
		}
		if !l.IsFiltered() {
			h = append(h, back)
		}
		h = append(h, hint{"s", "target file"}, hint{"/", "search all"}, hint{"[ ]", "section"})
	}
	return append(h, hint{"o", "docs"}, hint{"r", "reload"})
}

func cyclable(st *schema.Setting) bool {
	return st.Kind == schema.KindBool || (st.Kind == schema.KindEnum || st.Kind == schema.KindEnumOrString) && len(st.Options) > 0
}

// footer fits as many hints as the width allows and always keeps help and quit.
func (m *model) footer() string {
	render := func(h hint) string { return m.st.keyCap.Render(h.key) + " " + m.st.subtle.Render(h.desc) }
	tail := []hint{{"?", "all keys"}, {"q", "quit"}}
	if m.mode != modeBrowse {
		tail = nil
	}
	var tailParts []string
	for _, h := range tail {
		tailParts = append(tailParts, render(h))
	}
	tailView := strings.Join(tailParts, "   ")
	var parts []string
	used := lipgloss.Width(tailView)
	for _, h := range m.hints() {
		r := render(h)
		if used+lipgloss.Width(r)+3 > m.width-1 {
			break
		}
		parts = append(parts, r)
		used += lipgloss.Width(r) + 3
	}
	left := " " + strings.Join(parts, "   ")
	if tailView == "" {
		return left
	}
	return left + strings.Repeat(" ", max(3, m.width-lipgloss.Width(left)-lipgloss.Width(tailView)-1)) + tailView
}

func (m *model) header() string {
	var chips []string
	for _, sc := range []store.Scope{store.ScopeUser, store.ScopeProject, store.ScopeLocal} {
		if sc == m.scope {
			chips = append(chips, m.st.chipOn.Render(string(sc)))
		} else {
			chips = append(chips, m.st.chip.Render(string(sc)))
		}
	}
	target := m.targetScope(m.selected())
	path := tildify(store.Path(target, m.cwd))
	if target == store.ScopeGlobal {
		path += "  (this key always lives here)"
	}
	return " " + m.st.title.Render("ccfg") + "   " + m.st.subtle.Render("Target file ") + strings.Join(chips, "") + "  " + m.st.subtle.Render(path)
}

func (m *model) statusLine() string {
	var status string
	switch {
	case m.mode == modeConfirmUnset:
		if st := m.selected(); st != nil {
			sc := m.targetScope(st)
			status = m.st.warn.Render(fmt.Sprintf("Remove %s from the %s file (%s)?", st.Key, sc, tildify(store.Path(sc, m.cwd))))
		}
	case m.status != "" && m.statusErr:
		status = m.st.err.Render("✕ " + m.status)
	case m.status != "":
		status = m.st.ok.Render("✓ " + m.status)
	}
	for _, sc := range store.All {
		if err := m.errs[sc]; err != nil {
			status += "  " + m.st.err.Render(fmt.Sprintf("Can't read the %s file: %v", sc, err))
		}
	}
	return " " + status
}

// listFooter shows where the cursor is: the current section and position.
func (m *model) listFooter(width int) string {
	l := m.activeList()
	// Count rows, not section headings.
	n, at := 0, 0
	for i, li := range l.VisibleItems() {
		if !li.(item).header {
			n++
			if i <= l.Index() {
				at++
			}
		}
	}
	pos := ""
	if n > 0 {
		pos = fmt.Sprintf("%d/%d", at, n)
	}
	left := ""
	switch {
	case l.IsFiltered() && !l.SettingFilter():
		left = "filter: " + l.FilterValue()
	case toolName(m.selected()) != "":
		left = "off = listed in permissions.deny"
	case len(m.path) > 0:
		left = "esc back"
	}
	left = short(left, max(1, width-lipgloss.Width(pos)-2))
	return m.st.subtle.Render(left + strings.Repeat(" ", max(1, width-lipgloss.Width(left)-lipgloss.Width(pos))) + pos)
}

func (m *model) keysOverlay() string {
	s := m.st
	groups := []struct {
		title string
		rows  []hint
	}{
		{"Move", []hint{{"↑ ↓  j k", "move up and down"}, {"[ ]", "previous or next section"}, {"← →", "move between sections and their keys"}, {"enter", "open a group such as permissions"}, {"esc ←", "go back out of a group"}, {"/", "search every key in every section"}, {"pgup pgdn", "scroll the details"}}},
		{"Change", []hint{{"enter", "edit the setting, or turn a tool on or off"}, {"tab", "next value, or turn the tool on or off"}, {"u", "remove the key from the target file"}, {"s", "switch the target file: user, project, local"}}},
		{"Other", []hint{{"o", "open the docs page in your browser"}, {"r", "reload the settings files from disk"}, {"q", "quit"}}},
	}
	var b strings.Builder
	for i, g := range groups {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(s.label.Render(g.title) + "\n")
		for _, r := range g.rows {
			b.WriteString("  " + s.accent.Render(fmt.Sprintf("%-10s", r.key)) + "  " + s.value.Render(r.desc) + "\n")
		}
	}
	if m.height >= 28 {
		b.WriteString("\n" + s.subtle.Render("Edits save as soon as you confirm them."))
	}
	return s.paneFocus.Padding(0, 2).Render(strings.TrimRight(b.String(), "\n"))
}

func (m *model) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true
	if m.width == 0 {
		v.SetContent("loading…")
		return v
	}
	if m.mode == modeEdit && m.edit != nil {
		st := m.edit.st
		sc := m.targetScope(st)
		head := m.st.subtle.Render("Editing ") + m.st.key.Render(st.Key) + m.st.subtle.Render(" in the ") +
			m.st.accent.Bold(true).Render(string(sc)) + m.st.subtle.Render(" file  "+tildify(store.Path(sc, m.cwd)))
		head = ansi.Truncate(head, max(1, m.width-4), "…") + "\n" + m.st.subtle.Render("esc cancels without saving")
		v.SetContent(lipgloss.NewStyle().Padding(1, 2).MaxWidth(m.width).Render(head + "\n\n" + m.edit.form.View()))
		return v
	}

	bodyHeight := m.height - chromeRows
	var body string
	if m.mode == modeHelp {
		body = lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, m.keysOverlay())
	} else {
		lw := m.listWidth()
		sidePane, listPane := m.st.pane, m.st.paneFocus
		if m.sideFocus {
			sidePane, listPane = m.st.paneFocus, m.st.pane
		}
		side := sidePane.Width(m.sideWidth()).Height(bodyHeight).Render(m.sidebar(m.sideWidth()-4, bodyHeight-2))
		mid := listPane.Width(lw).Height(bodyHeight).Render(lipgloss.JoinVertical(lipgloss.Left, m.list.View(), m.listFooter(lw-4)))
		right := m.st.pane.Width(m.doc.Width() + 4).Height(bodyHeight).Render(m.doc.View())
		body = lipgloss.JoinHorizontal(lipgloss.Top, side, mid, right)
	}
	line := lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(1)
	v.SetContent(lipgloss.JoinVertical(lipgloss.Left,
		line.Render(m.header()),
		body,
		line.Width(m.width).Render(m.statusLine()),
		line.Render(m.footer()),
	))
	return v
}

// substringFilter matches keys and sections by plain case-insensitive substring,
// which is more predictable than fuzzy matching across 227 similar names.
func substringFilter(term string, targets []string) []list.Rank {
	term = strings.ToLower(strings.TrimSpace(term))
	var out []list.Rank
	for i, t := range targets {
		lt := strings.ToLower(t)
		idx := strings.Index(lt, term)
		if term == "" || idx >= 0 {
			var matched []int
			for j := idx; j >= 0 && j < idx+len(term); j++ {
				matched = append(matched, j)
			}
			out = append(out, list.Rank{Index: i, MatchedIndexes: matched})
		}
	}
	return out
}

// short truncates by display width so multi-byte values are never cut mid-rune.
func short(s string, n int) string { return ansi.Truncate(s, n, "…") }

// tildify shows paths under the home directory as ~/…
func tildify(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

func notReadHere(st *schema.Setting, sc store.Scope) string {
	return fmt.Sprintf("Claude Code doesn't read %s from the %s file (allowed: %s). Press s to pick another target file.", st.Key, sc, st.Scope)
}

func openURL(u string) {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", u)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		c = exec.Command("xdg-open", u)
	}
	_ = c.Start()
}
