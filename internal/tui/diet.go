package tui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jorden-parker/claude-config-cli/internal/diet"
	"github.com/jorden-parker/claude-config-cli/internal/schema"
	"github.com/jorden-parker/claude-config-cli/internal/store"
	"github.com/jorden-parker/claude-config-cli/internal/value"
)

// dietSection is the checklist's pane in the section list, next to Status line.
const dietSection = "Context diet"

// dietKeyPrefix is the settings path of a checklist row that has no trap key
// to remove. A warning row uses its trap key as its path instead, so the
// remove confirmation names that key.
const dietKeyPrefix = "diet."

// dietRow is one checklist row with its result, as the list holds it.
type dietRow = diet.Row

func isDiet(st *schema.Setting) bool { return st != nil && st.Section == dietSection }

// isDietTrap reports whether a checklist row is pointing at a trap key that
// the target file holds, so enter can offer to remove it.
func isDietTrap(st *schema.Setting) bool {
	return isDiet(st) && !strings.HasPrefix(st.Key, dietKeyPrefix)
}

// dietInput gives the checks the files that loaded. A file that failed to load
// is left out, so its half-read contents never count.
func (m *model) dietInput() diet.Input {
	files := map[store.Scope]*store.File{}
	for sc, f := range m.files {
		if f != nil && m.errs[sc] == nil {
			files[sc] = f
		}
	}
	home, _ := os.UserHomeDir()
	return diet.Input{Files: files, Cwd: m.cwd, Home: home}
}

// dietTrapHere returns the first offender the target file holds.
func (m *model) dietTrapHere(res diet.Result) (diet.Offender, bool) {
	f := m.files[m.scope]
	if f == nil || m.errs[m.scope] != nil {
		return diet.Offender{}, false
	}
	for _, o := range res.Offenders {
		if _, ok := f.Get(o.Key); ok {
			return o, true
		}
	}
	return diet.Offender{}, false
}

// dietItems builds the checklist: a summary line, then the rows in two groups.
// Every row is listed, whatever its state.
func (m *model) dietItems() []list.Item {
	rows := diet.Evaluate(m.dietInput())
	out := []list.Item{item{header: true, section: diet.Summarize(rows).Line()}}
	group := ""
	for _, row := range rows {
		if row.Rec.Group != group {
			group = row.Rec.Group
			out = append(out, item{header: true, section: diet.GroupTitle(group)})
		}
		r := row
		st := &schema.Setting{Key: dietKeyPrefix + row.Rec.ID, Section: dietSection, Kind: schema.KindGroup, ScopeClass: schema.ScopeAny, Desc: row.Rec.Why}
		if o, ok := m.dietTrapHere(row.Result); ok {
			st.Key = o.Key
		}
		out = append(out, item{name: row.Rec.Title, st: st, diet: &r})
	}
	return out
}

// dietAct is enter (or tab) on a checklist row.
func (m *model) dietAct() tea.Cmd {
	it, ok := m.selectedItem()
	if !ok || it.diet == nil {
		return nil
	}
	rec, res := it.diet.Rec, it.diet.Result
	switch {
	case res.State == diet.Applied:
		m.setStatus(m.dietAppliedMsg(rec, res), false)
		return nil
	case len(res.Offenders) > 0 && (res.State == diet.Warning || res.State == diet.NotApplied):
		return m.dietRemove(res)
	case res.State == diet.Warning:
		m.dietShow(res.Note + ". ccfg has no single key to remove here, so change it by hand.")
		return nil
	}
	switch rec.Apply.Kind {
	case diet.ApplySet:
		return m.dietSet(rec.Apply)
	case diet.ApplyEditor:
		return m.dietEditor(rec.Apply.Key)
	}
	m.dietShow(rec.Apply.Text)
	return nil
}

