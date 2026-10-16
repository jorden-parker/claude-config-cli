package diet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jorden-parker/claude-config-cli/internal/perm"
	"github.com/jorden-parker/claude-config-cli/internal/store"
)

// maxClaudeMDLines is the length the doc asks CLAUDE.md files to stay under.
const maxClaudeMDLines = 200

func applied(sc store.Scope, note string) Result {
	return Result{State: Applied, Scope: sc, Note: note}
}

func appliedByDefault(note string) Result {
	return Result{State: Applied, Default: true, Note: note}
}

func notApplied(note string) Result { return Result{State: NotApplied, Note: note} }

func info(note string) Result { return Result{State: Info, Note: note} }

func warning(note string, offenders ...Offender) Result {
	return Result{State: Warning, Note: note, Offenders: offenders}
}

// effortLevelCheck: a saved level of low or medium counts, from the env var or
// from modelSettings. Traps are the thinking settings that do nothing on the
// current models, and a top-level effortLevel in the user file.
func checkEffort(in Input) Result {
	var traps []string
	var offenders []Offender
	if _, sc, ok := in.value("alwaysThinkingEnabled"); ok {
		offenders = append(offenders, Offender{"alwaysThinkingEnabled", sc})
		traps = append(traps, "alwaysThinkingEnabled is set in the "+string(sc)+" file, which does nothing on current models")
	}
	if _, sc, ok := in.env("MAX_THINKING_TOKENS"); ok {
		offenders = append(offenders, Offender{"env.MAX_THINKING_TOKENS", sc})
		traps = append(traps, "MAX_THINKING_TOKENS is set in the "+string(sc)+" file; current models ignore it or ignore its number")
	}
	if v, ok := in.userValue("showThinkingSummaries"); ok && v == false {
		traps = append(traps, "collapsed thinking saves no tokens, because thinking is still billed")
	}
	if v, ok := in.userValue("effortLevel"); ok {
		offenders = append(offenders, Offender{"effortLevel", store.ScopeUser})
		traps = append(traps, fmt.Sprintf("the top-level effortLevel (%v) in the user file is ignored on Opus 5.5 and later", v))
	}
	if len(traps) > 0 {
		return warning(strings.Join(traps, "; "), offenders...)
	}

	var notes []string
	found, sc := false, store.Scope("")
	if val, esc, ok := in.env("CLAUDE_CODE_EFFORT_LEVEL"); ok {
		if val == "low" || val == "medium" {
			found, sc = true, esc
			notes = append(notes, "CLAUDE_CODE_EFFORT_LEVEL="+val+" in the "+string(esc)+" file")
		} else {
			notes = append(notes, fmt.Sprintf("CLAUDE_CODE_EFFORT_LEVEL=%s in the %s file overrides saved levels", val, esc))
		}
	} else if models, msc := lowModels(in); len(models) > 0 {
		found, sc = true, msc
		notes = append(notes, "saved at low or medium for "+strings.Join(models, ", "))
	}
	if found {
		return applied(sc, strings.Join(notes, "; "))
	}
	note := "No model is saved at low or medium."
	if len(notes) > 0 {
		note += " " + strings.Join(notes, "; ") + "."
	}
	return notApplied(note)
}

// lowModels lists the models whose saved effort is low or medium, and the file
// the modelSettings object comes from.
func lowModels(in Input) ([]string, store.Scope) {
	v, sc, ok := in.value("modelSettings")
	if !ok {
		return nil, ""
	}
	obj, _ := v.(map[string]any)
	var out []string
	for name, raw := range obj {
		entry, _ := raw.(map[string]any)
		if lvl, _ := entry["effortLevel"].(string); lvl == "low" || lvl == "medium" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, sc
}

// userValue reads a top-level key from the user settings file only.
func (in Input) userValue(key string) (any, bool) {
	f := in.Files[store.ScopeUser]
	if f == nil {
		return nil, false
	}
	return f.Get(key)
}

// checkModel: Sonnet or Haiku, by alias or by full model ID.
func checkModel(in Input) Result {
	v, sc, ok := in.value("model")
	if !ok || v == nil {
		return notApplied("model is not set, so Claude Code uses your account's default. The docs list that as Opus 5.5 on Pro, Max, Team, Enterprise and API.")
	}
	name := fmt.Sprint(v)
	if isSmallModel(name) {
		return applied(sc, "model is "+name+" in the "+string(sc)+" file")
	}
	return notApplied("model is " + name + " in the " + string(sc) + " file. Sonnet or Haiku cost less per token.")
}

func isSmallModel(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "sonnet") || strings.Contains(n, "haiku")
}

