// Package snapshot saves settings independently of Claude Code's configuration.
package snapshot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jorden-parker/claude-config-cli/internal/statusline"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

// Archive preserves file bytes, including unknown settings and key order.
type Archive struct {
	Version int               `json:"version"`
	Project string            `json:"project"`
	Files   map[string][]byte `json:"files"`
}

func paths(project string) map[string]string {
	result := map[string]string{}
	for _, scope := range store.Writable {
		result[string(scope)] = store.Path(scope, project)
		if scope != store.ScopeGlobal {
			result[string(scope)+"-statusline"] = statusline.ConfigPath(scope, project)
		}
	}
	return result
}

func validObject(data []byte) bool {
	var obj map[string]json.RawMessage
	return json.Unmarshal(data, &obj) == nil && obj != nil
}

// Save creates a new private archive. It never replaces an existing snapshot.
func Save(filename, project string) (int, error) {
	project, err := filepath.Abs(project)
	if err != nil {
		return 0, err
	}
	filename, err = filepath.Abs(filename)
	if err != nil {
		return 0, err
	}
	// Resolve the parent so a symlink cannot hide a snapshot inside .claude.
	parent, err := filepath.EvalSymlinks(filepath.Dir(filename))
	if err != nil {
		return 0, err
	}
	filename = filepath.Join(parent, filepath.Base(filename))
	targets := paths(project)
	for _, target := range targets {
		dir := filepath.Dir(target)
		if filepath.Base(dir) != ".claude" {
			continue
		}
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		}
		rel, err := filepath.Rel(dir, filename)
		if err == nil && rel != ".." && !filepath.IsAbs(rel) && (len(rel) < 3 || rel[:3] != ".."+string(filepath.Separator)) {
			return 0, fmt.Errorf("save the snapshot outside %s so deleting configuration will not delete it", dir)
		}
	}
	a := Archive{Version: 1, Project: project, Files: map[string][]byte{}}
	for key, path := range targets {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if !validObject(data) {
			return 0, fmt.Errorf("%s: expected a JSON object", path)
		}
		a.Files[key] = data
	}
	if len(a.Files) == 0 {
		return 0, fmt.Errorf("no settings files to snapshot")
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return 0, err
	}
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close()
		os.Remove(filename)
		return 0, err
	}
	if err := f.Close(); err != nil {
		os.Remove(filename)
		return 0, err
	}
	return len(a.Files), nil
}

// Restore validates the whole archive and checks every destination before writing.
// With no project override it restores project settings to their original root.
func Restore(filename, project string, force bool) (int, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return 0, err
	}
	var a Archive
	if err := json.Unmarshal(data, &a); err != nil {
		return 0, fmt.Errorf("invalid snapshot: %w", err)
	}
	if a.Version != 1 {
		return 0, fmt.Errorf("unsupported snapshot version %d", a.Version)
	}
	if len(a.Files) == 0 {
		return 0, fmt.Errorf("snapshot contains no files")
	}
	if project == "" {
		if !filepath.IsAbs(a.Project) {
			return 0, fmt.Errorf("snapshot project must be an absolute path")
		}
		project = a.Project
	}
	targets := paths(project)
	destinations := map[string][]byte{}
	for key, data := range a.Files {
		path, ok := targets[key]
		if !ok {
			return 0, fmt.Errorf("unknown snapshot file %q", key)
		}
		if !validObject(data) {
			return 0, fmt.Errorf("snapshot file %s: expected a JSON object", key)
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return 0, err
		}
		if previous, exists := destinations[path]; exists && !bytes.Equal(previous, data) {
			return 0, fmt.Errorf("snapshot contains conflicting contents for %s", path)
		}
		destinations[path] = data
		info, err := os.Lstat(path)
		if err == nil {
			if !info.Mode().IsRegular() {
				return 0, fmt.Errorf("%s: refusing to replace a non-regular file", path)
			}
			if !force {
				return 0, fmt.Errorf("%s already exists; use --force to replace saved files", path)
			}
		} else if !os.IsNotExist(err) {
			return 0, err
		}
	}
	count := 0
	for path, data := range destinations {
		if err := writeFile(path, data, force); err != nil {
			return count, fmt.Errorf("restored %d files before failing at %s: %w", count, path, err)
		}
		count++
	}
	return count, nil
}

func writeFile(path string, data []byte, force bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if !force {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			f.Close()
			os.Remove(path)
			return err
		}
		return f.Close()
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".ccfg-restore-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
