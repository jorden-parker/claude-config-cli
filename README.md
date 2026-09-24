# claude-config-cli

A terminal UI and CLI for every Claude Code setting, built with the
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

```sh
ccfg                            # in a project directory
ccfg -C ~/repo                  # another project
```

| Key     | Action                                         |
| ------- | ---------------------------------------------- |
| `enter` | edit the highlighted setting                   |
| `tab`   | next setting option / enable or disable a tool |
| `u`     | remove it from the target file                 |
| `s`     | switch the target file: user, project, local   |
| `/`     | filter by key or section                       |
| `[` `]` | jump between sections                          |
| `o`     | open the docs page for the key                 |
| `r`     | reload the settings files                      |
| `q`     | quit                                           |

Global-config keys (`~/.claude.json`) always write there, whatever the target.
Managed settings are shown read-only.

The **Tools** section lists Claude Code tools such as `NotebookEdit`, `Bash`,
and `Read`, displayed by name without a `tools.` prefix. Jump between sections
with `[` / `]`, or filter `/Tools` to show only tools. Filter with `/NotebookEdit`, then press `tab` (or `enter`) to toggle
that tool in the selected target file. Disabling adds its bare name to
`permissions.deny`; enabling removes that exact entry. Normal permission prompts,
scoped rules, and denies in other files still apply. The details pane shows which
files disable the tool. Tool availability depends on your Claude Code version
and session; the catalogue comes from the [tools reference](https://code.claude.com/docs/en/tools-reference).

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
```

Scopes: `user` (default), `project`, `local`, `global`.

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
tools/fetch.sh   # downloads settings-reference.md and env-vars.md, then runs gen.py
go test ./...
```

`tools/gen.py` holds the small hand-curated overrides (model aliases, built-in
output style names, hook event names) that the reference page describes in
prose rather than as a list.