func (m *model) dietAppliedMsg(rec diet.Rec, res diet.Result) string {
	switch {
	case res.Default:
		return rec.Title + " is already applied by default. Nothing to change."
	case res.Scope != "":
		return fmt.Sprintf("Already applied in the %s file (%s). Nothing to change.", res.Scope, tildify(store.Path(res.Scope, m.cwd)))
	}
	return "Already applied: " + res.Note + ". Nothing to change."
}

// dietRemove asks before removing a trap key from the target file, reusing the
// u confirmation. The row's key was set to the trap when the list was built.
func (m *model) dietRemove(res diet.Result) tea.Cmd {
	if _, ok := m.dietTrapHere(res); !ok {
		o := res.Offenders[0]
		m.setStatus(fmt.Sprintf("%s is set in the %s file, not %s. Press s to pick that file, then enter.", o.Key, o.Scope, m.scope), true)
		return nil
	}
	m.mode = modeConfirmUnset
	return nil
}

// dietSet writes a fixed value through the same checks and save path the
// settings editor uses.
func (m *model) dietSet(a diet.Apply) tea.Cmd {
	st := m.applySetting(a.Key)
	if st == nil {
		m.setStatus("ccfg doesn't know "+a.Key+" yet.", true)
		return nil
	}
	sc := m.targetScope(st)
	if !store.Allowed(st, sc) {
		m.setStatus(notReadHere(st, sc), true)
		return nil
	}
	v, err := value.Parse(st, a.Value)
	if err != nil {
		m.setStatus(err.Error(), true)
		return nil
	}
	if strings.HasPrefix(a.Key, "env.") {
		return m.saveEnv(st, v, false)
	}
	f, ok := m.editableFile(sc)
	if !ok {
		return nil
	}
	return m.saveValue(st, f, v)
}

// applySetting is the catalogue entry for a key, or a stand-in for an env var.
func (m *model) applySetting(key string) *schema.Setting {
	if name, ok := strings.CutPrefix(key, "env."); ok {
		return &schema.Setting{Key: "env." + name, Section: dietSection, Kind: schema.KindString, ScopeClass: schema.ScopeAny, Type: "string"}
	}
	return m.sch.Get(key)
}

// dietEditor opens the settings editor on key, for rows where the value is a
// choice rather than one fixed value.
func (m *model) dietEditor(key string) tea.Cmd {
	if m.sch.Get(key) == nil {
		m.setStatus("ccfg doesn't know "+key+" yet.", true)
		return nil
	}
	if m.searching {
		m.searching = false
		m.list.ResetFilter()
	}
	m.reveal(key)
	return m.startEdit()
}

// dietShow opens the details pane with a message, for rows with nothing to write.
func (m *model) dietShow(msg string) {
	m.details = true
	m.layout()
	m.setStatus(msg, false)
}

// dietURL is the docs page for the selected row.
func (m *model) dietURL() string {
	if it, ok := m.selectedItem(); ok && it.diet != nil {
		return it.diet.Rec.URL
	}
	return ""
}

// dietVerb names what enter does on the selected row, for the key hints.
func (m *model) dietVerb() string {
	it, ok := m.selectedItem()
	if !ok || it.diet == nil {
		return "details"
	}
	rec, res := it.diet.Rec, it.diet.Result
	if len(res.Offenders) > 0 && (res.State == diet.Warning || res.State == diet.NotApplied) {
		return "remove"
	}
	if res.State == diet.Applied || res.State == diet.Warning {
		return "details"
	}
	switch rec.Apply.Kind {
	case diet.ApplySet:
		return "apply"
	case diet.ApplyEditor:
		return "choose"
	}
	return "details"
}

