package statusline

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jorden-parker/claude-config-cli/internal/store"
)

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

// Program is the binary name written into settings when its path is unknown.
var Program = "ccfg"

// executable is swapped out in tests.
var executable = os.Executable

const renderArgs = " statusline render --config "

// IsOurs reports whether a statusLine command is ccfg's own renderer. It
// guards against wrapping the renderer inside itself.
func IsOurs(command string) bool { return strings.Contains(command, renderArgs) }

// CommandOf pulls the command out of a statusLine settings value.
func CommandOf(v any) string {
	obj, _ := v.(map[string]any)
	s, _ := obj["command"].(string)
	return s
}

// Command is the statusLine command for a scope. User and local settings name
// the binary and config by path, because the shell that runs a status line
// may have a different PATH. Project settings are shared, so they name both
// relative to the project instead.
func Command(scope store.Scope, cwd string) string {
	if scope == store.ScopeProject {
		return Program + renderArgs + ".claude/" + filepath.Base(ConfigPath(scope, cwd))
	}
	return shellPath(binary()) + renderArgs + shellPath(ConfigPath(scope, cwd))
}

// ProgramPath is the installed binary, written the way a shell reads it, for
// other hooks ccfg registers in settings.
func ProgramPath() string { return shellPath(binary()) }

func binary() string {
	exe, err := executable()
	// A `go run` or test binary is deleted when it exits; use the installed one.
	if err != nil || strings.Contains(exe, "go-build") || strings.HasSuffix(exe, ".test") {
		if found, err := exec.LookPath(Program); err == nil {
			return found
		}
		return Program
	}
	return exe
}

// shellPath writes a path so a shell reads it as one word. Forward slashes
// keep Git Bash on Windows from eating the separators.
func shellPath(p string) string {
	p = filepath.ToSlash(p)
	safe := func(s string) bool {
		return strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._/-") == ""
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rest, ok := strings.CutPrefix(p, filepath.ToSlash(home)+"/"); ok && safe(rest) {
			return "~/" + rest
		}
	}
	if safe(p) {
		return p
	}
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
}

// checkArg makes the renderer validate itself without running any saved
// status line command. Older binaries reject it, which is reported as an error.
const checkArg = " --check"

// Check runs ccfg's renderer command once in its no-execution mode with the
// example session. A renderer that cannot run would blank the whole status
// line, so callers check before pointing settings at it. The command must be
// one built by Command; the saved status line command is never run here, and
// there is no fallback to a normal render.
func Check(command, cwd string) error {
	if !IsOurs(command) {
		return fmt.Errorf("not checking %q: it is not %s's own renderer", command, Program)
	}
	if _, err := runShell(command+checkArg, Sample(cwd, time.Now())); err != nil {
		return fmt.Errorf("the status line command does not run yet (%v). Install this version of %s first, for example with ./install.sh. Command: %s", err, Program, command)
	}
	return nil
}

// Existing finds the status line command that ccfg's would sit beside for a
// target scope, and the settings file it comes from. It looks in the target
// file first, then at the value in effect from the other files. When that
// value is already ccfg's renderer, the command it kept is returned instead,
// with the file that command originally came from. That origin never exceeds
// the wrapper's own: a project or local wrapper stays project or local, and a
// user or managed wrapper needs a recognised recorded origin. An empty scope
// means the origin is unknown.
func Existing(files map[store.Scope]*store.File, target store.Scope, cwd string) (string, store.Scope) {
	order := []store.Scope{target, store.ScopeManaged, store.ScopeLocal, store.ScopeProject, store.ScopeUser}
	for _, sc := range order {
		f := files[sc]
		if f == nil {
			continue
		}
		v, ok := f.Get("statusLine")
		if !ok {
			continue
		}
		command := CommandOf(v)
		if IsOurs(command) {
			c, err := Load(ConfigPath(sc, cwd))
			if err != nil {
				return "", ""
			}
			return c.Inherited, boundedOrigin(sc, c.InheritedScope)
		}
		return command, sc
	}
	return "", ""
}

// boundedOrigin is the origin to believe for a command kept by a wrapper in
// the wrapper scope's file.
func boundedOrigin(wrapper, recorded store.Scope) store.Scope {
	if wrapper != store.ScopeUser && wrapper != store.ScopeManaged {
		return wrapper
	}
	switch recorded {
	case store.ScopeUser, store.ScopeProject, store.ScopeLocal, store.ScopeManaged:
		return recorded
	}
	return ""
}

