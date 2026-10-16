# Plan 003: Generate a validated schema from the checked-in references

> Follow the steps and gates in order. Stop on the conditions below. Update your status row in `plans/README.md` on completion unless the dispatching reviewer owns it. Do not refresh network references during this plan.

## Status

| Field              | Value                                                          |
| ------------------ | -------------------------------------------------------------- |
| Priority / finding | P1 / audit finding 3                                           |
| Effort / risk      | M, 8–12 hours / LOW–MED                                        |
| Depends on         | None; parser repair must precede regeneration within this plan |
| Category           | Correctness, DX, tests                                         |
| Planned at         | `7299fa2`, 2026-10-07, including existing working-tree changes |

## Drift check — run first

Run `git diff --stat 7299fa2..HEAD -- tools internal/schema internal/value/value_test.go README.md` and `git status --short`. Compare current-state excerpts; use `plans/baseline.json` as a content fingerprint. The README already has uncommitted snapshot/RTK edits: preserve them. Stop for unexplained parser/schema drift. Do not revert pre-existing changes.

## Why this matters

The checked-in Markdown uses dash bullets, but extraction recognizes only asterisks. A read-only run produced 243 entries with all type/scope metadata missing. Independently, container settings with enum-valued members are classified as scalar enums: valid `voice`/`spinnerVerbs` objects are rejected while incorrect strings can be saved. This plan makes offline generation reproducible, validates before publishing, and fixes those shapes without upgrading the catalogue.

## Current state

`tools/extract.py:44` and `:46` use these exact patterns:

```python
m=re.match(r'\* \*\*Scope\*\*: (.*)', l)
m=re.match(r'\* \*\*Type\*\*: (.*)', l)
```

Nested options likewise require ` *`. `tools/ref.md:2986` instead uses `- **Scope**` and `- **Type**`. `.lintstagedrc` runs Prettier on recognized files, so formatted Markdown is a normal input.

`tools/gen.py:56` classifies member options before root types:

```python
if e['options'] and all(o['value'].startswith('"') for o in e['options']):
    return 'enum', opts
# ... numeric classification ...
if t.startswith('array'): return 'array', []
if t.startswith('object'): return 'json', []
```

`internal/schema/schema.json:2334` describes `spinnerVerbs` as an object with `verbs` and `mode`, but stores `kind: enum` with `append/replace`. The same error affects `voice`, `skillOverrides`, `sandbox.credentials.sigv4`, and `allowedProviders`. The first four must be JSON objects (or groups only if documented child entries require it); `allowedProviders` must be an array of strings.

`gen.py:83` defaults scope class to `any`, writes schema at `:105`, then validates the tools catalogue at `:115`. The tools table header also has padded columns after formatting, which the exact-spaces regex does not accept.

Python uses only the standard library. Go tests are table-driven (`internal/value/value_test.go:19`):

```go
got, err := Parse(get(t, c.key), c.in)
if err != nil { t.Errorf("%s %q: %v", c.key, c.in, err); continue }
if !reflect.DeepEqual(got, c.want) { t.Errorf("%s: got %#v want %#v", c.key, got, c.want) }
```

Keep existing hand-curated aliases/hook events, removed-key lookup, source order, and documented scope restrictions. Root JSON type and member options are distinct concepts. No ADR changes these requirements.

## Commands

Existing verified gates are `go test ./...` and `go vet ./...`; build is `go build ./...`. No install is needed. The following Python commands are **new interfaces this plan creates**, not existing capabilities:

| Purpose               | Command                                                                             | Expected after implementation                                   |
| --------------------- | ----------------------------------------------------------------------------------- | --------------------------------------------------------------- |
| Offline Python tests  | `PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tools -p 'test_*.py' -v` | Exit 0, no network or repo writes                               |
| Offline regeneration  | `PYTHONDONTWRITEBYTECODE=1 python3 tools/gen.py`                                    | Exit 0; only bundled schema output changes                      |
| Read-only drift check | `PYTHONDONTWRITEBYTECODE=1 python3 tools/gen.py --check`                            | Exit 0 when semantically current; exit 1 on drift; never writes |
| Schema/editor tests   | `go test ./internal/schema ./internal/value ./internal/tui -count=1`                | Exit 0                                                          |
| Build / full / static | `go build ./...`, `go test ./...`, `go vet ./...`                                   | Each exits 0                                                    |

Use `gofmt -l internal/schema/schema_test.go internal/value/value_test.go` as a no-output format gate for affected Go tests. Preserve the generated file's repository JSON style; if local Prettier exists, format only `internal/schema/schema.json` using that installed binary. Do not install a formatter solely for this plan.

## Scope

Only modify `tools/extract.py`, `tools/gen.py`, new `tools/test_extract.py`, new `tools/test_gen.py`, `internal/schema/schema.json`, new `internal/schema/schema_test.go`, `internal/value/value_test.go`, README's schema-maintenance subsection, and the index status row. Put compact synthetic fixtures in the Python test files, not new unlisted fixture directories.

Read-only inputs: `tools/ref.md`, `tools/env.md`, `tools/tools.md`, and `internal/tui/tools.go`. Do not run `tools/fetch.sh` or `tools/check-tools.sh`; do not change those inputs, runtime value parsing, model aliases, hook event lists, dependencies, or unrelated docs. Fixing all numeric/map validation belongs elsewhere.

## Git workflow

Use the operator's checkout, or branch `advisor/003-repair-schema-generation` if needed. Include pre-existing dirty changes in any isolated worktree. Do not commit unless asked; suggested message if requested: `fix: validate schema generation before publishing`. Do not push/open PRs without instructions.

