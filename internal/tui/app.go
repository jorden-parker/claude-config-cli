// Package tui is the interactive settings editor built on Bubble Tea, Bubbles,
// Lip Gloss and Huh.
package tui

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

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
)

type item struct {
	st   *schema.Setting
	desc string
}

func (i item) Title() string       { return i.st.Key }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.st.Key + " " + i.st.Section }

type keymap struct {
	edit, cycle, unset, scope, docs, focus, quit, reload, sectionNext, sectionPrev key.Binding
}

var keys = keymap{
	edit:        key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
	unset:       key.NewBinding(key.WithKeys("u", "backspace", "delete"), key.WithHelp("u", "unset")),
	cycle:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next value")),
	scope:       key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "target file")),
	docs:        key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open docs")),
	focus:       key.NewBinding(key.WithKeys("right", "left"), key.WithHelp("←/→", "scroll doc")),
	quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	reload:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reload files")),
	sectionNext: key.NewBinding(key.WithKeys("]"), key.WithHelp("[/]", "section")),
	sectionPrev: key.NewBinding(key.WithKeys("[")),
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
	docFocus      bool
	edit          *editState
	status        string
	statusErr     bool
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
	del := list.NewDefaultDelegate()
	del.Styles = list.NewDefaultItemStyles(true)
	m.list = list.New(m.items(), del, 40, 20)
	m.list.Title = "Claude Code settings"
	m.list.SetShowHelp(false)
	m.list.SetStatusBarItemName("setting", "settings")
	m.list.KeyMap.Quit.SetEnabled(false)
	m.list.Filter = substringFilter
	m.doc = viewport.New()
	m.refreshDoc()
	return m
}

func (m *model) reload() {
	m.files, m.errs = store.OpenAll(m.cwd)
}

// items rebuilds the list rows so each description shows the live value.
func (m *model) items() []list.Item {
	var out []list.Item
	for i := range m.sch.Settings {
		st := &m.sch.Settings[i]
		out = append(out, item{st: st, desc: m.summary(st)})
	}
	return out
}

func (m *model) summary(st *schema.Setting) string {
	parts := []string{st.Section}
	if v, sc := m.effective(st); sc != "" {
		parts = append(parts, fmt.Sprintf("= %s [%s]", short(store.Format(v), 24), sc))
	}
	if st.Deprecated != "" {
		parts = append(parts, "removed")
	}
	return short(strings.Join(parts, " · "), max(m.listWidth()-8, 20))
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

func (m *model) selected() *schema.Setting {
	if it, ok := m.list.SelectedItem().(item); ok {
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
	case tea.BackgroundColorMsg:
		m.isDark = msg.IsDark()
		m.st = newStyles(m.isDark)
		del := list.NewDefaultDelegate()
		del.Styles = list.NewDefaultItemStyles(m.isDark)
		m.list.SetDelegate(del)
		m.refreshDoc()
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		if m.edit != nil {
			m.edit.form = m.edit.form.WithWidth(m.width - 4).WithHeight(m.height - 4)
		}
		return m, nil
	}

	switch m.mode {
	case modeEdit:
		return m.updateEdit(msg)
	case modeConfirmUnset:
		if k, ok := msg.(tea.KeyPressMsg); ok {
			m.mode = modeBrowse
			switch k.String() {
			case "y", "Y", "enter":
				return m, m.doUnset()
			}
		}
		return m, nil
	}
	return m.updateBrowse(msg)
}

func (m *model) updateBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && !m.list.SettingFilter() {
		switch {
		case key.Matches(k, keys.quit):
			return m, tea.Quit
		case key.Matches(k, keys.edit):
			return m, m.startEdit()
		case key.Matches(k, keys.cycle):
			return m, m.cycleValue()
		case key.Matches(k, keys.unset):
			st := m.selected()
			if st == nil {
				return m, nil
			}
			f := m.files[m.targetScope(st)]
			if _, ok := f.Get(st.Key); !ok {
				m.setStatus(fmt.Sprintf("%s is not set in %s", st.Key, f.Path), true)
				return m, nil
			}
			m.mode = modeConfirmUnset
			return m, nil
		case key.Matches(k, keys.scope):
			m.cycleScope()
			m.refreshDoc()
			return m, nil
		case key.Matches(k, keys.docs):
			if st := m.selected(); st != nil {
				openURL(st.DocURL())
				m.setStatus("opened "+st.DocURL(), false)
			}
			return m, nil
		case key.Matches(k, keys.reload):
			m.reload()
			m.setStatus("reloaded settings files", false)
			return m, m.afterWrite()
		case key.Matches(k, keys.sectionNext), key.Matches(k, keys.sectionPrev):
			m.jumpSection(k.String() == "]")
			m.refreshDoc()
			return m, nil
		case k.String() == "right", k.String() == "left", k.String() == "pgdown", k.String() == "pgup":
			var cmd tea.Cmd
			switch k.String() {
			case "right", "pgdown":
				m.doc.HalfPageDown()
			default:
				m.doc.HalfPageUp()
			}
			return m, cmd
		}
	}
	before := m.selected()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if m.selected() != before {
		m.refreshDoc()
	}
	return m, cmd
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
		m.setStatus("edit cancelled", false)
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
			m.setStatus("save failed: "+err.Error(), true)
			return m, nil
		}
		m.setStatus(fmt.Sprintf("saved %s = %s → %s", st.Key, short(store.Format(v), 40), f.Path), false)
		return m, m.afterWrite()
	}
	return m, cmd
}

