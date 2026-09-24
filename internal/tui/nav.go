package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

// The Settings pane shows one level of the settings tree at a time. The top
// level lists top-level keys under section headings; keys such as
// sandbox.network.allowedDomains live inside group rows that enter opens.
// Filtering searches every key regardless of level.

// parents is the set of keys that have nested keys under them.
func (m *model) parents() map[string]bool {
	if m.parentSet == nil {
		m.parentSet = map[string]bool{}
		for _, st := range m.sch.Settings {
			parts := strings.Split(st.Key, ".")
			for i := 1; i < len(parts); i++ {
				m.parentSet[strings.Join(parts[:i], ".")] = true
			}
		}
	}
	return m.parentSet
}

// groupSetting returns the schema entry for a group key, or a stand-in when
// the docs only describe its nested keys.
func (m *model) groupSetting(k, section string) *schema.Setting {
	if st := m.sch.Get(k); st != nil {
		return st
	}
	if m.synthetic == nil {
		m.synthetic = map[string]*schema.Setting{}
	}
	if st := m.synthetic[k]; st != nil {
		return st
	}
	st := &schema.Setting{Key: k, Section: section, Kind: schema.KindGroup, ScopeClass: schema.ScopeAny}
	m.synthetic[k] = st
	return st
}

// levelPrefix is the key prefix of the level on screen. A section whose only
// top-level key is a group (Sandbox holds just "sandbox") opens that group
// directly instead of showing a single row.
func (m *model) levelPrefix() string {
	if len(m.path) == 0 {
		return m.autoGroup()
	}
	return strings.Join(m.path, ".")
}

func (m *model) autoGroup() string {
	if m.section() == toolsSection {
		return ""
	}
	top := ""
	for _, st := range m.sch.Settings {
		if st.Section != m.section() {
			continue
		}
		seg, _, _ := strings.Cut(st.Key, ".")
		if top != "" && seg != top {
			return ""
		}
		top = seg
	}
	if !m.parents()[top] {
		return ""
	}
	return top
}

