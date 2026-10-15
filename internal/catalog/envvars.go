package catalog

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jorden-parker/claude-config-cli/internal/schema"
)

var (
	envHeaderRE = regexp.MustCompile(`^\|\s*Variable\s*\|\s*Purpose\b`)
	tableSepRE  = regexp.MustCompile(`^\|\s*:?-+`)
	envRowRE    = regexp.MustCompile("^\\| `([A-Z0-9_]+)`\\s+\\| (.*?)\\s+\\|")
)

// parseEnv reads the "## Variables" table of the environment variables page.
// Only that table lists variables Claude Code reads; later tables are
// examples. Rows the docs mark as removed come back separately.
func parseEnv(text string) (env, dropped []schema.EnvVar, err error) {
	_, after, ok := strings.Cut("\n"+text, "\n## Variables\n")
	if !ok {
		return nil, nil, fmt.Errorf(`env-vars page has no "## Variables" section`)
	}
	if i := strings.Index(after, "\n## "); i >= 0 {
		after = after[:i]
	}
	var rows []string
	for _, l := range strings.Split(after, "\n") {
		if strings.HasPrefix(l, "|") {
			rows = append(rows, l)
		}
	}
	if len(rows) < 3 || !envHeaderRE.MatchString(rows[0]) || !tableSepRE.MatchString(rows[1]) {
		return nil, nil, fmt.Errorf(`env-vars "## Variables" table has no Variable/Purpose header`)
	}
	names := map[string]bool{}
	env, dropped = []schema.EnvVar{}, []schema.EnvVar{}
	for _, l := range rows[2:] {
		m := envRowRE.FindStringSubmatch(l)
		if m == nil {
			return nil, nil, fmt.Errorf("malformed env-vars table row: %s", truncate(l, 80))
		}
		if names[m[1]] {
			return nil, nil, fmt.Errorf("duplicate env var: %s", m[1])
		}
		names[m[1]] = true
		v := schema.EnvVar{Name: m[1], Purpose: clean(m[2])}
		first, _, _ := strings.Cut(v.Purpose, ". ")
		if removed(first) {
			dropped = append(dropped, v)
		} else {
			env = append(env, v)
		}
	}
	if len(env) == 0 {
		return nil, nil, fmt.Errorf("env-vars table has no variables")
	}
	return env, dropped, nil
}
