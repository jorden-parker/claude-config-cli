package catalog

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
)

// Hand-curated facts the reference describes in prose rather than as lists.
var (
	modelAliases = []string{"default", "best", "fable", "sonnet", "opus", "haiku", "sonnet[1m]", "opus[1m]", "fable[1m]", "opusplan"}
	hookEvents   = []string{"SessionStart", "Setup", "InstructionsLoaded", "UserPromptSubmit", "UserPromptExpansion", "MessageDisplay", "PreToolUse", "PermissionRequest", "PostToolUse", "PostToolUseFailure", "PostToolBatch", "PermissionDenied", "Notification", "SubagentStart", "SubagentStop", "TaskCreated", "TaskCompleted", "Stop", "StopFailure", "TeammateIdle", "ConfigChange", "CwdChanged", "DirectoryAdded", "FileChanged", "WorktreeCreate", "WorktreeRemove", "PreCompact", "PostCompact", "PreModelSwitch", "PostModelSwitch", "SessionEnd", "Elicitation", "ElicitationResult"}
)

type override struct {
	Kind        schema.Kind
	Options     []string
	Hint        string
	Suggestions []string
}

var overrides = map[string]override{
	"model":                            {Kind: schema.KindEnumOrString, Options: modelAliases, Hint: "alias or full model ID such as claude-opus-5-5"},
	"advisorModel":                     {Kind: schema.KindEnumOrString, Options: []string{"fable", "opus", "sonnet"}, Hint: "alias or full model ID"},
	"teammateDefaultModel":             {Kind: schema.KindEnumOrString, Options: modelAliases, Hint: "alias or full model ID"},
	"availableModels":                  {Kind: schema.KindArray, Suggestions: modelAliases},
	"fallbackModel":                    {Kind: schema.KindArray, Suggestions: modelAliases},
	"outputStyle":                      {Kind: schema.KindEnumOrString, Options: []string{"Default", "Proactive", "Concise", "Explanatory", "Learning"}, Hint: "built-in name (case-sensitive) or a custom style name"},
	"theme":                            {Kind: schema.KindEnumOrString, Options: []string{"auto", "dark", "light", "dark-daltonized", "light-daltonized", "dark-ansi", "light-ansi"}, Hint: "or custom:<slug>"},
	"timeFormat":                       {Kind: schema.KindEnumOrString, Options: []string{"auto", "12-hour", "24-hour", "24-hour-utc"}, Hint: "or a strftime pattern such as %H:%M"},
	"strictPluginOnlyCustomization":    {Kind: schema.KindMultiSelect, Options: []string{"skills", "agents", "hooks", "mcp"}, Hint: "true locks all four; array locks the named kinds"},
	"hooks":                            {Kind: schema.KindJSON, Suggestions: hookEvents},
	"env":                              {Kind: schema.KindMap},
	"enabledPlugins":                   {Kind: schema.KindMapBool},
	"modelOverrides":                   {Kind: schema.KindMap},
	"vimInsertModeRemaps":              {Kind: schema.KindMap},
	"forceLoginOrgUUID":                {Kind: schema.KindArray},
	"permissions.defaultMode":          {Kind: schema.KindEnum},
	"taskOutputMaxChars":               {Kind: schema.KindNumber},
	"disableDesktopLocalSessions":      {Kind: schema.KindBool},
	"disableBrowserExternalNavigation": {Kind: schema.KindBool},
	"disableMobileSimulatorTools":      {Kind: schema.KindBool},
	"browserExternalPageTools":         {Kind: schema.KindEnum, Options: []string{"disabled"}},
}

// skip lists documented sub-entries that the multiselect parent already covers.
var skip = map[string]bool{
	"strictPluginOnlyCustomization.skills": true,
	"strictPluginOnlyCustomization.agents": true,
	"strictPluginOnlyCustomization.hooks":  true,
	"strictPluginOnlyCustomization.mcp":    true,
}

// removedWithoutMetadata lists removed keys whose entry carries no Scope or
// Type bullets. Only a removed key may appear here.
var removedWithoutMetadata = map[string]bool{"taskOutputMaxChars": true}

// scopePrefixes maps the first words of a Scope bullet to a scope class.
// "User" comes last because it is a prefix of two other entries.
var scopePrefixes = []struct {
	prefix string
	class  schema.ScopeClass
}{
	{"Any file", schema.ScopeAny},
	{"Managed", schema.ScopeManaged},
	{"User, local, or managed", schema.ScopeUserLocalManaged},
	{"User or managed", schema.ScopeUserManaged},
	{"Global config", schema.ScopeGlobal},
	{"User", schema.ScopeUser},
}

var (
	theStringRE = regexp.MustCompile("^the string `\"([^\"]+)\"`")
	quotedRE    = regexp.MustCompile("`\"([^\"]+)\"`")
)

