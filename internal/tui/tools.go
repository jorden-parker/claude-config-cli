package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jorden-parker/claude-config-cli/internal/schema"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

// Virtual rows: these are backed by permissions.deny, never standalone settings keys.
// Catalogue: https://code.claude.com/docs/en/tools-reference
var toolSettings = func() []schema.Setting {
	entries := []struct{ name, desc string }{
		{"Agent", "Delegate work to subagents."},
		{"Artifact", "Publish shareable interactive pages."},
		{"AskUserQuestion", "Ask you multiple-choice questions."},
		{"Bash", "Run shell commands."},
		{"CronCreate", "Schedule prompts within this session."},
		{"CronDelete", "Cancel a scheduled prompt."},
		{"CronList", "Show this session's scheduled prompts."},
		{"Edit", "Replace selected file content."},
		{"EndConversation", "End the current conversation."},
		{"EnterPlanMode", "Plan changes before implementation."},
		{"EnterWorktree", "Create or enter a worktree."},
		{"ExitPlanMode", "Request approval of a plan."},
		{"ExitWorktree", "Return from a worktree."},
		{"Glob", "Locate files by filename pattern."},
		{"Grep", "Find matching text inside files."},
		{"ListAgents", "Find agents available for messaging."},
		{"ListMcpResourcesTool", "Discover connected MCP resources."},
		{"LSP", "Inspect symbols, references, and diagnostics."},
		{"Monitor", "Watch command or WebSocket events."},
		{"NotebookEdit", "Change Jupyter notebook cells."},
		{"PowerShell", "Run commands using PowerShell."},
		{"PushNotification", "Notify your desktop or phone."},
		{"Read", "Open files for inspection."},
		{"ReadMcpResourceTool", "Open an MCP resource."},
		{"RemoteTrigger", "Manage routines on claude.ai."},
		{"ReportFindings", "Present structured code-review findings."},
		{"ScheduleWakeup", "Reschedule or stop self-paced loops."},
		{"SendFeedback", "Draft feedback for your review."},
		{"SendMessage", "Message agents or other sessions."},
		{"SendUserFile", "Deliver files to your device."},
		{"ShareOnboardingGuide", "Upload and share ONBOARDING.md."},
		{"Skill", "Run a skill's instructions."},
		{"SubagentHandback", "Return a subagent's final report."},
		{"TaskCreate", "Add a tracked task."},
		{"TaskGet", "Inspect one tracked task."},
		{"TaskList", "Show tracked tasks and status."},
		{"TaskOutput", "Read background-task output (deprecated)."},
		{"TaskStop", "Halt a background task."},
		{"TaskUpdate", "Modify or delete tracked tasks."},
		{"TodoWrite", "Maintain the legacy task checklist."},
		{"ToolSearch", "Discover and load deferred tools."},
		{"WaitForMcpServers", "Await pending MCP connections."},
		{"WebFetch", "Retrieve a web page."},
		{"WebSearch", "Find information online."},
		{"Workflow", "Orchestrate subagents through scripts."},
		{"Write", "Save or overwrite entire files."},
	}
	out := make([]schema.Setting, len(entries))
	for i, entry := range entries {
		out[i] = schema.Setting{Key: entry.name, Desc: entry.desc, Section: "Tools", Kind: schema.KindBool, ScopeClass: schema.ScopeAny}
	}
	return out
}()

func toolName(st *schema.Setting) string {
	for i := range toolSettings {
		if st == &toolSettings[i] {
			return st.Key
		}
	}
	return ""
}

func toolRules(f *store.File) ([]string, error) {
	v, ok := f.Get("permissions.deny")
	if !ok {
		return nil, nil
	}
	switch rules := v.(type) {
	case []string:
		return rules, nil
	case []any:
		out := make([]string, len(rules))
		for i, rule := range rules {
			s, ok := rule.(string)
			if !ok {
				return nil, fmt.Errorf("permissions.deny must contain only strings")
			}
			out[i] = s
		}
		return out, nil
	default:
		return nil, fmt.Errorf("permissions.deny must be an array of strings")
	}
}