func (m *model) levelItems() []list.Item {
	prefix := m.levelPrefix()
	parents := m.parents()
	seen := map[string]bool{}
	var out []list.Item
	for i := range m.sch.Settings {
		st := &m.sch.Settings[i]
		rel := st.Key
		if len(m.path) == 0 && st.Section != m.section() {
			continue
		}
		if prefix != "" {
			if !strings.HasPrefix(st.Key, prefix+".") {
				continue
			}
			rel = st.Key[len(prefix)+1:]
		}
		seg, _, _ := strings.Cut(rel, ".")
		k := seg
		if prefix != "" {
			k = prefix + "." + seg
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		var row item
		if parents[k] {
			row = item{st: m.groupSetting(k, st.Section), group: true, nSet: m.countSet(k)}
		} else {
			row = m.leafItem(st)
		}
		row.name = seg
		out = append(out, row)
	}
	if len(m.path) == 0 && m.section() == toolsSection {
		if len(out) > 0 {
			out = append(out, item{header: true, section: "Turn tools on or off"})
		}
		out = append(out, m.toolItems()...)
	}
	return out
}

// searchItems is every editable key, flat, for filtering.
func (m *model) searchItems() []list.Item {
	parents := m.parents()
	var out []list.Item
	for i := range m.sch.Settings {
		st := &m.sch.Settings[i]
		if parents[st.Key] {
			continue
		}
		row := m.leafItem(st)
		if i := strings.LastIndex(st.Key, "."); i >= 0 {
			row.name, row.parent = st.Key[i+1:], strings.ReplaceAll(st.Key[:i], ".", " › ")
		} else {
			row.name = st.Key
		}
		out = append(out, row)
	}
	return append(out, m.toolItems()...)
}

func (m *model) leafItem(st *schema.Setting) item {
	it := item{st: st}
	if v, sc := m.effective(st); sc != "" {
		it.val, it.scope = store.Format(v), string(sc)
	}
	return it
}

// countSet counts nested keys that have a value in some file.
func (m *model) countSet(prefix string) int {
	n := 0
	for i := range m.sch.Settings {
		st := &m.sch.Settings[i]
		if strings.HasPrefix(st.Key, prefix+".") && !m.parents()[st.Key] {
			if _, sc := m.effective(st); sc != "" {
				n++
			}
		}
	}
	return n
}

func (m *model) selectedItem() (item, bool) {
	it, ok := m.activeList().SelectedItem().(item)
	return it, ok
}

func (m *model) onGroup() bool {
	it, ok := m.selectedItem()
	return ok && it.group
}

// setLevel rebuilds the Settings list for the current path and selects key,
// or the first row when key isn't on this level.
func (m *model) setLevel(key string) tea.Cmd {
	if len(m.path) > 0 && strings.Join(m.path, ".") == m.autoGroup() {
		m.path = nil
	}
	m.list.Title = m.paneTitle()
	cmd := m.list.SetItems(m.items())
	m.list.Select(0)
	for i, li := range m.list.Items() {
		if it := li.(item); !it.header && it.st.Key == key {
			m.list.Select(i)
			break
		}
	}
	m.skipHeader(1)
	m.refreshDoc()
	return paneCommand(cmd)
}

func (m *model) openGroup() tea.Cmd {
	it, _ := m.selectedItem()
	m.path = strings.Split(it.st.Key, ".")
	return m.setLevel("")
}

// back leaves the current group and selects it in its parent level.
func (m *model) back() tea.Cmd {
	from := m.levelPrefix()
	m.path = m.path[:len(m.path)-1]
	return m.setLevel(from)
}

// reveal opens the section and level that hold key and selects it.
func (m *model) reveal(key string) {
	m.sideFocus = false
	section := toolsSection
	if st := m.sch.Get(key); st != nil {
		section = st.Section
	}
	for i, sec := range m.sections {
		if sec == section {
			m.sec = i
		}
	}
	parts := strings.Split(key, ".")
	m.path = nil
	for i := 1; i < len(parts); i++ {
		if !m.parents()[strings.Join(parts[:i], ".")] {
			break
		}
		m.path = parts[:i]
	}
	m.setLevel(key)
}

// skipHeader moves the cursor off section headings in the given direction,
// turning around at either end of the list.
func (m *model) skipHeader(dir int) {
	l := m.activeList()
	items := l.Items()
	if len(items) == 0 {
		return
	}
	i := l.Index()
	for tries := 0; tries < 2; tries++ {
		for i >= 0 && i < len(items) {
			if !items[i].(item).header {
				l.Select(i)
				return
			}
			i += dir
		}
		dir = -dir
		i = l.Index()
	}
}

func (m *model) breadcrumb() string {
	p := m.path
	if a := m.autoGroup(); a != "" && len(p) > 0 && p[0] == a {
		p = p[1:]
	}
	return strings.Join(p, " › ")
}

func (m *model) groupDoc(it item, w int) string {
	s := m.st
	var b strings.Builder
	b.WriteString(s.title.Render(strings.ReplaceAll(it.st.Key, ".", " › ")) + "\n")
	b.WriteString(s.subtle.Render(fmt.Sprintf("%s  group of settings", it.st.Section)) + "\n\n")
	if it.st.Desc != "" {
		b.WriteString(docDescription(s, it.st, w))
	}
	b.WriteString(s.label.Render("Inside") + "  " + s.subtle.Render("press enter to open") + "\n")
	saved := m.path
	m.path = strings.Split(it.st.Key, ".")
	children := m.levelItems()
	m.path = saved
	for _, li := range children {
		c := li.(item)
		right := s.faint.Render("–")
		switch {
		case c.group && c.nSet > 0:
			right = s.set.Render(fmt.Sprintf("%d set", c.nSet)) + s.subtle.Render(" ›")
		case c.group:
			right = s.subtle.Render("›")
		case c.val != "":
			right = s.set.Render(short(c.val, max(4, w/2))) + " " + s.subtle.Render(c.scope)
		}
		name := short(c.name, max(4, w-6-lipgloss.Width(right)))
		b.WriteString("  " + s.value.Render(name) + strings.Repeat(" ", max(2, w-2-lipgloss.Width(name)-lipgloss.Width(right))) + right + "\n")
	}
	if it.st.Desc != "" {
		b.WriteString("\n" + docMetadata(s, it.st, w))
	}
	return b.String()
}

// toolsSection is the reference's own Tools section; the tool on/off rows
// join it so everything about tools sits in one pane.
const toolsSection = "Tools"

// shortNames label the sidebar; the pane title keeps the full section name.
var shortNames = map[string]string{
	"Model and responses":                "Model",
	"Permission settings":                "Permissions",
	"Sandbox settings":                   "Sandbox",
	"Memory and context":                 "Memory",
	"Interface and terminal":             "Interface",
	"Git and attribution":                "Git",
	"Hooks and automation":               "Hooks",
	"Plugins and skills":                 "Plugins",
	"MCP":                                "MCP servers",
	"Agents, sessions, and worktrees":    "Agents",
	"Remote, desktop, and notifications": "Remote",
	"Authentication and providers":       "Sign-in",
	"Updates and versioning":             "Updates",
	"Privacy and telemetry":              "Privacy",
	"Enterprise and managed settings":    "Managed",
	"Global config settings":             "Global config",
}

func shortName(section string) string {
	if n := shortNames[section]; n != "" {
		return n
	}
	return section
}

func (m *model) section() string { return m.sections[m.sec] }

// setSection opens section i (clamped) at its top level.
func (m *model) setSection(i int) tea.Cmd {
	m.sec = max(0, min(len(m.sections)-1, i))
	m.path = nil
	if m.searching {
		m.searching = false
		m.list.ResetFilter()
	}
	return m.setLevel("")
}

func (m *model) startSearch() {
	m.searching = true
	m.list.Title = m.paneTitle()
	m.list.SetItems(m.items())
}

// sectionCount is how many keys in a section have a value, or for Tools how
// many tools are turned off.
func (m *model) sectionCount(section string) int {
	n := 0
	for i := range m.sch.Settings {
		st := &m.sch.Settings[i]
		if st.Section == section && !m.parents()[st.Key] {
			if _, sc := m.effective(st); sc != "" {
				n++
			}
		}
	}
	if section == toolsSection {
		for i := range toolSettings {
			if m.toolDeniedIn(&toolSettings[i]) != "" {
				n++
			}
		}
	}
	return n
}

// sidebar lists every section; the open one is marked, and a count shows
// how many of its keys have a value.
func (m *model) sidebar(w, h int) string {
	s := m.st
	lines := []string{s.subtle.Render("Sections"), ""}
	for i, sec := range m.sections {
		name := shortName(sec)
		right := ""
		if n := m.sectionCount(sec); n > 0 {
			right = s.set.Render(fmt.Sprint(n))
		}
		name = short(name, max(1, w-2-lipgloss.Width(right)-1))
		style, gutter := s.value, "  "
		if i == m.sec {
			style, gutter = s.key, s.accent.Render("▌ ")
			if !m.sideFocus {
				gutter = s.subtle.Render("▌ ")
			}
		}
		gap := max(1, w-2-lipgloss.Width(name)-lipgloss.Width(right))
		lines = append(lines, gutter+style.Render(name)+strings.Repeat(" ", gap)+right)
	}
	// Keep the open section visible when the terminal is short.
	if len(lines) > h {
		start := min(len(lines)-h, max(0, m.sec+2-h/2))
		lines = lines[start : start+h]
	}
	return strings.Join(lines, "\n")
}
