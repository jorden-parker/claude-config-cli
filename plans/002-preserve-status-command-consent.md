# Plan 002: Validate status-line activation without running inherited commands

> Follow the steps and verification gates. Stop on the conditions below. Update the index row when done unless the dispatching reviewer owns it. This is defensive maintenance; tests must use harmless commands in temporary directories.

## Status

| Field              | Value                                                               |
| ------------------ | ------------------------------------------------------------------- |
| Priority / finding | P1 / audit finding 2, including its related event-loop stall        |
| Effort / risk      | M, 8–12 hours after specifying integration coverage / MED           |
| Depends on         | None functionally; execute after 001 to avoid overlapping TUI edits |
| Category           | Security, correctness, performance, tests                           |
| Planned at         | `7299fa2`, 2026-10-07, including existing dirty changes             |

## Drift check — run first

Run `git diff --stat 7299fa2..HEAD -- internal/statusline internal/cli/statusline.go internal/tui/app.go internal/tui/status.go internal/tui/status_test.go README.md` and `git status --short`. Compare excerpts and `plans/baseline.json`. Changes from completed plan 001 to ordinary-edit load guards are expected and must be preserved; unexplained changes to activation/execution are a STOP condition. Existing snapshot/RTK working changes must also survive.

## Why this matters

The TUI promises not to preview repository-provided status commands until the user presses `p`. Activation instead saves the inherited command and executes the real renderer to check it, running that command without the preview action. Wrapping a project command in a user companion file also loses its original trust classification. Validation currently runs synchronously and can freeze input for about three seconds. This plan separates validation from execution, persists provenance conservatively, and makes activation cancellable while validation runs.

## Current state and constraints

`internal/tui/status.go:70` trusts only user/managed origins. The ordinary preview gate is:

```go
if !force && (command == m.sl.ranFor || !m.statusTrusted()) {
    return nil
}
```

But activation at `internal/tui/status.go:246` does:

```go
if err = statusline.Enable(&staged, &cfg, statusline.Command(sc, m.cwd), m.sl.kept); err == nil {
    err = statusline.Save(m.statusPath(), &cfg)
}
if err == nil {
    err = statusCheck(statusline.Command(sc, m.cwd), m.cwd)
}
```

`internal/statusline/install.go:84` calls `RunInherited(command, Sample(...))`; `internal/statusline/statusline.go:309` then runs `c.Inherited`. `Existing` at `install.go:107` unwraps a renderer but returns the wrapper's scope, losing the original command source. The CLI activation path repeats this call chain.

The Config holds `Fields`, `Separator`, `Color`, `Existing`, `Inherited`, and `Previous`. `Previous` is the original `statusLine` value for disabling; do not reinterpret it as a trust token. Real renderer execution intentionally runs the configured command; this plan must preserve that feature and CLI `statusline preview`'s explicitly documented execution behavior.

Bubble Tea work belongs in commands, as `internal/tui/status.go:84` demonstrates:

```go
input := statusline.Sample(m.cwd, time.Now())
return func() tea.Msg {
    out, err := statusline.RunInherited(command, input)
    return statusPreviewMsg{command: command, out: out, err: err}
}
```

`model.Update` handles typed completion messages before mode-specific input. Tests use `t.TempDir`, `t.Setenv`, `selectKey`, and the `run(m, cmd)` helper in `internal/tui/status_test.go:41`. `statusModel` replaces `statusCheck` with a stub: keep that convenience for unit cases, but do not use the stub as evidence that the real activation chain is safe.

## Commands

No installs required. Use Go 1.27.1 or the declared compatible toolchain; run from repo root.

| Purpose               | Command                                                                                                                                                                                                                                              | Expected                |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------- |
| Runtime/CLI tests     | `go test ./internal/statusline ./internal/cli -count=1`                                                                                                                                                                                              | Exit 0                  |
| TUI tests             | `go test ./internal/tui -count=1`                                                                                                                                                                                                                    | Exit 0                  |
| Concurrency gate      | `go test -race ./internal/statusline ./internal/cli ./internal/tui`                                                                                                                                                                                  | Exit 0, no race reports |
| Build / full / static | `go build ./...`, `go test ./...`, `go vet ./...`                                                                                                                                                                                                    | Each exits 0            |
| Format gate           | `gofmt -l internal/statusline/install.go internal/statusline/statusline.go internal/statusline/statusline_test.go internal/cli/statusline.go internal/cli/statusline_test.go internal/tui/app.go internal/tui/status.go internal/tui/status_test.go` | No output               |