## Steps

### Step 1: Make extraction importable and tolerant of Markdown list markers

Refactor `extract.py` into `extract(src: str) -> list[dict]` plus a guarded CLI entry point. Importing it must neither read/write files nor change cwd. Accept `*`, `-`, and `+` at the relevant bullet indentation for Scope/Type/Default and nested option rows; stop collecting the description at any recognized list marker. Preserve code-block extraction, warning handling, order, and existing entry fields.

Write `test_extract.py` using `unittest`. Equivalent synthetic documents with each marker and padded nested indentation must yield identical metadata/options. Read the real checked-in reference too: it has 243 entries at this baseline; every current entry must have type/scope. There is one known removed exception: `taskOutputMaxChars` at `tools/ref.md:3014` lacks both. Make that exception explicit and conditioned on its removed status in generator validation, not a general permission to omit metadata.

**Verify:** `PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tools -p 'test_extract.py' -v` → exit 0; marker variants and real-input metadata assertions pass. `git status --short` → no `entries.json` or `__pycache__` additions.

### Step 2: Validate an in-memory candidate before any output replacement

Refactor `gen.py` into importable functions plus a guarded entry point. Call extraction in memory; eliminate the `entries.json` subprocess handoff and global `chdir`. Resolve default inputs relative to `__file__`. Separate candidate creation/validation/publication so tests can use temporary output paths without touching the repository.

Reject duplicate keys, missing type/scope on non-removed entries, unknown scope prefixes, empty catalogues, and malformed/missing environment/tools tables. Recognize existing scope prefixes explicitly: Any file, Managed, User or managed, User/local/managed (including qualifying prose), and Global config. Preserve scope notes; never default an unknown prefix to writable `any`. Keep the existing `taskOutputMaxChars` override and explicit removed exception; new exceptions require evidence rather than silently widening the allowlist. Parse tools table columns after trimming whitespace so padded headers work. Validate tool-set equality before publication.

Add `--check`: compare parsed candidate/stored JSON semantically, ignoring only `generatedOn` and insignificant JSON formatting, without overwriting either. Normal generation records today's date, validates everything, then writes a same-directory temporary file and replaces output; clean up on failure. This Python publication path uses stdlib and does not depend on Go plan 004.

**Verify:** `PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tools -p 'test_gen.py' -v` → tests for missing metadata, unknown scope, duplicate keys, bad tables, tools drift, read-only check behavior, and unchanged output bytes on failure all pass. At this intermediate point `--check` may legitimately exit 1 because the committed schema has known defects; do not publish yet.

### Step 3: Classify root containers before member enums

Check object and array types before the quoted-options heuristic. Preserve `array of objects`/marketplace-source JSON handling, array-of-strings handling, Boolean/numeric/scalar enum handling, curated overrides, and subsequent group detection. Member option help can remain descriptive metadata, but must not force a scalar picker.

Add generator tests for the five defective settings and control cases `effortLevel`, `model`, `enabledPlugins`, and `strictPluginOnlyCustomization`. Add `TestContainerSettingKinds` in `internal/schema/schema_test.go` and `TestContainerExamplesParse` in `internal/value/value_test.go`: extract each documented example's setting value, encode it as JSON, call `Parse`, and compare the resulting container. The managed-only array is tested as data without bypassing write-scope protection.

**Verify:** Python tests → pass. `go test ./internal/schema ./internal/value -run 'TestContainerSettingKinds|TestContainerExamplesParse' -count=1` → expected assertion failures against the old bundled schema, identifying the container misclassification rather than a compile failure.

### Step 4: Regenerate offline, prove stability, and document the check

Run the offline generator once using only checked-in inputs. Review the semantic schema diff: the five root kinds must be corrected, current metadata/scopes retained, and catalogue/alias/tool membership unchanged. Stop if unrelated classification or scope changes cannot be explained by the agreed parser repair. Do not hand-edit the generated schema to hide a generator defect. Add `--check` and Python test commands to the README maintenance subsection, explicitly distinguishing offline generation from network refresh.

**Verify:** `PYTHONDONTWRITEBYTECODE=1 python3 tools/gen.py --check` → exit 0 and unchanged file bytes, including `generatedOn`. Run all Python tests, the schema/editor test command, build, full Go tests, and vet → all exit 0. Go format gate → no output. Inspect the changed/untracked file list against initial status → only scoped changes.

## Test plan and done criteria

| Requirement                                          | Machine-checkable gate                                                     |
| ---------------------------------------------------- | -------------------------------------------------------------------------- |
| Standard Markdown markers and padded tables work     | Both Python test modules pass against synthetic and checked-in fixtures    |
| Invalid candidates never replace existing output     | Failure-preservation tests compare output bytes and temporary-file cleanup |
| Five container shapes are correct and examples parse | Named Go tests in Step 3 pass after regeneration                           |
| Reproducible offline verification                    | `--check` exits 0, changes no files, and ignores only date/formatting      |
| Compatible bounded change                            | Build/full tests/vet pass; only scoped changes; index marked DONE          |

## STOP conditions

Stop if references/schema changed since the captured baseline, more missing metadata exceptions appear without documented removal, input repair requires network refresh, semantic regeneration alters unrelated scope policy or catalogue membership, or three attempts fail the same gate. Do not weaken validation to publish output. Report a newly encountered document shape with the key and source line, not a guessed fallback.

## Maintenance notes

Future docs refreshes should run offline tests and `--check` after generation. Any new root type needs an explicit fixture and classification rule. Preserve the distinction between a root scalar enum and an enum-valued member. Date-only regeneration must not cause drift-check failures; formatting must never determine schema semantics.
