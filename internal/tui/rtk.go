package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jorden-parker/claude-config-cli/internal/rtk"
	"github.com/jorden-parker/claude-config-cli/internal/schema"
	"github.com/jorden-parker/claude-config-cli/internal/statusline"
)

const rtkSection = "RTK"

// Virtual rows backed by two PreToolUse hook entries, never settings keys.
const (
	rtkBash = "rtk.bash"
	rtkRead = "rtk.read"
)

// rtkBinary is swapped out in tests, which may not have rtk installed.
var rtkBinary = rtk.Binary

func isRTK(st *schema.Setting) bool { return st != nil && st.Section == rtkSection }

func (m *model) rtkItems() []list.Item {
	st := rtk.On(m.files[m.scope])
	row := func(key, name, desc string, on bool) item {
		it := item{name: name, st: &schema.Setting{Key: key, Section: rtkSection, Kind: schema.KindBool, ScopeClass: schema.ScopeAny, Desc: desc}}
		it.tool, it.off = true, !on
		if on {
			it.scope = string(m.scope)
		}
		return it
	}
	return []list.Item{
		row(rtkBash, "Rewrite Bash commands", "rtk's own PreToolUse hook, the one rtk init -g registers. It rewrites commands such as cat, grep, pnpm, and bun to their rtk versions, so Claude reads a condensed result. Commands rtk does not know pass through unchanged.", st.Bash),
		row(rtkRead, "Refuse the Read tool", "ccfg's PreToolUse hook for the built-in Read tool. Built-in tools never pass through the Bash hook, so this refuses Read and tells Claude to run rtk read in Bash instead. Edits keep working: Claude Code counts a plain cat, head, tail, sed -n, or grep on one file as the read before an Edit, and the hook suggests sed -n, which rtk leaves alone.", st.Read),
	}
}

// rtkAct is tab or enter on an rtk row: it flips that hook in the target file.
func (m *model) rtkAct(st *schema.Setting) tea.Cmd {
	sc := m.scope
	if err := m.errs[sc]; err != nil {
		m.setStatus("Can't edit a settings file that failed to load: "+err.Error(), true)
		return nil
	}
	cur := rtk.On(m.files[sc])
	staged := *m.files[sc]
	var err error
	var msg string
	switch st.Key {
	case rtkBash:
		if !cur.Bash {
			if _, err = rtkBinary(); err != nil {
				m.setStatus(err.Error(), true)
				return nil
			}
		}
		err = rtk.SetBash(&staged, !cur.Bash)
		msg = "Bash rewrite " + onOff(!cur.Bash)
	case rtkRead:
		err = rtk.SetRead(&staged, !cur.Read, rtk.ReadCommand(sc, statusline.ProgramPath()))
		msg = "Read refusal " + onOff(!cur.Read)
	default:
		return nil
	}
	if err == nil {
		err = staged.Save()
	}
	if err != nil {
		m.setStatus("Not saved: "+err.Error(), true)
		return nil
	}
	m.files[sc] = &staged
	m.setStatus(msg+" in "+tildify(staged.Path)+". Restart Claude Code to pick it up.", false)
	if m.searching || m.list.IsFiltered() {
		return m.afterWrite()
	}
	return m.setLevel(st.Key)
}

func (m *model) rtkDoc(st *schema.Setting, w int) string {
	s := m.st
	wrap := lipgloss.NewStyle().Width(w)
	cur := rtk.On(m.files[m.scope])
	var b strings.Builder
	b.WriteString(s.title.Render("RTK") + "  " + s.subtle.Render("route command output through rtk") + "\n")
	b.WriteString(s.subtle.Render(short(fmt.Sprintf("%s file · %s", m.scope, tildify(m.files[m.scope].Path)), w)) + "\n\n")
	if p, err := rtkBinary(); err != nil {
		b.WriteString(s.warn.Render(wrap.Render("rtk is not on your PATH. Install it with: brew install rtk")) + "\n\n")
	} else {
		b.WriteString(s.label.Render("rtk binary") + "  " + s.subtle.Render(p) + "\n\n")
	}

	it, _ := m.selectedItem()
	b.WriteString(s.label.Render(it.name) + "  " + s.subtle.Render(onOff(!it.off)) + "\n" + wrap.Render(st.Desc) + "\n\n")

	b.WriteString(s.label.Render("Hooks in this file") + "\n")
	line := func(name string, on bool, command string) {
		mark := s.faint.Render("○")
		if on {
			mark = s.ok.Render("●")
		}
		b.WriteString(mark + " " + name + "  " + s.subtle.Render(short(command, max(1, w-len(name)-5))) + "\n")
	}
	line("Bash", cur.Bash, rtk.BashHook)
	line("Read", cur.Read, rtk.ReadCommand(m.scope, statusline.ProgramPath()))
	b.WriteString("\n" + s.subtle.Render(wrap.Render("Grep and Glob need nothing: on macOS and Linux, Claude Code searches with find and grep through Bash, which the Bash hook already covers. Other hooks in the file are left alone. Hooks are read once at session start, so restart Claude Code after a change.")) + "\n")

	if real := m.sch.Get("hooks"); real != nil {
		b.WriteString("\n" + m.ladder(real, w))
	}
	b.WriteString("\n" + s.accent.Render(rtk.DocURL) + "\n")
	return b.String()
}
