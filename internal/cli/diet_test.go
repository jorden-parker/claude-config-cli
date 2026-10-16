package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jorden-parker/claude-config-cli/internal/diet"
)

// runDiet runs ccfg diet and keeps stdout apart from stderr, so a catalogue
// note cannot end up inside the JSON.
func runDiet(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := Root()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(append([]string{"diet"}, args...))
	err := root.Execute()
	return stdout.String(), err
}

func TestDietJSONReportsStatesAndExitsZero(t *testing.T) {
	home, dir := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	seedUser(t, home, `{"env":{"ENABLE_TOOL_SEARCH":"false"},"includeGitInstructions":false}`)

	out, err := runDiet(t, "--json", "-C", dir)
	if err != nil {
		t.Fatalf("exit status: %v\n%s", err, out)
	}
	var rows []struct {
		ID        string   `json:"id"`
		State     string   `json:"state"`
		Where     string   `json:"where"`
		Offenders []string `json:"offenders"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if len(rows) != len(diet.Recs) {
		t.Fatalf("%d rows, want %d", len(rows), len(diet.Recs))
	}
	byID := map[string]int{}
	for i, r := range rows {
		byID[r.ID] = i
	}
	if r := rows[byID["git-instructions-off"]]; r.State != "applied" || r.Where != "user" {
		t.Fatalf("git-instructions-off = %+v", r)
	}
	if r := rows[byID["mcp-tool-search-not-disabled"]]; r.State != "warning" || len(r.Offenders) != 1 || r.Offenders[0] != "env.ENABLE_TOOL_SEARCH" {
		t.Fatalf("mcp-tool-search-not-disabled = %+v", r)
	}
	if r := rows[byID["habit-clear"]]; r.State != "info" {
		t.Fatalf("habit-clear = %+v", r)
	}
}

func TestDietTextPrintsEveryRow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	out, err := runDiet(t, "-C", t.TempDir())
	if err != nil {
		t.Fatalf("exit status: %v\n%s", err, out)
	}
	if !strings.HasPrefix(out, "Context diet: ") {
		t.Fatalf("no summary line:\n%s", out)
	}
	for _, rec := range diet.Recs {
		if !strings.Contains(out, rec.Title) {
			t.Fatalf("missing %q", rec.Title)
		}
	}
}
