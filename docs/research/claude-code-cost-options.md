# Reducing the work Claude Code API bill

Researched 2026-10-07. Scope: direct Anthropic API billing, mainly Sonnet 5.5, sometimes Sonnet 5/4.6 and Haiku. No settings changed. Haiku pricing below assumes Haiku 4.5; its exact work version was not supplied.

**Prioritize cache misses, unnecessary requests, and successful-task cost.** There is no demonstrated universal plugin that reduces the whole bill without workload-dependent tradeoffs.

## 1. Match the cache lifetime to work breaks

Claude Code already caches prompts. Model switches rebuild the cache; compaction generates a summary and rebuilds conversation history. Avoid changing models repeatedly within one large task or compacting on a timer. A gateway that strips cache markers can cause the entire history to be charged uncached, although this user uses Anthropic directly. Changing effort on Sonnet 5.5 with direct Anthropic API access preserves the cache; this exception does not apply universally to older models/providers. [Claude Code caching](https://code.claude.com/docs/en/prompt-caching)

**Investigate one-hour caching when returning to the same conversation after breaks of 5–60 minutes.** It is a conditional saving: the longer write lifetime costs more, so uninterrupted work or a conversation never revisited may be cheaper with five-minute caching. Claude Code documents the supported configuration and provider/version requirements in its [cache lifetime section](https://code.claude.com/docs/en/prompt-caching#cache-lifetime); verify the work installation before adopting it.

On Claude Code **v2.1.242 or later**, merge this setting into the existing work user settings file, or use the environment variable for a one-launch experiment:

```json
{
  "promptCacheTtl": "1h"
}
```

```sh
CLAUDE_CODE_PROMPT_CACHE_TTL=1h claude --model sonnet
```

This affects the main conversation. Leave `subagentPromptCacheTtl` at its default unless those requests also benefit from longer reuse. `FORCE_PROMPT_CACHING_5M=1` overrides both controls; `/usage` on recent versions shows the actual TTL and cache misses. [Exact controls and precedence](https://code.claude.com/docs/en/prompt-caching#choose-the-ttl-yourself)

Illustration using current Sonnet 5.5 rates: an unchanged 100,000-token prefix written twice, ten minutes apart, costs $0.25 + $0.25 = **$0.50** with five-minute writes. A one-hour write plus one cached read costs $0.40 + $0.02 = **$0.42**. That is 16% of this prefix cost, not 16% of the bill. Additional messages, output, and cache changes are excluded. Conversely, if the second request arrives while both caches remain warm, the longer write lifetime adds $0.15 for this prefix. [Pricing](https://platform.claude.com/docs/en/about-claude/pricing)

## 2. Stop requests you do not need

Set `"promptSuggestionEnabled": false` in work user settings, or turn off Prompt suggestions in `/config`. An environment override can supersede the setting:

```sh
export CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false
```

This removes the suggestion generator when it would otherwise run. Suggestions already skip several situations, including cold caches, so savings depend on actual invocation frequency. [Suggestion controls](https://code.claude.com/docs/en/interactive-mode#turn-prompt-suggestions-off)

Review `/usage` for attribution and heavy loops before adding optimizers. Its dollar estimate is local; the Anthropic Console is the billing authority. Do not run `/insights` repeatedly as a free meter: its analysis consumes tokens. Clear context when changing to unrelated work. Keep required project instructions concise and use specialist skills when needed. Language-server plugins can reduce broad source-code searches, but the official recommendation supplies no universal billing reduction. Agent teams duplicate contexts; use them where their additional work is necessary. [Cost guidance](https://code.claude.com/docs/en/costs)

For hooks, distinguish local command handlers from model-powered `prompt` and `agent` handlers. A formatter implemented as a shell command does not require a language-model decision. Review frequently triggered model hooks before removing useful checks; do not disable validation just to make the meter smaller. [Hook mechanisms](https://code.claude.com/docs/en/hooks)

## 3. Use the existing models economically

Current standard direct-API dollars per million tokens:

| Model      | New input | Output | Five-minute write | One-hour write | Cached read |
| ---------- | --------: | -----: | ----------------: | -------------: | ----------: |
| Sonnet 5.5 |        $2 |    $10 |             $2.50 |             $4 |       $0.20 |
| Sonnet 5   |        $2 |    $10 |             $2.50 |             $4 |       $0.20 |
| Sonnet 4.6 |        $3 |    $15 |             $3.75 |             $6 |       $0.30 |
| Haiku 4.5  |        $1 |     $5 |             $1.25 |             $2 |       $0.10 |

Sonnet 5.5 has no listed per-token price advantage over Sonnet 5. Moving from 4.6 lowers these unit rates by one third, but newer tokenizers can produce more tokens for identical text, and task behavior differs. These are unit-price comparisons, not promised task savings. [Official price table and tokenizer caveat](https://platform.claude.com/docs/en/about-claude/pricing)

On Sonnet 5.5, **medium is already the default**. `/effort medium` only saves against a higher effective setting. Trial `low` on tightly scoped edits you review; retain stronger verification for difficult bugs. Do not prescribe effort controls to Haiku 4.5, which does not support the same effort setting. Use `/effort` rather than assuming an old `alwaysThinkingEnabled` setting governs modern adaptive reasoning. [Model and effort configuration](https://code.claude.com/docs/en/model-config)

Anthropic's own experiments show higher effort doing more verification and making more autonomous choices; they are evidence of a tradeoff, not a fixed savings percentage. A clear specification can make low-effort iterations more useful. [Anthropic effort experiments](https://claude.dev/blog/spending-your-effort/)

## 4. Treat compression plugins as experiments

**RTK:** the earlier recommendation to prioritize it was too strong. Independent tests and the maintainer's follow-up do not establish reliable overall savings. See the [detailed evidence review](rtk-cost-evidence.md), including version differences, statistical uncertainty, and provenance problems in one paper's artifacts.

**Caveman:** JetBrains tested real agent tasks and found 8.5% fewer output tokens, with no statistically detectable quality difference. However, aggregate billed cost was 11.6% higher in the full run because one task became a costly outlier. The authors attribute that outlier to task variance, not proof the skill caused it. Their roughly 10% expected saving is not a guarantee. [Independent paired evaluation](https://blog.jetbrains.com/ai/2026/07/speak-to-ai-agents-like-cavemen-tosave-tokens/)

The project's current small Q&A benchmark says ordinary Caveman adds only 3% median output reduction beyond “Answer concisely,” while its rules add around 1,000 input tokens per call. My recommendation is to try the short instruction first. Its proxy is a different mechanism and needs its own comparison. [Maintainer's current results](https://github.com/JuliusBrussee/caveman)

**Context-mode:** its advertised reductions predominantly compare raw bytes with selected context bytes, such as large logs and browser snapshots. Those results establish that it can send less text, not that total paid requests, reasoning, retries, or final correctness improve. Trial it only on demonstrably verbose workloads, with an unmodified baseline. [Maintainer benchmarks](https://github.com/mksglu/context-mode/blob/main/BENCHMARK.md)

A separate eight-model research study found input linguistic compression could raise costs through longer responses and reduce accuracy. Its benchmark is not Claude Code, but it reinforces why preserving information and measuring complete tasks matters. [CAVEWOMAN paper](https://arxiv.org/abs/2606.24083)

## 5. Bound automation and measure completed work

For unattended Claude Code API jobs, use a bounded invocation such as:

```sh
claude -p --model sonnet --max-budget-usd 2 --max-turns 10 'Your bounded task'
```

These are print-mode controls. The budget uses the client estimate and may stop a task unfinished; it is not a guaranteed invoice ceiling. [CLI reference](https://code.claude.com/docs/en/cli-reference)

For independent, delay-tolerant jobs, Anthropic's **Message Batches API charges 50% of standard prices**. It requires a separate asynchronous API workflow, not a Claude Code setting. Most batches finish within an hour; requests can expire after 24 hours. Good candidates include bulk classification or independent evaluations rather than an interactive coding loop. [Batch API](https://platform.claude.com/docs/en/build-with-claude/batch-processing)

Evaluate one change at a time on representative tasks. Record input, both cache-write classes, cache reads, output, total dollars, retries, and whether the result passes the same acceptance check. Include failed attempts and outliers. Compare cost per successful completed task, not only tokens removed by a plugin. For subscriptions, API-equivalent figures are not the monthly bill; this user's work API arrangement makes actual metered spend the relevant target.

Next action: run `/usage` in one representative work session and inspect the prompt-cache statistics before changing cache lifetime.
