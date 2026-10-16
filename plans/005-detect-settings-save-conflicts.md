# Plan 005: Refuse saves when the loaded settings revision has changed

> Follow the steps and verification gates. Stop on the conditions below. Update the index row when done unless the dispatching reviewer owns it. This plan implements optimistic conflict detection, not a filesystem-wide transaction or a guarantee against non-cooperating writers.

## Status

| Field              | Value                                                                  |
| ------------------ | ---------------------------------------------------------------------- |
| Priority / finding | P1 / audit finding 5                                                   |
| Effort / risk      | M, 12–16 hours after accounting for companion and UI integration / MED |
| Depends on         | 001, 002, and 004 completed; 003 is independent                        |
| Category           | Correctness, tests                                                     |
| Planned at         | `7299fa2`, 2026-10-07, plus existing working-tree changes              |

## Drift check — run first

Run `git diff --stat 7299fa2..HEAD -- internal/atomicfile internal/store internal/statusline internal/tui/app.go internal/tui/env.go internal/tui/tools.go internal/tui/status.go internal/cli/statusline.go README.md` and `git status --short`. Compare the original excerpts below and predecessor contracts. Expected predecessor changes: 001 guards invalid loads; 002 adds provenance/no-execution validation and async activation; 004 introduces `atomicfile.Write` and routes settings/companion saves through it. Verify their named tests pass before proceeding. Unexplained changes are a STOP condition. Preserve baseline snapshot additions and RTK removal.

## Why this matters

The TUI holds settings loaded at startup/manual reload. Saving one value serializes that entire old map, silently discarding newer changes made by Claude or another editor. Companion status configuration has the same lifetime problem. Each loaded document should retain the observed on-disk revision; a save against a changed revision must fail visibly and leave the newer bytes intact.

The bounded guarantee is important: ordinary portable rename cannot atomically compare-and-swap against every external writer. This plan detects changes visible at a final pre-rename check and documents the remaining check-to-rename window. It does not introduce lock files that Claude would ignore or claim that passing Go's race detector proves interprocess safety.

## Current state and predecessor contracts

`internal/tui/app.go:185` loads documents once:

```go
func (m *model) reload() {
    m.files, m.errs = store.OpenAll(m.cwd)
    m.sl.cfg = nil
    m.statusSync()
}
```

`afterWrite` refreshes rows/status but does not reopen settings. `File` in `internal/store/store.go:99` stores Scope, Path, Exists, Data, and key order, but no observed disk revision. Its original `Save` encodes `Data` and directly writes it; after plan 004 it calls `atomicfile.Write` instead.

`statusline.Load` returns `Default()` when a companion is absent; `Save(path, c)` writes the entire Config. Plan 002 adds command provenance to Config, which must survive this plan. Runtime `Render` loads configuration without editing it; keep that read behavior cheap and compatible.

The environment save path (`internal/tui/env.go:131`) demonstrates staging:

```go
staged := *m.files[sc]
data, err := json.Marshal(staged.Data)
if err == nil {
    staged.Data = nil
    err = json.Unmarshal(data, &staged.Data)
}
// mutate staged; save; only then publish:
m.files[sc] = &staged
```

Tool/status activation also copy structs, while ordinary edits/status field changes currently mutate live maps/slices first. New revision metadata must survive struct copies and remain excluded from JSON. Conflict rejection must not leave an attempted change queued for an unrelated later save.

Tests use package-local table cases, `t.TempDir`, `t.Setenv`, exact byte comparisons, and real models. Follow `internal/tui/tools_test.go:79`'s failure/preservation assertions and `internal/store/store_test.go:69`'s key-order assertions. Preserve unknown keys, scope restrictions, no-execution activation validation, and `p` consent. No design document mandates automatic merging.

## Commands

Run from repository root with the declared Go toolchain. No new dependencies or installations.

| Purpose                      | Command                                                                                                    | Expected                        |
| ---------------------------- | ---------------------------------------------------------------------------------------------------------- | ------------------------------- |
| Conflict primitive           | `go test ./internal/atomicfile -run 'TestRevision                                                          | TestWriteIfUnchanged' -count=1` | Exit 0                |
| Settings/companion conflicts | `go test ./internal/store ./internal/statusline -run 'Test.*Conflict                                       | Test.*Revision                  | TestSaveKeepsKeyOrder | TestConfigRoundTripAndOrdering' -count=1` | Exit 0 |
| UI/CLI integration           | `go test ./internal/tui ./internal/cli -count=1`                                                           | Exit 0                          |
| Race checks                  | `go test -race ./internal/atomicfile ./internal/store ./internal/statusline ./internal/tui ./internal/cli` | Exit 0                          |
| Build / full / static        | `go build ./...`, `go test ./...`, `go vet ./...`                                                          | Each exits 0                    |

For formatting, run `gofmt -l` on the explicit Go scope below; expected output is empty. Use `git diff --check` as an additional exit-0 whitespace gate.

## Scope

