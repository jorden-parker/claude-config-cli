package catalog

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
)

// Diff lists what changed between two catalogues, ignoring the build date
// and format. Settings are compared by key and field; other sections by
// name. Empty means no semantic change.
func Diff(old, cur *schema.Schema) []string {
	a, b := generic(old), generic(cur)
	for _, k := range []string{"generatedOn", "format"} {
		delete(a, k)
		delete(b, k)
	}
	var out []string
	for _, k := range union(a, b) {
		if reflect.DeepEqual(a[k], b[k]) {
			continue
		}
		switch k {
		case "settings":
			out = append(out, diffSettings(a[k], b[k])...)
		case "envVars", "removedEnvVars":
			out = append(out, diffNamed(k, a[k], b[k])...)
		default:
			out = append(out, k)
		}
	}
	return out
}

func generic(s *schema.Schema) map[string]any {
	m := map[string]any{}
	if s == nil {
		return m
	}
	b, _ := json.Marshal(s)
	_ = json.Unmarshal(b, &m)
	return m
}

func union(a, b map[string]any) []string {
	seen := map[string]bool{}
	for k := range a {
		seen[k] = true
	}
	for k := range b {
		seen[k] = true
	}
	return sortedKeys(seen)
}

func byKey(v any, field string) (map[string]map[string]any, []string) {
	out := map[string]map[string]any{}
	var order []string
	list, _ := v.([]any)
	for _, item := range list {
		m, _ := item.(map[string]any)
		k, _ := m[field].(string)
		out[k] = m
		order = append(order, k)
	}
	return out, order
}

func diffSettings(a, b any) []string {
	sa, oa := byKey(a, "key")
	sb, ob := byKey(b, "key")
	var out []string
	keys := map[string]bool{}
	for k := range sa {
		keys[k] = true
	}
	for k := range sb {
		keys[k] = true
	}
	for _, k := range sortedKeys(keys) {
		x, okA := sa[k]
		y, okB := sb[k]
		switch {
		case !okA:
			out = append(out, fmt.Sprintf("settings[%s]: added", k))
		case !okB:
			out = append(out, fmt.Sprintf("settings[%s]: removed", k))
		case !reflect.DeepEqual(x, y):
			fields := map[string]bool{}
			for f := range x {
				fields[f] = true
			}
			for f := range y {
				fields[f] = true
			}
			var changed []string
			for _, f := range sortedKeys(fields) {
				if !reflect.DeepEqual(x[f], y[f]) {
					changed = append(changed, f)
				}
			}
			out = append(out, fmt.Sprintf("settings[%s]: %s", k, strings.Join(changed, ", ")))
		}
	}
	if len(sa) == len(sb) && len(out) == 0 && !reflect.DeepEqual(oa, ob) {
		out = append(out, "settings: order")
	}
	sort.Strings(out)
	return out
}

func diffNamed(section string, a, b any) []string {
	sa, _ := byKey(a, "name")
	sb, _ := byKey(b, "name")
	var out []string
	keys := map[string]bool{}
	for k := range sa {
		keys[k] = true
	}
	for k := range sb {
		keys[k] = true
	}
	for _, k := range sortedKeys(keys) {
		switch {
		case sa[k] == nil:
			out = append(out, fmt.Sprintf("%s[%s]: added", section, k))
		case sb[k] == nil:
			out = append(out, fmt.Sprintf("%s[%s]: removed", section, k))
		case !reflect.DeepEqual(sa[k], sb[k]):
			out = append(out, fmt.Sprintf("%s[%s]: purpose", section, k))
		}
	}
	return out
}
