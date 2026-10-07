package statusline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Field is one piece of the status line. Source names the stdin JSON path it
// reads, as documented at DocURL.
type Field struct {
	ID     string
	Group  string
	Name   string
	Desc   string
	Source string
	color  string // ANSI 256-colour number
	render func(input) string
}

// input is the JSON Claude Code pipes to a status line command. The docs list
// fields that can be absent or null; every lookup treats both as "nothing".
type input struct {
	d   map[string]any
	now time.Time
}

func parse(b []byte, now time.Time) input {
	in := input{d: map[string]any{}, now: now}
	_ = jsonUnmarshal(b, &in.d)
	return in
}

func (in input) at(path string) any {
	var cur any = in.d
	for _, p := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[p]
	}
	return cur
}

func (in input) str(path string) string {
	s, _ := in.at(path).(string)
	return s
}

func (in input) num(path string) (float64, bool) {
	n, ok := in.at(path).(float64)
	return n, ok
}

func (in input) flag(path string) bool {
	b, _ := in.at(path).(bool)
	return b
}

// Fields is the catalogue, in the order the editor lists it.
var Fields = []Field{
	{ID: "model", Group: "Session", Name: "Model", Source: "model.display_name", color: "214",
		Desc:   "The current model's display name, such as Opus.",
		render: func(in input) string { return in.str("model.display_name") }},
	{ID: "model_id", Group: "Session", Name: "Model ID", Source: "model.id", color: "245",
		Desc:   "The current model's full identifier.",
		render: func(in input) string { return in.str("model.id") }},
	{ID: "effort", Group: "Session", Name: "Effort", Source: "effort.level", color: "175",
		Desc:   "The reasoning effort in use: low, medium, high, xhigh, or max. Hidden when the model has no effort setting.",
		render: func(in input) string { return in.str("effort.level") }},
	{ID: "thinking", Group: "Session", Name: "Thinking", Source: "thinking.enabled", color: "175",
		Desc: "Shows \"thinking\" while extended thinking is on.",
		render: func(in input) string {
			if in.flag("thinking.enabled") {
				return "thinking"
			}
			return ""
		}},
	{ID: "fast", Group: "Session", Name: "Fast mode", Source: "fast_mode", color: "214",
		Desc: "Shows \"fast\" while fast mode is on.",
		render: func(in input) string {
			if in.flag("fast_mode") {
				return "fast"
			}
			return ""
		}},
	{ID: "vim", Group: "Session", Name: "Vim mode", Source: "vim.mode", color: "142",
		Desc:   "The vim mode: NORMAL, INSERT, VISUAL, or VISUAL LINE. Hidden unless vim mode is on.",
		render: func(in input) string { return in.str("vim.mode") }},
	{ID: "agent", Group: "Session", Name: "Agent", Source: "agent.name", color: "175",
		Desc:   "The agent name. Hidden unless the session runs with --agent or agent settings.",
		render: func(in input) string { return in.str("agent.name") }},
	{ID: "style", Group: "Session", Name: "Output style", Source: "output_style.name", color: "245",
		Desc:   "The name of the current output style.",
		render: func(in input) string { return in.str("output_style.name") }},
	{ID: "session", Group: "Session", Name: "Session name", Source: "session_name", color: "214",
		Desc:   "The session's name from --name, /rename, or its generated title. Hidden until it has one.",
		render: func(in input) string { return in.str("session_name") }},
	{ID: "session_id", Group: "Session", Name: "Session ID", Source: "session_id", color: "245",
		Desc: "The first 8 characters of the session identifier.",
		render: func(in input) string {
			id := in.str("session_id")
			if len(id) > 8 {
				id = id[:8]
			}
			return id
		}},
	{ID: "version", Group: "Session", Name: "Version", Source: "version", color: "245",
		Desc: "The Claude Code version.",
		render: func(in input) string {
			if v := in.str("version"); v != "" {
				return "v" + v
			}
			return ""
		}},

	{ID: "dir", Group: "Workspace", Name: "Directory", Source: "workspace.current_dir", color: "109",
		Desc: "The name of the current working directory.",
		render: func(in input) string {
			return base(in.currentDir())
		}},
	{ID: "project", Group: "Workspace", Name: "Project", Source: "workspace.project_dir", color: "109",
		Desc:   "The name of the directory Claude Code was launched in.",
		render: func(in input) string { return base(in.str("workspace.project_dir")) }},
	{ID: "added_dirs", Group: "Workspace", Name: "Added directories", Source: "workspace.added_dirs", color: "109",
		Desc: "How many directories were added with /add-dir. Hidden when there are none.",
		render: func(in input) string {
			dirs, _ := in.at("workspace.added_dirs").([]any)
			switch len(dirs) {
			case 0:
				return ""
			case 1:
				return "+1 dir"
			}
			return fmt.Sprintf("+%d dirs", len(dirs))
		}},
	{ID: "repo", Group: "Workspace", Name: "Repository", Source: "workspace.repo", color: "109",
		Desc: "The owner and name of the origin remote. Hidden outside a git repository or without an origin.",
		render: func(in input) string {
			owner, name := in.str("workspace.repo.owner"), in.str("workspace.repo.name")
			if owner == "" || name == "" {
				return ""
			}
			return owner + "/" + name
		}},
	{ID: "branch", Group: "Workspace", Name: "Git branch", Source: ".git/HEAD", color: "142",
		Desc:   "The checked-out branch. The session data has no branch, so this reads .git/HEAD in the current directory without running git.",
		render: func(in input) string { return gitBranch(in.currentDir()) }},
	{ID: "worktree", Group: "Workspace", Name: "Worktree", Source: "worktree.name", color: "142",
		Desc: "The worktree name, from the worktree session or the linked git worktree. Hidden in the main working tree.",
		render: func(in input) string {
			if n := in.str("worktree.name"); n != "" {
				return n
			}
			return in.str("workspace.git_worktree")
		}},
	{ID: "pr", Group: "Workspace", Name: "Pull request", Source: "pr", color: "142",
		Desc: "The open pull request or merge request for the branch, with its review state. Hidden when there is none.",
		render: func(in input) string {
			n, ok := in.num("pr.number")
			if !ok {
				return ""
			}
			mark := "#"
			if in.str("pr.kind") == "mr" {
				mark = "!"
			}
			return strings.TrimSpace(fmt.Sprintf("%s%d %s", mark, int(n), strings.ReplaceAll(in.str("pr.review_state"), "_", " ")))
		}},

	{ID: "ctx_used", Group: "Context", Name: "Context used", Source: "context_window.used_percentage", color: "108",
		Desc:   "How much of the context window is used, as a percentage. Hidden before the first response.",
		render: func(in input) string { return percent(in, "context_window.used_percentage", "", " ctx") }},
	{ID: "ctx_left", Group: "Context", Name: "Context left", Source: "context_window.remaining_percentage", color: "108",
		Desc:   "How much of the context window remains, as a percentage. Hidden before the first response.",
		render: func(in input) string { return percent(in, "context_window.remaining_percentage", "", " left") }},
	{ID: "ctx_size", Group: "Context", Name: "Context size", Source: "context_window.context_window_size", color: "108",
		Desc: "The context window's size in tokens: 200k, or 1M for extended context.",
		render: func(in input) string {
			if n, ok := in.num("context_window.context_window_size"); ok && n > 0 {
				return compact(n) + " window"
			}
			return ""
		}},
	{ID: "tokens", Group: "Context", Name: "Tokens", Source: "context_window.total_input_tokens", color: "108",
		Desc: "Input and output tokens in the context window, from the latest response.",
		render: func(in input) string {
			i, ok1 := in.num("context_window.total_input_tokens")
			o, ok2 := in.num("context_window.total_output_tokens")
			if !ok1 && !ok2 {
				return ""
			}
			return compact(i) + "↑ " + compact(o) + "↓"
		}},
	{ID: "over_200k", Group: "Context", Name: "Over 200k", Source: "exceeds_200k_tokens", color: "167",
		Desc: "Shows \">200k\" once the latest response passed 200,000 tokens.",
		render: func(in input) string {
			if in.flag("exceeds_200k_tokens") {
				return ">200k"
			}
			return ""
		}},
	{ID: "cache_hit", Group: "Context", Name: "Cache hits", Source: "prompt_cache.hit_ratio", color: "108",
		Desc: "The share of input tokens read from the prompt cache this session. Hidden before the first response.",
		render: func(in input) string {
			if r, ok := in.num("prompt_cache.hit_ratio"); ok {
				return fmt.Sprintf("cache %.0f%%", r*100)
			}
			return ""
		}},
	{ID: "cache_state", Group: "Context", Name: "Cache state", Source: "prompt_cache.warm", color: "108",
		Desc: "Whether the prompt cache is still warm, with its lifetime (5m or 1h). Hidden before the first response.",
		render: func(in input) string {
			warm, ok := in.at("prompt_cache.warm").(bool)
			switch {
			case !ok:
				return ""
			case !warm:
				return "cache cold"
			}
			return strings.TrimSpace("cache warm " + in.str("prompt_cache.ttl"))
		}},

	{ID: "cost", Group: "Cost and time", Name: "Cost", Source: "cost.total_cost_usd", color: "208",
		Desc: "The session's estimated cost in US dollars. An estimate at list price, not your bill.",
		render: func(in input) string {
			if c, ok := in.num("cost.total_cost_usd"); ok {
				return fmt.Sprintf("$%.2f", c)
			}
			return ""
		}},
	{ID: "duration", Group: "Cost and time", Name: "Duration", Source: "cost.total_duration_ms", color: "245",
		Desc:   "How long the session has been running.",
		render: func(in input) string { return duration(in, "cost.total_duration_ms", "") }},
	{ID: "api_time", Group: "Cost and time", Name: "API time", Source: "cost.total_api_duration_ms", color: "245",
		Desc:   "Time spent waiting for API responses.",
		render: func(in input) string { return duration(in, "cost.total_api_duration_ms", "api ") }},
	{ID: "lines", Group: "Cost and time", Name: "Lines changed", Source: "cost.total_lines_added", color: "208",
		Desc: "Lines of code added and removed this session.",
		render: func(in input) string {
			a, ok1 := in.num("cost.total_lines_added")
			r, ok2 := in.num("cost.total_lines_removed")
			if !ok1 && !ok2 {
				return ""
			}
			return fmt.Sprintf("+%d -%d", int(a), int(r))
		}},

	{ID: "rate_5h", Group: "Limits", Name: "5-hour limit", Source: "rate_limits.five_hour.used_percentage", color: "167",
		Desc:   "How much of the 5-hour rate limit is used. Only for Pro and Max subscribers, after the first response.",
		render: func(in input) string { return percent(in, "rate_limits.five_hour.used_percentage", "5h ", "") }},
	{ID: "reset_5h", Group: "Limits", Name: "5-hour reset", Source: "rate_limits.five_hour.resets_at", color: "167",
		Desc:   "When the 5-hour rate limit window resets.",
		render: func(in input) string { return reset(in, "rate_limits.five_hour.resets_at", "5h") }},
	{ID: "rate_7d", Group: "Limits", Name: "7-day limit", Source: "rate_limits.seven_day.used_percentage", color: "167",
		Desc:   "How much of the 7-day rate limit is used. Only for Pro and Max subscribers, after the first response.",
		render: func(in input) string { return percent(in, "rate_limits.seven_day.used_percentage", "7d ", "") }},
	{ID: "reset_7d", Group: "Limits", Name: "7-day reset", Source: "rate_limits.seven_day.resets_at", color: "167",
		Desc:   "When the 7-day rate limit window resets.",
		render: func(in input) string { return reset(in, "rate_limits.seven_day.resets_at", "7d") }},
	{ID: "spend", Group: "Limits", Name: "Spend limit", Source: "rate_limits.spend_limit", color: "167",
		Desc: "How much of a gateway spend limit is used, with dollar amounts when the gateway reports them.",
		render: func(in input) string {
			s := percent(in, "rate_limits.spend_limit.used_percentage", "spend ", "")
			used, ok1 := in.num("rate_limits.spend_limit.used_usd")
			limit, ok2 := in.num("rate_limits.spend_limit.limit_usd")
			if s != "" && ok1 && ok2 {
				s += fmt.Sprintf(" $%.0f/$%.0f", used, limit)
			}
			return s
		}},
}