// checkMCPDisable: a server is turned off in a settings file, or with /mcp,
// which records it in this project's entry in ~/.claude.json.
func checkMCPDisable(in Input) Result {
	var names []string
	sc := store.Scope("")
	for _, s := range precedence {
		f := in.Files[s]
		if f == nil {
			continue
		}
		rules, err := perm.Rules(f, "disabledMcpjsonServers")
		if err != nil || len(rules) == 0 {
			continue
		}
		names = append(names, rules...)
		if sc == "" {
			sc = s
		}
	}
	if len(names) > 0 {
		return applied(sc, "turned off in settings: "+strings.Join(names, ", "))
	}
	if off := stringList(in.projectRecord()["disabledMcpServers"]); len(off) > 0 {
		return applied(store.ScopeGlobal, "turned off with /mcp for this project: "+strings.Join(off, ", "))
	}
	servers := in.mcpServers()
	if len(servers) == 0 {
		return info("No MCP servers are set up in .mcp.json or ~/.claude.json, so there is nothing to turn off.")
	}
	return notApplied(fmt.Sprintf("%d MCP server(s) set up, none turned off: %s.", len(servers), strings.Join(serverNames(servers), ", ")))
}

// checkToolSearch: MCP tool search should stay on. Unset counts as on. The
// traps are the values that switch it off or keep it from working.
func checkToolSearch(in Input) Result {
	var offenders []Offender
	var traps []string
	ts, tsScope, tsSet := in.env("ENABLE_TOOL_SEARCH")
	if tsSet && ts == "" {
		tsSet = false
	}
	if v, sc, ok := in.env("CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS"); ok && v == "1" {
		offenders = append(offenders, Offender{"env.CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS", sc})
		traps = append(traps, "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1 turns MCP tool search off")
	}
	if tsSet && (ts == "false" || ts == "auto:5") {
		offenders = append(offenders, Offender{"env.ENABLE_TOOL_SEARCH", tsScope})
		if ts == "false" {
			traps = append(traps, "ENABLE_TOOL_SEARCH=false loads every MCP tool upfront")
		} else {
			traps = append(traps, "ENABLE_TOOL_SEARCH=auto:5 is old advice: it loads tools upfront under 5% of context, and the docs give no number to use")
		}
	}
	if tsSet && ts == "true" {
		if base, _, ok := in.env("ANTHROPIC_BASE_URL"); ok && base != "" && !strings.Contains(base, "api.anthropic.com") {
			traps = append(traps, "ENABLE_TOOL_SEARCH=true behind a proxy fails on proxies that do not forward tool_reference")
		}
	}
	if len(traps) > 0 {
		return warning(strings.Join(traps, "; "), offenders...)
	}
	if !tsSet {
		return appliedByDefault("On by default on Opus 4, Sonnet 4, Haiku 4.5 and later. ENABLE_TOOL_SEARCH is not set.")
	}
	if ts == "true" || ts == "auto" || strings.HasPrefix(ts, "auto:") {
		return applied(tsScope, "ENABLE_TOOL_SEARCH="+ts+" in the "+string(tsScope)+" file")
	}
	return Result{State: NotApplied, Scope: tsScope, Offenders: []Offender{{"env.ENABLE_TOOL_SEARCH", tsScope}},
		Note: fmt.Sprintf("ENABLE_TOOL_SEARCH=%s in the %s file is not a value the docs list. Remove it.", ts, tsScope)}
}

// checkAlwaysLoad: no MCP server may force its full tool list at startup.
func checkAlwaysLoad(in Input) Result {
	servers := in.mcpServers()
	var forced []string
	for _, s := range servers {
		if s.AlwaysLoad {
			forced = append(forced, s.Name+" ("+s.Source+")")
		}
	}
	if len(forced) > 0 {
		return notApplied("alwaysLoad is true for " + strings.Join(forced, ", ") + ". Their full tool lists load at startup.")
	}
	return Result{State: Applied, Note: fmt.Sprintf("None of the %d MCP server(s) set alwaysLoad.", len(servers))}
}