## Scope

Modify only the eight Go files in the format gate (`internal/cli/statusline_test.go` is new), README's status-line subsection, and the index status row. No dependency changes, unrelated CLI changes, field catalogue changes, or layout redesign.

Do not change actual runtime renderer timeout/output behavior, `-C` command cwd behavior (separate finding), snapshot restoration, or other preview features. Provenance metadata and the read-only validation mode are within scope. Do not introduce a shell script interpreter/parser.

## Git workflow

Use the supplied checkout or branch `advisor/002-preserve-status-command-consent`. Preserve existing work and plan 001 changes. Do not commit unless asked; suggested message if requested: `fix: preserve status command consent during activation`. No push/PR without instructions.

## Steps

### Step 1: Add a renderer validation mode that cannot execute inherited commands

Add an internal-use `--check` Boolean to `statusline render`. In this mode, read the input/config and compose ccfg's own fields only; never call `RunInherited` for the saved inherited command. Introduce a small exported function such as `CheckRender(configPath string, input []byte) error` in the statusline package so validation can reject a malformed existing companion rather than silently treating it as defaults. Missing config is valid for first activation. Normal `Render` keeps its current always-success/fallback behavior.

Change `Check(command, cwd)` to run only the generated renderer command with the static ` --check` suffix. All existing callers pass `statusline.Command(...)`; document that precondition. Never interpolate an inherited command into this suffix or fallback to a normal render when an older installed binary rejects the flag. Preserve the actionable install/update error. Perform this check **before** publishing the new companion in both CLI and TUI activation; it checks renderer availability/compatibility, not arbitrary shell-command health.

Add `TestCheckRenderDoesNotRunInherited`, `TestCheckRenderRejectsInvalidConfig`, and a CLI subprocess integration test `TestStatuslineCheckIntegration`. Build the real CLI once into `t.TempDir()` using `go build -o <temporary binary> ./cmd/ccfg` with the subprocess directory set to repo root; execute only against temporary HOME/project paths. Configure a harmless marker-producing inherited fixture and assert `render --check` leaves the marker absent, while ordinary rendering still runs it. Exercise `statusline on` through that same binary; check the marker remains absent and settings point at the normal renderer command without a permanently appended `--check`.

**Verify:** `go test ./internal/statusline ./internal/cli -run 'TestCheckRender|TestStatuslineCheckIntegration|TestRenderRunsInherited' -count=1` → all pass. Unsupported validation mode must be an error, never a fallback that executes the inherited command.

### Step 2: Preserve original command provenance without trusting project metadata

Add optional `InheritedScope store.Scope` metadata to Config, encoded as `inheritedScope`. Extend `Enable` to accept the fallback command's origin in addition to its string; update its two production callers and package tests. A direct command in the target file gets that file's scope. A fallback command keeps the scope returned by `Existing`. Re-enabling an existing wrapper must preserve its captured origin.

Update `Existing`'s unwrap branch to return the inherited origin, conservatively bounded by the wrapper location: project/local wrappers remain untrusted even if their companion claims `user` or `managed`. For user/managed wrappers, accept only recognized persisted origin values; absent/invalid legacy provenance becomes unknown and requires `p` for an automatic TUI preview. A malformed/unreadable companion must not confer trust. Direct non-wrapper user/managed commands retain automatic preview behavior. Keep lookup precedence and `Previous` restoration unchanged.

Add `TestInheritedScopeRoundTrip` and `TestWrappedCommandTrust` covering project-to-user inheritance before/after reload, forged trusted metadata inside a project companion, direct user commands, and legacy companions missing the field. Update details text to say `origin unknown` rather than a blank source for legacy data. Do not persist an `approved: true` bit in repository-controlled files.

