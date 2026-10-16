# Plan 004: Replace settings and companion files without truncating originals

> Follow the steps and gates. Stop on the conditions below. Update the index row on completion unless a dispatching reviewer owns it. No changes to snapshot restore are required.

## Status

| Field              | Value                                                                      |
| ------------------ | -------------------------------------------------------------------------- |
| Priority / finding | P1 / audit finding 4                                                       |
| Effort / risk      | M, 6–8 hours / MED                                                         |
| Depends on         | None functionally; recommended after 001–003 to serialize shared-file work |
| Category           | Correctness, tests                                                         |
| Planned at         | `7299fa2`, 2026-10-07, including existing dirty changes                    |

## Drift check — run first

Run `git diff --stat 7299fa2..HEAD -- internal/store internal/statusline/statusline.go internal/statusline/statusline_test.go internal/atomicfile README.md` and `git status --short`. Compare excerpts and `plans/baseline.json`. Plans 001/002 may have added load guards, provenance, and validation-mode handling; preserve those expected changes. Stop if the persistence operation has changed in an unexplained way. Preserve uncommitted snapshot/RTK work.

## Why this matters

`os.WriteFile` truncates the live destination before writing. A disk/write/close failure can leave a damaged settings document despite the caller reporting failure. A temporary file replaced only after successful writing keeps the old document intact on pre-replacement failures and exposes either a complete old or complete new document to readers.

## Current state

`internal/store/store.go:319` encodes the complete document before writing, so retain its ordered encoder:

```go
var buf bytes.Buffer
if err := encode(&buf, f.Data, "", "", f.order); err != nil {
    return err
}
b := append(buf.Bytes(), '\n')
if err := os.WriteFile(f.Path, b, 0o600); err != nil {
    return err
}
f.Exists = true
```

`internal/statusline/statusline.go:100` similarly marshals a Config and directly calls `os.WriteFile(path, append(b, '\n'), 0o600)`. Both currently create parent directories with 0755. New files use 0600; existing file modes are not changed by `os.WriteFile`.

The uncommitted snapshot implementation already uses a same-directory temporary file and rename (`internal/snapshot/snapshot.go:184`), but it is not a shared utility and its overwrite/restore policy differs. Read it only as context; do not alter it or reuse snapshot as a dependency.

Conventions: return errors, let callers format status messages, use `t.TempDir`, and preserve two-space indentation, unknown keys, key order, and trailing newline. The structural exemplar `internal/store/store_test.go:102` does:

```go
if err := f.Save(); err != nil { t.Fatal(err) }
if got, _ := os.ReadFile(path); string(got) != want {
    t.Fatalf("got:\n%s\nwant:\n%s", got, want)
}
```

## Commands

No installation/dependency change is needed; use the toolchain declared by `go.mod`.

| Purpose                           | Command                                                                                                                                                      | Expected     |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------ |
| New helper tests (after creation) | `go test ./internal/atomicfile -count=1`                                                                                                                     | Exit 0       |
| Persistence regression tests      | `go test ./internal/store ./internal/statusline ./internal/tui -count=1`                                                                                     | Exit 0       |
| Concurrency gate                  | `go test -race ./internal/atomicfile ./internal/store ./internal/statusline`                                                                                 | Exit 0       |
| Build / full / static             | `go build ./...`, `go test ./...`, `go vet ./...`                                                                                                            | Each exits 0 |
| Format gate                       | `gofmt -l internal/atomicfile internal/store/store.go internal/store/store_test.go internal/statusline/statusline.go internal/statusline/statusline_test.go` | No output    |

## Scope

Only modify new `internal/atomicfile/write.go` and `write_test.go`, `internal/store/store.go`/`store_test.go`, `internal/statusline/statusline.go`/`statusline_test.go`, a short README persistence note, and the index status row.

No snapshot changes, locking/conflict detection, store JSON representation changes, TUI rollback refactor, schema changes, installer changes, or new dependencies. No multi-file transaction guarantee. Do not migrate other writers merely because they use `os.WriteFile`.

## Git workflow

Use the operator's checkout or branch `advisor/004-atomically-replace-settings`. Preserve existing work and predecessor changes. Commit only when requested, with a message such as `fix: replace settings files atomically`. No push or PR without instructions.

## Chosen filesystem contract

The exported helper is `atomicfile.Write(path string, data []byte) error`. Preserve existing regular-file permission bits; new files are 0600. Resolve an existing final symlink to its regular-file referent and replace the referent, leaving the symlink intact; support relative links and symlinked parents. Refuse dangling final symlinks and non-regular destinations with an actionable error. Existing hard-link relationships and extended metadata are not promised preserved by rename; document this limitation rather than adding platform-specific copying.

