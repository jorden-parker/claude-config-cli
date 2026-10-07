package tui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
	"github.com/jorden-parker/claude-config-cli/internal/statusline"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

const statusSection = "Status line"

// Virtual rows: these are backed by the statusLine key and ccfg's own status
// line config file, never standalone settings keys.
const (
	slUse       = "statusline.use"
	slExisting  = "statusline.existing"
	slSeparator = "statusline.separator"
	slColor     = "statusline.color"
	slField     = "statusline.field."
)

// statusCheck is swapped out in tests, which have no installed binary.
var statusCheck = statusline.Check

func isStatus(st *schema.Setting) bool { return st != nil && st.Section == statusSection }

// statusState is the status line config for the target file, the command it
// keeps, and that command's output for the preview.
type statusState struct {
	cfg      *statusline.Config
	cfgErr   error
	scope    store.Scope // file cfg was loaded for
	kept     string      // existing status line command
	keptFrom store.Scope // settings file it comes from
	ranFor   string      // command the output below belongs to
	out      string
	runErr   string
	running  bool
}

type statusPreviewMsg struct {
	command, out string
	err          error
}

// statusSync reloads the config when the target file changed and looks up
// the existing command again.
func (m *model) statusSync() {
	if m.sl.cfg == nil || m.sl.scope != m.scope {
		m.sl.cfg, m.sl.cfgErr = statusline.Load(m.statusPath())
		m.sl.scope = m.scope
	}
	m.sl.kept, m.sl.keptFrom = statusline.Existing(m.files, m.scope, m.cwd)
}

func (m *model) statusPath() string { return statusline.ConfigPath(m.scope, m.cwd) }

// A project's files can come from a repository you haven't vetted, so only a
// command from your own user settings runs without being asked.
func (m *model) statusTrusted() bool {
	return m.sl.keptFrom == store.ScopeUser || m.sl.keptFrom == store.ScopeManaged
}

// statusPreview runs the existing command off the UI thread so its output
// can join the preview. It runs once per command unless forced.
func (m *model) statusPreview(force bool) tea.Cmd {
	command := m.sl.kept
	if command == "" || m.sl.cfg.Existing == statusline.ExistingHidden {
		return nil
	}
	if !force && (command == m.sl.ranFor || !m.statusTrusted()) {
		return nil
	}
	m.sl.ranFor, m.sl.out, m.sl.runErr, m.sl.running = command, "", "", true
	input := statusline.Sample(m.cwd, time.Now())
	return func() tea.Msg {
		out, err := statusline.RunInherited(command, input)
		return statusPreviewMsg{command: command, out: out, err: err}
	}
}

func (m *model) statusPreviewDone(msg statusPreviewMsg) {
	if msg.command != m.sl.ranFor {
		return
	}
	m.sl.running, m.sl.out, m.sl.runErr = false, msg.out, ""
	if msg.err != nil {
		m.sl.runErr = msg.err.Error()
	}
	m.refreshDoc()
}

func (m *model) statusItems() []list.Item {
	cfg := m.sl.cfg
	row := func(key, name, desc string) item {
		return item{name: name, st: &schema.Setting{Key: key, Section: statusSection, Kind: schema.KindBool, ScopeClass: schema.ScopeAny, Desc: desc}}
	}
	toggle := func(it item, on bool) item {
		it.tool, it.off = true, !on
		return it
	}
	existing := row(slExisting, "Existing status line", "Where the status line you already had goes: at the start or end of the line, on its own row above or below, or hidden. It is kept either way, never deleted.")
	existing.val = "none"
	if m.sl.kept != "" {
		existing.val = cfg.Existing
	}
	separator := row(slSeparator, "Separator", "The text between fields.")
	separator.val = strconv.Quote(cfg.Separator)
	out := []list.Item{
		toggle(row(slUse, "Use this status line", "Point statusLine at ccfg's renderer. The command that is already set is kept and its output shown beside your fields. Turning this off puts the original back."), statusline.On(m.files[m.scope])),
		existing,
		separator,
		toggle(row(slColor, "Colour", "Colour each field. The existing status line keeps its own colours either way."), cfg.Color),
	}

	now := time.Now()
	sample := statusline.Sample(m.cwd, now)
	field := func(f *statusline.Field) item {
		return row(slField+f.ID, f.Name, f.Desc+" Reads "+f.Source+".")
	}
	if len(cfg.Fields) > 0 {
		out = append(out, item{header: true, section: "Shown, in order"})
	}
	for _, id := range cfg.Fields {
		if f := statusline.Lookup(id); f != nil {
			it := field(f)
			if it.val = f.Sample(sample, now); it.val == "" {
				it.val = "on"
			}
			out = append(out, it)
		}
	}
	out = append(out, item{header: true, section: "More fields"})
	for i := range statusline.Fields {
		if f := &statusline.Fields[i]; !cfg.Has(f.ID) {
			out = append(out, toggle(field(f), false))
		}
	}
	return out
}

