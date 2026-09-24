package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

const envSection = "Environment variables"

func isEnv(st *schema.Setting) bool { return st != nil && st.Section == envSection }

// Explicit choices come from the environment reference, not from backticks in
// prose (which also contain examples, filenames and unrelated setting names).
var envChoices = map[string][]string{
	"CLAUDE_CODE_DEBUG_LOG_LEVEL": {"verbose", "debug", "info", "warn", "error"},
	"CLAUDE_CODE_USE_BEDROCK":     {"1", "0"},
	"CLAUDE_CODE_USE_VERTEX":      {"1", "0"},
	"CLAUDE_CODE_USE_FOUNDRY":     {"1", "0"},
	"CLAUDE_CODE_EFFORT_LEVEL":    {"low", "medium", "high", "xhigh", "max", "auto"},
	"MCP_PROTOCOL_NEGOTIATION":    {"auto", "legacy"},
	"MCP_SDK_GENERATION":          {"v1", "v2"},
	"FORCE_HYPERLINK":             {"1", "0"},
}

func envSetting(name, purpose string) *schema.Setting {
	st := &schema.Setting{Key: "env." + name, Section: envSection, Kind: schema.KindString,
		ScopeClass: schema.ScopeAny, Scope: "User, project, or local", Type: "string", Desc: purpose,
		Hint: "Type the value exactly as documented; separate multiple values as the variable requires."}
	st.Options = envChoices[name]
	if name == "ANTHROPIC_MODEL" || name == "CLAUDE_CODE_SUBAGENT_MODEL" {
		st.Options = schema.Load().ModelAliases
	}
	// Only recognize an explicit boolean instruction. Presence-based flags and
	// values such as file:<dir> deliberately remain free text.
	if len(st.Options) == 0 && !strings.Contains(purpose, "non-empty") && !strings.Contains(purpose, "file:<") {
		switch {
		case strings.HasPrefix(purpose, "Set to `1` to"), strings.HasPrefix(purpose, "Set to `0` to"):
			st.Options = []string{"1", "0"}
		case strings.HasPrefix(purpose, "Set to `true` to"), strings.HasPrefix(purpose, "Set to `false` to"):
			st.Options = []string{"true", "false"}
		}
	}
	if len(st.Options) > 0 {
		st.Kind = schema.KindEnumOrString
	}
	return st
}

// Include the documented catalogue and additional names already in settings.
// Never enumerate the host process's environment or copy it into a file.
func (m *model) envItems() []list.Item {
	entries := map[string]string{}
	for _, e := range m.sch.EnvVars {
		entries[e.Name] = e.Purpose
	}
	for _, f := range m.files {
		if f == nil {
			continue
		}
		if v, ok := f.Get("env"); ok {
			if vars, ok := v.(map[string]any); ok {
				for name := range vars {
					if _, known := entries[name]; !known {
						entries[name] = "Custom environment variable from your settings. Enter edits its string value."
					}
				}
			}
		}
	}
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]list.Item, 0, len(names))
	for _, name := range names {
		st := envSetting(name, entries[name])
		it := m.leafItem(st)
		it.name = name
		out = append(out, it)
	}
	return out
}

func (m *model) envDoc(st *schema.Setting, w int) string {
	s := m.st
	name := strings.TrimPrefix(st.Key, "env.")
	intro := s.ok.Render("✿ Variable garden") + "\n" + s.title.Width(w).Render(name) + "\n\n"
	help := "Enter edits · Enter saves · Esc cancels"
	if len(st.Options) > 0 {
		help = "Tab cycles & saves · Enter edits"
	}
	intro += s.subtle.Width(w).Render(help) + "\n\n"
	intro += docDescription(s, st, w)
	if len(st.Options) > 0 {
		intro += s.label.Render("Choices") + "\n" + s.accent.Width(w).Render(strings.Join(st.Options, "  /  ")) + "\n\n"
	}
	intro += m.ladder(st, w) + "\n"
	intro += s.subtle.Width(w).Render(fmt.Sprintf("Saved as a string in env.%s. These rows show settings-file values; your shell environment is separate. Restart Claude Code after removing a value or changing a startup-only option.", name))
	return intro
}

// Stage a private copy so an unsuccessful save cannot appear as a live value.
func (m *model) saveEnv(st *schema.Setting, v any, remove bool) tea.Cmd {
	sc := m.targetScope(st)
	if !store.Allowed(st, sc) {
		m.setStatus(notReadHere(st, sc), true)
		return nil
	}
	if err := m.errs[sc]; err != nil {
		m.setStatus("Can't edit a settings file that failed to load: "+err.Error(), true)
		return nil
	}
	staged := *m.files[sc]
	data, err := json.Marshal(staged.Data)
	// Unmarshal into a new map, not the original file's map.
	if err == nil {
		staged.Data = nil
		err = json.Unmarshal(data, &staged.Data)
	}
	if err == nil {
		if remove {
			staged.Unset(st.Key)
		} else {
			err = staged.Set(st.Key, v)
		}
	}
	if err == nil {
		err = staged.Save()
	}
	if err != nil {
		m.setStatus("Save failed: "+err.Error(), true)
		return nil
	}
	m.files[sc] = &staged
	action := "Saved "
	if remove {
		action = "Removed "
	}
	m.setStatus(action+strings.TrimPrefix(st.Key, "env.")+" in "+tildify(staged.Path), false)
	return m.afterWrite()
}
