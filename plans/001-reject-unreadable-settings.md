# Plan 001: Refuse edits to settings that failed to load

> Follow each step and its verification gate. Stop on the conditions below. Update this plan's row in `plans/README.md` when finished, unless the dispatching reviewer owns the index. This document authorizes no push, merge, or deployment.

## Status

| Field              | Value                                                            |
| ------------------ | ---------------------------------------------------------------- |
| Priority / finding | P1 / audit finding 1                                             |
| Effort / risk      | S, 3–4 hours / LOW                                               |
| Depends on         | None                                                             |
| Category           | Correctness, tests                                               |
| Planned at         | `7299fa2`, 2026-10-07, including the existing dirty working tree |

## Drift check — run first

Run `git diff --stat 7299fa2..HEAD -- internal/store internal/tui/app.go internal/tui/app_test.go internal/cli/root.go` and `git status --short`. Compare the excerpts below with live code. The planning baseline already contained uncommitted snapshot additions and RTK deletions; preserve them. HEAD alone is not the complete baseline. `plans/baseline.json` records content hashes for additional drift checking. Do not reset, stash, or discard unrelated work. Stop if the affected control flow differs for an unexplained reason.

## Why this matters

An unreadable or malformed settings file produces a `File` plus an error. Ordinary TUI mutations ignore that error and can replace the user's original document with an empty or partial map. A file containing JSON `null` currently loads successfully and later panics on map assignment. After this change, invalid documents remain untouched and the user gets a path-qualified error until a successful reload.

## Current state

`internal/store/store.go:110` initializes an empty map and accepts empty/missing files. At line 123:

```go
if err := json.Unmarshal(b, &f.Data); err != nil {
    return f, fmt.Errorf("%s: %w", f.Path, err)
}
```

JSON `null` sets `f.Data` to nil without returning an error. `Set` eventually assigns into that map. `OpenAll` retains the returned file alongside errors indexed by scope.

`internal/tui/app.go:510` submits ordinary edits without checking the scope's load error:

```go
sc := m.targetScope(st)
f := m.files[sc]
if err := f.Set(st.Key, v); err != nil {
    m.setStatus(err.Error(), true)
    return m, nil
}
if err := f.Save(); err != nil {
    m.setStatus("Save failed: "+err.Error(), true)
    return m, nil
}
```

`cycleValue` and `doUnset` have the same gap. Use the existing convention in `internal/tui/env.go:127`:

```go
if err := m.errs[sc]; err != nil {
    m.setStatus("Can't edit a settings file that failed to load: "+err.Error(), true)
    return nil
}
```

Tests are package-local Go tests with `t.TempDir`/`t.Setenv`. `internal/tui/app_test.go:23` demonstrates `newModel`, `selectKey`, and `pressTab`; `internal/tui/env_test.go:95` demonstrates driving a form with Bubble Tea messages. Keep global keys routed through `targetScope`, managed files read-only, and unknown keys/key order preserved. Missing and whitespace-only files intentionally behave as empty objects today; retain that behavior. No design/ADR overrides exist for this change.

## Commands

Run from the repository root using the Go version declared in `go.mod` (baseline 1.27.1). No dependency install is required.

| Purpose         | Command                                                                                                      | Expected                                    |
| --------------- | ------------------------------------------------------------------------------------------------------------ | ------------------------------------------- |
| Focused tests   | `go test ./internal/store ./internal/tui -count=1`                                                           | Exit 0                                      |
| Build/typecheck | `go build ./...`                                                                                             | Exit 0; no binary in repository root        |
| Full tests      | `go test ./...`                                                                                              | Exit 0                                      |
| Static checks   | `go vet ./...`                                                                                               | Exit 0                                      |
| Format check    | `gofmt -l internal/store/store.go internal/store/store_test.go internal/tui/app.go internal/tui/app_test.go` | No output after formatting changed Go files |

## Scope

Only modify `internal/store/store.go`, `internal/store/store_test.go`, `internal/tui/app.go`, `internal/tui/app_test.go`, and the status row in `plans/README.md`. Other files above are read-only exemplars.

Do not modify schema, snapshot/restore, env/tool semantics, CLI flags, dependency manifests, or unrelated TUI layout. Atomic replacement, numeric precision, external-change conflicts, and general rollback of failed edits belong to other plans/findings.

## Git workflow

Use the checkout/worktree supplied by the operator. If a new branch is needed, use `advisor/001-reject-unreadable-settings`. Preserve starting dirty changes; an isolated worktree must include them rather than silently auditing a HEAD-only version. Do not commit unless requested; if requested use Conventional Commits, such as `fix: reject edits to unreadable settings`, matching repository history. Do not push or open a PR without instructions.