// statusAct is tab or enter on a status line row.
func (m *model) statusAct(st *schema.Setting) tea.Cmd {
	if err := m.sl.cfgErr; err != nil {
		m.setStatus("Can't read "+tildify(m.statusPath())+": "+err.Error(), true)
		return nil
	}
	cfg := m.sl.cfg
	var msg string
	switch st.Key {
	case slUse:
		return m.statusToggleUse()
	case slExisting:
		if m.sl.kept == "" {
			m.setStatus("No status line was set before, so there is nothing to place.", false)
			return nil
		}
		cfg.Existing = next(statusline.Positions, cfg.Existing)
		msg = "Existing status line: " + cfg.Existing
	case slSeparator:
		cfg.Separator = next(statusline.Separators, cfg.Separator)
		msg = "Separator: " + strconv.Quote(cfg.Separator)
	case slColor:
		cfg.Color = !cfg.Color
		msg = "Colour " + onOff(cfg.Color)
	default:
		f := statusline.Lookup(strings.TrimPrefix(st.Key, slField))
		if f == nil {
			return nil
		}
		msg = f.Name + " " + onOff(cfg.Toggle(f.ID))
	}
	return m.statusSave(st.Key, msg)
}

// statusMove reorders a shown field.
func (m *model) statusMove(st *schema.Setting, delta int) tea.Cmd {
	id, isField := strings.CutPrefix(st.Key, slField)
	if !isField || !m.sl.cfg.Has(id) {
		m.setStatus("Only shown fields can be reordered. Tab turns a field on.", false)
		return nil
	}
	if !m.sl.cfg.Move(id, delta) {
		return nil
	}
	return m.statusSave(st.Key, "Moved "+statusline.Lookup(id).Name)
}

func (m *model) statusSave(key, msg string) tea.Cmd {
	if err := statusline.Save(m.statusPath(), m.sl.cfg); err != nil {
		m.setStatus("Save failed: "+err.Error(), true)
		return nil
	}
	if !statusline.On(m.files[m.scope]) {
		msg += ". Not live yet: turn on \"Use this status line\""
	}
	m.setStatus(msg, false)
	return m.statusRefresh(key)
}

// statusRefresh redraws the rows and keeps the cursor on key, which may have
// moved between the shown and hidden fields.
func (m *model) statusRefresh(key string) tea.Cmd {
	if m.searching || m.list.IsFiltered() {
		return m.afterWrite()
	}
	m.statusSync()
	return tea.Batch(m.setLevel(key), m.statusPreview(false))
}

// statusToggleUse turns ccfg's status line on or off in the target file.
// It works on copies so a failed save does not change the displayed state.
func (m *model) statusToggleUse() tea.Cmd {
	sc := m.scope
	if err := m.errs[sc]; err != nil {
		m.setStatus("Can't edit a settings file that failed to load: "+err.Error(), true)
		return nil
	}
	staged := *m.files[sc]
	b, err := json.Marshal(staged.Data)
	if err == nil {
		staged.Data = map[string]any{}
		err = json.Unmarshal(b, &staged.Data)
	}
	cfg := *m.sl.cfg
	var msg string
	if err == nil && statusline.On(&staged) {
		if err = statusline.Disable(&staged, &cfg); err == nil {
			err = staged.Save()
		}
		msg = "Status line off. Removed statusLine from " + tildify(staged.Path)
		if v, ok := staged.Get("statusLine"); ok {
			msg = "Status line off. Put back " + short(statusline.CommandOf(v), 40) + " in " + tildify(staged.Path)
		}
	} else if err == nil {
		if err = statusline.Enable(&staged, &cfg, statusline.Command(sc, m.cwd), m.sl.kept); err == nil {
			err = statusline.Save(m.statusPath(), &cfg)
		}
		if err == nil {
			err = statusCheck(statusline.Command(sc, m.cwd), m.cwd)
		}
		if err == nil {
			err = staged.Save()
		}
		msg = "Status line on in " + tildify(staged.Path)
		if cfg.Inherited != "" {
			msg += ". Kept " + short(cfg.Inherited, 40)
		}
	}
	if err != nil {
		m.setStatus("Not saved: "+err.Error(), true)
		return nil
	}
	m.files[sc], m.sl.cfg = &staged, &cfg
	m.setStatus(msg, false)
	return m.statusRefresh(slUse)
}