// mcpServer is one server entry from an MCP config.
type mcpServer struct {
	Name, Source string
	AlwaysLoad   bool
}

// mcpServers reads .mcp.json in the project, then the user-level and project
// entries in ~/.claude.json.
func (in Input) mcpServers() []mcpServer {
	var out []mcpServer
	if in.Cwd != "" {
		doc := readJSONObject(filepath.Join(in.Cwd, ".mcp.json"))
		out = append(out, serversIn(doc["mcpServers"], ".mcp.json")...)
	}
	if g := in.Files[store.ScopeGlobal]; g != nil {
		out = append(out, serversIn(g.Data["mcpServers"], "~/.claude.json")...)
	}
	out = append(out, serversIn(in.projectRecord()["mcpServers"], "~/.claude.json, this project")...)
	return out
}

func serversIn(v any, source string) []mcpServer {
	obj, _ := v.(map[string]any)
	names := make([]string, 0, len(obj))
	for name := range obj {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]mcpServer, 0, len(names))
	for _, name := range names {
		entry, _ := obj[name].(map[string]any)
		always, _ := entry["alwaysLoad"].(bool)
		out = append(out, mcpServer{Name: name, Source: source, AlwaysLoad: always})
	}
	return out
}

func serverNames(servers []mcpServer) []string {
	out := make([]string, len(servers))
	for i, s := range servers {
		out[i] = s.Name
	}
	return out
}

// readJSONObject returns a JSON file's top-level object, or nil if the file is
// missing or not an object.
func readJSONObject(path string) map[string]any {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc map[string]any
	if json.Unmarshal(b, &doc) != nil {
		return nil
	}
	return doc
}

// stringList reads a JSON array of strings, ignoring anything else.
func stringList(v any) []string {
	arr, _ := v.([]any)
	var out []string
	for _, x := range arr {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// claudeMDPaths are the instruction files the line check reads. AGENTS.md is
// counted because it was asked for, not because the docs list it.
func (in Input) claudeMDPaths() []struct{ label, path string } {
	var out []struct{ label, path string }
	if in.Home != "" {
		out = append(out, struct{ label, path string }{"~/.claude/CLAUDE.md", filepath.Join(in.Home, ".claude", "CLAUDE.md")})
	}
	if in.Cwd != "" {
		for _, p := range []struct{ label, rel string }{
			{"./CLAUDE.md", "CLAUDE.md"},
			{"./.claude/CLAUDE.md", filepath.Join(".claude", "CLAUDE.md")},
			{"./CLAUDE.local.md", "CLAUDE.local.md"},
			{"./AGENTS.md", "AGENTS.md"},
		} {
			out = append(out, struct{ label, path string }{p.label, filepath.Join(in.Cwd, p.rel)})
		}
	}
	return out
}

// checkClaudeMD: every instruction file under 200 lines. The traps are the env
// var that removes CLAUDE.md entirely, and @imports, which load at launch.
func checkClaudeMD(in Input) Result {
	var parts, over, imports []string
	for _, p := range in.claudeMDPaths() {
		b, err := os.ReadFile(p.path)
		if err != nil {
			continue
		}
		text := string(b)
		n := countLines(text)
		parts = append(parts, fmt.Sprintf("%s %d lines", p.label, n))
		if n >= maxClaudeMDLines {
			over = append(over, fmt.Sprintf("%s has %d lines; keep it under %d", p.label, n, maxClaudeMDLines))
		}
		imports = append(imports, importsIn(text)...)
	}
	var offenders []Offender
	var traps []string
	if v, sc, ok := in.env("CLAUDE_CODE_DISABLE_CLAUDE_MDS"); ok && v == "1" {
		offenders = append(offenders, Offender{"env.CLAUDE_CODE_DISABLE_CLAUDE_MDS", sc})
		traps = append(traps, "CLAUDE_CODE_DISABLE_CLAUDE_MDS=1 removes every CLAUDE.md from context, which is not a diet")
	}
	if len(imports) > 0 {
		traps = append(traps, "imports ("+strings.Join(imports, ", ")+") load at launch, so they save no context")
	}
	if len(traps) > 0 {
		return warning(strings.Join(traps, "; "), offenders...)
	}
	if len(over) > 0 {
		return notApplied(strings.Join(over, "; "))
	}
	if len(parts) == 0 {
		return Result{State: Applied, Note: "No CLAUDE.md or AGENTS.md file found."}
	}
	return Result{State: Applied, Note: strings.Join(parts, ", ")}
}

// importsIn lists @file.md lines, which pull another file in at launch.
func importsIn(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "@") && strings.HasSuffix(t, ".md") && !strings.ContainsAny(t, " \t") {
			out = append(out, t)
		}
	}
	return out
}