// Lookup returns the field with an ID, or nil.
func Lookup(id string) *Field {
	for i := range Fields {
		if Fields[i].ID == id {
			return &Fields[i]
		}
	}
	return nil
}

// Sample renders one field with the given input and no colour, for previews.
func (f *Field) Sample(in []byte, now time.Time) string { return f.render(parse(in, now)) }

func (in input) currentDir() string {
	if d := in.str("workspace.current_dir"); d != "" {
		return d
	}
	return in.str("cwd")
}

func base(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Base(dir)
}

func percent(in input, path, before, after string) string {
	n, ok := in.num(path)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s%.0f%%%s", before, n, after)
}

// compact shortens a token count: 950, 15.5k, 1M.
func compact(n float64) string {
	trim := func(s string) string { return strings.TrimSuffix(s, ".0") }
	switch {
	case n >= 1e6:
		return trim(fmt.Sprintf("%.1f", n/1e6)) + "M"
	case n >= 1e3:
		return trim(fmt.Sprintf("%.1f", n/1e3)) + "k"
	}
	return fmt.Sprintf("%.0f", n)
}

func duration(in input, path, before string) string {
	ms, ok := in.num(path)
	if !ok {
		return ""
	}
	d := time.Duration(ms) * time.Millisecond
	switch {
	case d >= time.Hour:
		return fmt.Sprintf("%s%dh%02dm", before, int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%s%dm", before, int(d.Minutes()))
	}
	return fmt.Sprintf("%s%ds", before, int(d.Seconds()))
}

// reset shows a reset time, with the weekday when it is more than a day away.
func reset(in input, path, tag string) string {
	ts, ok := in.num(path)
	if !ok {
		return ""
	}
	t := time.Unix(int64(ts), 0).In(in.now.Location())
	layout := "15:04"
	if t.Sub(in.now) >= 24*time.Hour || t.Before(in.now) {
		layout = "Mon 15:04"
	}
	return tag + " ↺ " + t.Format(layout)
}

// gitBranch reads the branch from .git/HEAD, walking up from dir. A detached
// HEAD shows as a short commit hash.
func gitBranch(dir string) string {
	for dir != "" {
		git := filepath.Join(dir, ".git")
		if fi, err := os.Stat(git); err == nil {
			if !fi.IsDir() {
				// A linked worktree's .git is a file that points at its real directory.
				b, _ := os.ReadFile(git)
				ref, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir: ")
				if !ok {
					return ""
				}
				if !filepath.IsAbs(ref) {
					ref = filepath.Join(dir, ref)
				}
				git = ref
			}
			b, err := os.ReadFile(filepath.Join(git, "HEAD"))
			if err != nil {
				return ""
			}
			head := strings.TrimSpace(string(b))
			if name, ok := strings.CutPrefix(head, "ref: refs/heads/"); ok {
				return name
			}
			if len(head) > 7 {
				head = head[:7]
			}
			return head
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
