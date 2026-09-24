package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

func TestToolTogglePersistsAndPreservesRules(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()
	m := newModel(cwd)
	f := m.files[store.ScopeUser]
	f.Set("permissions.deny", []any{"Bash(rm *)", "NotebookEdit(secret.ipynb)"})
	f.Set("permissions.allow", []any{"Read"})
	f.Set("model", "sonnet")
	selectKey(t, m, "NotebookEdit")
	for _, disabled := range []bool{true, false} {
		pressTab(m)
		if m.statusErr {
			t.Fatal(m.status)
		}
		saved, err := store.Open(store.ScopeUser, cwd)
		if err != nil {
			t.Fatal(err)
		}
		rules, err := toolRules(saved)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"Bash(rm *)", "NotebookEdit(secret.ipynb)"}
		if disabled {
			want = append(want, "NotebookEdit")
		}
		if !reflect.DeepEqual(rules, want) {
			t.Fatalf("rules = %v, want %v", rules, want)
		}
		if v, _ := saved.Get("model"); v != "sonnet" {
			t.Fatal("model changed")
		}
		if v, _ := saved.Get("permissions.allow"); !reflect.DeepEqual(v, []any{"Read"}) {
			t.Fatal("allow rules changed")
		}
		if _, ok := saved.Get("tools"); ok {
			t.Fatal("virtual key persisted")
		}
	}
}

func TestToolToggleUsesTargetAndShowsInheritedDeny(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())
	m.files[store.ScopeManaged].Set("permissions.deny", []string{"NotebookEdit"})
	selectKey(t, m, "NotebookEdit")
	m.Update(tea.KeyPressMsg{Code: 's'})
	pressTab(m)
	if _, ok := m.files[store.ScopeUser].Get("permissions.deny"); ok {
		t.Fatal("wrote wrong scope")
	}
	if got, _ := toolRules(m.files[store.ScopeProject]); !reflect.DeepEqual(got, []string{"NotebookEdit"}) {
		t.Fatalf("project denies = %v", got)
	}
	pressTab(m)
	if !strings.Contains(m.toolDoc(m.selected()), "Disabled in managed") {
		t.Fatal("managed deny hidden")
	}
	if got, _ := toolRules(m.files[store.ScopeManaged]); !reflect.DeepEqual(got, []string{"NotebookEdit"}) {
		t.Fatal("changed managed denies")
	}
}

func TestToolToggleRejectsInvalidRulesAndSaveFailures(t *testing.T) {
	for _, failure := range []string{"invalid array", "invalid parent", "unreadable", "save"} {
		t.Run(failure, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			m := newModel(t.TempDir())
			f := m.files[store.ScopeUser]
			switch failure {
			case "invalid array":
				f.Set("permissions.deny", []any{42})
			case "invalid parent":
				f.Set("permissions", "invalid")
			case "unreadable":
				m.errs[store.ScopeUser] = &storeError{}
			case "save":
				f.Path = t.TempDir()
			}
			before := store.Format(f.Data)
			selectKey(t, m, "NotebookEdit")
			pressTab(m)
			if !m.statusErr {
				t.Fatal("expected error")
			}
			if got := store.Format(m.files[store.ScopeUser].Data); got != before {
				t.Fatalf("failed toggle changed data: %s", got)
			}
		})
	}
}

type storeError struct{}

func (*storeError) Error() string { return "unreadable file" }
