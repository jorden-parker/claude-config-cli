package tui

import (
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/cursor"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

func envModel(t *testing.T) *model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	m := newModel(t.TempDir())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	return m
}

func TestEnvironmentCatalogueAndSearch(t *testing.T) {
	m := envModel(t)
	m.files[store.ScopeUser].Set("env.MY.CUSTOM", "yes")
	selectKey(t, m, "env.CLAUDE_CODE_EFFORT_LEVEL")
	if len(m.list.Items()) != len(m.sch.EnvVars)+1 {
		t.Fatal("missing catalogue entries or custom variables")
	}
	m.startSearch()
	found := false
	for _, li := range m.list.Items() {
		if li.(item).st.Key == "env.MY.CUSTOM" {
			found = true
		}
	}
	if !found {
		t.Fatal("custom variable missing from search")
	}
	m.searching = false
	selectKey(t, m, "env.MY.CUSTOM")
	if v, _ := m.effective(m.selected()); v != "yes" {
		t.Fatalf("literal name value = %v", v)
	}
	for _, width := range []int{80, 120, 160} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 32})
		view := ansi.Strip(m.View().Content)
		if lipgloss.Width(view) > width || lipgloss.Height(view) > 32 {
			t.Fatalf("view overflow: %dx%d at width %d", lipgloss.Width(view), lipgloss.Height(view), width)
		}
		for _, want := range []string{"Environment", "MY.CUSTOM", "q quit"} {
			if !strings.Contains(view, want) {
				t.Fatalf("width %d missing %s\n%s", width, want, view)
			}
		}
		m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
		if !strings.Contains(ansi.Strip(m.View().Content), "Variable garden") {
			t.Fatal("expanded details missing Variable garden")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	}
}

func TestEnvironmentCyclePreservesStringsAndScopes(t *testing.T) {
	m := envModel(t)
	m.scope = store.ScopeProject
	m.files[store.ScopeProject].Set("env.KEEP", "keep")
	m.files[store.ScopeProject].Set("model", "sonnet")
	m.files[store.ScopeManaged].Set("env.CLAUDE_CODE_EFFORT_LEVEL", "high")
	selectKey(t, m, "env.CLAUDE_CODE_EFFORT_LEVEL")
	for _, want := range []string{"low", "medium"} {
		pressTab(m)
		f, err := store.Open(store.ScopeProject, m.cwd)
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := f.Get("env.CLAUDE_CODE_EFFORT_LEVEL"); got != want {
			t.Fatalf("got %v want %s", got, want)
		}
		if got, _ := f.Get("env.KEEP"); got != "keep" {
			t.Fatal("lost sibling")
		}
		if got, _ := f.Get("model"); got != "sonnet" {
			t.Fatal("lost setting")
		}
		if it := m.list.SelectedItem().(item); it.scope != "managed" {
			t.Fatal("lost precedence")
		}
	}
	if _, ok := m.files[store.ScopeUser].Get("env.CLAUDE_CODE_EFFORT_LEVEL"); ok {
		t.Fatal("wrote wrong scope")
	}
}

func TestEnvironmentTextSubmitAndCancel(t *testing.T) {
	for _, text := range []string{"example.com,localhost", `"quoted value"`, "", "  padded  "} {
		t.Run(text, func(t *testing.T) {
			m := envModel(t)
			selectKey(t, m, "env.NO_PROXY")
			runEnvCommand(m, m.startEdit())
			if text != "" {
				m.Update(tea.PasteMsg{Content: text})
			}
			_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			runEnvCommand(m, cmd)
			if m.mode != modeBrowse {
				t.Fatal("Enter did not submit")
			}
			f, err := store.Open(store.ScopeUser, m.cwd)
			if err != nil {
				t.Fatal(err)
			}
			if got, _ := f.Get("env.NO_PROXY"); got != text {
				t.Fatalf("got %#v want %#v", got, text)
			}
			runEnvCommand(m, m.startEdit())
			m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
			if m.mode != modeBrowse || m.edit != nil {
				t.Fatal("Escape did not cancel")
			}
		})
	}
}

func TestEnvironmentFormTabDoesNotSave(t *testing.T) {
	m := envModel(t)
	selectKey(t, m, "env.CLAUDE_CODE_EFFORT_LEVEL")
	runEnvCommand(m, m.startEdit())
	pressTab(m)
	if m.mode != modeEdit {
		t.Fatal("Tab submitted form")
	}
	if _, ok := m.files[store.ScopeUser].Get(m.selected().Key); ok {
		t.Fatal("Tab saved form")
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	runEnvCommand(m, cmd)
	if got, _ := m.files[store.ScopeUser].Get("env.CLAUDE_CODE_EFFORT_LEVEL"); got != "medium" {
		t.Fatalf("option = %v", got)
	}
}

func TestEnvironmentPresenceFlagsAndFailedSaves(t *testing.T) {
	m := envModel(t)
	selectKey(t, m, "env.DISABLE_TELEMETRY")
	if cyclable(m.selected()) {
		t.Fatal("presence flag offered false toggle")
	}
	for _, failure := range []string{"save", "load"} {
		m := envModel(t)
		selectKey(t, m, "env.CLAUDE_CODE_EFFORT_LEVEL")
		if failure == "save" {
			m.files[store.ScopeUser].Path = t.TempDir()
		} else {
			m.errs[store.ScopeUser] = &storeError{}
		}
		before := store.Format(m.files[store.ScopeUser].Data)
		pressTab(m)
		if !m.statusErr || store.Format(m.files[store.ScopeUser].Data) != before {
			t.Fatal("failed save changed state")
		}
	}
}

func TestEnvironmentRemoveOnlySelectedVariable(t *testing.T) {
	m := envModel(t)
	m.files[store.ScopeUser].Set("env.NO_PROXY", "localhost")
	m.files[store.ScopeUser].Set("env.KEEP", "keep")
	selectKey(t, m, "env.NO_PROXY")
	m.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	if m.mode != modeConfirmUnset {
		t.Fatal("missing remove confirmation")
	}
	m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	f, err := store.Open(store.ScopeUser, m.cwd)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.Get("env.NO_PROXY"); ok {
		t.Fatal("not removed")
	}
	if v, _ := f.Get("env.KEEP"); v != "keep" {
		t.Fatal("removed sibling")
	}
}

// Deliver asynchronous form navigation just as Bubble Tea does. Ignore blink
// ticks so the test event loop terminates once the input has been handled.
func runEnvCommand(m *model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if _, ok := msg.(cursor.BlinkMsg); ok {
		return
	}
	rv := reflect.ValueOf(msg)
	if rv.IsValid() && rv.Kind() == reflect.Slice {
		for i := 0; i < rv.Len(); i++ {
			if child, ok := rv.Index(i).Interface().(tea.Cmd); ok {
				runEnvCommand(m, child)
			}
		}
		return
	}
	_, next := m.Update(msg)
	runEnvCommand(m, next)
}

func TestEnvironmentMultilineHeaders(t *testing.T) {
	m := envModel(t)
	selectKey(t, m, "env.ANTHROPIC_CUSTOM_HEADERS")
	runEnvCommand(m, m.startEdit())
	text := "X-First: one\nX-Second: two"
	m.Update(tea.PasteMsg{Content: text})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	runEnvCommand(m, cmd)
	if m.mode != modeBrowse {
		t.Fatal("Enter did not save headers")
	}
	if got, _ := m.files[store.ScopeUser].Get("env.ANTHROPIC_CUSTOM_HEADERS"); got != text {
		t.Fatalf("headers = %#v", got)
	}
}