func (m *model) startEdit() tea.Cmd {
	st := m.selected()
	if st == nil {
		return nil
	}
	sc := m.targetScope(st)
	if !store.Allowed(st, sc) {
		m.setStatus(fmt.Sprintf("%s isn't read from %s settings (docs: %s). Press s to change the target file.", st.Key, sc, st.Scope), true)
		return nil
	}
	cur, _ := m.files[sc].Get(st.Key)
	m.edit = newEdit(st, cur, m.sch, m.width-4, m.height-4, m.isDark)
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
	var opts []string
	switch st.Kind {
	case schema.KindBool:
		opts = []string{"true", "false"}
	case schema.KindEnum, schema.KindEnumOrString:
		opts = st.Options
	case schema.KindString, schema.KindNumber:
		return m.startEdit()
	default:
		m.setStatus(fmt.Sprintf("%s holds several values; press enter to edit it", st.Key), true)
		return nil
	}
	if len(opts) == 0 {
		return m.startEdit()
	}
	sc := m.targetScope(st)
	if !store.Allowed(st, sc) {
		m.setStatus(fmt.Sprintf("%s isn't read from %s settings (docs: %s). Press s to change the target file.", st.Key, sc, st.Scope), true)
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
		m.setStatus("save failed: "+err.Error(), true)
		return nil
	}
	m.setStatus(fmt.Sprintf("saved %s = %s → %s", st.Key, short(store.Format(v), 40), f.Path), false)
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
		m.setStatus("save failed: "+err.Error(), true)
		return nil
	}
	m.setStatus(fmt.Sprintf("removed %s from %s", st.Key, f.Path), false)
	return m.afterWrite()
}

