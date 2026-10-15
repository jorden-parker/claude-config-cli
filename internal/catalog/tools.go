package catalog

import (
	"fmt"
	"regexp"
	"strings"
)

// notDeniable are documented tools Claude Code rejects in permissions.deny,
// so the Tools pane leaves them out.
var notDeniable = map[string]bool{"SubagentHandback": true, "TaskOutput": true}

var (
	toolsHeaderRE = regexp.MustCompile(`^\|\s*Tool\s*\|\s*Description\b`)
	toolRowRE     = regexp.MustCompile("^\\|\\s*`(\\w+)`\\s*\\|")
)

// parseToolsTable returns the deniable tool names in the tools reference.
func parseToolsTable(text string) (map[string]bool, error) {
	lines := strings.Split(text, "\n")
	start := -1
	for i, l := range lines {
		if toolsHeaderRE.MatchString(l) {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("tools reference has no Tool/Description table")
	}
	var rows []string
	for _, l := range lines[start+1:] {
		if !strings.HasPrefix(l, "|") {
			break
		}
		rows = append(rows, l)
	}
	if len(rows) < 2 || !tableSepRE.MatchString(rows[0]) {
		return nil, fmt.Errorf("tools reference table has no separator row or no rows")
	}
	names := map[string]bool{}
	for _, l := range rows[1:] {
		m := toolRowRE.FindStringSubmatch(l)
		if m == nil {
			return nil, fmt.Errorf("malformed tools table row: %s", truncate(l, 80))
		}
		if names[m[1]] {
			return nil, fmt.Errorf("duplicate tool in tools reference table: %s", m[1])
		}
		names[m[1]] = true
	}
	for n := range notDeniable {
		delete(names, n)
	}
	return names, nil
}

// CheckTools compares the tools reference with the names the Tools pane
// lists and returns an error naming what to add or remove.
func CheckTools(toolsMD []byte, listed []string) error {
	documented, err := parseToolsTable(string(toolsMD))
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, n := range listed {
		have[n] = true
	}
	add, remove := map[string]bool{}, map[string]bool{}
	for n := range documented {
		if !have[n] {
			add[n] = true
		}
	}
	for n := range have {
		if !documented[n] {
			remove[n] = true
		}
	}
	if len(add)+len(remove) == 0 {
		return nil
	}
	return fmt.Errorf("the Tools pane is out of date: add %v, remove %v", sortedKeys(add), sortedKeys(remove))
}
