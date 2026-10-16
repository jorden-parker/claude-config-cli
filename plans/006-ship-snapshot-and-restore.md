# Plan 006: Ship `ccfg snapshot` and `ccfg restore` with alias-safe restore

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**:
> `git diff --stat 0d715d5..HEAD -- internal/snapshot internal/cli/snapshot.go internal/cli/snapshot_test.go internal/cli/root.go README.md`
> The snapshot files are expected to be **untracked** (never committed) at the
> time of writing, so `git diff` shows nothing for them. Instead compare the
> "Current state" excerpts below against the live files. On a mismatch, treat
> it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M (4–6 hours)
- **Risk**: LOW
- **Depends on**: none. Serialize with plans 004 and 005 if they are in
  progress, because they touch `internal/store` and the same README.
- **Category**: direction (feature completion) + bug
- **Planned at**: commit `0d715d5`, 2026-10-07, **plus the uncommitted working
  tree** (the snapshot feature and the RTK removal both live only in the
  working tree).

## Why this matters

Users want to delete `~/.claude` and the project `.claude` directory to reset
Claude Code, then get back the settings they edited with `ccfg`. The working
tree already contains a complete, tested `ccfg snapshot FILE` and
`ccfg restore FILE` implementation, but it is uncommitted and the deep audit
of 2026-10-07 found one real defect in it (finding 13): restore deduplicates
destinations by **path string**, not by **physical file**. When the project
directory is a symlink to the home directory (or any two scopes resolve to the
same file through a symlinked parent), a non-forced restore passes its
preflight, writes the first copy, then fails on the second with "file
exists", leaving a partial restore. With `--force` and differing contents, Go's
randomized map order picks the winner.

After this plan: the alias case is detected before any write, the two commands
are documented in the README command list, and the feature is in a state the
user can commit on its own, separate from the unrelated RTK removal that
shares the working tree.

## Current state

### Files

- `internal/snapshot/snapshot.go` (197 lines, untracked) — `Save` and
  `Restore`. Archive format is `{"version":1,"project":"<abs path>","files":{key: rawBytes}}`.
- `internal/snapshot/snapshot_test.go` (178 lines, untracked) — 6 tests, all
  passing. Pattern to copy for new tests.
- `internal/cli/snapshot.go` (45 lines, untracked) — cobra commands
  `snapshotCmd()` and `restoreCmd()`.
- `internal/cli/snapshot_test.go` (55 lines, untracked) — `TestSnapshotCLI`,
  end-to-end through `Root()`.
- `internal/cli/root.go` (modified) — line 69 registers `snapshotCmd(), restoreCmd()` and drops `rtkCmd()`.
- `README.md` (modified) — new "## Snapshot and restore" section at ~line 224;
  the "## CLI" command block at lines 197–215 does **not** list the two new
  commands yet.
- `internal/store/store.go:32` — `var Writable = []Scope{ScopeUser, ScopeProject, ScopeLocal, ScopeGlobal}`; `store.Path(scope, cwd)` at line 55 maps scopes to `~/.claude/settings.json`, `<cwd>/.claude/settings.json`, `<cwd>/.claude/settings.local.json`, `~/.claude.json`.
- `internal/statusline/statusline.go:72` — `ConfigPath(scope, cwd)` puts
  `ccfg-statusline.json` / `ccfg-statusline.local.json` beside the settings
  file for that scope.

### Key excerpt: how destinations are collected today

`internal/snapshot/snapshot.go:134-160` (inside `Restore`):

```go
	targets := paths(project)
	destinations := map[string][]byte{}
	for key, data := range a.Files {
		path, ok := targets[key]
		if !ok {
			return 0, fmt.Errorf("unknown snapshot file %q", key)
		}
		if !validObject(data) {
			return 0, fmt.Errorf("snapshot file %s: expected a JSON object", key)
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return 0, err
		}
		if previous, exists := destinations[path]; exists && !bytes.Equal(previous, data) {
			return 0, fmt.Errorf("snapshot contains conflicting contents for %s", path)
		}
		destinations[path] = data
		info, err := os.Lstat(path)
		if err == nil {
			if !info.Mode().IsRegular() {
				return 0, fmt.Errorf("%s: refusing to replace a non-regular file", path)
			}
			if !force {
				return 0, fmt.Errorf("%s already exists; use --force to replace saved files", path)
			}
		} else if !os.IsNotExist(err) {
			return 0, err
		}
	}
	count := 0
	for path, data := range destinations {
		if err := writeFile(path, data, force); err != nil {
			return count, fmt.Errorf("restored %d files before failing at %s: %w", count, path, err)
		}
		count++
	}
```

