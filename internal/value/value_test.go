package value

import (
	"reflect"
	"testing"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
)

func get(t *testing.T, key string) *schema.Setting {
	t.Helper()
	st := schema.Load().Get(key)
	if st == nil {
		t.Fatalf("schema has no key %s", key)
	}
	return st
}

func TestParseKinds(t *testing.T) {
	cases := []struct {
		key, in string
		want    any
		wantErr bool
	}{
		{"fastMode", "true", true, false},
		{"fastMode", "maybe", nil, true},
		{"effortLevel", "high", "high", false},
		{"effortLevel", "turbo", nil, true},
		{"theme", "custom:solarized", "custom:solarized", false},
		{"autoCompactWindow", "200000", int64(200000), false},
		{"feedbackSurveyRate", "0.5", 0.5, false},
		{"autoCompactWindow", "lots", nil, true},
		{"permissions.allow", "Bash(npm run *),Read(./.env)", []string{"Bash(npm run *)", "Read(./.env)"}, false},
		{"permissions.allow", `["Bash"]`, []string{"Bash"}, false},
		{"strictPluginOnlyCustomization", "true", true, false},
		{"strictPluginOnlyCustomization", "skills,mcp", []string{"skills", "mcp"}, false},
		{"strictPluginOnlyCustomization", "plugins", nil, true},
		{"env", "FOO=bar\nBAZ=1", map[string]any{"FOO": "bar", "BAZ": "1"}, false},
		{"enabledPlugins", "x@y=true", map[string]any{"x@y": true}, false},
		{"statusLine", `{"type":"command","command":"echo hi"}`, map[string]any{"type": "command", "command": "echo hi"}, false},
		{"statusLine", `{bad`, nil, true},
	}
	for _, c := range cases {
		got, err := Parse(get(t, c.key), c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s %q: expected error, got %v", c.key, c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s %q: %v", c.key, c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %q: got %#v want %#v", c.key, c.in, got, c.want)
		}
	}
}

func TestSchemaCoverage(t *testing.T) {
	s := schema.Load()
	if len(s.Settings) < 200 {
		t.Fatalf("expected 200+ settings, got %d", len(s.Settings))
	}
	for _, st := range s.Settings {
		if st.Kind == schema.KindEnum && len(st.Options) == 0 {
			t.Errorf("%s is an enum with no options", st.Key)
		}
		if st.Section == "" || st.Desc == "" {
			t.Errorf("%s is missing section or description", st.Key)
		}
	}
	if len(s.EnvVars) < 300 {
		t.Errorf("expected 300+ env vars, got %d", len(s.EnvVars))
	}
}
