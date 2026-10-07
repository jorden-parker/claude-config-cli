// Package statusline builds a Claude Code status line out of the fields the
// docs list at https://code.claude.com/docs/en/statusline, and keeps whatever
// status line command was already configured by running it alongside.
package statusline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jorden-parker/claude-config-cli/internal/store"
)

// DocURL is the reference the field catalogue comes from.
const DocURL = "https://code.claude.com/docs/en/statusline"

// Where the existing status line goes relative to ccfg's own fields.
const (
	ExistingStart  = "start"  // same line, before the fields
	ExistingEnd    = "end"    // same line, after the fields
	ExistingAbove  = "above"  // its own row above
	ExistingBelow  = "below"  // its own row below
	ExistingHidden = "hidden" // kept in the config, not shown
)

// Positions lists the placements in the order the editor cycles through them.
var Positions = []string{ExistingStart, ExistingEnd, ExistingAbove, ExistingBelow, ExistingHidden}

// Separators are the choices the editor cycles through. Any string is valid.
var Separators = []string{"  ", " | ", " · ", " › "}

// DefaultFields is what a new status line shows when there is nothing to keep.
var DefaultFields = []string{"model", "dir", "branch", "ctx_used"}

// inheritedTimeout bounds the existing command. The docs give no timeout, and
// a slow script holds up every update.
const inheritedTimeout = 3 * time.Second

// Config is one scope's status line, stored next to its settings file.
type Config struct {
	Fields    []string `json:"fields"`    // field IDs, in display order
	Separator string   `json:"separator"` // between fields
	Color     bool     `json:"color"`
	Existing  string   `json:"existing"` // one of Positions
	// Inherited is the status line command that was in place before ccfg's.
	// The renderer runs it with the same input and includes its output.
	Inherited string `json:"inherited,omitempty"`
	// Previous is the statusLine value ccfg replaced in the settings file,
	// put back as it was when the status line is turned off.
	Previous any `json:"previous,omitempty"`
}

// Default is the config used when no file exists yet.
func Default() *Config {
	return &Config{Fields: []string{}, Separator: Separators[0], Color: true, Existing: ExistingStart}
}

// ConfigPath is the config file that belongs to a scope's settings file.
func ConfigPath(scope store.Scope, cwd string) string {
	name := "ccfg-statusline.json"
	if scope == store.ScopeLocal {
		name = "ccfg-statusline.local.json"
	}
	return filepath.Join(filepath.Dir(store.Path(scope, cwd)), name)
}

// Load reads a config. A missing file yields Default.
func Load(path string) (*Config, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, c); err != nil {
		return Default(), err
	}
	if !contains(Positions, c.Existing) {
		c.Existing = ExistingStart
	}
	if c.Fields == nil {
		c.Fields = []string{}
	}
	return c, nil
}

// Save writes a config with two-space indentation.
func Save(path string, c *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

// Has reports whether a field is shown.
func (c *Config) Has(id string) bool { return contains(c.Fields, id) }

// Toggle shows a field at the end of the line, or hides it.
func (c *Config) Toggle(id string) bool {
	for i, f := range c.Fields {
		if f == id {
			c.Fields = append(c.Fields[:i:i], c.Fields[i+1:]...)
			return false
		}
	}
	c.Fields = append(c.Fields, id)
	return true
}

// Move swaps a shown field with the one delta places away and reports
// whether it moved.
func (c *Config) Move(id string, delta int) bool {
	for i, f := range c.Fields {
		if f == id {
			j := i + delta
			if j < 0 || j >= len(c.Fields) {
				return false
			}
			c.Fields[i], c.Fields[j] = c.Fields[j], c.Fields[i]
			return true
		}
	}
	return false
}

// Compose builds the status line from the JSON Claude Code sends on stdin and
// the output of the existing command. Fields with nothing to show are dropped.
func Compose(c *Config, input []byte, inherited string, now time.Time) string {
	in := parse(input, now)
	var segs []string
	for _, id := range c.Fields {
		f := Lookup(id)
		if f == nil {
			continue
		}
		if v := f.render(in); v != "" {
			segs = append(segs, paint(c.Color, f.color, v))
		}
	}
	sep := c.Separator
	if strings.TrimSpace(sep) != "" {
		sep = paint(c.Color, "245", sep)
	}
	own := strings.Join(segs, sep)

	inherited = strings.TrimRight(inherited, "\r\n")
	if c.Existing == ExistingHidden || strings.TrimSpace(inherited) == "" {
		return own
	}
	if own == "" {
		return inherited
	}
	lines := strings.Split(inherited, "\n")
	switch c.Existing {
	case ExistingEnd:
		lines[0] = own + sep + lines[0]
	case ExistingAbove:
		lines = append(lines, own)
	case ExistingBelow:
		lines = append([]string{own}, lines...)
	default:
		lines[len(lines)-1] += sep + own
	}
	return strings.Join(lines, "\n")
}

func paint(on bool, color, s string) string {
	if !on || color == "" {
		return s
	}
	return "\x1b[38;5;" + color + "m" + s + "\x1b[0m"
}

// RunInherited runs an existing status line command the way Claude Code does:
// in a shell, with the session JSON on stdin. A failure, a timeout, or a
// non-zero exit returns an error, and the caller leaves that part out.
func RunInherited(command string, input []byte) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), inheritedTimeout)
	defer cancel()
	shell, flag := "sh", "-c"
	if runtime.GOOS == "windows" {
		// Claude Code uses Git Bash when it is installed, PowerShell otherwise.
		shell = "bash"
		if _, err := exec.LookPath(shell); err != nil {
			shell, flag = "powershell", "-Command"
		}
	}
	cmd := exec.CommandContext(ctx, shell, flag, command)
	cmd.Stdin = bytes.NewReader(input)
	cmd.WaitDelay = 200 * time.Millisecond // don't wait on grandchildren that hold the pipe
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return "", errors.New("timed out after " + inheritedTimeout.String())
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			if msg := strings.TrimSpace(string(exit.Stderr)); msg != "" {
				return "", errors.New(lastLine(msg))
			}
		}
		return "", err
	}
	return string(out), nil
}

func lastLine(s string) string {
	lines := strings.Split(s, "\n")
	return lines[len(lines)-1]
}

// Render is the status line command itself: it loads the config, runs the
// existing command, and returns the text to print. It never fails, because a
// non-zero exit blanks the whole status line.
func Render(configPath string, input []byte) string {
	c, _ := Load(resolve(configPath, input))
	out := ""
	if c.Inherited != "" && c.Existing != ExistingHidden && !IsOurs(c.Inherited) {
		out, _ = RunInherited(c.Inherited, input)
	}
	return Compose(c, input, out, time.Now())
}

// resolve expands ~ and anchors a relative path at the project directory, so
// a project's settings file can name its config without an absolute path.
func resolve(path string, input []byte) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	in := parse(input, time.Time{})
	if dir := in.str("workspace.project_dir"); dir != "" {
		return filepath.Join(dir, path)
	}
	return path
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
