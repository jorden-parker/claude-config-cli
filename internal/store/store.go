// Package store reads and writes Claude Code settings files without
// discarding keys it doesn't know about.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
)

// Scope is one settings file.
type Scope string

const (
	ScopeUser    Scope = "user"    // ~/.claude/settings.json
	ScopeProject Scope = "project" // .claude/settings.json
	ScopeLocal   Scope = "local"   // .claude/settings.local.json
	ScopeGlobal  Scope = "global"  // ~/.claude.json
	ScopeManaged Scope = "managed" // read-only, deployed by an organization
)

// Writable scopes in precedence order, lowest first.
var Writable = []Scope{ScopeUser, ScopeProject, ScopeLocal, ScopeGlobal}

// All scopes shown in the UI, lowest precedence first.
var All = []Scope{ScopeUser, ScopeProject, ScopeLocal, ScopeManaged, ScopeGlobal}

// ParseScope converts user input into a Scope.
func ParseScope(s string) (Scope, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "user", "u", "":
		return ScopeUser, nil
	case "project", "p":
		return ScopeProject, nil
	case "local", "l":
		return ScopeLocal, nil
	case "global", "g":
		return ScopeGlobal, nil
	case "managed", "m":
		return ScopeManaged, nil
	}
	return "", fmt.Errorf("unknown scope %q (use user, project, local, or global)", s)
}

// Path returns the file for a scope. cwd is the project root.
func Path(scope Scope, cwd string) string {
	home, _ := os.UserHomeDir()
	switch scope {
	case ScopeUser:
		return filepath.Join(home, ".claude", "settings.json")
	case ScopeProject:
		return filepath.Join(cwd, ".claude", "settings.json")
	case ScopeLocal:
		return filepath.Join(cwd, ".claude", "settings.local.json")
	case ScopeGlobal:
		return filepath.Join(home, ".claude.json")
	case ScopeManaged:
		switch runtime.GOOS {
		case "darwin":
			return "/Library/Application Support/ClaudeCode/managed-settings.json"
		case "windows":
			return `C:\Program Files\ClaudeCode\managed-settings.json`
		default:
			return "/etc/claude-code/managed-settings.json"
		}
	}
	return ""
}

// Allowed reports whether a setting may be written to a scope, per the docs.
func Allowed(st *schema.Setting, scope Scope) bool {
	if scope == ScopeManaged {
		return false
	}
	switch st.ScopeClass {
	case schema.ScopeGlobal:
		return scope == ScopeGlobal
	case schema.ScopeManaged:
		return false
	case schema.ScopeUserManaged:
		return scope == ScopeUser
	case schema.ScopeUserLocalManaged:
		return scope == ScopeUser || scope == ScopeLocal
	default:
		return scope == ScopeUser || scope == ScopeProject || scope == ScopeLocal
	}
}

// File is one loaded settings file.
type File struct {
	Scope  Scope
	Path   string
	Exists bool
	Data   map[string]any
}

// Open loads a scope's file. A missing file yields an empty document.
func Open(scope Scope, cwd string) (*File, error) {
	f := &File{Scope: scope, Path: Path(scope, cwd), Data: map[string]any{}}
	b, err := os.ReadFile(f.Path)
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	f.Exists = true
	if len(strings.TrimSpace(string(b))) == 0 {
		return f, nil
	}
	if err := json.Unmarshal(b, &f.Data); err != nil {
		return f, fmt.Errorf("%s: %w", f.Path, err)
	}
	return f, nil
}

// OpenAll loads every scope. Errors are returned per scope and don't stop the rest.
func OpenAll(cwd string) (map[Scope]*File, map[Scope]error) {
	files := map[Scope]*File{}
	errs := map[Scope]error{}
	for _, s := range All {
		f, err := Open(s, cwd)
		files[s] = f
		if err != nil {
			errs[s] = err
		}
	}
	return files, errs
}

func split(path string) []string {
	if strings.HasPrefix(path, "env.") {
		return []string{"env", strings.TrimPrefix(path, "env.")}
	}
	return strings.Split(path, ".")
}

// Get returns the value at a dotted path.
func (f *File) Get(path string) (any, bool) {
	var cur any = f.Data
	for _, p := range split(path) {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// Set writes a value at a dotted path, creating parent objects as needed.
func (f *File) Set(path string, v any) error {
	parts := split(path)
	cur := f.Data
	for _, p := range parts[:len(parts)-1] {
		next, ok := cur[p]
		if !ok {
			m := map[string]any{}
			cur[p] = m
			cur = m
			continue
		}
		m, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: %q is not an object", path, p)
		}
		cur = m
	}
	cur[parts[len(parts)-1]] = v
	return nil
}

// Unset removes a dotted path. Empty parent objects are left in place.
func (f *File) Unset(path string) bool {
	parts := split(path)
	cur := f.Data
	for _, p := range parts[:len(parts)-1] {
		m, ok := cur[p].(map[string]any)
		if !ok {
			return false
		}
		cur = m
	}
	last := parts[len(parts)-1]
	if _, ok := cur[last]; !ok {
		return false
	}
	delete(cur, last)
	return true
}

// Save writes the file with two-space indentation.
func (f *File) Save() error {
	if f.Scope == ScopeManaged {
		return errors.New("managed settings are read-only")
	}
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f.Data, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.WriteFile(f.Path, b, 0o600); err != nil {
		return err
	}
	f.Exists = true
	return nil
}

// Format renders a value as compact JSON for display.
func Format(v any) string {
	if v == nil {
		return "null"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

// FormatPretty renders a value as indented JSON.
func FormatPretty(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}