func (m *model) afterWrite() tea.Cmd {
	idx := m.list.Index()
	cmd := m.list.SetItems(m.items())
	m.list.Select(idx)
	m.refreshDoc()
	return cmd
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

func (m *model) jumpSection(forward bool) {
	st := m.selected()
	if st == nil {
		return
	}
	items := m.list.Items()
	i := m.list.Index()
	if forward {
		for j := i + 1; j < len(items); j++ {
			if items[j].(item).st.Section != st.Section {
				m.list.Select(j)
				return
			}
		}
		return
	}
	// previous: go to first item of the previous section
	j := i
	for j > 0 && items[j-1].(item).st.Section == st.Section {
		j--
	}
	if j == 0 {
		m.list.Select(0)
		return
	}
	prev := items[j-1].(item).st.Section
	for j > 0 && items[j-1].(item).st.Section == prev {
		j--
	}
	m.list.Select(j)
}

func (m *model) setStatus(s string, isErr bool) { m.status, m.statusErr = s, isErr }

func (m *model) listWidth() int {
	lw := m.width * 2 / 5
	if lw < 34 {
		lw = 34
	}
	if lw > 64 {
		lw = 64
	}
	return lw
}

func (m *model) layout() {
	if m.width == 0 {
		return
	}
	lw := m.listWidth()
	h := m.height - 2 // header + status
	m.list.SetSize(lw-4, h-2)
	m.doc.SetWidth(m.width - lw - 6)
	m.doc.SetHeight(h - 2)
	m.refreshDoc()
}

func (m *model) refreshDoc() {
	st := m.selected()
	if st == nil {
		m.doc.SetContent("")
		return
	}
	w := m.doc.Width()
	if w < 20 {
		w = 60
	}
	var b strings.Builder
	b.WriteString(RenderDoc(st, w))
	b.WriteString("\n" + m.st.label.Render("Current values") + "\n")
	any := false
	for _, sc := range store.All {
		f := m.files[sc]
		if f == nil {
			continue
		}
		if st.Global && sc != store.ScopeGlobal || !st.Global && sc == store.ScopeGlobal {
			continue
		}
		if v, ok := f.Get(st.Key); ok {
			any = true
			line := fmt.Sprintf("  %-8s %s", sc, store.Format(v))
			b.WriteString(lipgloss.NewStyle().Width(w).Render(m.st.ok.Render(line)) + "\n")
		}
	}
	if !any {
		b.WriteString(m.st.subtle.Render("  not set in any file; default applies") + "\n")
	}
	target := m.targetScope(st)
	allowed := store.Allowed(st, target)
	tl := fmt.Sprintf("\nTarget file: %s (%s)", target, store.Path(target, m.cwd))
	if allowed {
		b.WriteString(m.st.subtle.Render(tl) + "\n")
	} else {
		b.WriteString(m.st.warn.Render(tl+" — not read from here; press s to switch") + "\n")
	}
	m.doc.SetContent(b.String())
	m.doc.GotoTop()
}

func (m *model) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true
	if m.width == 0 {
		v.SetContent("loading…")
		return v
	}
	if m.mode == modeEdit && m.edit != nil {
		v.SetContent(lipgloss.NewStyle().Padding(1, 2).Render(m.edit.form.View()))
		return v
	}

	// header
	var badges []string
	for _, sc := range []store.Scope{store.ScopeUser, store.ScopeProject, store.ScopeLocal} {
		if sc == m.scope {
			badges = append(badges, m.st.badgeOn.Render(string(sc)))
		} else {
			badges = append(badges, m.st.badge.Render(string(sc)))
		}
	}
	header := lipgloss.JoinHorizontal(lipgloss.Center,
		m.st.title.Render(" claude-config-cli "),
		m.st.subtle.Render(" write to: "),
		strings.Join(badges, ""),
	)

	lw := m.listWidth()
	h := m.height - 2
	left := m.st.paneFocus.Width(lw - 2).Height(h - 2).Render(m.list.View())
	right := m.st.pane.Height(h - 2).Render(m.doc.View())
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	status := m.st.help.Render("enter edit · tab next value · u unset · s target file · [ ] section · / filter · o docs · r reload · q quit")
	if m.mode == modeConfirmUnset {
		if st := m.selected(); st != nil {
			status = m.st.warn.Render(fmt.Sprintf("Remove %s from %s? (y/n)", st.Key, m.targetScope(st)))
		}
	} else if m.status != "" {
		if m.statusErr {
			status = m.st.err.Render(m.status)
		} else {
			status = m.st.ok.Render(m.status)
		}
	}
	for sc, err := range m.errs {
		status += "  " + m.st.err.Render(fmt.Sprintf("[%s file unreadable: %v]", sc, err))
	}
	v.SetContent(lipgloss.JoinVertical(lipgloss.Left, header, body, lipgloss.NewStyle().MaxWidth(m.width).Render(status)))
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

func short(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
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
