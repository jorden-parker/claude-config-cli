// Package diet is the context diet checklist: settings, environment variables,
// and files that cut what Claude Code sends as context. Each row has a check
// that reads the settings files the app has loaded. The rows follow
// docs/research/context-diet-recommendations.md. The TUI and `ccfg diet` both
// read this package, so they always agree.
package diet

import (
	"fmt"
	"strings"

	"github.com/jorden-parker/claude-config-cli/internal/store"
)

// State is the badge a row shows.
type State int

const (
	NotApplied State = iota // the recommendation is not in place
	Applied                 // it is in place, in a file or by default
	Warning                 // a trap is set: the key does nothing or hurts
	Info                    // ccfg cannot check this from the files
)

// String is the word the badge and the CLI show.
func (s State) String() string {
	switch s {
	case Applied:
		return "applied"
	case Warning:
		return "warning"
	case Info:
		return "info"
	}
	return "not applied"
}

// Groups of rows, in display order.
const (
	GroupSettings = "settings"
	GroupHabits   = "habits"
)

// GroupTitle is the heading a group gets in the list.
func GroupTitle(group string) string {
	if group == GroupHabits {
		return "Habits ccfg can't check"
	}
	return "Settings and files"
}

// precedence is the order a settings key is read in, highest first. It is the
// same order the rest of the app uses.
var precedence = []store.Scope{store.ScopeManaged, store.ScopeLocal, store.ScopeProject, store.ScopeUser}

// Input is what the checks read: the settings files that loaded, the project
// directory, and the home directory that holds the user-level files. Tests
// pass temporary directories here.
type Input struct {
	Files map[store.Scope]*store.File
	Cwd   string
	Home  string
}

// Offender is a key that sets a trap, or that stops a recommendation from
// working. Key is a settings path such as "env.ENABLE_TOOL_SEARCH".
type Offender struct {
	Key   string
	Scope store.Scope // file the key was found in
}

// Result is what a check found.
type Result struct {
	State     State
	Scope     store.Scope // file that holds the value that counts; "" if none
	Default   bool        // nothing is set, so Claude Code's default counts
	Note      string      // one plain line: what was found
	Offenders []Offender  // keys to remove to clear a warning
}

// Where names the file or default a row's state comes from; "" when neither.
func (r Result) Where() string {
	switch {
	case r.Scope != "":
		return string(r.Scope)
	case r.Default:
		return "default"
	}
	return ""
}

// ApplyKind says what Enter does on a row that is not applied.
type ApplyKind int

const (
	ApplyNone   ApplyKind = iota // nothing to change: a habit or a check only
	ApplySet                     // write Value to Key in the target file
	ApplyEditor                  // open the editor for Key so the value can be chosen
	ApplyManual                  // edit a file by hand; Text says which
)

// Apply describes how a row is applied.
type Apply struct {
	Kind  ApplyKind
	Key   string // settings path for ApplySet and ApplyEditor
	Value string // text for ApplySet, parsed the same way the editor parses it
	Text  string // how to apply it, in plain words
}

// Rec is one row of the checklist.
type Rec struct {
	ID         string
	Title      string
	Group      string
	Why        string   // one line: what it does and why it helps
	Caveat     string   // the doc's caveat, in plain words
	URL        string   // the doc page
	Confidence string   // "high" or "medium" from the doc; "" for habits
	Keys       []string // settings paths the details pane lists by file
	Apply      Apply
	Check      func(Input) Result // nil for rows ccfg cannot check
}

// Row is a checklist row with its result.
type Row struct {
	Rec    Rec
	Result Result
}

// Evaluate runs every check. Rows without a check are Info.
func Evaluate(in Input) []Row {
	out := make([]Row, 0, len(Recs))
	for _, r := range Recs {
		res := Result{State: Info, Note: "ccfg can't check this from settings files."}
		if r.Check != nil {
			res = r.Check(in)
		}
		out = append(out, Row{Rec: r, Result: res})
	}
	return out
}

// Summary counts the settings rows that can be checked.
type Summary struct {
	Checked  int
	Applied  int
	Warnings int
}

// Summarize counts settings rows that are not Info. Habits are left out.
func Summarize(rows []Row) Summary {
	var s Summary
	for _, r := range rows {
		if r.Rec.Group != GroupSettings || r.Result.State == Info {
			continue
		}
		s.Checked++
		switch r.Result.State {
		case Applied:
			s.Applied++
		case Warning:
			s.Warnings++
		}
	}
	return s
}

// Line is the one-line summary, such as "3 of 12 applied · 1 warning".
func (s Summary) Line() string {
	line := fmt.Sprintf("%d of %d applied", s.Applied, s.Checked)
	switch s.Warnings {
	case 0:
	case 1:
		line += " · 1 warning"
	default:
		line += fmt.Sprintf(" · %d warnings", s.Warnings)
	}
	return line
}

// value returns the highest-precedence value set for a settings path.
func (in Input) value(key string) (any, store.Scope, bool) {
	for _, sc := range precedence {
		if f := in.Files[sc]; f != nil {
			if v, ok := f.Get(key); ok {
				return v, sc, true
			}
		}
	}
	return nil, "", false
}

// env returns the text of an environment variable that counts, and its file.
func (in Input) env(name string) (string, store.Scope, bool) {
	v, sc, ok := in.value("env." + name)
	if !ok || v == nil {
		return "", "", false
	}
	return fmt.Sprint(v), sc, true
}

// projectRecord is this project's entry in ~/.claude.json, or nil.
func (in Input) projectRecord() map[string]any {
	g := in.Files[store.ScopeGlobal]
	if g == nil || in.Cwd == "" {
		return nil
	}
	projects, _ := g.Data["projects"].(map[string]any)
	rec, _ := projects[in.Cwd].(map[string]any)
	return rec
}

// countLines counts lines the way an editor shows them: a last line without a
// newline still counts.
func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}
