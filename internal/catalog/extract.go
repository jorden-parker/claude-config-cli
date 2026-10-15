// Package catalog builds the settings catalogue from the official Claude Code
// docs: the settings reference, the environment variables page, and the
// tools reference. It is a port of the former Python generator and works on
// bytes only, so the same code serves "ccfg sync" at runtime and regenerates
// the embedded schema.json.
package catalog

import (
	"errors"
	"regexp"
	"strings"
)

// entry is one "### `key`" block of the settings reference.
type entry struct {
	Key, Section, Desc, Scope, Type, Default, Example string
	Options                                           []option
	body                                              []string
}

type option struct{ Value, Help string }

const startHeading = "## Model and responses"

// Markdown list markers: the docs use "*", Prettier rewrites to "-".
const marker = `[-*+]`

var (
	scopeRE   = regexp.MustCompile(`^` + marker + ` \*\*Scope\*\*: (.*)`)
	typeRE    = regexp.MustCompile(`^` + marker + ` \*\*Type\*\*: (.*)`)
	defaultRE = regexp.MustCompile(`^` + marker + ` \*\*Default\*\*: (.*)`)
	nestedRE  = regexp.MustCompile(`^ +` + marker + ` (.*)`)
	optionRE  = regexp.MustCompile("^`([^`]+)`:?\\s*(.*)")
	listRE    = regexp.MustCompile(`^\s*` + marker + `\s`)
	headingRE = regexp.MustCompile("^### `([^`]+)`")
	scopeLink = regexp.MustCompile("\\[`([^`]+)`\\]\\([^)]*\\)")
)

// extract returns every setting entry in the reference, in document order.
func extract(src string) ([]entry, error) {
	lines := strings.Split(src, "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, startHeading) {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, errors.New(`reference has no "` + startHeading + `" heading`)
	}
	var entries []entry
	var cur *entry
	section := ""
	flush := func() {
		if cur != nil {
			entries = append(entries, *cur)
			cur = nil
		}
	}
	for _, l := range lines[start:] {
		switch {
		case strings.HasPrefix(l, "## "):
			flush()
			section = strings.TrimSpace(l[3:])
		case strings.HasPrefix(l, "### `"):
			flush()
			m := headingRE.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			cur = &entry{Key: m[1], Section: section}
		case cur != nil:
			cur.body = append(cur.body, l)
		}
	}
	flush()
	for i := range entries {
		fill(&entries[i])
	}
	return entries, nil
}

// fill reads the description paragraph, the Scope/Type/Default bullets, the
// nested option bullets under Type, and the first ```json example.
func fill(e *entry) {
	var para []string
	inNote := false
	for _, l := range e.body {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "<Warning>") || strings.HasPrefix(t, "<Note>") {
			inNote = true
			continue
		}
		if strings.HasPrefix(t, "</Warning>") || strings.HasPrefix(t, "</Note>") {
			inNote = false
			continue
		}
		if inNote {
			continue
		}
		if t == "" {
			if len(para) > 0 {
				break
			}
			continue
		}
		if listRE.MatchString(l) || strings.HasPrefix(l, "```") || strings.HasPrefix(l, "<") {
			break
		}
		para = append(para, t)
	}
	e.Desc = strings.Join(para, " ")
	for j, l := range e.body {
		if m := scopeRE.FindStringSubmatch(l); m != nil {
			e.Scope = scopeLink.ReplaceAllString(m[1], "$1")
		}
		if m := typeRE.FindStringSubmatch(l); m != nil {
			e.Type = m[1]
			for k := j + 1; k < len(e.body); k++ {
				nm := nestedRE.FindStringSubmatch(e.body[k])
				if nm == nil {
					break
				}
				if om := optionRE.FindStringSubmatch(nm[1]); om != nil {
					e.Options = append(e.Options, option{Value: om[1], Help: om[2]})
				} else {
					e.Options = append(e.Options, option{Value: nm[1]})
				}
			}
		}
		if m := defaultRE.FindStringSubmatch(l); m != nil {
			e.Default = m[1]
		}
		if strings.HasPrefix(l, "```json") && e.Example == "" {
			var ex []string
			for k := j + 1; k < len(e.body) && !strings.HasPrefix(e.body[k], "```"); k++ {
				ex = append(ex, e.body[k])
			}
			e.Example = strings.Join(ex, "\n")
		}
	}
}

var (
	warningRE = regexp.MustCompile(`(?s)<Warning>(.*?)</Warning>`)
	depWordRE = regexp.MustCompile(`[Rr]emoved|[Dd]eprecated|no effect`)
	removedRE = regexp.MustCompile(`Removed in v|has no effect`)
	linkRE    = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	boldRE    = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	spaceRE   = regexp.MustCompile(`\s+`)
)

// deprecated returns the entry's Warning text when it says the key is
// deprecated, removed, or without effect.
func (e *entry) deprecated() string {
	w := warningRE.FindStringSubmatch(strings.Join(e.body, "\n"))
	if w == nil || !depWordRE.MatchString(w[1]) {
		return ""
	}
	return strings.TrimSpace(spaceRE.ReplaceAllString(linkRE.ReplaceAllString(w[1], "$1"), " "))
}

// removed says whether a deprecation note means current versions ignore the key.
func removed(note string) bool { return removedRE.MatchString(note) }

// clean strips Markdown links and bold.
func clean(s string) string {
	return strings.TrimSpace(boldRE.ReplaceAllString(linkRE.ReplaceAllString(s, "$1"), "$1"))
}
