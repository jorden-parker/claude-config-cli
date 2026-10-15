package perm

import (
	"reflect"
	"testing"

	"github.com/jorden-parker/claude-config-cli/internal/store"
)

func file(perms map[string]any) *store.File {
	return &store.File{Scope: store.ScopeUser, Data: map[string]any{"model": "opus", "permissions": perms}}
}

func TestParseRule(t *testing.T) {
	cases := map[string]Rule{
		"Bash":                   {Tool: "Bash", Bare: true},
		"Bash(*)":                {Tool: "Bash", Spec: "*", Bare: true},
		"Bash(npm *)":            {Tool: "Bash", Spec: "npm *"},
		"Read(./.env)":           {Tool: "Read", Spec: "./.env"},
		"mcp__x__y":              {Tool: "mcp__x__y", Bare: true},
		"Bash(a(b)":              {Tool: "Bash", Spec: "a(b"},
		" WebFetch(domain:x.y) ": {Tool: "WebFetch", Spec: "domain:x.y"},
	}
	for in, want := range cases {
		if got := ParseRule(in); got != want {
			t.Errorf("ParseRule(%q) = %+v, want %+v", in, got, want)
		}
	}
}

func TestConflicts(t *testing.T) {
	cases := []struct {
		ka, a, kb, b string
		want         bool
	}{
		{Deny, "Bash", Allow, "Bash", true},
		{Allow, "Bash", Deny, "Bash", true},
		{Deny, "Bash", Allow, "Bash(npm *)", true},
		{Deny, "Bash(*)", Ask, "Bash(npm *)", true},
		{Allow, "Bash(npm *)", Deny, "Bash", true},
		{Deny, "Bash(rm *)", Allow, "Bash", false},
		{Deny, "Bash(rm *)", Allow, "Bash(npm *)", false},
		{Deny, "Bash", Allow, "Read", false},
		{Ask, "Bash", Allow, "Bash(npm *)", false},
		{Ask, "Bash(npm *)", Allow, "Bash(npm *)", true},
		{Deny, "Bash", Deny, "Bash", false},
	}
	for _, c := range cases {
		if got := Conflicts(c.ka, c.a, c.kb, c.b); got != c.want {
			t.Errorf("Conflicts(%s %q, %s %q) = %v", c.ka, c.a, c.kb, c.b, got)
		}
	}
}

func TestReconcileRemovesConflicts(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		rules   []string
		want    map[string][]any
		removed map[string][]string
	}{
		{
			name: "deny whole tool",
			key:  Deny, rules: []string{"Bash", "Bash"},
			want:    map[string][]any{"deny": {"Bash"}, "allow": {"Read"}, "ask": {"Edit(*.go)"}},
			removed: map[string][]string{Allow: {"Bash", "Bash(npm *)"}, Ask: {"Bash(rm *)"}},
		},
		{
			name: "allow narrower rule revives it",
			key:  Allow, rules: []string{"Bash(npm *)", "Read"},
			want:    map[string][]any{"deny": {"Edit(*.md)"}, "allow": {"Bash(npm *)", "Read"}, "ask": {"Bash(rm *)", "Edit(*.go)"}},
			removed: map[string][]string{Deny: {"Bash"}},
		},
		{
			name: "ask exact only against allow; a whole-tool deny still yields",
			key:  Ask, rules: []string{"Read", "Bash(rm *)"},
			want:    map[string][]any{"deny": {"Edit(*.md)"}, "allow": {"Bash", "Bash(npm *)"}, "ask": {"Read", "Bash(rm *)"}},
			removed: map[string][]string{Deny: {"Bash"}, Allow: {"Read"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := file(map[string]any{
				"deny":        []any{"Bash", "Edit(*.md)"},
				"allow":       []any{"Bash", "Bash(npm *)", "Read"},
				"ask":         []any{"Bash(rm *)", "Edit(*.go)"},
				"defaultMode": "plan",
			})
			removed, err := Reconcile(f, c.key, c.rules)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(removed, c.removed) {
				t.Fatalf("removed = %v, want %v", removed, c.removed)
			}
			perms := f.Data["permissions"].(map[string]any)
			for list, want := range c.want {
				got := perms[list]
				if got, ok := got.([]string); ok {
					var anys []any
					for _, g := range got {
						anys = append(anys, g)
					}
					if !reflect.DeepEqual(anys, want) {
						t.Errorf("%s = %v, want %v", list, got, want)
					}
					continue
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s = %v, want %v", list, got, want)
				}
			}
			if perms["defaultMode"] != "plan" || f.Data["model"] != "opus" {
				t.Fatal("other keys changed")
			}
		})
	}
}

func TestReconcileEmptiesButKeepsList(t *testing.T) {
	f := file(map[string]any{"allow": []any{"Bash"}})
	if _, err := Reconcile(f, Deny, []string{"Bash"}); err != nil {
		t.Fatal(err)
	}
	v, ok := f.Get(Allow)
	if !ok || len(v.([]string)) != 0 {
		t.Fatalf("allow = %v %v", v, ok)
	}
}

func TestReconcileRejectsInvalidListsWithoutMutating(t *testing.T) {
	for _, bad := range []map[string]any{
		{"ask": []any{42}, "allow": []any{"Bash"}},
		{"allow": "Bash"},
	} {
		f := file(bad)
		before := store.Format(f.Data)
		if _, err := Reconcile(f, Deny, []string{"Bash"}); err == nil {
			t.Fatal("expected error")
		}
		if store.Format(f.Data) != before {
			t.Fatal("data changed on error")
		}
	}
	if _, err := Reconcile(file(nil), "model", nil); err == nil {
		t.Fatal("non-list key accepted")
	}
}

func TestCheckAndDescribe(t *testing.T) {
	f := file(map[string]any{
		"deny":  []any{"Bash", "Read(.env)"},
		"ask":   []any{"Bash(rm *)", "Read(.env)", "Edit"},
		"allow": []any{"Bash(npm *)", "Edit", "Read(.env)"},
	})
	got, err := Check(f)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, c := range got {
		lines = append(lines, c.String())
	}
	want := []string{
		`deny "Bash" makes ask "Bash(rm *)" dead`,
		`"Read(.env)" is in both deny and ask; deny wins`,
		`deny "Bash" makes allow "Bash(npm *)" dead`,
		`"Read(.env)" is in both deny and allow; deny wins`,
		`"Read(.env)" is in both ask and allow; ask wins`,
		`"Edit" is in both ask and allow; ask wins`,
	}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("got\n%v\nwant\n%v", lines, want)
	}
	if _, err := Check(file(map[string]any{"deny": 1})); err == nil {
		t.Fatal("expected error")
	}
	if d := Describe(nil); d != "" {
		t.Fatal(d)
	}
	d := Describe(map[string][]string{Allow: {"Bash", "Bash(*)"}, Deny: {"Read"}})
	if d != `removed "Read" from permissions.deny and "Bash", "Bash(*)" from permissions.allow` {
		t.Fatal(d)
	}
	if !reflect.DeepEqual(Dedupe([]string{"a", "b", "a"}), []string{"a", "b"}) {
		t.Fatal("dedupe")
	}
}