// toolDeniedIn returns the highest-precedence file whose permissions.deny
// lists the bare tool name, or "" if the tool is on everywhere.
func (m *model) toolDeniedIn(st *schema.Setting) store.Scope {
	for _, sc := range []store.Scope{store.ScopeManaged, store.ScopeLocal, store.ScopeProject, store.ScopeUser} {
		if denies(m.files[sc], toolName(st)) {
			return sc
		}
	}
	return ""
}

func denies(f *store.File, name string) bool {
	if f == nil {
		return false
	}
	rules, _ := toolRules(f)
	for _, rule := range rules {
		if rule == name {
			return true
		}
	}
	return false
}

func (m *model) toggleTool(st *schema.Setting) tea.Cmd {
	sc := m.targetScope(st)
	if !store.Allowed(st, sc) {
		m.setStatus("Tools can only be toggled in user, project, or local settings", true)
		return nil
	}
	if err := m.errs[sc]; err != nil {
		m.setStatus("Can't edit a settings file that failed to load: "+err.Error(), true)
		return nil
	}
	f := m.files[sc]
	rules, err := toolRules(f)
	if err != nil {
		m.setStatus(err.Error(), true)
		return nil
	}
	name := toolName(st)
	next := make([]string, 0, len(rules)+1)
	disabled := false
	for _, rule := range rules {
		if rule == name {
			disabled = true
		} else {
			next = append(next, rule)
		}
	}
	state := "on"
	if !disabled {
		next = append(next, name)
		state = "off"
	}
	// Work on a copy so a failed save does not change the displayed state.
	var data map[string]any
	if err := jsonUnmarshal(store.Format(f.Data), &data); err != nil {
		m.setStatus(err.Error(), true)
		return nil
	}
	updated := *f
	updated.Data = data
	if err := updated.Set("permissions.deny", next); err != nil {
		m.setStatus(err.Error(), true)
		return nil
	}
	if err := updated.Save(); err != nil {
		m.setStatus("Save failed: "+err.Error(), true)
		return nil
	}
	m.files[sc] = &updated
	m.setStatus(fmt.Sprintf("Turned %s %s in %s", name, state, tildify(f.Path)), false)
	return m.afterWrite()
}

func (m *model) toolDoc(st *schema.Setting, w int) string {
	s := m.st
	name := toolName(st)
	state := s.ok.Render("on")
	if m.toolDeniedIn(st) != "" {
		state = s.err.Render("off")
	}
	var b strings.Builder
	b.WriteString(s.title.Render(name) + "\n" + s.subtle.Render("Claude Code tool  ") + state + "\n\n")
	b.WriteString(docDescription(s, st, w))
	b.WriteString(s.label.Render("Where it's turned off") + "  " + s.subtle.Render("any file can turn it off") + "\n")
	for _, sc := range []store.Scope{store.ScopeManaged, store.ScopeLocal, store.ScopeProject, store.ScopeUser} {
		label := s.value.Render(fmt.Sprintf("%-9s", sc))
		if sc == m.scope {
			label = s.accent.Bold(true).Render(fmt.Sprintf("%-9s", sc))
		}
		val := s.faint.Render("–")
		if _, err := toolRules(m.files[sc]); err != nil {
			val = s.err.Render(err.Error())
		} else if denies(m.files[sc], name) {
			val = s.err.Render("disabled")
		}
		line := "  " + label + " " + val
		tag := ""
		switch {
		case sc == m.scope:
			tag = s.accent.Render("◂ target file")
		case sc == store.ScopeManaged:
			tag = s.subtle.Render("read-only")
		}
		if tag != "" {
			line += strings.Repeat(" ", max(2, w-lipgloss.Width(line)-lipgloss.Width(tag))) + tag
		}
		b.WriteString(line + "\n")
	}
	wrap := lipgloss.NewStyle().Width(w)
	b.WriteString("\n" + wrap.Render(fmt.Sprintf("Press tab or enter to turn %s on or off in the target file.", name)) + "\n\n")
	b.WriteString(s.subtle.Render(wrap.Render(fmt.Sprintf("Turning it off adds %q to permissions.deny. Turning it on removes that entry; the usual permission prompts still apply. Narrower rules such as %s(…) stay as they are. Whether a tool exists also depends on your Claude Code version.", name, name))) + "\n\n")
	b.WriteString(s.accent.Render("https://code.claude.com/docs/en/tools-reference") + "\n")
	return b.String()
}