// statusKey handles the keys only status line rows have.
func (m *model) statusKey(k tea.KeyPressMsg) (tea.Cmd, bool) {
	st := m.selected()
	if !isStatus(st) || m.sl.cfgErr != nil {
		return nil, false
	}
	switch k.String() {
	case "J", "shift+down":
		return m.statusMove(st, 1), true
	case "K", "shift+up":
		return m.statusMove(st, -1), true
	case "p":
		cmd := m.statusPreview(true)
		if cmd == nil {
			m.setStatus("No existing status line to run.", false)
		}
		m.refreshDoc()
		return cmd, true
	}
	return nil, false
}

// statusPreviewText is the status line for the docs' example session, with
// the existing command's real output when it has been run.
func (m *model) statusPreviewText() string {
	now := time.Now()
	out := ""
	if m.sl.ranFor == m.sl.kept && !m.sl.running {
		out = m.sl.out
	}
	return statusline.Compose(m.sl.cfg, statusline.Sample(m.cwd, now), out, now)
}

func (m *model) statusDoc(st *schema.Setting, w int) string {
	s := m.st
	wrap := lipgloss.NewStyle().Width(w)
	on := statusline.On(m.files[m.scope])
	state := s.subtle.Render("○ not in use")
	if on {
		state = s.ok.Render("● in use")
	}
	var b strings.Builder
	b.WriteString(s.title.Render("Status line") + "  " + state + "\n")
	b.WriteString(s.subtle.Render(short(fmt.Sprintf("%s file · %s", m.scope, tildify(m.statusPath())), w)) + "\n\n")
	if err := m.sl.cfgErr; err != nil {
		b.WriteString(s.err.Render(wrap.Render("Can't read the status line config: "+err.Error())) + "\n\n")
	}

	b.WriteString(s.label.Render("Preview") + "  " + s.subtle.Render("the docs' example session") + "\n")
	if preview := m.statusPreviewText(); strings.TrimSpace(preview) != "" {
		for _, line := range strings.Split(preview, "\n") {
			b.WriteString(ansi.Wrap(line, w, "") + "\n")
		}
	} else {
		b.WriteString(s.subtle.Render(wrap.Render("Nothing to show yet. Tab turns a field on.")) + "\n")
	}

	b.WriteString("\n" + s.label.Render("Existing status line") + "  " + s.subtle.Render("kept, never deleted") + "\n")
	switch {
	case m.sl.kept == "":
		b.WriteString(s.faint.Render("none set") + "\n")
	default:
		b.WriteString(wrap.Render(m.sl.kept) + "\n")
		b.WriteString(s.subtle.Render(fmt.Sprintf("from the %s file · placed: %s", m.sl.keptFrom, m.sl.cfg.Existing)) + "\n")
		switch {
		case m.sl.cfg.Existing == statusline.ExistingHidden:
			b.WriteString(s.subtle.Render(wrap.Render("Hidden, so it is not run.")) + "\n")
		case m.sl.running:
			b.WriteString(s.subtle.Render("Running it for the preview…") + "\n")
		case m.sl.ranFor != m.sl.kept:
			b.WriteString(s.warn.Render(wrap.Render(fmt.Sprintf("Not run yet: it comes from the %s file, which a repository can supply. Press p to run it for the preview.", m.sl.keptFrom))) + "\n")
		case m.sl.runErr != "":
			b.WriteString(s.warn.Render(wrap.Render("Left out of the preview: "+m.sl.runErr)) + "\n")
		case strings.TrimSpace(m.sl.out) == "":
			b.WriteString(s.subtle.Render(wrap.Render("It printed nothing for the example session.")) + "\n")
		}
	}

	it, _ := m.selectedItem()
	b.WriteString("\n" + s.label.Render(it.name) + "\n" + wrap.Render(st.Desc) + "\n")
	if strings.HasPrefix(st.Key, slField) && m.sl.cfg.Has(strings.TrimPrefix(st.Key, slField)) {
		b.WriteString(s.subtle.Render("J / K move it later or earlier.") + "\n")
	}

	if real := m.sch.Get("statusLine"); real != nil {
		b.WriteString("\n" + m.ladder(real, w))
		if _, winner := m.effective(real); on && winner != m.scope {
			b.WriteString("\n" + s.warn.Render(wrap.Render(fmt.Sprintf("The %s file also sets statusLine and wins, so this one is not shown.", winner))) + "\n")
		}
	}
	b.WriteString("\n" + s.accent.Render(statusline.DocURL) + "\n")
	return b.String()
}

// next returns the option after cur, wrapping around; an unknown cur starts over.
func next(opts []string, cur string) string {
	for i, o := range opts {
		if o == cur {
			return opts[(i+1)%len(opts)]
		}
	}
	return opts[0]
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}
