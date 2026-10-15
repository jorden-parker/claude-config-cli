package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
)

// entryMD renders one reference entry with the given list marker.
func entryMD(mark, key, typ, scope string, opts ...string) string {
	var b strings.Builder
	b.WriteString("### `" + key + "`\n\nDescribes " + key + " in [docs](/x) with **bold**.\nSecond line.\n\nSecond paragraph is ignored.\n\n")
	b.WriteString(mark + " **Scope**: [`" + scope + "`](#scopes)\n")
	b.WriteString(mark + " **Type**: " + typ + "\n")
	for _, o := range opts {
		b.WriteString("  " + mark + " " + o + "\n")
	}
	b.WriteString(mark + " **Default**: `true`\n\n```json\n{\"" + key + "\": true}\n```\n\n```json\n{\"second\": 1}\n```\n\n")
	return b.String()
}

func refDoc(entries ...string) string {
	return "# All settings\n\nindex\n\n## Model and responses\n\n" + strings.Join(entries, "")
}

func fixtures(t *testing.T) Inputs {
	t.Helper()
	in, err := ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func build(t *testing.T, ref string) []schema.Setting {
	t.Helper()
	in := fixtures(t)
	in.Ref = []byte(ref)
	s, err := Build(in, Options{Today: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	return s.Settings
}

func TestExtractAcceptsBulletMarkers(t *testing.T) {
	var want []entry
	for _, mark := range []string{"*", "-", "+"} {
		got, err := extract(refDoc(entryMD(mark, "editorMode", "string, one of:", "Any file", "`\"normal\"`: plain", "`\"vim\"`: modal")))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("%s: %d entries", mark, len(got))
		}
		got[0].body = nil
		if want == nil {
			want = got
			e := got[0]
			if e.Scope != "Any file" || e.Type != "string, one of:" || e.Default != "`true`" || e.Section != "Model and responses" {
				t.Fatalf("%+v", e)
			}
			if e.Desc != "Describes editorMode in [docs](/x) with **bold**. Second line." {
				t.Fatalf("desc = %q", e.Desc)
			}
			if !reflect.DeepEqual(e.Options, []option{{"\"normal\"", "plain"}, {"\"vim\"", "modal"}}) {
				t.Fatalf("options = %+v", e.Options)
			}
			if e.Example != "{\"editorMode\": true}" {
				t.Fatalf("example = %q", e.Example)
			}
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("marker %s differs: %+v vs %+v", mark, got, want)
		}
	}
}

func TestExtractDescriptionSkipsWarningAndNote(t *testing.T) {
	src := refDoc("### `voiceEnabled`\n\n<Warning>\n  Deprecated since v2.1.92: use [`voice`](#voice) instead.\n</Warning>\n\nThe real description.\n\n* **Scope**: [`Any file`](#scopes)\n* **Type**: Boolean\n")
	got, err := extract(src)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Desc != "The real description." {
		t.Fatalf("desc = %q", got[0].Desc)
	}
	if d := got[0].deprecated(); d != "Deprecated since v2.1.92: use `voice` instead." {
		t.Fatalf("deprecated = %q", d)
	}
	if removed(got[0].deprecated()) {
		t.Fatal("deprecated is not removed")
	}
	if !removed("Removed in v2.1.277. Use x.") || !removed("This key has no effect since v2") {
		t.Fatal("removed detection")
	}
}

func TestExtractNeedsStartHeading(t *testing.T) {
	if _, err := extract("# nothing\n"); err == nil {
		t.Fatal("expected error")
	}
}

func TestClassifyContainersBeforeEnums(t *testing.T) {
	cases := []struct {
		typ  string
		opts []string
		kind schema.Kind
		want []string
	}{
		{"object with `verbs` and `mode`", []string{"`\"append\"`: x", "`\"replace\"`: y"}, schema.KindJSON, nil},
		{"object with `enabled`, `mode`", []string{"`\"hold\"`", "`\"tap\"`"}, schema.KindJSON, nil},
		{"array of strings, each one of:", []string{"`\"anthropic\"`", "`\"bedrock\"`"}, schema.KindArray, nil},
		{"array of objects, each with `id`", nil, schema.KindJSON, nil},
		{"array of marketplace source objects", nil, schema.KindJSON, nil},
		{"Boolean; only `true` takes effect", nil, schema.KindBool, nil},
		{"the string `\"disable\"`", nil, schema.KindEnum, []string{"disable"}},
		{"string, one of:", []string{"`\"low\"`: a", "`\"high\"`: b"}, schema.KindEnum, []string{"low", "high"}},
		{"string, one of `\"5m\"` or `\"1h\"`", nil, schema.KindEnum, []string{"5m", "1h"}},
		{"string, `\"notify\"`, `\"quiet\"`, or `\"notify\"`", nil, schema.KindEnum, []string{"notify", "quiet"}},
		{"number of tokens", nil, schema.KindNumber, nil},
		{"integer, milliseconds", nil, schema.KindNumber, nil},
		{"string, a shell command", nil, schema.KindString, nil},
	}
	for _, c := range cases {
		e := entry{Type: c.typ}
		for _, o := range c.opts {
			m := optionRE.FindStringSubmatch(o)
			e.Options = append(e.Options, option{Value: m[1], Help: m[2]})
		}
		kind, opts := classify(e)
		if kind != c.kind || !reflect.DeepEqual(opts, c.want) {
			t.Errorf("%q: got %s %v, want %s %v", c.typ, kind, opts, c.kind, c.want)
		}
	}
}

func TestScopeClasses(t *testing.T) {
	cases := map[string]struct {
		class      schema.ScopeClass
		head, note string
	}{
		"Any file": {schema.ScopeAny, "Any file", ""},
		"Any file. A `true` in any file applies.":             {schema.ScopeAny, "Any file", "A `true` in any file applies."},
		"Any file, with limits on project and local settings": {schema.ScopeAny, "Any file, with limits on project and local settings", ""},
		"Managed. Claude Code ignores the key elsewhere.":     {schema.ScopeManaged, "Managed", "Claude Code ignores the key elsewhere."},
		"User or managed": {schema.ScopeUserManaged, "User or managed", ""},
		"User, local, or managed, and files passed with `--settings`": {schema.ScopeUserLocalManaged, "User, local, or managed, and files passed with `--settings`", ""},
		"Global config": {schema.ScopeGlobal, "Global config", ""},
		"User, in `~/.claude/settings.json` on the remote machine": {schema.ScopeUser, "User, in `~/.claude/settings.json` on the remote machine", ""},
	}
	for in, want := range cases {
		class, head, note, err := scopeClass(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if class != want.class || head != want.head || note != want.note {
			t.Errorf("%q: got %s %q %q", in, class, head, note)
		}
	}
	if _, _, _, err := scopeClass("Usermanaged"); err == nil {
		t.Fatal("prefix must end at a word boundary")
	}
	if _, _, _, err := scopeClass("Project only"); err == nil || !strings.Contains(err.Error(), "unknown scope prefix") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildValidation(t *testing.T) {
	cases := map[string]string{
		"duplicate setting key":     refDoc(entryMD("*", "model", "string", "Any file"), entryMD("*", "model", "string", "Any file")),
		"is missing type":           refDoc("### `model`\n\nText.\n\n* **Scope**: [`Any file`](#scopes)\n"),
		"is missing scope":          refDoc("### `model`\n\nText.\n\n* **Type**: string\n"),
		"not documented as removed": refDoc("### `model`\n\nText.\n"),
		"unknown scope prefix":      refDoc(entryMD("*", "model", "string", "Project")),
		"no settings entries":       refDoc(),
	}
	for want, ref := range cases {
		in := fixtures(t)
		in.Ref = []byte(ref)
		_, err := Build(in, Options{})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v", want, err)
		}
	}
}

func TestRemovedExceptionIsExplicit(t *testing.T) {
	ref := refDoc("### `taskOutputMaxChars`\n\n<Warning>\n  Removed in v2.1.277. Output is no longer truncated.\n</Warning>\n\nOld text.\n")
	got := build(t, ref)
	if len(got) != 1 || !got[0].Removed || got[0].Kind != schema.KindNumber || got[0].ScopeClass != schema.ScopeAny {
		t.Fatalf("%+v", got)
	}
	// The same shape under another key is an error.
	in := fixtures(t)
	in.Ref = []byte(strings.Replace(ref, "taskOutputMaxChars", "otherKey", 1))
	if _, err := Build(in, Options{}); err == nil {
		t.Fatal("removed exception leaked to another key")
	}
}

func TestBuildAppliesOverridesGroupsAndSkips(t *testing.T) {
	ref := refDoc(
		entryMD("*", "model", "string, a model alias", "Any file"),
		entryMD("*", "sandbox", "object with `enabled`", "Any file"),
		entryMD("*", "sandbox.enabled", "Boolean", "Any file"),
		entryMD("*", "strictPluginOnlyCustomization", "`true` or array", "Managed"),
		entryMD("*", "strictPluginOnlyCustomization.skills", "the string `\"skills\"`", "Managed"),
		entryMD("*", "autoConnectIde", "Boolean", "Global config"),
	) + "\n## Global config settings\n\n" + entryMD("*", "copyOnSelect", "Boolean", "Global config")
	got := build(t, ref)
	by := map[string]schema.Setting{}
	for _, st := range got {
		by[st.Key] = st
	}
	if _, ok := by["strictPluginOnlyCustomization.skills"]; ok {
		t.Fatal("skipped key present")
	}
	if st := by["model"]; st.Kind != schema.KindEnumOrString || !reflect.DeepEqual(st.Options, modelAliases) || st.Hint == "" {
		t.Fatalf("model override: %+v", st)
	}
	if st := by["sandbox"]; st.Kind != schema.KindGroup {
		t.Fatalf("sandbox kind = %s", st.Kind)
	}
	if st := by["strictPluginOnlyCustomization"]; st.Kind != schema.KindMultiSelect || st.ScopeClass != schema.ScopeManaged {
		t.Fatalf("%+v", st)
	}
	if !by["copyOnSelect"].Global || by["autoConnectIde"].Global {
		t.Fatal("global flag follows the section, not the scope")
	}
	for _, st := range got {
		if st.Options == nil || st.Suggestions == nil || st.OptionHelp == nil {
			t.Fatalf("%s has nil collections", st.Key)
		}
	}
}

func TestParseEnvTable(t *testing.T) {
	in := fixtures(t)
	env, dropped, err := parseEnv(string(in.Env))
	if err != nil {
		t.Fatal(err)
	}
	want := []schema.EnvVar{
		{Name: "ANTHROPIC_API_KEY", Purpose: "API key sent as the `X-Api-Key` header. See docs."},
		{Name: "CLAUDE_CODE_DISABLE_THINKING", Purpose: "Turns thinking off."},
		{Name: "BASH_DEFAULT_TIMEOUT_MS", Purpose: "Default timeout for Bash commands."},
	}
	if !reflect.DeepEqual(env, want) {
		t.Fatalf("env = %+v", env)
	}
	if len(dropped) != 1 || dropped[0].Name != "OLD_VAR" {
		t.Fatalf("dropped = %+v", dropped)
	}
	bad := map[string]string{
		"no \"## Variables\" section":  "# page\n\n| Variable | Purpose |\n",
		"no Variable/Purpose header":   "## Variables\n\n| Name | Purpose |\n| - | - |\n| `A` | b |\n",
		"malformed env-vars table row": "## Variables\n\n| Variable | Purpose |\n| - | - |\n| oops |\n",
		"duplicate env var":            "## Variables\n\n| Variable | Purpose |\n| - | - |\n| `A` | b |\n| `A` | c |\n",
		"no variables":                 "## Variables\n\n| Variable | Purpose |\n| - | - |\n| `A` | Removed in v2.0. |\n",
	}
	for want, text := range bad {
		if _, _, err := parseEnv(text); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v", want, err)
		}
	}
}

func TestParseEnvAcceptsPaddedTable(t *testing.T) {
	text := "## Variables\n\n| Variable        | Purpose      |\n| :-------------- | :----------- |\n| `A_B`           | Does a.      |\n"
	env, _, err := parseEnv(text)
	if err != nil || len(env) != 1 || env[0].Purpose != "Does a." {
		t.Fatalf("%v %+v", err, env)
	}
}

func TestParseToolsTableAndCheck(t *testing.T) {
	in := fixtures(t)
	names, err := parseToolsTable(string(in.Tools))
	if err != nil {
		t.Fatal(err)
	}
	if names["SubagentHandback"] || names["TaskOutput"] || !names["Bash"] {
		t.Fatalf("names = %v", names)
	}
	var listed []string
	for n := range names {
		listed = append(listed, n)
	}
	if err := CheckTools(in.Tools, listed); err != nil {
		t.Fatal(err)
	}
	err = CheckTools(in.Tools, append(listed[1:], "Bogus"))
	if err == nil || !strings.Contains(err.Error(), "add ["+listed[0]+"]") || !strings.Contains(err.Error(), "remove [Bogus]") {
		t.Fatalf("err = %v", err)
	}
	bad := map[string]string{
		"no Tool/Description table": "# x\n",
		"no separator row":          "| Tool | Description |\nprose\n",
		"malformed tools table row": "| Tool | Description |\n| - | - |\n| nope | x |\n",
		"duplicate tool":            "| Tool | Description |\n| - | - |\n| `A` | x |\n| `A` | y |\n",
	}
	for want, text := range bad {
		if _, err := parseToolsTable(text); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v", want, err)
		}
	}
}

func TestDiff(t *testing.T) {
	mk := func(keys ...string) *schema.Schema {
		s := &schema.Schema{Format: 1, GeneratedOn: "2026-01-01", GeneratedFrom: GeneratedFrom, HookEvents: hookEvents, EnvVars: []schema.EnvVar{{Name: "A", Purpose: "a"}}}
		for _, k := range keys {
			s.Settings = append(s.Settings, schema.Setting{Key: k, Kind: schema.KindBool, Desc: "d"})
		}
		return s
	}
	a, b := mk("x", "y"), mk("x", "y")
	b.GeneratedOn, b.Format = "2027-01-01", 0
	if d := Diff(a, b); len(d) != 0 {
		t.Fatalf("date/format must be ignored: %v", d)
	}
	b = mk("y", "x")
	if d := Diff(a, b); !reflect.DeepEqual(d, []string{"settings: order"}) {
		t.Fatalf("%v", d)
	}
	b = mk("x", "z")
	b.Settings[0].Desc = "changed"
	b.Settings[0].Kind = schema.KindString
	b.EnvVars = append(b.EnvVars, schema.EnvVar{Name: "B", Purpose: "b"})
	b.HookEvents = []string{"Stop"}
	want := []string{"envVars[B]: added", "hookEvents", "settings[x]: desc, kind", "settings[y]: removed", "settings[z]: added"}
	if d := Diff(a, b); !reflect.DeepEqual(d, want) {
		t.Fatalf("got %v\nwant %v", d, want)
	}
}

func TestFetchHonoursBaseAndTimeout(t *testing.T) {
	in := fixtures(t)
	pages := map[string][]byte{Pages[0]: in.Ref, Pages[1]: in.Env, Pages[2]: in.Tools}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("slow") != "" {
			time.Sleep(200 * time.Millisecond)
		}
		b, ok := pages[strings.TrimPrefix(r.URL.Path, "/docs/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	defer srv.Close()
	got, err := Fetch(context.Background(), srv.Client(), srv.URL+"/docs/")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatal("pages differ")
	}
	if _, err := Fetch(context.Background(), srv.Client(), srv.URL+"/nope/"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v", err)
	}
	fast := &http.Client{Timeout: 50 * time.Millisecond}
	if _, err := Fetch(context.Background(), fast, srv.URL+"/docs/?slow=1&p="); err == nil {
		t.Fatal("expected timeout")
	}
}

func TestEncodeRoundTripsThroughSchemaParse(t *testing.T) {
	s, err := Build(fixtures(t), Options{Today: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "{\n  \"format\": 1,\n") || !strings.HasSuffix(string(b), "}\n") {
		t.Fatalf("encoding: %.40q", b)
	}
	p, err := schema.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if p.GeneratedOn != "2026-10-15" || p.Format != schema.CurrentFormat {
		t.Fatalf("%+v", p)
	}
	d, err := Decode(b)
	if err != nil || len(d.Settings) != len(s.Settings) {
		t.Fatal(err, len(d.Settings))
	}
	if diff := Diff(s, d); len(diff) != 0 {
		t.Fatal(diff)
	}
}

func TestReadDirAcceptsPageNames(t *testing.T) {
	dir := t.TempDir()
	in := fixtures(t)
	for i, b := range [][]byte{in.Ref, in.Env, in.Tools} {
		os.WriteFile(filepath.Join(dir, Pages[i]), b, 0o600)
	}
	got, err := ReadDir(dir)
	if err != nil || !reflect.DeepEqual(got, in) {
		t.Fatal(err)
	}
	if _, err := ReadDir(t.TempDir()); err == nil {
		t.Fatal("expected error")
	}
}

// The checked-in reference is the 2026-10-07 docs after Prettier rewrote its
// bullets to "-". Building from it must agree with the embedded catalogue
// on everything but wording, which the live regeneration may have changed.
func TestBuildFromCheckedInReferenceMatchesEmbedded(t *testing.T) {
	s, err := Build(fixtures(t), Options{Today: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Settings) < 200 {
		t.Fatalf("%d settings", len(s.Settings))
	}
	emb, err := schema.Parse(schema.Embedded())
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range s.Settings {
		if st.Type == "" && !st.Removed {
			t.Errorf("%s has no type", st.Key)
		}
	}
	for _, key := range []string{"advisorModel", "effortLevel", "model", "enabledPlugins", "strictPluginOnlyCustomization", "permissions.defaultMode", "hooks", "taskOutputMaxChars", "sandbox", "voice", "spinnerVerbs", "skillOverrides", "sandbox.credentials.sigv4", "allowedProviders", "env", "theme", "useAutoModeDuringPlan", "copyOnSelect"} {
		var got *schema.Setting
		for i := range s.Settings {
			if s.Settings[i].Key == key {
				got = &s.Settings[i]
			}
		}
		want := emb.Get(key)
		if got == nil || want == nil {
			t.Errorf("%s: built %v embedded %v", key, got != nil, want != nil)
			continue
		}
		if got.Kind != want.Kind || !reflect.DeepEqual(got.Options, want.Options) || got.ScopeClass != want.ScopeClass || got.Global != want.Global || got.Removed != want.Removed || got.Section != want.Section {
			t.Errorf("%s: built kind=%s opts=%v class=%s; embedded kind=%s opts=%v class=%s", key, got.Kind, got.Options, got.ScopeClass, want.Kind, want.Options, want.ScopeClass)
		}
	}
	again, _ := Build(fixtures(t), Options{Today: time.Now()})
	if !reflect.DeepEqual(s, again) {
		t.Fatal("build is not stable")
	}
}
