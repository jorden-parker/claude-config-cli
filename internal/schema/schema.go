// Package schema holds the catalogue of every Claude Code setting, generated
// from the official settings reference at
// https://code.claude.com/docs/en/settings-reference.
package schema

import (
	_ "embed"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jorden-parker/claude-config-cli/internal/match"
)

// Regenerate the embedded catalogue from the live docs (needs the network):
//
//go:generate go run ../../cmd/ccfg sync --embed --out schema.json
//go:embed schema.json
var raw []byte

// CurrentFormat is the catalogue layout this build understands. A cached
// catalogue with another format is ignored in favour of the embedded one.
const CurrentFormat = 1

// Kind describes how a setting is edited and validated.
type Kind string

const (
	KindBool         Kind = "bool"
	KindEnum         Kind = "enum"
	KindEnumOrString Kind = "enum-or-string"
	KindString       Kind = "string"
	KindNumber       Kind = "number"
	KindArray        Kind = "array"
	KindMultiSelect  Kind = "multiselect"
	KindMap          Kind = "map"
	KindMapBool      Kind = "map-bool"
	KindJSON         Kind = "json"
	KindGroup        Kind = "group"
)

// ScopeClass says which settings files may hold a key.
type ScopeClass string

const (
	ScopeAny              ScopeClass = "any"
	ScopeManaged          ScopeClass = "managed"
	ScopeUserManaged      ScopeClass = "user-managed"
	ScopeUserLocalManaged ScopeClass = "user-local-managed"
	ScopeUser             ScopeClass = "user"
	ScopeGlobal           ScopeClass = "global"
)

// Setting is one settings.json key.
type Setting struct {
	Key         string            `json:"key"`
	Section     string            `json:"section"`
	Scope       string            `json:"scope"`
	ScopeClass  ScopeClass        `json:"scopeClass"`
	ScopeNote   string            `json:"scopeNote"`
	Type        string            `json:"type"`
	Default     string            `json:"default"`
	Desc        string            `json:"desc"`
	Kind        Kind              `json:"kind"`
	Options     []string          `json:"options"`
	OptionHelp  map[string]string `json:"optionHelp"`
	Example     string            `json:"example"`
	Deprecated  string            `json:"deprecated"`
	Removed     bool              `json:"removed"`
	Hint        string            `json:"hint"`
	Suggestions []string          `json:"suggestions"`
	Global      bool              `json:"global"`
}

// EnvVar is one environment variable Claude Code reads.
type EnvVar struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
}

// Schema is the whole catalogue.
type Schema struct {
	// Format is CurrentFormat for catalogues this build can read. The embedded
	// catalogue may predate the field, in which case it is zero.
	Format        int       `json:"format,omitempty"`
	GeneratedFrom string    `json:"generatedFrom"`
	GeneratedOn   string    `json:"generatedOn"`
	Settings      []Setting `json:"settings"`
	EnvVars       []EnvVar  `json:"envVars"`
	// RemovedEnvVars are variables current versions ignore; nothing lists them.
	RemovedEnvVars []EnvVar `json:"removedEnvVars"`
	HookEvents     []string `json:"hookEvents"`
	ModelAliases   []string `json:"modelAliases"`

	// removed holds keys current Claude Code versions ignore. Get still finds
	// them so an old settings file can be cleaned up; nothing lists them.
	removed  []Setting
	byKey    map[string]*Setting
	sections []string
}

var (
	once   sync.Once
	loaded *Schema
	origin Origin
)

// Load returns the catalogue, parsed once: the cached copy written by
// "sync" when it is valid, otherwise the embedded one. Source says which.
func Load() *Schema {
	once.Do(func() {
		path := ""
		if cacheEnabled {
			path, _ = CachePath()
		}
		loaded, origin = load(path, raw, time.Now())
	})
	return loaded
}

// Loaded reports whether Load has run, so a caller can report staleness
// without forcing a parse.
func Loaded() bool { return loaded != nil }

// Parse decodes a catalogue and builds its indexes. Entries marked removed
// are kept out of Settings but still found by Get.
func Parse(b []byte) (*Schema, error) {
	s := &Schema{}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, err
	}
	if len(s.Settings) == 0 {
		return nil, errors.New("catalogue lists no settings")
	}
	all := s.Settings
	s.Settings = make([]Setting, 0, len(all))
	for _, st := range all {
		if st.Removed {
			s.removed = append(s.removed, st)
		} else {
			s.Settings = append(s.Settings, st)
		}
	}
	s.byKey = make(map[string]*Setting, len(all))
	for i := range s.removed {
		s.byKey[s.removed[i].Key] = &s.removed[i]
	}
	seen := map[string]bool{}
	for i := range s.Settings {
		st := &s.Settings[i]
		s.byKey[st.Key] = st
		if !seen[st.Section] {
			seen[st.Section] = true
			s.sections = append(s.sections, st.Section)
		}
	}
	return s, nil
}

// Get returns the setting for key, or nil.
func (s *Schema) Get(key string) *Setting { return s.byKey[key] }

// Sections returns section names in documentation order.
func (s *Schema) Sections() []string { return s.sections }

// BySection returns settings in one section, in documentation order.
func (s *Schema) BySection(section string) []*Setting {
	var out []*Setting
	for i := range s.Settings {
		if s.Settings[i].Section == section {
			out = append(out, &s.Settings[i])
		}
	}
	return out
}

// Search returns the settings that match q, best first: the key's last
// segment, then the full key, section, and words in the description (see
// package match). An empty q returns everything in documentation order.
func (s *Schema) Search(q string) []*Setting {
	rows := make([]match.Fields, len(s.Settings))
	for i := range s.Settings {
		st := &s.Settings[i]
		rows[i] = match.Fields{Name: lastSegment(st.Key), Key: st.Key, Section: st.Section, Desc: st.Desc}
	}
	var out []*Setting
	for _, h := range match.Rank(q, rows) {
		out = append(out, &s.Settings[h.Index])
	}
	return out
}

// lastSegment is the part of a dotted key after the final dot.
func lastSegment(key string) string {
	if i := strings.LastIndex(key, "."); i >= 0 {
		return key[i+1:]
	}
	return key
}

// Keys returns every current key, sorted.
func (s *Schema) Keys() []string {
	out := make([]string, 0, len(s.Settings))
	for i := range s.Settings {
		out = append(out, s.Settings[i].Key)
	}
	sort.Strings(out)
	return out
}

// EnvVar returns the env var doc for name, or nil.
func (s *Schema) EnvVar(name string) *EnvVar {
	for i := range s.EnvVars {
		if s.EnvVars[i].Name == name {
			return &s.EnvVars[i]
		}
	}
	return nil
}

// EnvVarNames returns every documented env var name.
func (s *Schema) EnvVarNames() []string {
	out := make([]string, len(s.EnvVars))
	for i, e := range s.EnvVars {
		out[i] = e.Name
	}
	return out
}

// DocURL is the anchor into the official reference for this key.
func (st *Setting) DocURL() string {
	anchor := strings.ToLower(strings.ReplaceAll(st.Key, ".", ""))
	return "https://code.claude.com/docs/en/settings-reference#" + anchor
}

// Children returns keys that are nested under this one (e.g. sandbox.enabled).
func (s *Schema) Children(key string) []*Setting {
	var out []*Setting
	for i := range s.Settings {
		if strings.HasPrefix(s.Settings[i].Key, key+".") {
			out = append(out, &s.Settings[i])
		}
	}
	return out
}