// trimmedSkill is a skillOverrides value that hides the skill from the listing
// or shortens it.
var trimmedSkill = map[string]bool{"name-only": true, "user-invocable-only": true, "off": true}

// checkSkillListing: any skillOverrides entry that trims a skill. The entries
// are read from every file, because the docs merge them.
func checkSkillListing(in Input) Result {
	var names []string
	sc := store.Scope("")
	for _, s := range precedence {
		f := in.Files[s]
		if f == nil {
			continue
		}
		v, ok := f.Get("skillOverrides")
		if !ok {
			continue
		}
		obj, _ := v.(map[string]any)
		for name, raw := range obj {
			if val, _ := raw.(string); trimmedSkill[val] {
				names = append(names, name+" ("+val+")")
				if sc == "" {
					sc = s
				}
			}
		}
	}
	if len(names) == 0 {
		return notApplied("No skill is trimmed. Every skill is listed with its description.")
	}
	sort.Strings(names)
	return applied(sc, "trimmed: "+strings.Join(names, ", "))
}

// checkPromptSuggestions: the env var wins over the setting when it is set.
func checkPromptSuggestions(in Input) Result {
	// Trap: any value turns off updates, telemetry and feature flags. It saves
	// no tokens, and it is not a way to switch suggestions off.
	if v, sc, ok := in.env("CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"); ok && v != "" {
		return warning(fmt.Sprintf("CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC is set in the %s file. It is not a token setting: it also turns off updates, telemetry and feature-flag fetching", sc))
	}
	if v, sc, ok := in.env("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION"); ok {
		if strings.EqualFold(v, "false") {
			return applied(sc, "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false in the "+string(sc)+" file, which wins over the setting")
		}
		return notApplied(fmt.Sprintf("CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=%s in the %s file wins over the setting, so changing the setting alone does nothing.", v, sc))
	}
	if v, sc, ok := in.value("promptSuggestionEnabled"); ok && v == false {
		return applied(sc, "promptSuggestionEnabled is false in the "+string(sc)+" file")
	}
	return notApplied("Prompt suggestions are on (the default).")
}

// checkTerminalTitle: the env var set to 1.
func checkTerminalTitle(in Input) Result {
	if v, sc, ok := in.env("CLAUDE_CODE_DISABLE_TERMINAL_TITLE"); ok && v == "1" {
		return applied(sc, "CLAUDE_CODE_DISABLE_TERMINAL_TITLE=1 in the "+string(sc)+" file")
	}
	return notApplied("Terminal titles update automatically.")
}

// checkGitInstructions: includeGitInstructions false.
func checkGitInstructions(in Input) Result {
	if v, sc, ok := in.value("includeGitInstructions"); ok && v == false {
		return applied(sc, "includeGitInstructions is false in the "+string(sc)+" file")
	}
	return notApplied("Built-in commit and PR instructions are on (the default).")
}

// checkCacheTTL: the env var wins over promptCacheTtl when it is set. The trap
// is FORCE_PROMPT_CACHING_5M=1, which overrides the 1-hour setting.
func checkCacheTTL(in Input) Result {
	if v, sc, ok := in.env("FORCE_PROMPT_CACHING_5M"); ok && v == "1" {
		return warning("FORCE_PROMPT_CACHING_5M=1 forces the 5-minute cache and overrides the 1-hour setting",
			Offender{"env.FORCE_PROMPT_CACHING_5M", sc})
	}
	if v, sc, ok := in.env("CLAUDE_CODE_PROMPT_CACHE_TTL"); ok {
		if v == "1h" {
			return applied(sc, "CLAUDE_CODE_PROMPT_CACHE_TTL=1h in the "+string(sc)+" file")
		}
		return notApplied(fmt.Sprintf("CLAUDE_CODE_PROMPT_CACHE_TTL=%s in the %s file wins over promptCacheTtl.", v, sc))
	}
	v, sc, ok := in.value("promptCacheTtl")
	if ok && v == "1h" {
		return applied(sc, "promptCacheTtl is 1h in the "+string(sc)+" file")
	}
	if ok {
		return notApplied(fmt.Sprintf("promptCacheTtl is %v in the %s file.", v, sc))
	}
	return notApplied("promptCacheTtl is not set, so API keys get the 5-minute cache.")
}

