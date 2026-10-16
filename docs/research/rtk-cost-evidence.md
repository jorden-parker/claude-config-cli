# RTK: evidence about actual Claude Code cost

Researched 7 October 2026. Scope: whether RTK lowers end-to-end API cost, rather than its own count of removed command-output tokens. No user configuration was changed and no paid evaluation was run.

## Decision

**Do not adopt RTK as a proven cost-saving default.** The evidence supports a narrower conclusion: compression is real, but reliable savings on completed Claude Code tasks have not been established across the tested workloads. One credible experiment found an increase; several others were statistically inconclusive; a larger paper reports a small stock-RTK reduction with important qualifications. “RTK always costs more” would overstate the evidence too.

The earlier recommendation to make RTK the first compression tool to try was too strong. For a user paying Anthropic directly for Sonnet 5.5, none of the results below is a direct estimate of their savings: model, effort, RTK version, task distribution, and integration all differ. Treat a proposed compressor as an experiment with a budget, not a discounted API tariff.

## Evidence at a glance

| Source                            | Tested setup                                                              | Reported cost result                                          | Interpretation                                               |
| --------------------------------- | ------------------------------------------------------------------------- | ------------------------------------------------------------- | ------------------------------------------------------------ |
| JetBrains                         | RTK 0.43.0, Claude Code 2.1.201, Sonnet 5, SkillsBench                    | Low effort: median +7.6%, p=.004; high: +0.1%, p=.99          | Evidence of an increase in one setting, not all settings     |
| Glia                              | Six Aider Polyglot tasks; Opus 5 low effort, three attempts/task/arm      | Aggregate +5.8%; task-median +2.4%, interval −18.0% to +33.3% | No reliable direction                                        |
| RTK maintainers, August follow-up | RTK 0.45.0, SkillsBench subset; 12 balanced tasks, four attempts/arm/task | Pooled −4.8%, permutation p=.305                              | Favorable point estimate, no demonstrated saving             |
| PointFive paper v5                | 103 tasks; four arms, 712 executions/arm                                  | Stock RTK −2.7%; holdout interval crosses zero                | Suggestive small saving; provenance mismatch described below |

