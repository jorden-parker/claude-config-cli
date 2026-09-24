package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/jorden-parker/claude-config-cli/internal/schema"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

// Virtual rows: these are backed by permissions.deny, never standalone settings keys.
// Catalogue: https://code.claude.com/docs/en/tools-reference
var toolSettings = func() []schema.Setting {
	names := strings.Fields(`Agent Artifact AskUserQuestion Bash CronCreate CronDelete CronList Edit EndConversation EnterPlanMode EnterWorktree ExitPlanMode ExitWorktree Glob Grep ListAgents ListMcpResourcesTool LSP Monitor NotebookEdit PowerShell PushNotification Read ReadMcpResourceTool RemoteTrigger ReportFindings ScheduleWakeup SendFeedback SendMessage SendUserFile ShareOnboardingGuide Skill SubagentHandback TaskCreate TaskGet TaskList TaskOutput TaskStop TaskUpdate TodoWrite ToolSearch WaitForMcpServers WebFetch WebSearch Workflow Write`)
	out := make([]schema.Setting, len(names))
	for i, name := range names {
		out[i] = schema.Setting{Key: name, Section: "Tools", Kind: schema.KindBool, ScopeClass: schema.ScopeAny}
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

func (m *model) toolSummary(st *schema.Setting) string {
	name := toolName(st)
	rules, err := toolRules(m.files[m.scope])
	if err != nil {
		return "Tools · invalid permissions.deny"
	}
	for _, rule := range rules {
		if rule == name {
			return fmt.Sprintf("Tools · disabled [%s]", m.scope)
		}
	}
	return fmt.Sprintf("Tools · enabled [%s]", m.scope)
}

func (m *model) toggleTool(st *schema.Setting) tea.Cmd {
	sc := m.targetScope(st)
	if !store.Allowed(st, sc) {
		m.setStatus("Tools can only be toggled in user, project, or local settings", true)
		return nil
	}
	if err := m.errs[sc]; err != nil {
		m.setStatus("Cannot edit unreadable settings: "+err.Error(), true)
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
	state := "enabled"
	if !disabled {
		next = append(next, name)
		state = "disabled"
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
		m.setStatus("save failed: "+err.Error(), true)
		return nil
	}
	m.files[sc] = &updated
	m.setStatus(fmt.Sprintf("%s %s → %s", name, state, f.Path), false)
	return m.afterWrite()
}

func (m *model) toolDoc(st *schema.Setting) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n%s\n\nTab or Enter: enable/disable in the target file.\nDisabling adds the bare tool name to permissions.deny.\nEnabling removes that exact entry; normal permission prompts still apply.\nScoped rules and denies in other files remain in effect.\nAvailability also depends on your Claude Code version and session.\n", toolName(st), m.toolSummary(st))
	for _, sc := range []store.Scope{store.ScopeUser, store.ScopeProject, store.ScopeLocal, store.ScopeManaged} {
		rules, err := toolRules(m.files[sc])
		if err != nil {
			fmt.Fprintf(&b, "\n%s: %v", sc, err)
			continue
		}
		for _, rule := range rules {
			if rule == toolName(st) {
				fmt.Fprintf(&b, "\nDisabled in %s", sc)
				break
			}
		}
	}
	fmt.Fprintf(&b, "\n\nTarget file: %s (%s)\n", m.scope, store.Path(m.scope, m.cwd))
	return b.String()
}
