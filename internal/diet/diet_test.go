package diet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jorden-parker/claude-config-cli/internal/store"
)

var allScopes = []store.Scope{store.ScopeManaged, store.ScopeLocal, store.ScopeProject, store.ScopeUser, store.ScopeGlobal}

// input builds the files the checks read. settings holds each file's JSON,
// with __CWD__ standing for the project directory; missing scopes are empty. files holds Markdown or JSON written under the
// temporary home ("home:" prefix) or project ("cwd:" prefix).
func input(t *testing.T, settings map[store.Scope]string, files map[string]string) Input {
	t.Helper()
	home, cwd := t.TempDir(), t.TempDir()
	in := Input{Files: map[store.Scope]*store.File{}, Cwd: cwd, Home: home}
	for _, sc := range allScopes {
		data := map[string]any{}
		if s, ok := settings[sc]; ok {
			// __CWD__ stands for this test's project directory.
			s = strings.ReplaceAll(s, "__CWD__", cwd)
			if err := json.Unmarshal([]byte(s), &data); err != nil {
				t.Fatalf("%s fixture: %v", sc, err)
			}
		}
		in.Files[sc] = &store.File{Scope: sc, Path: string(sc), Data: data}
	}
	for key, content := range files {
		root, rel, _ := strings.Cut(key, ":")
		base := cwd
		if root == "home" {
			base = home
		}
		p := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return in
}

func resultFor(t *testing.T, in Input, id string) Result {
	t.Helper()
	for _, row := range Evaluate(in) {
		if row.Rec.ID == id {
			return row.Result
		}
	}
	t.Fatalf("no row %s", id)
	return Result{}
}

func TestDetection(t *testing.T) {
	type want struct {
		state     State
		scope     store.Scope
		isDefault bool
		offenders []string
	}
	cases := []struct {
		name     string
		id       string
		settings map[store.Scope]string
		files    map[string]string
		want     want
	}{
		// effort-low-or-medium
		{name: "effort saved per model in user", id: "effort-low-or-medium",
			settings: map[store.Scope]string{store.ScopeUser: `{"modelSettings":{"claude-opus-5-5":{"effortLevel":"medium"}}}`},
			want:     want{state: Applied, scope: store.ScopeUser}},
		{name: "effort from env var in local", id: "effort-low-or-medium",
			settings: map[store.Scope]string{store.ScopeLocal: `{"env":{"CLAUDE_CODE_EFFORT_LEVEL":"low"}}`},
			want:     want{state: Applied, scope: store.ScopeLocal}},
		{name: "env effort high overrides saved low", id: "effort-low-or-medium",
			settings: map[store.Scope]string{store.ScopeUser: `{"env":{"CLAUDE_CODE_EFFORT_LEVEL":"high"},"modelSettings":{"x":{"effortLevel":"low"}}}`},
			want:     want{state: NotApplied}},
		{name: "effort not saved", id: "effort-low-or-medium",
			want: want{state: NotApplied}},
		{name: "effort top-level in user is a trap", id: "effort-low-or-medium",
			settings: map[store.Scope]string{store.ScopeUser: `{"effortLevel":"low"}`},
			want:     want{state: Warning, offenders: []string{"effortLevel"}}},
		{name: "thinking setting is a trap", id: "effort-low-or-medium",
			settings: map[store.Scope]string{store.ScopeProject: `{"alwaysThinkingEnabled":false,"modelSettings":{"x":{"effortLevel":"low"}}}`},
			want:     want{state: Warning, offenders: []string{"alwaysThinkingEnabled"}}},

		// model-sonnet-default
		{name: "model haiku in project", id: "model-sonnet-default",
			settings: map[store.Scope]string{store.ScopeProject: `{"model":"haiku"}`},
			want:     want{state: Applied, scope: store.ScopeProject}},
		{name: "model full sonnet id in local", id: "model-sonnet-default",
			settings: map[store.Scope]string{store.ScopeLocal: `{"model":"claude-sonnet-5-5"}`},
			want:     want{state: Applied, scope: store.ScopeLocal}},
		{name: "model opus is not applied", id: "model-sonnet-default",
			settings: map[store.Scope]string{store.ScopeUser: `{"model":"opus"}`},
			want:     want{state: NotApplied}},

		// mcp-disable-unused
		{name: "server disabled in local file", id: "mcp-disable-unused",
			settings: map[store.Scope]string{store.ScopeLocal: `{"disabledMcpjsonServers":["fs"]}`},
			want:     want{state: Applied, scope: store.ScopeLocal}},
		{name: "server turned off with /mcp is in global file", id: "mcp-disable-unused",
			files:    map[string]string{"cwd:.mcp.json": `{"mcpServers":{"fs":{}}}`},
			settings: map[store.Scope]string{store.ScopeGlobal: `{"projects":{"__CWD__":{"disabledMcpServers":["fs"]}}}`},
			want:     want{state: Applied, scope: store.ScopeGlobal}},
		{name: "servers present, none off", id: "mcp-disable-unused",
			files: map[string]string{"cwd:.mcp.json": `{"mcpServers":{"fs":{},"gh":{}}}`},
			want:  want{state: NotApplied}},
		{name: "no servers at all is info", id: "mcp-disable-unused",
			want: want{state: Info}},

		// mcp-tool-search-not-disabled
		{name: "tool search unset is on by default", id: "mcp-tool-search-not-disabled",
			want: want{state: Applied, isDefault: true}},
		{name: "tool search auto in project", id: "mcp-tool-search-not-disabled",
			settings: map[store.Scope]string{store.ScopeProject: `{"env":{"ENABLE_TOOL_SEARCH":"auto"}}`},
			want:     want{state: Applied, scope: store.ScopeProject}},
		{name: "tool search false is a trap", id: "mcp-tool-search-not-disabled",
			settings: map[store.Scope]string{store.ScopeUser: `{"env":{"ENABLE_TOOL_SEARCH":"false"}}`},
			want:     want{state: Warning, offenders: []string{"env.ENABLE_TOOL_SEARCH"}}},
		{name: "auto:5 is old advice", id: "mcp-tool-search-not-disabled",
			settings: map[store.Scope]string{store.ScopeUser: `{"env":{"ENABLE_TOOL_SEARCH":"auto:5"}}`},
			want:     want{state: Warning, offenders: []string{"env.ENABLE_TOOL_SEARCH"}}},
		{name: "experimental betas off turns search off", id: "mcp-tool-search-not-disabled",
			settings: map[store.Scope]string{store.ScopeUser: `{"env":{"CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS":"1"}}`},
			want:     want{state: Warning, offenders: []string{"env.CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"}}},

		// mcp-no-always-load
		{name: "alwaysLoad true in .mcp.json", id: "mcp-no-always-load",
			files: map[string]string{"cwd:.mcp.json": `{"mcpServers":{"db":{"alwaysLoad":true}}}`},
			want:  want{state: NotApplied}},
		{name: "alwaysLoad true in ~/.claude.json", id: "mcp-no-always-load",
			settings: map[store.Scope]string{store.ScopeGlobal: `{"mcpServers":{"db":{"alwaysLoad":true}}}`},
			want:     want{state: NotApplied}},
		{name: "alwaysLoad false is fine", id: "mcp-no-always-load",
			files: map[string]string{"cwd:.mcp.json": `{"mcpServers":{"db":{"alwaysLoad":false}}}`},
			want:  want{state: Applied}},

		// skill-listing-trim
		{name: "skill trimmed in local", id: "skill-listing-trim",
			settings: map[store.Scope]string{store.ScopeLocal: `{"skillOverrides":{"deploy":"off"}}`},
			want:     want{state: Applied, scope: store.ScopeLocal}},
		{name: "skill override on is not a trim", id: "skill-listing-trim",
			settings: map[store.Scope]string{store.ScopeUser: `{"skillOverrides":{"deploy":"on"}}`},
			want:     want{state: NotApplied}},

		// prompt-suggestions-off
		{name: "suggestions off in user", id: "prompt-suggestions-off",
			settings: map[store.Scope]string{store.ScopeUser: `{"promptSuggestionEnabled":false}`},
			want:     want{state: Applied, scope: store.ScopeUser}},
		{name: "env var true beats setting false", id: "prompt-suggestions-off",
			settings: map[store.Scope]string{store.ScopeUser: `{"promptSuggestionEnabled":false,"env":{"CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION":"true"}}`},
			want:     want{state: NotApplied}},
		{name: "nonessential traffic off is a trap", id: "prompt-suggestions-off",
			settings: map[store.Scope]string{store.ScopeUser: `{"env":{"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC":"1"}}`},
			want:     want{state: Warning}},
		{name: "project true overrides user false", id: "prompt-suggestions-off",
			settings: map[store.Scope]string{store.ScopeUser: `{"promptSuggestionEnabled":false}`, store.ScopeProject: `{"promptSuggestionEnabled":true}`},
			want:     want{state: NotApplied}},

		// terminal-title-off
		{name: "terminal title off in project env", id: "terminal-title-off",
			settings: map[store.Scope]string{store.ScopeProject: `{"env":{"CLAUDE_CODE_DISABLE_TERMINAL_TITLE":"1"}}`},
			want:     want{state: Applied, scope: store.ScopeProject}},
		{name: "terminal title on by default", id: "terminal-title-off",
			want: want{state: NotApplied}},

		// git-instructions-off
		{name: "git instructions off in local", id: "git-instructions-off",
			settings: map[store.Scope]string{store.ScopeLocal: `{"includeGitInstructions":false}`},
			want:     want{state: Applied, scope: store.ScopeLocal}},
		{name: "git instructions on", id: "git-instructions-off",
			settings: map[store.Scope]string{store.ScopeUser: `{"includeGitInstructions":true}`},
			want:     want{state: NotApplied}},

		// cache-ttl-1h-api
		{name: "ttl 1h in user", id: "cache-ttl-1h-api",
			settings: map[store.Scope]string{store.ScopeUser: `{"promptCacheTtl":"1h"}`},
			want:     want{state: Applied, scope: store.ScopeUser}},
		{name: "env ttl 5m wins over setting 1h", id: "cache-ttl-1h-api",
			settings: map[store.Scope]string{store.ScopeUser: `{"promptCacheTtl":"1h","env":{"CLAUDE_CODE_PROMPT_CACHE_TTL":"5m"}}`},
			want:     want{state: NotApplied}},
		{name: "force 5m is a trap", id: "cache-ttl-1h-api",
			settings: map[store.Scope]string{store.ScopeUser: `{"promptCacheTtl":"1h","env":{"FORCE_PROMPT_CACHING_5M":"1"}}`},
			want:     want{state: Warning, offenders: []string{"env.FORCE_PROMPT_CACHING_5M"}}},

		// keep-prompt-cache-on
		{name: "caching on by default", id: "keep-prompt-cache-on",
			want: want{state: Applied, isDefault: true}},
		{name: "disable caching is a trap", id: "keep-prompt-cache-on",
			settings: map[store.Scope]string{store.ScopeUser: `{"env":{"DISABLE_PROMPT_CACHING":"1"}}`},
			want:     want{state: Warning, offenders: []string{"env.DISABLE_PROMPT_CACHING"}}},
		{name: "per-model disable is a trap", id: "keep-prompt-cache-on",
			settings: map[store.Scope]string{store.ScopeLocal: `{"env":{"DISABLE_PROMPT_CACHING_SONNET":"1"}}`},
			want:     want{state: Warning, offenders: []string{"env.DISABLE_PROMPT_CACHING_SONNET"}}},

		// claude-md-under-200-lines
		{name: "no instruction files is applied", id: "claude-md-under-200-lines",
			want: want{state: Applied}},
		{name: "199 lines is fine", id: "claude-md-under-200-lines",
			files: map[string]string{"cwd:CLAUDE.md": strings.Repeat("line\n", 199)},
			want:  want{state: Applied}},
		{name: "200 lines is too long", id: "claude-md-under-200-lines",
			files: map[string]string{"cwd:CLAUDE.md": strings.Repeat("line\n", 200)},
			want:  want{state: NotApplied}},
		{name: "long user CLAUDE.md counts", id: "claude-md-under-200-lines",
			files: map[string]string{"home:.claude/CLAUDE.md": strings.Repeat("x\n", 250)},
			want:  want{state: NotApplied}},
		{name: "claude mds disabled is a trap", id: "claude-md-under-200-lines",
			settings: map[store.Scope]string{store.ScopeUser: `{"env":{"CLAUDE_CODE_DISABLE_CLAUDE_MDS":"1"}}`},
			want:     want{state: Warning, offenders: []string{"env.CLAUDE_CODE_DISABLE_CLAUDE_MDS"}}},
		{name: "@import is a trap", id: "claude-md-under-200-lines",
			files: map[string]string{"cwd:CLAUDE.md": "# Rules\n@rules/style.md\n"},
			want:  want{state: Warning}},

		// subagents-smaller-model
		{name: "subagent on haiku", id: "subagents-smaller-model",
			files: map[string]string{"cwd:.claude/agents/scan.md": "---\nname: scan\nmodel: haiku\n---\nBody\n"},
			want:  want{state: Applied, scope: store.ScopeProject}},
		{name: "subagent on opus", id: "subagents-smaller-model",
			files: map[string]string{"home:.claude/agents/review.md": "---\nmodel: opus\n---\n"},
			want:  want{state: NotApplied}},
		{name: "no subagents is info", id: "subagents-smaller-model",
			want: want{state: Info}},

		// habits and frontmatter reminders
		{name: "habits are info", id: "habit-clear", want: want{state: Info}},
		{name: "skill frontmatter counted, info", id: "skill-frontmatter-info",
			files: map[string]string{"cwd:.claude/skills/a/SKILL.md": "---\ndisable-model-invocation: true\n---\n"},
			want:  want{state: Info}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := tc.settings
			if settings == nil {
				settings = map[store.Scope]string{}
			}
			in := input(t, settings, tc.files)
			got := resultFor(t, in, tc.id)
			if got.State != tc.want.state {
				t.Fatalf("state = %s (%s), want %s", got.State, got.Note, tc.want.state)
			}
			if got.Scope != tc.want.scope {
				t.Fatalf("scope = %q, want %q (%s)", got.Scope, tc.want.scope, got.Note)
			}
			if got.Default != tc.want.isDefault {
				t.Fatalf("default = %v, want %v", got.Default, tc.want.isDefault)
			}
			var keys []string
			for _, o := range got.Offenders {
				keys = append(keys, o.Key)
			}
			if !reflect.DeepEqual(keys, tc.want.offenders) {
				t.Fatalf("offenders = %v, want %v", keys, tc.want.offenders)
			}
		})
	}
}

