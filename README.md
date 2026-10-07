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
| `/`           | search every key and section (`esc` clears it)                      |
| `[` `]`       | open the previous or next section                                   |
| `pgup` `pgdn` | scroll the details pane                                             |
| `o`           | open the docs page for the key                                      |
| `J` `K`       | status line: move a shown field later or earlier                    |
| `p`           | status line: run the existing command for the preview               |
| `r`           | reload the settings files                                           |
| `?`           | show every key                                                      |
| `q`           | quit                                                                |

Each section of the reference (Model, Permissions, Sandbox, … Tools), plus
Environment, Status line, and RTK, has its own pane; pick one from the list on the left. Nested keys such as
`permissions.allow` live inside group rows (`permissions ›`); press `enter` to
open one. The Tools pane also lists every Claude Code tool with an on/off
toggle. Each row shows the value in effect and the file it comes
from, with a description preview underneath. The details pane starts with
**What it does**, explaining the selected setting, then lists the settings files from highest to lowest
priority, shows which value wins, and marks the target file your edits go to.
Search matches setting names, sections, and descriptions. In the edit form,
`esc` cancels without saving.

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

## CLI

```sh
ccfg ls                         # every key, grouped by section (alias of list)
ccfg ls -q sandbox              # filter
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
ccfg rtk on                     # route command output through rtk (see the recipe below)
ccfg rtk status
ccfg rtk off
```

Scopes: `user` (default), `project`, `local`, `global`.

`ccfg ls` includes the full description of each setting. Use `ccfg doc KEY`
for its description, allowed values, default, scope, and example.

## Recipe: route command output through RTK

[RTK](https://github.com/rtk-ai/rtk) shortens the output of shell commands
such as `grep`, `pnpm test`, and `bun test` before Claude Code reads it.

```sh
brew install rtk
ccfg rtk on                     # or: ccfg rtk on -s project
ccfg rtk status
```

`ccfg rtk on`, or the **RTK** section in the interactive editor, adds two
`PreToolUse` hooks to the target settings file and leaves every other hook
alone. In the editor each hook is a row; `tab` turns it on or off in the
target file, and the details pane shows which hooks the file carries:

- **Bash:** rtk's own `rtk hook claude`, the same entry `rtk init -g` writes.
  It rewrites commands, for example `cat f` to `rtk read f`.
- **Read:** `ccfg rtk read-hook`, which refuses the built-in `Read` tool and
  tells Claude to run `rtk read` in Bash instead. Built-in tools never pass
  through the Bash hook, so this is the only way to send reads through rtk.
  Pass `--keep-read` to skip it.

`Grep` and `Glob` need nothing: on macOS and Linux, Claude Code leaves them
out of the default tool set and searches with `find` and `grep` through
`Bash` ([tools reference](https://code.claude.com/docs/en/tools-reference)).

Edits keep working. Claude Code accepts a plain `cat`, `head`, `tail`,
`sed -n 'X,Yp'`, or `grep` on a single file as the read that must come before
an `Edit`. rtk rewrites `cat`, `head`, and `tail` to `rtk read`, which does not
count, but leaves `sed` alone, so the Read hook's message suggests `sed -n`.
Refusing `Read` also turns off the newer models' shortcut of editing a file
without reading it first.

`ccfg rtk off` removes the two entries. Restart Claude Code after either
command; hooks are read once at session start. Check savings after a session
with `rtk gain`. Commands that must stay raw go in `exclude_commands` under
`[hooks]` in rtk's `config.toml` (`~/Library/Application Support/rtk/config.toml`
on macOS). rtk's awareness notes for `CLAUDE.md` come from
`rtk init -g --no-patch`, which prints a hook snippet you can ignore.

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

Install the Git hooks after cloning (requires Node.js, npm, and Go):

```sh
npm ci
```

Commits format staged files with Prettier and Go files with `gofmt`, then run
`go build ./...` and `go test ./...`. Commit messages must follow
[Conventional Commits](https://www.conventionalcommits.org/), for example
`feat: add a setting` or `fix: correct scope validation`.

## Keeping the schema current

`internal/schema/schema.json` is generated from the docs. To refresh it:

```sh
tools/fetch.sh   # downloads the settings, env-vars and tools references, then runs gen.py
go test ./...
tools/check-tools.sh   # optional: asks your installed Claude Code to validate every tool name
```

Settings and variables the docs mark as removed are left out of every list.
`ccfg get KEY` and `ccfg rm KEY` still work on them, so you can clean up an old
file. `gen.py` fails if the Tools pane no longer matches the tools reference.

`tools/gen.py` holds the small hand-curated overrides (model aliases, built-in
output style names, hook event names) that the reference page describes in
prose rather than as a list.
