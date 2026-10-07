// Package rtk routes Claude Code's command output through rtk
// (https://github.com/rtk-ai/rtk), which condenses it before Claude reads it.
//
// Two PreToolUse hooks do the work. rtk's own hook rewrites Bash commands,
// for example `cat f` to `rtk read f`. ccfg's hook refuses the built-in Read
// tool and tells Claude which shell command to run instead, since built-in
// tools never pass through the Bash hook.
package rtk

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/jorden-parker/claude-config-cli/internal/store"
)

// Program is the binary name written into settings when its path is unknown.
var Program = "ccfg"

// BashHook is the command rtk registers for Claude Code (rtk init -g).
const BashHook = "rtk hook claude"

// readArgs marks ccfg's Read hook so it can be found and removed later.
const readArgs = " rtk read-hook"

// Binary locates rtk on PATH.
func Binary() (string, error) {
	p, err := exec.LookPath("rtk")
	if err != nil {
		return "", fmt.Errorf("rtk is not on your PATH. Install it first: brew install rtk (https://github.com/rtk-ai/rtk)")
	}
	return p, nil
}

// ReadCommand is the Read hook command for a scope. Project settings are
// shared, so they name the binary without a path.
func ReadCommand(scope store.Scope, program string) string {
	if scope == store.ScopeProject {
		return Program + readArgs
	}
	return program + readArgs
}

// State reports which of the two hooks a settings file has.
type State struct {
	Bash bool
	Read bool
}

// On reports which hooks f carries.
func On(f *store.File) State {
	var st State
	if f == nil {
		return st
	}
	for _, entry := range preToolUse(f) {
		switch {
		case hasCommand(entry, func(c string) bool { return c == BashHook }):
			st.Bash = true
		case hasCommand(entry, func(c string) bool { return strings.HasSuffix(c, readArgs) }):
			st.Read = true
		}
	}
	return st
}

func preToolUse(f *store.File) []any {
	v, _ := f.Get("hooks.PreToolUse")
	list, _ := v.([]any)
	return list
}

func hasCommand(entry any, match func(string) bool) bool {
	obj, _ := entry.(map[string]any)
	hooks, _ := obj["hooks"].([]any)
	for _, h := range hooks {
		hm, _ := h.(map[string]any)
		if c, _ := hm["command"].(string); match(c) {
			return true
		}
	}
	return false
}

func entry(matcher, command string) map[string]any {
	return map[string]any{
		"matcher": matcher,
		"hooks":   []any{map[string]any{"type": "command", "command": command}},
	}
}

// Enable appends the Bash hook and, when readCommand is not empty, the Read
// hook to f's PreToolUse hooks. Entries already there are left alone, and so
// is every other hook.
func Enable(f *store.File, readCommand string) error {
	if err := SetBash(f, true); err != nil {
		return err
	}
	if readCommand == "" {
		return nil
	}
	return SetRead(f, true, readCommand)
}

// Disable removes the two hooks from f, leaving other hooks byte-identical.
func Disable(f *store.File) {
	_ = SetBash(f, false)
	_ = SetRead(f, false, "")
}

// SetBash adds or removes rtk's Bash hook.
func SetBash(f *store.File, on bool) error {
	return set(f, on, func(c string) bool { return c == BashHook }, entry("Bash", BashHook))
}

// SetRead adds or removes ccfg's Read hook. command is the hook command to
// write when turning it on.
func SetRead(f *store.File, on bool, command string) error {
	return set(f, on, func(c string) bool { return strings.HasSuffix(c, readArgs) }, entry("Read", command))
}

// set removes every entry that match recognises and, when on, appends add.
// Empty hooks objects left behind are removed too.
func set(f *store.File, on bool, match func(string) bool, add map[string]any) error {
	var kept []any
	for _, e := range preToolUse(f) {
		if !hasCommand(e, match) {
			kept = append(kept, e)
		}
	}
	if on {
		kept = append(kept, add)
	}
	if len(kept) > 0 {
		return f.Set("hooks.PreToolUse", kept)
	}
	f.Unset("hooks.PreToolUse")
	if v, ok := f.Get("hooks"); ok {
		if m, _ := v.(map[string]any); len(m) == 0 {
			f.Unset("hooks")
		}
	}
	return nil
}

// DocURL is rtk's home.
const DocURL = "https://github.com/rtk-ai/rtk"

// ReadHook answers a PreToolUse call for the Read tool: it refuses the call
// and names the shell commands to use instead. `sed -n` is suggested for the
// read that must come before an Edit, because rtk leaves sed alone while it
// rewrites cat, head, and tail to `rtk read`, which Claude Code does not count
// as a read of the file.
func ReadHook(in io.Reader, out io.Writer) error {
	var req struct {
		ToolInput struct {
			FilePath string `json:"file_path"`
		} `json:"tool_input"`
	}
	if err := json.NewDecoder(in).Decode(&req); err != nil {
		return fmt.Errorf("hook input: %w", err)
	}
	path := req.ToolInput.FilePath
	if path == "" {
		path = "<file>"
	}
	reason := fmt.Sprintf("Read is off; command output goes through rtk. Run `rtk read %s` in Bash. Just before an Edit, view the file with `sed -n '1,200p' %s` instead.", path, path)
	return json.NewEncoder(out).Encode(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		},
	})
}