Sources: [JetBrains](https://blog.jetbrains.com/ai/2026/07/rtk-claude-code-token-savings/), [Glia](https://gliai.com/research/rtk-token-savings-benchmark), [RTK follow-up](https://www.rtk-ai.app/blog/rtk-on-skillsbench/), [PointFive paper v5](https://arxiv.org/html/2607.12161v5).

## JetBrains: useful evidence, with bounded generality

The full runs retained 80 clean task pairs at low effort and 71 at high effort. The advertised 425 billed trials include smoke tests and repetitions, not 425 independent tasks. Low effort also showed more turns and cache reads. The experiment used paired comparisons and instrumented hook activation. [JetBrains study](https://blog.jetbrains.com/ai/2026/07/rtk-claude-code-token-savings/)

Errored pairs were excluded symmetrically. One excluded task exposed a genuine RTK/glibc compatibility failure. Quality scores did not differ detectably; this does not prove equivalence. Full suites used one attempt per task per arm, despite repeated pilot trials. The article does not link a downloadable harness or complete raw dataset, limiting independent verification. JetBrains publishes competing coding-agent products, so this is vendor research, although that does not invalidate its measurements. [Study and methodology](https://blog.jetbrains.com/ai/2026/07/rtk-claude-code-token-savings/)

Do not translate a median percentage into a guaranteed change in a monthly bill. A median describes the middle task; a monthly total weights expensive tasks more heavily. Nor does significance in one effort setting and nonsignificance in another itself establish an effort-by-treatment interaction.

## Glia: the headline increase is not a reliable increase

Glia’s main Claude result contains only six distinct tasks, not 18 independent tasks or 132 independent Claude comparisons. Its 132-run total also includes Codex experiments. Main costs were $2.66 with RTK versus $2.52 without; 17/18 RTK attempts passed, versus 18/18 baseline attempts. Its new-input target showed −0.5%, with a −10.0% to +25.2% interval. [Glia study](https://gliai.com/research/rtk-token-savings-benchmark)

The article excludes cache reads from that primary input metric, while reporting full Claude costs separately. It does not identify an RTK or Claude Code version, link raw run data, or fully specify interval construction. Its authors appropriately say the evidence does not establish increased cost. Glia sells AI implementation services; its result is first-party commercial research, not an independently reproduced benchmark. [Benchmark](https://gliai.com/research/rtk-token-savings-benchmark), [company services](https://gliai.com/)

Six task clusters give limited information about other repositories and workflows. More repetitions reduce within-task noise; they do not create new task diversity. One extra failure is a reason to measure correctness, not evidence that RTK has a precisely estimated reliability penalty.

## The maintainers' follow-up matters

The August 7 maintainer report tests RTK 0.45.0, after changes to earlier behavior. Its four-attempt experiment retained 12 balanced tasks, excluding a verifier-problem task from the original 13. Pooled cost decreased 4.8%, but permutation p=.305 did not establish a reliable saving. A smaller three-task, six-attempt experiment likewise reported −2.6%, p=.89. The article links raw experiment downloads, improving auditability; those archives were not independently reanalyzed for this note. [Maintainer follow-up](https://www.rtk-ai.app/blog/rtk-on-skillsbench/)

This result prevents a fair review from stopping at July's negative headline. It also does not reverse the recommendation: a nonsignificant favorable point estimate is not proof that installing the tool saves money. The authors maintain RTK and have an obvious interest in its performance, just as competing vendors have their own interests. Judge the design and available artifacts alongside those interests.

Version history matters. The changelog records safe-final-pipeline rewriting in 0.44.0 and further pipeline-safety work in 0.49.0. Therefore, specific pipeline failures in older experiments should not be described as confirmed current defects. Fixing a failure mechanism is useful engineering evidence; it is not an end-to-end cost replication. [RTK changelog](https://github.com/rtk-ai/rtk/blob/develop/CHANGELOG.md)

## PointFive: distinguish stock RTK from RTK-ML

The paper's +6.8% cost result belongs to **RTK-ML, a custom nine-gate system**, not ordinary RTK. Its stock result is −2.7% with a −5.6% to −0.1% interval; the holdout alone is −2.3%, interval −7.4% to +2.1%. Stock cost-per-success ratio is .968, interval .937–1.004. The authors disclose PointFive affiliation and author-configured integrations. [Paper v5](https://arxiv.org/html/2607.12161v5)

The release contains task manifests, deterministic judges, run metadata, and analysis scripts. I cloned immutable tag `v1.0.0`, commit `b2d9eae84a6b9bed78d483589a9e587b442a2ec9`, and inspected the artifacts. Deduplicating by `run_id`, removing calibration and defective tasks, and retaining completed non-infrastructure-error records reproduces **2,848 analyzed runs, 103 tasks, 712 runs per arm**. This is a direct arithmetic check of released records, not a rerun of the experiment. [Released metadata](https://github.com/PointFiveLabs/ai-efficiency-benchmark/blob/v1.0.0/deliverables/results_metadata.jsonl), [analysis logic](https://github.com/PointFiveLabs/ai-efficiency-benchmark/blob/v1.0.0/analysis/stats2.py)

The resulting raw totals are $38.90434045 for baseline and $37.72548525 for the stock-labelled arm, with 685 and 686 successes respectively. These totals are not the paper's task-weighted estimator and should not be substituted for its confidence interval. Their value here is verifying that the retained records are substantive and broadly consistent with the campaign size and direction. Repeated model/effort executions of one task remain correlated; the provided analysis groups paired differences by task before bootstrapping. [Metadata](https://github.com/PointFiveLabs/ai-efficiency-benchmark/blob/v1.0.0/deliverables/results_metadata.jsonl), [statistics implementation](https://github.com/PointFiveLabs/ai-efficiency-benchmark/blob/v1.0.0/analysis/stats2.py)

**A material provenance contradiction remains:** the paper says all its RTK experiments used 0.44.1, but the pinned release's provenance document explicitly says clean release **0.7.6**, binary `rtk_v076`, no patches. The runner, analysis arm labels, and run records also identify `rtk_v0.7.6`; an accompanying analysis names tag `985e04e`. This is more than an ambiguous display label because the provenance file explicitly records the release and build procedure. I cannot reconcile these claims from the released material. Do not cite this as a verified 0.44.1 benchmark without clarification from the authors. [Paper](https://arxiv.org/html/2607.12161v5), [frozen provenance](https://github.com/PointFiveLabs/ai-efficiency-benchmark/blob/v1.0.0/docs/ARM_PROVENANCE.md), [stock-arm analysis](https://github.com/PointFiveLabs/ai-efficiency-benchmark/blob/v1.0.0/deliverables/RTK_V076_ANALYSIS.md)

Raw transcripts are withheld. Accordingly, independent readers can recompute statistics from derived ledgers but cannot fully audit the underlying traces. That distinction is important when the question is whether a filter changed reasoning, suppressed an error, or caused additional retrieval. [Artifact release](https://github.com/PointFiveLabs/ai-efficiency-benchmark/tree/v1.0.0)

## What actually establishes a saving

The relevant counterfactual is the same completed task without the add-on. A command can print megabytes while the agent harness delivers a truncated result; removing bytes the model would never receive does not reduce an API bill. Compression can also change subsequent actions. Extra diagnosis, retries, or verification can offset a smaller initial result. Conversely, clearer output could shorten a trajectory. The direction needs measurement.

For direct API billing, compare dollars per successfully completed task and preserve the underlying categories: uncached input, cache creation, cache reads, and generated output. Multiplying a tool's estimated removed tokens by the full input price is not a valid bill reconstruction. Cache discounts, repeated use, instruction overhead, and different trajectories all affect the answer.

A defensible local trial would pin model, effort, CLI and plugin versions; use fresh identical worktrees; alternate or randomize arm order; include several genuinely different tasks with repeated attempts; and score completion with tests. Include failed attempts and all subagent usage. Report both total spend and task-level paired changes with uncertainty. Set a maximum experiment spend before starting. Do not run a paid benchmark simply to justify a plugin whose expected saving is smaller than the experiment's cost.

For this user's Sonnet 5.5 workflow, the practical decision is to leave RTK out of the cost-saving baseline unless representative local evidence shows otherwise. Focus first on optimizations with transparent mechanisms—appropriate model and effort, bounded tasks, useful context, and fewer unnecessary agent calls—and validate their effect on successful work too.
