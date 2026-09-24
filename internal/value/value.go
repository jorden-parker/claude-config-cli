// Package value converts text typed by a person into the JSON value a
// setting expects, and checks it against the documented options.
package value

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
)

// Parse turns text into the JSON value for st.
//
// Text forms accepted per kind:
//   - bool: true/false (also yes/no, on/off, 1/0)
//   - enum: one of the documented options
//   - enum-or-string, string: any text
//   - number: an integer or decimal
//   - array: a JSON array, or one item per line, or comma-separated
//   - multiselect: a JSON array, "true", or one item per line
//   - map: a JSON object or KEY=VALUE lines
//   - map-bool: a JSON object or KEY=true|false lines
//   - json, group: a JSON document
func Parse(st *schema.Setting, text string) (any, error) {
	t := strings.TrimSpace(text)
	switch st.Kind {
	case schema.KindBool:
		switch strings.ToLower(t) {
		case "true", "yes", "on", "1":
			return true, nil
		case "false", "no", "off", "0":
			return false, nil
		}
		return nil, fmt.Errorf("setting %s expects true or false", st.Key)

	case schema.KindEnum:
		t = strings.Trim(t, `"`)
		for _, o := range st.Options {
			if o == t {
				return t, nil
			}
		}
		return nil, fmt.Errorf("setting %s must be one of: %s", st.Key, strings.Join(st.Options, ", "))

	case schema.KindEnumOrString, schema.KindString:
		if t == "" {
			return nil, fmt.Errorf("setting %s needs a value", st.Key)
		}
		return strings.Trim(t, `"`), nil

	case schema.KindNumber:
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return nil, fmt.Errorf("setting %s expects a number", st.Key)
		}
		if f == math.Trunc(f) {
			return int64(f), nil
		}
		return f, nil

	case schema.KindArray:
		return parseArray(t)

	case schema.KindMultiSelect:
		if strings.EqualFold(t, "true") {
			return true, nil
		}
		arr, err := parseArray(t)
		if err != nil {
			return nil, err
		}
		for _, item := range arr {
			ok := false
			for _, o := range st.Options {
				if o == item {
					ok = true
				}
			}
			if !ok {
				return nil, fmt.Errorf("%s: %q is not one of %s", st.Key, item, strings.Join(st.Options, ", "))
			}
		}
		return arr, nil

	case schema.KindMap, schema.KindMapBool:
		if strings.HasPrefix(t, "{") {
			var m map[string]any
			if err := json.Unmarshal([]byte(t), &m); err != nil {
				return nil, fmt.Errorf("%s: invalid JSON object: %w", st.Key, err)
			}
			return m, nil
		}
		m := map[string]any{}
		for _, line := range strings.Split(t, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				return nil, fmt.Errorf("%s: line %q is not KEY=VALUE", st.Key, line)
			}
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			if st.Kind == schema.KindMapBool {
				b, err := strconv.ParseBool(v)
				if err != nil {
					return nil, fmt.Errorf("%s: %s must be true or false", st.Key, k)
				}
				m[k] = b
			} else {
				m[k] = v
			}
		}
		return m, nil

	default: // json, group
		if t == "" {
			return nil, fmt.Errorf("setting %s needs a JSON value", st.Key)
		}
		var v any
		if err := json.Unmarshal([]byte(t), &v); err != nil {
			return nil, fmt.Errorf("%s: invalid JSON: %w", st.Key, err)
		}
		return v, nil
	}
}

func parseArray(t string) ([]string, error) {
	if strings.HasPrefix(t, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(t), &arr); err != nil {
			return nil, fmt.Errorf("invalid JSON array of strings: %w", err)
		}
		return arr, nil
	}
	sep := "\n"
	if !strings.Contains(t, "\n") {
		sep = ","
	}
	var out []string
	for _, p := range strings.Split(t, sep) {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}

// Text renders a stored value back into the edit-friendly text form Parse accepts.
func Text(st *schema.Setting, v any) string {
	if v == nil {
		return ""
	}
	switch st.Kind {
	case schema.KindArray, schema.KindMultiSelect:
		if arr, ok := v.([]any); ok {
			var lines []string
			for _, a := range arr {
				lines = append(lines, fmt.Sprint(a))
			}
			return strings.Join(lines, "\n")
		}
	case schema.KindMap, schema.KindMapBool:
		if m, ok := v.(map[string]any); ok {
			var lines []string
			for k, val := range m {
				lines = append(lines, fmt.Sprintf("%s=%v", k, val))
			}
			return strings.Join(lines, "\n")
		}
	case schema.KindBool, schema.KindNumber, schema.KindString, schema.KindEnum, schema.KindEnumOrString:
		return fmt.Sprint(v)
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
