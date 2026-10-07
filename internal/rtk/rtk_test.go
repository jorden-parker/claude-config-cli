package rtk

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jorden-parker/claude-config-cli/internal/store"
)

func file(t *testing.T, src string) *store.File {
	t.Helper()
	f := &store.File{Scope: store.ScopeUser, Data: map[string]any{}}
	if err := json.Unmarshal([]byte(src), &f.Data); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestEnableAddsBothHooksBesideOthers(t *testing.T) {
	f := file(t, `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"~/.claude/hooks/other.py"}]}]}}`)
	if err := Enable(f, "/usr/local/bin/ccfg rtk read-hook"); err != nil {
		t.Fatal(err)
	}
	st := On(f)
	if !st.Bash || !st.Read {
		t.Fatalf("want both hooks on, got %+v", st)
	}
	list := preToolUse(f)
	if len(list) != 3 {
		t.Fatalf("want 3 entries, got %d", len(list))
	}
	if !hasCommand(list[0], func(c string) bool { return c == "~/.claude/hooks/other.py" }) {
		t.Fatal("the existing hook moved or changed")
	}
	if err := Enable(f, "/usr/local/bin/ccfg rtk read-hook"); err != nil {
		t.Fatal(err)
	}
	if len(preToolUse(f)) != 3 {
		t.Fatal("enabling twice duplicated entries")
	}
}

func TestEnableWithoutReadHook(t *testing.T) {
	f := file(t, `{}`)
	if err := Enable(f, ""); err != nil {
		t.Fatal(err)
	}
	if st := On(f); !st.Bash || st.Read {
		t.Fatalf("want only the Bash hook, got %+v", st)
	}
}

func TestDisableRemovesOnlyOurs(t *testing.T) {
	f := file(t, `{"model":"opus","hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"~/.claude/hooks/other.py"}]}],"SessionStart":[{"hooks":[{"type":"command","command":"x"}]}]}}`)
	if err := Enable(f, "ccfg rtk read-hook"); err != nil {
		t.Fatal(err)
	}
	Disable(f)
	if st := On(f); st.Bash || st.Read {
		t.Fatalf("want both off, got %+v", st)
	}
	if len(preToolUse(f)) != 1 {
		t.Fatalf("want the other hook kept, got %v", preToolUse(f))
	}
	if _, ok := f.Get("hooks.SessionStart"); !ok {
		t.Fatal("SessionStart hooks were lost")
	}
}

func TestDisableDropsEmptyHooksObject(t *testing.T) {
	f := file(t, `{}`)
	if err := Enable(f, "ccfg rtk read-hook"); err != nil {
		t.Fatal(err)
	}
	Disable(f)
	if _, ok := f.Get("hooks"); ok {
		t.Fatalf("want hooks removed, got %v", f.Data)
	}
}

func TestReadHookDenies(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader(`{"tool_name":"Read","tool_input":{"file_path":"/tmp/x.go"}}`)
	if err := ReadHook(in, &out); err != nil {
		t.Fatal(err)
	}
	var got struct {
		H struct {
			Event    string `json:"hookEventName"`
			Decision string `json:"permissionDecision"`
			Reason   string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.H.Event != "PreToolUse" || got.H.Decision != "deny" {
		t.Fatalf("got %+v", got.H)
	}
	for _, want := range []string{"rtk read /tmp/x.go", "sed -n '1,200p' /tmp/x.go"} {
		if !strings.Contains(got.H.Reason, want) {
			t.Fatalf("reason %q lacks %q", got.H.Reason, want)
		}
	}
}

func TestReadCommandByScope(t *testing.T) {
	if got := ReadCommand(store.ScopeProject, "/opt/ccfg"); got != "ccfg rtk read-hook" {
		t.Fatalf("project: %q", got)
	}
	if got := ReadCommand(store.ScopeUser, "~/.local/bin/ccfg"); got != "~/.local/bin/ccfg rtk read-hook" {
		t.Fatalf("user: %q", got)
	}
}
