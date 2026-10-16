# Reducing Claude Code token consumption

Research date: 2026-10-07. Based on primary documentation as currently served; features vary by installed version, model, provider, and account. Research only: no settings changed or plugins installed.

Follow-up: the user's target is a work API bill on direct Anthropic, mainly Sonnet 5.5 plus Haiku. Use the [API cost recommendations](claude-code-cost-options.md) for that setup and the [RTK evidence review](rtk-cost-evidence.md) for the independent benchmarks. The earlier recommendation to prioritize RTK was too strong: reliable end-to-end cost savings have not been established. The plugin comparisons below are capability descriptions, not demonstrated billing reductions.

**Recommended starting point: measure usage, lower effort for bounded tasks, shrink persistent context, then trial one output-reduction tool.** The ranking below is my recommendation, not a measured benchmark of this repository.

## Five priorities

1. **Measure before changing anything.** Run `/usage` and `/context`. Current `/usage` reports session tokens and, for subscriptions, approximate attribution to plugins, skills, subagents, and MCP servers. Its dollar estimate is not a subscription invoice. [Official cost guidance](https://code.claude.com/docs/en/costs)
2. **Match effort to the task.** Start routine work at `medium`; try `low` for small changes you review immediately. Raise effort when correctness requires deeper checking. Higher effort allocates more tokens and tool calls; the cheapest setting can lose its advantage through retries. [Anthropic effort tutorial](https://academy.claude.com/tutorials/choosing-the-right-effort-level-in-claude-code)
3. **Remove irrelevant context.** Use `/clear` between unrelated tasks. Disable unused MCP servers; prefer existing CLIs where practical. These are explicit Anthropic recommendations. [Official cost guidance](https://code.claude.com/docs/en/costs)
4. **Make instructions selective.** Trim `CLAUDE.md`; move specialist instructions into path-scoped rules or skills. Imported files still load, so splitting a large file into `@imports` does not save context. [Memory documentation](https://code.claude.com/docs/en/memory)
5. **Trial targeted tools.** Start with language intelligence for code navigation. Add context-mode only if large tool responses dominate. See the comparison below.

## Settings and commands worth using

For a one-session experiment:

```sh
claude --effort medium
```

Inside Claude Code, `/effort medium` saves a level for the active model. `--effort` applies to one launch. Current versions store persistent choices in `modelSettings`. A top-level `effortLevel` in **user** settings no longer applies to Opus 5.5 and later; project/local/managed settings behave differently. Prefer the command over copying an old JSON preset. [Model configuration](https://code.claude.com/docs/en/model-config)

| Control                                | Recommendation                                                     | Caveat                                                                                                |
| -------------------------------------- | ------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------- |
| `/model sonnet`                        | Compare against your current model on routine coding.              | Lower price per token does not establish lower total tokens or cost per completed task.               |
| `/effort medium`                       | Practical starting experiment; `low` for small supervised changes. | Defaults and supported levels differ by model.                                                        |
| `ENABLE_TOOL_SEARCH`                   | Leave unset on supported direct connections.                       | Current default defers all MCP definitions; `auto:5` is not a universal improvement.                  |
| `promptSuggestionEnabled: false`       | Disable next-prompt suggestions if you do not use them.            | Removes their background generation requests; likely a smaller saving than context or effort changes. |
| `CLAUDE_CODE_DISABLE_TERMINAL_TITLE=1` | Optional small reduction if automatic titles are unnecessary.      | Skips a background title-generation request; unlikely to dominate usage.                              |

The model command and effort controls are documented in [model configuration](https://code.claude.com/docs/en/model-config); tool-search behavior in [MCP documentation](https://code.claude.com/docs/en/mcp); environment controls in the [environment reference](https://code.claude.com/docs/en/env-vars). Prompt suggestions and their background requests are covered in [cost guidance](https://code.claude.com/docs/en/costs#background-token-usage). Merge `"promptSuggestionEnabled": false` into existing settings rather than replacing the file; this repository also supports `ccfg set promptSuggestionEnabled false`.

**Thinking presets can silently do nothing.** Adaptive models ignore positive fixed thinking budgets. On Opus 5.5, Sonnet 5.5, and Fable, neither `MAX_THINKING_TOKENS=0` nor `alwaysThinkingEnabled: false` turns thinking off. Use effort. Collapsing displayed reasoning does not avoid billed thinking tokens. [Model configuration](https://code.claude.com/docs/en/model-config)

For API billing, compare **cost per successful task**, including retries. A stronger model can finish with fewer turns and less re-reading despite a higher unit price. This is distinct from reducing raw token count. [Anthropic cost/intelligence guidance](https://platform.claude.com/docs/en/about-claude/models/optimizing-for-cost-and-intelligence)

## Context, skills, and MCP

Keep each `CLAUDE.md` focused; Anthropic suggests under 200 lines. Path-scoped rules load when relevant. Auto-memory's initial index is limited to 200 lines or 25KB; topic files load on demand. My recommendation is to prune stale entries before disabling useful memory, which may prevent repeated exploration. [Memory documentation](https://code.claude.com/docs/en/memory)

Skill descriptions normally consume context; bodies load when invoked and then persist. Set `disable-model-invocation: true` for explicitly invoked workflows to remove their descriptions from automatic context. Alternatively, `/skills` supports visibility overrides, including `user-invocable-only`. Keep long references in supporting files. [Skills documentation](https://code.claude.com/docs/en/skills)

MCP tool search now defers definitions by default, retaining tool names and server instructions. `ENABLE_TOOL_SEARCH=false` loads everything upfront; `auto` uses a 10% threshold and `auto:N` selects another percentage. Custom API proxies may disable deferral unless explicitly enabled and supported. `alwaysLoad: true` defeats deferral for that server. Old advice to “enable auto:5” needs checking against your version and connection. [MCP documentation](https://code.claude.com/docs/en/mcp)

Subagents can isolate verbose work, but teams add separate contexts and usage. Use them when they prevent enough parent-context growth or duplicated investigation to justify their work. Keep teams small and assignments bounded. [Official cost guidance](https://code.claude.com/docs/en/costs)

## Plugins and utilities, ranked

| Choice                          | Best fit                                  | Evidence and recommendation                                                                                                                                                                                                                                         |
| ------------------------------- | ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Language intelligence / LSP** | Navigating a typed codebase               | Anthropic recommends precise symbol navigation to reduce speculative searches and file reads. Highest-confidence plugin category. [Official guidance](https://code.claude.com/docs/en/costs)                                                                        |
| **context-mode**                | Huge logs, browser results, MCP responses | Sandboxes/indexes output and retrieves excerpts. Its advertised 98% reduction is a selected output benchmark; the plugin also adds hooks and MCP tools. Trial only when this is your bottleneck. [context-mode](https://github.com/mksglu/context-mode)             |
| **Caveman**                     | Overly verbose final responses            | Try a plain concise-output instruction first. Its own ten-question benchmark reports only 3% median incremental benefit over a concise baseline. Its input proxy is a separate feature with separate tradeoffs. [Caveman](https://github.com/JuliusBrussee/caveman) |
| **ccusage**                     | Local usage reporting                     | `npx ccusage@latest` analyzes local usage records. Useful measurement, not a token-saving mechanism itself. [ccusage guide](https://ccusage.com/guide/)                                                                                                             |

For this Go repository, the official **gopls-lsp** plugin is the relevant language-intelligence option. Install its language-server binary and ensure `GOPATH/bin` is on `PATH`, then install the plugin in Claude Code:

```sh
go install golang.org/x/tools/gopls@latest
```

```text
/plugin install gopls-lsp@claude-plugins-official
```

The binary requirement comes from the [official gopls plugin README](https://github.com/anthropics/claude-plugins-official/blob/main/plugins/gopls-lsp/README.md); installation syntax is documented in the [official marketplace repository](https://github.com/anthropics/claude-plugins-official).

## Caching: mainly a cost lever

Caching reuses an unchanged prompt prefix at a lower rate; it does not remove those tokens from context. [Anthropic tool-context guidance](https://platform.claude.com/docs/en/agents-and-tools/tool-use/manage-tool-context)

Leave prompt caching enabled. Switching models rebuilds cache; changing effort does too on many models, although current Opus 5.5, Sonnet 5.5, and Fable 5.1 direct API/subscription sessions preserve it by default. Compaction rewrites history. Avoid repeatedly toggling models or compacting after every exchange. `opusplan` changes models between planning and execution, so it also rebuilds cache. [Claude Code caching](https://code.claude.com/docs/en/prompt-caching)

Current defaults give subscription main conversations a one-hour cache within included usage, versus five minutes for API/usage-credit traffic. `promptCacheTtl: "1h"` can help API users with pauses, but writes cost more; it can be worse for uninterrupted bursts. `subagentPromptCacheTtl` controls the other request bucket. Both settings require v2.1.242+. Verify actual behavior in `/usage`. [Claude Code caching](https://code.claude.com/docs/en/prompt-caching)

Do not use `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` as a generic token optimization: it also disables updates and feature fetching. Prefer a specific control for a specific unwanted feature. [Environment reference](https://code.claude.com/docs/en/env-vars)

## Verify an improvement

This is a proposed experiment, not a claimed result:

1. Record `/context`, `/usage`, model, effort, installed tools, and successful task outcome.
2. Change one setting or plugin; repeat a comparable task from the same repository state.
3. Compare input, output, cache reads/writes, retries, completion quality, and total cost—not just compressed output size.
4. Repeat across three representative tasks; keep a change only if the practical result improves.

**Next action, under two minutes:** run `/context` in your usual Claude Code session and identify its largest avoidable category.