Create missing parents with the existing 0755 behavior. Stage in the **resolved destination directory**, never a global temporary directory. Write all bytes, sync the staged file, close, then rename. Cleanup on every failure. Do not remove/truncate the destination as a fallback when rename fails. This promises atomic replacement on supported macOS/Linux local filesystems, not immunity to power loss after rename or network-filesystem semantics. Do not add a post-rename operation whose failure is reported as if replacement never happened.

## Steps

### Step 1: Implement the helper with deterministic failure seams

Implement the chosen contract in `write.go`. Keep exported API small; use private operation interfaces/functions passed to an internal implementation for tests. Avoid mutable global hooks. The seam must allow deterministic short-write, write-error, sync-error, close-error, and rename-error cases without root-permission assumptions or filling a disk. Preserve the first actionable error while cleaning up the temporary file.

Add `TestWriteCreatesPrivateFile`, `TestWritePreservesMode`, `TestWritePreservesOriginalOnFailure`, and `TestWriteCleansTemporaryFiles`. The failure table must compare original bytes and enumerate the directory before/after; successful writes must compare exact new bytes. Explicitly turn an incomplete write without an error into `io.ErrShortWrite` or complete it correctly. Directory creation side effects are acceptable; changing original bytes before rename is not.

**Verify:** `go test ./internal/atomicfile -run 'TestWrite(CreatesPrivateFile|PreservesMode|PreservesOriginalOnFailure|CleansTemporaryFiles)' -count=1` → all named tests and failure subcases pass.

### Step 2: Specify link/destination behavior with filesystem tests

Add `TestWritePreservesSymlink` for absolute/relative final symlinks and symlinked parents. Assert the link itself remains a link, referent bytes change completely, and the stage is not left behind. Add `TestWriteRejectsInvalidDestination` for directory, dangling link, and a portable non-regular fixture where supported. Use temporary directories; skip only link capabilities that the host explicitly lacks. Do not use chmod-only failure tests, which are unreliable under privileged execution.

**Verify:** `go test ./internal/atomicfile -run 'TestWritePreservesSymlink|TestWriteRejectsInvalidDestination' -count=1` → all supported cases pass, with any platform skip explicitly explained. Entire helper test suite passes.

### Step 3: Route the two settings writers through the helper

Replace only the final direct write in `File.Save` and `statusline.Save` with `atomicfile.Write`. Keep encoding and scope restrictions unchanged; either retain parent creation at callers or move it solely into the helper, but do not perform two independent policy implementations. `f.Exists` becomes true only after success. Do not publish altered in-memory metadata on a failed write.

Add `TestSaveAtomicIntegration` in both package test files, using valid and invalid destination fixtures to verify error propagation, original data preservation, private new-file creation, and existing modes. Existing `TestSaveKeepsKeyOrder` and `TestConfigRoundTripAndOrdering` must pass unchanged in meaning. Deterministic I/O fault coverage belongs in helper tests; do not expose test-only public setters in the callers.

**Verify:** `go test ./internal/store ./internal/statusline -run 'TestSaveAtomicIntegration|TestSaveKeepsKeyOrder|TestConfigRoundTripAndOrdering|TestRoundTrip' -count=1` → all pass. `rg -n 'os\.WriteFile' internal/store/store.go internal/statusline/statusline.go` → no matches (exit 1 is the expected search result).

### Step 4: Verify compatibility and document the bounded guarantee

Add a concise README note describing complete-file replacement, preservation of existing symlinks, rejection of dangling/non-regular destinations, and no hard-link/extended-metadata preservation guarantee. Do not promise conflict prevention or multi-file transactions. Review the helper's error handling: before successful rename, every error must leave existing destination bytes unchanged.

**Verify:** all command-table gates pass; `git diff --check` exits 0. Review new/changed files relative to starting status; only scoped implementation changes. Mark the index row DONE with the actual commands run.

## Test plan and done criteria

| Requirement                                           | Gate                                                                               |
| ----------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Pre-replacement failures retain original bytes        | All five injected failure categories pass                                          |
| New-file/mode/link policies are explicit              | Named mode/symlink/destination tests pass                                          |
| No temporary-file leaks                               | Directory enumeration assertions pass after success and failure                    |
| Existing callers use the helper without changing JSON | Search finds no direct writes in the two production writers; regression tests pass |
| Deliverable verified                                  | Build/full tests/vet/race/format pass, scoped diff, index DONE                     |

## STOP conditions

Stop if a supported filesystem cannot rename a staged file over the destination without deleting the original first, existing callers require hard-link or extended-metadata preservation beyond the chosen contract, unrelated source files must change, or three reasonable fixes fail the same gate. Do not substitute remove-then-rename, copy-over-live-file, or silent fallback to `os.WriteFile`.

## Maintenance notes

Plan 005 will add observed-revision conflict checks immediately before replacement. Keep staging and final rename cleanly separated internally to support that check. Atomic replacement alone does not detect another process's newer content. Review any future durability feature carefully because errors after a successful rename have different semantics from pre-commit errors.