The defect: `filepath.Abs` only cleans the string. `/tmp/link/.claude/settings.json` and `/home/u/.claude/settings.json` are different keys in `destinations` even when `/tmp/link -> /home/u`.

### Key excerpt: `paths()` keys

`internal/snapshot/snapshot.go:22-32`:

```go
func paths(project string) map[string]string {
	result := map[string]string{}
	for _, scope := range store.Writable {
		result[string(scope)] = store.Path(scope, project)
		if scope != store.ScopeGlobal {
			result[string(scope)+"-statusline"] = statusline.ConfigPath(scope, project)
		}
	}
	return result
}
```

Keys: `user`, `project`, `local`, `global`, `user-statusline`,
`project-statusline`, `local-statusline`. Seven total. Do not rename them;
they are the archive format.

### Key excerpt: `Save` already resolves symlinks for its own location

`internal/snapshot/snapshot.go:50-54` uses `filepath.EvalSymlinks` on the
parent of the snapshot file. Use the same function for the fix; it is the
repo's existing approach.

### Conventions

- Errors: `fmt.Errorf` with lowercase messages, `%w` for wrapping. Match
  existing messages in `Restore`.
- Tests: table-less, `t.TempDir()`, `t.Setenv("HOME", ...)` and
  `t.Setenv("USERPROFILE", ...)` via `setup(t)` helper in
  `internal/snapshot/snapshot_test.go:11-17`. `put(t, path, data)` writes a
  file. Symlink tests already exist (see `TestSnapshotLocationAndMissingSettings`, line ~140).
- Formatting: `gofmt`. Pre-commit runs `gofmt -w` and `prettier` through
  lint-staged (`.lintstagedrc`).
- Commit messages: conventional commits, e.g. `feat: add ccfg rtk to route command output through rtk`, `fix: reject edits to unreadable settings`.
- README style: short paragraphs, commands in fenced `sh` blocks with
  trailing `# comment`.

## Commands you will need

| Purpose   | Command                                                     | Expected on success      |
| --------- | ----------------------------------------------------------- | ------------------------ |
| Build     | `go build ./...`                                            | exit 0                   |
| Vet       | `go vet ./...`                                              | exit 0                   |
| Format    | `gofmt -l .`                                                | prints nothing           |
| Tests     | `go test ./... -count=1`                                    | all `ok`                 |
| Pkg tests | `go test ./internal/snapshot -count=1 -v`                   | all PASS incl. new tests |
| Race      | `go test -race ./internal/snapshot ./internal/cli -count=1` | ok                       |

Verified during planning: build, vet, gofmt, and full tests all pass on the
current working tree.

## Scope

**In scope** (the only files you should modify):

- `internal/snapshot/snapshot.go`
- `internal/snapshot/snapshot_test.go`
- `README.md` — only the `## CLI` command block (add two lines) and, if needed,
  one sentence in `## Snapshot and restore`.
- `plans/README.md` — status row only.

**Out of scope** (do NOT touch, even though they look related):

- `internal/cli/snapshot.go`, `internal/cli/snapshot_test.go`,
  `internal/cli/root.go` — already correct; no change needed.
- `internal/store/store.go`, `internal/statusline/statusline.go` — plans 004
  and 005 own these.
- Honoring `CLAUDE_CONFIG_DIR` (audit finding 7). Separate plan; do not add it
  here.
- Anything about RTK (`internal/rtk`, `internal/tui/rtk.go`, TUI palette). The
  RTK removal is a separate, already-made change in the same working tree.
  Leave it exactly as it is.
- Making restore an all-or-nothing transaction. Rejected in the audit;
  per-file partial restore after a genuine I/O failure is documented and
  intended.
- Rewriting paths embedded in settings values on restore. By design.
- Adding snapshot to the interactive TUI.

