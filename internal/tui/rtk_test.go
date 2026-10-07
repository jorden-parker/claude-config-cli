package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jorden-parker/claude-config-cli/internal/rtk"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

func rtkModel(t *testing.T, found bool) (*model, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	body := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"~/.claude/hooks/other.py"}]}]}}`
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	prev := rtkBinary
	rtkBinary = func() (string, error) {
		if found {
			return "/opt/homebrew/bin/rtk", nil
		}
		return "", errors.New("rtk is not on your PATH")
	}
	t.Cleanup(func() { rtkBinary = prev })
	m := newModel(t.TempDir())
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 50})
	return m, home
}

func TestRTKRowsToggleHooksAndKeepOthers(t *testing.T) {
	m, home := rtkModel(t, true)
	selectKey(t, m, rtkBash)
	if !strings.Contains(ansi.Strip(m.doc.GetContent()), "○ Bash") {
		t.Fatalf("Bash hook should start off:\n%s", ansi.Strip(m.doc.GetContent()))
	}
	pressTab(m)
	selectKey(t, m, rtkRead)
	pressTab(m)

	f, err := store.Open(store.ScopeUser, "")
	if err != nil {
		t.Fatal(err)
	}
	if st := rtk.On(f); !st.Bash || !st.Read {
		t.Fatalf("both hooks should be on after two tabs, got %+v", st)
	}
	raw, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if !strings.Contains(string(raw), "other.py") {
		t.Fatalf("the existing hook was lost:\n%s", raw)
	}
	if !strings.Contains(ansi.Strip(m.doc.GetContent()), "● Read") {
		t.Fatalf("details should show the Read hook on:\n%s", ansi.Strip(m.doc.GetContent()))
	}

	selectKey(t, m, rtkBash)
	pressTab(m)
	f, _ = store.Open(store.ScopeUser, "")
	if st := rtk.On(f); st.Bash || !st.Read {
		t.Fatalf("tab again should turn only Bash off, got %+v", st)
	}
}

func TestRTKBashNeedsTheBinary(t *testing.T) {
	m, _ := rtkModel(t, false)
	selectKey(t, m, rtkBash)
	pressTab(m)
	f, _ := store.Open(store.ScopeUser, "")
	if st := rtk.On(f); st.Bash {
		t.Fatal("the Bash hook must not be written when rtk is missing")
	}
	if !strings.Contains(m.status, "rtk") {
		t.Fatalf("status should explain, got %q", m.status)
	}
}

func TestRTKSectionInCabinetAndSearch(t *testing.T) {
	m, _ := rtkModel(t, true)
	if !contains(m.sections, rtkSection) {
		t.Fatal("RTK section missing")
	}
	m.startSearch()
	found := false
	for _, li := range m.items() {
		if it := li.(item); !it.header && it.st.Key == rtkRead {
			found = true
		}
	}
	if !found {
		t.Fatal("search should list the rtk rows")
	}
}
