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

Each section of the reference (Model, Permissions, Sandbox, … Tools) has its
own pane; pick one from the list on the left. Nested keys such as
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
  press **p**, because a repository can supply those files.

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
```

Scopes: `user` (default), `project`, `local`, `global`.

`ccfg ls` includes the full description of each setting. Use `ccfg doc KEY`
for its description, allowed values, default, scope, and example.

## Recipe: route command output through RTK

[RTK](https://github.com/rtk-ai/rtk) shortens the output of shell commands
such as `grep`, `pnpm test`, and `bun test` before Claude Code reads it. It
installs its own `PreToolUse` hook, so `ccfg` has no option for it. Two steps
cover every command:

```sh
brew install rtk
rtk init -g --auto-patch        # hook + ~/.claude/RTK.md, backs up settings.json
rtk init --show                 # confirm the hook is listed
```

The hook rewrites `Bash` commands only. Claude Code's built-in `Read`, `Grep`,
and `Glob` tools do not pass through it. To push searches into the shell, where
the hook applies, turn `Grep` and `Glob` off in the **Tools** pane; this adds
them to `permissions.deny` in the chosen file. Leave `Read` on: `Edit` needs a
prior `Read` of the file and fails without it.

Check it works after a session with `rtk gain`. Commands that must stay raw
go in `exclude_commands` under `[hooks]` in rtk's `config.toml`
(`~/Library/Application Support/rtk/config.toml` on macOS).

## Recipe: caveman mode and the generated status line

The caveman hook and the status line script in `~/.claude` on this machine
come from [claude-ui](https://github.com/jmep17/claude-ui), a separate local
dashboard, not from `ccfg`. It vendors the `caveman` and `caveman-compress`
skills from [caveman](https://github.com/JuliusBrussee/caveman) and installs
them as a "setup piece": the two skills, a `SessionStart` hook at
`~/.claude/hooks/caveman-activate.py` that prints the ruleset into every
session, and the level file `~/.claude/.caveman-active`. Its status line
piece writes `~/.claude/statusline.sh` and points `statusLine` at it.

Install from the browser:

```sh
git clone https://github.com/jmep17/claude-ui.git
claude-ui/bin/claude-ui          # opens http://localhost:7333
```

Then open the **Setup** tab and press **Apply** on _Caveman_ and _Claude Code
statusline_. The same pieces apply from a terminal with no server running:

```sh
cd claude-ui/bin
python3 -c "from claude_ui.setup import setup_apply; setup_apply('caveman')"
python3 -c "from claude_ui.setup import setup_apply; setup_apply('statusline')"
python3 -c "from claude_ui.setup import setup_state; print(setup_state())"   # verify
```

Both are idempotent and reversible with `setup_remove(...)`. Caveman appends
one block to `hooks` in `settings.json` and removes only that block. Change
the level by writing `lite`, `full`, `ultra`, or a `wenyan-*` value to
`~/.claude/.caveman-active`; delete the file to turn it off. A status line
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
