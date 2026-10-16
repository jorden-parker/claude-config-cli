# claude-config-cli

A little workshop for your Claude Code settings. Browse, tinker, and tend your
variable garden from a terminal UI and CLI built with the
[Charm](https://charm.land/) libraries (Bubble Tea, Bubbles, Lip Gloss, Huh, Fang).

Every key, its allowed values, its default, and which file it belongs in come
from the official reference: <https://code.claude.com/docs/en/settings-reference>.
The tool refuses values the docs don't allow and refuses to write a key into a
file the docs say Claude Code won't read it from.

## Install

On macOS or Linux, install with Bash, Git, and Go (the Go version in `go.mod`
or newer; Go's automatic toolchain download can supply it):

```sh
curl -fsSL https://raw.githubusercontent.com/jorden-parker/claude-config-cli/main/install.sh | bash
```

The script builds from `main`, installs both `ccfg` and `claude-config-cli` into
`~/.local/bin`, and adds that directory to your shell's PATH (Bash, Zsh, or Fish).
Open a new terminal, then run `ccfg --help`. Both commands run the same CLI;
the examples below use the short name. Run the installer again to update.

From a local checkout, install your current source with:

```sh
./install.sh
```

Or build locally:

```sh
go build .            # claude-config-cli
go build ./cmd/ccfg   # ccfg
```

## Interactive editor

A tiny `˙ᵕ˙` workshop companion greets you, with contextual hints for exploring
settings, tending environment variables, and browsing the toolbox. Save messages
keep the exact setting and destination visible; the command-line data output stays
plain for scripts. Press **Ctrl+K** to open the **curiosity cabinet**, a Huh picker
with searchable section shortcuts and **Surprise me**, which reveals a random
non-deprecated setting and its documentation without changing its value. Use `/`
to filter destinations and `esc` to return. The orchid, honey, and mint palette adapts to light and dark terminals. Editors
and confirmations open on a framed workbench, with the destination visible.
Press **Shift+S** for a destination picker that explains user, project, and local
settings. **Ctrl+D** expands documentation; **Esc** brings you back.

```sh
ccfg                            # in a project directory
ccfg -C ~/repo                  # another project
```

| Key           | Action                                                              |
| ------------- | ------------------------------------------------------------------- |
| `ctrl+k`      | open the curiosity cabinet: jump to a section or discover a setting |
| `enter`       | edit the setting, open a group, or toggle a tool                    |
| `←` `→`       | move between the section list and its keys                          |
| `esc` `←`     | go back out of a group                                              |
| `tab`         | next value, or turn the tool on or off                              |
| `u`           | remove the key from the target file                                 |
| `shift+s`     | choose a target file with descriptions                              |
| `ctrl+d`      | expand details (Esc returns)                                        |
| `s`           | switch the target file: user, project, local                        |
| `/`           | search every key and section, best matches first (`esc` clears it)  |
| `[` `]`       | open the previous or next section                                   |
| `pgup` `pgdn` | scroll the details pane                                             |
| `o`           | open the docs page for the key                                      |
| `J` `K`       | status line: move a shown field later or earlier                    |
| `p`           | status line: run the existing command for the preview               |
| `r`           | reload the settings files                                           |
| `m`           | mouse on or off; off lets your terminal select text                 |
| `?`           | show every key                                                      |
| `q`           | quit                                                                |

### Mouse

The mouse works in the browse screen, on by default:

- **Wheel** scrolls the pane under the pointer: the sections, the keys, or the
  details. It moves one section or three keys per notch.
- **Click a section** to open it. **Click a key row** to select it; click the
  same row again to act like `enter` (edit, open a group, or toggle a tool).
- **Click the details pane** to move keyboard focus off the sections.
- **Click a footer hint** (such as `ctrl+k` or `/`) to run that key.
- Forms and dialogs ignore the mouse, so Huh inputs and the editor work as
  before. The field guide ignores clicks but its wheel still scrolls.

Press `m` to turn the mouse off, so your terminal can select and copy text.
Press `m` again to turn it back on. The status row says which state you are in.
To start with the mouse off, set `CCFG_NO_MOUSE=1`.

Each section of the reference (Model, Permissions, Sandbox, … Tools) has its
own pane; pick one from the list on the left. Nested keys such as
`permissions.allow` live inside group rows (`permissions ›`); press `enter` to
open one. The Tools pane also lists every Claude Code tool with an on/off
toggle. Each row shows the value in effect and the file it comes
from, with a description preview underneath. The details pane starts with
**What it does**, explaining the selected setting, then lists the settings files from highest to lowest
priority, shows which value wins, and marks the target file your edits go to.
Search ranks matches on the setting name first, then the full key, the section,
and words in the description. Terms under three letters skip descriptions.
In the edit form, `esc` cancels without saving.

Global-config keys (`~/.claude.json`) always write there, whatever the target.
Managed settings are shown read-only.

The **Tools** view lists Claude Code tools such as `NotebookEdit`, `Bash`,
and `Read`. Settings and Tools share one column, with only one visible at a time.
Every tool includes a searchable description preview and a **What it does**
section in the details pane.
Choose Tools from the section list or the curiosity cabinet. In Tools,
filter with `/NotebookEdit`, then
press `tab` (or `enter`) to toggle the tool in the selected target file.
The details pane follows the focused list; `pgup` / `pgdn` scroll its contents.
At 100 columns and wider, documentation sits beside the settings. Smaller
terminals show a wider setting list; press `ctrl+d` to read details. Below
60 columns, sections and settings share one pane; use `←` and `enter` to switch.

Disabling adds the tool's bare name to `permissions.deny` and removes it (and
`Tool(*)`) from `permissions.allow` in the same file; enabling removes that exact deny entry
and does not restore the allow rule. Normal permission prompts, scoped rules, and denies in other files
still apply. Tool availability depends on your Claude Code version and session;
the catalogue comes from the [tools reference](https://code.claude.com/docs/en/tools-reference).
Reload and save messages appear above the keyboard-help footer, which stays visible.

### Environment variables

Open **Environment** in the section list, or press `/` and search a variable
name. The ✿ **Variable garden** lists all 377 variables in the bundled Claude
Code reference, plus custom names already present in your settings. The details
pane shows the full description, suggested choices, and settings-file precedence.

- **Tab in the list** cycles and saves documented choices, such as effort levels
  and boolean strings. **Enter** opens the editor, including a custom-value option.
- **Tab / Shift+Tab in a picker** previews the next / previous choice;
  **Enter** confirms it.
- **Text and multiple values:** press Enter, type the exact value (for example
  `localhost,example.com` for `NO_PROXY`), then Enter to save. Quotes, spaces,
  commas, and empty strings are preserved. Newline-separated headers use a
  multiline editor: Alt+Enter adds a line, Enter saves. Escape cancels.
- **s** chooses user, project, or local settings; **u** asks before removing only
  the selected variable. Values are stored as strings under `env`.

This panel displays settings-file values, not the shell's exported environment.
Variables enabled by any non-empty value stay text inputs, because `0` does not
turn them off. Restart Claude Code after removing a variable or changing an
option that it reads only at startup.

The panel uses [Bubbles](https://github.com/charmbracelet/bubbles) for searchable
navigation, [Huh](https://github.com/charmbracelet/huh) for inputs and pickers, and
[Lip Gloss](https://github.com/charmbracelet/lipgloss) for adaptive terminal styles.
These fit the existing Bubble Tea event loop without introducing a second form
framework. The garden motif adds a small floral accent while retaining the app's
blue focus, amber saved values, and light/dark palettes.

### Status line

Open **Status line** in the section list to build a status line from the fields
in the [status line reference](https://code.claude.com/docs/en/statusline):
model, directory, git branch, context use, cost, rate limits, and more. The
details pane previews the result for the example session in the docs.

A status line you already have is kept. Claude Code runs one `statusLine`
command, so `ccfg` points that key at its own renderer and the renderer runs
your existing command with the same input. Its output, such as a plugin's badge,
appears beside the fields you pick, in the preview and in Claude Code.

- **Tab or Enter** turns a field on or off, and cycles the other rows.
  **J / K** (or Shift+↓ / Shift+↑) reorder the shown fields.
- **Use this status line** makes it live in the target file. Until then your
  picks are saved but `statusLine` is untouched. Turning it off puts the
  original `statusLine` value back. `padding`, `refreshInterval`, and
  `hideVimModeIndicator` are kept as they were.
- **Existing status line** places the kept output at the start or end of the
  line, on its own row above or below, or hides it.
- A command from the project or local file is not run for the preview until you
  press **p**, because a repository can supply those files. A command whose
  origin is unknown is treated the same way.
- Turning the status line on checks that the renderer runs, without running
  your saved command (`ccfg statusline render --check`). The check runs in the
  background, and **Esc** cancels it. Claude Code runs the saved command as
  usual when it draws the status line, and **p** is how you run an untrusted
  or unknown-origin command for the preview.
- The config records which file the kept command came from. A project or local
  config never counts as trusted, whatever it says. A config written by an
  older version has no record, so its kept command needs **p** until you turn
  the status line off and on again.

Your picks live next to the settings file: `~/.claude/ccfg-statusline.json` for
user settings, `.claude/ccfg-statusline.json` for project settings, and
`.claude/ccfg-statusline.local.json` for local settings. Project settings call
`ccfg` by name, so teammates need it on their PATH. If the existing command
fails, times out after 3 seconds, or prints nothing, it is left out and the
rest still renders.

Spaces at either end of the existing command's rows are trimmed where they meet
your fields, so the separator is the only gap and the rows line up. A run of
spaces anywhere in the line becomes one space, in the existing command's output
too. Colours are left alone.

Editors per value kind:

- **bool, enum**: a picker with each option's meaning from the docs
- **enum-or-string** (`model`, `theme`, `outputStyle`, ...): picker plus a
  free-text "custom…" entry
- **array**: one item per line
- **map** (`env`, `enabledPlugins`): `KEY=VALUE` per line
- **json** (`hooks`, `statusLine`, `sandbox`, ...): a JSON editor pre-filled
  with the docs example; `ctrl+e` opens `$EDITOR`

### Context diet

Open **Context diet** in the section list for a checklist of the settings and
habits that cut what Claude Code sends as context. Each row shows a state:

- **✓ applied**: in place. The badge names the file it is set in (`user`,
  `project`, `local`, `managed`, or `global`), or `default` when Claude Code's
  default already does it.
- **○ not applied**: not in place yet.
- **▲ warning**: a trap is set, such as `ENABLE_TOOL_SEARCH=false`. The
  setting does nothing, or costs more than it saves.
- **◇ info**: ccfg cannot check it from settings files, such as `/clear` habits.

Each row has a one-line reason under its title. The details pane shows what
the row does, its caveat, which files set its keys and to what value, and the
docs page. `o` opens that page.

- **Enter** on a not-applied row writes the change to the target file (`s`
  changes the target), with the same checks the settings editor uses. Rows
  where you choose the value, such as effort per model or the list of disabled
  servers, open the editor for that key instead.
- **Enter** on an applied row only says that it is applied, and where.
- **Enter** on a warning row asks before it removes the trap key from the
  target file. If the trap is in another file, the message names that file.
- **Enter** on a habit or a check row shows its details.

ccfg reads values the way Claude Code does: managed, then local, project, and
user, with `~/.claude.json` for global keys. It also counts the lines in
`CLAUDE.md` and `AGENTS.md`, reads `alwaysLoad` in `.mcp.json` and
`~/.claude.json`, and reads the `model:` line of subagent files. It never runs
Claude Code, and it changes only the file you choose.

## CLI

```sh
ccfg ls                         # every key, grouped by section (alias of list)
ccfg ls -q sandbox              # filter, best matches first
ccfg ls sections
ccfg doc permissions.defaultMode
ccfg get model                  # value in every file
ccfg set effortLevel high       # writes ~/.claude/settings.json
ccfg set theme light -s project
ccfg set permissions.allow 'Bash(npm run *),Read(./.env)' -s local
ccfg set env 'FOO=bar' -s project
echo '{"type":"command","command":"~/bin/status.sh"}' | ccfg set statusLine
ccfg rm theme -s project        # alias of unset
ccfg show -s user
ccfg paths
ccfg env CLAUDE_CODE_           # documented environment variables
ccfg statusline fields          # every status line field and the data it reads
ccfg statusline preview         # render it for the docs' example session
ccfg statusline on --fields model,branch,ctx_used --existing end
ccfg statusline off             # put the previous status line back
ccfg snapshot ~/claude-settings.json   # save every settings file outside .claude
ccfg restore ~/claude-settings.json    # put them back (--force to replace existing)
ccfg sync                       # refresh the list of settings from the official docs
ccfg sync --check               # only report what the docs changed (exit 1 on drift)
ccfg check                      # find permission rules that contradict each other
ccfg diet                       # the context diet checklist: applied, not applied, traps (--json)
```

Scopes: `user` (default), `project`, `local`, `global`.

`ccfg ls` includes the full description of each setting. Use `ccfg doc KEY`
for its description, allowed values, default, scope, and example.

## Snapshot and restore

Save your settings before deleting Claude Code's configuration:

```sh
ccfg snapshot ~/claude-settings-snapshot.json
# After your configuration reset:
ccfg restore ~/claude-settings-snapshot.json
```

The snapshot includes existing user, global, project, and local settings, plus
ccfg's status-line companion files. It preserves complete file contents,
including unknown keys and settings edited outside ccfg. Missing files and
managed settings are skipped. Plugins, external hook scripts, session history,
and credentials stored elsewhere are not backed up; reinstall those separately.

Choose a new snapshot filename outside `.claude`; existing snapshots are never
replaced. The file is private (mode `0600`), but not encrypted, and may contain
secrets from your settings. Keep it somewhere that survives your reset.

Restore uses your current home and the original project's absolute directory.
Use `ccfg restore FILE -C ~/new-project` to restore project and local settings
elsewhere. Commands embedded in settings retain their original paths.
Existing destination files cause restore to stop before writing anything;
`--force` explicitly replaces files included in the snapshot. Other files are
left alone. Each file is restored separately; an I/O failure reports how many
files were restored before the failure. Restart Claude Code after restoring.

## How files are written

ccfg saves settings and status-line files by writing a complete temporary file
next to the target, then renaming it over the original. A failed write leaves
the old file intact. Existing symlinks are followed and kept; dangling links
and non-regular files (directories, pipes) are refused. Hard links and extended
metadata (ACLs, xattrs) are not preserved. This does not prevent conflicting
edits from other programs, and it does not make multi-file changes atomic.

## Recipe: caveman mode with hooks and the status line badge

[caveman](https://github.com/JuliusBrussee/caveman) ships as a Claude Code
plugin. Its installer wires the plugin, the Claude Code hooks, and the
`[CAVEMAN]` status line badge in one go (Node.js 22.13 or newer):

```sh
curl -fsSL https://raw.githubusercontent.com/JuliusBrussee/caveman/v3.1.0/install.sh | bash -s -- --only claude --with-hooks --force
```

`bash -s --` passes the flags through to the installer: `--only claude`
limits it to Claude Code, `--with-hooks` wires the hooks into
`settings.json`, and `--force` reinstalls a plugin that is already there.
From a checkout, the same installer runs as `node bin/install.js` with the
same flags; add `--dry-run` to see what it would write first. Plugin only,
without the hooks or the badge:

```sh
claude plugin marketplace add JuliusBrussee/caveman && claude plugin install caveman@caveman
```

Verify after a restart:

```sh
cat "${CLAUDE_CONFIG_DIR:-$HOME/.claude}/.caveman-active"   # expected: caveman
```

The hooks live in `~/.claude/hooks/` (`caveman-activate.js`,
`caveman-mode-tracker.js`, `caveman-statusline.sh`, and more) with matching
entries in `settings.json`. `/caveman`, `/ultracave`, and `/megacave` set the
level per window; `/caveman off` or saying `stop caveman` turns it off. Update
with `claude plugin update caveman@caveman` and restart, since hooks are read
once at session start. Uninstall with
`npx -y github:JuliusBrussee/caveman -- --uninstall`.

Source: the caveman README and `INSTALL.md` at v3.1.0. A status line
installed this way shows up in `ccfg statusline` as the existing command and
is kept when you turn `ccfg`'s fields on.

## Development

Install the Git hooks after cloning (requires Bun and Go):

```sh
bun install
```

Commits format staged files with Prettier and Go files with `gofmt`, then run
`go build ./...` and `go test ./...`. Tests never touch the network or your
synced catalogue. Commit messages must follow
[Conventional Commits](https://www.conventionalcommits.org/), for example
`feat: add a setting` or `fix: correct scope validation`.

## Keeping the catalogue current

Every key, type, and default comes from the official docs. The list is built
in, and `ccfg sync` refreshes it without a new build:

```sh
ccfg sync            # download the three docs pages and save the catalogue
ccfg sync --check    # report what changed since the catalogue you have; exit 1 on drift
ccfg paths           # the last line shows where the synced catalogue lives
```

`sync` saves to `ccfg/schema.json` under your user cache directory
(`~/Library/Caches` on macOS, `$XDG_CACHE_HOME` or `~/.cache` on Linux). Set
`CCFG_CACHE_DIR` to put it elsewhere. Every command prefers that copy over
the built-in one; pass `--no-cache` to ignore it for one run. When the
catalogue is more than a week old, commands print a one-line reminder and the
editor's status row says so.

Settings and variables the docs mark as removed are left out of every list.
`ccfg get KEY` and `ccfg rm KEY` still work on them, so you can clean up an old
file.

The Tools pane is hand-written in `internal/tui/tools.go`. When the docs add
or drop a tool, `ccfg sync` says so; the rest of the catalogue still updates.

### Permission rules

`ccfg set permissions.allow`, `permissions.ask`, and `permissions.deny`, and
the same edits in the editor, keep the three lists consistent within a file:
the rule you write wins, and rules in the other two lists that contradict it
are removed and reported. Two rules contradict each other when they are
identical, or when a whole tool is denied (`Bash` or `Bash(*)`) while a rule
for that tool is allowed or asked, since the denied tool can never match it.
Allowing a whole tool while denying part of it (`Bash` allowed, `Bash(rm *)`
denied) is the usual pattern and is left alone. `ccfg check` reports
conflicts in every file without changing anything.

### For maintainers: the built-in catalogue

`internal/schema/schema.json` is generated by the same code (`internal/catalog`):

```sh
go run ./cmd/ccfg sync --embed    # or: go generate ./internal/schema
go test ./...
tools/check-tools.sh              # optional: asks your installed Claude Code to validate every tool name
```

`--embed` fails if the Tools pane disagrees with the tools reference. The
hand-curated facts the reference gives only in prose (model aliases, built-in
output style names, hook event names) live in `internal/catalog/classify.go`.
`internal/catalog/testdata/ref.md` is a checked-in copy of the reference used
by the tests; it is not an input to the build.