## Git workflow

- Branch: `advisor/006-ship-snapshot-and-restore`, created from the current
  working tree (not from HEAD; HEAD lacks the feature). If you are in an
  isolated worktree based on HEAD only, the snapshot files will be missing:
  STOP.
- Do NOT commit, push, or open a PR unless the operator instructed it.
- If the operator asks you to commit, make **two** commits so the history
  stays reviewable:
  1. `refactor: remove the RTK command and editor section` — the deletions
     (`internal/rtk/`, `internal/cli/rtk.go`, `internal/tui/rtk.go`,
     `internal/tui/rtk_test.go`), the `internal/tui/*.go` edits, the RTK
     hunks of `internal/cli/root.go`, and the RTK hunks of `README.md`.
  2. `feat: add ccfg snapshot and restore` — everything else from this plan
     plus the snapshot hunks of `root.go` and `README.md`. Use
     `git add -p` to split `root.go` and `README.md`. `docs/` and `plans/`
     are the operator's to commit or not; leave them unstaged.

## Steps

### Step 1: Add a `physical` path helper

In `internal/snapshot/snapshot.go`, add an unexported function that returns
the canonical destination for a path that may not exist yet:

```go
// physical resolves symlinks in the longest existing ancestor of path and
// rejoins the missing tail, so two aliases of one file compare equal even
// before the file exists.
func physical(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	existing, tail := path, ""
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			break // reached the root without finding anything
		}
		tail = filepath.Join(filepath.Base(existing), tail)
		existing = parent
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, tail), nil
}
```

Note: `filepath.Join("x", "")` returns `"x"`, so an empty tail is safe.

**Verify**: `go build ./... && go vet ./...` → exit 0.

### Step 2: Use `physical` when collecting destinations in `Restore`

Replace the `path, err = filepath.Abs(path)` call in the excerpt above with
`path, err = physical(path)`. Nothing else in the loop changes: the existing
`destinations[path]` conflict check and `os.Lstat` preflight now operate on
canonical paths. Keep the error text for conflicts as is
(`snapshot contains conflicting contents for %s`).

Also keep the write loop unchanged; it iterates `destinations`, which is now
deduplicated physically.

**Verify**: `go test ./internal/snapshot -count=1` → `ok` (all existing tests
still pass).

### Step 3: Add alias tests

Append to `internal/snapshot/snapshot_test.go`, modelled on
`TestHomeAsProject` and the symlink part of
`TestSnapshotLocationAndMissingSettings`:

```go
func TestSymlinkedProjectAliasesUserScope(t *testing.T) {
	_, archive := setup(t)
	home := os.Getenv("HOME")
	link := filepath.Join(t.TempDir(), "project-link")
	if err := os.Symlink(home, link); err != nil {
		t.Fatal(err)
	}
	// Save with the alias: user and project keys hold identical bytes.
	put(t, paths(home)["user"], []byte(`{"model":"saved"}`))
	if _, err := Save(archive, link); err != nil {
		t.Fatal(err)
	}
	os.Remove(paths(home)["user"])
	os.Remove(filepath.Join(home, ".claude"))
	// Non-forced restore must succeed once, not fail on the second alias.
	if n, err := Restore(archive, "", false); err != nil || n != 1 {
		t.Fatalf("restore = %d, %v", n, err)
	}
}

func TestConflictingAliasContentsAreRejected(t *testing.T) {
	_, archive := setup(t)
	home := os.Getenv("HOME")
	link := filepath.Join(t.TempDir(), "project-link")
	if err := os.Symlink(home, link); err != nil {
		t.Fatal(err)
	}
	a := Archive{Version: 1, Project: link, Files: map[string][]byte{
		"user":    []byte(`{"model":"a"}`),
		"project": []byte(`{"model":"b"}`),
	}}
	data, _ := json.Marshal(a)
	put(t, archive, data)
	if _, err := Restore(archive, "", true); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(paths(home)["user"]); !os.IsNotExist(err) {
		t.Fatal("wrote before rejecting the conflict")
	}
}
```