## Steps

### Step 1: Specify invalid-document behavior in store tests

Add `TestOpenRejectsInvalidDocuments` with named cases for `null`, array, scalar, malformed JSON, and invalid trailing JSON. Assert a non-nil path-qualified error; do not assert the partial decoded data. Add `TestOpenAcceptsEmptyDocuments` for missing, zero-byte, whitespace-only, and `{}` files; accepted files must have a writable non-nil map. Use a project scope under `t.TempDir`; never open the real home settings.

**Verify:** `go test ./internal/store -run 'TestOpen(RejectsInvalidDocuments|AcceptsEmptyDocuments)' -count=1` → before the fix, the null case fails an assertion (not compilation); accepted-document cases pass.

### Step 2: Reject a null document without changing valid decoding

After successful JSON unmarshalling, reject `f.Data == nil` with `expected a JSON object` including `f.Path`. Keep a non-nil empty map in the returned error-bearing File for safe read-only display. Preserve the existing parse error, missing-file, and empty-file paths. Do not change numeric decoding or key-order scanning in this plan.

**Verify:** rerun Step 1's command → all cases pass; `go test ./internal/store -run 'TestRoundTrip|TestSaveKeepsKeyOrder|TestAllowed' -count=1` → all pass.

### Step 3: Guard all ordinary TUI mutation entry points

Add a small model helper, for example `editableFile(sc store.Scope) (*store.File, bool)`, that checks `m.errs[sc]`, verifies a non-nil file, and sets the established error status before returning failure. It must not reload or write anything. Use it in `startEdit`, in completed ordinary `updateEdit` before `Set`, in ordinary `cycleValue` before mutation, and in `doUnset` before `Unset`. Do not rely only on opening the form: retain the submit-time guard. Keep existing `Allowed` checks and specialized env/tool/status branches.

Add `TestOrdinaryEditsRejectLoadErrors`: table-drive form opening, completed form submission, cycling, and unsetting against a malformed file. For submit-time coverage, open a valid form first, then install a load error and the invalid fixture before completing it; use a fixture whose prior model map contains the selected key so removal can be exercised. Compare disk bytes and the model map before/after; require an error status. Include user and global target routing, plus a directory-as-file read error. Use `store.Format` for map comparison and raw bytes for disk comparison.

**Verify:** `go test ./internal/tui -run 'TestOrdinaryEditsRejectLoadErrors|TestTabCyclesSingleValueSettings|TestEscCancelsEdit' -count=1` → all pass, all four mutation subcases run, no panic, no file changes.

### Step 4: Verify recovery and integration

Add `TestOrdinaryEditsResumeAfterReload`: start with a broken file, demonstrate rejection, replace it with a valid temporary fixture, call the model's existing reload path, and perform one edit. Assert the edit persists and a sibling unknown key survives. Repairing disk alone must not clear the stored error until reload succeeds.

**Verify:** `go test ./internal/store ./internal/tui -count=1`, `go build ./...`, `go test ./...`, and `go vet ./...` → all exit 0. Run the format-check command above → no output. Review `git diff --name-only` and `git ls-files --others --exclude-standard` against starting status; only scoped additions/edits belong to this plan.

## Test plan and done criteria

| Required assertion                                                              | Gate                                                                                |
| ------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| Null and invalid roots return errors; missing/empty inputs remain writable      | Named store tests in Step 1 pass                                                    |
| Every ordinary mutation refuses a failed-load scope without altering disk/model | `TestOrdinaryEditsRejectLoadErrors` passes                                          |
| Successful reload permits editing and preserves siblings                        | `TestOrdinaryEditsResumeAfterReload` passes                                         |
| Existing behavior stays compatible                                              | Build, full tests, vet, and format check pass                                       |
| Work is bounded                                                                 | Only scoped implementation changes; index row marked DONE with verification results |

## STOP conditions

Stop and report if the relevant excerpts drift unexpectedly, the caller no longer retains load errors per scope, a fix requires an out-of-scope file, or three reasonable attempts fail the same gate. Name the doubtful assumption rather than weakening assertions. Do not normalize arbitrary invalid JSON into `{}` to make tests pass.

## Maintenance notes

Review every future mutation entry point for the same guard. Keep parse/load failures distinct from write failures and external-change conflicts. Plans 004 and 005 will alter persistence, but must preserve these rejection tests. General failed-write model rollback is not claimed solved here.