| Area             | Only files allowed to change                                                                                                                                                                                         |
| ---------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| File replacement | `internal/atomicfile/write.go`, `internal/atomicfile/write_test.go`, new `internal/atomicfile/revision.go`, new `internal/atomicfile/revision_test.go`                                                               |
| Documents        | `internal/store/store.go`, `internal/store/store_test.go`, `internal/statusline/statusline.go`, `internal/statusline/statusline_test.go`                                                                             |
| TUI              | `internal/tui/app.go`, `internal/tui/app_test.go`, `internal/tui/env.go`, `internal/tui/env_test.go`, `internal/tui/tools.go`, `internal/tui/tools_test.go`, `internal/tui/status.go`, `internal/tui/status_test.go` |
| CLI/docs         | `internal/cli/statusline.go`, `internal/cli/statusline_test.go`, README persistence guidance, index status row                                                                                                       |

No snapshot conflict/alias changes, configuration-root overrides, schema changes, numeric parser changes, dependency changes, or automatic merge/force-save feature. Do not extract a broad TUI service layer. Minimal staging needed for correct conflict handling is in scope; unrelated architecture cleanup is not. Snapshot restore has an explicit separate overwrite policy and stays unchanged.

## Git workflow

Use the checkout/worktree containing completed predecessors and baseline dirty changes. If needed use `advisor/005-detect-settings-save-conflicts`. Do not commit unless requested; suggested message: `fix: reject saves from stale settings revisions`. No push/PR without instructions.

## Chosen API and conflict policy

Extend `internal/atomicfile` with an opaque `Revision` value, `Read(path string) ([]byte, Revision, error)`, `WriteIfUnchanged(path string, data []byte, expected Revision) (Revision, error)`, and an `ErrConflict` sentinel discoverable with `errors.Is`. Keep `Write` for callers that intentionally need unconditional replacement; ordinary settings and companion persistence will no longer use it.

A Revision binds the requested normalized path, resolved referent, existence, file identity, and content digest. Capture identity with the file descriptor used to read; detect a changing read using pre/post metadata and return an error instead of blessing a visibly unstable read. Missing files produce a tracked absent revision, nil bytes, and no read error. Store/open callers must preserve their empty/missing semantics explicitly. Do not include revision data in persisted JSON or error messages; conflicts report only a path and a reload instruction.

WriteIfUnchanged stages the new data via plan 004's helper, then compares current destination identity/path/content/existence immediately before rename. Compare content, not just mtime or size. Changed bytes, removal/recreation, creation after observing absence, symlink retargeting, or a different target path reject the save; identical unchanged content/identity is allowed. An ordinary I/O error is returned as such, never treated as permission to overwrite. Clean up the stage on rejection. Return the revision of the committed staged file from its known bytes/identity; do not reread later disk contents and accidentally adopt an external writer's revision.

No automatic merge, reload-and-retry, or implicit `--force`. A user explicitly reloads, reviews the newer state, and repeats the edit. An external write between the last check and rename remains possible and must be stated in documentation. Stop if the operator requires a strict compare-and-swap guarantee; that needs a different storage/cooperation contract.

## Steps

### Step 1: Add revision reads and a checked replacement primitive

Implement the API above around plan 004's staging/rename boundary. Reuse its symlink/destination policy. Keep the final comparison after writing/syncing/closing the stage to minimize the unchecked interval. Add a private test hook through injected operations, not an exported hook or mutable global, to change the destination just before the final comparison.

Add `TestRevisionRead`, `TestWriteIfUnchangedConflict`, and `TestWriteIfUnchangedSuccess`. Cover absent/present files, changed content of the same length, deletion, creation, replacement with identical bytes but different identity, and symlink retargeting. Deterministically change a file after staging but before comparison and require `errors.Is(err, ErrConflict)`, unchanged external bytes, and no leaked stage. Two independently loaded revisions must demonstrate first save succeeds/second stale save fails. Freshly returned revisions must permit a second save by the same writer.

**Verify:** `go test ./internal/atomicfile -run 'TestRevision|TestWriteIfUnchanged' -count=1` → all named scenarios pass. `go test ./internal/atomicfile -count=1` → all plan 004 fault/mode/link tests still pass.

### Step 2: Bind settings and companion saves to their loaded revisions

Add unexported revision state to `store.File` and `statusline.Config`; update Open/Load to use `atomicfile.Read` and retain its observation. A failed parse must not produce a writable trusted revision. Save checks that the revision belongs to the same requested path, uses `WriteIfUnchanged`, and advances revision/Exists only on success. Struct copies preserve immutable revision state.

A `Default()` Config with no load observation may create a new missing companion, but must refuse to replace an existing one: require `Load` before editing existing contents. After a successful first save, repeated saves of the same Config are supported. A manually constructed File without a read observation must require Open rather than implicitly overwriting. Existing test fixtures that intentionally write raw initial files should use `os.WriteFile` in their temporary setup or Load first, not weaken the production policy.

Add `TestFileSaveConflict`, `TestConfigSaveConflict`, and `TestRevisionAdvancesAfterSave`. Exercise unchanged files, external writes, deletion/recreation, absent-to-present, same bytes/different identity, and repeat saves. Ensure JSON excludes metadata and key order survives. Add a non-mutating `File.CheckUnchanged() error` and equivalent `statusline.CheckUnchanged(path, cfg) error` for activation preflight; these are advisory checks and Save still performs the final check.

