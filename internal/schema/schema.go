// Package schema holds the catalogue of every Claude Code setting, generated
// from the official settings reference at
// https://code.claude.com/docs/en/settings-reference.
package schema

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

//go:embed schema.json
var raw []byte

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
	GeneratedFrom string    `json:"generatedFrom"`
	GeneratedOn   string    `json:"generatedOn"`
	Settings      []Setting `json:"settings"`
	EnvVars       []EnvVar  `json:"envVars"`
	HookEvents    []string  `json:"hookEvents"`
	ModelAliases  []string  `json:"modelAliases"`

	byKey    map[string]*Setting
	sections []string
}

var (
	once   sync.Once
	loaded *Schema
)

// Load parses the embedded schema once and returns it.
func Load() *Schema {
	once.Do(func() {
		s := &Schema{}
		if err := json.Unmarshal(raw, s); err != nil {
			panic("schema: embedded schema.json is invalid: " + err.Error())
		}
		s.byKey = make(map[string]*Setting, len(s.Settings))
		seen := map[string]bool{}
		for i := range s.Settings {
			st := &s.Settings[i]
			s.byKey[st.Key] = st
			if !seen[st.Section] {
				seen[st.Section] = true
				s.sections = append(s.sections, st.Section)
			}
		}
		loaded = s
	})
	return loaded
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

// Search returns settings whose key or description contains q (case-insensitive).
func (s *Schema) Search(q string) []*Setting {
	q = strings.ToLower(strings.TrimSpace(q))
	var out []*Setting
	for i := range s.Settings {
		st := &s.Settings[i]
		if q == "" || strings.Contains(strings.ToLower(st.Key), q) || strings.Contains(strings.ToLower(st.Desc), q) {
			out = append(out, st)
		}
	}
	return out
}

// Keys returns every key, sorted.
func (s *Schema) Keys() []string {
	out := make([]string, 0, len(s.byKey))
	for k := range s.byKey {
		out = append(out, k)
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