**Verify:** `go test ./internal/statusline ./internal/tui -run 'TestInheritedScopeRoundTrip|TestWrappedCommandTrust|TestExistingFollowsOurOwnRenderer|TestEnable' -count=1` → all pass; wrapped project commands never become automatically trusted on reload.

### Step 3: Move activation validation into a cancellable command

In `statusToggleUse`, prepare private copies and capture target scope, path, cwd, command, and a monotonically increasing request ID. Return a `tea.Cmd` that performs only `statusCheck` and emits a typed activation-result message; it must not access/mutate the live model or write settings. Copy relevant slices/maps before capture. The event loop owns all eventual persistence.

While activation validation is pending, use a bounded pending mode/state: render `Checking status line… Esc cancels`, continue handling resize, allow Esc cancellation and quit/Ctrl+C, and refuse other mutation/scope/reload actions. This avoids changing the target underneath the request. Handle the result before mode dispatch: ignore stale/cancelled IDs; on error return to browsing without saving; on success save companion then staged settings using existing error handling, publish model state only for the matching active request, and leave pending state. Disabling can keep its existing synchronous non-command path. Keep the save order explicit; this plan does not promise an all-or-nothing two-file transaction.

Adapt existing tests that assumed `pressTab` finishes activation synchronously: capture returned commands and deliver their messages via `run`. Add `TestStatusActivationPending` with channel-controlled stub validation, proving `Update` returns while the stub is blocked, resize is handled, mutations are refused, cancellation leaves bytes unchanged, and a late result cannot activate. Avoid arbitrary sleeps; use bounded test timeouts only to detect a hang. No goroutine may mutate model state.

**Verify:** `go test ./internal/tui -run 'TestStatusActivationPending|TestStatusLineRefusesACommandThatDoesNotRun|TestStatusLineKeepsExisting' -count=1` → all pass. `go test -race ./internal/tui` → no races.

### Step 4: Test the trust boundary through activation and document it

Add `TestStatusActivationPreservesConsent`: use temporary project/local settings with a harmless marker command and the production `statusline.Check` wired to the temporary built CLI via the test PATH/program setup. Cover activation in project scope and default user scope inheriting the project command. Drain returned activation/preview commands and reload the model; the marker must stay absent until explicit `p`. Include hidden inherited output, direct trusted user preview, and ordinary runtime rendering compatibility. Do not globally mock away the check in this regression test.

Document that activation validates the renderer without running the saved command; `p` explicitly runs an untrusted/unknown-origin preview, and actual Claude status rendering runs the saved command normally. Legacy companions lacking provenance may require `p` until reconfigured. Keep current backup/disable behavior.

**Verify:** `go test ./internal/tui -run 'TestStatusActivationPreservesConsent|TestStatusLineDoesNotAutoRunProjectCommands' -count=1` → all pass. Run all command-table gates → pass/no formatting output. Review changed files against starting status; only scoped implementation edits belong to this plan.

## Test plan and done criteria

| Requirement                                                      | Gate                                                                                 |
| ---------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| Validation and real CLI activation do not run inherited commands | `TestCheckRender*` and `TestStatuslineCheckIntegration` pass using the real binary   |
| Wrapping/reloading does not elevate project/local trust          | Provenance and activation-consent tests pass, including forged/legacy metadata cases |
| Validation does not block model updates or apply stale results   | Channel-controlled pending/cancellation test and race tests pass                     |
| Real rendering and disable restoration remain compatible         | Existing statusline/TUI tests, full build/tests/vet pass                             |
| Bounded deliverable                                              | Format gate empty; only scoped changes; index row DONE with results                  |

## STOP conditions

Stop for unexplained drift, any path where validation falls back to executing an inherited command, a test that needs real user configuration or external programs beyond Go/the system shell, an out-of-scope change, or three failed repair attempts. If provenance cannot be established, retain unknown/untrusted status; do not infer trust from a command string or file-name substring.

## Maintenance notes

New activation paths must use the no-execution check. Persist command provenance whenever wrapping; wrapper scope alone is insufficient. The `--check` mode is an executable compatibility contract, so older binaries should fail explicitly. Keep test globals serial and restore them with cleanup. Plans 004/005 will change persistence and must retain these activation and consent gates.