// dietDoc is the details pane for the selected row.
func (m *model) dietDoc(w int) string {
	it, ok := m.selectedItem()
	if !ok || it.diet == nil {
		return ""
	}
	rec, res := it.diet.Rec, it.diet.Result
	s := m.st
	wrap := lipgloss.NewStyle().Width(w)
	var b strings.Builder
	b.WriteString(s.title.Render(rec.Title) + "\n")
	b.WriteString(dietBadge(s, res) + "  " + s.subtle.Render(diet.GroupTitle(rec.Group)) + "\n\n")
	b.WriteString(s.label.Render("What it does") + "\n" + wrap.Render(rec.Why) + "\n\n")
	if rec.Caveat != "" {
		b.WriteString(s.label.Render("Caveat") + "\n" + wrap.Render(rec.Caveat) + "\n\n")
	}
	b.WriteString(s.label.Render("Now") + "\n" + wrap.Render(res.Note) + "\n\n")
	if len(rec.Keys) > 0 {
		b.WriteString(s.label.Render("Where it's set") + "  " + s.subtle.Render("higher rows win") + "\n")
		for _, key := range rec.Keys {
			b.WriteString(m.dietLadder(key, w))
		}
		b.WriteString("\n")
	}
	b.WriteString(s.label.Render("How to apply") + "  " + s.subtle.Render(m.dietHow(it.diet)) + "\n")
	b.WriteString(wrap.Render(rec.Apply.Text) + "\n\n")
	if rec.Confidence != "" {
		b.WriteString(s.subtle.Render("Confidence  "+rec.Confidence) + "\n")
	}
	b.WriteString(s.accent.Render(rec.URL) + "\n")
	return b.String()
}

// dietHow is the short hint beside "How to apply".
func (m *model) dietHow(row *dietRow) string {
	res := row.Result
	if len(res.Offenders) > 0 && (res.State == diet.Warning || res.State == diet.NotApplied) {
		return "enter offers to remove the trap key from the target file"
	}
	switch {
	case res.State == diet.Applied:
		return "already applied, nothing to do"
	case row.Rec.Apply.Kind == diet.ApplySet:
		return "enter writes it to the target file (s changes the file)"
	case row.Rec.Apply.Kind == diet.ApplyEditor:
		return "enter opens the editor for " + row.Rec.Apply.Key
	case row.Rec.Apply.Kind == diet.ApplyManual:
		return "by hand"
	}
	return "nothing to change"
}

// dietLadder lists the files that may set a key, highest precedence first,
// with the value each holds and the target file marked.
func (m *model) dietLadder(key string, w int) string {
	s := m.st
	if !m.setAnywhere(key) {
		return s.value.Render(key) + "  " + s.faint.Render("not set in any file") + "\n"
	}
	var b strings.Builder
	b.WriteString(s.value.Render(key) + "\n")
	won := false
	for _, sc := range precedence() {
		var v any
		ok := false
		if f := m.files[sc]; f != nil {
			v, ok = f.Get(key)
		}
		tag := ""
		if sc == m.scope {
			tag = s.accent.Render("◂ target file")
		}
		room := max(4, w-14-lipgloss.Width(tag))
		marker, val := "  ", s.faint.Render("–")
		switch {
		case ok && !won:
			won = true
			marker, val = s.set.Render("● "), s.set.Render(short(store.Format(v), room))
		case ok:
			marker, val = s.subtle.Render("○ "), s.subtle.Strikethrough(true).Render(short(store.Format(v), room))
		}
		name := s.value.Render(fmt.Sprintf("%-9s", sc))
		if sc == m.scope {
			name = s.accent.Bold(true).Render(fmt.Sprintf("%-9s", sc))
		}
		line := "  " + marker + name + " " + val
		if tag != "" {
			line += strings.Repeat(" ", max(2, w-lipgloss.Width(line)-lipgloss.Width(tag))) + tag
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// setAnywhere reports whether any settings file holds key.
func (m *model) setAnywhere(key string) bool {
	for _, sc := range precedence() {
		if f := m.files[sc]; f != nil {
			if _, ok := f.Get(key); ok {
				return true
			}
		}
	}
	return false
}

// precedence is the order the checklist reads a settings key, highest first.
func precedence() []store.Scope {
	return []store.Scope{store.ScopeManaged, store.ScopeLocal, store.ScopeProject, store.ScopeUser}
}
