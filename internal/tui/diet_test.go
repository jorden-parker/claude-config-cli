package tui

import (
	"bytes"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jorden-parker/claude-config-cli/internal/diet"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

// dietModel opens the checklist pane in a model whose home and project are
// temporary directories, so nothing touches the real settings.
func dietModel(t *testing.T) *model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	openDiet(t, m)
	return m
}

func openDiet(t *testing.T, m *model) {
	t.Helper()
	for i, sec := range m.sections {
		if sec == dietSection {
			m.setSection(i)
			return
		}
	}
	t.Fatal("no Context diet section")
}

// selectDiet moves the cursor to a checklist row by its id.
func selectDiet(t *testing.T, m *model, id string) {
	t.Helper()
	for i, li := range m.list.Items() {
		if it := li.(item); it.diet != nil && it.diet.Rec.ID == id {
			m.list.Select(i)
			return
		}
	}
	t.Fatalf("no checklist row %s", id)
}

func enter(m *model) { m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) }

// writeSettings saves a value straight into a file, then reloads and rebuilds
// the rows the way the r key does.
func writeSettings(t *testing.T, m *model, sc store.Scope, key string, v any) {
	t.Helper()
	f, err := store.Open(sc, m.cwd)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Set(key, v); err != nil {
		t.Fatal(err)
	}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	m.reload()
	m.afterWrite()
}

func TestDietSectionSitsNextToStatusLineAndRendersEveryRow(t *testing.T) {
	m := dietModel(t)
	idx := map[string]int{}
	for i, sec := range m.sections {
		idx[sec] = i
	}
	if idx[dietSection] != idx[statusSection]+1 {
		t.Fatalf("Context diet at %d, Status line at %d", idx[dietSection], idx[statusSection])
	}

	seen := map[string]bool{}
	for _, li := range m.list.Items() {
		it := li.(item)
		if it.header {
			continue
		}
		if it.diet == nil {
			t.Fatalf("row %q has no checklist data", it.name)
		}
		seen[it.diet.Rec.ID] = true
		var buf bytes.Buffer
		renderDietRow(&buf, m.st, 100, false, it)
		plain := ansi.Strip(buf.String())
		if !strings.Contains(plain, it.name) || !strings.Contains(plain, it.diet.Result.State.String()) {
			t.Fatalf("row %s renders without its title or state:\n%s", it.diet.Rec.ID, plain)
		}
	}
	for _, rec := range diet.Recs {
		if !seen[rec.ID] {
			t.Fatalf("row %s is missing from the checklist", rec.ID)
		}
	}
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "Context diet") || !strings.Contains(view, "Settings and files") {
		t.Fatalf("pane does not show its heading:\n%s", view)
	}
}

func TestDietDetailsShowWhatItDoesCaveatFilesAndDocs(t *testing.T) {
	m := dietModel(t)
	selectDiet(t, m, "prompt-suggestions-off")
	doc := ansi.Strip(m.dietDoc(70))
	for _, want := range []string{"What it does", "Caveat", "Where it's set", "promptSuggestionEnabled", "How to apply", "https://code.claude.com/docs/en/interactive-mode#turn-prompt-suggestions-off"} {
		if !strings.Contains(doc, want) {
			t.Fatalf("details missing %q:\n%s", want, doc)
		}
	}
}

func TestDietEnterWritesToTheTargetFile(t *testing.T) {
	m := dietModel(t)
	selectDiet(t, m, "git-instructions-off")
	enter(m)
	user, _ := store.Open(store.ScopeUser, m.cwd)
	if v, ok := user.Get("includeGitInstructions"); !ok || v != false {
		t.Fatalf("user file: includeGitInstructions = %v, %v; status %q", v, ok, m.status)
	}

	// Switch the target to the project file: the next write must land there.
	m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	if m.scope != store.ScopeProject {
		t.Fatalf("target = %s", m.scope)
	}
	selectDiet(t, m, "prompt-suggestions-off")
	enter(m)
	project, _ := store.Open(store.ScopeProject, m.cwd)
	if v, ok := project.Get("promptSuggestionEnabled"); !ok || v != false {
		t.Fatalf("project file: promptSuggestionEnabled = %v, %v; status %q", v, ok, m.status)
	}
	if _, ok := user.Get("promptSuggestionEnabled"); ok {
		t.Fatal("write went to the user file too")
	}
}