// classify picks the editor kind and fixed options for an entry. Containers
// (object, array) are decided before quoted member options, because the
// options listed under an object type describe its members, not the value.
func classify(e entry) (schema.Kind, []string) {
	t := e.Type
	switch {
	case strings.HasPrefix(t, "Boolean"):
		return schema.KindBool, nil
	}
	if m := theStringRE.FindStringSubmatch(t); m != nil {
		return schema.KindEnum, []string{m[1]}
	}
	switch {
	case strings.HasPrefix(t, "array of objects"), strings.HasPrefix(t, "array of marketplace source"):
		return schema.KindJSON, nil
	case strings.HasPrefix(t, "array"):
		return schema.KindArray, nil
	case strings.HasPrefix(t, "object"):
		return schema.KindJSON, nil
	}
	if len(e.Options) > 0 {
		all := true
		opts := make([]string, 0, len(e.Options))
		for _, o := range e.Options {
			if !strings.HasPrefix(o.Value, `"`) {
				all = false
				break
			}
			opts = append(opts, strings.Trim(o.Value, `"`))
		}
		if all {
			return schema.KindEnum, opts
		}
	}
	if strings.HasPrefix(t, "string, one of") || strings.HasPrefix(t, "string, `\"") {
		var vals []string
		seen := map[string]bool{}
		for _, m := range quotedRE.FindAllStringSubmatch(t, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				vals = append(vals, m[1])
			}
		}
		if len(vals) > 0 {
			return schema.KindEnum, vals
		}
	}
	if strings.HasPrefix(t, "number") || strings.HasPrefix(t, "integer") {
		return schema.KindNumber, nil
	}
	return schema.KindString, nil
}

// scopeClass splits a cleaned Scope bullet into its class, the leading
// phrase shown as the scope, and the rest as a note.
func scopeClass(scope string) (class schema.ScopeClass, head, note string, err error) {
	head, note = splitSentence(scope)
	for _, p := range scopePrefixes {
		if regexp.MustCompile(`^` + regexp.QuoteMeta(p.prefix) + `\b`).MatchString(head) {
			return p.class, head, note, nil
		}
	}
	return "", "", "", fmt.Errorf("unknown scope prefix: %q", truncate(scope, 80))
}

// splitSentence cuts at the first full stop that ends a sentence, so a dot
// inside a path such as ~/.claude does not split the phrase.
func splitSentence(s string) (head, rest string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' && (i+1 == len(s) || s[i+1] == ' ') {
			return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
		}
	}
	return strings.TrimSpace(s), ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// validate rejects an empty reference, duplicate keys, and entries without
// Type or Scope unless the docs mark them removed and they are listed above.
func validate(entries []entry) error {
	if len(entries) == 0 {
		return fmt.Errorf("no settings entries were extracted from the reference")
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Key] {
			return fmt.Errorf("duplicate setting key: %s", e.Key)
		}
		seen[e.Key] = true
		if e.Type != "" && e.Scope != "" {
			continue
		}
		isRemoved := removed(e.deprecated())
		if isRemoved && removedWithoutMetadata[e.Key] {
			continue
		}
		var missing []string
		if e.Type == "" {
			missing = append(missing, "type")
		}
		if e.Scope == "" {
			missing = append(missing, "scope")
		}
		why := " (not documented as removed)"
		if isRemoved {
			why = ""
		}
		return fmt.Errorf("setting %s is missing %s%s", e.Key, strings.Join(missing, " and "), why)
	}
	return nil
}

// buildSettings turns entries into catalogue settings.
func buildSettings(entries []entry) ([]schema.Setting, error) {
	if err := validate(entries); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(entries))
	for _, e := range entries {
		keys = append(keys, e.Key)
	}
	var out []schema.Setting
	for _, e := range entries {
		if skip[e.Key] {
			continue
		}
		kind, opts := classify(e)
		if opts == nil {
			opts = []string{}
		}
		hasChildren := false
		for _, k := range keys {
			if strings.HasPrefix(k, e.Key+".") {
				hasChildren = true
				break
			}
		}
		help := map[string]string{}
		for _, o := range e.Options {
			if o.Help != "" {
				help[strings.Trim(o.Value, `"`)] = clean(o.Help)
			}
		}
		st := schema.Setting{
			Key: e.Key, Section: e.Section, Type: clean(e.Type), Default: clean(e.Default),
			Desc: clean(e.Desc), Kind: kind, Options: opts, OptionHelp: help,
			Example: e.Example, Deprecated: e.deprecated(), Suggestions: []string{},
		}
		if hasChildren && kind == schema.KindJSON {
			st.Kind = schema.KindGroup
		}
		if o, ok := overrides[e.Key]; ok {
			st.Kind = o.Kind
			if o.Options != nil {
				st.Options = o.Options
			}
			if o.Suggestions != nil {
				st.Suggestions = o.Suggestions
			}
			st.Hint = o.Hint
		}
		st.Removed = removed(st.Deprecated)
		st.Global = e.Section == "Global config settings"
		if sc := clean(e.Scope); sc != "" {
			class, head, note, err := scopeClass(sc)
			if err != nil {
				return nil, fmt.Errorf("setting %s: %w", e.Key, err)
			}
			st.ScopeClass, st.Scope, st.ScopeNote = class, head, note
		} else {
			// Only reachable for the removed exception validated above.
			st.ScopeClass = schema.ScopeAny
		}
		out = append(out, st)
	}
	return out, nil
}

// sortedKeys is a small helper for deterministic messages.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
