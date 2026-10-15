// Package perm keeps the three permission rule lists of one settings file
// consistent with each other.
//
// Claude Code checks deny rules first, then ask, then allow. Two rules
// conflict, within one file, when:
//
//   - the same rule string appears in two lists;
//   - a bare tool name such as "Bash" or "Bash(*)" is denied while any rule for
//     that tool, such as "Bash(npm *)", is allowed or asked: the deny removes the
//     tool, so those rules can never match.
//
// Allowing a whole tool while denying part of it ("Bash" allowed, "Bash(rm *)"
// denied) is the usual "allow all but" pattern and is not a conflict. Ask and
// allow conflict only on identical rules. Rules in different files are left
// alone; Check reports them per file so you can decide.
package perm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jorden-parker/claude-config-cli/internal/store"
)

// The three rule lists, highest precedence first.
const (
	Deny  = "permissions.deny"
	Ask   = "permissions.ask"
	Allow = "permissions.allow"
)

// Lists holds the keys in precedence order.
var Lists = []string{Deny, Ask, Allow}

// IsList reports whether key is one of the permission rule lists.
func IsList(key string) bool { return key == Deny || key == Ask || key == Allow }

// Rule is a parsed permission rule.
type Rule struct {
	Tool string // Bash, Read, mcp__server__tool, …
	Spec string // what is inside the parentheses, or ""
	Bare bool   // "Tool" or "Tool(*)": the rule covers the whole tool
}

// ParseRule splits "Tool(spec)" into its parts.
func ParseRule(s string) Rule {
	s = strings.TrimSpace(s)
	i := strings.IndexByte(s, '(')
	if i < 0 || !strings.HasSuffix(s, ")") {
		return Rule{Tool: s, Bare: true}
	}
	spec := s[i+1 : len(s)-1]
	return Rule{Tool: s[:i], Spec: spec, Bare: spec == "*"}
}

// Rules returns the rule strings stored under key, or an error when the
// value is not an array of strings. A missing key yields nil.
func Rules(f *store.File, key string) ([]string, error) {
	v, ok := f.Get(key)
	if !ok {
		return nil, nil
	}
	switch rules := v.(type) {
	case []string:
		return rules, nil
	case []any:
		out := make([]string, len(rules))
		for i, rule := range rules {
			s, ok := rule.(string)
			if !ok {
				return nil, fmt.Errorf("%s must contain only strings", key)
			}
			out[i] = s
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%s must be an array of strings", key)
	}
}

// Dedupe drops repeated rules, keeping the first occurrence and the order.
func Dedupe(rules []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

// Conflicts reports whether rule a in list keyA clashes with rule b in list
// keyB, per the package definition.
func Conflicts(keyA, a, keyB, b string) bool {
	if keyA == keyB {
		return false
	}
	if a == b {
		return true
	}
	// Only a deny of the whole tool covers the other list's narrower rules.
	var denied, other string
	switch {
	case keyA == Deny:
		denied, other = a, b
	case keyB == Deny:
		denied, other = b, a
	default:
		return false
	}
	d, o := ParseRule(denied), ParseRule(other)
	return d.Bare && d.Tool == o.Tool
}

// Reconcile stores rules under key and removes the rules in the other two
// lists that conflict with them. It reports what it removed, keyed by list,
// and changes nothing when any list is unreadable. It does not save.
func Reconcile(f *store.File, key string, rules []string) (removed map[string][]string, err error) {
	if !IsList(key) {
		return nil, fmt.Errorf("%s is not a permission rule list", key)
	}
	current := map[string][]string{}
	for _, k := range Lists {
		if current[k], err = Rules(f, k); err != nil {
			return nil, err
		}
	}
	rules = Dedupe(rules)
	if err := f.Set(key, rules); err != nil {
		return nil, err
	}
	removed = map[string][]string{}
	for _, other := range Lists {
		if other == key {
			continue
		}
		kept := make([]string, 0, len(current[other]))
		for _, r := range current[other] {
			clash := false
			for _, w := range rules {
				if Conflicts(key, w, other, r) {
					clash = true
					break
				}
			}
			if clash {
				removed[other] = append(removed[other], r)
			} else {
				kept = append(kept, r)
			}
		}
		if len(removed[other]) > 0 {
			if err := f.Set(other, kept); err != nil {
				return nil, err
			}
		}
	}
	return removed, nil
}

// Conflict is one pair of rules in a file that contradict each other.
type Conflict struct {
	Winner, WinnerRule string // the list and rule that take effect
	Loser, LoserRule   string // the list and rule that never do
}

func (c Conflict) String() string {
	short := func(k string) string { return strings.TrimPrefix(k, "permissions.") }
	if c.WinnerRule == c.LoserRule {
		return fmt.Sprintf("%q is in both %s and %s; %s wins", c.WinnerRule, short(c.Winner), short(c.Loser), short(c.Winner))
	}
	return fmt.Sprintf("%s %q makes %s %q dead", short(c.Winner), c.WinnerRule, short(c.Loser), c.LoserRule)
}

// Check lists the conflicts in one file, higher-precedence list first.
func Check(f *store.File) ([]Conflict, error) {
	lists := map[string][]string{}
	for _, k := range Lists {
		var err error
		if lists[k], err = Rules(f, k); err != nil {
			return nil, err
		}
	}
	var out []Conflict
	for i, hi := range Lists {
		for _, lo := range Lists[i+1:] {
			for _, a := range lists[hi] {
				for _, b := range lists[lo] {
					if Conflicts(hi, a, lo, b) {
						out = append(out, Conflict{Winner: hi, WinnerRule: a, Loser: lo, LoserRule: b})
					}
				}
			}
		}
	}
	return out, nil
}

// Describe renders Reconcile's result as one clause, or "" when nothing moved.
func Describe(removed map[string][]string) string {
	var parts []string
	for _, k := range Lists {
		if rules := removed[k]; len(rules) > 0 {
			quoted := make([]string, len(rules))
			for i, r := range rules {
				quoted[i] = fmt.Sprintf("%q", r)
			}
			parts = append(parts, fmt.Sprintf("%s from %s", strings.Join(quoted, ", "), k))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "removed " + strings.Join(parts, " and ")
}

// Sorted returns the keys of removed in precedence order; a helper for callers
// that print per list.
func Sorted(removed map[string][]string) []string {
	var keys []string
	for k := range removed {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return index(keys[i]) < index(keys[j]) })
	return keys
}

func index(k string) int {
	for i, l := range Lists {
		if l == k {
			return i
		}
	}
	return len(Lists)
}