func TestDietEnterOnTerminalTitleWritesEnvVar(t *testing.T) {
	m := dietModel(t)
	selectDiet(t, m, "terminal-title-off")
	enter(m)
	user, _ := store.Open(store.ScopeUser, m.cwd)
	if v, _ := user.Get("env.CLAUDE_CODE_DISABLE_TERMINAL_TITLE"); v != "1" {
		t.Fatalf("env var = %v; status %q", v, m.status)
	}
}

func TestDietEnterOnAppliedRowOnlyReports(t *testing.T) {
	m := dietModel(t)
	writeSettings(t, m, store.ScopeUser, "includeGitInstructions", false)
	selectDiet(t, m, "git-instructions-off")
	p := store.Path(store.ScopeUser, m.cwd)
	before := diskState(p)
	enter(m)
	if !strings.Contains(m.status, "Already applied") || !strings.Contains(m.status, "user file") {
		t.Fatalf("status = %q", m.status)
	}
	if diskState(p) != before {
		t.Fatal("applied row changed the file")
	}
}

func TestDietWarningRemovesTrapKeyAfterConfirmation(t *testing.T) {
	m := dietModel(t)
	writeSettings(t, m, store.ScopeUser, "env", map[string]any{"ENABLE_TOOL_SEARCH": "false"})
	selectDiet(t, m, "mcp-tool-search-not-disabled")
	enter(m)
	if m.mode != modeConfirmUnset {
		t.Fatalf("mode = %v, status %q", m.mode, m.status)
	}
	m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	user, _ := store.Open(store.ScopeUser, m.cwd)
	if _, ok := user.Get("env.ENABLE_TOOL_SEARCH"); ok {
		t.Fatal("trap key still in the user file")
	}
	if it, _ := m.selectedItem(); it.diet.Result.State != diet.Applied {
		t.Fatalf("row after removal = %s", it.diet.Result.State)
	}
}

func TestDietWarningOutsideTargetSaysWhere(t *testing.T) {
	m := dietModel(t)
	writeSettings(t, m, store.ScopeUser, "env", map[string]any{"ENABLE_TOOL_SEARCH": "false"})
	m.Update(tea.KeyPressMsg{Code: 's', Text: "s"}) // project is now the target
	selectDiet(t, m, "mcp-tool-search-not-disabled")
	enter(m)
	if m.mode == modeConfirmUnset || !m.statusErr || !strings.Contains(m.status, "in the user file") {
		t.Fatalf("mode = %v, status %q", m.mode, m.status)
	}
}

func TestDietChoiceRowOpensTheEditor(t *testing.T) {
	m := dietModel(t)
	selectDiet(t, m, "effort-low-or-medium")
	enter(m)
	if m.mode != modeEdit || m.edit == nil || m.edit.st.Key != "modelSettings" {
		t.Fatalf("mode = %v, status %q", m.mode, m.status)
	}
	if m.section() != "Model and responses" {
		t.Fatalf("editor opened in %q", m.section())
	}
}

func TestDietInfoRowShowsDetails(t *testing.T) {
	m := dietModel(t)
	selectDiet(t, m, "habit-clear")
	enter(m)
	if !m.details || m.status == "" {
		t.Fatalf("details = %v, status %q", m.details, m.status)
	}
}

func TestDietOpensOwnDocsURL(t *testing.T) {
	m := dietModel(t)
	selectDiet(t, m, "keep-prompt-cache-on")
	if got := m.dietURL(); got != "https://code.claude.com/docs/en/prompt-caching#disable-prompt-caching" {
		t.Fatalf("url = %s", got)
	}
	// The browser is not opened here: openURL would launch one.
}
