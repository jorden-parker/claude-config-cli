# claude-config

A terminal UI and CLI for every Claude Code setting, built with the
[Charm](https://charm.land/) libraries (Bubble Tea, Bubbles, Lip Gloss, Huh, Fang).

Every key, its allowed values, its default, and which file it belongs in come
from the official reference: <https://code.claude.com/docs/en/settings-reference>.
The tool refuses values the docs don't allow and refuses to write a key into a
file the docs say Claude Code won't read it from.

## Install

```sh
go install github.com/jorden-parker/claude-config-cli/cmd/claude-config@latest
```

Or build locally:

```sh
go build ./cmd/claude-config
```

## Interactive editor

```sh
claude-config            # in a project directory
claude-config -C ~/repo  # another project
```

| Key         | Action                                       |
| ----------- | -------------------------------------------- |
| `enter`     | edit the highlighted setting                 |
| `tab`       | single-value setting: set the next option    |
| `u`         | remove it from the target file               |
| `s`         | switch the target file: user, project, local |
| `/`         | filter by key or section                     |
| `[` `]`     | jump between sections                        |
| `o`         | open the docs page for the key               |
| `r`         | reload the settings files                    |
| `q`         | quit                                         |

Global-config keys (`~/.claude.json`) always write there, whatever the target.
Managed settings are shown read-only.

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
claude-config list                       # every key, grouped by section
claude-config list -q sandbox            # filter
claude-config list sections
claude-config doc permissions.defaultMode
claude-config get model                  # value in every file
claude-config set effortLevel high       # writes ~/.claude/settings.json
claude-config set theme light -s project
claude-config set permissions.allow 'Bash(npm run *),Read(./.env)' -s local
claude-config set env 'FOO=bar' -s project
echo '{"type":"command","command":"~/bin/status.sh"}' | claude-config set statusLine
claude-config unset theme -s project
claude-config show -s user
claude-config paths
claude-config env CLAUDE_CODE_            # documented environment variables
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