// promptCacheOffs are the variables that switch prompt caching off.
var promptCacheOffs = []string{"DISABLE_PROMPT_CACHING", "DISABLE_PROMPT_CACHING_HAIKU", "DISABLE_PROMPT_CACHING_SONNET", "DISABLE_PROMPT_CACHING_OPUS", "DISABLE_PROMPT_CACHING_FABLE"}

// checkKeepCache: any of the disable variables set to 1 is a trap.
func checkKeepCache(in Input) Result {
	var offenders []Offender
	var names []string
	for _, name := range promptCacheOffs {
		if v, sc, ok := in.env(name); ok && v == "1" {
			offenders = append(offenders, Offender{"env." + name, sc})
			names = append(names, name)
		}
	}
	if len(offenders) > 0 {
		return warning("Prompt caching is off: "+strings.Join(names, ", ")+"=1. Every turn then pays full price for the whole context.", offenders...)
	}
	return appliedByDefault("Prompt caching is on. None of the disable variables is set to 1.")
}

// checkSubagents: at least one subagent file with model: haiku. The subagent
// files are the ones in .claude/agents and ~/.claude/agents.
func checkSubagents(in Input) Result {
	agents := in.agents()
	if len(agents) == 0 {
		return info("No subagents in .claude/agents or ~/.claude/agents yet.")
	}
	var small []string
	sc := store.Scope("")
	for _, a := range agents {
		model := frontmatter(a.path)["model"]
		if strings.Contains(strings.ToLower(model), "haiku") {
			small = append(small, a.name)
			if sc == "" {
				sc = a.scope
			}
		}
	}
	if len(small) > 0 {
		return applied(sc, "model: haiku in "+strings.Join(small, ", "))
	}
	return notApplied(fmt.Sprintf("%d subagent(s), none with model: haiku.", len(agents)))
}

// checkSkillFrontmatter counts skills that set disable-model-invocation. The
// docs do not confirm what it does to the listing, so the row is informational.
func checkSkillFrontmatter(in Input) Result {
	skills := in.skills()
	if len(skills) == 0 {
		return info("No skills in .claude/skills or ~/.claude/skills yet.")
	}
	n := 0
	for _, s := range skills {
		if frontmatter(s.path)["disable-model-invocation"] == "true" {
			n++
		}
	}
	return info(fmt.Sprintf("%d of %d skill(s) set disable-model-invocation: true. The docs don't confirm that this hides the skill from Claude's listing, so ccfg doesn't score it.", n, len(skills)))
}

// file is an agent or skill found on disk.
type file struct {
	name, path string
	scope      store.Scope
}

func (in Input) agents() []file {
	var out []file
	if in.Cwd != "" {
		out = append(out, mdFiles(filepath.Join(in.Cwd, ".claude", "agents"), store.ScopeProject)...)
	}
	if in.Home != "" {
		out = append(out, mdFiles(filepath.Join(in.Home, ".claude", "agents"), store.ScopeUser)...)
	}
	return out
}

func (in Input) skills() []file {
	var out []file
	for _, base := range []struct {
		dir string
		sc  store.Scope
	}{{in.Cwd, store.ScopeProject}, {in.Home, store.ScopeUser}} {
		if base.dir == "" {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(base.dir, ".claude", "skills"))
		if err != nil {
			continue
		}
		for _, e := range entries {
			p := filepath.Join(base.dir, ".claude", "skills", e.Name(), "SKILL.md")
			if e.IsDir() {
				if _, err := os.Stat(p); err == nil {
					out = append(out, file{e.Name(), p, base.sc})
				}
			}
		}
	}
	return out
}

func mdFiles(dir string, sc store.Scope) []file {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []file
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		out = append(out, file{strings.TrimSuffix(e.Name(), ".md"), filepath.Join(dir, e.Name()), sc})
	}
	return out
}

// frontmatter reads the flat "key: value" lines between the leading --- lines
// of a Markdown file. Nested or unreadable content is skipped.
func frontmatter(path string) map[string]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}
	out := map[string]string{}
	for _, raw := range lines[1:] {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "---" {
			break
		}
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return out
}