// On reports whether a settings file's statusLine is ccfg's renderer.
func On(f *store.File) bool {
	if f == nil {
		return false
	}
	v, _ := f.Get("statusLine")
	return IsOurs(CommandOf(v))
}

// Enable points statusLine in f at command and records what it replaces in c.
// existing is the command to keep showing when f itself sets none, and
// existingScope is the file it came from. Other keys
// of the statusLine object, such as padding and refreshInterval, are kept.
// The caller saves c first, then f.
func Enable(f *store.File, c *Config, command, existing string, existingScope store.Scope) error {
	cur, has := f.Get("statusLine")
	next := map[string]any{}
	if obj, ok := cur.(map[string]any); ok {
		for k, v := range obj {
			next[k] = v
		}
	}
	if !IsOurs(CommandOf(cur)) {
		c.Inherited, c.InheritedScope = CommandOf(cur), f.Scope
		if c.Inherited == "" && !IsOurs(existing) {
			c.Inherited, c.InheritedScope = existing, existingScope
		}
		if c.Inherited == "" {
			c.InheritedScope = ""
		}
		c.Previous = nil
		if has {
			c.Previous = cur
		}
	}
	if len(c.Fields) == 0 && c.Inherited == "" {
		c.Fields = append([]string{}, DefaultFields...)
	}
	next["type"], next["command"] = "command", command
	return f.Set("statusLine", next)
}

// Disable puts back the statusLine value that Enable replaced, or removes the
// key when there was none. It leaves a status line that isn't ccfg's alone.
func Disable(f *store.File, c *Config) error {
	if !On(f) {
		return nil
	}
	if c.Previous != nil {
		return f.Set("statusLine", c.Previous)
	}
	f.Unset("statusLine")
	return nil
}

// Sample is the example input from the docs, pointed at dir, for previews.
// Reset times are set ahead of now, since Claude Code drops expired windows.
func Sample(dir string, now time.Time) []byte {
	b, _ := json.Marshal(map[string]any{
		"cwd":             dir,
		"session_id":      "abc12345-sample",
		"session_name":    "my-session",
		"transcript_path": filepath.Join(dir, "transcript.jsonl"),
		"model":           map[string]any{"id": "claude-opus-5-5", "display_name": "Opus"},
		"workspace": map[string]any{
			"current_dir": dir,
			"project_dir": dir,
			"added_dirs":  []any{},
			"repo":        map[string]any{"host": "github.com", "owner": "anthropics", "name": "claude-code"},
		},
		"version":      "2.1.90",
		"output_style": map[string]any{"name": "default"},
		"cost": map[string]any{
			"total_cost_usd":        0.01234,
			"total_duration_ms":     45000,
			"total_api_duration_ms": 2300,
			"total_lines_added":     156,
			"total_lines_removed":   23,
		},
		"context_window": map[string]any{
			"total_input_tokens":   15500,
			"total_output_tokens":  1200,
			"context_window_size":  200000,
			"used_percentage":      8,
			"remaining_percentage": 92,
			"current_usage": map[string]any{
				"input_tokens":                8500,
				"output_tokens":               1200,
				"cache_creation_input_tokens": 5000,
				"cache_read_input_tokens":     2000,
			},
		},
		"exceeds_200k_tokens": false,
		"prompt_cache": map[string]any{
			"warm": true, "caching_observed": true, "ttl": "1h",
			"expires_at": now.Add(time.Hour).Unix(), "hit_ratio": 0.91,
		},
		"fast_mode": false,
		"effort":    map[string]any{"level": "high"},
		"thinking":  map[string]any{"enabled": true},
		"rate_limits": map[string]any{
			"five_hour": map[string]any{"used_percentage": 23.5, "resets_at": now.Add(2 * time.Hour).Unix()},
			"seven_day": map[string]any{"used_percentage": 41.2, "resets_at": now.Add(72 * time.Hour).Unix()},
			"spend_limit": map[string]any{
				"used_percentage": 62.8, "resets_at": now.Add(240 * time.Hour).Unix(),
				"used_usd": 314.12, "limit_usd": 500, "period": "monthly",
			},
		},
		"vim":   map[string]any{"mode": "NORMAL"},
		"agent": map[string]any{"name": "security-reviewer"},
		"pr": map[string]any{
			"number": 1234, "url": "https://github.com/anthropics/claude-code/pull/1234", "review_state": "pending",
		},
		"worktree": map[string]any{"name": "my-feature", "branch": "worktree-my-feature"},
	})
	return b
}