// A value counts from whichever file it is set in, so each file is checked.
// ~/.claude.json is not a settings file, so a settings key there does not count.
func TestAppliedInEachFile(t *testing.T) {
	for _, sc := range []store.Scope{store.ScopeManaged, store.ScopeLocal, store.ScopeProject, store.ScopeUser} {
		t.Run(string(sc), func(t *testing.T) {
			in := input(t, map[store.Scope]string{sc: `{"includeGitInstructions":false}`}, nil)
			got := resultFor(t, in, "git-instructions-off")
			if got.State != Applied || got.Scope != sc {
				t.Fatalf("state = %s, scope = %q; want applied in %s (%s)", got.State, got.Scope, sc, got.Note)
			}
		})
	}
}

// Managed settings win over every other file, as the rest of the app reads them.
func TestManagedWinsOverUser(t *testing.T) {
	in := input(t, map[store.Scope]string{
		store.ScopeUser:    `{"includeGitInstructions":true}`,
		store.ScopeManaged: `{"includeGitInstructions":false}`,
	}, nil)
	if got := resultFor(t, in, "git-instructions-off"); got.State != Applied || got.Scope != store.ScopeManaged {
		t.Fatalf("state = %s, scope = %q", got.State, got.Scope)
	}
}

func TestCountLines(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{"", 0},
		{"a", 1},
		{"a\n", 1},
		{"a\nb", 2},
		{"a\n\n", 2},
		{strings.Repeat("x\n", 200), 200},
	} {
		if got := countLines(tc.in); got != tc.want {
			t.Errorf("countLines(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestRecsAreComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Recs {
		if r.ID == "" || r.Title == "" || r.Why == "" || r.URL == "" {
			t.Errorf("%q is missing a field", r.ID)
		}
		if seen[r.ID] {
			t.Errorf("duplicate id %s", r.ID)
		}
		seen[r.ID] = true
		if r.Group != GroupSettings && r.Group != GroupHabits {
			t.Errorf("%s has group %q", r.ID, r.Group)
		}
		if r.Apply.Kind == ApplySet && (r.Apply.Key == "" || r.Apply.Value == "") {
			t.Errorf("%s sets a value without a key", r.ID)
		}
		if r.Apply.Kind == ApplyEditor && r.Apply.Key == "" {
			t.Errorf("%s opens an editor without a key", r.ID)
		}
	}
}

func TestSummary(t *testing.T) {
	rows := []Row{
		{Rec: Rec{Group: GroupSettings}, Result: Result{State: Applied}},
		{Rec: Rec{Group: GroupSettings}, Result: Result{State: NotApplied}},
		{Rec: Rec{Group: GroupSettings}, Result: Result{State: Warning}},
		{Rec: Rec{Group: GroupSettings}, Result: Result{State: Info}},
		{Rec: Rec{Group: GroupHabits}, Result: Result{State: Applied}},
	}
	s := Summarize(rows)
	if s != (Summary{Checked: 3, Applied: 1, Warnings: 1}) {
		t.Fatalf("summary = %+v", s)
	}
	if got := s.Line(); got != "1 of 3 applied · 1 warning" {
		t.Fatalf("line = %q", got)
	}
}