**Verify:** the settings/companion command-table gate passes; `go test ./internal/store ./internal/statusline -count=1` → all existing runtime/render/load tests pass. No conflict error prints file contents.

### Step 3: Make TUI conflict rejection leave no attempted edit in memory

For ordinary form submission/cycling/removal, stage a private Data map and copy the File before changing it, retaining revision and key-order state. A small `File.Clone()` helper is acceptable if it recursively copies `map[string]any`, `[]any`, `[]string`, and metadata while retaining immutable primitives; do not JSON-roundtrip to introduce more numeric conversions. Env/tools can reuse it where that removes duplicate copy logic. Do not otherwise change decoding/numeric behavior.

For status field/placement/color/reorder edits, deep-copy Config's mutable fields before changing them, keep its revision, save the copy, then publish. Retain plan 002 activation's own staged/pending flow. On conflict, keep the last loaded state in memory and show `File changed on disk; press r to reload, then retry` with the destination path. Do not call `afterWrite` as if successful and do not clear the observation. Explicit reload resets observations and restores normal editing; it discards the rejected attempted edit.

Add `TestTUISettingsConflict` and `TestTUIStatusConfigConflict`: externally modify a temp file after model load, attempt a change, assert external disk bytes survive and attempted model changes do not appear. Cover ordinary edits, unset/cycle, env, tools, and companion fields/reorder. Then reload and successfully perform a different edit; assert it retains the external change and does not include the rejected edit. Use existing form/message helpers; avoid sleeps.

**Verify:** `go test ./internal/tui -run 'TestTUISettingsConflict|TestTUIStatusConfigConflict|TestOrdinaryEdits|TestToolToggle|TestEnvironment' -count=1` → all pass. Run the complete TUI suite to retain plan 002 consent/pending behavior.

### Step 4: Preflight both activation documents and report conflicts accurately

After no-execution validation succeeds and before saving either activation document, use both CheckUnchanged operations on settings and companion in CLI `statusline on` and TUI activation completion. If either is already stale, save neither. Keep per-save final checks as well; preflight is not a transaction. If the companion save succeeds but the subsequent settings save encounters a later conflict, do not automatically roll back over possible newer external content. Report which document saved and which conflicted, and do not claim activation completed. The existing two-file ordering remains companion first, then settings.

Add `TestStatusActivationConflict` covering settings stale before preflight, companion stale before preflight, and a deterministically injected settings conflict after companion commit. Tests must assert the exact bounded outcome and preserved newer settings bytes. Keep tests local and preserve no-execution validation; do not use a real project command to orchestrate races. If a seam is needed, make it an unexported injected save operation in the relevant package, with no live-model access from goroutines.

**Verify:** `go test ./internal/cli ./internal/tui -run 'TestStatusActivationConflict|TestStatusActivationPreservesConsent|TestStatuslineCheckIntegration' -count=1` → all pass; stale preflight cases write neither document and later-conflict cases report partial state honestly.

### Step 5: Document the guarantee and run integrated verification

Document observable external-change refusal, explicit reload/retry, companion coverage, and the remaining final-check/rename window with non-cooperating writers. State that status activation still updates two files separately. Do not describe this as locking or a multi-file transaction. Update errors/comments so future maintainers cannot mistake a preflight check for the final protection.

**Verify:** all command-table gates pass; `gofmt -l` over scoped Go files prints nothing; `git diff --check` exits 0. `rg -n 'atomicfile\.Write\(' internal/store/store.go internal/statusline/statusline.go` → no matches (exit 1 expected). Review changed/untracked paths relative to starting status and mark the index row DONE with the actual gates and documented limitation.

## Test plan and done criteria

| Requirement                                                               | Gate                                                                                       |
| ------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| Detect content/existence/identity/path changes visible before replacement | Primitive and document conflict tables pass, including deterministic post-staging mutation |
| Advance revisions only after success                                      | Repeated successful save passes; stale copies fail without changing external bytes         |
| No rejected edit leaks through UI state                                   | TUI conflict/reload/retry tests pass for ordinary, env, tools, and companion mutations     |
| Activation handles two-file conflicts honestly                            | Both preflight and later-conflict tests pass without inherited execution                   |
| Bounded verified deliverable                                              | Build/full tests/vet/race/format pass; documented residual race; scoped diff; index DONE   |

## STOP conditions

Stop for unmet predecessors, unexplained source drift, any proposal to silently refresh-and-overwrite on conflict, a requirement for strict external-writer compare-and-swap, an out-of-scope change, or three failed attempts at the same gate. Report the doubtful assumption. Do not implement an unreliable timestamp-only comparison or add stale lock-file recovery as an unplanned workaround.

## Maintenance notes

Revision metadata is process state, not serialized user configuration. Future clone/load/save paths must retain or intentionally establish it. Review post-rename revision updates and partial activation errors carefully. Minimal staging overlaps audit finding 8 as a necessary part of conflict handling; record the covered paths rather than claiming every possible failed-write scenario is fixed. Numeric precision, snapshot aliases, and custom configuration roots remain deferred.