Rationale for the first test's expected count of 1: `Save` with the alias
stores `user` and `project` (same bytes), and the directory for `.claude`
does not exist at restore time, so `physical` must resolve through the
symlinked parent `link -> home` even though `.claude` is absent. That is the
exact case the old code got wrong.

**Verify**: `go test ./internal/snapshot -count=1 -v 2>&1 | grep -E '^(--- |ok|FAIL)'`
→ 8 `--- PASS` lines and `ok`.

**Verify the tests catch the bug**: temporarily change `physical(path)` back
to `filepath.Abs(path)` in `Restore`, run the same command, and confirm
`TestSymlinkedProjectAliasesUserScope` fails with a "file exists" error.
Revert the temporary change. Run again → all pass.

### Step 4: Document the commands in the README CLI block

In `README.md`, inside the `## CLI` fenced block (currently ending with
`ccfg statusline off             # put the previous status line back`), add:

```sh
ccfg snapshot ~/claude-settings.json   # save every settings file outside .claude
ccfg restore ~/claude-settings.json    # put them back (--force to replace existing)
```

Align the `#` comments with the existing column. Leave the
`## Snapshot and restore` section as is unless a sentence there contradicts
the new behavior; it does not, as of planning.

**Verify**: `grep -n 'ccfg snapshot\|ccfg restore' README.md` → at least 4
matches (2 in the CLI block, 2 in the snapshot section).

### Step 5: Full gates

**Verify**:

```sh
gofmt -l .                      # prints nothing
go build ./... && go vet ./...  # exit 0
go test ./... -count=1          # every package ok
go test -race ./internal/snapshot ./internal/cli -count=1   # ok
```

## Test plan

- New tests (both in `internal/snapshot/snapshot_test.go`):
  - `TestSymlinkedProjectAliasesUserScope` — regression for finding 13,
    non-forced path.
  - `TestConflictingAliasContentsAreRejected` — forced path, differing bytes,
    must fail before any write.
- Existing tests to keep green: all 6 in `internal/snapshot`,
  `TestSnapshotCLI` in `internal/cli`.
- Pattern: `TestHomeAsProject` (same-file alias by identical lexical path).

## Done criteria

- [ ] `go build ./...`, `go vet ./...` exit 0; `gofmt -l .` prints nothing
- [ ] `go test ./... -count=1` exits 0 with the 2 new tests present and passing
- [ ] `grep -n 'filepath.Abs(path)' internal/snapshot/snapshot.go` matches
      only inside `physical`, not inside `Restore`
- [ ] README CLI block lists `ccfg snapshot` and `ccfg restore`
- [ ] `git status` shows no modified files outside the in-scope list (the
      pre-existing RTK removal and untracked `docs/` / `plans/` are expected
      and must be left alone)
- [ ] `plans/README.md` status row for 006 updated

## STOP conditions

Stop and report back (do not improvise) if:

- `internal/snapshot/snapshot.go` or `internal/cli/snapshot.go` is missing or
  its `Restore` loop does not match the excerpt above (the feature was
  committed, reworked, or dropped since planning).
- Your worktree was created from HEAD and lacks the untracked snapshot files.
- `TestSymlinkedProjectAliasesUserScope` still passes with the fix reverted
  (the test is not exercising the bug; do not ship a test that cannot fail).
- `filepath.EvalSymlinks` behaves differently than expected on the platform
  (for example a test fails only on Windows). Report, do not add build tags.
- The fix seems to need changes in `internal/store` or `internal/statusline`.

## Maintenance notes

- Plan 007 (future, audit finding 7) will make `store.Path` honor
  `CLAUDE_CONFIG_DIR`. `paths()` here derives from `store.Path`, so it will
  pick that up automatically; re-run the snapshot tests then with the variable
  set and unset.
- If a new per-scope companion file is ever added beside settings, add it to
  `paths()` with a new key and bump nothing: unknown keys in old archives are
  already rejected, and old archives lacking the key restore fine.
- Reviewer focus: `physical` must never follow symlinks in the _missing_ tail
  (there is nothing to follow), and must not error on a missing `.claude`
  directory. The first new test covers both.
- The snapshot file is plaintext and may contain secrets copied from
  settings (`env` values, hook commands). This is documented in the README
  and was accepted in the audit; do not add encryption here.
