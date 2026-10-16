# Claude Code worktree paths and settings

Researched 2026-10-07 against Anthropic's current documentation. Local CLI reports 2.1.292. Research only: no Claude settings or worktrees changed, and examples were not executed.

**For a custom CLI worktree directory, create it with Git and launch Claude there, or configure a `WorktreeCreate` hook. No CLI `worktree.root` or `worktree.path` setting is documented.** The settings reference enumerates four worktree properties; none controls the destination directory. This is a conclusion about the documented interface, not a claim about undocumented internals. [Settings reference](https://code.claude.com/docs/en/settings-reference#worktree)

## 1. Choose the directory

Ordinary CLI usage creates `<repository>/.claude/worktrees/feature-auth/`, on branch `worktree-feature-auth`:

```sh
claude --worktree feature-auth
# Equivalent: claude -w feature-auth
```

The argument is a name, not a documented arbitrary-path option. Omit it for a generated name. For an explicit location, run this from the repository root, using an unused directory and branch name:

```sh
git worktree add ../claude-config-cli-feature-auth -b feature-auth
cd ../claude-config-cli-feature-auth
claude
```

The manual Git route also lets you select an existing branch. Do not relocate automatic worktrees by symlinking `.claude/worktrees`: Claude Code rejects symlinks at `.claude`, `.claude/worktrees`, or the destination itself. [Worktree creation and manual management](https://code.claude.com/docs/en/worktrees)

**Desktop has a separate location control:** Settings → Claude Code → Worktree location. Desktop also offers a worktree branch prefix. The documentation does not expose these desktop controls as CLI JSON settings. [Desktop sessions](https://code.claude.com/docs/en/desktop#work-in-parallel-with-sessions)

## 2. Configure the supported settings

All four properties accept any settings-file scope. [Worktree settings reference](https://code.claude.com/docs/en/settings-reference#worktree)

| Key                           | Values and default                                | Effect                                                                                                |
| ----------------------------- | ------------------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| `worktree.baseRef`            | `"fresh"` (default), `"head"`                     | Remote default branch or current checkout's committed `HEAD`; arbitrary branch names are unsupported. |
| `worktree.symlinkDirectories` | Directory-string array; unset by default          | Share directories from the main repository; paths are repository-relative.                            |
| `worktree.sparsePaths`        | Directory-string array; unset means full checkout | Select checked-out directories plus root-level files.                                                 |
| `worktree.bgIsolation`        | `"worktree"` (default), `"none"`                  | Control background-job isolation; not the destination directory.                                      |

For example, merge this into the chosen settings file:

```json
{
  "worktree": {
    "baseRef": "head",
    "symlinkDirectories": ["node_modules"]
  }
}
```

`"head"` includes committed local work, not uncommitted edits. The background setting has an exception: sessions moved to the background with `←` or `/background` edit in place regardless of it. [Exact property semantics](https://code.claude.com/docs/en/settings-reference#worktree)

## 3. Put settings in the right scope

Use `~/.claude/settings.json` for personal defaults, `.claude/settings.json` for committed project defaults, or `.claude/settings.local.json` for personal project overrides. `CLAUDE_CONFIG_DIR` relocates user configuration; it is not documented as a worktree-location control. Priority is managed policy → command line → local → project → user. Lists combine across scopes rather than replacing lower-priority lists. [Files and precedence](https://code.claude.com/docs/en/settings)

A one-session experiment without saving settings:

```sh
claude --settings '{"worktree":{"baseRef":"head"}}' -w feature-auth
```

On current macOS/Linux versions, local settings normally resolve to the main checkout's root even from a worktree. Exceptions include Windows and ownership checks. Shared settings resolve from the session's primary working directory. Use `/status` to check which files loaded. [Local-file placement and session overrides](https://code.claude.com/docs/en/settings#where-claude-code-keeps-the-local-file-in-a-git-repository)

For sparse checkouts, list directories relative to the repository root, and include `.claude` to retain tracked root settings and rules:

```json
{
  "worktree": {
    "sparsePaths": [".claude", "packages/api", "packages/shared"]
  }
}
```

Sparse and symlink settings are read before creation from the launch directory; the resulting session starts at the worktree root and loads project settings there. All worktrees in a session use the same sparse list. Local lists can add paths but cannot subtract project entries. [Monorepo worktree configuration](https://code.claude.com/docs/en/large-codebases#check-out-only-the-directories-you-need)

## 4. Automate a custom location with hooks

Configure `hooks.WorktreeCreate` with a command script. The script reads JSON stdin containing `name` and common fields such as `cwd`, creates the checkout at your chosen location, and prints its normalized absolute path as the last nonempty stdout line. Send diagnostics to stderr. This replaces built-in creation entirely; implement your own file copying and other setup. [Creation hook contract](https://code.claude.com/docs/en/hooks#worktreecreate)

Return an existing directory without `.` or `..` path components or symlinks below the repository root. Pair it with `WorktreeRemove`, which receives `worktree_path`; custom branch cleanup belongs in that hook too. Without a removal hook, interactive removal falls back to `git worktree remove --force` for recognized Git worktrees. [Path validation and cleanup](https://code.claude.com/docs/en/hooks#worktreecreate-output)

For standard creation, `.worktreeinclude` copies matching gitignored files, such as `.env`; hook-based creation bypasses it. `${CLAUDE_PROJECT_DIR}` stays anchored at the session's original project, while the hook input's `cwd` tracks the current directory. [Worktree setup](https://code.claude.com/docs/en/worktrees)

Repository implication: [the bundled schema](../../internal/schema/schema.json) already exposes all four documented properties. This research found no missing built-in path setting to add; no schema change is needed.

Next action (30 seconds): choose manual Git creation for a one-off custom path, or Desktop's Worktree location if using the desktop app.
